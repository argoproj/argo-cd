package grpc

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/golang-jwt/jwt/v5"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/selector"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

func logRequest(ctx context.Context, entry *logrus.Entry, info string, pbMsg any, logClaims bool) {
	if logClaims {
		claims := ctx.Value("claims")
		mapClaims, ok := claims.(jwt.MapClaims)
		if ok {
			claimsCopy := make(map[string]any)
			for k, v := range mapClaims {
				if k != "groups" || entry.Logger.IsLevelEnabled(logrus.DebugLevel) {
					claimsCopy[k] = v
				}
			}
			if data, err := json.Marshal(claimsCopy); err == nil {
				entry = entry.WithField("grpc.request.claims", string(data))
			}
		}
	}
	// The logging interceptor runs ahead of this one and injects its fields into the context, so the
	// source IP ends up on the same line as the claims rather than only on the call lines.
	it := logging.ExtractFields(ctx).Iterator()
	for it.Next() {
		switch k, v := it.At(); k {
		case sourceIPField, forwardedForField:
			entry = entry.WithField(k, v)
		}
	}
	if p, ok := pbMsg.(proto.Message); ok {
		entry = entry.WithField("grpc.request.content", &jsonpbMarshalleble{p})
	}
	entry.Info(info)
}

type jsonpbMarshalleble struct {
	proto.Message
}

func (j *jsonpbMarshalleble) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	m := &jsonpb.Marshaler{}
	err := m.Marshal(&b, j.Message)
	if err != nil {
		return nil, fmt.Errorf("jsonpb serializer failed: %w", err)
	}
	return b.Bytes(), nil
}

type reporter struct {
	ctx       context.Context
	entry     *logrus.Entry
	logClaims bool
	info      string
}

func (r *reporter) PostCall(_ error, _ time.Duration) {}

func (r *reporter) PostMsgSend(_ any, _ error, _ time.Duration) {}

func (r *reporter) PostMsgReceive(payload any, err error, _ time.Duration) {
	if err == nil {
		logRequest(r.ctx, r.entry, r.info, payload, r.logClaims)
	}
}

func PayloadStreamServerInterceptor(entry *logrus.Entry, logClaims bool, decider func(context.Context, interceptors.CallMeta) bool) grpc.StreamServerInterceptor {
	return selector.StreamServerInterceptor(interceptors.StreamServerInterceptor(reportable(entry, "streaming", logClaims)), selector.MatchFunc(decider))
}

func PayloadUnaryServerInterceptor(entry *logrus.Entry, logClaims bool, decider func(context.Context, interceptors.CallMeta) bool) grpc.UnaryServerInterceptor {
	return selector.UnaryServerInterceptor(interceptors.UnaryServerInterceptor(reportable(entry, "unary", logClaims)), selector.MatchFunc(decider))
}

func reportable(entry *logrus.Entry, callType string, logClaims bool) interceptors.CommonReportableFunc {
	return func(ctx context.Context, c interceptors.CallMeta) (interceptors.Reporter, context.Context) {
		return &reporter{
			ctx:       ctx,
			entry:     entry,
			info:      fmt.Sprintf("received %s call %s", callType, c.FullMethod()),
			logClaims: logClaims,
		}, ctx
	}
}

// InterceptorLogger adapts logrus logger to interceptor logger.
func InterceptorLogger(l logrus.FieldLogger) logging.Logger {
	return logging.LoggerFunc(func(_ context.Context, lvl logging.Level, msg string, fields ...any) {
		f := make(map[string]any, len(fields)/2)
		i := logging.Fields(fields).Iterator()
		for i.Next() {
			k, v := i.At()
			f[k] = v
		}
		l := l.WithFields(f)

		switch lvl {
		case logging.LevelDebug:
			l.Debug(msg)
		case logging.LevelInfo:
			l.Info(msg)
		case logging.LevelWarn:
			l.Warn(msg)
		case logging.LevelError:
			l.Error(msg)
		default:
			panic(fmt.Sprintf("unknown level %v", lvl))
		}
	})
}

const (
	// GatewayTokenMetadataKey carries a per-process secret proving that a call was relayed by this
	// process's own grpc-gateway, and ClientIPMetadataKey the address that gateway observed.
	GatewayTokenMetadataKey = "x-argocd-gateway"
	ClientIPMetadataKey     = "x-argocd-client-ip"

	sourceIPField     = "source.ip"
	forwardedForField = "forwarded.for"

	// X-Forwarded-For is unauthenticated caller-controlled input -- the logging interceptor runs ahead
	// of grpc_auth, and net/http accepts around a megabyte of headers -- so bound both the parse and
	// the log line rather than letting a caller decide how much work we do per request.
	maxForwardedForEntries = 32
	maxForwardedForLen     = 512
)

// ParseTrustedProxies parses a list of CIDRs or bare addresses.
func ParseTrustedProxies(values []string) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v == "" {
			continue
		}
		if addr, err := netip.ParseAddr(v); err == nil {
			addr = addr.Unmap()
			prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
			continue
		}
		p, err := netip.ParsePrefix(v)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy %q: %w", v, err)
		}
		prefixes = append(prefixes, p.Masked())
	}
	return prefixes, nil
}

// HTTPClientIP returns the client address of an HTTP request, as resolved by ResolveClientIP.
func HTTPClientIP(r *http.Request, trustedProxies []netip.Prefix, clientIPHeader string) string {
	var headerIP string
	if clientIPHeader != "" {
		if v := r.Header.Values(clientIPHeader); len(v) > 0 {
			headerIP = v[len(v)-1]
		}
	}
	return ResolveClientIP(r.RemoteAddr, headerIP, r.Header.Values("X-Forwarded-For"), trustedProxies)
}

// ResolveClientIP returns the address of the nearest hop that is not a trusted proxy. If remote is not
// trusted, that is remote itself. Otherwise it is headerIP, when the proxy set one, or else the
// rightmost X-Forwarded-For entry that is not trusted. xff holds one value per header line.
//
// Only entries to the right of an untrusted one could have been chosen by the client, so the walk
// goes right to left and stops at an entry that is not a valid address rather than returning it.
func ResolveClientIP(remote string, headerIP string, xff []string, trustedProxies []netip.Prefix) string {
	remote = stripPort(remote)
	if !isTrustedProxy(remote, trustedProxies) {
		return remote
	}
	if a, err := netip.ParseAddr(stripPort(strings.TrimSpace(headerIP))); err == nil {
		return a.Unmap().String()
	}
	resolved := remote
	seen := 0
walk:
	for _, rest := range slices.Backward(xff) {
		for rest != "" {
			seen++
			if seen > maxForwardedForEntries {
				break walk
			}
			var e string
			if j := strings.LastIndexByte(rest, ','); j >= 0 {
				e, rest = rest[j+1:], rest[:j]
			} else {
				e, rest = rest, ""
			}
			if e = strings.TrimSpace(e); e == "" {
				continue
			}
			a, err := netip.ParseAddr(stripPort(e))
			if err != nil {
				break walk
			}
			resolved = a.Unmap().String()
			if !isTrustedProxy(resolved, trustedProxies) {
				break walk
			}
		}
	}
	return resolved
}

func isTrustedProxy(addr string, trustedProxies []netip.Prefix) bool {
	a, err := netip.ParseAddr(addr)
	if err != nil {
		return false
	}
	a = a.Unmap()
	for _, p := range trustedProxies {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// sourceIPFields returns the client address observed by the API server and, separately, the
// X-Forwarded-For chain that the caller supplied.
//
// The gateway serving REST and UI traffic dials the API server's own listener over localhost, so its
// gRPC peer address says nothing about the client. It identifies itself with gatewayToken instead and
// passes on the address it resolved; anything else is attributed to its socket peer, unless that peer
// is one of trustedProxies.
func sourceIPFields(ctx context.Context, gatewayToken string, trustedProxies []netip.Prefix, clientIPHeader string) (sourceIP string, forwardedFor string) {
	md, _ := metadata.FromIncomingContext(ctx)
	gateway := fromGateway(md, gatewayToken)

	// Other callers send one value per X-Forwarded-For header line, so the chain spans all of them.
	// From the gateway only the last value counts, for the same reason the token does: grpc-gateway
	// forwards a caller's Grpc-Metadata-X-Forwarded-For ahead of the chain it assembles itself.
	values := md.Get("x-forwarded-for")
	limit := maxForwardedForEntries
	if gateway {
		values = values[max(len(values)-1, 0):]
		// Leave room for the address the gateway appends, which is trimmed below.
		limit++
	}
	var xff []string
	truncated := false
parse:
	for _, v := range values {
		for e := range strings.SplitSeq(v, ",") {
			if e = strings.TrimSpace(e); e == "" {
				continue
			}
			if len(xff) == limit {
				truncated = true
				break parse
			}
			xff = append(xff, e)
		}
	}

	if !gateway {
		var headerIP string
		if clientIPHeader != "" {
			headerIP = lastValue(md, strings.ToLower(clientIPHeader))
		}
		return ResolveClientIP(peerIPFromContext(ctx), headerIP, values, trustedProxies), joinForwardedFor(xff, truncated)
	}

	sourceIP = stripPort(lastValue(md, ClientIPMetadataKey))
	// grpc-gateway appends the address of its HTTP peer to the chain it received. Report only what
	// arrived at the API server: that address is sourceIP, or a trusted proxy that resolved to it.
	if n := len(xff); !truncated && n > 0 && (xff[n-1] == sourceIP || isTrustedProxy(xff[n-1], trustedProxies)) {
		xff = xff[:n-1]
	}
	if len(xff) > maxForwardedForEntries {
		xff, truncated = xff[:maxForwardedForEntries], true
	}
	return sourceIP, joinForwardedFor(xff, truncated)
}

// joinForwardedFor renders the chain, keeping the leftmost entries: those name the original client,
// whereas the tail is the hops nearest to us, which peer.address and source.ip already cover.
func joinForwardedFor(xff []string, truncated bool) string {
	joined := strings.Join(xff, ", ")
	if len(joined) > maxForwardedForLen {
		// grpc-gateway does not check this header for valid UTF-8 the way it checks the others, so a
		// byte-boundary cut can split a rune.
		joined, truncated = strings.ToValidUTF8(joined[:maxForwardedForLen], ""), true
	}
	if truncated {
		joined += "..."
	}
	return joined
}

// fromGateway reports whether md carries this process's gateway token. Only the last value counts:
// grpc-gateway forwards Grpc-Metadata-* request headers too, and those are joined ahead of the ones
// its own annotator adds, so a client can put a value of its choosing first.
func fromGateway(md metadata.MD, gatewayToken string) bool {
	if gatewayToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(lastValue(md, GatewayTokenMetadataKey)), []byte(gatewayToken)) == 1
}

func lastValue(md metadata.MD, key string) string {
	v := md.Get(key)
	if len(v) == 0 {
		return ""
	}
	return v[len(v)-1]
}

func peerIPFromContext(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return ""
	}
	return stripPort(p.Addr.String())
}

// stripPort drops the port from an address. Values that are not host:port (a bare IPv6 address, a
// Unix socket path) are returned unchanged.
func stripPort(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}

// SourceIPLoggingOption adds the source IP of the client to every gRPC log line. gatewayToken is the
// secret this process's grpc-gateway identifies itself with; trustedProxies and clientIPHeader are as
// for ResolveClientIP.
func SourceIPLoggingOption(gatewayToken string, trustedProxies []netip.Prefix, clientIPHeader string) logging.Option {
	return logging.WithFieldsFromContext(func(ctx context.Context) logging.Fields {
		sourceIP, forwardedFor := sourceIPFields(ctx, gatewayToken, trustedProxies, clientIPHeader)
		var fields logging.Fields
		if sourceIP != "" {
			fields = append(fields, sourceIPField, sourceIP)
		}
		if forwardedFor != "" {
			fields = append(fields, forwardedForField, forwardedFor)
		}
		return fields
	})
}
