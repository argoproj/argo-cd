package settings

import (
	"fmt"
	"testing"

	"github.com/argoproj/notifications-engine/pkg/api"
	"github.com/argoproj/notifications-engine/pkg/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v2/reposerver/apiclient/mocks"
	service "github.com/argoproj/argo-cd/v2/util/notification/argocd"
)

const (
	testNamespace       = "default"
	testContextKey      = "test-context-key"
	testContextKeyValue = "test-context-key-value"
)

func TestInitGetVars(t *testing.T) {
	notificationsCm := corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "argocd-notifications-cm",
		},
		Data: map[string]string{
			"context":              fmt.Sprintf("%s: %s", testContextKey, testContextKeyValue),
			"service.webhook.test": "url: https://test.example.com",
			"template.app-created": "email:\n  subject: Application {{.app.metadata.name}} has been created.\nmessage: Application {{.app.metadata.name}} has been created.\nteams:\n  title: Application {{.app.metadata.name}} has been created.\n",
			"trigger.on-created":   "- description: Application is created.\n  oncePer: app.metadata.name\n  send:\n  - app-created\n  when: \"true\"\n",
		},
	}
	notificationsSecret := corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "argocd-notifications-secret",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"notification-secret": []byte("secret-value"),
		},
	}
	kubeclientset := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "argocd-notifications-cm",
		},
		Data: notificationsCm.Data,
	},
		&corev1.Secret{
			ObjectMeta: v1.ObjectMeta{
				Name:      "argocd-notifications-secret",
				Namespace: testNamespace,
			},
			Data: notificationsSecret.Data,
		})
	mockRepoClient := &mocks.Clientset{RepoServerServiceClient: &mocks.RepoServerServiceClient{}}
	argocdService, err := service.NewArgoCDService(kubeclientset, testNamespace, mockRepoClient)
	require.NoError(t, err)
	defer argocdService.Close()
	config := api.Config{}
	testDestination := services.Destination{
		Service: "webhook",
	}
	emptyAppData := map[string]interface{}{}

	varsProvider, _ := initGetVars(argocdService, &config, &notificationsCm, &notificationsSecret)

	t.Run("Vars provider serves Application data on app key", func(t *testing.T) {
		appData := map[string]interface{}{
			"name": "app-name",
		}
		result := varsProvider(appData, testDestination)
		assert.NotNil(t, t, result["app"])
		assert.Equal(t, result["app"], appData)
	})
	t.Run("Vars provider serves notification context data on context key", func(t *testing.T) {
		expectedContext := map[string]string{
			testContextKey:     testContextKeyValue,
			"notificationType": testDestination.Service,
		}
		result := varsProvider(emptyAppData, testDestination)
		assert.NotNil(t, result["context"])
		assert.Equal(t, expectedContext, result["context"])
	})
	t.Run("Vars provider serves notification secrets on secrets key", func(t *testing.T) {
		result := varsProvider(emptyAppData, testDestination)
		assert.NotNil(t, result["secrets"])
		assert.Equal(t, result["secrets"], notificationsSecret.Data)
	})
}
