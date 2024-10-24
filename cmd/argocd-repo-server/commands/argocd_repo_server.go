package commands

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/argoproj/pkg/stats"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/health/grpc_health_v1"
	"k8s.io/apimachinery/pkg/api/resource"

	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/reposerver"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/reposerver/askpass"
	reposervercache "github.com/argoproj/argo-cd/v2/reposerver/cache"
	"github.com/argoproj/argo-cd/v2/reposerver/metrics"
	"github.com/argoproj/argo-cd/v2/reposerver/repository"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/gpg"
	"github.com/argoproj/argo-cd/v2/util/healthz"
	ioutil "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/tls"
	traceutil "github.com/argoproj/argo-cd/v2/util/trace"
)

const (
	// CLIName is the name of the CLI
	cliName = "argocd-repo-server"
)

var (
	gnuPGSourcePath                              = env.StringFromEnv(common.EnvGPGDataPath, "/app/config/gpg/source")
	pauseGenerationAfterFailedGenerationAttempts = env.ParseNumFromEnv(common.EnvPauseGenerationAfterFailedAttempts, 3, 0, math.MaxInt32)
	pauseGenerationOnFailureForMinutes           = env.ParseNumFromEnv(common.EnvPauseGenerationMinutes, 60, 0, math.MaxInt32)
	pauseGenerationOnFailureForRequests          = env.ParseNumFromEnv(common.EnvPauseGenerationRequests, 0, 0, math.MaxInt32)
	gitSubmoduleEnabled                          = env.ParseBoolFromEnv(common.EnvGitSubmoduleEnabled, true)
)

func NewCommand() *cobra.Command {
	var (
		parallelismLimit                  int64
		listenPort                        int
		listenHost                        string
		metricsPort                       int
		metricsHost                       string
		otlpAddress                       string
		otlpInsecure                      bool
		otlpHeaders                       map[string]string
		otlpAttrs                         []string
		cacheSrc                          func() (*reposervercache.Cache, error)
		tlsConfigCustomizer               tls.ConfigCustomizer
		tlsConfigCustomizerSrc            func() (tls.ConfigCustomizer, error)
		redisClient                       *redis.Client
		disableTLS                        bool
		maxCombinedDirectoryManifestsSize string
		cmpTarExcludedGlobs               []string
		allowOutOfBoundsSymlinks          bool
		streamedManifestMaxTarSize        string
		streamedManifestMaxExtractedSize  string
		helmManifestMaxExtractedSize      string
		helmRegistryMaxIndexSize          string
		disableManifestMaxExtractedSize   bool
		includeHiddenDirectories          bool
		cmpUseManifestGeneratePaths       bool
	)
	command := cobra.Command{
		Use:               cliName,
		Short:             "Run ArgoCD Repository Server",
		Long:              "ArgoCD Repository Server is an internal service which maintains a local cache of the Git repository holding the application manifests, and is responsible for generating and returning the Kubernetes manifests.  This command runs Repository Server in the foreground.  It can be configured by following options.",
		DisableAutoGenTag: true,
		RunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()

			vers := common.GetVersion()
			vers.LogStartupInfo(
				"ArgoCD Repository Server",
				map[string]any{
					"port": listenPort,
				},
			)

			cli.SetLogFormat(cmdutil.LogFormat)
			cli.SetLogLevel(cmdutil.LogLevel)

			if !disableTLS {
				var err error
				tlsConfigCustomizer, err = tlsConfigCustomizerSrc()
				errors.CheckError(err)
			}

			cache, err := cacheSrc()
			errors.CheckError(err)

			maxCombinedDirectoryManifestsQuantity, err := resource.ParseQuantity(maxCombinedDirectoryManifestsSize)
			errors.CheckError(err)

			streamedManifestMaxTarSizeQuantity, err := resource.ParseQuantity(streamedManifestMaxTarSize)
			errors.CheckError(err)

			streamedManifestMaxExtractedSizeQuantity, err := resource.ParseQuantity(streamedManifestMaxExtractedSize)
			errors.CheckError(err)

			helmManifestMaxExtractedSizeQuantity, err := resource.ParseQuantity(helmManifestMaxExtractedSize)
			errors.CheckError(err)

			helmRegistryMaxIndexSizeQuantity, err := resource.ParseQuantity(helmRegistryMaxIndexSize)
			errors.CheckError(err)

			askPassServer := askpass.NewServer(askpass.SocketPath)
			metricsServer := metrics.NewMetricsServer()
			cacheutil.CollectMetrics(redisClient, metricsServer)
			server, err := reposerver.NewServer(metricsServer, cache, tlsConfigCustomizer, repository.RepoServerInitConstants{
				ParallelismLimit: parallelismLimit,
				PauseGenerationAfterFailedGenerationAttempts: pauseGenerationAfterFailedGenerationAttempts,
				PauseGenerationOnFailureForMinutes:           pauseGenerationOnFailureForMinutes,
				PauseGenerationOnFailureForRequests:          pauseGenerationOnFailureForRequests,
				SubmoduleEnabled:                             gitSubmoduleEnabled,
				MaxCombinedDirectoryManifestsSize:            maxCombinedDirectoryManifestsQuantity,
				CMPTarExcludedGlobs:                          cmpTarExcludedGlobs,
				AllowOutOfBoundsSymlinks:                     allowOutOfBoundsSymlinks,
				StreamedManifestMaxExtractedSize:             streamedManifestMaxExtractedSizeQuantity.ToDec().Value(),
				StreamedManifestMaxTarSize:                   streamedManifestMaxTarSizeQuantity.ToDec().Value(),
				HelmManifestMaxExtractedSize:                 helmManifestMaxExtractedSizeQuantity.ToDec().Value(),
				HelmRegistryMaxIndexSize:                     helmRegistryMaxIndexSizeQuantity.ToDec().Value(),
				IncludeHiddenDirectories:                     includeHiddenDirectories,
				CMPUseManifestGeneratePaths:                  cmpUseManifestGeneratePaths,
			}, askPassServer)
			errors.CheckError(err)

			if otlpAddress != "" {
				var closer func()
				var err error
				closer, err = traceutil.InitTracer(ctx, "argocd-repo-server", otlpAddress, otlpInsecure, otlpHeaders, otlpAttrs)
				if err != nil {
					log.Fatalf("failed to initialize tracing: %v", err)
				}
				defer closer()
			}

			grpc := server.CreateGRPC()
			listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", listenHost, listenPort))
			errors.CheckError(err)

			healthz.ServeHealthCheck(http.DefaultServeMux, func(r *http.Request) error {
				if val, ok := r.URL.Query()["full"]; ok && len(val) > 0 && val[0] == "true" {
					// connect to itself to make sure repo server is able to serve connection
					// used by liveness probe to auto restart repo server
					// see https://github.com/argoproj/argo-cd/issues/5110 for more information
					conn, err := apiclient.NewConnection(fmt.Sprintf("localhost:%d", listenPort), 60, &apiclient.TLSConfiguration{DisableTLS: disableTLS})
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
			http.Handle("/metrics", metricsServer.GetHandler())
			go func() { errors.CheckError(http.ListenAndServe(fmt.Sprintf("%s:%d", metricsHost, metricsPort), nil)) }()
			go func() { errors.CheckError(askPassServer.Run()) }()

			if gpg.IsGPGEnabled() {
				log.Infof("Initializing GnuPG keyring at %s", common.GetGnuPGHomePath())
				err = gpg.InitializeGnuPG()
				errors.CheckError(err)

				log.Infof("Populating GnuPG keyring with keys from %s", gnuPGSourcePath)
				added, removed, err := gpg.SyncKeyRingFromDirectory(gnuPGSourcePath)
				errors.CheckError(err)
				log.Infof("Loaded %d (and removed %d) keys from keyring", len(added), len(removed))

				go func() { errors.CheckError(reposerver.StartGPGWatcher(gnuPGSourcePath)) }()
			}

			log.Infof("argocd-repo-server is listening on %s", listener.Addr())
			stats.RegisterStackDumper()
			stats.StartStatsTicker(10 * time.Minute)
			stats.RegisterHeapDumper("memprofile")

			// Graceful shutdown code adapted from https://gist.github.com/embano1/e0bf49d24f1cdd07cffad93097c04f0a
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
			if err != nil {
				log.Fatalf("could not serve: %v", err)
			}
			wg.Wait()
			log.Println("clean shutdown")

			return nil
		},
	}
	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", env.StringFromEnv("ARGOCD_REPO_SERVER_LOGFORMAT", "text"), "Set the logging format. One of: text|json")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", env.StringFromEnv("ARGOCD_REPO_SERVER_LOGLEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	command.Flags().Int64Var(&parallelismLimit, "parallelismlimit", int64(env.ParseNumFromEnv("ARGOCD_REPO_SERVER_PARALLELISM_LIMIT", 0, 0, math.MaxInt32)), "Limit on number of concurrent manifests generate requests. Any value less the 1 means no limit.")
	command.Flags().StringVar(&listenHost, "address", env.StringFromEnv("ARGOCD_REPO_SERVER_LISTEN_ADDRESS", common.DefaultAddressRepoServer), "Listen on given address for incoming connections")
	command.Flags().IntVar(&listenPort, "port", common.DefaultPortRepoServer, "Listen on given port for incoming connections")
	command.Flags().StringVar(&metricsHost, "metrics-address", env.StringFromEnv("ARGOCD_REPO_SERVER_METRICS_LISTEN_ADDRESS", common.DefaultAddressRepoServerMetrics), "Listen on given address for metrics")
	command.Flags().IntVar(&metricsPort, "metrics-port", common.DefaultPortRepoServerMetrics, "Start metrics server on given port")
	command.Flags().StringVar(&otlpAddress, "otlp-address", env.StringFromEnv("ARGOCD_REPO_SERVER_OTLP_ADDRESS", ""), "OpenTelemetry collector address to send traces to")
	command.Flags().BoolVar(&otlpInsecure, "otlp-insecure", env.ParseBoolFromEnv("ARGOCD_REPO_SERVER_OTLP_INSECURE", true), "OpenTelemetry collector insecure mode")
	command.Flags().StringToStringVar(&otlpHeaders, "otlp-headers", env.ParseStringToStringFromEnv("ARGOCD_REPO_OTLP_HEADERS", map[string]string{}, ","), "List of OpenTelemetry collector extra headers sent with traces, headers are comma-separated key-value pairs(e.g. key1=value1,key2=value2)")
	command.Flags().StringSliceVar(&otlpAttrs, "otlp-attrs", env.StringsFromEnv("ARGOCD_REPO_SERVER_OTLP_ATTRS", []string{}, ","), "List of OpenTelemetry collector extra attrs when send traces, each attribute is separated by a colon(e.g. key:value)")
	command.Flags().BoolVar(&disableTLS, "disable-tls", env.ParseBoolFromEnv("ARGOCD_REPO_SERVER_DISABLE_TLS", false), "Disable TLS on the gRPC endpoint")
	command.Flags().StringVar(&maxCombinedDirectoryManifestsSize, "max-combined-directory-manifests-size", env.StringFromEnv("ARGOCD_REPO_SERVER_MAX_COMBINED_DIRECTORY_MANIFESTS_SIZE", "10M"), "Max combined size of manifest files in a directory-type Application")
	command.Flags().StringArrayVar(&cmpTarExcludedGlobs, "plugin-tar-exclude", env.StringsFromEnv("ARGOCD_REPO_SERVER_PLUGIN_TAR_EXCLUSIONS", []string{}, ";"), "Globs to filter when sending tarballs to plugins.")
	command.Flags().BoolVar(&allowOutOfBoundsSymlinks, "allow-oob-symlinks", env.ParseBoolFromEnv("ARGOCD_REPO_SERVER_ALLOW_OUT_OF_BOUNDS_SYMLINKS", false), "Allow out-of-bounds symlinks in repositories (not recommended)")
	command.Flags().StringVar(&streamedManifestMaxTarSize, "streamed-manifest-max-tar-size", env.StringFromEnv("ARGOCD_REPO_SERVER_STREAMED_MANIFEST_MAX_TAR_SIZE", "100M"), "Maximum size of streamed manifest archives")
	command.Flags().StringVar(&streamedManifestMaxExtractedSize, "streamed-manifest-max-extracted-size", env.StringFromEnv("ARGOCD_REPO_SERVER_STREAMED_MANIFEST_MAX_EXTRACTED_SIZE", "1G"), "Maximum size of streamed manifest archives when extracted")
	command.Flags().StringVar(&helmManifestMaxExtractedSize, "helm-manifest-max-extracted-size", env.StringFromEnv("ARGOCD_REPO_SERVER_HELM_MANIFEST_MAX_EXTRACTED_SIZE", "1G"), "Maximum size of helm manifest archives when extracted")
	command.Flags().StringVar(&helmRegistryMaxIndexSize, "helm-registry-max-index-size", env.StringFromEnv("ARGOCD_REPO_SERVER_HELM_MANIFEST_MAX_INDEX_SIZE", "1G"), "Maximum size of registry index file")
	command.Flags().BoolVar(&disableManifestMaxExtractedSize, "disable-helm-manifest-max-extracted-size", env.ParseBoolFromEnv("ARGOCD_REPO_SERVER_DISABLE_HELM_MANIFEST_MAX_EXTRACTED_SIZE", false), "Disable maximum size of helm manifest archives when extracted")
	command.Flags().BoolVar(&includeHiddenDirectories, "include-hidden-directories", env.ParseBoolFromEnv("ARGOCD_REPO_SERVER_INCLUDE_HIDDEN_DIRECTORIES", false), "Include hidden directories from Git")
	command.Flags().BoolVar(&cmpUseManifestGeneratePaths, "plugin-use-manifest-generate-paths", env.ParseBoolFromEnv("ARGOCD_REPO_SERVER_PLUGIN_USE_MANIFEST_GENERATE_PATHS", false), "Pass the resources described in argocd.argoproj.io/manifest-generate-paths value to the cmpserver to generate the application manifests.")
	tlsConfigCustomizerSrc = tls.AddTLSFlagsToCmd(&command)
	cacheSrc = reposervercache.AddCacheFlagsToCmd(&command, cacheutil.Options{
		OnClientCreated: func(client *redis.Client) {
			redisClient = client
		},
	})
	return &command
}
