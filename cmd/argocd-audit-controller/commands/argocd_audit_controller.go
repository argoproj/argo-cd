package commands

import (
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v3/auditcontroller"
	cmdutil "github.com/argoproj/argo-cd/v3/cmd/util"
	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/util/cli"
	"github.com/argoproj/argo-cd/v3/util/env"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

const shutdownTimeout = 10 * time.Second

// NewCommand returns a new instance of an argocd-audit-controller command
func NewCommand() *cobra.Command {
	var (
		clientConfig    clientcmd.ClientConfig
		listenHost      string
		listenPort      int
		metricsHost     string
		metricsPort     int
		token           string
		bufferSize      int
		logFile         string
		watchEvents     bool
		eventNamespaces []string
		eventReasons    []string
	)
	command := &cobra.Command{
		Use:   common.CommandAuditController,
		Short: "Run the Argo CD audit controller",
		Long: `The Argo CD audit controller writes Argo CD's audit trail: who did what, when, from where and with what result.

It receives a record for every mutating API call and web terminal session from argocd-server, and turns the
events Argo CD's controllers emit (automated syncs, sync results, resource actions...) into records too. Every
record is written as one JSON line to standard output, so the audit trail is the pod's log:

  kubectl logs deploy/argocd-audit-controller | jq 'select(.kind == "AuditRecord")'

Recent records can also be queried over HTTP on /api/v1/records.`,
		RunE: cli.WithSignalContextE(func(cmd *cobra.Command, _ []string, _ context.CancelFunc) error {
			ctx := cmd.Context()
			cli.SetLogFormat(cmdutil.LogFormat)
			cli.SetLogLevel(cmdutil.LogLevel)
			// Operational logs go to stderr so that stdout carries nothing but audit records.
			log.SetOutput(os.Stderr)

			common.GetVersion().LogStartupInfo("Argo CD Audit Controller", map[string]any{
				"port":        listenPort,
				"watchEvents": watchEvents,
			})
			if token == "" {
				log.Warn("No audit token is configured: anyone who can reach the audit controller can write to and read the audit trail. Set ARGOCD_AUDIT_TOKEN.")
			}

			outputs := []io.Writer{os.Stdout}
			if logFile != "" {
				f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
				if err != nil {
					return fmt.Errorf("failed to open audit log file: %w", err)
				}
				defer utilio.Close(f)
				outputs = append(outputs, f)
				log.Infof("Also writing audit records to %s", logFile)
			}

			metrics := auditcontroller.NewMetrics()
			recorder := auditcontroller.NewRecorder(bufferSize, metrics, outputs...)

			if watchEvents {
				restConfig, err := clientConfig.ClientConfig()
				if err != nil {
					return fmt.Errorf("failed to load Kubernetes client config: %w", err)
				}
				restConfig.UserAgent = fmt.Sprintf("%s/%s", common.CommandAuditController, common.GetVersion().Version)
				client, err := kubernetes.NewForConfig(restConfig)
				if err != nil {
					return fmt.Errorf("failed to create Kubernetes client: %w", err)
				}
				if len(eventNamespaces) == 0 {
					ns, _, err := clientConfig.Namespace()
					if err != nil {
						return fmt.Errorf("failed to determine namespace: %w", err)
					}
					eventNamespaces = []string{ns}
				}
				watcher := auditcontroller.NewEventWatcher(client, recorder, metrics, auditcontroller.EventWatcherOptions{
					Namespaces:     eventNamespaces,
					Reasons:        eventReasons,
					IgnoredSources: auditcontroller.DefaultIgnoredEventSources,
				})
				go func() {
					if err := watcher.Run(ctx); err != nil {
						log.WithError(err).Error("Event watcher stopped")
					}
				}()
			}

			apiServer := &http.Server{
				Addr:              net.JoinHostPort(listenHost, strconv.Itoa(listenPort)),
				Handler:           auditcontroller.NewServer(recorder, token, metrics).Handler(),
				ReadHeaderTimeout: 10 * time.Second,
			}
			metricsServer := &http.Server{
				Addr:              net.JoinHostPort(metricsHost, strconv.Itoa(metricsPort)),
				Handler:           metrics.Handler(),
				ReadHeaderTimeout: 10 * time.Second,
			}

			errCh := make(chan error, 2)
			for _, srv := range []*http.Server{apiServer, metricsServer} {
				go func() {
					log.Infof("Listening on %s", srv.Addr)
					if err := srv.ListenAndServe(); err != nil && !stderrors.Is(err, http.ErrServerClosed) {
						errCh <- fmt.Errorf("server on %s failed: %w", srv.Addr, err)
					}
				}()
			}

			var runErr error
			select {
			case <-ctx.Done():
				log.Info("Shutting down audit controller")
			case runErr = <-errCh:
			}
			shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
			defer cancel()
			// Stop accepting records only after in-flight ingest requests are written out.
			_ = apiServer.Shutdown(shutdownCtx)
			_ = metricsServer.Shutdown(shutdownCtx)
			return runErr
		}),
	}

	clientConfig = cli.AddKubectlFlagsToCmd(command)
	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", env.StringFromEnv("ARGOCD_AUDIT_CONTROLLER_LOGFORMAT", "json"), "Set the logging format. One of: json|text")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", env.StringFromEnv("ARGOCD_AUDIT_CONTROLLER_LOGLEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	command.Flags().StringVar(&listenHost, "address", env.StringFromEnv("ARGOCD_AUDIT_CONTROLLER_LISTEN_ADDRESS", "0.0.0.0"), "Listen on given address")
	command.Flags().IntVar(&listenPort, "port", common.DefaultPortAuditController, "Listen on given port")
	command.Flags().StringVar(&metricsHost, "metrics-address", env.StringFromEnv("ARGOCD_AUDIT_CONTROLLER_METRICS_LISTEN_ADDRESS", "0.0.0.0"), "Listen for metrics on given address")
	command.Flags().IntVar(&metricsPort, "metrics-port", common.DefaultPortAuditControllerMetrics, "Start metrics server on given port")
	command.Flags().StringVar(&token, "token", env.StringFromEnv("ARGOCD_AUDIT_TOKEN", ""), "Shared token argocd-server authenticates with; also required to query records")
	command.Flags().IntVar(&bufferSize, "buffer-size", env.ParseNumFromEnv("ARGOCD_AUDIT_CONTROLLER_BUFFER_SIZE", 1000, 1, 1000000), "Number of recent records kept in memory for /api/v1/records")
	command.Flags().StringVar(&logFile, "log-file", env.StringFromEnv("ARGOCD_AUDIT_CONTROLLER_LOG_FILE", ""), "Also append audit records to this file (e.g. on a persistent volume)")
	command.Flags().BoolVar(&watchEvents, "watch-events", env.ParseBoolFromEnv("ARGOCD_AUDIT_CONTROLLER_WATCH_EVENTS", true), "Turn Kubernetes Events emitted by Argo CD controllers (automated syncs, sync results, ...) into audit records")
	command.Flags().StringSliceVar(&eventNamespaces, "event-namespaces", env.StringsFromEnv("ARGOCD_AUDIT_CONTROLLER_EVENT_NAMESPACES", []string{}, ","), "Namespaces to watch Argo CD events in. Defaults to the controller's namespace; add the namespaces of apps-in-any-namespace")
	command.Flags().StringSliceVar(&eventReasons, "event-reasons", env.StringsFromEnv("ARGOCD_AUDIT_CONTROLLER_EVENT_REASONS", auditcontroller.DefaultEventReasons, ","), "Reasons of the Argo CD events turned into audit records")
	return command
}
