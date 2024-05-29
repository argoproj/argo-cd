package commands

import (
	"context"
	"fmt"
	flag "github.com/spf13/pflag"
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

type RepoServerConfig struct {
	flags                                        *flag.FlagSet
	parallelismLimit                             int64
	listenPort                                   int
	listenHost                                   string
	metricsPort                                  int
	metricsHost                                  string
	otlpAddress                                  string
	otlpInsecure                                 bool
	otlpHeaders                                  map[string]string
	otlpAttrs                                    []string
	cacheSrc                                     func() (*reposervercache.Cache, error)
	tlsConfigCustomizer               tls.ConfigCustomizer
	tlsConfigCustomizerSrc                       func() (tls.ConfigCustomizer, error)
	redisClient                                  *redis.Client
	disableTLS                                   bool
	maxCombinedDirectoryManifestsSize            string
	cmpTarExcludedGlobs                          []string
	allowOutOfBoundsSymlinks                     bool
	streamedManifestMaxTarSize                   string
	streamedManifestMaxExtractedSize             string
	helmManifestMaxExtractedSize                 string
	helmRegistryMaxIndexSize                     string
	disableManifestMaxExtractedSize              bool
	gnuPGSourcePath                              string
	gnuPGWrapperPath                             string
	pauseGenerationAfterFailedGenerationAttempts int
	pauseGenerationOnFailureForMinutes           int
	pauseGenerationOnFailureForRequests          int
	gitSubmoduleEnabled                          bool
	includeHiddenDirectories          bool
	cmpUseManifestGeneratePaths       bool
}

func NewRepoServerConfig(flags *flag.FlagSet) *RepoServerConfig {
	return &RepoServerConfig{flags: flags}
}

func (c *RepoServerConfig) WithDefaultFlags() *RepoServerConfig {
	c.flags.StringVar(&cmdutil.LogFormat, "logformat", env.StringFromEnv("ARGOCD_REPO_SERVER_LOGFORMAT", "text"), "Set the logging format. One of: text|json")
	c.flags.StringVar(&cmdutil.LogLevel, "loglevel", env.StringFromEnv("ARGOCD_REPO_SERVER_LOGLEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	c.flags.Int64Var(&c.parallelismLimit, "parallelismlimit", int64(env.ParseNumFromEnv("ARGOCD_REPO_SERVER_PARALLELISM_LIMIT", 0, 0, math.MaxInt32)), "Limit on number of concurrent manifests generate requests. Any value less the 1 means no limit.")
	c.flags.StringVar(&c.listenHost, "address", env.StringFromEnv("ARGOCD_REPO_SERVER_LISTEN_ADDRESS", common.DefaultAddressRepoServer), "Listen on given address for incoming connections")
	c.flags.IntVar(&c.listenPort, "port", common.DefaultPortRepoServer, "Listen on given port for incoming connections")
	c.flags.StringVar(&c.metricsHost, "metrics-address", env.StringFromEnv("ARGOCD_REPO_SERVER_METRICS_LISTEN_ADDRESS", common.DefaultAddressRepoServerMetrics), "Listen on given address for metrics")
	c.flags.IntVar(&c.metricsPort, "metrics-port", common.DefaultPortRepoServerMetrics, "Start metrics server on given port")
	c.flags.StringVar(&c.otlpAddress, "otlp-address", env.StringFromEnv("ARGOCD_REPO_SERVER_OTLP_ADDRESS", ""), "OpenTelemetry collector address to send traces to")
	c.flags.BoolVar(&c.otlpInsecure, "otlp-insecure", env.ParseBoolFromEnv("ARGOCD_REPO_SERVER_OTLP_INSECURE", true), "OpenTelemetry collector insecure mode")
	c.flags.StringToStringVar(&c.otlpHeaders, "otlp-headers", env.ParseStringToStringFromEnv("ARGOCD_REPO_OTLP_HEADERS", map[string]string{}, ","), "List of OpenTelemetry collector extra headers sent with traces, headers are comma-separated key-value pairs(e.g. key1=value1,key2=value2)")
	c.flags.StringSliceVar(&c.otlpAttrs, "otlp-attrs", env.StringsFromEnv("ARGOCD_REPO_SERVER_OTLP_ATTRS", []string{}, ","), "List of OpenTelemetry collector extra attrs when send traces, each attribute is separated by a colon(e.g. key:value)")
	c.flags.BoolVar(&c.disableTLS, "disable-tls", env.ParseBoolFromEnv("ARGOCD_REPO_SERVER_DISABLE_TLS", false), "Disable TLS on the gRPC endpoint")
	c.flags.StringVar(&c.maxCombinedDirectoryManifestsSize, "max-combined-directory-manifests-size", env.StringFromEnv("ARGOCD_REPO_SERVER_MAX_COMBINED_DIRECTORY_MANIFESTS_SIZE", "10M"), "Max combined size of manifest files in a directory-type Application")
	c.flags.StringArrayVar(&c.cmpTarExcludedGlobs, "plugin-tar-exclude", env.StringsFromEnv("ARGOCD_REPO_SERVER_PLUGIN_TAR_EXCLUSIONS", []string{}, ";"), "Globs to filter when sending tarballs to plugins.")
	c.flags.BoolVar(&c.allowOutOfBoundsSymlinks, "allow-oob-symlinks", env.ParseBoolFromEnv("ARGOCD_REPO_SERVER_ALLOW_OUT_OF_BOUNDS_SYMLINKS", false), "Allow out-of-bounds symlinks in repositories (not recommended)")
	c.flags.StringVar(&c.streamedManifestMaxTarSize, "streamed-manifest-max-tar-size", env.StringFromEnv("ARGOCD_REPO_SERVER_STREAMED_MANIFEST_MAX_TAR_SIZE", "100M"), "Maximum size of streamed manifest archives")
	c.flags.StringVar(&c.streamedManifestMaxExtractedSize, "streamed-manifest-max-extracted-size", env.StringFromEnv("ARGOCD_REPO_SERVER_STREAMED_MANIFEST_MAX_EXTRACTED_SIZE", "1G"), "Maximum size of streamed manifest archives when extracted")
	c.flags.StringVar(&c.helmManifestMaxExtractedSize, "helm-manifest-max-extracted-size", env.StringFromEnv("ARGOCD_REPO_SERVER_HELM_MANIFEST_MAX_EXTRACTED_SIZE", "1G"), "Maximum size of helm manifest archives when extracted")
	c.flags.StringVar(&c.helmRegistryMaxIndexSize, "helm-registry-max-index-size", env.StringFromEnv("ARGOCD_REPO_SERVER_HELM_MANIFEST_MAX_INDEX_SIZE", "1G"), "Maximum size of registry index file")
	c.flags.BoolVar(&c.disableManifestMaxExtractedSize, "disable-helm-manifest-max-extracted-size", env.ParseBoolFromEnv("ARGOCD_REPO_SERVER_DISABLE_HELM_MANIFEST_MAX_EXTRACTED_SIZE", false), "Disable maximum size of helm manifest archives when extracted")
	c.flags.BoolVar(&c.includeHiddenDirectories, "include-hidden-directories", env.ParseBoolFromEnv("ARGOCD_REPO_SERVER_INCLUDE_HIDDEN_DIRECTORIES", false), "Include hidden directories from Git")
	c.flags.BoolVar(&c.cmpUseManifestGeneratePaths, "plugin-use-manifest-generate-paths", env.ParseBoolFromEnv("ARGOCD_REPO_SERVER_PLUGIN_USE_MANIFEST_GENERATE_PATHS", false), "Pass the resources described in argocd.argoproj.io/manifest-generate-paths value to the cmpserver to generate the application manifests.")
	c.cacheSrc = reposervercache.AddCacheFlagsToCmd(c.flags, cacheutil.Options{
		OnClientCreated: func(client *redis.Client) {
			c.redisClient = client
		},
	})
	c.tlsConfigCustomizerSrc = tls.AddTLSFlagsToCmd(c.flags)

	c.gnuPGSourcePath = env.StringFromEnv(common.EnvGPGDataPath, "/app/config/gpg/source")
	c.gnuPGWrapperPath = env.StringFromEnv(common.EnvGPGWrapperPath, "")
	c.pauseGenerationAfterFailedGenerationAttempts = env.ParseNumFromEnv(common.EnvPauseGenerationAfterFailedAttempts, 3, 0, math.MaxInt32)
	c.pauseGenerationOnFailureForMinutes = env.ParseNumFromEnv(common.EnvPauseGenerationMinutes, 60, 0, math.MaxInt32)
	c.pauseGenerationOnFailureForRequests = env.ParseNumFromEnv(common.EnvPauseGenerationRequests, 0, 0, math.MaxInt32)
	c.gitSubmoduleEnabled = env.ParseBoolFromEnv(common.EnvGitSubmoduleEnabled, true)
	return c
}

func (c *RepoServerConfig) CreateRepoServer(ctx context.Context) error {
	vers := common.GetVersion()
	vers.LogStartupInfo(
		"ArgoCD Repository Server",
		map[string]any{
			"port": c.listenPort,
		},
	)

	cli.SetLogFormat(cmdutil.LogFormat)
	cli.SetLogLevel(cmdutil.LogLevel)

	var tlsConfigCustomizer tls.ConfigCustomizer
	if !c.disableTLS {
		var err error
		tlsConfigCustomizer, err = c.tlsConfigCustomizerSrc()
		errors.CheckError(err)
	}

	cache, err := c.cacheSrc()
	errors.CheckError(err)

	maxCombinedDirectoryManifestsQuantity, err := resource.ParseQuantity(c.maxCombinedDirectoryManifestsSize)
	errors.CheckError(err)

	streamedManifestMaxTarSizeQuantity, err := resource.ParseQuantity(c.streamedManifestMaxTarSize)
	errors.CheckError(err)

	streamedManifestMaxExtractedSizeQuantity, err := resource.ParseQuantity(c.streamedManifestMaxExtractedSize)
	errors.CheckError(err)

	helmManifestMaxExtractedSizeQuantity, err := resource.ParseQuantity(c.helmManifestMaxExtractedSize)
	errors.CheckError(err)

	helmRegistryMaxIndexSizeQuantity, err := resource.ParseQuantity(c.helmRegistryMaxIndexSize)
	errors.CheckError(err)

			askPassServer := askpass.NewServer(askpass.SocketPath)
			metricsServer := metrics.NewMetricsServer()
			cacheutil.CollectMetrics(c.redisClient, metricsServer)
			server, err := reposerver.NewServer(metricsServer, cache, tlsConfigCustomizer, repository.RepoServerInitConstants{
				ParallelismLimit: c.parallelismLimit,
				PauseGenerationAfterFailedGenerationAttempts: c.pauseGenerationAfterFailedGenerationAttempts,
				PauseGenerationOnFailureForMinutes:           c.pauseGenerationOnFailureForMinutes,
				PauseGenerationOnFailureForRequests:          c.pauseGenerationOnFailureForRequests,
				SubmoduleEnabled:                             c.gitSubmoduleEnabled,
				MaxCombinedDirectoryManifestsSize:            maxCombinedDirectoryManifestsQuantity,
				CMPTarExcludedGlobs:                          c.cmpTarExcludedGlobs,
				AllowOutOfBoundsSymlinks:                     c.allowOutOfBoundsSymlinks,
				StreamedManifestMaxExtractedSize:             streamedManifestMaxExtractedSizeQuantity.ToDec().Value(),
				StreamedManifestMaxTarSize:                   streamedManifestMaxTarSizeQuantity.ToDec().Value(),
				HelmManifestMaxExtractedSize:                 helmManifestMaxExtractedSizeQuantity.ToDec().Value(),
				HelmRegistryMaxIndexSize:                     helmRegistryMaxIndexSizeQuantity.ToDec().Value(),
				IncludeHiddenDirectories:                     c.includeHiddenDirectories,
				CMPUseManifestGeneratePaths:                  c.cmpUseManifestGeneratePaths,
			}, askPassServer)
			errors.CheckError(err)

	if c.otlpAddress != "" {
		var closer func()
		var err error
		closer, err = traceutil.InitTracer(ctx, "argocd-repo-server", c.otlpAddress, c.otlpInsecure, c.otlpHeaders, c.otlpAttrs)
		if err != nil {
			log.Fatalf("failed to initialize tracing: %v", err)
		}
		defer closer()
	}

	grpc := server.CreateGRPC()
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", c.listenHost, c.listenPort))
	errors.CheckError(err)

			healthz.ServeHealthCheck(http.DefaultServeMux, func(r *http.Request) error {
				if val, ok := r.URL.Query()["full"]; ok && len(val) > 0 && val[0] == "true" {
					// connect to itself to make sure repo server is able to serve connection
					// used by liveness probe to auto restart repo server
					// see https://github.com/argoproj/argo-cd/issues/5110 for more information
					conn, err := apiclient.NewConnection(fmt.Sprintf("localhost:%d", c.listenPort), 60, &apiclient.TLSConfiguration{DisableTLS: c.disableTLS})
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
			go func() { errors.CheckError(http.ListenAndServe(fmt.Sprintf("%s:%d", c.metricsHost, c.metricsPort), nil)) }()
			go func() { errors.CheckError(askPassServer.Run()) }()

	if gpg.IsGPGEnabled() {
		log.Infof("Initializing GnuPG keyring at %s", common.GetGnuPGHomePath())
		err = gpg.InitializeGnuPG()
		errors.CheckError(err)

		log.Infof("Populating GnuPG keyring with keys from %s", c.gnuPGSourcePath)
		added, removed, err := gpg.SyncKeyRingFromDirectory(c.gnuPGSourcePath, c.gnuPGWrapperPath)
		errors.CheckError(err)
		log.Infof("Loaded %d (and removed %d) keys from keyring", len(added), len(removed))

		go func() { errors.CheckError(reposerver.StartGPGWatcher(c.gnuPGSourcePath, c.gnuPGWrapperPath)) }()
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
	tlsConfigCustomizerSrc = tls.AddTLSFlagsToCmd(&command)
	cacheSrc = reposervercache.AddCacheFlagsToCmd(&command, cacheutil.Options{
		OnClientCreated: func(client *redis.Client) {
			redisClient = client
		},
	}
	config = NewRepoServerConfig(command.Flags()).WithDefaultFlags()
	return &command
}
