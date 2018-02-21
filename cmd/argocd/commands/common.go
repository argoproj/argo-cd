package commands

import (
	"log"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	cliName = "argocd"
)

var (
// Parts of the image for installation
// These values may be overridden by the link flags during build
//imageNamespace = "argoproj"
//imageTag       = "latest"
)

// GetKubeConfig creates new kubernetes client config using specified config path and config overrides variables
func GetKubeConfig(configPath string, overrides clientcmd.ConfigOverrides) *rest.Config {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = configPath
	clientConfig := clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)

	var err error
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		log.Fatal(err)
	}
	return restConfig
}

// GetKubeClient creates new kubernetes client using specified config path and config overrides variables
func GetKubeClient(configPath string, overrides clientcmd.ConfigOverrides) *kubernetes.Clientset {
	restConfig := GetKubeConfig(configPath, overrides)
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Fatal(err)
	}
	return clientset
}
