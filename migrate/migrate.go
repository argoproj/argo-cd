package argocd

import (
	"bufio"
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"strings"

	"github.com/argoproj/argo-cd/server/application"
	"github.com/argoproj/argo-cd/server/repository"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/git"
	"k8s.io/client-go/kubernetes"
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

func renameSecrets(clientOpts argocdclient.ClientOptions) {
	config, err := clientConfig.ClientConfig()
	if err != nil {
		panic(err)
	}

	namespace, _, err := clientConfig.Namespace()
	if err != nil {
		panic(err)
	}

	kubeclientset := kubernetes.NewForConfigOrDie(config)

	conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoClientOrDie()
	defer util.Close(conn)
	repos, err := repoIf.List(context.Background(), &repository.RepoQuery{})
	if err != nil {
		panic(err)
	}

	for _, repo := range repos {
		oldSecretName := origRepoURLToSecretName(repo.Name)
		repoSecret, err = kubeclientset.CoreV1().Secrets(namespace).Get(oldSecretName, metav1.GetOptions{})
		if err != nil {
			panic(err)
		}

		newSecretName := repoURLToSecretName(repo.Name)
		repoSecret.ObjectMeta.Name = newSecretName

		repoSecret, err = kubeclientset.CoreV1().Secrets(namespace).Create(repoSecret)
		if err != nil {
			panic(err)
		}

		err := kubeclientset.CoreV1().Secrets(namespace).Delete(oldSecretName, &metav1.DeleteOptions{})
		if err != nil {
			panic(err)
		}
	}
}

// InputString requests an input from the user
// For security reasons, please do not use for password input
func inputString(prompt string, printArgs ...string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf(prompt, printArgs...)
	inputRaw, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(inputRaw)
}

func fillDestinations(clientOpts argocdclient.ClientOptions) {
	conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
	defer util.Close(conn)
	apps, err := appIf.List(context.Background(), &application.ApplicationQuery{})
	if err != nil {
		panic(err)
	}

	for _, app := range apps {
		if app.Spec.Destination.Server == "" {
			app.Spec.Destination.Server = inputString("Server for %s: ", appName)
		}
		if app.Spec.Destination.Namespace == "" {
			app.Spec.Destination.Namespace = inputString("Namespace for %s: ", appName)
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
		ServerAddr: server,
		Insecure:   globalClientOpts.Insecure,
		PlainText:  globalClientOpts.PlainText,
	}
	renameSecrets(clientOpts)
	fillDestinations(clientOpts)
}
