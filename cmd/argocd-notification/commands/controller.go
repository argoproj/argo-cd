package commands

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"

	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/errors"
	service "github.com/argoproj/argo-cd/v2/util/notification/argocd"
	"github.com/argoproj/argo-cd/v2/util/tls"

	notificationscontroller "github.com/argoproj/argo-cd/v2/notification_controller/controller"

	"github.com/argoproj/notifications-engine/pkg/controller"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultMetricsPort = 9001
)

type NotificationsControllerConfig struct {
	cmd                            *cobra.Command
	processorsCount                int
	appLabelSelector               string
	logLevel                       string
	logFormat                      string
	metricsPort                    int
	argocdRepoServer               string
	argocdRepoServerPlaintext      bool
	argocdRepoServerStrictTLS      bool
	configMapName                  string
	secretName                     string
	applicationNamespaces          []string
	selfServiceNotificationEnabled bool
	config                         *rest.Config
	namespace                      string
	clientConfig                   clientcmd.ClientConfig
}

func NewNotificationsControllerConfig(cmd *cobra.Command) *NotificationsControllerConfig {
	return &NotificationsControllerConfig{cmd: cmd}
}

func (c *NotificationsControllerConfig) WithDefaultFlags() *NotificationsControllerConfig {
	c.cmd.Flags().IntVar(&c.processorsCount, "processors-count", 1, "Processors count.")
	c.cmd.Flags().StringVar(&c.appLabelSelector, "app-label-selector", "", "App label selector.")
	c.cmd.Flags().StringVar(&c.namespace, "namespace", "", "Namespace which controller handles. Current namespace if empty.")
	c.cmd.Flags().StringVar(&c.logLevel, "loglevel", env.StringFromEnv("ARGOCD_NOTIFICATIONS_CONTROLLER_LOGLEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	c.cmd.Flags().StringVar(&c.logFormat, "logformat", env.StringFromEnv("ARGOCD_NOTIFICATIONS_CONTROLLER_LOGFORMAT", "text"), "Set the logging format. One of: text|json")
	c.cmd.Flags().IntVar(&c.metricsPort, "metrics-port", defaultMetricsPort, "Metrics port")
	c.cmd.Flags().StringVar(&c.argocdRepoServer, "argocd-repo-server", common.DefaultRepoServerAddr, "Argo CD repo server address")
	c.cmd.Flags().BoolVar(&c.argocdRepoServerPlaintext, "argocd-repo-server-plaintext", env.ParseBoolFromEnv("ARGOCD_NOTIFICATION_CONTROLLER_REPO_SERVER_PLAINTEXT", false), "Use a plaintext client (non-TLS) to connect to repository server")
	c.cmd.Flags().BoolVar(&c.argocdRepoServerStrictTLS, "argocd-repo-server-strict-tls", false, "Perform strict validation of TLS certificates when connecting to repo server")
	c.cmd.Flags().StringVar(&c.configMapName, "config-map-name", "argocd-notifications-cm", "Set notifications ConfigMap name")
	c.cmd.Flags().StringVar(&c.secretName, "secret-name", "argocd-notifications-secret", "Set notifications Secret name")
	c.cmd.Flags().StringSliceVar(&c.applicationNamespaces, "application-namespaces", env.StringsFromEnv("ARGOCD_APPLICATION_NAMESPACES", []string{}, ","), "List of additional namespaces that this controller should send notifications for")
	c.cmd.Flags().BoolVar(&c.selfServiceNotificationEnabled, "self-service-notification-enabled", env.ParseBoolFromEnv("ARGOCD_NOTIFICATION_CONTROLLER_SELF_SERVICE_NOTIFICATION_ENABLED", false), "Allows the Argo CD notification controller to pull notification config from the namespace that the resource is in. This is useful for self-service notification.")
	return c
}

func (c *NotificationsControllerConfig) WithK8sSettings(namespace string, config *rest.Config) *NotificationsControllerConfig {
	c.config = config
	c.namespace = namespace
	return c
}

func (c *NotificationsControllerConfig) WithKubectlFlags() *NotificationsControllerConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := clientcmd.ConfigOverrides{}
	kflags := clientcmd.RecommendedConfigOverrideFlags("")
	c.cmd.PersistentFlags().StringVar(&loadingRules.ExplicitPath, "kubeconfig", "", "Path to a kube config. Only required if out-of-cluster")
	clientcmd.BindOverrideFlags(&overrides, c.cmd.PersistentFlags(), kflags)
	c.clientConfig = clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)
	return c
}

func (c *NotificationsControllerConfig) CreateNotificationsController(ctx context.Context) error {
	vers := common.GetVersion()

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

	vers.LogStartupInfo(
		"ArgoCD Notifications Controller",
		map[string]any{
			"namespace": namespace,
		},
	)

	config.UserAgent = fmt.Sprintf("argocd-notifications-controller/%s (%s)", vers.Version, vers.Platform)
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}
	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}
	level, err := log.ParseLevel(c.logLevel)
	if err != nil {
		return fmt.Errorf("failed to parse log level: %w", err)
	}
	log.SetLevel(level)

	switch strings.ToLower(c.logFormat) {
	case "json":
		log.SetFormatter(&log.JSONFormatter{})
	case "text":
		if os.Getenv("FORCE_LOG_COLORS") == "1" {
			log.SetFormatter(&log.TextFormatter{ForceColors: true})
		}
	default:
		return fmt.Errorf("unknown log format '%s'", c.logFormat)
	}

	tlsConfig := apiclient.TLSConfiguration{
		DisableTLS:       c.argocdRepoServerPlaintext,
		StrictValidation: c.argocdRepoServerStrictTLS,
	}
	if !tlsConfig.DisableTLS && tlsConfig.StrictValidation {
		pool, err := tls.LoadX509CertPool(
			fmt.Sprintf("%s/reposerver/tls/tls.crt", env.StringFromEnv(common.EnvAppConfigPath, common.DefaultAppConfigPath)),
			fmt.Sprintf("%s/reposerver/tls/ca.crt", env.StringFromEnv(common.EnvAppConfigPath, common.DefaultAppConfigPath)),
		)
		if err != nil {
			return fmt.Errorf("failed to load repo-server certificate pool: %w", err)
		}
		tlsConfig.Certificates = pool
	}
	repoClientset := apiclient.NewRepoServerClientset(c.argocdRepoServer, 5, tlsConfig)
	argocdService, err := service.NewArgoCDService(k8sClient, namespace, repoClientset)
	if err != nil {
		return fmt.Errorf("failed to initialize Argo CD service: %w", err)
	}
	defer argocdService.Close()

	registry := controller.NewMetricsRegistry("argocd")
	metrics := http.NewServeMux()
	metrics.Handle("/metrics", promhttp.HandlerFor(prometheus.Gatherers{registry, prometheus.DefaultGatherer}, promhttp.HandlerOpts{}))

	go func() {
		log.Fatal(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", c.metricsPort), metrics))
	}()
	log.Infof("serving metrics on port %d", c.metricsPort)
	log.Infof("loading configuration %d", c.metricsPort)

	ctrl := notificationscontroller.NewController(k8sClient, dynamicClient, argocdService, namespace, c.applicationNamespaces, c.appLabelSelector, registry, c.secretName, c.configMapName, c.selfServiceNotificationEnabled)
	err = ctrl.Init(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize controller: %w", err)
	}

	go ctrl.Run(ctx, c.processorsCount)

	return nil
}

func NewCommand() *cobra.Command {
	var config *NotificationsControllerConfig
	command := cobra.Command{
		Use:   "controller",
		Short: "Starts Argo CD Notifications controller",
		RunE: func(c *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(c.Context())
			defer cancel()
			err := config.CreateNotificationsController(ctx)
			if err != nil {
				return err
			}

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				s := <-sigCh
				log.Printf("got signal %v, attempting graceful shutdown", s)
				cancel()
			}()

			<-c.Context().Done()
			return nil
		},
	}
	config = NewNotificationsControllerConfig(&command).WithDefaultFlags().WithKubectlFlags()
	return &command
}
