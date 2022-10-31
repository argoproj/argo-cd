package command

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/argoproj/pkg/stats"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/argoproj/argo-cd/v2/applicationset/controllers"
	"github.com/argoproj/argo-cd/v2/applicationset/generators"
	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	"github.com/argoproj/argo-cd/v2/applicationset/webhook"
	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/reposerver/askpass"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/github_app"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v2/applicationset/services"
	appsetv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/errors"
	argosettings "github.com/argoproj/argo-cd/v2/util/settings"
)

// TODO: load this using Cobra.
func getSubmoduleEnabled() bool {
	return env.ParseBoolFromEnv(common.EnvGitSubmoduleEnabled, true)
}

func NewCommand() *cobra.Command {
	var (
		clientConfig         clientcmd.ClientConfig
		metricsAddr          string
		probeBindAddr        string
		webhookAddr          string
		enableLeaderElection bool
		namespace            string
		argocdRepoServer     string
		policy               string
		debugLog             bool
		dryRun               bool
	)
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appsetv1alpha1.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	var command = cobra.Command{
		Use:   "controller",
		Short: "Starts Argo CD ApplicationSet controller",
		RunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()

			vers := common.GetVersion()
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			vers.LogStartupInfo(
				"ArgoCD ApplicationSet Controller",
				map[string]any{
					"namespace": namespace,
				},
			)

			cli.SetLogFormat(cmdutil.LogFormat)
			cli.SetLogLevel(cmdutil.LogLevel)

			restConfig, err := clientConfig.ClientConfig()
			if err != nil {
				return err
			}

			restConfig.UserAgent = fmt.Sprintf("argocd-applicationset-controller/%s (%s)", vers.Version, vers.Platform)

			policyObj, exists := utils.Policies[policy]
			if !exists {
				log.Info("Policy value can be: sync, create-only, create-update")
				os.Exit(1)
			}

			mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
				Scheme:             scheme,
				MetricsBindAddress: metricsAddr,
				// Our cache and thus watches and client queries are restricted to the namespace we're running in. This assumes
				// the applicationset controller is in the same namespace as argocd, which should be the same namespace of
				// all cluster Secrets and Applications we interact with.
				NewCache:               cache.MultiNamespacedCacheBuilder([]string{namespace}),
				HealthProbeBindAddress: probeBindAddr,
				Port:                   9443,
				LeaderElection:         enableLeaderElection,
				LeaderElectionID:       "58ac56fa.applicationsets.argoproj.io",
				DryRunClient:           dryRun,
			})
			if err != nil {
				log.Error(err, "unable to start manager")
				os.Exit(1)
			}
			dynamicClient, err := dynamic.NewForConfig(mgr.GetConfig())
			if err != nil {
				return err
			}
			k8sClient, err := kubernetes.NewForConfig(mgr.GetConfig())
			if err != nil {
				return err
			}
			argoSettingsMgr := argosettings.NewSettingsManager(ctx, k8sClient, namespace)
			appSetConfig := appclientset.NewForConfigOrDie(mgr.GetConfig())
			argoCDDB := db.NewDB(namespace, argoSettingsMgr, k8sClient)

			askPassServer := askpass.NewServer()
			scmAuth := generators.SCMAuthProviders{
				GitHubApps: github_app.NewAuthCredentials(argoCDDB.(db.RepoCredsDB)),
			}
			terminalGenerators := map[string]generators.Generator{
				"List":                    generators.NewListGenerator(),
				"Clusters":                generators.NewClusterGenerator(mgr.GetClient(), ctx, k8sClient, namespace),
				"Git":                     generators.NewGitGenerator(services.NewArgoCDService(argoCDDB, askPassServer, getSubmoduleEnabled())),
				"SCMProvider":             generators.NewSCMProviderGenerator(mgr.GetClient(), scmAuth),
				"ClusterDecisionResource": generators.NewDuckTypeGenerator(ctx, dynamicClient, k8sClient, namespace),
				"PullRequest":             generators.NewPullRequestGenerator(mgr.GetClient(), scmAuth),
			}

			nestedGenerators := map[string]generators.Generator{
				"List":                    terminalGenerators["List"],
				"Clusters":                terminalGenerators["Clusters"],
				"Git":                     terminalGenerators["Git"],
				"SCMProvider":             terminalGenerators["SCMProvider"],
				"ClusterDecisionResource": terminalGenerators["ClusterDecisionResource"],
				"PullRequest":             terminalGenerators["PullRequest"],
				"Matrix":                  generators.NewMatrixGenerator(terminalGenerators),
				"Merge":                   generators.NewMergeGenerator(terminalGenerators),
			}

			topLevelGenerators := map[string]generators.Generator{
				"List":                    terminalGenerators["List"],
				"Clusters":                terminalGenerators["Clusters"],
				"Git":                     terminalGenerators["Git"],
				"SCMProvider":             terminalGenerators["SCMProvider"],
				"ClusterDecisionResource": terminalGenerators["ClusterDecisionResource"],
				"PullRequest":             terminalGenerators["PullRequest"],
				"Matrix":                  generators.NewMatrixGenerator(nestedGenerators),
				"Merge":                   generators.NewMergeGenerator(nestedGenerators),
			}

			// start a webhook server that listens to incoming webhook payloads
			webhookHandler, err := webhook.NewWebhookHandler(namespace, argoSettingsMgr, mgr.GetClient(), topLevelGenerators)
			if err != nil {
				log.Error(err, "failed to create webhook handler")
			}
			if webhookHandler != nil {
				startWebhookServer(webhookHandler, webhookAddr)
			}

			go func() { errors.CheckError(askPassServer.Run(askpass.SocketPath)) }()
			if err = (&controllers.ApplicationSetReconciler{
				Generators:       topLevelGenerators,
				Client:           mgr.GetClient(),
				Scheme:           mgr.GetScheme(),
				Recorder:         mgr.GetEventRecorderFor("applicationset-controller"),
				Renderer:         &utils.Render{},
				Policy:           policyObj,
				ArgoAppClientset: appSetConfig,
				KubeClientset:    k8sClient,
				ArgoDB:           argoCDDB,
			}).SetupWithManager(mgr); err != nil {
				log.Error(err, "unable to create controller", "controller", "ApplicationSet")
				os.Exit(1)
			}

			stats.StartStatsTicker(10 * time.Minute)
			log.Info("Starting manager")
			if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
				log.Error(err, "problem running manager")
				os.Exit(1)
			}
			return nil
		},
	}
	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.Flags().StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	command.Flags().StringVar(&probeBindAddr, "probe-addr", ":8081", "The address the probe endpoint binds to.")
	command.Flags().StringVar(&webhookAddr, "webhook-addr", ":7000", "The address the webhook endpoint binds to.")
	command.Flags().BoolVar(&enableLeaderElection, "enable-leader-election", env.ParseBoolFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_LEADER_ELECTION", false),
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	command.Flags().StringVar(&namespace, "namespace", env.StringFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_NAMESPACE", ""), "Argo CD repo namespace (default: argocd)")
	command.Flags().StringVar(&argocdRepoServer, "argocd-repo-server", env.StringFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_REPO_SERVER", common.DefaultRepoServerAddr), "Argo CD repo server address")
	command.Flags().StringVar(&policy, "policy", env.StringFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_POLICY", "sync"), "Modify how application is synced between the generator and the cluster. Default is 'sync' (create & update & delete), options: 'create-only', 'create-update' (no deletion)")
	command.Flags().BoolVar(&debugLog, "debug", env.ParseBoolFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_DEBUG", false), "Print debug logs. Takes precedence over loglevel")
	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", env.StringFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_LOGFORMAT", "text"), "Set the logging format. One of: text|json")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", env.StringFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_LOGLEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	command.Flags().BoolVar(&dryRun, "dry-run", env.ParseBoolFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_DRY_RUN", false), "Enable dry run mode")
	return &command
}

func startWebhookServer(webhookHandler *webhook.WebhookHandler, webhookAddr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/webhook", webhookHandler.Handler)
	go func() {
		log.Info("Starting webhook server")
		err := http.ListenAndServe(webhookAddr, mux)
		if err != nil {
			log.Error(err, "failed to start webhook server")
			os.Exit(1)
		}
	}()
}
