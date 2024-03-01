package commands

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/argoproj/pkg/stats"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/controller"
	"github.com/argoproj/argo-cd/v2/controller/sharding"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/v2/pkg/ratelimiter"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/errors"
	kubeutil "github.com/argoproj/argo-cd/v2/util/kube"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/argoproj/argo-cd/v2/util/tls"
	"github.com/argoproj/argo-cd/v2/util/trace"
)

const (
	// CLIName is the name of the CLI
	cliName = common.ApplicationController
	// Default time in seconds for application resync period
	defaultAppResyncPeriod = 180
	// Default time in seconds for application hard resync period
	defaultAppHardResyncPeriod = 0
)

func NewCommand() *cobra.Command {
	var (
		workqueueRateLimit               ratelimiter.AppControllerRateLimiterConfig
		clientConfig                     clientcmd.ClientConfig
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
		metricsAplicationLabels          []string
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
	)
	var command = cobra.Command{
		Use:               cliName,
		Short:             "Run ArgoCD Application Controller",
		Long:              "ArgoCD application controller is a Kubernetes controller that continuously monitors running applications and compares the current, live state against the desired target state (as specified in the repo). This command runs Application Controller in the foreground.  It can be configured by following options.",
		DisableAutoGenTag: true,
		RunE: func(c *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(c.Context())
			defer cancel()

			vers := common.GetVersion()
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			vers.LogStartupInfo(
				"ArgoCD Application Controller",
				map[string]any{
					"namespace": namespace,
				},
			)

			cli.SetLogFormat(cmdutil.LogFormat)
			cli.SetLogLevel(cmdutil.LogLevel)
			cli.SetGLogLevel(glogLevel)

			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			errors.CheckError(v1alpha1.SetK8SConfigDefaults(config))
			config.UserAgent = fmt.Sprintf("%s/%s (%s)", common.DefaultApplicationControllerName, vers.Version, vers.Platform)

			kubeClient := kubernetes.NewForConfigOrDie(config)
			appClient := appclientset.NewForConfigOrDie(config)

			hardResyncDuration := time.Duration(appHardResyncPeriod) * time.Second

			var resyncDuration time.Duration
			if appResyncPeriod == 0 {
				// Re-sync should be disabled if period is 0. Set duration to a very long duration
				resyncDuration = time.Hour * 24 * 365 * 100
			} else {
				resyncDuration = time.Duration(appResyncPeriod) * time.Second
			}

			tlsConfig := apiclient.TLSConfiguration{
				DisableTLS:       repoServerPlaintext,
				StrictValidation: repoServerStrictTLS,
			}

			// Load CA information to use for validating connections to the
			// repository server, if strict TLS validation was requested.
			if !repoServerPlaintext && repoServerStrictTLS {
				pool, err := tls.LoadX509CertPool(
					fmt.Sprintf("%s/controller/tls/tls.crt", env.StringFromEnv(common.EnvAppConfigPath, common.DefaultAppConfigPath)),
					fmt.Sprintf("%s/controller/tls/ca.crt", env.StringFromEnv(common.EnvAppConfigPath, common.DefaultAppConfigPath)),
				)
				if err != nil {
					log.Fatalf("%v", err)
				}
				tlsConfig.Certificates = pool
			}

			repoClientset := apiclient.NewRepoServerClientset(repoServerAddress, repoServerTimeoutSeconds, tlsConfig)

			cache, err := cacheSource()
			errors.CheckError(err)
			cache.Cache.SetClient(cacheutil.NewTwoLevelClient(cache.Cache.GetClient(), 10*time.Minute))

			var appController *controller.ApplicationController

			settingsMgr := settings.NewSettingsManager(ctx, kubeClient, namespace, settings.WithRepoOrClusterChangedHandler(func() {
				appController.InvalidateProjectsCache()
			}))
			kubectl := kubeutil.NewKubectl()
			clusterSharding, err := sharding.GetClusterSharding(kubeClient, settingsMgr, shardingAlgorithm, enableDynamicClusterDistribution)
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
				time.Duration(appResyncJitter)*time.Second,
				time.Duration(selfHealTimeoutSeconds)*time.Second,
				time.Duration(repoErrorGracePeriod)*time.Second,
				metricsPort,
				metricsCacheExpiration,
				metricsAplicationLabels,
				kubectlParallelismLimit,
				persistResourceHealth,
				clusterSharding,
				applicationNamespaces,
				&workqueueRateLimit,
				serverSideDiff,
				enableDynamicClusterDistribution,
			)
			errors.CheckError(err)
			cacheutil.CollectMetrics(redisClient, appController.GetMetricsServer())

			stats.RegisterStackDumper()
			stats.StartStatsTicker(10 * time.Minute)
			stats.RegisterHeapDumper("memprofile")

			if otlpAddress != "" {
				closeTracer, err := trace.InitTracer(ctx, "argocd-controller", otlpAddress, otlpInsecure, otlpHeaders, otlpAttrs)
				if err != nil {
					log.Fatalf("failed to initialize tracing: %v", err)
				}
				defer closeTracer()
			}

			go appController.Run(ctx, statusProcessors, operationProcessors)

			// Wait forever
			select {}
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.Flags().Int64Var(&appResyncPeriod, "app-resync", int64(env.ParseDurationFromEnv("ARGOCD_RECONCILIATION_TIMEOUT", defaultAppResyncPeriod*time.Second, 0, math.MaxInt64).Seconds()), "Time period in seconds for application resync.")
	command.Flags().Int64Var(&appHardResyncPeriod, "app-hard-resync", int64(env.ParseDurationFromEnv("ARGOCD_HARD_RECONCILIATION_TIMEOUT", defaultAppHardResyncPeriod*time.Second, 0, math.MaxInt64).Seconds()), "Time period in seconds for application hard resync.")
	command.Flags().Int64Var(&appResyncJitter, "app-resync-jitter", int64(env.ParseDurationFromEnv("ARGOCD_RECONCILIATION_JITTER", 0*time.Second, 0, math.MaxInt64).Seconds()), "Maximum time period in seconds to add as a delay jitter for application resync.")
	command.Flags().Int64Var(&repoErrorGracePeriod, "repo-error-grace-period-seconds", int64(env.ParseDurationFromEnv("ARGOCD_REPO_ERROR_GRACE_PERIOD_SECONDS", defaultAppResyncPeriod*time.Second, 0, math.MaxInt64).Seconds()), "Grace period in seconds for ignoring consecutive errors while communicating with repo server.")
	command.Flags().StringVar(&repoServerAddress, "repo-server", env.StringFromEnv("ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER", common.DefaultRepoServerAddr), "Repo server address.")
	command.Flags().IntVar(&repoServerTimeoutSeconds, "repo-server-timeout-seconds", env.ParseNumFromEnv("ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_TIMEOUT_SECONDS", 60, 0, math.MaxInt64), "Repo server RPC call timeout seconds.")
	command.Flags().IntVar(&statusProcessors, "status-processors", env.ParseNumFromEnv("ARGOCD_APPLICATION_CONTROLLER_STATUS_PROCESSORS", 20, 0, math.MaxInt32), "Number of application status processors")
	command.Flags().IntVar(&operationProcessors, "operation-processors", env.ParseNumFromEnv("ARGOCD_APPLICATION_CONTROLLER_OPERATION_PROCESSORS", 10, 0, math.MaxInt32), "Number of application operation processors")
	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", env.StringFromEnv("ARGOCD_APPLICATION_CONTROLLER_LOGFORMAT", "text"), "Set the logging format. One of: text|json")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", env.StringFromEnv("ARGOCD_APPLICATION_CONTROLLER_LOGLEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	command.Flags().IntVar(&glogLevel, "gloglevel", 0, "Set the glog logging level")
	command.Flags().IntVar(&metricsPort, "metrics-port", common.DefaultPortArgoCDMetrics, "Start metrics server on given port")
	command.Flags().DurationVar(&metricsCacheExpiration, "metrics-cache-expiration", env.ParseDurationFromEnv("ARGOCD_APPLICATION_CONTROLLER_METRICS_CACHE_EXPIRATION", 0*time.Second, 0, math.MaxInt64), "Prometheus metrics cache expiration (disabled  by default. e.g. 24h0m0s)")
	command.Flags().IntVar(&selfHealTimeoutSeconds, "self-heal-timeout-seconds", env.ParseNumFromEnv("ARGOCD_APPLICATION_CONTROLLER_SELF_HEAL_TIMEOUT_SECONDS", 5, 0, math.MaxInt32), "Specifies timeout between application self heal attempts")
	command.Flags().Int64Var(&kubectlParallelismLimit, "kubectl-parallelism-limit", env.ParseInt64FromEnv("ARGOCD_APPLICATION_CONTROLLER_KUBECTL_PARALLELISM_LIMIT", 20, 0, math.MaxInt64), "Number of allowed concurrent kubectl fork/execs. Any value less than 1 means no limit.")
	command.Flags().BoolVar(&repoServerPlaintext, "repo-server-plaintext", env.ParseBoolFromEnv("ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_PLAINTEXT", false), "Disable TLS on connections to repo server")
	command.Flags().BoolVar(&repoServerStrictTLS, "repo-server-strict-tls", env.ParseBoolFromEnv("ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_STRICT_TLS", false), "Whether to use strict validation of the TLS cert presented by the repo server")
	command.Flags().StringSliceVar(&metricsAplicationLabels, "metrics-application-labels", []string{}, "List of Application labels that will be added to the argocd_application_labels metric")
	command.Flags().StringVar(&otlpAddress, "otlp-address", env.StringFromEnv("ARGOCD_APPLICATION_CONTROLLER_OTLP_ADDRESS", ""), "OpenTelemetry collector address to send traces to")
	command.Flags().BoolVar(&otlpInsecure, "otlp-insecure", env.ParseBoolFromEnv("ARGOCD_APPLICATION_CONTROLLER_OTLP_INSECURE", true), "OpenTelemetry collector insecure mode")
	command.Flags().StringToStringVar(&otlpHeaders, "otlp-headers", env.ParseStringToStringFromEnv("ARGOCD_APPLICATION_CONTROLLER_OTLP_HEADERS", map[string]string{}, ","), "List of OpenTelemetry collector extra headers sent with traces, headers are comma-separated key-value pairs(e.g. key1=value1,key2=value2)")
	command.Flags().StringSliceVar(&otlpAttrs, "otlp-attrs", env.StringsFromEnv("ARGOCD_APPLICATION_CONTROLLER_OTLP_ATTRS", []string{}, ","), "List of OpenTelemetry collector extra attrs when send traces, each attribute is separated by a colon(e.g. key:value)")
	command.Flags().StringSliceVar(&applicationNamespaces, "application-namespaces", env.StringsFromEnv("ARGOCD_APPLICATION_NAMESPACES", []string{}, ","), "List of additional namespaces that applications are allowed to be reconciled from")
	command.Flags().BoolVar(&persistResourceHealth, "persist-resource-health", env.ParseBoolFromEnv("ARGOCD_APPLICATION_CONTROLLER_PERSIST_RESOURCE_HEALTH", true), "Enables storing the managed resources health in the Application CRD")
	command.Flags().StringVar(&shardingAlgorithm, "sharding-method", env.StringFromEnv(common.EnvControllerShardingAlgorithm, common.DefaultShardingAlgorithm), "Enables choice of sharding method. Supported sharding methods are : [legacy, round-robin] ")
	// global queue rate limit config
	command.Flags().Int64Var(&workqueueRateLimit.BucketSize, "wq-bucket-size", env.ParseInt64FromEnv("WORKQUEUE_BUCKET_SIZE", 500, 1, math.MaxInt64), "Set Workqueue Rate Limiter Bucket Size, default 500")
	command.Flags().Int64Var(&workqueueRateLimit.BucketQPS, "wq-bucket-qps", env.ParseInt64FromEnv("WORKQUEUE_BUCKET_QPS", 50, 1, math.MaxInt64), "Set Workqueue Rate Limiter Bucket QPS, default 50")
	// individual item rate limit config
	// when WORKQUEUE_FAILURE_COOLDOWN is 0 per item rate limiting is disabled(default)
	command.Flags().DurationVar(&workqueueRateLimit.FailureCoolDown, "wq-cooldown-ns", time.Duration(env.ParseInt64FromEnv("WORKQUEUE_FAILURE_COOLDOWN_NS", 0, 0, (24*time.Hour).Nanoseconds())), "Set Workqueue Per Item Rate Limiter Cooldown duration in ns, default 0(per item rate limiter disabled)")
	command.Flags().DurationVar(&workqueueRateLimit.BaseDelay, "wq-basedelay-ns", time.Duration(env.ParseInt64FromEnv("WORKQUEUE_BASE_DELAY_NS", time.Millisecond.Nanoseconds(), time.Nanosecond.Nanoseconds(), (24*time.Hour).Nanoseconds())), "Set Workqueue Per Item Rate Limiter Base Delay duration in nanoseconds, default 1000000 (1ms)")
	command.Flags().DurationVar(&workqueueRateLimit.MaxDelay, "wq-maxdelay-ns", time.Duration(env.ParseInt64FromEnv("WORKQUEUE_MAX_DELAY_NS", time.Second.Nanoseconds(), 1*time.Millisecond.Nanoseconds(), (24*time.Hour).Nanoseconds())), "Set Workqueue Per Item Rate Limiter Max Delay duration in nanoseconds, default 1000000000 (1s)")
	command.Flags().Float64Var(&workqueueRateLimit.BackoffFactor, "wq-backoff-factor", env.ParseFloat64FromEnv("WORKQUEUE_BACKOFF_FACTOR", 1.5, 0, math.MaxFloat64), "Set Workqueue Per Item Rate Limiter Backoff Factor, default is 1.5")
	command.Flags().BoolVar(&enableDynamicClusterDistribution, "dynamic-cluster-distribution-enabled", env.ParseBoolFromEnv(common.EnvEnableDynamicClusterDistribution, false), "Enables dynamic cluster distribution.")
	command.Flags().BoolVar(&serverSideDiff, "server-side-diff-enabled", env.ParseBoolFromEnv(common.EnvServerSideDiff, false), "Feature flag to enable ServerSide diff. Default (\"false\")")
	cacheSource = appstatecache.AddCacheFlagsToCmd(&command, cacheutil.Options{
		OnClientCreated: func(client *redis.Client) {
			redisClient = client
		},
	})
	return &command
}
