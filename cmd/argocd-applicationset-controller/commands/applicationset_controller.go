package command

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	logutils "github.com/argoproj/argo-cd/v2/util/log"
	"github.com/argoproj/argo-cd/v2/util/tls"

	"github.com/argoproj/argo-cd/v2/applicationset/controllers"
	"github.com/argoproj/argo-cd/v2/applicationset/generators"
	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	"github.com/argoproj/argo-cd/v2/applicationset/webhook"
	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/github_app"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	appsetmetrics "github.com/argoproj/argo-cd/v2/applicationset/metrics"
	"github.com/argoproj/argo-cd/v2/applicationset/services"
	appv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/errors"
	argosettings "github.com/argoproj/argo-cd/v2/util/settings"
)

var gitSubmoduleEnabled = env.ParseBoolFromEnv(common.EnvGitSubmoduleEnabled, true)

type ApplicationSetControllerConfig struct {
	cmd                          *cobra.Command
	config                       *rest.Config
	namespace                    string
	clientConfig                 clientcmd.ClientConfig
	metricsAddr                  string
	probeBindAddr                string
	webhookAddr                  string
	enableLeaderElection         bool
	applicationSetNamespaces     []string
	argocdRepoServer             string
	policy                       string
	enablePolicyOverride         bool
	debugLog                     bool
	dryRun                       bool
	enableProgressiveSyncs       bool
	enableNewGitFileGlobbing     bool
	repoServerPlaintext          bool
	repoServerStrictTLS          bool
	repoServerTimeoutSeconds     int
	maxConcurrentReconciliations int
	scmRootCAPath                string
	allowedScmProviders          []string
	globalPreservedAnnotations   []string
	globalPreservedLabels        []string
	metricsApplicationsetLabels   []string
	enableScmProviders           bool
	webhookParallelism           int
	tokenRefStrictMode           bool
}

func NewApplicationSetControllerConfig(cmd *cobra.Command) *ApplicationSetControllerConfig {
	return &ApplicationSetControllerConfig{cmd: cmd}
}

func (c *ApplicationSetControllerConfig) WithDefaultFlags() *ApplicationSetControllerConfig {
	c.cmd.Flags().StringVar(&c.metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	c.cmd.Flags().StringVar(&c.probeBindAddr, "probe-addr", ":8081", "The address the probe endpoint binds to.")
	c.cmd.Flags().StringVar(&c.webhookAddr, "webhook-addr", ":7000", "The address the webhook endpoint binds to.")
	c.cmd.Flags().BoolVar(&c.enableLeaderElection, "enable-leader-election", env.ParseBoolFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_LEADER_ELECTION", false),
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	c.cmd.Flags().StringSliceVar(&c.applicationSetNamespaces, "applicationset-namespaces", env.StringsFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_NAMESPACES", []string{}, ","), "Argo CD applicationset namespaces")
	c.cmd.Flags().StringVar(&c.argocdRepoServer, "argocd-repo-server", env.StringFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_REPO_SERVER", common.DefaultRepoServerAddr), "Argo CD repo server address")
	c.cmd.Flags().StringVar(&c.policy, "policy", env.StringFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_POLICY", ""), "Modify how application is synced between the generator and the cluster. Default is 'sync' (create &c. update &c. delete), options: 'create-only', 'create-update' (no deletion), 'create-delete' (no update)")
	c.cmd.Flags().BoolVar(&c.enablePolicyOverride, "enable-policy-override", env.ParseBoolFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_POLICY_OVERRIDE", c.policy == ""), "For security reason if 'policy' is set, it is not possible to override it at applicationSet level. 'allow-policy-override' allows user to define their own policy")
	c.cmd.Flags().BoolVar(&c.debugLog, "debug", env.ParseBoolFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_DEBUG", false), "Print debug logs. Takes precedence over loglevel")
	c.cmd.Flags().StringSliceVar(&c.allowedScmProviders, "allowed-scm-providers", env.StringsFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_ALLOWED_SCM_PROVIDERS", []string{}, ","), "The list of allowed custom SCM provider API URLs. This restriction does not apply to SCM or PR generators which do not accept a custom API URL. (Default: Empty = all)")
	c.cmd.Flags().BoolVar(&c.enableScmProviders, "enable-scm-providers", env.ParseBoolFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_SCM_PROVIDERS", true), "Enable retrieving information from SCM providers, used by the SCM and PR generators (Default: true)")
	c.cmd.Flags().BoolVar(&c.dryRun, "dry-run", env.ParseBoolFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_DRY_RUN", false), "Enable dry run mode")
	c.cmd.Flags().BoolVar(&c.tokenRefStrictMode, "token-ref-strict-mode", env.ParseBoolFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_TOKENREF_STRICT_MODE", false), fmt.Sprintf("Set to true to require secrets referenced by SCM providers to have the %s=%s label set (Default: false)", common.LabelKeySecretType, common.LabelValueSecretTypeSCMCreds))
	c.cmd.Flags().BoolVar(&c.enableProgressiveSyncs, "enable-progressive-syncs", env.ParseBoolFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS", false), "Enable use of the experimental progressive syncs feature.")
	c.cmd.Flags().BoolVar(&c.enableNewGitFileGlobbing, "enable-new-git-file-globbing", env.ParseBoolFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_NEW_GIT_FILE_GLOBBING", false), "Enable new globbing in Git files generator.")
	c.cmd.Flags().BoolVar(&c.repoServerPlaintext, "repo-server-plaintext", env.ParseBoolFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_REPO_SERVER_PLAINTEXT", false), "Disable TLS on connections to repo server")
	c.cmd.Flags().BoolVar(&c.repoServerStrictTLS, "repo-server-strict-tls", env.ParseBoolFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_REPO_SERVER_STRICT_TLS", false), "Whether to use strict validation of the TLS cert presented by the repo server")
	c.cmd.Flags().IntVar(&c.repoServerTimeoutSeconds, "repo-server-timeout-seconds", env.ParseNumFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_REPO_SERVER_TIMEOUT_SECONDS", 60, 0, math.MaxInt64), "Repo server RPC call timeout seconds.")
	c.cmd.Flags().IntVar(&c.maxConcurrentReconciliations, "concurrent-reconciliations", env.ParseNumFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_CONCURRENT_RECONCILIATIONS", 10, 1, 100), "Max concurrent reconciliations limit for the controller")
	c.cmd.Flags().StringVar(&c.scmRootCAPath, "scm-root-ca-path", env.StringFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_SCM_ROOT_CA_PATH", ""), "Provide Root CA Path for self-signed TLS Certificates")
	c.cmd.Flags().StringSliceVar(&c.globalPreservedAnnotations, "preserved-annotations", env.StringsFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_GLOBAL_PRESERVED_ANNOTATIONS", []string{}, ","), "Sets global preserved field values for annotations")
	c.cmd.Flags().StringSliceVar(&c.globalPreservedLabels, "preserved-labels", env.StringsFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_GLOBAL_PRESERVED_LABELS", []string{}, ","), "Sets global preserved field values for labels")
	c.cmd.Flags().IntVar(&c.webhookParallelism, "webhook-parallelism-limit", env.ParseNumFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_WEBHOOK_PARALLELISM_LIMIT", 50, 1, 1000), "Number of webhook requests processed concurrently")
	c.cmd.Flags().StringSliceVar(&c.metricsApplicationsetLabels, "metrics-applicationset-labels", []string{}, "List of Application labels that will be added to the argocd_applicationset_labels metric")
	return c
}

func (c *ApplicationSetControllerConfig) WithKubectlFlags() *ApplicationSetControllerConfig {
	c.clientConfig = cli.AddKubectlFlagsToSet(c.cmd.PersistentFlags())
	return c
}

func (c *ApplicationSetControllerConfig) WithK8sSettings(namespace string, config *rest.Config) *ApplicationSetControllerConfig {
	c.config = config
	c.namespace = namespace
	return c
}

func (c *ApplicationSetControllerConfig) CreateApplicationSetController(ctx context.Context) (manager.Manager, error) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)

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

	c.applicationSetNamespaces = append(c.applicationSetNamespaces, namespace)

	vers.LogStartupInfo(
		"ArgoCD ApplicationSet Controller",
		map[string]any{
			"namespace": namespace,
		},
	)

	ctrl.SetLogger(logutils.NewLogrusLogger(logutils.NewWithCurrentConfig()))
	config.UserAgent = fmt.Sprintf("argocd-applicationset-controller/%s (%s)", vers.Version, vers.Platform)

	policyObj, exists := utils.Policies[c.policy]
	if !exists {
		log.Error("Policy value can be: sync, create-only, create-update, create-delete, default value: sync")
		os.Exit(1)
	}

	// By default, watch all namespaces
	watchedNamespace := ""

	// If the applicationset-namespaces contains only one namespace it corresponds to the current namespace
	if len(c.applicationSetNamespaces) == 1 {
		watchedNamespace = (c.applicationSetNamespaces)[0]
	} else if c.enableScmProviders && len(c.allowedScmProviders) == 0 {
		log.Error("When enabling applicationset in any namespace using applicationset-namespaces, you must either set --enable-scm-providers=false or specify --allowed-scm-providers")
		os.Exit(1)
	}

	var cacheOpt ctrlcache.Options

	if watchedNamespace != "" {
		cacheOpt = ctrlcache.Options{
			DefaultNamespaces: map[string]ctrlcache.Config{
				watchedNamespace: {},
			},
		}
	}

	cfg := ctrl.GetConfigOrDie()
	err := appv1alpha1.SetK8SConfigDefaults(cfg)
	if err != nil {
		log.Error(err, "Unable to apply K8s REST config defaults")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: c.metricsAddr,
		},
		Cache:                  cacheOpt,
		HealthProbeBindAddress: c.probeBindAddr,
		LeaderElection:         c.enableLeaderElection,
		LeaderElectionID:       "58ac56fa.applicationsets.argoproj.io",
		Client: ctrlclient.Options{
			DryRun: &c.dryRun,
		},
	})
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}
	dynamicClient, err := dynamic.NewForConfig(mgr.GetConfig())
	errors.CheckError(err)
	k8sClient, err := kubernetes.NewForConfig(mgr.GetConfig())
	errors.CheckError(err)

	argoSettingsMgr := argosettings.NewSettingsManager(ctx, k8sClient, namespace)
	argoCDDB := db.NewDB(namespace, argoSettingsMgr, k8sClient)

	scmConfig := generators.NewSCMConfig(c.scmRootCAPath, c.allowedScmProviders, c.enableScmProviders, github_app.NewAuthCredentials(argoCDDB.(db.RepoCredsDB)), c.tokenRefStrictMode)

	tlsConfig := apiclient.TLSConfiguration{
		DisableTLS:       c.repoServerPlaintext,
		StrictValidation: c.repoServerStrictTLS,
	}

	if !c.repoServerPlaintext && c.repoServerStrictTLS {
		pool, err := tls.LoadX509CertPool(
			fmt.Sprintf("%s/reposerver/tls/tls.crt", env.StringFromEnv(common.EnvAppConfigPath, common.DefaultAppConfigPath)),
			fmt.Sprintf("%s/reposerver/tls/ca.crt", env.StringFromEnv(common.EnvAppConfigPath, common.DefaultAppConfigPath)),
		)
		errors.CheckError(err)
		tlsConfig.Certificates = pool
	}

	repoClientset := apiclient.NewRepoServerClientset(c.argocdRepoServer, c.repoServerTimeoutSeconds, tlsConfig)
	argoCDService, err := services.NewArgoCDService(argoCDDB.GetRepository, gitSubmoduleEnabled, repoClientset, c.enableNewGitFileGlobbing)
	errors.CheckError(err)

	topLevelGenerators := generators.GetGenerators(ctx, mgr.GetClient(), k8sClient, namespace, argoCDService, dynamicClient, scmConfig)

	// start a webhook server that listens to incoming webhook payloads
	webhookHandler, err := webhook.NewWebhookHandler(namespace, c.webhookParallelism, argoSettingsMgr, mgr.GetClient(), topLevelGenerators)
	if err != nil {
		log.Error(err, "failed to create webhook handler")
	}
	if webhookHandler != nil {
		startWebhookServer(webhookHandler, c.webhookAddr)
	}

	metrics := appsetmetrics.NewApplicationsetMetrics(
		utils.NewAppsetLister(mgr.GetClient()),
		c.metricsApplicationsetLabels,
		func(appset *appv1alpha1.ApplicationSet) bool {
			return utils.IsNamespaceAllowed(c.applicationSetNamespaces, appset.Namespace)
		})

	if err = (&controllers.ApplicationSetReconciler{
		Generators:                 topLevelGenerators,
		Client:                     mgr.GetClient(),
		Scheme:                     mgr.GetScheme(),
		Recorder:                   mgr.GetEventRecorderFor("applicationset-controller"),
		Renderer:                   &utils.Render{},
		Policy:                     policyObj,
		EnablePolicyOverride:       c.enablePolicyOverride,
		KubeClientset:              k8sClient,
		ArgoDB:                     argoCDDB,
		ArgoCDNamespace:            namespace,
		ApplicationSetNamespaces:   c.applicationSetNamespaces,
		EnableProgressiveSyncs:     c.enableProgressiveSyncs,
		SCMRootCAPath:              c.scmRootCAPath,
		GlobalPreservedAnnotations: c.globalPreservedAnnotations,
		GlobalPreservedLabels:      c.globalPreservedLabels,
		Metrics:                    &metrics,
	}).SetupWithManager(mgr, c.enableProgressiveSyncs, c.maxConcurrentReconciliations); err != nil {
		log.Error(err, "unable to create controller", "controller", "ApplicationSet")
		os.Exit(1)
	}

	return mgr, nil
}

func NewCommand() *cobra.Command {
	var config *ApplicationSetControllerConfig
	command := cobra.Command{
		Use:   "controller",
		Short: "Starts Argo CD ApplicationSet controller",
		RunE: func(c *cobra.Command, args []string) error {
			log.Info("Starting manager")
			mgr, err := config.CreateApplicationSetController(c.Context())
			if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
				log.Error(err, "problem running manager")
				os.Exit(1)
			}
			return err
		},
	}

	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", env.StringFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_LOGFORMAT", "text"), "Set the logging format. One of: text|json")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", env.StringFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_LOGLEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	config = NewApplicationSetControllerConfig(&command).WithDefaultFlags().WithKubectlFlags()
	return &command
}

func startWebhookServer(webhookHandler *webhook.WebhookHandler, webhookAddr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/webhook", webhookHandler.Handler)
	go func() {
		log.Infof("Starting webhook server %s", webhookAddr)
		err := http.ListenAndServe(webhookAddr, mux)
		if err != nil {
			log.Error(err, "failed to start webhook server")
			os.Exit(1)
		}
	}()
}
