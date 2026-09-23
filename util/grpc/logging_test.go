package grpc

import (
	"bytes"
	"context"
	"fmt"
	"net"
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
			sourceIP, forwardedFor := sourceIPFields(ctx, testGatewayToken)
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

	sourceIP, _ := sourceIPFields(ctx, "")
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

	out := call(t, SourceIPLoggingOption(testGatewayToken))
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
	chained := logging.UnaryServerInterceptor(InterceptorLogger(entry), SourceIPLoggingOption(testGatewayToken))
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
		_, forwardedFor := sourceIPFields(ctx, testGatewayToken)
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
			_, forwardedFor := sourceIPFields(metadata.NewIncomingContext(t.Context(), md), testGatewayToken)
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
		_, got := sourceIPFields(ctx, testGatewayToken)
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
