package commands

import (
	"context"
	"fmt"
	"github.com/argoproj/argo-cd/v2/util/settings"
	flag "github.com/spf13/pflag"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"math"
	"time"

	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/controller"
	"github.com/argoproj/argo-cd/v2/controller/sharding"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/v2/pkg/ratelimiter"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/argo/normalizers"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/errors"
	kubeutil "github.com/argoproj/argo-cd/v2/util/kube"
	"github.com/argoproj/argo-cd/v2/util/tls"
	"github.com/argoproj/argo-cd/v2/util/trace"
	"github.com/argoproj/pkg/stats"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

const (
	// CLIName is the name of the CLI
	cliName = common.ApplicationController
	// Default time in seconds for application resync period
	defaultAppResyncPeriod = 180
	// Default time in seconds for application hard resync period
	defaultAppHardResyncPeriod = 0
)

type ApplicationControllerConfig struct {
	flags                            *flag.FlagSet
	pflags                           *flag.FlagSet
	workqueueRateLimit               ratelimiter.AppControllerRateLimiterConfig
	appResyncPeriod                  int64
	appHardResyncPeriod              int64
	appResyncJitter                  int64
	repoErrorGracePeriod             int64
	repoServerAddress                string
	repoServerTimeoutSeconds         int
	selfHealTimeoutSeconds           int
	statusProcessors                 int
	operationProcessors              int
	glogLevel                        int
	metricsPort                      int
	metricsCacheExpiration           time.Duration
	metricsApplicationLabels         []string
	kubectlParallelismLimit          int64
	cacheSource                      func() (*appstatecache.Cache, error)
	redisClient                      *redis.Client
	repoServerPlaintext              bool
	repoServerStrictTLS              bool
	otlpAddress                      string
	otlpInsecure                     bool
	otlpHeaders                      map[string]string
	otlpAttrs                        []string
	applicationNamespaces            []string
	persistResourceHealth            bool
	shardingAlgorithm                string
	enableDynamicClusterDistribution bool
	serverSideDiff                   bool
	clientConfig                     clientcmd.ClientConfig
	config                           *rest.Config
	namespace                        string
	ignoreNormalizerOpts             normalizers.IgnoreNormalizerOpts
}

func NewApplicationControllerConfig(flags, pflags *flag.FlagSet) *ApplicationControllerConfig {
	return &ApplicationControllerConfig{flags: flags, pflags: pflags}
}

func (c *ApplicationControllerConfig) WithDefaultFlags() *ApplicationControllerConfig {
	c.flags.Int64Var(&c.appResyncPeriod, "app-resync", int64(env.ParseDurationFromEnv("ARGOCD_RECONCILIATION_TIMEOUT", defaultAppResyncPeriod*time.Second, 0, math.MaxInt64).Seconds()), "Time period in seconds for application resync.")
	c.flags.Int64Var(&c.appHardResyncPeriod, "app-hard-resync", int64(env.ParseDurationFromEnv("ARGOCD_HARD_RECONCILIATION_TIMEOUT", defaultAppHardResyncPeriod*time.Second, 0, math.MaxInt64).Seconds()), "Time period in seconds for application hard resync.")
	c.flags.Int64Var(&c.appResyncJitter, "app-resync-jitter", int64(env.ParseDurationFromEnv("ARGOCD_RECONCILIATION_JITTER", 0*time.Second, 0, math.MaxInt64).Seconds()), "Maximum time period in seconds to add as a delay jitter for application resync.")
	c.flags.Int64Var(&c.repoErrorGracePeriod, "repo-error-grace-period-seconds", int64(env.ParseDurationFromEnv("ARGOCD_REPO_ERROR_GRACE_PERIOD_SECONDS", defaultAppResyncPeriod*time.Second, 0, math.MaxInt64).Seconds()), "Grace period in seconds for ignoring consecutive errors while communicating with repo server.")
	c.flags.StringVar(&c.repoServerAddress, "repo-server", env.StringFromEnv("ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER", common.DefaultRepoServerAddr), "Repo server address.")
	c.flags.IntVar(&c.repoServerTimeoutSeconds, "repo-server-timeout-seconds", env.ParseNumFromEnv("ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_TIMEOUT_SECONDS", 60, 0, math.MaxInt64), "Repo server RPC call timeout seconds.")
	c.flags.IntVar(&c.statusProcessors, "status-processors", env.ParseNumFromEnv("ARGOCD_APPLICATION_CONTROLLER_STATUS_PROCESSORS", 20, 0, math.MaxInt32), "Number of application status processors")
	c.flags.IntVar(&c.operationProcessors, "operation-processors", env.ParseNumFromEnv("ARGOCD_APPLICATION_CONTROLLER_OPERATION_PROCESSORS", 10, 0, math.MaxInt32), "Number of application operation processors")
	c.flags.StringVar(&cmdutil.LogFormat, "logformat", env.StringFromEnv("ARGOCD_APPLICATION_CONTROLLER_LOGFORMAT", "text"), "Set the logging format. One of: text|json")
	c.flags.StringVar(&cmdutil.LogLevel, "loglevel", env.StringFromEnv("ARGOCD_APPLICATION_CONTROLLER_LOGLEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	c.flags.IntVar(&c.glogLevel, "gloglevel", 0, "Set the glog logging level")
	c.flags.IntVar(&c.metricsPort, "metrics-port", common.DefaultPortArgoCDMetrics, "Start metrics server on given port")
	c.flags.DurationVar(&c.metricsCacheExpiration, "metrics-cache-expiration", env.ParseDurationFromEnv("ARGOCD_APPLICATION_CONTROLLER_METRICS_CACHE_EXPIRATION", 0*time.Second, 0, math.MaxInt64), "Prometheus metrics cache expiration (disabled  by default. e.g. 24h0m0s)")
	c.flags.IntVar(&c.selfHealTimeoutSeconds, "self-heal-timeout-seconds", env.ParseNumFromEnv("ARGOCD_APPLICATION_CONTROLLER_SELF_HEAL_TIMEOUT_SECONDS", 5, 0, math.MaxInt32), "Specifies timeout between application self heal attempts")
	c.flags.Int64Var(&c.kubectlParallelismLimit, "kubectl-parallelism-limit", env.ParseInt64FromEnv("ARGOCD_APPLICATION_CONTROLLER_KUBECTL_PARALLELISM_LIMIT", 20, 0, math.MaxInt64), "Number of allowed concurrent kubectl fork/execs. Any value less than 1 means no limit.")
	c.flags.BoolVar(&c.repoServerPlaintext, "repo-server-plaintext", env.ParseBoolFromEnv("ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_PLAINTEXT", false), "Disable TLS on connections to repo server")
	c.flags.BoolVar(&c.repoServerStrictTLS, "repo-server-strict-tls", env.ParseBoolFromEnv("ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_STRICT_TLS", false), "Whether to use strict validation of the TLS cert presented by the repo server")
	c.flags.StringSliceVar(&c.metricsApplicationLabels, "metrics-application-labels", []string{}, "List of Application labels that will be added to the argocd_application_labels metric")
	c.flags.StringVar(&c.otlpAddress, "otlp-address", env.StringFromEnv("ARGOCD_APPLICATION_CONTROLLER_OTLP_ADDRESS", ""), "OpenTelemetry collector address to send traces to")
	c.flags.BoolVar(&c.otlpInsecure, "otlp-insecure", env.ParseBoolFromEnv("ARGOCD_APPLICATION_CONTROLLER_OTLP_INSECURE", true), "OpenTelemetry collector insecure mode")
	c.flags.StringToStringVar(&c.otlpHeaders, "otlp-headers", env.ParseStringToStringFromEnv("ARGOCD_APPLICATION_CONTROLLER_OTLP_HEADERS", map[string]string{}, ","), "List of OpenTelemetry collector extra headers sent with traces, headers are comma-separated key-value pairs(e.g. key1=value1,key2=value2)")
	c.flags.StringSliceVar(&c.otlpAttrs, "otlp-attrs", env.StringsFromEnv("ARGOCD_APPLICATION_CONTROLLER_OTLP_ATTRS", []string{}, ","), "List of OpenTelemetry collector extra attrs when send traces, each attribute is separated by a colon(e.g. key:value)")
	c.flags.StringSliceVar(&c.applicationNamespaces, "application-namespaces", env.StringsFromEnv("ARGOCD_APPLICATION_NAMESPACES", []string{}, ","), "List of additional namespaces that applications are allowed to be reconciled from")
	c.flags.BoolVar(&c.persistResourceHealth, "persist-resource-health", env.ParseBoolFromEnv("ARGOCD_APPLICATION_CONTROLLER_PERSIST_RESOURCE_HEALTH", true), "Enables storing the managed resources health in the Application CRD")
	c.flags.StringVar(&c.shardingAlgorithm, "sharding-method", env.StringFromEnv(common.EnvControllerShardingAlgorithm, common.DefaultShardingAlgorithm), "Enables choice of sharding method. Supported sharding methods are : [legacy, round-robin] ")
	// global queue rate limit config
	c.flags.Int64Var(&c.workqueueRateLimit.BucketSize, "wq-bucket-size", env.ParseInt64FromEnv("WORKQUEUE_BUCKET_SIZE", 500, 1, math.MaxInt64), "Set Workqueue Rate Limiter Bucket Size, default 500")
	c.flags.Float64Var(&c.workqueueRateLimit.BucketQPS, "wq-bucket-qps", env.ParseFloat64FromEnv("WORKQUEUE_BUCKET_QPS", math.MaxFloat64, 1, math.MaxFloat64), "Set Workqueue Rate Limiter Bucket QPS, default c.flags to MaxFloat64 which disables the bucket limiter")
	// individual item rate limit config
	// when WORKQUEUE_FAILURE_COOLDOWN is 0 per item rate limiting is disabled(default)
	c.flags.DurationVar(&c.workqueueRateLimit.FailureCoolDown, "wq-cooldown-ns", time.Duration(env.ParseInt64FromEnv("WORKQUEUE_FAILURE_COOLDOWN_NS", 0, 0, (24*time.Hour).Nanoseconds())), "Set Workqueue Per Item Rate Limiter Cooldown duration in ns, default 0(per item rate limiter disabled)")
	c.flags.DurationVar(&c.workqueueRateLimit.BaseDelay, "wq-basedelay-ns", time.Duration(env.ParseInt64FromEnv("WORKQUEUE_BASE_DELAY_NS", time.Millisecond.Nanoseconds(), time.Nanosecond.Nanoseconds(), (24*time.Hour).Nanoseconds())), "Set Workqueue Per Item Rate Limiter Base Delay duration in nanoseconds, default 1000000 (1ms)")
	c.flags.DurationVar(&c.workqueueRateLimit.MaxDelay, "wq-maxdelay-ns", time.Duration(env.ParseInt64FromEnv("WORKQUEUE_MAX_DELAY_NS", time.Second.Nanoseconds(), 1*time.Millisecond.Nanoseconds(), (24*time.Hour).Nanoseconds())), "Set Workqueue Per Item Rate Limiter Max Delay duration in nanoseconds, default 1000000000 (1s)")
	c.flags.Float64Var(&c.workqueueRateLimit.BackoffFactor, "wq-backoff-factor", env.ParseFloat64FromEnv("WORKQUEUE_BACKOFF_FACTOR", 1.5, 0, math.MaxFloat64), "Set Workqueue Per Item Rate Limiter Backoff Factor, default is 1.5")
	c.flags.BoolVar(&c.enableDynamicClusterDistribution, "dynamic-cluster-distribution-enabled", env.ParseBoolFromEnv(common.EnvEnableDynamicClusterDistribution, false), "Enables dynamic cluster distribution.")
	c.flags.BoolVar(&c.serverSideDiff, "server-side-diff-enabled", env.ParseBoolFromEnv(common.EnvServerSideDiff, false), "Feature flag to enable ServerSide diff. Default (\"false\")")
	c.cacheSource = appstatecache.AddCacheFlagsToCmd(c.flags, cacheutil.Options{
		OnClientCreated: func(client *redis.Client) {
			c.redisClient = client
		},
	})
	return c
}

func (c *ApplicationControllerConfig) WithKubectlFlags() *ApplicationControllerConfig {
	c.clientConfig = cli.AddKubectlFlagsToSet(c.pflags)
	return c
}

func (c *ApplicationControllerConfig) WithK8sSettings(namespace string, config *rest.Config) *ApplicationControllerConfig {
	c.config = config
	c.namespace = namespace
	return c
}

func (c *ApplicationControllerConfig) CreateApplicationController(ctx context.Context) error {
	var namespace string
	var config *rest.Config
	if c.clientConfig != nil {
		ns, _, err := c.clientConfig.Namespace()
		errors.CheckError(err)
		config, err = c.clientConfig.ClientConfig()
		errors.CheckError(err)
		namespace = ns
	} else {
		config = c.config
		namespace = c.namespace
	}

	errors.CheckError(v1alpha1.SetK8SConfigDefaults(config))

	vers := common.GetVersion()
	vers.LogStartupInfo(
		"ArgoCD Application Controller",
		map[string]any{
			"namespace": namespace,
		},
	)

	config.UserAgent = fmt.Sprintf("%s/%s (%s)", common.DefaultApplicationControllerName, vers.Version, vers.Platform)

	kubeClient := kubernetes.NewForConfigOrDie(config)
	appClient := appclientset.NewForConfigOrDie(config)

	hardResyncDuration := time.Duration(c.appHardResyncPeriod) * time.Second

	var resyncDuration time.Duration
	if c.appResyncPeriod == 0 {
		// Re-sync should be disabled if period is 0. Set duration to a very long duration
		resyncDuration = time.Hour * 24 * 365 * 100
	} else {
		resyncDuration = time.Duration(c.appResyncPeriod) * time.Second
	}

	tlsConfig := apiclient.TLSConfiguration{
		DisableTLS:       c.repoServerPlaintext,
		StrictValidation: c.repoServerStrictTLS,
	}

	// Load CA information to use for validating connections to the
	// repository server, if strict TLS validation was requested.
	if !c.repoServerPlaintext && c.repoServerStrictTLS {
		pool, err := tls.LoadX509CertPool(
			fmt.Sprintf("%s/controller/tls/tls.crt", env.StringFromEnv(common.EnvAppConfigPath, common.DefaultAppConfigPath)),
			fmt.Sprintf("%s/controller/tls/ca.crt", env.StringFromEnv(common.EnvAppConfigPath, common.DefaultAppConfigPath)),
		)
		if err != nil {
			log.Fatalf("%v", err)
		}
		tlsConfig.Certificates = pool
	}

	repoClientset := apiclient.NewRepoServerClientset(c.repoServerAddress, c.repoServerTimeoutSeconds, tlsConfig)

	cache, err := c.cacheSource()
	errors.CheckError(err)
	cache.Cache.SetClient(cacheutil.NewTwoLevelClient(cache.Cache.GetClient(), 10*time.Minute))

	var appController *controller.ApplicationController

	settingsMgr := settings.NewSettingsManager(ctx, kubeClient, namespace, settings.WithRepoOrClusterChangedHandler(func() {
		appController.InvalidateProjectsCache()
	}))
	kubectl := kubeutil.NewKubectl()
	clusterSharding, err := sharding.GetClusterSharding(kubeClient, settingsMgr, c.shardingAlgorithm, c.enableDynamicClusterDistribution)
	errors.CheckError(err)
	appController, err = controller.NewApplicationController(
		namespace,
		settingsMgr,
		kubeClient,
		appClient,
		repoClientset,
		cache,
		kubectl,
		resyncDuration,
		hardResyncDuration,
		time.Duration(c.appResyncJitter)*time.Second,
		time.Duration(c.selfHealTimeoutSeconds)*time.Second,
		time.Duration(c.repoErrorGracePeriod)*time.Second,
		c.metricsPort,
		c.metricsCacheExpiration,
		c.metricsApplicationLabels,
		c.kubectlParallelismLimit,
		c.persistResourceHealth,
		clusterSharding,
		c.applicationNamespaces,
		&c.workqueueRateLimit,
		c.serverSideDiff,
		c.enableDynamicClusterDistribution,
		c.ignoreNormalizerOpts,
	)
	errors.CheckError(err)
	cacheutil.CollectMetrics(c.redisClient, appController.GetMetricsServer())

	stats.RegisterStackDumper()
	stats.StartStatsTicker(10 * time.Minute)
	stats.RegisterHeapDumper("memprofile")

	if c.otlpAddress != "" {
		closeTracer, err := trace.InitTracer(ctx, "argocd-controller", c.otlpAddress, c.otlpInsecure, c.otlpHeaders, c.otlpAttrs)
		if err != nil {
			log.Fatalf("failed to initialize tracing: %v", err)
		}
		defer closeTracer()
	}

	go appController.Run(ctx, c.statusProcessors, c.operationProcessors)

	return nil
}

func NewCommand() *cobra.Command {
	var config *ApplicationControllerConfig
	var command = cobra.Command{
		Use:               cliName,
		Short:             "Run ArgoCD Application Controller",
		Long:              "ArgoCD application controller is a Kubernetes controller that continuously monitors running applications and compares the current, live state against the desired target state (as specified in the repo). This command runs Application Controller in the foreground.  It can be configured by following options.",
		DisableAutoGenTag: true,
		RunE: func(c *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(c.Context())
			defer cancel()
			cli.SetLogFormat(cmdutil.LogFormat)
			cli.SetLogLevel(cmdutil.LogLevel)
			cli.SetGLogLevel(config.glogLevel)
			err := config.CreateApplicationController(ctx)
			errors.CheckError(err)
			// Wait forever
			select {}
		},
	}

	config = NewApplicationControllerConfig(command.Flags(), command.PersistentFlags()).WithDefaultFlags().WithKubectlFlags()
	return &command
}
