package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	kubecache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/controller"
	"github.com/argoproj/argo-cd/v2/controller/cache"
	"github.com/argoproj/argo-cd/v2/controller/metrics"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	appinformers "github.com/argoproj/argo-cd/v2/pkg/client/informers/externalversions"
	argocdclient "github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/argo"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/config"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/io"
	kubeutil "github.com/argoproj/argo-cd/v2/util/kube"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

func NewAppCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "app",
		Short: "Manage applications configuration",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}

	command.AddCommand(NewGenAppSpecCommand())
	command.AddCommand(NewReconcileCommand())
	command.AddCommand(NewDiffReconcileResults())
	return command
}

// NewGenAppSpecCommand generates declarative configuration file for given application
func NewGenAppSpecCommand() *cobra.Command {
	var (
		appOpts      cmdutil.AppOptions
		fileURL      string
		appName      string
		labels       []string
		outputFormat string
		annotations  []string
		inline       bool
	)
	var command = &cobra.Command{
		Use:   "generate-spec APPNAME",
		Short: "Generate declarative config for an application",
		Example: `
	# Generate declarative config for a directory app
	argocd admin app generate-spec guestbook --repo https://github.com/argoproj/argocd-example-apps.git --path guestbook --dest-namespace default --dest-server https://kubernetes.default.svc --directory-recurse

	# Generate declarative config for a Jsonnet app
	argocd admin app generate-spec jsonnet-guestbook --repo https://github.com/argoproj/argocd-example-apps.git --path jsonnet-guestbook --dest-namespace default --dest-server https://kubernetes.default.svc --jsonnet-ext-str replicas=2

	# Generate declarative config for a Helm app
	argocd admin app generate-spec helm-guestbook --repo https://github.com/argoproj/argocd-example-apps.git --path helm-guestbook --dest-namespace default --dest-server https://kubernetes.default.svc --helm-set replicaCount=2

	# Generate declarative config for a Helm app from a Helm repo
	argocd admin app generate-spec nginx-ingress --repo https://charts.helm.sh/stable --helm-chart nginx-ingress --revision 1.24.3 --dest-namespace default --dest-server https://kubernetes.default.svc

	# Generate declarative config for a Kustomize app
	argocd admin app generate-spec kustomize-guestbook --repo https://github.com/argoproj/argocd-example-apps.git --path kustomize-guestbook --dest-namespace default --dest-server https://kubernetes.default.svc --kustomize-image gcr.io/heptio-images/ks-guestbook-demo:0.1

	# Generate declarative config for a app using a custom tool:
	argocd admin app generate-spec ksane --repo https://github.com/argoproj/argocd-example-apps.git --path plugins/kasane --dest-namespace default --dest-server https://kubernetes.default.svc --config-management-plugin kasane
`,
		Run: func(c *cobra.Command, args []string) {
			apps, err := cmdutil.ConstructApps(fileURL, appName, labels, annotations, args, appOpts, c.Flags())
			errors.CheckError(err)
			if len(apps) > 1 {
				errors.CheckError(fmt.Errorf("failed to generate spec, more than one application is not supported"))
			}
			app := apps[0]
			if app.Name == "" {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			out, closer, err := getOutWriter(inline, fileURL)
			errors.CheckError(err)
			defer io.Close(closer)

			errors.CheckError(PrintResources(outputFormat, out, app))
		},
	}
	command.Flags().StringVar(&appName, "name", "", "A name for the app, ignored if a file is set (DEPRECATED)")
	command.Flags().StringVarP(&fileURL, "file", "f", "", "Filename or URL to Kubernetes manifests for the app")
	command.Flags().StringArrayVarP(&labels, "label", "l", []string{}, "Labels to apply to the app")
	command.Flags().StringArrayVarP(&annotations, "annotations", "", []string{}, "Set metadata annotations (e.g. example=value)")
	command.Flags().StringVarP(&outputFormat, "output", "o", "yaml", "Output format. One of: json|yaml")
	command.Flags().BoolVarP(&inline, "inline", "i", false, "If set then generated resource is written back to the file specified in --file flag")

	// Only complete files with appropriate extension.
	err := command.Flags().SetAnnotation("file", cobra.BashCompFilenameExt, []string{"json", "yaml", "yml"})
	errors.CheckError(err)

	cmdutil.AddAppFlags(command, &appOpts)
	return command
}

type appReconcileResult struct {
	Name       string                          `json:"name"`
	Health     *v1alpha1.HealthStatus          `json:"health"`
	Sync       *v1alpha1.SyncStatus            `json:"sync"`
	Conditions []v1alpha1.ApplicationCondition `json:"conditions"`
}

type reconcileResults struct {
	Applications []appReconcileResult `json:"applications"`
}

func (r *reconcileResults) getAppsMap() map[string]appReconcileResult {
	res := map[string]appReconcileResult{}
	for i := range r.Applications {
		res[r.Applications[i].Name] = r.Applications[i]
	}
	return res
}

func printLine(format string, a ...interface{}) {
	_, _ = fmt.Printf(format+"\n", a...)
}

func NewDiffReconcileResults() *cobra.Command {
	var command = &cobra.Command{
		Use:   "diff-reconcile-results PATH1 PATH2",
		Short: "Compare results of two reconciliations and print diff.",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			path1 := args[0]
			path2 := args[1]
			var res1 reconcileResults
			var res2 reconcileResults
			errors.CheckError(config.UnmarshalLocalFile(path1, &res1))
			errors.CheckError(config.UnmarshalLocalFile(path2, &res2))
			errors.CheckError(diffReconcileResults(res1, res2))
		},
	}

	return command
}

func toUnstructured(val interface{}) (*unstructured.Unstructured, error) {
	data, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	res := make(map[string]interface{})
	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: res}, nil
}

type diffPair struct {
	name   string
	first  *unstructured.Unstructured
	second *unstructured.Unstructured
}

func diffReconcileResults(res1 reconcileResults, res2 reconcileResults) error {
	var pairs []diffPair
	resMap1 := res1.getAppsMap()
	resMap2 := res2.getAppsMap()
	for k, v := range resMap1 {
		firstUn, err := toUnstructured(v)
		if err != nil {
			return err
		}
		var secondUn *unstructured.Unstructured
		second, ok := resMap2[k]
		if ok {
			secondUn, err = toUnstructured(second)
			if err != nil {
				return err
			}
			delete(resMap2, k)
		}
		pairs = append(pairs, diffPair{name: k, first: firstUn, second: secondUn})
	}
	for k, v := range resMap2 {
		secondUn, err := toUnstructured(v)
		if err != nil {
			return err
		}
		pairs = append(pairs, diffPair{name: k, first: nil, second: secondUn})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].name < pairs[j].name
	})
	for _, item := range pairs {
		printLine(item.name)
		_ = cli.PrintDiff(item.name, item.first, item.second)
	}

	return nil
}

func NewReconcileCommand() *cobra.Command {
	var (
		clientConfig      clientcmd.ClientConfig
		selector          string
		repoServerAddress string
		outputFormat      string
		refresh           bool
	)

	var command = &cobra.Command{
		Use:   "get-reconcile-results PATH",
		Short: "Reconcile all applications and stores reconciliation summary in the specified file.",
		Run: func(c *cobra.Command, args []string) {
			// get rid of logging error handler
			runtime.ErrorHandlers = runtime.ErrorHandlers[1:]

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			outputPath := args[0]

			errors.CheckError(os.Setenv(v1alpha1.EnvVarFakeInClusterConfig, "true"))
			cfg, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)

			var result []appReconcileResult
			if refresh {
				if repoServerAddress == "" {
					printLine("Repo server is not provided, trying to port-forward to argocd-repo-server pod.")
					overrides := clientcmd.ConfigOverrides{}
					repoServerPort, err := kubeutil.PortForward(8081, namespace, &overrides, "app.kubernetes.io/name=argocd-repo-server")
					errors.CheckError(err)
					repoServerAddress = fmt.Sprintf("localhost:%d", repoServerPort)
				}
				repoServerClient := argocdclient.NewRepoServerClientset(repoServerAddress, 60, argocdclient.TLSConfiguration{DisableTLS: false, StrictValidation: false})

				appClientset := appclientset.NewForConfigOrDie(cfg)
				kubeClientset := kubernetes.NewForConfigOrDie(cfg)
				result, err = reconcileApplications(kubeClientset, appClientset, namespace, repoServerClient, selector, newLiveStateCache)
				errors.CheckError(err)
			} else {
				appClientset := appclientset.NewForConfigOrDie(cfg)
				result, err = getReconcileResults(appClientset, namespace, selector)
			}

			errors.CheckError(saveToFile(err, outputFormat, reconcileResults{Applications: result}, outputPath))
		},
	}
	clientConfig = cli.AddKubectlFlagsToCmd(command)
	command.Flags().StringVar(&repoServerAddress, "repo-server", "", "Repo server address.")
	command.Flags().StringVar(&selector, "l", "", "Label selector")
	command.Flags().StringVar(&outputFormat, "o", "yaml", "Output format (yaml|json)")
	command.Flags().BoolVar(&refresh, "refresh", false, "If set to true then recalculates apps reconciliation")

	return command
}

func saveToFile(err error, outputFormat string, result reconcileResults, outputPath string) error {
	errors.CheckError(err)
	var data []byte
	switch outputFormat {
	case "yaml":
		if data, err = yaml.Marshal(result); err != nil {
			return err
		}
	case "json":
		if data, err = json.Marshal(result); err != nil {
			return err
		}
	default:
		return fmt.Errorf("format %s is not supported", outputFormat)
	}

	return ioutil.WriteFile(outputPath, data, 0644)
}

func getReconcileResults(appClientset appclientset.Interface, namespace string, selector string) ([]appReconcileResult, error) {
	appsList, err := appClientset.ArgoprojV1alpha1().Applications(namespace).List(context.Background(), v1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}

	var items []appReconcileResult
	for _, app := range appsList.Items {
		items = append(items, appReconcileResult{
			Name:       app.Name,
			Conditions: app.Status.Conditions,
			Health:     &app.Status.Health,
			Sync:       &app.Status.Sync,
		})
	}
	return items, nil
}

func reconcileApplications(
	kubeClientset kubernetes.Interface,
	appClientset appclientset.Interface,
	namespace string,
	repoServerClient argocdclient.Clientset,
	selector string,
	createLiveStateCache func(argoDB db.ArgoDB, appInformer kubecache.SharedIndexInformer, settingsMgr *settings.SettingsManager, server *metrics.MetricsServer) cache.LiveStateCache,
) ([]appReconcileResult, error) {

	settingsMgr := settings.NewSettingsManager(context.Background(), kubeClientset, namespace)
	argoDB := db.NewDB(namespace, settingsMgr, kubeClientset)
	appInformerFactory := appinformers.NewSharedInformerFactoryWithOptions(
		appClientset,
		1*time.Hour,
		appinformers.WithNamespace(namespace),
		appinformers.WithTweakListOptions(func(options *v1.ListOptions) {}),
	)

	appInformer := appInformerFactory.Argoproj().V1alpha1().Applications().Informer()
	projInformer := appInformerFactory.Argoproj().V1alpha1().AppProjects().Informer()
	go appInformer.Run(context.Background().Done())
	go projInformer.Run(context.Background().Done())
	if !kubecache.WaitForCacheSync(context.Background().Done(), appInformer.HasSynced, projInformer.HasSynced) {
		return nil, fmt.Errorf("failed to sync cache")
	}

	appLister := appInformerFactory.Argoproj().V1alpha1().Applications().Lister()
	projLister := appInformerFactory.Argoproj().V1alpha1().AppProjects().Lister()
	server, err := metrics.NewMetricsServer("", appLister, func(obj interface{}) bool {
		return true
	}, func(r *http.Request) error {
		return nil
	}, []string{})

	if err != nil {
		return nil, err
	}
	stateCache := createLiveStateCache(argoDB, appInformer, settingsMgr, server)
	if err := stateCache.Init(); err != nil {
		return nil, err
	}

	cache := appstatecache.NewCache(
		cacheutil.NewCache(cacheutil.NewInMemoryCache(1*time.Minute)),
		1*time.Minute,
	)

	appStateManager := controller.NewAppStateManager(
		argoDB, appClientset, repoServerClient, namespace, kubeutil.NewKubectl(), settingsMgr, stateCache, projInformer, server, cache, time.Second, argo.NewResourceTracking())

	appsList, err := appClientset.ArgoprojV1alpha1().Applications(namespace).List(context.Background(), v1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}

	sort.Slice(appsList.Items, func(i, j int) bool {
		return appsList.Items[i].Spec.Destination.Server < appsList.Items[j].Spec.Destination.Server
	})

	var items []appReconcileResult
	prevServer := ""
	for _, app := range appsList.Items {
		if prevServer != app.Spec.Destination.Server {
			if prevServer != "" {
				if clusterCache, err := stateCache.GetClusterCache(prevServer); err == nil {
					clusterCache.Invalidate()
				}
			}
			printLine("Reconciling apps of %s", app.Spec.Destination.Server)
			prevServer = app.Spec.Destination.Server
		}
		printLine(app.Name)

		proj, err := projLister.AppProjects(namespace).Get(app.Spec.Project)
		if err != nil {
			return nil, err
		}

		res := appStateManager.CompareAppState(&app, proj, app.Spec.Source.TargetRevision, app.Spec.Source, false, false, nil)
		items = append(items, appReconcileResult{
			Name:       app.Name,
			Conditions: app.Status.Conditions,
			Health:     res.GetHealthStatus(),
			Sync:       res.GetSyncStatus(),
		})
	}
	return items, nil
}

func newLiveStateCache(argoDB db.ArgoDB, appInformer kubecache.SharedIndexInformer, settingsMgr *settings.SettingsManager, server *metrics.MetricsServer) cache.LiveStateCache {
	return cache.NewLiveStateCache(argoDB, appInformer, settingsMgr, kubeutil.NewKubectl(), server, func(managedByApp map[string]bool, ref apiv1.ObjectReference) {}, nil, argo.NewResourceTracking())
}
