package util

import (
	"log"
	"os"
	"os/user"
	"path"

	"k8s.io/client-go/rest"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
)

type Kube struct {
	Namespace string
	Context   string
}

func getDefaultKubeConfigPath(homeDir string) string {
	return path.Join(homeDir, ".kube", "config")
}

func getKubeConfigPath() string {
	var kubeConfigPath string
	currentUser, _ := user.Current()
	if currentUser != nil {
		kubeConfigPath = os.Getenv("KUBECONFIG")
		if kubeConfigPath == "" {
			kubeConfigPath = getDefaultKubeConfigPath(currentUser.HomeDir)
		}
	}
	return kubeConfigPath
}

func ConnectToK8sArgoClientSet() *appclientset.Clientset {
	config, err := clientcmd.BuildConfigFromFlags("", getKubeConfigPath())
	if err != nil {
		log.Panicln("failed to create Argocd K8s config")
	}
	return appclientset.NewForConfigOrDie(config)
}

func ConnectToK8sConfig() *rest.Config {
	config, err := clientcmd.BuildConfigFromFlags("", getKubeConfigPath())
	if err != nil {
		log.Panicln("failed to create K8s config")
	}
	return config
}

func ConnectToK8sClientSet() *kubernetes.Clientset {
	config, err := clientcmd.BuildConfigFromFlags("", getKubeConfigPath())
	if err != nil {
		log.Panicln("failed to create K8s config")
	}
	return kubernetes.NewForConfigOrDie(config)
}
