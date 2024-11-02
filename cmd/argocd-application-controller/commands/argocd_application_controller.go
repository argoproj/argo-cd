package commands

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/controller"
	"github.com/argoproj/argo-cd/v2/controller/sharding"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/v2/pkg/ratelimiter"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/argo/normalizers"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/errors"
	kubeutil "github.com/argoproj/argo-cd/v2/util/kube"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/argoproj/argo-cd/v2/util/tls"
	"github.com/argoproj/argo-cd/v2/util/trace"
	"github.com/argoproj/pkg/stats"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
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
	cmd                              *cobra.Command
	workqueueRateLimit               ratelimiter.AppControllerRateLimiterConfig
	appResyncPeriod                  int64
	appHardResyncPeriod              int64
	appResyncJitter                  int64
	repoErrorGracePeriod             int64
	repoServerAddress                string
	repoServerTimeoutSeconds         int
	selfHealTimeoutSeconds           int
	selfHealBackoffTimeoutSeconds    int
	selfHealBackoffFactor            int
	selfHealBackoffCapSeconds        int
	statusProcessors                 int
	operationProcessors              int
	glogLevel                        int
	metricsPort                      int
	metricsCacheExpiration           time.Duration
	metricsApplicationLabels         []string
	metricsAplicationConditions      []string
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

	// argocd k8s event logging flag
	enableK8sEvent []string
}

func NewApplicationControllerConfig(cmd *cobra.Command) *ApplicationControllerConfig {
	return &ApplicationControllerConfig{cmd: cmd}
}

func (c *ApplicationControllerConfig) WithDefaultFlags() *ApplicationControllerConfig {
	c.cmd.Flags().Int64Var(&c.appResyncPeriod, "app-resync", int64(env.ParseDurationFromEnv("ARGOCD_RECONCILIATION_TIMEOUT", defaultAppResyncPeriod*time.Second, 0, math.MaxInt64).Seconds()), "Time period in seconds for application resync.")
	c.cmd.Flags().Int64Var(&c.appHardResyncPeriod, "app-hard-resync", int64(env.ParseDurationFromEnv("ARGOCD_HARD_RECONCILIATION_TIMEOUT", defaultAppHardResyncPeriod*time.Second, 0, math.MaxInt64).Seconds()), "Time period in seconds for application hard resync.")
	c.cmd.Flags().Int64Var(&c.appResyncJitter, "app-resync-jitter", int64(env.ParseDurationFromEnv("ARGOCD_RECONCILIATION_JITTER", 0*time.Second, 0, math.MaxInt64).Seconds()), "Maximum time period in seconds to add as a delay jitter for application resync.")
	c.cmd.Flags().Int64Var(&c.repoErrorGracePeriod, "repo-error-grace-period-seconds", int64(env.ParseDurationFromEnv("ARGOCD_REPO_ERROR_GRACE_PERIOD_SECONDS", defaultAppResyncPeriod*time.Second, 0, math.MaxInt64).Seconds()), "Grace period in seconds for ignoring consecutive errors while communicating with repo server.")
	c.cmd.Flags().StringVar(&c.repoServerAddress, "repo-server", env.StringFromEnv("ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER", common.DefaultRepoServerAddr), "Repo server address.")
	c.cmd.Flags().IntVar(&c.repoServerTimeoutSeconds, "repo-server-timeout-seconds", env.ParseNumFromEnv("ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_TIMEOUT_SECONDS", 60, 0, math.MaxInt64), "Repo server RPC call timeout seconds.")
	c.cmd.Flags().IntVar(&c.statusProcessors, "status-processors", env.ParseNumFromEnv("ARGOCD_APPLICATION_CONTROLLER_STATUS_PROCESSORS", 20, 0, math.MaxInt32), "Number of application status processors")
	c.cmd.Flags().IntVar(&c.operationProcessors, "operation-processors", env.ParseNumFromEnv("ARGOCD_APPLICATION_CONTROLLER_OPERATION_PROCESSORS", 10, 0, math.MaxInt32), "Number of application operation processors")
	c.cmd.Flags().IntVar(&c.glogLevel, "gloglevel", 0, "Set the glog logging level")
	c.cmd.Flags().IntVar(&c.metricsPort, "metrics-port", common.DefaultPortArgoCDMetrics, "Start metrics server on given port")
	c.cmd.Flags().DurationVar(&c.metricsCacheExpiration, "metrics-cache-expiration", env.ParseDurationFromEnv("ARGOCD_APPLICATION_CONTROLLER_METRICS_CACHE_EXPIRATION", 0*time.Second, 0, math.MaxInt64), "Prometheus metrics cache expiration (disabled  by default. e.g. 24h0m0s)")
	c.cmd.Flags().IntVar(&c.selfHealTimeoutSeconds, "self-heal-timeout-seconds", env.ParseNumFromEnv("ARGOCD_APPLICATION_CONTROLLER_SELF_HEAL_TIMEOUT_SECONDS", 0, 0, math.MaxInt32), "Specifies timeout between application self heal attempts")
	c.cmd.Flags().IntVar(&c.selfHealBackoffTimeoutSeconds, "self-heal-backoff-timeout-seconds", env.ParseNumFromEnv("ARGOCD_APPLICATION_CONTROLLER_SELF_HEAL_BACKOFF_TIMEOUT_SECONDS", 2, 0, math.MaxInt32), "Specifies initial timeout of exponential backoff between self heal attempts")
	c.cmd.Flags().IntVar(&c.selfHealBackoffFactor, "self-heal-backoff-factor", env.ParseNumFromEnv("ARGOCD_APPLICATION_CONTROLLER_SELF_HEAL_BACKOFF_FACTOR", 3, 0, math.MaxInt32), "Specifies factor of exponential timeout between application self heal attempts")
	c.cmd.Flags().IntVar(&c.selfHealBackoffCapSeconds, "self-heal-backoff-cap-seconds", env.ParseNumFromEnv("ARGOCD_APPLICATION_CONTROLLER_SELF_HEAL_BACKOFF_CAP_SECONDS", 300, 0, math.MaxInt32), "Specifies max timeout of exponential backoff between application self heal attempts")
	c.cmd.Flags().Int64Var(&c.kubectlParallelismLimit, "kubectl-parallelism-limit", env.ParseInt64FromEnv("ARGOCD_APPLICATION_CONTROLLER_KUBECTL_PARALLELISM_LIMIT", 20, 0, math.MaxInt64), "Number of allowed concurrent kubectl fork/execs. Any value less than 1 means no limit.")
	c.cmd.Flags().BoolVar(&c.repoServerPlaintext, "repo-server-plaintext", env.ParseBoolFromEnv("ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_PLAINTEXT", false), "Disable TLS on connections to repo server")
	c.cmd.Flags().BoolVar(&c.repoServerStrictTLS, "repo-server-strict-tls", env.ParseBoolFromEnv("ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_STRICT_TLS", false), "Whether to use strict validation of the TLS cert presented by the repo server")
	c.cmd.Flags().StringSliceVar(&c.metricsApplicationLabels, "metrics-application-labels", []string{}, "List of Application labels that will be added to the argocd_application_labels metric")
	c.cmd.Flags().StringSliceVar(&c.metricsAplicationConditions, "metrics-application-conditions", []string{}, "List of Application conditions that will be added to the argocd_application_conditions metric")
	c.cmd.Flags().StringVar(&c.otlpAddress, "otlp-address", env.StringFromEnv("ARGOCD_APPLICATION_CONTROLLER_OTLP_ADDRESS", ""), "OpenTelemetry collector address to send traces to")
	c.cmd.Flags().BoolVar(&c.otlpInsecure, "otlp-insecure", env.ParseBoolFromEnv("ARGOCD_APPLICATION_CONTROLLER_OTLP_INSECURE", true), "OpenTelemetry collector insecure mode")
	c.cmd.Flags().StringToStringVar(&c.otlpHeaders, "otlp-headers", env.ParseStringToStringFromEnv("ARGOCD_APPLICATION_CONTROLLER_OTLP_HEADERS", map[string]string{}, ","), "List of OpenTelemetry collector extra headers sent with traces, headers are comma-separated key-value pairs(e.g. key1=value1,key2=value2)")
	c.cmd.Flags().StringSliceVar(&c.otlpAttrs, "otlp-attrs", env.StringsFromEnv("ARGOCD_APPLICATION_CONTROLLER_OTLP_ATTRS", []string{}, ","), "List of OpenTelemetry collector extra attrs when send traces, each attribute is separated by a colon(e.g. key:value)")
	c.cmd.Flags().StringSliceVar(&c.applicationNamespaces, "application-namespaces", env.StringsFromEnv("ARGOCD_APPLICATION_NAMESPACES", []string{}, ","), "List of additional namespaces that applications are allowed to be reconciled from")
	c.cmd.Flags().BoolVar(&c.persistResourceHealth, "persist-resource-health", env.ParseBoolFromEnv("ARGOCD_APPLICATION_CONTROLLER_PERSIST_RESOURCE_HEALTH", true), "Enables storing the managed resources health in the Application CRD")
	c.cmd.Flags().StringVar(&c.shardingAlgorithm, "sharding-method", env.StringFromEnv(common.EnvControllerShardingAlgorithm, common.DefaultShardingAlgorithm), "Enables choice of sharding method. Supported sharding methods are : [legacy, round-robin, consistent-hashing] ")
	// global queue rate limit config
	c.cmd.Flags().Int64Var(&c.workqueueRateLimit.BucketSize, "wq-bucket-size", env.ParseInt64FromEnv("WORKQUEUE_BUCKET_SIZE", 500, 1, math.MaxInt64), "Set Workqueue Rate Limiter Bucket Size, default 500")
	c.cmd.Flags().Float64Var(&c.workqueueRateLimit.BucketQPS, "wq-bucket-qps", env.ParseFloat64FromEnv("WORKQUEUE_BUCKET_QPS", math.MaxFloat64, 1, math.MaxFloat64), "Set Workqueue Rate Limiter Bucket QPS, default set to MaxFloat64 which disables the bucket limiter")
	// individual item rate limit config
	// when WORKQUEUE_FAILURE_COOLDOWN is 0 per item rate limiting is disabled(default)
	c.cmd.Flags().DurationVar(&c.workqueueRateLimit.FailureCoolDown, "wq-cooldown-ns", time.Duration(env.ParseInt64FromEnv("WORKQUEUE_FAILURE_COOLDOWN_NS", 0, 0, (24*time.Hour).Nanoseconds())), "Set Workqueue Per Item Rate Limiter Cooldown duration in ns, default 0(per item rate limiter disabled)")
	c.cmd.Flags().DurationVar(&c.workqueueRateLimit.BaseDelay, "wq-basedelay-ns", time.Duration(env.ParseInt64FromEnv("WORKQUEUE_BASE_DELAY_NS", time.Millisecond.Nanoseconds(), time.Nanosecond.Nanoseconds(), (24*time.Hour).Nanoseconds())), "Set Workqueue Per Item Rate Limiter Base Delay duration in nanoseconds, default 1000000 (1ms)")
	c.cmd.Flags().DurationVar(&c.workqueueRateLimit.MaxDelay, "wq-maxdelay-ns", time.Duration(env.ParseInt64FromEnv("WORKQUEUE_MAX_DELAY_NS", time.Second.Nanoseconds(), 1*time.Millisecond.Nanoseconds(), (24*time.Hour).Nanoseconds())), "Set Workqueue Per Item Rate Limiter Max Delay duration in nanoseconds, default 1000000000 (1s)")
	c.cmd.Flags().Float64Var(&c.workqueueRateLimit.BackoffFactor, "wq-backoff-factor", env.ParseFloat64FromEnv("WORKQUEUE_BACKOFF_FACTOR", 1.5, 0, math.MaxFloat64), "Set Workqueue Per Item Rate Limiter Backoff Factor, default is 1.5")
	c.cmd.Flags().BoolVar(&c.enableDynamicClusterDistribution, "dynamic-cluster-distribution-enabled", env.ParseBoolFromEnv(common.EnvEnableDynamicClusterDistribution, false), "Enables dynamic cluster distribution.")
	c.cmd.Flags().BoolVar(&c.serverSideDiff, "server-side-diff-enabled", env.ParseBoolFromEnv(common.EnvServerSideDiff, false), "Feature flag to enable ServerSide diff. Default (\"false\")")
	c.cmd.Flags().DurationVar(&c.ignoreNormalizerOpts.JQExecutionTimeout, "ignore-normalizer-jq-execution-timeout-seconds", env.ParseDurationFromEnv("ARGOCD_IGNORE_NORMALIZER_JQ_TIMEOUT", 0*time.Second, 0, math.MaxInt64), "Set ignore normalizer JQ execution timeout")
	// argocd k8s event logging flag
	c.cmd.Flags().StringSliceVar(&c.enableK8sEvent, "enable-k8s-event", env.StringsFromEnv("ARGOCD_ENABLE_K8S_EVENT", argo.DefaultEnableEventList(), ","), "Enable ArgoCD to use k8s event. For disabling all events, set the value as `none`. (e.g --enable-k8s-event=none), For enabling specific events, set the value as `event reason`. (e.g --enable-k8s-event=StatusRefreshed,ResourceCreated)")
	c.cacheSource = appstatecache.AddCacheFlagsToCmd(c.cmd, cacheutil.Options{
		OnClientCreated: func(client *redis.Client) {
			c.redisClient = client
		},
	})
	return c
}

func (c *ApplicationControllerConfig) WithKubectlFlags() *ApplicationControllerConfig {
	c.clientConfig = cli.AddKubectlFlagsToSet(c.cmd.PersistentFlags())
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
	var selfHealBackoff *wait.Backoff
	if c.selfHealBackoffTimeoutSeconds != 0 {
		selfHealBackoff = &wait.Backoff{
			Duration: time.Duration(c.selfHealBackoffTimeoutSeconds) * time.Second,
			Factor:   float64(c.selfHealBackoffFactor),
			Cap:      time.Duration(c.selfHealBackoffCapSeconds) * time.Second,
		}
	}
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
		selfHealBackoff,
		time.Duration(c.repoErrorGracePeriod)*time.Second,
		c.metricsPort,
		c.metricsCacheExpiration,
		c.metricsApplicationLabels,
		c.metricsAplicationConditions,
		c.kubectlParallelismLimit,
		c.persistResourceHealth,
		clusterSharding,
		c.applicationNamespaces,
		&c.workqueueRateLimit,
		c.serverSideDiff,
		c.enableDynamicClusterDistribution,
		c.ignoreNormalizerOpts,
		c.enableK8sEvent,
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
	command := cobra.Command{
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

			// Graceful shutdown code
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			go func() {
				s := <-sigCh
				log.Printf("got signal %v, attempting graceful shutdown", s)
				cancel()
			}()

			err := config.CreateApplicationController(ctx)
			errors.CheckError(err)

			<-ctx.Done()

			log.Println("clean shutdown")

			return nil
		},
	}

	config = NewApplicationControllerConfig(&command).WithDefaultFlags().WithKubectlFlags()
	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", env.StringFromEnv("ARGOCD_APPLICATION_CONTROLLER_LOGFORMAT", "text"), "Set the logging format. One of: text|json")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", env.StringFromEnv("ARGOCD_APPLICATION_CONTROLLER_LOGLEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	return &command
}
