package util

import (
	"context"
	"math/rand"
	"sort"
	"time"

	log "github.com/sirupsen/logrus"

	"k8s.io/client-go/kubernetes"
	//"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/db"
	//"github.com/argoproj/argo-cd/v2/controller/sharding"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// type ShardDistribution struct {
// 	Apps int
// 	Clusters int
// }

func GetAppDistribution(numapps int, appdist string, validClusters []appv1.Cluster) (map[string]int, error) {
	numClusters := len(validClusters)
	appDistribution := make(map[string]int)
	medianAppsPerCluster := int(numapps / numClusters)
	rand.Seed(time.Now().UnixNano())
	if appdist == "equal" {
		for _, cluster := range validClusters {
			appDistribution[cluster.Name] = medianAppsPerCluster
		}
	} else if appdist == "random" {
		totalApps := 0
		x := make([]int, numClusters+1)
		y := make(map[int]int)
		x[0] = 0
		x[numClusters] = numapps
		for i := 1; i < numClusters; i++ {
			for {
				randNum := int(rand.Intn((numapps - 1) + 1))
				if _, ok := y[randNum]; !ok {
					y[randNum] = 1
					x[i] = randNum
					break
				}
			}
		}
		sort.Slice(x, func(i, j int) bool {
			return x[i] < x[j]
		})
		i := 0
		for _, cluster := range validClusters {
			appDistribution[cluster.Name] = x[i+1] - x[i]
			i++
		}
		for _, val := range appDistribution {
			totalApps += val
		}
	}

	return appDistribution, nil
}

func WaitAppCondition(argoClientSet *appclientset.Clientset, namespace string, commit string, status string, allAppsFlag bool) {
	apps, _ := argoClientSet.ArgoprojV1alpha1().Applications(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/generated-by=argocd-benchmark",
	})
	numApps := len(apps.Items)
	numAppsWithCorrectCondition := 0

	for exitCondition(numApps, numAppsWithCorrectCondition, allAppsFlag) {
		numAppsWithCorrectCondition = 0
		apps, _ := argoClientSet.ArgoprojV1alpha1().Applications(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/generated-by=argocd-benchmark",
		})
		for _, app := range apps.Items {
			correctStatus := true
			correctCommit := true
			if status != "" && string(app.Status.Sync.Status) != status {
				correctStatus = false
			}
			if commit != "" && string(app.Status.Sync.Revision) != commit {
				correctCommit = false
			}
			if correctCommit && correctStatus {
				numAppsWithCorrectCondition++
			}
		}
		time.Sleep(5 * time.Second)
	}
}

func exitCondition(numApps int, numAppsWithCorrectCondition int, allAppsFlag bool) bool {
	if allAppsFlag {
		return numApps != numAppsWithCorrectCondition
	} else {
		return numAppsWithCorrectCondition == 0
	}
}

// func GetClusterShards(replicas int, shardingAlgorithm string, namespace string, argoClientSet *appclientset.Clientset, argoDB db.ArgoDB) (map[int]ShardDistribution, error) {
// 	clusterByApps := make(map[string]int, replicas)
// 	clusterSharding := sharding.NewClusterSharding(-1, replicas, shardingAlgorithm, argoClientSet, namespace)
// 	clusters,_ := argoDB.ListClusters(context.TODO())
// 	appItems, err := argoClientSet.ArgoprojV1alpha1().Applications(namespace).List(context.TODO(), metav1.ListOptions{})
// 	if err != nil {
// 		return nil, err
// 	}
// 	apps := appItems.Items
// 	appsList := make([]interface{},len(appItems.Items))

// 	for i, app := range apps {
// 		err := argo.ValidateDestination(context.TODO(), &app.Spec.Destination, argoDB)
// 		if err == nil {
// 			apps[i] = app
// 			appsList = append(appsList, app)

// 			if _,ok := clusterByApps[app.Spec.Destination.Server]; !ok {
// 				clusterByApps[app.Spec.Destination.Server] = 0
// 			}
// 			clusterByApps[app.Spec.Destination.Server]++
// 		}
// 	}

// 	clusterSharding.Init(clusters, appsList)
// 	shards := clusterSharding.GetDistribution()

// 	shardDistribution := make(map[int]ShardDistribution, replicas)

// 	for cluster,shard := range shards {
// 		if shard == -1 {
// 			continue
// 		}
// 		entry,ok := shardDistribution[shard]
// 		if !ok {
// 			shardDistribution[shard] = ShardDistribution{
// 				Apps: 0,
// 				Clusters: 0,
// 			}
// 		}
// 		entry.Apps += clusterByApps[cluster]
// 		entry.Clusters++
// 		shardDistribution[shard] = entry
// 	}

// 	return shardDistribution,nil
// }

func GetEndpoint(clientSet *kubernetes.Clientset) string {
	log.Debug("Getting Gitea endpoint.")
	endpoints, _ := clientSet.CoreV1().Endpoints("gitea").List(context.TODO(), metav1.ListOptions{})
	var giteaEndpoint string

	for _, endpoint := range endpoints.Items {
		if endpoint.Name == "gitea-http" {
			giteaEndpoint = endpoint.Subsets[0].Addresses[0].IP
		}
	}

	return giteaEndpoint
}

func GetClusterList(argoDB db.ArgoDB) ([]appv1.Cluster, error) {
	log.Debug("Getting Cluster list.")
	validClusters := []appv1.Cluster{}
	clusters, _ := argoDB.ListClusters(context.TODO())
	for _, cluster := range clusters.Items {
		if cluster.Server != "https://kubernetes.default.svc" {
			validClusters = append(validClusters, cluster)
		}
	}
	return validClusters, nil
}
