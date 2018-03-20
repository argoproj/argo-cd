package util

import (
	"reflect"
	"testing"

	"github.com/pborman/uuid"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const testSecretNamespace = "default"

// makeTestSecretClientConfig creates an empty client config to use for K8s API calls
// TODO (@merenbach): make this more general for use in other tests
func makeTestSecretClientConfig() (config *rest.Config, err error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := clientcmd.ConfigOverrides{}
	configRaw := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &overrides)
	config, err = configRaw.ClientConfig()
	return
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
	var (
		secret *apiv1.Secret
		err    error
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

	config, err := makeTestSecretClientConfig()
	if err != nil {
		t.Errorf("Could not create test client config: %v", err)
	}
	mgr, err := NewSecretManager(testSecretNamespace, config)
	if err != nil {
		t.Errorf("Could not create secret manager: %v", err)
	}

	secret, err = mgr.Create(secretName, secretData1, label)
	if secretDataRetrieved := testConvertSecretStringData(secret); !reflect.DeepEqual(secretDataRetrieved, secretData1) {
		t.Errorf("Err = %v; Created data did not match: had %v, wanted %v", err, secretDataRetrieved, secretData1)
	}

	secret, err = mgr.Read(secretName)
	if err != nil {
		t.Errorf("Could not read secret: %v", err)
	}

	if secretDataRetrieved := testConvertSecretStringData(secret); !reflect.DeepEqual(secretDataRetrieved, secretData1) {
		t.Errorf("Read data did not match: had %v, wanted %v", secretDataRetrieved, secretData1)
	}

	secret, err = mgr.Update(secretName, secretData2)
	if err != nil {
		t.Errorf("Could not update secret: %v", err)
	}
	if secretDataRetrieved := testConvertSecretStringData(secret); !reflect.DeepEqual(secretDataRetrieved, secretDataAfterUpdate) {
		t.Errorf("Updated data did not match: had %v, wanted %v", secretDataRetrieved, secretData1)
	}

	err = mgr.Delete(secretName)
	if err != nil {
		t.Errorf("Could not delete secret: %v", err)
	}

	secret, err = mgr.Read(secretName)
	if err == nil {
		secretDataRetrieved := testConvertSecretStringData(secret)
		t.Errorf("Read data did not match: had %v, wanted nil for name %s and label %s; trying again, but it may need to be deleted manually", secretDataRetrieved, secretName, label)
		_ = mgr.Delete(secretName)
	}
}
