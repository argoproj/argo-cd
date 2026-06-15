package commands

import (
	"context"
	stderrors "errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/health/grpc_health_v1"

	cmdutil "github.com/argoproj/argo-cd/v3/cmd/util"
	"github.com/argoproj/argo-cd/v3/commitserver"
	"github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v3/commitserver/metrics"
	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/util/askpass"
	"github.com/argoproj/argo-cd/v3/util/cli"
	"github.com/argoproj/argo-cd/v3/util/env"
	"github.com/argoproj/argo-cd/v3/util/errors"
	"github.com/argoproj/argo-cd/v3/util/gpgsign"
	"github.com/argoproj/argo-cd/v3/util/healthz"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/sourceintegrity"
)

// NewCommand returns a new instance of an argocd-commit-server command
func NewCommand() *cobra.Command {
	var (
		listenHost               string
		listenPort               int
		metricsPort              int
		metricsHost              string
		signingKeyPath           string
		signingKeyPassphraseFile string
	)
	command := &cobra.Command{
		Use:   common.CommandCommitServer,
		Short: "Run Argo CD Commit Server",
		Long:  "Argo CD Commit Server is an internal service which commits and pushes hydrated manifests to git. This command runs Commit Server in the foreground.",
		RunE: func(cmd *cobra.Command, _ []string) error {
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

			errors.CheckError(validateSigningFlags(signingKeyPath, signingKeyPassphraseFile))

			// A configured signing key path is what enables signing: there is
			// no separate on/off toggle. Inferring intent from the key path
			// avoids contradictory states (enabled-but-no-key, key-but-disabled).
			var signingConfig *gpgsign.Config
			if signingKeyPath != "" {
				cfg, err := setupSigningKey(signingKeyPath, signingKeyPassphraseFile)
				errors.CheckError(err)
				signingConfig = cfg
				log.WithFields(log.Fields{
					"keyID":       cfg.KeyID,
					"fingerprint": cfg.Fingerprint,
					"gnupgHome":   common.GetGnuPGHomePath(),
					"gpgProgram":  cfg.GPGProgram,
				}).Info("Hydrated commit signing enabled")
			}

			server := commitserver.NewServer(askPassServer, metricsServer, signingConfig)
			grpc := server.CreateGRPC()
			ctx := cmd.Context()

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
				grpc.GracefulStop()
			})

			log.Println("starting grpc server")
			err = grpc.Serve(listener)
			errors.CheckError(err)
			wg.Wait()
			log.Println("clean shutdown")

			return nil
		},
	}
	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", env.StringFromEnv("ARGOCD_COMMIT_SERVER_LOGFORMAT", "json"), "Set the logging format. One of: json|text")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", env.StringFromEnv("ARGOCD_COMMIT_SERVER_LOGLEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	command.Flags().StringVar(&listenHost, "address", env.StringFromEnv("ARGOCD_COMMIT_SERVER_LISTEN_ADDRESS", common.DefaultAddressCommitServer), "Listen on given address for incoming connections")
	command.Flags().IntVar(&listenPort, "port", common.DefaultPortCommitServer, "Listen on given port for incoming connections")
	command.Flags().StringVar(&metricsHost, "metrics-address", env.StringFromEnv("ARGOCD_COMMIT_SERVER_METRICS_LISTEN_ADDRESS", common.DefaultAddressCommitServerMetrics), "Listen on given address for metrics")
	command.Flags().IntVar(&metricsPort, "metrics-port", common.DefaultPortCommitServerMetrics, "Start metrics server on given port")
	command.Flags().StringVar(&signingKeyPath, "signing-key-path", env.StringFromEnv("ARGOCD_COMMIT_SERVER_SIGNING_KEY_PATH", ""), "Path to the ASCII-armored GPG private key used to sign hydrated commits. Setting this enables signing; when set, the commit server fails at startup if the key cannot be loaded.")
	command.Flags().StringVar(&signingKeyPassphraseFile, "signing-key-passphrase-file", env.StringFromEnv("ARGOCD_COMMIT_SERVER_SIGNING_KEY_PASSPHRASE_FILE", ""), "Optional path to a file containing the passphrase for the signing key.")

	return command
}

// validateSigningFlags rejects contradictory signing configuration before
// startup. Signing is enabled solely by setting the key path, so a passphrase
// without a key is meaningless — reject it rather than silently ignoring it.
func validateSigningFlags(keyPath, passphraseFile string) error {
	if keyPath == "" && passphraseFile != "" {
		return stderrors.New("signing-key-passphrase-file is set but signing-key-path is empty; set signing-key-path to enable signing")
	}
	return nil
}

// setupSigningKey initializes the shared GNUPGHOME, imports the configured
// private signing key, and writes the loopback gpg wrapper. Any error here
// must propagate up to abort startup — silently falling back to unsigned
// commits would defeat the feature's purpose.
func setupSigningKey(keyPath, passphraseFile string) (*gpgsign.Config, error) {
	if err := sourceintegrity.InitializeGnuPG(); err != nil {
		return nil, fmt.Errorf("failed to initialize GnuPG home: %w", err)
	}

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read signing key from %s: %w", keyPath, err)
	}
	if strings.TrimSpace(string(keyData)) == "" {
		return nil, fmt.Errorf("signing key file %s is empty", keyPath)
	}

	var passphrase string
	if passphraseFile != "" {
		pp, err := os.ReadFile(passphraseFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read signing key passphrase from %s: %w", passphraseFile, err)
		}
		// Strip trailing newline that file-editors love to add. We deliberately
		// don't TrimSpace — a passphrase legitimately can start/end with spaces.
		passphrase = strings.TrimRight(string(pp), "\r\n")
	}

	cfg, err := gpgsign.ImportSigningKey(context.Background(), keyData, passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to import signing key: %w", err)
	}

	// Keep the wrapper script out of GNUPGHOME — we don't want executable
	// scratch state mixed in with the secret keyring. A pod-scoped temp dir
	// is good enough; the commit server doesn't persist anything here.
	wrapperDir, err := os.MkdirTemp("", "argocd-commit-sign-")
	if err != nil {
		return nil, fmt.Errorf("failed to create wrapper dir: %w", err)
	}
	wrapper, err := gpgsign.WriteSignWrapper(wrapperDir, passphraseFile)
	if err != nil {
		return nil, fmt.Errorf("failed to write gpg sign wrapper: %w", err)
	}
	cfg.GPGProgram = wrapper
	return cfg, nil
}
