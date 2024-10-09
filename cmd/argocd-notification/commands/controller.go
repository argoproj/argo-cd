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

func addK8SFlagsToCmd(cmd *cobra.Command) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := clientcmd.ConfigOverrides{}
	kflags := clientcmd.RecommendedConfigOverrideFlags("")
	cmd.PersistentFlags().StringVar(&loadingRules.ExplicitPath, "kubeconfig", "", "Path to a kube config. Only required if out-of-cluster")
	clientcmd.BindOverrideFlags(&overrides, cmd.PersistentFlags(), kflags)
	return clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)
}

func NewCommand() *cobra.Command {
	var (
		clientConfig                   clientcmd.ClientConfig
		processorsCount                int
		namespace                      string
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
	)
	command := cobra.Command{
		Use:   "controller",
		Short: "Starts Argo CD Notifications controller",
		RunE: func(c *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			vers := common.GetVersion()
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			vers.LogStartupInfo(
				"ArgoCD Notifications Controller",
				map[string]any{
					"namespace": namespace,
				},
			)

			restConfig, err := clientConfig.ClientConfig()
			if err != nil {
				return fmt.Errorf("failed to create REST client config: %w", err)
			}
			restConfig.UserAgent = fmt.Sprintf("argocd-notifications-controller/%s (%s)", vers.Version, vers.Platform)
			dynamicClient, err := dynamic.NewForConfig(restConfig)
			if err != nil {
				return fmt.Errorf("failed to create dynamic client: %w", err)
			}
			k8sClient, err := kubernetes.NewForConfig(restConfig)
			if err != nil {
				return fmt.Errorf("failed to create Kubernetes client: %w", err)
			}
			if namespace == "" {
				namespace, _, err = clientConfig.Namespace()
				if err != nil {
					return fmt.Errorf("failed to determine controller's host namespace: %w", err)
				}
			}
			level, err := log.ParseLevel(logLevel)
			if err != nil {
				return fmt.Errorf("failed to parse log level: %w", err)
			}
			log.SetLevel(level)

			switch strings.ToLower(logFormat) {
			case "json":
				log.SetFormatter(&log.JSONFormatter{})
			case "text":
				if os.Getenv("FORCE_LOG_COLORS") == "1" {
					log.SetFormatter(&log.TextFormatter{ForceColors: true})
				}
			default:
				return fmt.Errorf("unknown log format '%s'", logFormat)
			}

			tlsConfig := apiclient.TLSConfiguration{
				DisableTLS:       argocdRepoServerPlaintext,
				StrictValidation: argocdRepoServerStrictTLS,
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
			repoClientset := apiclient.NewRepoServerClientset(argocdRepoServer, 5, tlsConfig)
			argocdService, err := service.NewArgoCDService(k8sClient, namespace, repoClientset)
			if err != nil {
				return fmt.Errorf("failed to initialize Argo CD service: %w", err)
			}
			defer argocdService.Close()

			registry := controller.NewMetricsRegistry("argocd")
			http.Handle("/metrics", promhttp.HandlerFor(prometheus.Gatherers{registry, prometheus.DefaultGatherer}, promhttp.HandlerOpts{}))

			go func() {
				log.Fatal(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", metricsPort), http.DefaultServeMux))
			}()
			log.Infof("serving metrics on port %d", metricsPort)
			log.Infof("loading configuration %d", metricsPort)

			ctrl := notificationscontroller.NewController(k8sClient, dynamicClient, argocdService, namespace, applicationNamespaces, appLabelSelector, registry, secretName, configMapName, selfServiceNotificationEnabled)
			err = ctrl.Init(ctx)
			if err != nil {
				return fmt.Errorf("failed to initialize controller: %w", err)
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

			go ctrl.Run(ctx, processorsCount)
			<-ctx.Done()
			return nil
		},
	}
	clientConfig = addK8SFlagsToCmd(&command)
	command.Flags().IntVar(&processorsCount, "processors-count", 1, "Processors count.")
	command.Flags().StringVar(&appLabelSelector, "app-label-selector", "", "App label selector.")
	command.Flags().StringVar(&namespace, "namespace", "", "Namespace which controller handles. Current namespace if empty.")
	command.Flags().StringVar(&logLevel, "loglevel", env.StringFromEnv("ARGOCD_NOTIFICATIONS_CONTROLLER_LOGLEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	command.Flags().StringVar(&logFormat, "logformat", env.StringFromEnv("ARGOCD_NOTIFICATIONS_CONTROLLER_LOGFORMAT", "text"), "Set the logging format. One of: text|json")
	command.Flags().IntVar(&metricsPort, "metrics-port", defaultMetricsPort, "Metrics port")
	command.Flags().StringVar(&argocdRepoServer, "argocd-repo-server", common.DefaultRepoServerAddr, "Argo CD repo server address")
	command.Flags().BoolVar(&argocdRepoServerPlaintext, "argocd-repo-server-plaintext", env.ParseBoolFromEnv("ARGOCD_NOTIFICATION_CONTROLLER_REPO_SERVER_PLAINTEXT", false), "Use a plaintext client (non-TLS) to connect to repository server")
	command.Flags().BoolVar(&argocdRepoServerStrictTLS, "argocd-repo-server-strict-tls", false, "Perform strict validation of TLS certificates when connecting to repo server")
	command.Flags().StringVar(&configMapName, "config-map-name", "argocd-notifications-cm", "Set notifications ConfigMap name")
	command.Flags().StringVar(&secretName, "secret-name", "argocd-notifications-secret", "Set notifications Secret name")
	command.Flags().StringSliceVar(&applicationNamespaces, "application-namespaces", env.StringsFromEnv("ARGOCD_APPLICATION_NAMESPACES", []string{}, ","), "List of additional namespaces that this controller should send notifications for")
	command.Flags().BoolVar(&selfServiceNotificationEnabled, "self-service-notification-enabled", env.ParseBoolFromEnv("ARGOCD_NOTIFICATION_CONTROLLER_SELF_SERVICE_NOTIFICATION_ENABLED", false), "Allows the Argo CD notification controller to pull notification config from the namespace that the resource is in. This is useful for self-service notification.")
	return &command
}
