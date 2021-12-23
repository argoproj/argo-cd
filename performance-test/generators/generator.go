package generator

import (
	"log"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"

	"k8s.io/client-go/tools/clientcmd"

	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
)

var labels = map[string]string{
	"app.kubernetes.io/generated-by": "argocd-generator",
}

type GenerateOpts struct {
	Samples     int
	GithubToken string
}

type Generator interface {
	Generate(opts *GenerateOpts) error
	Clean() error
}

func ConnectToK8s() *appclientset.Clientset {
	home, exists := os.LookupEnv("HOME")
	if !exists {
		home = "/root"
	}

	configPath := filepath.Join(home, ".kube", "cf-kubeconfig")

	config, err := clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		log.Panicln("failed to create K8s config")
	}

	return appclientset.NewForConfigOrDie(config)
}

func ConnectToK8s2() *kubernetes.Clientset {
	home, exists := os.LookupEnv("HOME")
	if !exists {
		home = "/root"
	}

	configPath := filepath.Join(home, ".kube", "cf-kubeconfig")

	config, err := clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		log.Panicln("failed to create K8s config")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Panicln("Failed to create K8s clientset")
	}

	return clientset
}
