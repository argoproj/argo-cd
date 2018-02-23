package commands

import (
	"log"
	"os"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	cliName = "argocd"
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
