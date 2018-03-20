package util

import (
	"reflect"
	"testing"

	"github.com/pborman/uuid"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const testConfigMapNamespace = "default"

// makeTestConfigMapClientConfig creates an empty client config to use for K8s API calls
// TODO (@merenbach): make this more general for use in other tests
func makeTestConfigMapClientConfig() (config *rest.Config, err error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := clientcmd.ConfigOverrides{}
	configRaw := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &overrides)
	config, err = configRaw.ClientConfig()
	return
}

func TestConfigMapManager(t *testing.T) {
	configMapName := uuid.New()
	configMapData1 := map[string]string{
		"some":  "data",
		"hello": "world",
	}
	configMapData2 := map[string]string{
		"other": "data",
		"some":  "thing else",
		"world": "hello",
	}

	config, err := makeTestConfigMapClientConfig()
	if err != nil {
		t.Errorf("Could not create test client config: %v", err)
	}
	mgr, err := NewConfigMapManager(testConfigMapNamespace, config)
	if err != nil {
		t.Errorf("Could not create config map manager: %v", err)
	}
	configmap, err := mgr.CreateConfigMap(configMapName, configMapData1)
	if err != nil || !reflect.DeepEqual(configmap.Data, configMapData1) {
		t.Errorf("Err = %v; Created data did not match: had %v, wanted %v", err, configmap, configMapData1)
	}

	configmap, err = mgr.ReadConfigMap(configMapName)
	if err != nil || !reflect.DeepEqual(configmap.Data, configMapData1) {
		t.Errorf("Err = %v; Read data did not match: had %v, wanted %v", err, configmap, configMapData1)
	}

	configmap, err = mgr.UpdateConfigMap(configMapName, configMapData2)
	if err != nil || !reflect.DeepEqual(configmap.Data, configMapData2) {
		t.Errorf("Err = %v; Read data did not match: had %v, wanted %v", err, configmap, configMapData1)
	}

	err = mgr.DeleteConfigMap(configMapName)
	if err != nil {
		t.Errorf("Err = %v", err)
	}

	configmap, err = mgr.ReadConfigMap(configMapName)
	if err == nil {
		t.Errorf("Err = %v; Read data did not match: had %v, wanted nil; trying again, but it may need to be deleted manually", err, configmap)
		_ = mgr.DeleteConfigMap(configMapName)
	}
}
