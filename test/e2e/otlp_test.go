package e2e

import (
	"context"
	"crypto/tls"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

// Must match ARGOCD_OTLP_ADDRESS in the start-e2e-local Makefile target.
const otlpAddress = "127.0.0.1:4327"

type otlpCollector struct {
	collectortrace.UnimplementedTraceServiceServer
	headers chan metadata.MD
}

func (c *otlpCollector) Export(ctx context.Context, _ *collectortrace.ExportTraceServiceRequest) (*collectortrace.ExportTraceServiceResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	select {
	case c.headers <- md:
	default:
	}
	return &collectortrace.ExportTraceServiceResponse{}, nil
}

// TestOTLPExportToAuthenticatedCollector checks the controller exports traces to a
// collector serving a certificate from a private CA, trusted via
// argocd-tls-certs-cm, and sends a header value containing '='.
func TestOTLPExportToAuthenticatedCollector(t *testing.T) {
	if IsRemote() {
		t.Skip("restarting the controller with custom env is only supported locally")
	}

	// Signed by argocd-test-ca, which CustomCACertAdded installs for 127.0.0.1.
	serverCert, err := tls.LoadX509KeyPair("../fixture/certs/argocd-test-server.crt", "../fixture/certs/argocd-test-server.key")
	require.NoError(t, err)
	// start-e2e-local points every component at this address, with sampling off.
	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", otlpAddress)
	require.NoError(t, err)
	srv := grpc.NewServer(grpc.Creds(credentials.NewServerTLSFromCert(&serverCert)))
	collector := &otlpCollector{headers: make(chan metadata.MD, 1)}
	collectortrace.RegisterTraceServiceServer(srv, collector)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	ctx := Given(t).CustomCACertAdded()

	require.NoError(t, RestartProcess(ApplicationControllerProcName, map[string]string{
		// test/container/Procfile passes no --otlp-address, so the controller reads this.
		"ARGOCD_APPLICATION_CONTROLLER_OTLP_ADDRESS":      otlpAddress,
		"ARGOCD_APPLICATION_CONTROLLER_OTLP_SAMPLE_RATIO": "1",
		"ARGOCD_APPLICATION_CONTROLLER_OTLP_INSECURE":     "false",
		"ARGOCD_APPLICATION_CONTROLLER_OTLP_HEADERS":      "x-otlp-token=dG9rZW4=",
	}))

	ctx.
		Path(guestbookPath).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced))

	select {
	case md := <-collector.headers:
		assert.Equal(t, []string{"dG9rZW4="}, md.Get("x-otlp-token"))
	case <-time.After(time.Minute):
		t.Fatal("collector received no traces from the controller")
	}
}
