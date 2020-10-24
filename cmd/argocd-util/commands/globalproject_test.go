package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"k8s.io/client-go/tools/clientcmd"
)

func TestGlobalProjectGen(t *testing.T) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)

	globalProj := generateGlobalProject(clientConfig, "test_clusterrole.yaml")
	assert.True(t, len(globalProj.Spec.NamespaceResourceWhitelist) > 0)
}
