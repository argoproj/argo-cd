package benchmark

import (
	"fmt"
	"os"
	"math"
	"time"
	"errors"
	"context"
	"text/tabwriter"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/hack/benchmark/util"
	"github.com/argoproj/argo-cd/v2/util/db"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"

	"k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SyncTest(clientSet *kubernetes.Clientset, argoClientSet *appclientset.Clientset, argoDB db.ArgoDB, namespace string, clusters []appv1.Cluster) (string, error) {
	apps,_ := argoClientSet.ArgoprojV1alpha1().Applications(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/generated-by=argocd-benchmark",
	})

	if len(apps.Items) == 0 {
		return "", errors.New("No applications found. Build the benchmark environment first before running a benchmark.")
	}

	argocdCM,_ := clientSet.CoreV1().ConfigMaps(namespace).Get(context.TODO(), "argocd-cmd-params-cm", metav1.GetOptions{})
	shardingAlgorithm := argocdCM.Data["controller.sharding.algorithm"]

	appControllerSTS,_ := clientSet.AppsV1().StatefulSets(namespace).Get(context.TODO(), "argocd-application-controller", metav1.GetOptions{})
	replicas := int(appControllerSTS.Status.Replicas)

	//shards,_ := util.GetClusterShards(replicas, shardingAlgorithm, namespace, argoClientSet, argoDB)
	log.Print("Running Sync test.")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Number of Clusters:\t\t%d\n", len(clusters))
	fmt.Fprintf(w, "Number of Applications:\t\t%d\n", len(apps.Items))
	fmt.Fprintf(w, "Status Processors:\t\t%s\n", argocdCM.Data["controller.status.processors"])
	fmt.Fprintf(w, "Operation Processors:\t\t%s\n", argocdCM.Data["controller.operation.processors"])
	fmt.Fprintf(w, "Sharding Algorithm:\t\t%s\n", shardingAlgorithm)
	fmt.Fprintf(w, "App Controller Replicas:\t\t%d\n", replicas)
	// fmt.Fprintf(w, "\nShard Distrubtion:\n")
	// fmt.Fprintf(w, "SHARD\t\tCLUSTERS\t\tAPPS\n")

	// for i:=0;i<replicas;i++ {
	// 	fmt.Fprintf(w,"%d\t\t%d\t\t%d\n", i+1, shards[i].Clusters, shards[i].Apps)
	// }
	_ = w.Flush()

	//Wait till all apps are synced
	util.WaitAppCondition(argoClientSet, namespace, "", "Synced", true)

	cmd,err := util.ForwardGit()
	if err != nil {
		return "", err
	}

	commit,err := util.PushGit()
	if err != nil {
		return "", err
	}

	util.WaitAppCondition(argoClientSet, namespace, commit, "", false)
	log.Print("Starting Test.")
	testStart := time.Now() 
	util.WaitAppCondition(argoClientSet, namespace, commit, "Synced", true)
	log.Print("All Apps Synced.")
	testEnd := time.Since(testStart)
	log.Printf("Elapsed Time: %d secs.", int(math.Floor(testEnd.Seconds())))
	cmd.Process.Kill()
	return "", nil
}

