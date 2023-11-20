package env

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/hack/benchmark/util"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/helm"

	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/kubernetes"

	"code.gitea.io/sdk/gitea"
)

func BuildEnv(clientSet *kubernetes.Clientset, argoClientSet *appclientset.Clientset, numapps int, appdist string, namespace string, validClusters []appv1.Cluster) (string, error) {
	log.Print("Starting build.")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Number of Apps:\t\t%d\n", numapps)
	fmt.Fprintf(w, "App Distribution:\t\t%s\n", appdist)
	cmd, _ := helm.NewCmd("/tmp", "v3", "")
	_, err := cmd.Freestyle(
		"upgrade",
		"--install",
		"gitea",
		"gitea",
		"--repo", "https://dl.gitea.com/charts/",
		"--set", "redis-cluster.enabled=false",
		"--set", "postgresql.enabled=false",
		"--set", "postgresql-ha.enabled=false",
		"--set", "persistence.enabled=false",
		"--set", "gitea.config.database.DB_TYPE=sqlite3",
		"--set", "gitea.config.session.PROVIDER=memory",
		"--set", "gitea.config.cache.ADAPTER=memory",
		"--set", "gitea.config.queue.TYPE=level",
		"--set", "gitea.admin.username=adminuser",
		"--set", "gitea.admin.password=password",
		"--namespace", "gitea",
		"--create-namespace",
		"--wait",
	)
	if err != nil {
		return "", err
	}

	forwardCmd, err := util.ForwardGit()
	if err != nil {
		return "", err
	}

	gitClient, err := gitea.NewClient("http://localhost:3000", gitea.SetBasicAuth("adminuser", "password"))
	if err != nil {
		return "", err
	}

	_, _, err = gitClient.CreateRepo(gitea.CreateRepoOption{
		Name:    "argobenchmark",
		Private: false,
	})
	if err != nil {
		log.Print(err)
	}

	_, err = util.PushGit()
	if err != nil {
		return "", err
	}

	log.Print("Deleting all applications created by previous benchmark environment.")
	appsToDelete, _ := argoClientSet.ArgoprojV1alpha1().Applications(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/generated-by=argocd-benchmark",
	})
	deleteErrors := 0
	for _, app := range appsToDelete.Items {
		err := argoClientSet.ArgoprojV1alpha1().Applications(namespace).Delete(context.TODO(), app.Name, metav1.DeleteOptions{})
		if err != nil {
			deleteErrors++
		}
	}

	if deleteErrors != 0 {
		return "", fmt.Errorf("There were %d delete errors.", deleteErrors)
	}

	var labels = map[string]string{
		"app.kubernetes.io/generated-by": "argocd-benchmark",
	}

	giteaEndpoint := util.GetEndpoint(clientSet)
	appDistribution, _ := util.GetAppDistribution(numapps, appdist, validClusters)
	numAppsCreated := 0

	log.Print("Creating applications...")
	for _, cluster := range validClusters {
		for i := 0; i < appDistribution[cluster.Name]; i++ {
			numAppsCreated++
			appToCreate := names.SimpleNameGenerator.GenerateName(fmt.Sprintf("%s-configmap2kb-%d", cluster.Name, i))
			_, err = argoClientSet.ArgoprojV1alpha1().Applications(namespace).Create(context.TODO(), &appv1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      appToCreate,
					Namespace: namespace,
					Labels:    labels,
				},
				Spec: appv1.ApplicationSpec{
					Project: "default",
					Destination: appv1.ApplicationDestination{
						Namespace: appToCreate,
						Server:    cluster.Server,
					},
					Source: &appv1.ApplicationSource{
						RepoURL:        "http://" + giteaEndpoint + ":3000/adminuser/argobenchmark/",
						Path:           "configmap2kb",
						TargetRevision: "master",
					},
					SyncPolicy: &appv1.SyncPolicy{
						Automated: &appv1.SyncPolicyAutomated{},
						SyncOptions: appv1.SyncOptions{
							"CreateNamespace=true",
						},
					},
				},
			}, metav1.CreateOptions{})
			if err != nil {
				log.Print(err)
			}
		}
	}
	log.Printf("Created %d applications", numAppsCreated)
	err = forwardCmd.Process.Kill()
	if err != nil {
		return "", err
	}
	return "success", nil
}
