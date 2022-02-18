package admin

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/controller/sharding"
	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/v2/util/argo"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/clusterauth"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/glob"
	kubeutil "github.com/argoproj/argo-cd/v2/util/kube"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/argoproj/argo-cd/v2/util/text/label"
)

func NewClusterCommand(pathOpts *clientcmd.PathOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "cluster",
		Short: "Manage clusters configuration",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}

	command.AddCommand(NewClusterConfig())
	command.AddCommand(NewGenClusterConfigCommand(pathOpts))
	command.AddCommand(NewClusterStatsCommand())
	command.AddCommand(NewClusterShardsCommand())
	namespacesCommand := NewClusterNamespacesCommand()
	namespacesCommand.AddCommand(NewClusterEnableNamespacedMode())
	namespacesCommand.AddCommand(NewClusterDisableNamespacedMode())
	command.AddCommand(namespacesCommand)

	return command
}

type ClusterWithInfo struct {
	argoappv1.Cluster
	// Shard holds controller shard number that handles the cluster
	Shard int
	// Namespaces holds list of namespaces managed by Argo CD in the cluster
	Namespaces []string
}

func loadClusters(kubeClient *kubernetes.Clientset, appClient *versioned.Clientset, replicas int, namespace string, portForwardRedis bool, cacheSrc func() (*appstatecache.Cache, error), shard int) ([]ClusterWithInfo, error) {
	settingsMgr := settings.NewSettingsManager(context.Background(), kubeClient, namespace)

	argoDB := db.NewDB(namespace, settingsMgr, kubeClient)
	clustersList, err := argoDB.ListClusters(context.Background())
	if err != nil {
		return nil, err
	}
	var cache *appstatecache.Cache
	if portForwardRedis {
		overrides := clientcmd.ConfigOverrides{}
		port, err := kubeutil.PortForward(6379, namespace, &overrides,
			"app.kubernetes.io/name=argocd-redis-ha-haproxy", "app.kubernetes.io/name=argocd-redis")
		if err != nil {
			return nil, err
		}
		client := redis.NewClient(&redis.Options{Addr: fmt.Sprintf("localhost:%d", port)})
		cache = appstatecache.NewCache(cacheutil.NewCache(cacheutil.NewRedisCache(client, time.Hour)), time.Hour)
	} else {
		cache, err = cacheSrc()
		if err != nil {
			return nil, err
		}
	}

	appItems, err := appClient.ArgoprojV1alpha1().Applications(namespace).List(context.Background(), v1.ListOptions{})
	if err != nil {
		return nil, err
	}
	apps := appItems.Items
	for i, app := range apps {
		err := argo.ValidateDestination(context.Background(), &app.Spec.Destination, argoDB)
		if err != nil {
			return nil, err
		}
		apps[i] = app
	}
	clusters := make([]ClusterWithInfo, len(clustersList.Items))
	batchSize := 10
	batchesCount := int(math.Ceil(float64(len(clusters)) / float64(batchSize)))
	for batchNum := 0; batchNum < batchesCount; batchNum++ {
		batchStart := batchSize * batchNum
		batchEnd := batchSize * (batchNum + 1)
		if batchEnd > len(clustersList.Items) {
			batchEnd = len(clustersList.Items)
		}
		batch := clustersList.Items[batchStart:batchEnd]
		_ = kube.RunAllAsync(len(batch), func(i int) error {
			cluster := batch[i]
			clusterShard := 0
			if replicas > 0 {
				clusterShard = sharding.GetShardByID(cluster.ID, replicas)
			}

			if shard != -1 && clusterShard != shard {
				return nil
			}
			nsSet := map[string]bool{}
			for _, app := range apps {
				if app.Spec.Destination.Server == cluster.Server {
					nsSet[app.Spec.Destination.Namespace] = true
				}
			}
			var namespaces []string
			for ns := range nsSet {
				namespaces = append(namespaces, ns)
			}
			_ = cache.GetClusterInfo(cluster.Server, &cluster.Info)
			clusters[batchStart+i] = ClusterWithInfo{cluster, clusterShard, namespaces}
			return nil
		})
	}
	return clusters, nil
}

func getControllerReplicas(kubeClient *kubernetes.Clientset, namespace string) (int, error) {
	controllerPods, err := kubeClient.CoreV1().Pods(namespace).List(context.Background(), v1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=argocd-application-controller"})
	if err != nil {
		return 0, err
	}
	return len(controllerPods.Items), nil
}

func NewClusterShardsCommand() *cobra.Command {
	var (
		shard            int
		replicas         int
		clientConfig     clientcmd.ClientConfig
		cacheSrc         func() (*appstatecache.Cache, error)
		portForwardRedis bool
	)
	var command = cobra.Command{
		Use:   "shards",
		Short: "Print information about each controller shard and portion of Kubernetes resources it is responsible for.",
		Run: func(cmd *cobra.Command, args []string) {
			log.SetLevel(log.WarnLevel)

			clientCfg, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			kubeClient := kubernetes.NewForConfigOrDie(clientCfg)
			appClient := versioned.NewForConfigOrDie(clientCfg)

			if replicas == 0 {
				replicas, err = getControllerReplicas(kubeClient, namespace)
				errors.CheckError(err)
			}
			if replicas == 0 {
				return
			}

			clusters, err := loadClusters(kubeClient, appClient, replicas, namespace, portForwardRedis, cacheSrc, shard)
			errors.CheckError(err)
			if len(clusters) == 0 {
				return
			}

			printStatsSummary(clusters)
		},
	}
	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.Flags().IntVar(&shard, "shard", -1, "Cluster shard filter")
	command.Flags().IntVar(&replicas, "replicas", 0, "Application controller replicas count. Inferred from number of running controller pods if not specified")
	command.Flags().BoolVar(&portForwardRedis, "port-forward-redis", true, "Automatically port-forward ha proxy redis from current namespace?")
	cacheSrc = appstatecache.AddCacheFlagsToCmd(&command)
	return &command
}

func printStatsSummary(clusters []ClusterWithInfo) {
	totalResourcesCount := int64(0)
	resourcesCountByShard := map[int]int64{}
	for _, c := range clusters {
		totalResourcesCount += c.Info.CacheInfo.ResourcesCount
		resourcesCountByShard[c.Shard] += c.Info.CacheInfo.ResourcesCount
	}

	avgResourcesByShard := totalResourcesCount / int64(len(resourcesCountByShard))
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "SHARD\tRESOURCES COUNT\n")
	for shard := 0; shard < len(resourcesCountByShard); shard++ {
		cnt := resourcesCountByShard[shard]
		percent := (float64(cnt) / float64(avgResourcesByShard)) * 100.0
		_, _ = fmt.Fprintf(w, "%d\t%s\n", shard, fmt.Sprintf("%d (%.0f%%)", cnt, percent))
	}
	_ = w.Flush()
}

func runClusterNamespacesCommand(clientConfig clientcmd.ClientConfig, action func(appClient *versioned.Clientset, argoDB db.ArgoDB, clusters map[string][]string) error) error {
	clientCfg, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}
	namespace, _, err := clientConfig.Namespace()
	if err != nil {
		return err
	}

	kubeClient := kubernetes.NewForConfigOrDie(clientCfg)
	appClient := versioned.NewForConfigOrDie(clientCfg)

	settingsMgr := settings.NewSettingsManager(context.Background(), kubeClient, namespace)
	argoDB := db.NewDB(namespace, settingsMgr, kubeClient)
	clustersList, err := argoDB.ListClusters(context.Background())
	if err != nil {
		return err
	}
	appItems, err := appClient.ArgoprojV1alpha1().Applications(namespace).List(context.Background(), v1.ListOptions{})
	if err != nil {
		return err
	}
	apps := appItems.Items
	for i, app := range apps {
		err := argo.ValidateDestination(context.Background(), &app.Spec.Destination, argoDB)
		if err != nil {
			return err
		}
		apps[i] = app
	}

	clusters := map[string][]string{}
	for _, cluster := range clustersList.Items {
		nsSet := map[string]bool{}
		for _, app := range apps {
			if app.Spec.Destination.Server != cluster.Server {
				continue
			}
			// Use namespaces of actually deployed resources, since some application use dummy target namespace
			// If resources list is empty then use target namespace
			if len(app.Status.Resources) != 0 {
				for _, res := range app.Status.Resources {
					if res.Namespace != "" {
						nsSet[res.Namespace] = true
					}
				}
			} else {
				if app.Spec.Destination.Server == cluster.Server {
					nsSet[app.Spec.Destination.Namespace] = true
				}
			}
		}
		var namespaces []string
		for ns := range nsSet {
			namespaces = append(namespaces, ns)
		}
		clusters[cluster.Server] = namespaces
	}
	return action(appClient, argoDB, clusters)
}

func NewClusterNamespacesCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
	)
	var command = cobra.Command{
		Use:   "namespaces",
		Short: "Print information namespaces which Argo CD manages in each cluster.",
		Run: func(cmd *cobra.Command, args []string) {
			log.SetLevel(log.WarnLevel)

			err := runClusterNamespacesCommand(clientConfig, func(appClient *versioned.Clientset, _ db.ArgoDB, clusters map[string][]string) error {
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				_, _ = fmt.Fprintf(w, "CLUSTER\tNAMESPACES\n")

				for cluster, namespaces := range clusters {
					// print shortest namespace names first
					sort.Slice(namespaces, func(i, j int) bool {
						return len(namespaces[j]) > len(namespaces[i])
					})
					namespacesStr := ""
					if len(namespaces) > 4 {
						namespacesStr = fmt.Sprintf("%s (total %d)", strings.Join(namespaces[:4], ","), len(namespaces))
					} else {
						namespacesStr = strings.Join(namespaces, ",")
					}

					_, _ = fmt.Fprintf(w, "%s\t%s\n", cluster, namespacesStr)
				}
				_ = w.Flush()
				return nil
			})
			errors.CheckError(err)
		},
	}
	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	return &command
}

func NewClusterEnableNamespacedMode() *cobra.Command {
	var (
		clientConfig     clientcmd.ClientConfig
		dryRun           bool
		clusterResources bool
		namespacesCount  int
	)
	var command = cobra.Command{
		Use:   "enable-namespaced-mode PATTERN",
		Short: "Enable namespaced mode for clusters which name matches to the specified pattern.",
		Run: func(cmd *cobra.Command, args []string) {
			log.SetLevel(log.WarnLevel)

			if len(args) == 0 {
				cmd.HelpFunc()(cmd, args)
				os.Exit(1)
			}
			pattern := args[0]

			errors.CheckError(runClusterNamespacesCommand(clientConfig, func(_ *versioned.Clientset, argoDB db.ArgoDB, clusters map[string][]string) error {
				for server, namespaces := range clusters {
					if len(namespaces) == 0 || len(namespaces) > namespacesCount || !glob.Match(pattern, server) {
						continue
					}

					cluster, err := argoDB.GetCluster(context.Background(), server)
					if err != nil {
						return err
					}
					cluster.Namespaces = namespaces
					cluster.ClusterResources = clusterResources
					fmt.Printf("Setting cluster %s namespaces to %v...", server, namespaces)
					if !dryRun {
						_, err = argoDB.UpdateCluster(context.Background(), cluster)
						if err != nil {
							return err
						}
						fmt.Println("done")
					} else {
						fmt.Println("done (dry run)")
					}

				}
				return nil
			}))
		},
	}
	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.Flags().BoolVar(&dryRun, "dry-run", true, "Print what will be performed")
	command.Flags().BoolVar(&clusterResources, "cluster-resources", false, "Indicates if cluster level resources should be managed.")
	command.Flags().IntVar(&namespacesCount, "max-namespace-count", 0, "Max number of namespaces that cluster should managed managed namespaces is less or equal to specified count")

	return &command
}

func NewClusterDisableNamespacedMode() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
		dryRun       bool
	)
	var command = cobra.Command{
		Use:   "disable-namespaced-mode PATTERN",
		Short: "Disable namespaced mode for clusters which name matches to the specified pattern.",
		Run: func(cmd *cobra.Command, args []string) {
			log.SetLevel(log.WarnLevel)

			if len(args) == 0 {
				cmd.HelpFunc()(cmd, args)
				os.Exit(1)
			}

			pattern := args[0]

			errors.CheckError(runClusterNamespacesCommand(clientConfig, func(_ *versioned.Clientset, argoDB db.ArgoDB, clusters map[string][]string) error {
				for server := range clusters {
					if !glob.Match(pattern, server) {
						continue
					}

					cluster, err := argoDB.GetCluster(context.Background(), server)
					if err != nil {
						return err
					}

					if len(cluster.Namespaces) == 0 {
						continue
					}

					cluster.Namespaces = nil
					fmt.Printf("Disabling namespaced mode for cluster %s...", server)
					if !dryRun {
						_, err = argoDB.UpdateCluster(context.Background(), cluster)
						if err != nil {
							return err
						}
						fmt.Println("done")
					} else {
						fmt.Println("done (dry run)")
					}

				}
				return nil
			}))
		},
	}
	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.Flags().BoolVar(&dryRun, "dry-run", true, "Print what will be performed")
	return &command
}

func NewClusterStatsCommand() *cobra.Command {
	var (
		shard            int
		replicas         int
		clientConfig     clientcmd.ClientConfig
		cacheSrc         func() (*appstatecache.Cache, error)
		portForwardRedis bool
	)
	var command = cobra.Command{
		Use:   "stats",
		Short: "Prints information cluster statistics and inferred shard number",
		Run: func(cmd *cobra.Command, args []string) {
			log.SetLevel(log.WarnLevel)

			clientCfg, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)

			kubeClient := kubernetes.NewForConfigOrDie(clientCfg)
			appClient := versioned.NewForConfigOrDie(clientCfg)
			if replicas == 0 {
				replicas, err = getControllerReplicas(kubeClient, namespace)
				errors.CheckError(err)
			}
			clusters, err := loadClusters(kubeClient, appClient, replicas, namespace, portForwardRedis, cacheSrc, shard)
			errors.CheckError(err)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintf(w, "SERVER\tSHARD\tCONNECTION\tNAMESPACES COUNT\tAPPS COUNT\tRESOURCES COUNT\n")
			for _, cluster := range clusters {
				_, _ = fmt.Fprintf(w, "%s\t%d\t%s\t%d\t%d\t%d\n", cluster.Server, cluster.Shard, cluster.Info.ConnectionState.Status, len(cluster.Namespaces), cluster.Info.ApplicationsCount, cluster.Info.CacheInfo.ResourcesCount)
			}
			_ = w.Flush()
		},
	}
	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.Flags().IntVar(&shard, "shard", -1, "Cluster shard filter")
	command.Flags().IntVar(&replicas, "replicas", 0, "Application controller replicas count. Inferred from number of running controller pods if not specified")
	command.Flags().BoolVar(&portForwardRedis, "port-forward-redis", true, "Automatically port-forward ha proxy redis from current namespace?")
	cacheSrc = appstatecache.AddCacheFlagsToCmd(&command)
	return &command
}

// NewClusterConfig returns a new instance of `argocd admin kubeconfig` command
func NewClusterConfig() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
	)
	var command = &cobra.Command{
		Use:               "kubeconfig CLUSTER_URL OUTPUT_PATH",
		Short:             "Generates kubeconfig for the specified cluster",
		DisableAutoGenTag: true,
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			serverUrl := args[0]
			output := args[1]
			conf, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			kubeclientset, err := kubernetes.NewForConfig(conf)
			errors.CheckError(err)

			cluster, err := db.NewDB(namespace, settings.NewSettingsManager(context.Background(), kubeclientset, namespace), kubeclientset).GetCluster(context.Background(), serverUrl)
			errors.CheckError(err)
			err = kube.WriteKubeConfig(cluster.RawRestConfig(), namespace, output)
			errors.CheckError(err)
		},
	}
	clientConfig = cli.AddKubectlFlagsToCmd(command)
	return command
}

func NewGenClusterConfigCommand(pathOpts *clientcmd.PathOptions) *cobra.Command {
	var (
		clusterOpts   cmdutil.ClusterOptions
		bearerToken   string
		generateToken bool
		outputFormat  string
		labels        []string
		annotations   []string
	)
	var command = &cobra.Command{
		Use:   "generate-spec CONTEXT",
		Short: "Generate declarative config for a cluster",
		Run: func(c *cobra.Command, args []string) {
			log.SetLevel(log.WarnLevel)
			var configAccess clientcmd.ConfigAccess = pathOpts
			if len(args) == 0 {
				log.Error("Choose a context name from:")
				cmdutil.PrintKubeContexts(configAccess)
				os.Exit(1)
			}
			cfgAccess, err := configAccess.GetStartingConfig()
			errors.CheckError(err)
			contextName := args[0]
			clstContext := cfgAccess.Contexts[contextName]
			if clstContext == nil {
				log.Fatalf("Context %s does not exist in kubeconfig", contextName)
				return
			}

			overrides := clientcmd.ConfigOverrides{
				Context: *clstContext,
			}
			clientConfig := clientcmd.NewDefaultClientConfig(*cfgAccess, &overrides)
			conf, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			kubeClientset := fake.NewSimpleClientset()

			var awsAuthConf *argoappv1.AWSAuthConfig
			var execProviderConf *argoappv1.ExecProviderConfig
			if clusterOpts.AwsClusterName != "" {
				awsAuthConf = &argoappv1.AWSAuthConfig{
					ClusterName: clusterOpts.AwsClusterName,
					RoleARN:     clusterOpts.AwsRoleArn,
				}
			} else if clusterOpts.ExecProviderCommand != "" {
				execProviderConf = &argoappv1.ExecProviderConfig{
					Command:     clusterOpts.ExecProviderCommand,
					Args:        clusterOpts.ExecProviderArgs,
					Env:         clusterOpts.ExecProviderEnv,
					APIVersion:  clusterOpts.ExecProviderAPIVersion,
					InstallHint: clusterOpts.ExecProviderInstallHint,
				}
			} else if generateToken {
				bearerToken, err = GenerateToken(clusterOpts, conf)
				errors.CheckError(err)
			} else if bearerToken == "" {
				bearerToken = "bearer-token"
			}
			if clusterOpts.Name != "" {
				contextName = clusterOpts.Name
			}

			labelsMap, err := label.Parse(labels)
			errors.CheckError(err)
			annotationsMap, err := label.Parse(annotations)
			errors.CheckError(err)

			clst := cmdutil.NewCluster(contextName, clusterOpts.Namespaces, clusterOpts.ClusterResources, conf, bearerToken, awsAuthConf, execProviderConf, labelsMap, annotationsMap)
			if clusterOpts.InCluster {
				clst.Server = argoappv1.KubernetesInternalAPIServerAddr
			}
			if clusterOpts.Shard >= 0 {
				clst.Shard = &clusterOpts.Shard
			}

			settingsMgr := settings.NewSettingsManager(context.Background(), kubeClientset, ArgoCDNamespace)
			argoDB := db.NewDB(ArgoCDNamespace, settingsMgr, kubeClientset)

			_, err = argoDB.CreateCluster(context.Background(), clst)
			errors.CheckError(err)

			secName, err := db.URIToSecretName("cluster", clst.Server)
			errors.CheckError(err)

			secret, err := kubeClientset.CoreV1().Secrets(ArgoCDNamespace).Get(context.Background(), secName, v1.GetOptions{})
			errors.CheckError(err)

			errors.CheckError(PrintResources(outputFormat, os.Stdout, secret))
		},
	}
	command.PersistentFlags().StringVar(&pathOpts.LoadingRules.ExplicitPath, pathOpts.ExplicitFileFlag, pathOpts.LoadingRules.ExplicitPath, "use a particular kubeconfig file")
	command.Flags().StringVar(&bearerToken, "bearer-token", "", "Authentication token that should be used to access K8S API server")
	command.Flags().BoolVar(&generateToken, "generate-bearer-token", false, "Generate authentication token that should be used to access K8S API server")
	command.Flags().StringVar(&clusterOpts.ServiceAccount, "service-account", "argocd-manager", fmt.Sprintf("System namespace service account to use for kubernetes resource management. If not set then default \"%s\" SA will be used", clusterauth.ArgoCDManagerServiceAccount))
	command.Flags().StringVar(&clusterOpts.SystemNamespace, "system-namespace", common.DefaultSystemNamespace, "Use different system namespace")
	command.Flags().StringVarP(&outputFormat, "output", "o", "yaml", "Output format. One of: json|yaml")
	command.Flags().StringArrayVar(&labels, "label", nil, "Set metadata labels (e.g. --label key=value)")
	command.Flags().StringArrayVar(&annotations, "annotation", nil, "Set metadata annotations (e.g. --annotation key=value)")
	cmdutil.AddClusterFlags(command, &clusterOpts)
	return command
}

func GenerateToken(clusterOpts cmdutil.ClusterOptions, conf *rest.Config) (string, error) {
	clientset, err := kubernetes.NewForConfig(conf)
	errors.CheckError(err)

	bearerToken, err := clusterauth.GetServiceAccountBearerToken(clientset, clusterOpts.SystemNamespace, clusterOpts.ServiceAccount)
	if err != nil {
		return "", err
	}
	return bearerToken, nil
}
