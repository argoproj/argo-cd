package main

import (
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"strings"

	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/server/application"
	"github.com/argoproj/argo-cd/server/repository"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/git"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// origRepoURLToSecretName hashes repo URL to the secret name using a formula.
// Part of the original repo name is incorporated for debugging purposes
func origRepoURLToSecretName(repo string) string {
	repo = git.NormalizeGitURL(repo)
	h := fnv.New32a()
	_, _ = h.Write([]byte(repo))
	parts := strings.Split(strings.TrimSuffix(repo, ".git"), "/")
	return fmt.Sprintf("repo-%s-%v", strings.ToLower(parts[len(parts)-1]), h.Sum32())
}

// repoURLToSecretName hashes repo URL to the secret name using a formula.
// Part of the original repo name is incorporated for debugging purposes
func repoURLToSecretName(repo string) string {
	repo = strings.ToLower(git.NormalizeGitURL(repo))
	h := fnv.New32a()
	_, _ = h.Write([]byte(repo))
	parts := strings.Split(strings.TrimSuffix(repo, ".git"), "/")
	return fmt.Sprintf("repo-%s-%v", parts[len(parts)-1], h.Sum32())
}

// RenameSecret renames a Kubernetes secret in a given namespace.
func renameSecret(namespace, oldName, newName string) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := clientcmd.ConfigOverrides{}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &overrides)

	fmt.Printf("*** Renaming secret %q to %q in namespace %q\n", oldName, newName, namespace)

	config, err := clientConfig.ClientConfig()
	if err != nil {
		fmt.Println("Could not retrieve client config: ", err)
		return
	}

	kubeclientset := kubernetes.NewForConfigOrDie(config)
	repoSecret, err := kubeclientset.CoreV1().Secrets(namespace).Get(oldName, metav1.GetOptions{})
	if err != nil {
		fmt.Println("Could not retrieve old secret: ", err)
		return
	}

	repoSecret.ObjectMeta.Name = newName
	repoSecret.ObjectMeta.ResourceVersion = ""

	repoSecret, err = kubeclientset.CoreV1().Secrets(namespace).Create(repoSecret)
	if err != nil {
		fmt.Println("Could not create new secret: ", err)
		return
	}

	err = kubeclientset.CoreV1().Secrets(namespace).Delete(oldName, &metav1.DeleteOptions{})
	if err != nil {
		fmt.Println("Could not remove old secret: ", err)
	}
}

// RenameRepositorySecrets ensures that repository secrets use the new naming format.
func renameRepositorySecrets(clientOpts argocdclient.ClientOptions, namespace string) {
	conn, repoIf := argocdclient.NewClientOrDie(&clientOpts).NewRepoClientOrDie()
	defer util.Close(conn)
	repos, err := repoIf.List(context.Background(), &repository.RepoQuery{})
	if err != nil {
		fmt.Println("An error occurred, so skipping secret renaming: ", err)
		return
	}

	for _, repo := range repos.Items {
		oldSecretName := origRepoURLToSecretName(repo.Repo)
		newSecretName := repoURLToSecretName(repo.Repo)
		renameSecret(namespace, oldSecretName, newSecretName)
	}
}

// PopulateAppDestinations ensures that apps have a Server and Namespace set explicitly.
func populateAppDestinations(clientOpts argocdclient.ClientOptions) {
	conn, appIf := argocdclient.NewClientOrDie(&clientOpts).NewApplicationClientOrDie()
	defer util.Close(conn)
	apps, err := appIf.List(context.Background(), &application.ApplicationQuery{})
	if err != nil {
		fmt.Println("An error occurred, so skipping destination population: ", err)
		return
	}

	for _, app := range apps.Items {
		fmt.Printf("*** Ensuring destination field is populated on app %q\n", app.ObjectMeta.Name)
		if app.Spec.Destination.Server == "" {
			app.Spec.Destination.Server = app.Status.ComparisonResult.Server
		}
		if app.Spec.Destination.Namespace == "" {
			app.Spec.Destination.Namespace = app.Status.ComparisonResult.Namespace
		}

		_, err = appIf.UpdateSpec(context.Background(), &application.ApplicationSpecRequest{
			AppName: app.Name,
			Spec:    &app.Spec,
		})
		if err != nil {
			fmt.Println("An error occurred (but continuing anyway): ", err)
		}
	}
}

func main() {
	clientOpts := argocdclient.ClientOptions{
		ConfigPath: "",
		ServerAddr: os.Args[0],
		Insecure:   false,
		PlainText:  false,
	}
	renameSecrets(clientOpts, namespace)
	populateAppDestinations(clientOpts)
}
