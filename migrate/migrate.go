package main

import (
	"context"

	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/server/application"
	"github.com/argoproj/argo-cd/server/repository"
	"github.com/argoproj/argo-cd/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	restclient "k8s.io/client-go/rest"
)

func renameSecret(clientOpts argocdclient.ClientOptions, oldName, newName string) {
	clientConfig := &restclient.Config{}

	config, err := clientConfig.ClientConfig()
	if err != nil {
		panic(err)
	}

	namespace := "default"
	// namespace, _, err := clientConfig.Namespace()
	// if err != nil {
	// 	panic(err)
	// }

	kubeclientset := kubernetes.NewForConfigOrDie(config)
	repoSecret, err := kubeclientset.CoreV1().Secrets(namespace).Get(oldName, metav1.GetOptions{})
	if err != nil {
		panic(err)
	}

	repoSecret.ObjectMeta.Name = newName

	repoSecret, err = kubeclientset.CoreV1().Secrets(namespace).Create(repoSecret)
	if err != nil {
		panic(err)
	}

	err = kubeclientset.CoreV1().Secrets(namespace).Delete(oldName, &metav1.DeleteOptions{})
	if err != nil {
		panic(err)
	}
}

func renameSecrets(clientOpts argocdclient.ClientOptions) {
	conn, repoIf := argocdclient.NewClientOrDie(&clientOpts).NewRepoClientOrDie()
	defer util.Close(conn)
	repos, err := repoIf.List(context.Background(), &repository.RepoQuery{})
	if err != nil {
		panic(err)
	}

	for _, repo := range repos.Items {
		oldSecretName := origRepoURLToSecretName(repo.Repo)
		newSecretName := repoURLToSecretName(repo.Repo)
		renameSecret(clientOpts, oldSecretName, newSecretName)
	}
}

func fillDestinations(clientOpts argocdclient.ClientOptions) {
	conn, appIf := argocdclient.NewClientOrDie(&clientOpts).NewApplicationClientOrDie()
	defer util.Close(conn)
	apps, err := appIf.List(context.Background(), &application.ApplicationQuery{})
	if err != nil {
		panic(err)
	}

	for _, app := range apps.Items {
		if app.Spec.Destination.Server == "" {
			app.Spec.Destination.Server = inputString("Server for %s: ", app.Name)
		}
		if app.Spec.Destination.Namespace == "" {
			app.Spec.Destination.Namespace = inputString("Namespace for %s: ", app.Name)
		}

		_, err = appIf.UpdateSpec(context.Background(), &application.ApplicationSpecRequest{
			AppName: app.Name,
			Spec:    &app.Spec,
		})
		if err != nil {
			panic(err)
		}
	}
}

func main() {
	clientOpts := argocdclient.ClientOptions{
		ConfigPath: "",
		ServerAddr: "127.0.0.1:8080",
		Insecure:   false,
		PlainText:  false,
	}
	renameSecret(clientOpts, "mysecret1", "mysecret2")
	// renameSecrets(clientOpts)
	// fillDestinations(clientOpts)
}
