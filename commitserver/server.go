package commitserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v3/commitserver/commit"
	"github.com/argoproj/argo-cd/v3/commitserver/metrics"
	versionpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/v3/server/version"
	"github.com/argoproj/argo-cd/v3/util/configbus"
	"github.com/argoproj/argo-cd/v3/util/errors"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/healthz"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

// ArgoCDCommitServer is the server that handles commit requests.
type ArgoCDCommitServer struct {
	commitService  *commit.Service
	Config         Config
	configProvider configbus.Provider
}

// NewServer returns a new instance of the commit server.
func NewServer(cfg Config, gitCredsStore git.CredsStore, metricsServer *metrics.Server) *ArgoCDCommitServer {
	a := &ArgoCDCommitServer{
		commitService: commit.NewService(gitCredsStore, metricsServer),
		Config:        cfg,
	}
	a.InitConfigProvider()
	return a
}

// CreateGRPC creates a new gRPC server.
func (a *ArgoCDCommitServer) CreateGRPC() *grpc.Server {
	server := grpc.NewServer(grpc.MaxRecvMsgSize(apiclient.MaxGRPCMessageSize))
	versionpkg.RegisterVersionServiceServer(server, version.NewServer(nil, &configbus.StaticProvider{Fields: configbus.StaticFields{
		DisableAuth: configbus.Ptr(true),
	}}))
	apiclient.RegisterCommitServiceServer(server, a.commitService)

	healthService := health.NewServer()
	grpc_health_v1.RegisterHealthServer(server, healthService)

	return server
}

// Run starts the metrics listener and gRPC server using Config/Legacy listen
// addresses, then blocks until graceful shutdown. Callers must register the
// metrics handler on http.DefaultServeMux before calling Run.
func (a *ArgoCDCommitServer) Run(ctx context.Context) error {
	metricsHost, err := a.configProvider.CommitserverMetricsListenAddress(ctx)
	errors.CheckError(err)
	metricsPort, err := a.configProvider.CommitserverMetricsPort(ctx)
	errors.CheckError(err)
	listenHost, err := a.configProvider.CommitserverListenAddress(ctx)
	errors.CheckError(err)
	listenPort, err := a.configProvider.CommitserverListenPort(ctx)
	errors.CheckError(err)

	go func() {
		errors.CheckError(http.ListenAndServe(fmt.Sprintf("%s:%d", metricsHost, metricsPort), nil))
	}()

	grpcServer := a.CreateGRPC()

	lc := &net.ListenConfig{}
	listener, err := lc.Listen(ctx, "tcp", fmt.Sprintf("%s:%d", listenHost, listenPort))
	errors.CheckError(err)

	healthz.ServeHealthCheck(http.DefaultServeMux, func(r *http.Request) error {
		if val, ok := r.URL.Query()["full"]; ok && len(val) > 0 && val[0] == "true" {
			// connect to itself to make sure commit server is able to serve connection
			// used by liveness probe to auto restart commit server
			conn, err := apiclient.NewConnection(fmt.Sprintf("localhost:%d", listenPort))
			if err != nil {
				return err
			}
			defer utilio.Close(conn)
			client := grpc_health_v1.NewHealthClient(conn)
			res, err := client.Check(r.Context(), &grpc_health_v1.HealthCheckRequest{})
			if err != nil {
				return err
			}
			if res.Status != grpc_health_v1.HealthCheckResponse_SERVING {
				return fmt.Errorf("grpc health check status is '%v'", res.Status)
			}
			return nil
		}
		return nil
	})

	// Graceful shutdown code adapted from here: https://gist.github.com/embano1/e0bf49d24f1cdd07cffad93097c04f0a
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	wg := sync.WaitGroup{}
	wg.Go(func() {
		s := <-sigCh
		log.Printf("got signal %v, attempting graceful shutdown", s)
		grpcServer.GracefulStop()
	})

	log.Println("starting grpc server")
	err = grpcServer.Serve(listener)
	errors.CheckError(err)
	wg.Wait()
	log.Println("clean shutdown")

	return nil
}
