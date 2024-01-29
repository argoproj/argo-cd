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
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"

	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/controller/sharding"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
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

func NewClusterCommand(clientOpts *argocdclient.ClientOptions, pathOpts *clientcmd.PathOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "cluster",
		Short: "Manage clusters configuration",
		Example: `
#Generate declarative config for a cluster
argocd admin cluster generate-spec my-cluster -o yaml

#Generate a kubeconfig for a cluster named "my-cluster" and display it in the console
argocd admin cluster kubeconfig my-cluster

#Print information namespaces which Argo CD manages in each cluster
argocd admin cluster namespaces my-cluster `,
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}

	command.AddCommand(NewClusterConfig())
	command.AddCommand(NewGenClusterConfigCommand(pathOpts))
	command.AddCommand(NewClusterStatsCommand(clientOpts))
	command.AddCommand(NewClusterShardsCommand(clientOpts))
	namespacesCommand := NewClusterNamespacesCommand()
	namespacesCommand.AddCommand(NewClusterEnableNamespacedMode())
	namespacesCommand.AddCommand(NewClusterDisableNamespacedMode())
	command.AddCommand(namespacesCommand)

	return command
}

type ClusterWithInfo struct {
	v1alpha1.Cluster
	// Shard holds controller shard number that handles the cluster
	Shard int
	// Namespaces holds list of namespaces managed by Argo CD in the cluster
	Namespaces []string
}

func loadClusters(ctx context.Context, kubeClient *kubernetes.Clientset, appClient *versioned.Clientset, replicas int, shardingAlgorithm string, namespace string, portForwardRedis bool, cacheSrc func() (*appstatecache.Cache, error), shard int, redisName string, redisHaProxyName string, redisCompressionStr string) ([]ClusterWithInfo, error) {
	settingsMgr := settings.NewSettingsManager(ctx, kubeClient, namespace)

	argoDB := db.NewDB(namespace, settingsMgr, kubeClient)
	clustersList, err := argoDB.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	clusterShardingCache := sharding.NewClusterSharding(argoDB, shard, replicas, shardingAlgorithm)
	clusterShardingCache.Init(clustersList)
	clusterShards := clusterShardingCache.GetDistribution()

	var cache *appstatecache.Cache
	if portForwardRedis {
		overrides := clientcmd.ConfigOverrides{}
		redisHaProxyPodLabelSelector := common.LabelKeyAppName + "=" + redisHaProxyName
		redisPodLabelSelector := common.LabelKeyAppName + "=" + redisName
		port, err := kubeutil.PortForward(6379, namespace, &overrides,
			redisHaProxyPodLabelSelector, redisPodLabelSelector)
		if err != nil {
			return nil, err
		}
		client := redis.NewClient(&redis.Options{Addr: fmt.Sprintf("localhost:%d", port)})
		compressionType, err := cacheutil.CompressionTypeFromString(redisCompressionStr)
		if err != nil {
			return nil, err
		}
		cache = appstatecache.NewCache(cacheutil.NewCache(cacheutil.NewRedisCache(client, time.Hour, compressionType)), time.Hour)
	} else {
		cache, err = cacheSrc()
		if err != nil {
			return nil, err
		}
	}

	appItems, err := appClient.ArgoprojV1alpha1().Applications(namespace).List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, err
	}
	apps := appItems.Items
	for i, app := range apps {
		err := argo.ValidateDestination(ctx, &app.Spec.Destination, argoDB)
		if err != nil {
			return nil, err
		}
		apps[i] = app
	}
	clusters := make([]ClusterWithInfo, len(clustersList.Items))

	batchSize := 10
	batchesCount := int(math.Ceil(float64(len(clusters)) / float64(batchSize)))
	clusterSharding := &sharding.ClusterSharding{
		Shard:    shard,
		Replicas: replicas,
		Shards:   make(map[string]int),
		Clusters: make(map[string]*v1alpha1.Cluster),
	}
	for batchNum := 0; batchNum < batchesCount; batchNum++ {
		batchStart := batchSize * batchNum
		batchEnd := batchSize * (batchNum + 1)
		if batchEnd > len(clustersList.Items) {
			batchEnd = len(clustersList.Items)
		}
		batch := clustersList.Items[batchStart:batchEnd]
		_ = kube.RunAllAsync(len(batch), func(i int) error {
			clusterShard := 0
			cluster := batch[i]
			if replicas > 0 {
				distributionFunction := sharding.GetDistributionFunction(clusterSharding.GetClusterAccessor(), common.DefaultShardingAlgorithm, replicas)
				distributionFunction(&cluster)
				clusterShard := clusterShards[cluster.Server]
				cluster.Shard = pointer.Int64(int64(clusterShard))
				log.Infof("Cluster with uid: %s will be processed by shard %d", cluster.ID, clusterShard)
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

func getControllerReplicas(ctx context.Context, kubeClient *kubernetes.Clientset, namespace string, appControllerName string) (int, error) {
	appControllerPodLabelSelector := common.LabelKeyAppName + "=" + appControllerName
	controllerPods, err := kubeClient.CoreV1().Pods(namespace).List(ctx, v1.ListOptions{
		LabelSelector: appControllerPodLabelSelector})
	if err != nil {
		return 0, err
	}
	return len(controllerPods.Items), nil
}

func NewClusterShardsCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		shard               int
		replicas            int
		shardingAlgorithm   string
		clientConfig        clientcmd.ClientConfig
		cacheSrc            func() (*appstatecache.Cache, error)
		portForwardRedis    bool
		redisCompressionStr string
	)
	var command = cobra.Command{
		Use:   "shards",
		Short: "Print information about each controller shard and the estimated portion of Kubernetes resources it is responsible for.",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()

			log.SetLevel(log.WarnLevel)

			clientCfg, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			kubeClient := kubernetes.NewForConfigOrDie(clientCfg)
			appClient := versioned.NewForConfigOrDie(clientCfg)

			if replicas == 0 {
				replicas, err = getControllerReplicas(ctx, kubeClient, namespace, clientOpts.AppControllerName)
				errors.CheckError(err)
			}
			if replicas == 0 {
				return
			}
			clusters, err := loadClusters(ctx, kubeClient, appClient, replicas, shardingAlgorithm, namespace, portForwardRedis, cacheSrc, shard, clientOpts.RedisName, clientOpts.RedisHaProxyName, redisCompressionStr)
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
	command.Flags().StringVar(&shardingAlgorithm, "sharding-method", common.DefaultShardingAlgorithm, "Sharding method. Defaults: legacy. Supported sharding methods are : [legacy, round-robin] ")
	command.Flags().BoolVar(&portForwardRedis, "port-forward-redis", true, "Automatically port-forward ha proxy redis from current namespace?")

	cacheSrc = appstatecache.AddCacheFlagsToCmd(&command)

	// parse all added flags so far to get the redis-compression flag that was added by AddCacheFlagsToCmd() above
	// we can ignore unchecked error here as the command will be parsed again and checked when command.Execute() is run later
	// nolint:errcheck
	command.ParseFlags(os.Args[1:])
	redisCompressionStr, _ = command.Flags().GetString(cacheutil.CLIFlagRedisCompress)
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

func runClusterNamespacesCommand(ctx context.Context, clientConfig clientcmd.ClientConfig, action func(appClient *versioned.Clientset, argoDB db.ArgoDB, clusters map[string][]string) error) error {
	clientCfg, err := clientConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("error while creating client config: %w", err)
	}
	namespace, _, err := clientConfig.Namespace()
	if err != nil {
		return fmt.Errorf("error while getting namespace from client config: %w", err)
	}

	kubeClient := kubernetes.NewForConfigOrDie(clientCfg)
	appClient := versioned.NewForConfigOrDie(clientCfg)

	settingsMgr := settings.NewSettingsManager(ctx, kubeClient, namespace)
	argoDB := db.NewDB(namespace, settingsMgr, kubeClient)
	clustersList, err := argoDB.ListClusters(ctx)
	if err != nil {
		return fmt.Errorf("error listing clusters: %w", err)
	}
	appItems, err := appClient.ArgoprojV1alpha1().Applications(namespace).List(ctx, v1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing application: %w", err)
	}
	apps := appItems.Items
	for i, app := range apps {
		if err := argo.ValidateDestination(ctx, &app.Spec.Destination, argoDB); err != nil {
			return fmt.Errorf("error validating application destination: %w", err)
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
				nsSet[app.Spec.Destination.Namespace] = true
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
			ctx := cmd.Context()

			log.SetLevel(log.WarnLevel)

			err := runClusterNamespacesCommand(ctx, clientConfig, func(appClient *versioned.Clientset, _ db.ArgoDB, clusters map[string][]string) error {
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
			ctx := cmd.Context()

			log.SetLevel(log.WarnLevel)

			if len(args) == 0 {
				cmd.HelpFunc()(cmd, args)
				os.Exit(1)
			}
			pattern := args[0]

			errors.CheckError(runClusterNamespacesCommand(ctx, clientConfig, func(_ *versioned.Clientset, argoDB db.ArgoDB, clusters map[string][]string) error {
				for server, namespaces := range clusters {
					if len(namespaces) == 0 || len(namespaces) > namespacesCount || !glob.Match(pattern, server) {
						continue
					}

					cluster, err := argoDB.GetCluster(ctx, server)
					if err != nil {
						return fmt.Errorf("error getting cluster from server: %w", err)
					}
					cluster.Namespaces = namespaces
					cluster.ClusterResources = clusterResources
					fmt.Printf("Setting cluster %s namespaces to %v...", server, namespaces)
					if !dryRun {
						if _, err = argoDB.UpdateCluster(ctx, cluster); err != nil {
							return fmt.Errorf("error updating cluster: %w", err)
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
			ctx := cmd.Context()

			log.SetLevel(log.WarnLevel)

			if len(args) == 0 {
				cmd.HelpFunc()(cmd, args)
				os.Exit(1)
			}

			pattern := args[0]

			errors.CheckError(runClusterNamespacesCommand(ctx, clientConfig, func(_ *versioned.Clientset, argoDB db.ArgoDB, clusters map[string][]string) error {
				for server := range clusters {
					if !glob.Match(pattern, server) {
						continue
					}

					cluster, err := argoDB.GetCluster(ctx, server)
					if err != nil {
						return fmt.Errorf("error getting cluster from server: %w", err)
					}

					if len(cluster.Namespaces) == 0 {
						continue
					}

					cluster.Namespaces = nil
					fmt.Printf("Disabling namespaced mode for cluster %s...", server)
					if !dryRun {
						if _, err = argoDB.UpdateCluster(ctx, cluster); err != nil {
							return fmt.Errorf("error updating cluster: %w", err)
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

func NewClusterStatsCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		shard               int
		replicas            int
		shardingAlgorithm   string
		clientConfig        clientcmd.ClientConfig
		cacheSrc            func() (*appstatecache.Cache, error)
		portForwardRedis    bool
		redisCompressionStr string
	)
	var command = cobra.Command{
		Use:   "stats",
		Short: "Prints information cluster statistics and inferred shard number",
		Example: `
#Display stats and shards for clusters 
argocd admin cluster stats

#Display Cluster Statistics for a Specific Shard
argocd admin cluster stats --shard=1

#In a multi-cluster environment to print stats for a specific cluster say(target-cluster)
argocd admin cluster stats target-cluster`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()

			log.SetLevel(log.WarnLevel)

			clientCfg, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)

			kubeClient := kubernetes.NewForConfigOrDie(clientCfg)
			appClient := versioned.NewForConfigOrDie(clientCfg)
			if replicas == 0 {
				replicas, err = getControllerReplicas(ctx, kubeClient, namespace, clientOpts.AppControllerName)
				errors.CheckError(err)
			}
			clusters, err := loadClusters(ctx, kubeClient, appClient, replicas, shardingAlgorithm, namespace, portForwardRedis, cacheSrc, shard, clientOpts.RedisName, clientOpts.RedisHaProxyName, redisCompressionStr)
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
	command.Flags().StringVar(&shardingAlgorithm, "sharding-method", common.DefaultShardingAlgorithm, "Sharding method. Defaults: legacy. Supported sharding methods are : [legacy, round-robin] ")
	command.Flags().BoolVar(&portForwardRedis, "port-forward-redis", true, "Automatically port-forward ha proxy redis from current namespace?")
	cacheSrc = appstatecache.AddCacheFlagsToCmd(&command)

	// parse all added flags so far to get the redis-compression flag that was added by AddCacheFlagsToCmd() above
	// we can ignore unchecked error here as the command will be parsed again and checked when command.Execute() is run later
	// nolint:errcheck
	command.ParseFlags(os.Args[1:])
	redisCompressionStr, _ = command.Flags().GetString(cacheutil.CLIFlagRedisCompress)
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
		Example: `
#Generate a kubeconfig for a cluster named "my-cluster" on console
argocd admin cluster kubeconfig my-cluster

#Listing available kubeconfigs for clusters managed by argocd
argocd admin cluster kubeconfig

#Removing a specific kubeconfig file 
argocd admin cluster kubeconfig my-cluster --delete

#Generate a Kubeconfig for a Cluster with TLS Verification Disabled
argocd admin cluster kubeconfig https://cluster-api-url:6443 /path/to/output/kubeconfig.yaml --insecure-skip-tls-verify`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

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

			cluster, err := db.NewDB(namespace, settings.NewSettingsManager(ctx, kubeclientset, namespace), kubeclientset).GetCluster(ctx, serverUrl)
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
			ctx := c.Context()

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

			if clusterOpts.InCluster && clusterOpts.ClusterEndpoint != "" {
				log.Fatal("Can only use one of --in-cluster or --cluster-endpoint")
				return
			}

			overrides := clientcmd.ConfigOverrides{
				Context: *clstContext,
			}
			clientConfig := clientcmd.NewDefaultClientConfig(*cfgAccess, &overrides)
			conf, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			kubeClientset := fake.NewSimpleClientset()

			var awsAuthConf *v1alpha1.AWSAuthConfig
			var execProviderConf *v1alpha1.ExecProviderConfig
			if clusterOpts.AwsClusterName != "" {
				awsAuthConf = &v1alpha1.AWSAuthConfig{
					ClusterName: clusterOpts.AwsClusterName,
					RoleARN:     clusterOpts.AwsRoleArn,
					Profile:     clusterOpts.AwsProfile,
				}
			} else if clusterOpts.ExecProviderCommand != "" {
				execProviderConf = &v1alpha1.ExecProviderConfig{
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
			if clusterOpts.InClusterEndpoint() {
				clst.Server = v1alpha1.KubernetesInternalAPIServerAddr
			}
			if clusterOpts.ClusterEndpoint == string(cmdutil.KubePublicEndpoint) {
				// Ignore `kube-public` cluster endpoints, since this command is intended to run without invoking any network connections.
				log.Warn("kube-public cluster endpoints are not supported. Falling back to the endpoint listed in the kubconfig context.")
			}
			if clusterOpts.Shard >= 0 {
				clst.Shard = &clusterOpts.Shard
			}

			settingsMgr := settings.NewSettingsManager(ctx, kubeClientset, ArgoCDNamespace)
			argoDB := db.NewDB(ArgoCDNamespace, settingsMgr, kubeClientset)

			_, err = argoDB.CreateCluster(ctx, clst)
			errors.CheckError(err)

			secName, err := db.URIToSecretName("cluster", clst.Server)
			errors.CheckError(err)

			secret, err := kubeClientset.CoreV1().Secrets(ArgoCDNamespace).Get(ctx, secName, v1.GetOptions{})
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

	bearerToken, err := clusterauth.GetServiceAccountBearerToken(clientset, clusterOpts.SystemNamespace, clusterOpts.ServiceAccount, common.BearerTokenTimeout)
	if err != nil {
		return "", err
	}
	return bearerToken, nil
}
