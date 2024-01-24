package commands

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/argoproj/pkg/stats"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	reposervercache "github.com/argoproj/argo-cd/v2/reposerver/cache"
	"github.com/argoproj/argo-cd/v2/server"
	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/dex"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/kube"
	"github.com/argoproj/argo-cd/v2/util/templates"
	"github.com/argoproj/argo-cd/v2/util/tls"
	traceutil "github.com/argoproj/argo-cd/v2/util/trace"
)

const (
	failureRetryCountEnv              = "ARGOCD_K8S_RETRY_COUNT"
	failureRetryPeriodMilliSecondsEnv = "ARGOCD_K8S_RETRY_DURATION_MILLISECONDS"
)

var (
	failureRetryCount              = env.ParseNumFromEnv(failureRetryCountEnv, 0, 0, 10)
	failureRetryPeriodMilliSeconds = env.ParseNumFromEnv(failureRetryPeriodMilliSecondsEnv, 100, 0, 1000)
)

// NewCommand returns a new instance of an argocd command
func NewCommand() *cobra.Command {
	var (
		redisClient              *redis.Client
		insecure                 bool
		listenHost               string
		listenPort               int
		metricsHost              string
		metricsPort              int
		otlpAddress              string
		otlpInsecure             bool
		otlpHeaders              map[string]string
		otlpAttrs                []string
		glogLevel                int
		clientConfig             clientcmd.ClientConfig
		repoServerTimeoutSeconds int
		baseHRef                 string
		rootPath                 string
		repoServerAddress        string
		dexServerAddress         string
		disableAuth              bool
		contentTypes             string
		enableGZip               bool
		tlsConfigCustomizerSrc   func() (tls.ConfigCustomizer, error)
		cacheSrc                 func() (*servercache.Cache, error)
		repoServerCacheSrc       func() (*reposervercache.Cache, error)
		frameOptions             string
		contentSecurityPolicy    string
		repoServerPlaintext      bool
		repoServerStrictTLS      bool
		dexServerPlaintext       bool
		dexServerStrictTLS       bool
		staticAssetsDir          string
		applicationNamespaces    []string
		enableProxyExtension     bool
	)
	var command = &cobra.Command{
		Use:               cliName,
		Short:             "Run the ArgoCD API server",
		Long:              "The API server is a gRPC/REST server which exposes the API consumed by the Web UI, CLI, and CI/CD systems.  This command runs API server in the foreground.  It can be configured by following options.",
		DisableAutoGenTag: true,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			vers := common.GetVersion()
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			vers.LogStartupInfo(
				"ArgoCD API Server",
				map[string]any{
					"namespace": namespace,
					"port":      listenPort,
				},
			)

			cli.SetLogFormat(cmdutil.LogFormat)
			cli.SetLogLevel(cmdutil.LogLevel)
			cli.SetGLogLevel(glogLevel)

			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			errors.CheckError(v1alpha1.SetK8SConfigDefaults(config))

			tlsConfigCustomizer, err := tlsConfigCustomizerSrc()
			errors.CheckError(err)
			cache, err := cacheSrc()
			errors.CheckError(err)
			repoServerCache, err := repoServerCacheSrc()
			errors.CheckError(err)

			kubeclientset := kubernetes.NewForConfigOrDie(config)

			appclientsetConfig, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			errors.CheckError(v1alpha1.SetK8SConfigDefaults(appclientsetConfig))
			config.UserAgent = fmt.Sprintf("argocd-server/%s (%s)", vers.Version, vers.Platform)

			if failureRetryCount > 0 {
				appclientsetConfig = kube.AddFailureRetryWrapper(appclientsetConfig, failureRetryCount, failureRetryPeriodMilliSeconds)
			}
			appClientSet := appclientset.NewForConfigOrDie(appclientsetConfig)
			tlsConfig := apiclient.TLSConfiguration{
				DisableTLS:       repoServerPlaintext,
				StrictValidation: repoServerStrictTLS,
			}

			// Load CA information to use for validating connections to the
			// repository server, if strict TLS validation was requested.
			if !repoServerPlaintext && repoServerStrictTLS {
				pool, err := tls.LoadX509CertPool(
					fmt.Sprintf("%s/server/tls/tls.crt", env.StringFromEnv(common.EnvAppConfigPath, common.DefaultAppConfigPath)),
					fmt.Sprintf("%s/server/tls/ca.crt", env.StringFromEnv(common.EnvAppConfigPath, common.DefaultAppConfigPath)),
				)
				if err != nil {
					log.Fatalf("%v", err)
				}
				tlsConfig.Certificates = pool
			}

			dexTlsConfig := &dex.DexTLSConfig{
				DisableTLS:       dexServerPlaintext,
				StrictValidation: dexServerStrictTLS,
			}

			if !dexServerPlaintext && dexServerStrictTLS {
				pool, err := tls.LoadX509CertPool(
					fmt.Sprintf("%s/dex/tls/ca.crt", env.StringFromEnv(common.EnvAppConfigPath, common.DefaultAppConfigPath)),
				)
				if err != nil {
					log.Fatalf("%v", err)
				}
				dexTlsConfig.RootCAs = pool
				cert, err := tls.LoadX509Cert(
					fmt.Sprintf("%s/dex/tls/tls.crt", env.StringFromEnv(common.EnvAppConfigPath, common.DefaultAppConfigPath)),
				)
				if err != nil {
					log.Fatalf("%v", err)
				}
				dexTlsConfig.Certificate = cert.Raw
			}

			repoclientset := apiclient.NewRepoServerClientset(repoServerAddress, repoServerTimeoutSeconds, tlsConfig)
			if rootPath != "" {
				if baseHRef != "" && baseHRef != rootPath {
					log.Warnf("--basehref and --rootpath had conflict: basehref: %s rootpath: %s", baseHRef, rootPath)
				}
				baseHRef = rootPath
			}

			var contentTypesList []string
			if contentTypes != "" {
				contentTypesList = strings.Split(contentTypes, ";")
			}

			argoCDOpts := server.ArgoCDServerOpts{
				Insecure:              insecure,
				ListenPort:            listenPort,
				ListenHost:            listenHost,
				MetricsPort:           metricsPort,
				MetricsHost:           metricsHost,
				Namespace:             namespace,
				BaseHRef:              baseHRef,
				RootPath:              rootPath,
				KubeClientset:         kubeclientset,
				AppClientset:          appClientSet,
				RepoClientset:         repoclientset,
				DexServerAddr:         dexServerAddress,
				DexTLSConfig:          dexTlsConfig,
				DisableAuth:           disableAuth,
				ContentTypes:          contentTypesList,
				EnableGZip:            enableGZip,
				TLSConfigCustomizer:   tlsConfigCustomizer,
				Cache:                 cache,
				RepoServerCache:       repoServerCache,
				XFrameOptions:         frameOptions,
				ContentSecurityPolicy: contentSecurityPolicy,
				RedisClient:           redisClient,
				StaticAssetsDir:       staticAssetsDir,
				ApplicationNamespaces: applicationNamespaces,
				EnableProxyExtension:  enableProxyExtension,
			}

			stats.RegisterStackDumper()
			stats.StartStatsTicker(10 * time.Minute)
			stats.RegisterHeapDumper("memprofile")
			argocd := server.NewServer(ctx, argoCDOpts)
			argocd.Init(ctx)
			lns, err := argocd.Listen()
			errors.CheckError(err)
			for {
				var closer func()
				ctx, cancel := context.WithCancel(ctx)
				if otlpAddress != "" {
					closer, err = traceutil.InitTracer(ctx, "argocd-server", otlpAddress, otlpInsecure, otlpHeaders, otlpAttrs)
					if err != nil {
						log.Fatalf("failed to initialize tracing: %v", err)
					}
				}
				argocd.Run(ctx, lns)
				cancel()
				if closer != nil {
					closer()
				}
			}
		},
		Example: templates.Examples(`
			# Start the Argo CD API server with default settings
			$ argocd-server
				
			# Start the Argo CD API server on a custom port and enable tracing
			$ argocd-server --port 8888 --otlp-address localhost:4317
		`),
	}

	clientConfig = cli.AddKubectlFlagsToCmd(command)
	command.Flags().BoolVar(&insecure, "insecure", env.ParseBoolFromEnv("ARGOCD_SERVER_INSECURE", false), "Run server without TLS")
	command.Flags().StringVar(&staticAssetsDir, "staticassets", env.StringFromEnv("ARGOCD_SERVER_STATIC_ASSETS", "/shared/app"), "Directory path that contains additional static assets")
	command.Flags().StringVar(&baseHRef, "basehref", env.StringFromEnv("ARGOCD_SERVER_BASEHREF", "/"), "Value for base href in index.html. Used if Argo CD is running behind reverse proxy under subpath different from /")
	command.Flags().StringVar(&rootPath, "rootpath", env.StringFromEnv("ARGOCD_SERVER_ROOTPATH", ""), "Used if Argo CD is running behind reverse proxy under subpath different from /")
	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", env.StringFromEnv("ARGOCD_SERVER_LOGFORMAT", "text"), "Set the logging format. One of: text|json")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", env.StringFromEnv("ARGOCD_SERVER_LOG_LEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	command.Flags().IntVar(&glogLevel, "gloglevel", 0, "Set the glog logging level")
	command.Flags().StringVar(&repoServerAddress, "repo-server", env.StringFromEnv("ARGOCD_SERVER_REPO_SERVER", common.DefaultRepoServerAddr), "Repo server address")
	command.Flags().StringVar(&dexServerAddress, "dex-server", env.StringFromEnv("ARGOCD_SERVER_DEX_SERVER", common.DefaultDexServerAddr), "Dex server address")
	command.Flags().BoolVar(&disableAuth, "disable-auth", env.ParseBoolFromEnv("ARGOCD_SERVER_DISABLE_AUTH", false), "Disable client authentication")
	command.Flags().StringVar(&contentTypes, "api-content-types", env.StringFromEnv("ARGOCD_API_CONTENT_TYPES", "application/json"), "Semicolon separated list of allowed content types for non GET api requests. Any content type is allowed if empty.")
	command.Flags().BoolVar(&enableGZip, "enable-gzip", env.ParseBoolFromEnv("ARGOCD_SERVER_ENABLE_GZIP", true), "Enable GZIP compression")
	command.AddCommand(cli.NewVersionCmd(cliName))
	command.Flags().StringVar(&listenHost, "address", env.StringFromEnv("ARGOCD_SERVER_LISTEN_ADDRESS", common.DefaultAddressAPIServer), "Listen on given address")
	command.Flags().IntVar(&listenPort, "port", common.DefaultPortAPIServer, "Listen on given port")
	command.Flags().StringVar(&metricsHost, env.StringFromEnv("ARGOCD_SERVER_METRICS_LISTEN_ADDRESS", "metrics-address"), common.DefaultAddressAPIServerMetrics, "Listen for metrics on given address")
	command.Flags().IntVar(&metricsPort, "metrics-port", common.DefaultPortArgoCDAPIServerMetrics, "Start metrics on given port")
	command.Flags().StringVar(&otlpAddress, "otlp-address", env.StringFromEnv("ARGOCD_SERVER_OTLP_ADDRESS", ""), "OpenTelemetry collector address to send traces to")
	command.Flags().BoolVar(&otlpInsecure, "otlp-insecure", env.ParseBoolFromEnv("ARGOCD_SERVER_OTLP_INSECURE", true), "OpenTelemetry collector insecure mode")
	command.Flags().StringToStringVar(&otlpHeaders, "otlp-headers", env.ParseStringToStringFromEnv("ARGOCD_SERVER_OTLP_HEADERS", map[string]string{}, ","), "List of OpenTelemetry collector extra headers sent with traces, headers are comma-separated key-value pairs(e.g. key1=value1,key2=value2)")
	command.Flags().StringSliceVar(&otlpAttrs, "otlp-attrs", env.StringsFromEnv("ARGOCD_SERVER_OTLP_ATTRS", []string{}, ","), "List of OpenTelemetry collector extra attrs when send traces, each attribute is separated by a colon(e.g. key:value)")
	command.Flags().IntVar(&repoServerTimeoutSeconds, "repo-server-timeout-seconds", env.ParseNumFromEnv("ARGOCD_SERVER_REPO_SERVER_TIMEOUT_SECONDS", 60, 0, math.MaxInt64), "Repo server RPC call timeout seconds.")
	command.Flags().StringVar(&frameOptions, "x-frame-options", env.StringFromEnv("ARGOCD_SERVER_X_FRAME_OPTIONS", "sameorigin"), "Set X-Frame-Options header in HTTP responses to `value`. To disable, set to \"\".")
	command.Flags().StringVar(&contentSecurityPolicy, "content-security-policy", env.StringFromEnv("ARGOCD_SERVER_CONTENT_SECURITY_POLICY", "frame-ancestors 'self';"), "Set Content-Security-Policy header in HTTP responses to `value`. To disable, set to \"\".")
	command.Flags().BoolVar(&repoServerPlaintext, "repo-server-plaintext", env.ParseBoolFromEnv("ARGOCD_SERVER_REPO_SERVER_PLAINTEXT", false), "Use a plaintext client (non-TLS) to connect to repository server")
	command.Flags().BoolVar(&repoServerStrictTLS, "repo-server-strict-tls", env.ParseBoolFromEnv("ARGOCD_SERVER_REPO_SERVER_STRICT_TLS", false), "Perform strict validation of TLS certificates when connecting to repo server")
	command.Flags().BoolVar(&dexServerPlaintext, "dex-server-plaintext", env.ParseBoolFromEnv("ARGOCD_SERVER_DEX_SERVER_PLAINTEXT", false), "Use a plaintext client (non-TLS) to connect to dex server")
	command.Flags().BoolVar(&dexServerStrictTLS, "dex-server-strict-tls", env.ParseBoolFromEnv("ARGOCD_SERVER_DEX_SERVER_STRICT_TLS", false), "Perform strict validation of TLS certificates when connecting to dex server")
	command.Flags().StringSliceVar(&applicationNamespaces, "application-namespaces", env.StringsFromEnv("ARGOCD_APPLICATION_NAMESPACES", []string{}, ","), "List of additional namespaces where application resources can be managed in")
	command.Flags().BoolVar(&enableProxyExtension, "enable-proxy-extension", env.ParseBoolFromEnv("ARGOCD_SERVER_ENABLE_PROXY_EXTENSION", false), "Enable Proxy Extension feature")
	tlsConfigCustomizerSrc = tls.AddTLSFlagsToCmd(command)
	cacheSrc = servercache.AddCacheFlagsToCmd(command, cacheutil.Options{
		OnClientCreated: func(client *redis.Client) {
			redisClient = client
		},
	})
	repoServerCacheSrc = reposervercache.AddCacheFlagsToCmd(command, cacheutil.Options{FlagPrefix: "repo-server-"})
	return command
}
