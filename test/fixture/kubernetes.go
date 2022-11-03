package fixture

import (
	"encoding/json"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AddPartOfArgoCDLabel(objs ...metav1.Object) {
	for _, obj := range objs {
		labels := obj.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels["app.kubernetes.io/part-of"] = "argocd"
		obj.SetLabels(labels)
	}
}

// NewSecret creates a new Kubernetes secret object in given namespace, with
// given name and with given data.
func NewSecret(namespace, name string, entries map[string][]byte) *v1.Secret {
	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Data: entries,
	}
	return &secret
}

// NewSecret creates a new Kubernetes secret object in given namespace, with
// given name and with given data.
func NewConfigMap(namespace, name string, entries map[string]string) *v1.ConfigMap {
	configMap := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Data: entries,
	}
	return &configMap
}

// MustCreateSecretFromFile reads a Kubernetes secret definition from filepath
// and returns a Secret object. Panics on error.
func MustCreateSecretFromFile(filepath string) *v1.Secret {
	jsonData := MustReadFile(filepath)
	return MustCreateSecretFromJson(jsonData)
}

// MustCreateSecretFromJson creates a Kubernetes secret from given JSON data
// and returns a Secret object. Panics on error.
func MustCreateSecretFromJson(jsonData string) *v1.Secret {
	var s v1.Secret
	err := json.Unmarshal([]byte(jsonData), &s)
	if err != nil {
		panic(err)
	}
	return &s
}
