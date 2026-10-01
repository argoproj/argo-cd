package trace

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

type fakeTraceCollector struct {
	collectortrace.UnimplementedTraceServiceServer
	headers chan metadata.MD
}

func (c *fakeTraceCollector) Export(ctx context.Context, _ *collectortrace.ExportTraceServiceRequest) (*collectortrace.ExportTraceServiceResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	c.headers <- md
	return &collectortrace.ExportTraceServiceResponse{}, nil
}

// selfSignedCert returns a server certificate for 127.0.0.1 that is its own
// CA, and the path of that CA in PEM.
func selfSignedCert(t *testing.T) (tls.Certificate, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "collector"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	caPath := filepath.Join(t.TempDir(), "ca.crt")
	require.NoError(t, os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, caPath
}

// startTLSCollector serves a fake OTLP trace collector with a self-signed
// certificate and returns its address and the path of the CA to trust.
func startTLSCollector(t *testing.T) (string, string, *fakeTraceCollector) {
	t.Helper()
	serverCert, caPath := selfSignedCert(t)
	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer(grpc.Creds(credentials.NewServerTLSFromCert(&serverCert)))
	collector := &fakeTraceCollector{headers: make(chan metadata.MD, 10)}
	collectortrace.RegisterTraceServiceServer(srv, collector)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().String(), caPath, collector
}

func exportSpan(t *testing.T, addr string, headers map[string]string) {
	t.Helper()
	closer, err := InitTracer(t.Context(), "test", addr, false, headers, nil, 1.0)
	require.NoError(t, err)
	t.Cleanup(closer)
	_, span := otel.Tracer("test").Start(t.Context(), "span")
	span.End()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, otel.GetTracerProvider().(*sdktrace.TracerProvider).ForceFlush(ctx))
}

func TestInitTracer_HonorsStandardOTLPEnv(t *testing.T) {
	addr, caPath, collector := startTLSCollector(t)
	t.Setenv("OTEL_EXPORTER_OTLP_CERTIFICATE", caPath)
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "authorization=Bearer from-env")

	exportSpan(t, addr, nil)

	select {
	case md := <-collector.headers:
		assert.Equal(t, []string{"Bearer from-env"}, md.Get("authorization"))
	default:
		t.Fatal("collector received no export")
	}
}

func TestInitTracer_ExplicitHeadersOverrideEnv(t *testing.T) {
	addr, caPath, collector := startTLSCollector(t)
	t.Setenv("OTEL_EXPORTER_OTLP_CERTIFICATE", caPath)
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "authorization=Bearer from-env")

	exportSpan(t, addr, map[string]string{"authorization": "Bearer from-flag"})

	select {
	case md := <-collector.headers:
		assert.Equal(t, []string{"Bearer from-flag"}, md.Get("authorization"))
	default:
		t.Fatal("collector received no export")
	}
}

func TestInitTracer_UsesCAFromTLSCertsConfigMap(t *testing.T) {
	// None of these override the ConfigMap CA for traces: the SDK ignores
	// empty values and other signals' variables.
	for name, env := range map[string]map[string]string{
		"no env vars":         {},
		"empty certificate":   {"OTEL_EXPORTER_OTLP_CERTIFICATE": " "},
		"metrics certificate": {"OTEL_EXPORTER_OTLP_METRICS_CERTIFICATE": "/nonexistent/ca.crt"},
	} {
		t.Run(name, func(t *testing.T) {
			addr, caPath, collector := startTLSCollector(t)
			// argocd-tls-certs-cm is mounted as one file per hostname.
			tlsDataPath := t.TempDir()
			ca, err := os.ReadFile(caPath)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(tlsDataPath, "127.0.0.1"), ca, 0o600))
			t.Setenv("ARGOCD_TLS_DATA_PATH", tlsDataPath)
			for k, v := range env {
				t.Setenv(k, v)
			}

			exportSpan(t, addr, nil)

			select {
			case <-collector.headers:
			default:
				t.Fatal("collector received no export")
			}
		})
	}
}

// clientCertFiles writes a self-signed client certificate and key, and returns
// their paths and a pool for the collector to verify them with.
func clientCertFiles(t *testing.T) (string, string, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "argocd"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "client.crt"), filepath.Join(dir, "client.key")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600))
	parsed, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(parsed)
	return certPath, keyPath, pool
}

// tryExportSpan exports one span and reports whether the collector received it.
func tryExportSpan(t *testing.T, addr string, insecure bool, received <-chan metadata.MD) bool {
	t.Helper()
	closer, err := InitTracer(t.Context(), "test", addr, insecure, nil, nil, 1.0)
	require.NoError(t, err)
	t.Cleanup(closer)
	_, span := otel.Tracer("test").Start(t.Context(), "span")
	span.End()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	_ = otel.GetTracerProvider().(*sdktrace.TracerProvider).ForceFlush(ctx)
	select {
	case <-received:
		return true
	default:
		return false
	}
}

func TestInitTracer_SecureIgnoresInsecureEnv(t *testing.T) {
	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer() // plaintext
	collector := &fakeTraceCollector{headers: make(chan metadata.MD, 10)}
	collectortrace.RegisterTraceServiceServer(srv, collector)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")

	assert.False(t, tryExportSpan(t, lis.Addr().String(), false, collector.headers),
		"otlp.insecure=false must not be downgraded to plaintext by OTEL_EXPORTER_OTLP_INSECURE")
}

func TestInitTracer_MutualTLS(t *testing.T) {
	for name, tc := range map[string]struct {
		prefix          string
		clientCert      bool
		caFromConfigMap bool
		mixed           bool // signal-specific cert with a generic key
		want            bool
	}{
		"generic vars":   {prefix: "OTEL_EXPORTER_OTLP_", clientCert: true, want: true},
		"traces vars":    {prefix: "OTEL_EXPORTER_OTLP_TRACES_", clientCert: true, want: true},
		"mixed vars":     {prefix: "OTEL_EXPORTER_OTLP_", clientCert: true, mixed: true, want: true},
		"no client cert": {prefix: "OTEL_EXPORTER_OTLP_", clientCert: false, want: false},
		"configmap CA":   {prefix: "OTEL_EXPORTER_OTLP_", clientCert: true, caFromConfigMap: true, want: true},
	} {
		t.Run(name, func(t *testing.T) {
			serverCert, caPath := selfSignedCert(t)
			clientCertPath, clientKeyPath, clientCAs := clientCertFiles(t)
			lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
			require.NoError(t, err)
			srv := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{
				Certificates: []tls.Certificate{serverCert},
				ClientAuth:   tls.RequireAndVerifyClientCert,
				ClientCAs:    clientCAs,
			})))
			collector := &fakeTraceCollector{headers: make(chan metadata.MD, 10)}
			collectortrace.RegisterTraceServiceServer(srv, collector)
			go func() { _ = srv.Serve(lis) }()
			t.Cleanup(srv.Stop)

			if tc.caFromConfigMap {
				tlsDataPath := t.TempDir()
				ca, err := os.ReadFile(caPath)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(filepath.Join(tlsDataPath, "127.0.0.1"), ca, 0o600))
				t.Setenv("ARGOCD_TLS_DATA_PATH", tlsDataPath)
			} else {
				t.Setenv(tc.prefix+"CERTIFICATE", caPath)
			}
			switch {
			case tc.mixed:
				t.Setenv("OTEL_EXPORTER_OTLP_TRACES_CLIENT_CERTIFICATE", clientCertPath)
				t.Setenv("OTEL_EXPORTER_OTLP_CLIENT_KEY", clientKeyPath)
			case tc.clientCert:
				t.Setenv(tc.prefix+"CLIENT_CERTIFICATE", clientCertPath)
				t.Setenv(tc.prefix+"CLIENT_KEY", clientKeyPath)
			}

			assert.Equal(t, tc.want, tryExportSpan(t, lis.Addr().String(), false, collector.headers))
		})
	}
}

func TestInitTracer_InvalidTLSEnvFails(t *testing.T) {
	clientCertPath, clientKeyPath, _ := clientCertFiles(t)
	notPEM := filepath.Join(t.TempDir(), "ca.crt")
	require.NoError(t, os.WriteFile(notPEM, []byte("not a certificate"), 0o600))

	for name, env := range map[string]map[string]string{
		"client cert without key":  {"OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE": clientCertPath},
		"client key without cert":  {"OTEL_EXPORTER_OTLP_TRACES_CLIENT_KEY": clientKeyPath},
		"mismatched client pair":   {"OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE": clientCertPath, "OTEL_EXPORTER_OTLP_CLIENT_KEY": clientCertPath},
		"unreadable CA":            {"OTEL_EXPORTER_OTLP_CERTIFICATE": "/nonexistent/ca.crt"},
		"CA without a certificate": {"OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE": notPEM},
	} {
		t.Run(name, func(t *testing.T) {
			// Without credentials, this would let the exporter fall back to plaintext.
			t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
			for k, v := range env {
				t.Setenv(k, v)
			}
			_, err := InitTracer(t.Context(), "test", "127.0.0.1:4317", false, nil, nil, 1.0)
			assert.Error(t, err)
		})
	}
}
