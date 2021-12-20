package generator

import (
	"log"
	"os"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"

	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
)

type Generator interface {
	Generate() error
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
