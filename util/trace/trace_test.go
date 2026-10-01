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

type fakeCollector struct {
	collectortrace.UnimplementedTraceServiceServer
	headers chan metadata.MD
}

func (c *fakeCollector) Export(ctx context.Context, _ *collectortrace.ExportTraceServiceRequest) (*collectortrace.ExportTraceServiceResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	c.headers <- md
	return &collectortrace.ExportTraceServiceResponse{}, nil
}

// startTLSCollector serves a fake OTLP trace collector with a self-signed
// certificate and returns its address and the path of the CA to trust.
func startTLSCollector(t *testing.T) (string, string, *fakeCollector) {
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

	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer(grpc.Creds(credentials.NewServerTLSFromCert(&tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key})))
	collector := &fakeCollector{headers: make(chan metadata.MD, 10)}
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
	addr, caPath, collector := startTLSCollector(t)
	// argocd-tls-certs-cm is mounted as one file per hostname.
	tlsDataPath := t.TempDir()
	ca, err := os.ReadFile(caPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(tlsDataPath, "127.0.0.1"), ca, 0o600))
	t.Setenv("ARGOCD_TLS_DATA_PATH", tlsDataPath)

	exportSpan(t, addr, nil)

	select {
	case <-collector.headers:
	default:
		t.Fatal("collector received no export")
	}
}
