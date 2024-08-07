package commands

import (
	"context"
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

type RepoServerConfig struct {
	cmd                                          *cobra.Command
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

func NewRepoServerConfig(cmd *cobra.Command) *RepoServerConfig {
	return &RepoServerConfig{cmd: cmd}
}

func (c *RepoServerConfig) WithDefaultFlags() *RepoServerConfig {
	c.cmd.Flags().StringVar(&cmdutil.LogFormat, "logformat", env.StringFromEnv("ARGOCD_REPO_SERVER_LOGFORMAT", "text"), "Set the logging format. One of: text|json")
	c.cmd.Flags().StringVar(&cmdutil.LogLevel, "loglevel", env.StringFromEnv("ARGOCD_REPO_SERVER_LOGLEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	c.cmd.Flags().Int64Var(&c.parallelismLimit, "parallelismlimit", int64(env.ParseNumFromEnv("ARGOCD_REPO_SERVER_PARALLELISM_LIMIT", 0, 0, math.MaxInt32)), "Limit on number of concurrent manifests generate requests. Any value less the 1 means no limit.")
	c.cmd.Flags().StringVar(&c.listenHost, "address", env.StringFromEnv("ARGOCD_REPO_SERVER_LISTEN_ADDRESS", common.DefaultAddressRepoServer), "Listen on given address for incoming connections")
	c.cmd.Flags().IntVar(&c.listenPort, "port", common.DefaultPortRepoServer, "Listen on given port for incoming connections")
	c.cmd.Flags().StringVar(&c.metricsHost, "metrics-address", env.StringFromEnv("ARGOCD_REPO_SERVER_METRICS_LISTEN_ADDRESS", common.DefaultAddressRepoServerMetrics), "Listen on given address for metrics")
	c.cmd.Flags().IntVar(&c.metricsPort, "metrics-port", common.DefaultPortRepoServerMetrics, "Start metrics server on given port")
	c.cmd.Flags().StringVar(&c.otlpAddress, "otlp-address", env.StringFromEnv("ARGOCD_REPO_SERVER_OTLP_ADDRESS", ""), "OpenTelemetry collector address to send traces to")
	c.cmd.Flags().BoolVar(&c.otlpInsecure, "otlp-insecure", env.ParseBoolFromEnv("ARGOCD_REPO_SERVER_OTLP_INSECURE", true), "OpenTelemetry collector insecure mode")
	c.cmd.Flags().StringToStringVar(&c.otlpHeaders, "otlp-headers", env.ParseStringToStringFromEnv("ARGOCD_REPO_OTLP_HEADERS", map[string]string{}, ","), "List of OpenTelemetry collector extra headers sent with traces, headers are comma-separated key-value pairs(e.g. key1=value1,key2=value2)")
	c.cmd.Flags().StringSliceVar(&c.otlpAttrs, "otlp-attrs", env.StringsFromEnv("ARGOCD_REPO_SERVER_OTLP_ATTRS", []string{}, ","), "List of OpenTelemetry collector extra attrs when send traces, each attribute is separated by a colon(e.g. key:value)")
	c.cmd.Flags().BoolVar(&c.disableTLS, "disable-tls", env.ParseBoolFromEnv("ARGOCD_REPO_SERVER_DISABLE_TLS", false), "Disable TLS on the gRPC endpoint")
	c.cmd.Flags().StringVar(&c.maxCombinedDirectoryManifestsSize, "max-combined-directory-manifests-size", env.StringFromEnv("ARGOCD_REPO_SERVER_MAX_COMBINED_DIRECTORY_MANIFESTS_SIZE", "10M"), "Max combined size of manifest files in a directory-type Application")
	c.cmd.Flags().StringArrayVar(&c.cmpTarExcludedGlobs, "plugin-tar-exclude", env.StringsFromEnv("ARGOCD_REPO_SERVER_PLUGIN_TAR_EXCLUSIONS", []string{}, ";"), "Globs to filter when sending tarballs to plugins.")
	c.cmd.Flags().BoolVar(&c.allowOutOfBoundsSymlinks, "allow-oob-symlinks", env.ParseBoolFromEnv("ARGOCD_REPO_SERVER_ALLOW_OUT_OF_BOUNDS_SYMLINKS", false), "Allow out-of-bounds symlinks in repositories (not recommended)")
	c.cmd.Flags().StringVar(&c.streamedManifestMaxTarSize, "streamed-manifest-max-tar-size", env.StringFromEnv("ARGOCD_REPO_SERVER_STREAMED_MANIFEST_MAX_TAR_SIZE", "100M"), "Maximum size of streamed manifest archives")
	c.cmd.Flags().StringVar(&c.streamedManifestMaxExtractedSize, "streamed-manifest-max-extracted-size", env.StringFromEnv("ARGOCD_REPO_SERVER_STREAMED_MANIFEST_MAX_EXTRACTED_SIZE", "1G"), "Maximum size of streamed manifest archives when extracted")
	c.cmd.Flags().StringVar(&c.helmManifestMaxExtractedSize, "helm-manifest-max-extracted-size", env.StringFromEnv("ARGOCD_REPO_SERVER_HELM_MANIFEST_MAX_EXTRACTED_SIZE", "1G"), "Maximum size of helm manifest archives when extracted")
	c.cmd.Flags().StringVar(&c.helmRegistryMaxIndexSize, "helm-registry-max-index-size", env.StringFromEnv("ARGOCD_REPO_SERVER_HELM_MANIFEST_MAX_INDEX_SIZE", "1G"), "Maximum size of registry index file")
	c.cmd.Flags().BoolVar(&c.disableManifestMaxExtractedSize, "disable-helm-manifest-max-extracted-size", env.ParseBoolFromEnv("ARGOCD_REPO_SERVER_DISABLE_HELM_MANIFEST_MAX_EXTRACTED_SIZE", false), "Disable maximum size of helm manifest archives when extracted")
	c.cmd.Flags().BoolVar(&c.includeHiddenDirectories, "include-hidden-directories", env.ParseBoolFromEnv("ARGOCD_REPO_SERVER_INCLUDE_HIDDEN_DIRECTORIES", false), "Include hidden directories from Git")
	c.cmd.Flags().BoolVar(&c.cmpUseManifestGeneratePaths, "plugin-use-manifest-generate-paths", env.ParseBoolFromEnv("ARGOCD_REPO_SERVER_PLUGIN_USE_MANIFEST_GENERATE_PATHS", false), "Pass the resources described in argocd.argoproj.io/manifest-generate-paths value to the cmpserver to generate the application manifests.")
	c.cacheSrc = reposervercache.AddCacheFlagsToCmd(c.cmd, cacheutil.Options{
		OnClientCreated: func(client *redis.Client) {
			c.redisClient = client
		},
	})
	c.tlsConfigCustomizerSrc = tls.AddTLSFlagsToCmd(c.cmd)

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
	go func() {
		errors.CheckError(http.ListenAndServe(fmt.Sprintf("%s:%d", c.metricsHost, c.metricsPort), nil))
	}()
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
}

func NewCommand() *cobra.Command {
	var config *RepoServerConfig
	command := cobra.Command{
		Use:               cliName,
		Short:             "Run ArgoCD Repository Server",
		Long:              "ArgoCD Repository Server is an internal service which maintains a local cache of the Git repository holding the application manifests, and is responsible for generating and returning the Kubernetes manifests.  This command runs Repository Server in the foreground.  It can be configured by following options.",
		DisableAutoGenTag: true,
		RunE: func(c *cobra.Command, args []string) error {
			return config.CreateRepoServer(c.Context())
		},
	}
	config = NewRepoServerConfig(&command).WithDefaultFlags()
	return &command
}
