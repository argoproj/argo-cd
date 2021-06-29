package commands

import (
	"context"
	"fmt"
	"math"
	"os"
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
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/clusterauth"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/errors"
	kubeutil "github.com/argoproj/argo-cd/v2/util/kube"
	"github.com/argoproj/argo-cd/v2/util/settings"
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

	return command
}

type ClusterWithInfo struct {
	argoappv1.Cluster
	Shard int
}

func loadClusters(kubeClient *kubernetes.Clientset, replicas int, namespace string, portForwardRedis bool, cacheSrc func() (*appstatecache.Cache, error), shard int) ([]ClusterWithInfo, error) {
	settingsMgr := settings.NewSettingsManager(context.Background(), kubeClient, namespace)

	argoDB := db.NewDB(namespace, settingsMgr, kubeClient)
	clustersList, err := argoDB.ListClusters(context.Background())
	if err != nil {
		return nil, err
	}
	var cache *appstatecache.Cache
	if portForwardRedis {
		overrides := clientcmd.ConfigOverrides{}
		port, err := kubeutil.PortForward("app.kubernetes.io/name=argocd-redis-ha-haproxy", 6379, namespace, &overrides)
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

			_ = cache.GetClusterInfo(cluster.Server, &cluster.Info)
			clusters[batchStart+i] = ClusterWithInfo{cluster, clusterShard}
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

			if replicas == 0 {
				replicas, err = getControllerReplicas(kubeClient, namespace)
				errors.CheckError(err)
			}
			if replicas == 0 {
				return
			}

			clusters, err := loadClusters(kubeClient, replicas, namespace, portForwardRedis, cacheSrc, shard)
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
			if replicas == 0 {
				replicas, err = getControllerReplicas(kubeClient, namespace)
				errors.CheckError(err)
			}
			clusters, err := loadClusters(kubeClient, replicas, namespace, portForwardRedis, cacheSrc, shard)
			errors.CheckError(err)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintf(w, "SERVER\tSHARD\tCONNECTION\tAPPS COUNT\tRESOURCES COUNT\n")
			for _, cluster := range clusters {
				_, _ = fmt.Fprintf(w, "%s\t%d\t%s\t%d\t%d\n", cluster.Server, cluster.Shard, cluster.Info.ConnectionState.Status, cluster.Info.ApplicationsCount, cluster.Info.CacheInfo.ResourcesCount)
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

// NewClusterConfig returns a new instance of `argocd-util kubeconfig` command
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
			clst := cmdutil.NewCluster(contextName, clusterOpts.Namespaces, conf, bearerToken, awsAuthConf, execProviderConf)
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

			cmdutil.ConvertSecretData(secret)
			var printResources []interface{}
			printResources = append(printResources, secret)
			errors.CheckError(cmdutil.PrintResources(printResources, outputFormat))
		},
	}
	command.PersistentFlags().StringVar(&pathOpts.LoadingRules.ExplicitPath, pathOpts.ExplicitFileFlag, pathOpts.LoadingRules.ExplicitPath, "use a particular kubeconfig file")
	command.Flags().StringVar(&bearerToken, "bearer-token", "", "Authentication token that should be used to access K8S API server")
	command.Flags().BoolVar(&generateToken, "generate-bearer-token", false, "Generate authentication token that should be used to access K8S API server")
	command.Flags().StringVar(&clusterOpts.ServiceAccount, "service-account", "argocd-manager", fmt.Sprintf("System namespace service account to use for kubernetes resource management. If not set then default \"%s\" SA will be used", clusterauth.ArgoCDManagerServiceAccount))
	command.Flags().StringVar(&clusterOpts.SystemNamespace, "system-namespace", common.DefaultSystemNamespace, "Use different system namespace")
	command.Flags().StringVarP(&outputFormat, "output", "o", "yaml", "Output format. One of: json|yaml")
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
