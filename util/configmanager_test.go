package util

import (
	"reflect"
	"testing"

	apiv1 "k8s.io/api/core/v1"

	"github.com/pborman/uuid"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

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

func TestConfigManager(t *testing.T) {
	const namespace = "default"
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

	var (
		configMap  *apiv1.ConfigMap
		configMaps *apiv1.ConfigMapList
	)

	config, err := makeTestConfigMapClientConfig()
	if err != nil {
		t.Errorf("Could not create test client config: %v", err)
	}
	mgr, err := NewConfigManager(config)
	if err != nil {
		t.Errorf("Could not create config map manager: %v", err)
	}
	configMap, err = mgr.CreateConfigMap(namespace, configMapName, configMapData1)
	if err != nil || !reflect.DeepEqual(configMap.Data, configMapData1) {
		t.Errorf("Err = %v; Created data did not match: had %v, wanted %v", err, configMap.Data, configMapData1)
	}

	configMap, err = mgr.ReadConfigMap(namespace, configMapName)
	if err != nil || !reflect.DeepEqual(configMap.Data, configMapData1) {
		t.Errorf("Err = %v; Read data did not match: had %v, wanted %v", err, configMap.Data, configMapData1)
	}

	configMap, err = mgr.UpdateConfigMap(namespace, configMapName, configMapData2)
	if err != nil || !reflect.DeepEqual(configMap.Data, configMapData2) {
		t.Errorf("Err = %v; Updated data did not match: had %v, wanted %v", err, configMap.Data, configMapData1)
	}

	configMaps, err = mgr.ListConfigMaps(namespace)
	if err != nil || !reflect.DeepEqual(configMaps.Items[0], *configMap) {
		t.Errorf("Err = %v; Updated data in List did not match: had %v, wanted %v", err, configMaps.Items[0], configMap)
	}

	err = mgr.DeleteConfigMap(namespace, configMapName)
	if err != nil {
		t.Errorf("Err = %v", err)
	}

	configMap, err = mgr.ReadConfigMap(namespace, configMapName)
	if err == nil {
		t.Errorf("Read data did not match: had %v, wanted nil for name %s; trying again, but it may need to be deleted manually", configMap.Data, configMapName)
		_ = mgr.DeleteConfigMap(namespace, configMapName)
	}
}
