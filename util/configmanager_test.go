package util

import (
	"reflect"
	"testing"

	apiv1 "k8s.io/api/core/v1"

	"github.com/pborman/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// makeTestConfigMapClientConfig creates an empty client config to use for K8s API calls
// TODO (@merenbach): make this more general for use in other tests
func makeTestConfigManagerClientset() (clientset kubernetes.Interface, err error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := clientcmd.ConfigOverrides{}
	configRaw := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &overrides)
	config, err := configRaw.ClientConfig()
	if err == nil {
		clientset, err = kubernetes.NewForConfig(config)
	}
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

	clientset, err := makeTestConfigManagerClientset()
	if err != nil {
		t.Errorf("Could not create test client config: %v", err)
	}
	mgr := NewConfigManager(clientset)

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

// testConvertSecretStringData returns a mapping of a secret data as a map[string]string instead of map[string][]byte.
func testConvertSecretStringData(secret *apiv1.Secret) map[string]string {
	out := make(map[string]string)
	for k, v := range secret.Data {
		out[k] = string(v)
	}
	return out
}

func TestSecretManager(t *testing.T) {
	const namespace = "default"
	var (
		secret  *apiv1.Secret
		secrets *apiv1.SecretList
		err     error
	)

	secretName := uuid.New()
	label := "test"
	secretData1 := map[string]string{
		"some":  "data",
		"hello": "world",
	}
	secretData2 := map[string]string{
		"other": "data",
		"some":  "thing else",
		"world": "hello",
	}
	secretDataAfterUpdate := map[string]string{
		"other": "data",
		"some":  "thing else",
		"world": "hello",
		"hello": "world",
	}

	clientset, err := makeTestConfigManagerClientset()
	if err != nil {
		t.Errorf("Could not create test client config: %v", err)
	}
	mgr := NewConfigManager(clientset)

	secret, err = mgr.CreateSecret(namespace, secretName, secretData1, label)
	if secretDataRetrieved := testConvertSecretStringData(secret); !reflect.DeepEqual(secretDataRetrieved, secretData1) {
		t.Errorf("Err = %v; Created data did not match: had %v, wanted %v", err, secretDataRetrieved, secretData1)
	}

	secret, err = mgr.ReadSecret(namespace, secretName)
	if err != nil {
		t.Errorf("Could not read secret: %v", err)
	}

	if secretDataRetrieved := testConvertSecretStringData(secret); !reflect.DeepEqual(secretDataRetrieved, secretData1) {
		t.Errorf("Read data did not match: had %v, wanted %v", secretDataRetrieved, secretData1)
	}

	secret, err = mgr.UpdateSecret(namespace, secretName, secretData2)
	if err != nil {
		t.Errorf("Could not update secret: %v", err)
	}
	if secretDataRetrieved := testConvertSecretStringData(secret); !reflect.DeepEqual(secretDataRetrieved, secretDataAfterUpdate) {
		t.Errorf("Updated data did not match: had %v, wanted %v", secretDataRetrieved, secretData1)
	}

	secrets, err = mgr.ListSecrets(namespace)
	if err != nil || !reflect.DeepEqual(secrets.Items[0], *secret) {
		t.Errorf("Err = %v; Updated data in List did not match: had %v, wanted %v", err, secrets.Items[0], secret)
	}

	err = mgr.DeleteSecret(namespace, secretName)
	if err != nil {
		t.Errorf("Could not delete secret: %v", err)
	}

	secret, err = mgr.ReadSecret(namespace, secretName)
	if err == nil {
		secretDataRetrieved := testConvertSecretStringData(secret)
		t.Errorf("Read data did not match: had %v, wanted nil for name %s and label %s; trying again, but it may need to be deleted manually", secretDataRetrieved, secretName, label)
		_ = mgr.DeleteSecret(namespace, secretName)
	}
}
