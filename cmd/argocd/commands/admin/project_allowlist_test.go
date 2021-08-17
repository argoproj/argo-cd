package admin

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/undefinedlabs/go-mpatch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func TestProjectAllowListGen(t *testing.T) {
	useMock := true
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)

	if useMock {
		var patchClientConfig *mpatch.Patch
		patchClientConfig, err := mpatch.PatchInstanceMethodByName(reflect.TypeOf(clientConfig), "ClientConfig", func(*clientcmd.DeferredLoadingClientConfig) (*restclient.Config, error) {
			return nil, nil
		})
		assert.NoError(t, err)

		patch, err := mpatch.PatchMethod(discovery.NewDiscoveryClientForConfig, func(c *restclient.Config) (*discovery.DiscoveryClient, error) {
			return &discovery.DiscoveryClient{LegacyPrefix: "/api"}, nil
		})
		assert.NoError(t, err)

		var patchSeverPreferredResources *mpatch.Patch
		discoClient := &discovery.DiscoveryClient{}
		patchSeverPreferredResources, err = mpatch.PatchInstanceMethodByName(reflect.TypeOf(discoClient), "ServerPreferredResources", func(*discovery.DiscoveryClient) ([]*metav1.APIResourceList, error) {
			res := metav1.APIResource{
				Name: "services",
				Kind: "Service",
			}
			resourceList := []*metav1.APIResourceList{{APIResources: []metav1.APIResource{res}}}
			return resourceList, nil
		})
		assert.NoError(t, err)

		defer func() {
			err = patchClientConfig.Unpatch()
			assert.NoError(t, err)
			err = patch.Unpatch()
			assert.NoError(t, err)
			err = patchSeverPreferredResources.Unpatch()
			err = patch.Unpatch()
		}()
	}

	globalProj := generateProjectAllowList(clientConfig, "testdata/test_clusterrole.yaml", "testproj")
	assert.True(t, len(globalProj.Spec.NamespaceResourceWhitelist) > 0)
}
