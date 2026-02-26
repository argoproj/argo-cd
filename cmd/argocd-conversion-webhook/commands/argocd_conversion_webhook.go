package commands

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	cmdutil "github.com/argoproj/argo-cd/v3/cmd/util"
	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/server/conversion"
	"github.com/argoproj/argo-cd/v3/util/cli"
	"github.com/argoproj/argo-cd/v3/util/env"
	tlsutil "github.com/argoproj/argo-cd/v3/util/tls"
)

const (
	cliName         = "argocd-conversion-webhook"
	defaultPort     = 8443
	defaultCertPath = "/tls/tls.crt"
	defaultKeyPath  = "/tls/tls.key"
)

func NewCommand() *cobra.Command {
	var (
		port        int
		tlsCertPath string
		tlsKeyPath  string
		serviceName string
		namespace   string
	)

	command := &cobra.Command{
		Use:               cliName,
		Short:             "Run the ArgoCD Application CRD conversion webhook server",
		Long:              "A lightweight server that handles conversion between Application API versions (v1alpha1 <-> v1beta1)",
		DisableAutoGenTag: true,
		RunE: func(c *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(c.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			vers := common.GetVersion()
			vers.LogStartupInfo("ArgoCD Conversion Webhook", nil)

			cli.SetLogFormat(cmdutil.LogFormat)
			cli.SetLogLevel(cmdutil.LogLevel)

			// Build list of hosts for self-signed cert generation
			hosts := []string{
				"localhost",
				serviceName,
				fmt.Sprintf("%s.%s", serviceName, namespace),
				fmt.Sprintf("%s.%s.svc", serviceName, namespace),
				fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace),
			}

			// Create TLS config (will generate self-signed cert if not provided)
			tlsConfig, err := tlsutil.CreateTLSConfig(tlsCertPath, tlsKeyPath, hosts, true)
			if err != nil {
				return fmt.Errorf("failed to create TLS config: %w", err)
			}

			// Get the certificate for CA bundle injection
			if len(tlsConfig.Certificates) == 0 {
				return errors.New("no TLS certificates available")
			}
			cert := &tlsConfig.Certificates[0]

			// Set up in-cluster config for CA bundle injection
			restConfig, err := rest.InClusterConfig()
			if err != nil {
				log.Warnf("Not running in cluster, CA bundle injection disabled: %v", err)
			} else {
				// Verify we can create a client
				_, err = kubernetes.NewForConfig(restConfig)
				if err != nil {
					log.Warnf("Failed to create kubernetes client, CA bundle injection disabled: %v", err)
					restConfig = nil
				}
			}

			// Inject CA bundle into CRD
			if restConfig != nil && cert != nil {
				go func() {
					// Give the server a moment to start
					time.Sleep(2 * time.Second)
					if err := conversion.InjectCABundle(ctx, restConfig, cert); err != nil {
						log.Errorf("Failed to inject CA bundle into Application CRD: %v", err)
					}
				}()
			}

			// Set up HTTP server with conversion handler
			mux := http.NewServeMux()
			mux.Handle("/convert", conversion.NewHandler())

			// Health check endpoint
			mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			})

			// Ready check endpoint
			mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			})

			server := &http.Server{
				Addr:      fmt.Sprintf(":%d", port),
				Handler:   mux,
				TLSConfig: tlsConfig,
				// Timeouts to prevent slow client attacks
				ReadTimeout:  10 * time.Second,
				WriteTimeout: 10 * time.Second,
				IdleTimeout:  60 * time.Second,
			}

			log.Infof("Starting conversion webhook server on port %d", port)

			// Run server in background
			errCh := make(chan error, 1)
			go func() {
				// TLS cert/key are already loaded in tlsConfig, so we pass empty strings
				if err := server.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
					errCh <- err
				}
				close(errCh)
			}()

			log.Infof("Started conversion webhook server on port %d", port)

			// Wait for shutdown signal or server error
			select {
			case err := <-errCh:
				return fmt.Errorf("server error: %w", err)
			case <-ctx.Done():
				log.Info("Received shutdown signal, shutting down gracefully...")
			}

			// Graceful shutdown
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()
			if err := server.Shutdown(shutdownCtx); err != nil {
				log.Errorf("Error during shutdown: %v", err)
			}

			log.Info("Server stopped")
			return nil
		},
	}

	command.Flags().IntVar(&port, "port", env.ParseNumFromEnv("ARGOCD_CONVERSION_WEBHOOK_PORT", defaultPort, 1, 65535), "Port to listen on")
	command.Flags().StringVar(&tlsCertPath, "tls-cert-path", env.StringFromEnv("ARGOCD_CONVERSION_WEBHOOK_TLS_CERT_PATH", defaultCertPath), "Path to TLS certificate file (if not provided, a self-signed cert will be generated)")
	command.Flags().StringVar(&tlsKeyPath, "tls-key-path", env.StringFromEnv("ARGOCD_CONVERSION_WEBHOOK_TLS_KEY_PATH", defaultKeyPath), "Path to TLS key file (if not provided, a self-signed key will be generated)")
	command.Flags().StringVar(&serviceName, "service-name", env.StringFromEnv("ARGOCD_CONVERSION_WEBHOOK_SERVICE_NAME", "argocd-conversion-webhook"), "Kubernetes service name (used for self-signed cert SANs)")
	command.Flags().StringVar(&namespace, "namespace", env.StringFromEnv("ARGOCD_CONVERSION_WEBHOOK_NAMESPACE", "argocd"), "Kubernetes namespace (used for self-signed cert SANs)")
	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", env.StringFromEnv("ARGOCD_CONVERSION_WEBHOOK_LOGFORMAT", "text"), "Set the logging format. One of: json|text")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", env.StringFromEnv("ARGOCD_CONVERSION_WEBHOOK_LOGLEVEL", "info"), "Set the logging level. One of: trace|debug|info|warn|error")

	return command
}
