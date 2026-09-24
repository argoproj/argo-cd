package grpc

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/account"
)

func Test_JSONLogging(t *testing.T) {
	t.Parallel()
	l := logrus.New()
	l.SetFormatter(&logrus.JSONFormatter{})
	var buf bytes.Buffer
	l.SetOutput(&buf)
	entry := logrus.NewEntry(l)

	c := t.Context()
	req := new(account.CreateTokenRequest)
	req.Name = "create-token-name"
	info := &grpc.UnaryServerInfo{}
	handler := func(_ context.Context, _ any) (any, error) {
		return nil, nil
	}
	decider := func(_ context.Context, _ interceptors.CallMeta) bool {
		return true
	}
	interceptor := PayloadUnaryServerInterceptor(entry, false, decider)
	_, err := interceptor(c, req, info, handler)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, fmt.Sprintf(`"grpc.request.content":{"name":%q`, req.Name))
}

func Test_logRequest(t *testing.T) {
	t.Parallel()
	c := t.Context()
	//nolint:staticcheck
	c = context.WithValue(c, "claims", jwt.MapClaims{"groups": []string{"expected-group-claim"}})
	req := new(account.CreateTokenRequest)
	req.Name = "create-token-name"
	info := &grpc.UnaryServerInfo{}
	handler := func(_ context.Context, _ any) (any, error) {
		return nil, nil
	}
	decider := func(_ context.Context, _ interceptors.CallMeta) bool {
		return true
	}

	t.Run("with debug enabled, group claims are logged", func(t *testing.T) {
		t.Parallel()
		l := logrus.New()
		l.SetFormatter(&logrus.JSONFormatter{})
		var buf bytes.Buffer
		l.SetOutput(&buf)
		l.SetLevel(logrus.DebugLevel)
		entry := logrus.NewEntry(l)

		interceptor := PayloadUnaryServerInterceptor(entry, true, decider)

		_, err := interceptor(c, req, info, handler)
		require.NoError(t, err)

		out := buf.String()
		assert.Contains(t, out, "expected-group-claim")
	})

	t.Run("with debug not enabled, group claims aren't logged", func(t *testing.T) {
		t.Parallel()
		l := logrus.New()
		l.SetFormatter(&logrus.JSONFormatter{})
		var buf bytes.Buffer
		l.SetOutput(&buf)
		l.SetLevel(logrus.InfoLevel)
		entry := logrus.NewEntry(l)

		interceptor := PayloadUnaryServerInterceptor(entry, true, decider)

		_, err := interceptor(c, req, info, handler)
		require.NoError(t, err)

		out := buf.String()
		assert.NotContains(t, out, "expected-group-claim")
	})
}

const testGatewayToken = "GATEWAYTOKEN000000000000AB"

func Test_sourceIPFields(t *testing.T) {
	t.Parallel()
	// The gateway dials the API server's own listener, so it looks like any other loopback client.
	gatewayPeer := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 41234}
	gatewayMD := func(clientIP string, xff ...string) metadata.MD {
		md := metadata.MD{}
		for _, v := range xff {
			md.Append("x-forwarded-for", v)
		}
		md.Append(GatewayTokenMetadataKey, testGatewayToken)
		md.Append(ClientIPMetadataKey, clientIP)
		return md
	}

	tests := []struct {
		name         string
		md           metadata.MD
		peerAddr     net.Addr
		sourceIP     string
		forwardedFor string
	}{
		{
			name: "no metadata and no peer",
		},
		{
			name:     "direct gRPC client",
			peerAddr: &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 4242},
			sourceIP: "10.0.0.1",
		},
		{
			name:     "peer without a port",
			peerAddr: &net.UnixAddr{Name: "/tmp/argocd.sock", Net: "unix"},
			sourceIP: "/tmp/argocd.sock",
		},
		{
			name:     "REST request with no proxy in front",
			md:       gatewayMD("203.0.113.9", "203.0.113.9"),
			peerAddr: gatewayPeer,
			sourceIP: "203.0.113.9",
		},
		{
			name:         "REST request behind a proxy",
			md:           gatewayMD("10.0.0.5", "203.0.113.9, 10.0.0.5"),
			peerAddr:     gatewayPeer,
			sourceIP:     "10.0.0.5",
			forwardedFor: "203.0.113.9",
		},
		{
			// The chain is caller-controlled, but the gateway tells us what it saw.
			name:         "REST request with a spoofed chain",
			md:           gatewayMD("10.0.0.5", "9.9.9.9, 8.8.8.8, 10.0.0.5"),
			peerAddr:     gatewayPeer,
			sourceIP:     "10.0.0.5",
			forwardedFor: "9.9.9.9, 8.8.8.8",
		},
		{
			// An Istio sidecar delivers inbound traffic from 127.0.0.6 and passes X-Forwarded-For
			// through unchanged, so a loopback peer is no proof that the gateway relayed the call.
			name:         "in-mesh client behind a sidecar cannot forge the source",
			md:           metadata.Pairs("x-forwarded-for", "9.9.9.9"),
			peerAddr:     &net.TCPAddr{IP: net.ParseIP("127.0.0.6"), Port: 4242},
			sourceIP:     "127.0.0.6",
			forwardedFor: "9.9.9.9",
		},
		{
			// grpc-gateway forwards Grpc-Metadata-* headers ahead of its annotator's values, so a
			// client-supplied token must not win over the real one.
			name: "client-supplied token ahead of the gateway's is ignored",
			md: func() metadata.MD {
				md := metadata.MD{}
				md.Append(GatewayTokenMetadataKey, "not-the-token")
				md.Append(ClientIPMetadataKey, "9.9.9.9")
				return md
			}(),
			peerAddr: &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 4242},
			sourceIP: "10.0.0.1",
		},
		{
			name: "client cannot prepend its own token to the gateway's",
			md: func() metadata.MD {
				md := metadata.MD{}
				md.Append(GatewayTokenMetadataKey, "not-the-token")
				md.Append(ClientIPMetadataKey, "9.9.9.9")
				md.Append(GatewayTokenMetadataKey, testGatewayToken)
				md.Append(ClientIPMetadataKey, "203.0.113.9")
				return md
			}(),
			peerAddr: gatewayPeer,
			sourceIP: "203.0.113.9",
		},
		{
			// A caller's Grpc-Metadata-X-Forwarded-For is forwarded ahead of the chain grpc-gateway
			// assembles, so prepended values must not displace the real one.
			name: "client-prepended chain is ignored",
			md: func() metadata.MD {
				md := metadata.MD{}
				md.Append("x-forwarded-for", "9.9.9.9, 8.8.8.8")
				md.Append("x-forwarded-for", "203.0.113.9, 10.0.0.5")
				md.Append(GatewayTokenMetadataKey, testGatewayToken)
				md.Append(ClientIPMetadataKey, "10.0.0.5")
				return md
			}(),
			peerAddr:     gatewayPeer,
			sourceIP:     "10.0.0.5",
			forwardedFor: "203.0.113.9",
		},
		{
			// grpc-web and native gRPC deliver each X-Forwarded-For header line as its own value.
			name:         "chain split across header lines keeps the original client",
			md:           metadata.MD{"x-forwarded-for": {"203.0.113.7", "10.0.0.5"}},
			peerAddr:     &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 4242},
			sourceIP:     "10.0.0.1",
			forwardedFor: "203.0.113.7, 10.0.0.5",
		},
		{
			// Azure Application Gateway appends client-ip:port rather than a bare address.
			name:     "address carrying a port",
			md:       gatewayMD("1.2.3.4:56789"),
			peerAddr: gatewayPeer,
			sourceIP: "1.2.3.4",
		},
		{
			name:     "empty forwarded-for falls back to the peer",
			md:       metadata.Pairs("x-forwarded-for", ""),
			peerAddr: &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 4242},
			sourceIP: "10.0.0.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			if tt.md != nil {
				ctx = metadata.NewIncomingContext(ctx, tt.md)
			}
			if tt.peerAddr != nil {
				ctx = peer.NewContext(ctx, &peer.Peer{Addr: tt.peerAddr})
			}
			sourceIP, forwardedFor := sourceIPFields(ctx, testGatewayToken, nil, "")
			assert.Equal(t, tt.sourceIP, sourceIP)
			assert.Equal(t, tt.forwardedFor, forwardedFor)
		})
	}
}

func Test_sourceIPFields_noGatewayToken(t *testing.T) {
	t.Parallel()
	md := metadata.Pairs(GatewayTokenMetadataKey, "", ClientIPMetadataKey, "9.9.9.9")
	ctx := metadata.NewIncomingContext(t.Context(), md)
	ctx = peer.NewContext(ctx, &peer.Peer{Addr: &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 4242}})

	sourceIP, _ := sourceIPFields(ctx, "", nil, "")
	assert.Equal(t, "10.0.0.1", sourceIP, "an empty token must never match")
}

func Test_SourceIPLogging(t *testing.T) {
	t.Parallel()
	newEntry := func(buf *bytes.Buffer) *logrus.Entry {
		l := logrus.New()
		l.SetFormatter(&logrus.JSONFormatter{})
		l.SetOutput(buf)
		return logrus.NewEntry(l)
	}
	call := func(t *testing.T, opts ...logging.Option) string {
		t.Helper()
		var buf bytes.Buffer
		md := metadata.Pairs("x-forwarded-for", "1.2.3.4, 10.0.0.5",
			GatewayTokenMetadataKey, testGatewayToken, ClientIPMetadataKey, "10.0.0.5")
		ctx := metadata.NewIncomingContext(t.Context(), md)
		ctx = peer.NewContext(ctx, &peer.Peer{Addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 4242}})
		interceptor := logging.UnaryServerInterceptor(InterceptorLogger(newEntry(&buf)), opts...)
		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/account.AccountService/CreateToken"}, func(_ context.Context, _ any) (any, error) {
			return nil, nil
		})
		require.NoError(t, err)
		return buf.String()
	}

	assert.NotContains(t, call(t), "source.ip")

	out := call(t, SourceIPLoggingOption(testGatewayToken, nil, ""))
	assert.Contains(t, out, `"source.ip":"10.0.0.5"`)
	assert.Contains(t, out, `"forwarded.for":"1.2.3.4"`)
}

func Test_SourceIPOnPayloadLog(t *testing.T) {
	t.Parallel()
	l := logrus.New()
	l.SetFormatter(&logrus.JSONFormatter{})
	var buf bytes.Buffer
	l.SetOutput(&buf)
	entry := logrus.NewEntry(l)

	//nolint:staticcheck
	ctx := context.WithValue(t.Context(), "claims", jwt.MapClaims{"sub": "alice"})
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
		"x-forwarded-for", "203.0.113.9",
		GatewayTokenMetadataKey, testGatewayToken,
		ClientIPMetadataKey, "203.0.113.9",
	))

	req := new(account.CreateTokenRequest)
	handler := func(_ context.Context, _ any) (any, error) { return nil, nil }
	decider := func(_ context.Context, _ interceptors.CallMeta) bool { return true }

	// The logging interceptor injects the fields; the payload interceptor is the one logging claims.
	chained := logging.UnaryServerInterceptor(InterceptorLogger(entry), SourceIPLoggingOption(testGatewayToken, nil, ""))
	inner := PayloadUnaryServerInterceptor(entry, true, decider)
	_, err := chained(ctx, req, &grpc.UnaryServerInfo{FullMethod: "/account.AccountService/CreateToken"},
		func(ctx context.Context, r any) (any, error) {
			return inner(ctx, r, &grpc.UnaryServerInfo{FullMethod: "/account.AccountService/CreateToken"}, handler)
		})
	require.NoError(t, err)

	var payloadLine string
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if strings.Contains(line, "grpc.request.claims") {
			payloadLine = line
		}
	}
	require.NotEmpty(t, payloadLine, "expected a payload log line carrying the claims")
	assert.Contains(t, payloadLine, `"source.ip":"203.0.113.9"`, "claims and source IP must land on the same line")
}

func Test_forwardedForCaps(t *testing.T) {
	t.Parallel()
	// Feed the chain in the way a request actually carries it, so the parse bound is exercised too.
	fieldFor := func(t *testing.T, chain string) string {
		t.Helper()
		ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("x-forwarded-for", chain))
		ctx = peer.NewContext(ctx, &peer.Peer{Addr: &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 4242}})
		_, forwardedFor := sourceIPFields(ctx, testGatewayToken, nil, "")
		return forwardedFor
	}

	t.Run("short chains are untouched", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "1.2.3.4, 10.0.0.5", fieldFor(t, "1.2.3.4, 10.0.0.5"))
		assert.Empty(t, fieldFor(t, ""))
	})

	t.Run("too many entries keeps the leftmost", func(t *testing.T) {
		t.Parallel()
		entries := make([]string, 100)
		for i := range entries {
			entries[i] = fmt.Sprintf("10.0.0.%d", i%256)
		}
		got := fieldFor(t, strings.Join(entries, ", "))
		assert.True(t, strings.HasPrefix(got, "10.0.0.0, 10.0.0.1, "), "the original client must survive, got %q", got)
		assert.True(t, strings.HasSuffix(got, "..."), "truncation must be visible, got %q", got)
		assert.LessOrEqual(t, len(got), maxForwardedForLen+3)
		assert.Equal(t, maxForwardedForEntries, strings.Count(got, ",")+1, "no more than the entry cap is parsed")
	})

	t.Run("the gateway's appended address does not count toward the cap", func(t *testing.T) {
		t.Parallel()
		gatewayFieldFor := func(n int) string {
			entries := make([]string, n)
			for i := range entries {
				entries[i] = fmt.Sprintf("10.0.0.%d", i)
			}
			md := metadata.Pairs(
				"x-forwarded-for", strings.Join(entries, ", ")+", 198.51.100.1",
				GatewayTokenMetadataKey, testGatewayToken,
				ClientIPMetadataKey, "198.51.100.1",
			)
			_, forwardedFor := sourceIPFields(metadata.NewIncomingContext(t.Context(), md), testGatewayToken, nil, "")
			return forwardedFor
		}

		got := gatewayFieldFor(maxForwardedForEntries)
		assert.False(t, strings.HasSuffix(got, "..."), "a chain at the cap is complete, got %q", got)
		assert.Equal(t, maxForwardedForEntries, strings.Count(got, ",")+1)

		got = gatewayFieldFor(maxForwardedForEntries + 1)
		assert.True(t, strings.HasSuffix(got, "..."), "a chain over the cap is truncated, got %q", got)
		assert.NotContains(t, got, "198.51.100.1")
	})

	t.Run("the cap spans header lines", func(t *testing.T) {
		t.Parallel()
		values := make([]string, 100)
		for i := range values {
			values[i] = "9.9.9.9"
		}
		ctx := metadata.NewIncomingContext(t.Context(), metadata.MD{"x-forwarded-for": values})
		_, got := sourceIPFields(ctx, testGatewayToken, nil, "")
		assert.True(t, strings.HasSuffix(got, "..."), "got %q", got)
		assert.Equal(t, maxForwardedForEntries, strings.Count(got, ",")+1)
	})

	t.Run("a single oversized entry is cut", func(t *testing.T) {
		t.Parallel()
		got := fieldFor(t, strings.Repeat("a", 4096))
		assert.Len(t, got, maxForwardedForLen+3)
		assert.True(t, strings.HasSuffix(got, "..."))
	})

	t.Run("truncation does not split a rune", func(t *testing.T) {
		t.Parallel()
		// 512 is not a multiple of 3, so a byte-boundary cut lands inside one of these.
		got := fieldFor(t, strings.Repeat("\u20ac", 1000))
		assert.True(t, utf8.ValidString(got), "truncated chain must stay valid UTF-8, got %q", got)
		assert.True(t, strings.HasSuffix(got, "..."))
	})

	t.Run("an oversized header cannot inflate the log line", func(t *testing.T) {
		t.Parallel()
		// net/http accepts ~1MB of headers, and this runs before authentication.
		got := fieldFor(t, strings.Repeat("9.9.9.9, ", 100_000))
		assert.LessOrEqual(t, len(got), maxForwardedForLen+3)
	})
}

func Test_ParseTrustedProxies(t *testing.T) {
	t.Parallel()
	got, err := ParseTrustedProxies([]string{"10.0.0.0/8", " 192.0.2.1 ", "", "2001:db8::/32", "::ffff:198.51.100.7", "10.1.2.3/16"})
	require.NoError(t, err)
	assert.Equal(t, []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("192.0.2.1/32"),
		netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParsePrefix("198.51.100.7/32"),
		netip.MustParsePrefix("10.1.0.0/16"),
	}, got)

	_, err = ParseTrustedProxies([]string{"10.0.0.0/8", "not-a-cidr"})
	require.ErrorContains(t, err, `"not-a-cidr"`)
}

func Test_ResolveClientIP(t *testing.T) {
	t.Parallel()
	trusted, err := ParseTrustedProxies([]string{"10.0.0.0/8", "127.0.0.0/8"})
	require.NoError(t, err)

	tests := []struct {
		name     string
		remote   string
		headerIP string
		xff      []string
		trusted  []netip.Prefix
		want     string
	}{
		{name: "no trusted proxies", remote: "10.0.0.1:443", xff: []string{"203.0.113.9"}, want: "10.0.0.1"},
		{name: "untrusted peer ignores both headers", remote: "198.51.100.1:443", headerIP: "9.9.9.9", xff: []string{"8.8.8.8"}, trusted: trusted, want: "198.51.100.1"},
		{name: "trusted peer without headers", remote: "10.0.0.1:443", trusted: trusted, want: "10.0.0.1"},
		{name: "client IP header wins", remote: "10.0.0.1:443", headerIP: "203.0.113.9", xff: []string{"8.8.8.8"}, trusted: trusted, want: "203.0.113.9"},
		{name: "invalid client IP header falls back to the chain", remote: "10.0.0.1:443", headerIP: "unknown", xff: []string{"203.0.113.9"}, trusted: trusted, want: "203.0.113.9"},
		{name: "rightmost untrusted entry", remote: "10.0.0.1:443", xff: []string{"9.9.9.9, 203.0.113.9, 10.0.0.2"}, trusted: trusted, want: "203.0.113.9"},
		{name: "chain across header lines", remote: "10.0.0.1:443", xff: []string{"9.9.9.9", "203.0.113.9", "10.0.0.2"}, trusted: trusted, want: "203.0.113.9"},
		{name: "entry with a port", remote: "10.0.0.1:443", xff: []string{"203.0.113.9:5555"}, trusted: trusted, want: "203.0.113.9"},
		{name: "IPv4-mapped entries are unmapped", remote: "[::ffff:10.0.0.1]:443", xff: []string{"::ffff:203.0.113.9"}, trusted: trusted, want: "203.0.113.9"},
		{name: "all trusted resolves to the leftmost", remote: "127.0.0.6:443", xff: []string{"10.0.0.3, 10.0.0.2"}, trusted: trusted, want: "10.0.0.3"},
		{name: "invalid entry stops the walk", remote: "10.0.0.1:443", xff: []string{"203.0.113.9, garbage, 10.0.0.2"}, trusted: trusted, want: "10.0.0.2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ResolveClientIP(tt.remote, tt.headerIP, tt.xff, tt.trusted))
		})
	}

	t.Run("the walk is bounded", func(t *testing.T) {
		t.Parallel()
		// A client past the cap cannot be reached through trusted hops, however many there are.
		chain := "203.0.113.9, " + strings.Repeat("10.0.0.2, ", 100) + "10.0.0.2"
		assert.Equal(t, "10.0.0.2", ResolveClientIP("10.0.0.1:443", "", []string{chain}, trusted))
		assert.Equal(t, "10.0.0.1", ResolveClientIP("10.0.0.1:443", "", []string{strings.Repeat(",", 100_000)}, trusted))
	})
}

func Test_HTTPClientIP(t *testing.T) {
	t.Parallel()
	trusted, err := ParseTrustedProxies([]string{"10.0.0.0/8"})
	require.NoError(t, err)
	req := func(remote string, header http.Header) *http.Request {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/applications", http.NoBody)
		r.RemoteAddr = remote
		r.Header = header
		return r
	}

	assert.Equal(t, "10.0.0.1", HTTPClientIP(req("10.0.0.1:443", http.Header{"X-Forwarded-For": {"203.0.113.9"}}), nil, ""))
	assert.Equal(t, "203.0.113.9", HTTPClientIP(req("10.0.0.1:443", http.Header{"X-Forwarded-For": {"9.9.9.9", "203.0.113.9"}}), trusted, ""))
	assert.Equal(t, "192.0.2.4", HTTPClientIP(req("10.0.0.1:443", http.Header{
		"X-Forwarded-For":  {"203.0.113.9"},
		"Cf-Connecting-Ip": {"192.0.2.4"},
	}), trusted, "CF-Connecting-IP"))
	assert.Equal(t, "198.51.100.1", HTTPClientIP(req("198.51.100.1:443", http.Header{"Cf-Connecting-Ip": {"192.0.2.4"}}), trusted, "CF-Connecting-IP"))
}

func Test_sourceIPFields_trustedProxies(t *testing.T) {
	t.Parallel()
	trusted, err := ParseTrustedProxies([]string{"127.0.0.0/8", "10.0.0.0/8"})
	require.NoError(t, err)
	fields := func(t *testing.T, md metadata.MD, peerIP string) (string, string) {
		t.Helper()
		ctx := metadata.NewIncomingContext(t.Context(), md)
		ctx = peer.NewContext(ctx, &peer.Peer{Addr: &net.TCPAddr{IP: net.ParseIP(peerIP), Port: 4242}})
		return sourceIPFields(ctx, testGatewayToken, trusted, "X-Real-IP")
	}

	t.Run("gRPC through a trusted sidecar", func(t *testing.T) {
		t.Parallel()
		sourceIP, forwardedFor := fields(t, metadata.Pairs("x-forwarded-for", "203.0.113.9, 10.0.0.5"), "127.0.0.6")
		assert.Equal(t, "203.0.113.9", sourceIP)
		assert.Equal(t, "203.0.113.9, 10.0.0.5", forwardedFor)
	})

	t.Run("gRPC with a client IP header from a trusted proxy", func(t *testing.T) {
		t.Parallel()
		sourceIP, _ := fields(t, metadata.Pairs("x-real-ip", "192.0.2.4"), "10.0.0.5")
		assert.Equal(t, "192.0.2.4", sourceIP)
	})

	t.Run("gRPC from an untrusted peer", func(t *testing.T) {
		t.Parallel()
		sourceIP, _ := fields(t, metadata.Pairs("x-real-ip", "192.0.2.4", "x-forwarded-for", "9.9.9.9"), "198.51.100.1")
		assert.Equal(t, "198.51.100.1", sourceIP)
	})

	t.Run("gateway drops the trusted proxy it appended", func(t *testing.T) {
		t.Parallel()
		// The gateway resolved past its peer, 10.0.0.5, which it still appends to the chain.
		md := metadata.Pairs(
			"x-forwarded-for", "203.0.113.9, 10.0.0.5",
			GatewayTokenMetadataKey, testGatewayToken,
			ClientIPMetadataKey, "203.0.113.9",
		)
		sourceIP, forwardedFor := fields(t, md, "127.0.0.1")
		assert.Equal(t, "203.0.113.9", sourceIP)
		assert.Equal(t, "203.0.113.9", forwardedFor)
	})
}
