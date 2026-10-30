package trace

import (
	"context"
	"crypto/tls"
	"crypto/x509"
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

	tlsutil "github.com/argoproj/argo-cd/v3/util/tls"
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

// selfSignedCert generates a self-signed certificate for 127.0.0.1, writes it
// and its key as PEM, and returns it with its paths and a pool trusting it.
func selfSignedCert(t *testing.T, usage x509.ExtKeyUsage) (tls.Certificate, string, string, *x509.CertPool) {
	t.Helper()
	cert, err := tlsutil.GenerateX509KeyPair(tlsutil.CertOptions{
		Hosts:        []string{"127.0.0.1"},
		Organization: "argocd",
		IsCA:         true,
		ECDSACurve:   "P256",
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
	})
	require.NoError(t, err)
	certPEM, keyPEM := tlsutil.EncodeX509KeyPair(*cert)
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0o600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0o600))
	pool := x509.NewCertPool()
	pool.AddCert(cert.Leaf)
	return *cert, certPath, keyPath, pool
}

// startCollector serves a fake OTLP trace collector, in plaintext unless opts
// set credentials.
func startCollector(t *testing.T, opts ...grpc.ServerOption) (string, *fakeTraceCollector) {
	t.Helper()
	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer(opts...)
	collector := &fakeTraceCollector{headers: make(chan metadata.MD, 10)}
	collectortrace.RegisterTraceServiceServer(srv, collector)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().String(), collector
}

// startTLSCollector serves a fake OTLP trace collector with a self-signed
// certificate and returns its address and the path of the CA to trust.
func startTLSCollector(t *testing.T) (string, string, *fakeTraceCollector) {
	t.Helper()
	serverCert, caPath, _, _ := selfSignedCert(t, x509.ExtKeyUsageServerAuth)
	addr, collector := startCollector(t, grpc.Creds(credentials.NewServerTLSFromCert(&serverCert)))
	return addr, caPath, collector
}

// trustViaConfigMap trusts caPath for 127.0.0.1 the way a mounted
// argocd-tls-certs-cm does: one file per hostname.
func trustViaConfigMap(t *testing.T, caPath string) {
	t.Helper()
	tlsDataPath := t.TempDir()
	ca, err := os.ReadFile(caPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(tlsDataPath, "127.0.0.1"), ca, 0o600))
	t.Setenv("ARGOCD_TLS_DATA_PATH", tlsDataPath)
}

// exportSpan exports one span and returns the headers the collector received,
// if it received the span.
func exportSpan(t *testing.T, addr string, insecure bool, headers map[string]string, collector *fakeTraceCollector) (metadata.MD, bool) {
	t.Helper()
	closer, err := InitTracer(t.Context(), "test", addr, insecure, headers, nil, 1.0)
	require.NoError(t, err)
	t.Cleanup(closer)
	_, span := otel.Tracer("test").Start(t.Context(), "span")
	span.End()
	// A failed handshake retries until this deadline, so keep it short.
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	_ = otel.GetTracerProvider().(*sdktrace.TracerProvider).ForceFlush(ctx)
	select {
	case md := <-collector.headers:
		return md, true
	default:
		return nil, false
	}
}

func TestInitTracer_Headers(t *testing.T) {
	for name, tc := range map[string]struct {
		headers map[string]string
		want    string
	}{
		"from env":           {want: "Bearer from-env"},
		"flag overrides env": {headers: map[string]string{"authorization": "Bearer from-flag"}, want: "Bearer from-flag"},
	} {
		t.Run(name, func(t *testing.T) {
			addr, caPath, collector := startTLSCollector(t)
			t.Setenv("OTEL_EXPORTER_OTLP_CERTIFICATE", caPath)
			t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "authorization=Bearer from-env")

			md, ok := exportSpan(t, addr, false, tc.headers, collector)
			require.True(t, ok, "collector received no export")
			assert.Equal(t, []string{tc.want}, md.Get("authorization"))
		})
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
			trustViaConfigMap(t, caPath)
			for k, v := range env {
				t.Setenv(k, v)
			}

			_, ok := exportSpan(t, addr, false, nil, collector)
			assert.True(t, ok, "collector received no export")
		})
	}
}

func TestInitTracer_MutualTLS(t *testing.T) {
	for name, tc := range map[string]struct {
		prefix          string
		clientCert      bool
		caFromConfigMap bool
		strayCert       bool // incomplete traces pair next to a complete generic one
		want            bool
	}{
		"generic vars":      {prefix: "OTEL_EXPORTER_OTLP_", clientCert: true, want: true},
		"traces vars":       {prefix: "OTEL_EXPORTER_OTLP_TRACES_", clientCert: true, want: true},
		"stray traces cert": {prefix: "OTEL_EXPORTER_OTLP_", clientCert: true, strayCert: true, want: true},
		"no client cert":    {prefix: "OTEL_EXPORTER_OTLP_", clientCert: false, want: false},
		"configmap CA":      {prefix: "OTEL_EXPORTER_OTLP_", clientCert: true, caFromConfigMap: true, want: true},
	} {
		t.Run(name, func(t *testing.T) {
			serverCert, caPath, _, _ := selfSignedCert(t, x509.ExtKeyUsageServerAuth)
			_, clientCertPath, clientKeyPath, clientCAs := selfSignedCert(t, x509.ExtKeyUsageClientAuth)
			addr, collector := startCollector(t, grpc.Creds(credentials.NewTLS(&tls.Config{
				Certificates: []tls.Certificate{serverCert},
				ClientAuth:   tls.RequireAndVerifyClientCert,
				ClientCAs:    clientCAs,
			})))

			if tc.caFromConfigMap {
				trustViaConfigMap(t, caPath)
			} else {
				t.Setenv(tc.prefix+"CERTIFICATE", caPath)
			}
			if tc.clientCert {
				t.Setenv(tc.prefix+"CLIENT_CERTIFICATE", clientCertPath)
				t.Setenv(tc.prefix+"CLIENT_KEY", clientKeyPath)
			}
			if tc.strayCert {
				// Ignored, as in the SDK: the traces pair is incomplete.
				t.Setenv("OTEL_EXPORTER_OTLP_TRACES_CLIENT_CERTIFICATE", "/nonexistent/client.crt")
			}

			_, ok := exportSpan(t, addr, false, nil, collector)
			assert.Equal(t, tc.want, ok)
		})
	}
}

func TestInitTracer_InvalidTLSEnvFails(t *testing.T) {
	_, clientCertPath, clientKeyPath, _ := selfSignedCert(t, x509.ExtKeyUsageClientAuth)
	notPEM := filepath.Join(t.TempDir(), "ca.crt")
	require.NoError(t, os.WriteFile(notPEM, []byte("not a certificate"), 0o600))

	for name, env := range map[string]map[string]string{
		"client cert without key":      {"OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE": clientCertPath},
		"client key without cert":      {"OTEL_EXPORTER_OTLP_TRACES_CLIENT_KEY": clientKeyPath},
		"traces cert with generic key": {"OTEL_EXPORTER_OTLP_TRACES_CLIENT_CERTIFICATE": clientCertPath, "OTEL_EXPORTER_OTLP_CLIENT_KEY": clientKeyPath},
		"mismatched client pair":       {"OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE": clientCertPath, "OTEL_EXPORTER_OTLP_CLIENT_KEY": clientCertPath},
		"unreadable CA":                {"OTEL_EXPORTER_OTLP_CERTIFICATE": "/nonexistent/ca.crt"},
		"CA without a certificate":     {"OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE": notPEM},
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

// otlp.insecure decides plaintext vs TLS against a plaintext collector,
// whatever the OTEL_EXPORTER_OTLP_* env vars say.
func TestInitTracer_InsecureFlagWinsOverEnv(t *testing.T) {
	_, caPath, _, _ := selfSignedCert(t, x509.ExtKeyUsageServerAuth)
	for name, tc := range map[string]struct {
		insecure bool
		env      string
		value    string
	}{
		"secure ignores OTEL_EXPORTER_OTLP_INSECURE":      {insecure: false, env: "OTEL_EXPORTER_OTLP_INSECURE", value: "true"},
		"insecure ignores OTEL_EXPORTER_OTLP_CERTIFICATE": {insecure: true, env: "OTEL_EXPORTER_OTLP_CERTIFICATE", value: caPath},
	} {
		t.Run(name, func(t *testing.T) {
			addr, collector := startCollector(t)
			t.Setenv(tc.env, tc.value)

			_, ok := exportSpan(t, addr, tc.insecure, nil, collector)
			assert.Equal(t, tc.insecure, ok)
		})
	}
}
