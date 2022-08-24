package kube

import (
	"context"
	"testing"

	appv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj-labs/argocd-image-updater/test/fake"
	"github.com/argoproj-labs/argocd-image-updater/test/fixture"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewKubernetesClient(t *testing.T) {
	t.Run("Get new K8s client for remote cluster instance", func(t *testing.T) {
		client, err := NewKubernetesClientFromConfig(context.TODO(), "", "../../test/testdata/kubernetes/config")
		require.NoError(t, err)
		assert.NotNil(t, client)
		assert.Equal(t, "default", client.Namespace)
	})

	t.Run("Get new K8s client for remote cluster instance specified namespace", func(t *testing.T) {
		client, err := NewKubernetesClientFromConfig(context.TODO(), "argocd", "../../test/testdata/kubernetes/config")
		require.NoError(t, err)
		assert.NotNil(t, client)
		assert.Equal(t, "argocd", client.Namespace)
	})
}

func Test_GetDataFromSecrets(t *testing.T) {
	t.Run("Get all data from dummy secret", func(t *testing.T) {
		secret := fixture.MustCreateSecretFromFile("../../test/testdata/resources/dummy-secret.json")
		clientset := fake.NewFakeClientsetWithResources(secret)
		client := &KubernetesClient{Clientset: clientset}
		data, err := client.GetSecretData("test-namespace", "test-secret")
		require.NoError(t, err)
		require.NotNil(t, data)
		assert.Len(t, data, 1)
		assert.Equal(t, "argocd", string(data["namespace"]))
	})

	t.Run("Get string data from dummy secret existing field", func(t *testing.T) {
		secret := fixture.MustCreateSecretFromFile("../../test/testdata/resources/dummy-secret.json")
		clientset := fake.NewFakeClientsetWithResources(secret)
		client := &KubernetesClient{Clientset: clientset}
		data, err := client.GetSecretField("test-namespace", "test-secret", "namespace")
		require.NoError(t, err)
		assert.Equal(t, "argocd", data)
	})

	t.Run("Get string data from dummy secret non-existing field", func(t *testing.T) {
		secret := fixture.MustCreateSecretFromFile("../../test/testdata/resources/dummy-secret.json")
		clientset := fake.NewFakeClientsetWithResources(secret)
		client := &KubernetesClient{Clientset: clientset}
		data, err := client.GetSecretField("test-namespace", "test-secret", "nonexisting")
		require.Error(t, err)
		require.Empty(t, data)
	})

	t.Run("Get string data from non-existing secret non-existing field", func(t *testing.T) {
		secret := fixture.MustCreateSecretFromFile("../../test/testdata/resources/dummy-secret.json")
		clientset := fake.NewFakeClientsetWithResources(secret)
		client := &KubernetesClient{Clientset: clientset}
		data, err := client.GetSecretField("test-namespace", "test", "namespace")
		require.Error(t, err)
		require.Empty(t, data)
	})
}

func Test_CreateApplicationEvent(t *testing.T) {
	t.Run("Create Event", func(t *testing.T) {
		application := &appv1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-app",
				Namespace: "argocd",
			},
			Spec: appv1alpha1.ApplicationSpec{},
			Status: appv1alpha1.ApplicationStatus{
				Summary: appv1alpha1.ApplicationSummary{
					Images: []string{"nginx:1.12.2", "that/image", "quay.io/dexidp/dex:v1.23.0"},
				},
			},
		}
		annotations := map[string]string{
			"origin": "nginx:1.12.2",
		}
		clientset := fake.NewFakeClientsetWithResources()
		client := &KubernetesClient{Clientset: clientset, Namespace: "default"}
		event, err := client.CreateApplicationEvent(application, "TestEvent", "test-message", annotations)
		require.NoError(t, err)
		require.NotNil(t, event)
		assert.Equal(t, "ArgocdImageUpdater", event.Source.Component)
		assert.Equal(t, "default", client.Namespace)
	})
}
