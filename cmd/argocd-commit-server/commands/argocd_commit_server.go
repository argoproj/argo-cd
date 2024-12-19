package commands

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/health/grpc_health_v1"

	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/commitserver"
	"github.com/argoproj/argo-cd/v2/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v2/commitserver/metrics"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/askpass"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/healthz"
	ioutil "github.com/argoproj/argo-cd/v2/util/io"
)

// NewCommand returns a new instance of an argocd-commit-server command
func NewCommand() *cobra.Command {
	var (
		listenHost  string
		listenPort  int
		metricsPort int
		metricsHost string
	)
	command := &cobra.Command{
		Use:   "argocd-commit-server",
		Short: "Run Argo CD Commit Server",
		Long:  "Argo CD Commit Server is an internal service which commits and pushes hydrated manifests to git. This command runs Commit Server in the foreground.",
		RunE: func(cmd *cobra.Command, args []string) error {
			vers := common.GetVersion()
			vers.LogStartupInfo(
				"Argo CD Commit Server",
				map[string]any{
					"port": listenPort,
				},
			)

			cli.SetLogFormat(cmdutil.LogFormat)
			cli.SetLogLevel(cmdutil.LogLevel)

			metricsServer := metrics.NewMetricsServer()
			http.Handle("/metrics", metricsServer.GetHandler())
			go func() { errors.CheckError(http.ListenAndServe(fmt.Sprintf("%s:%d", metricsHost, metricsPort), nil)) }()

			askPassServer := askpass.NewServer(askpass.CommitServerSocketPath)
			go func() { errors.CheckError(askPassServer.Run()) }()

			server := commitserver.NewServer(askPassServer, metricsServer)
			grpc := server.CreateGRPC()

			listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", listenHost, listenPort))
			errors.CheckError(err)

			healthz.ServeHealthCheck(http.DefaultServeMux, func(r *http.Request) error {
				if val, ok := r.URL.Query()["full"]; ok && len(val) > 0 && val[0] == "true" {
					// connect to itself to make sure commit server is able to serve connection
					// used by liveness probe to auto restart commit server
					conn, err := apiclient.NewConnection(fmt.Sprintf("localhost:%d", listenPort))
					if err != nil {
						return err
					}
					defer ioutil.Close(conn)
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
			wg.Add(1)
			go func() {
				s := <-sigCh
				log.Printf("got signal %v, attempting graceful shutdown", s)
				grpc.GracefulStop()
				wg.Done()
			}()

			log.Println("starting grpc server")
			err = grpc.Serve(listener)
			errors.CheckError(err)
			wg.Wait()
			log.Println("clean shutdown")

			return nil
		},
	}
	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", env.StringFromEnv("ARGOCD_COMMIT_SERVER_LOGFORMAT", "text"), "Set the logging format. One of: text|json")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", env.StringFromEnv("ARGOCD_COMMIT_SERVER_LOGLEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	command.Flags().StringVar(&listenHost, "address", env.StringFromEnv("ARGOCD_COMMIT_SERVER_LISTEN_ADDRESS", common.DefaultAddressCommitServer), "Listen on given address for incoming connections")
	command.Flags().IntVar(&listenPort, "port", common.DefaultPortCommitServer, "Listen on given port for incoming connections")
	command.Flags().StringVar(&metricsHost, "metrics-address", env.StringFromEnv("ARGOCD_COMMIT_SERVER_METRICS_LISTEN_ADDRESS", common.DefaultAddressCommitServerMetrics), "Listen on given address for metrics")
	command.Flags().IntVar(&metricsPort, "metrics-port", common.DefaultPortCommitServerMetrics, "Start metrics server on given port")

	return command
}
