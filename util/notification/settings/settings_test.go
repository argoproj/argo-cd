package settings

import (
	"fmt"
	"testing"

	"github.com/argoproj/notifications-engine/pkg/api"
	"github.com/argoproj/notifications-engine/pkg/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient/mocks"
	service "github.com/argoproj/argo-cd/v3/util/notification/argocd"
	servicemocks "github.com/argoproj/argo-cd/v3/util/notification/argocd/mocks"
)

const (
	testNamespace       = "default"
	testContextKey      = "test-context-key"
	testContextKeyValue = "test-context-key-value"
)

func TestInitGetVars(t *testing.T) {
	t.Parallel()
	notificationsCm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
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
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-notifications-secret",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"notification-secret": []byte("secret-value"),
		},
	}
	kubeclientset := fake.NewClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "argocd-notifications-cm",
		},
		Data: notificationsCm.Data,
	},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-notifications-secret",
				Namespace: testNamespace,
			},
			Data: notificationsSecret.Data,
		})
	mockRepoClient := &mocks.Clientset{RepoServerServiceClient: &mocks.RepoServerServiceClient{}}
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	argocdService, err := service.NewArgoCDService(kubeclientset, dynamicClient, testNamespace, mockRepoClient)
	require.NoError(t, err)
	t.Cleanup(argocdService.Close)
	config := api.Config{}
	testDestination := services.Destination{
		Service: "webhook",
	}
	emptyAppData := map[string]any{}

	varsProvider, _ := initGetVars(argocdService, nil, &config, &notificationsCm, &notificationsSecret)

	t.Run("Vars provider serves Application data on app key", func(t *testing.T) {
		t.Parallel()
		appData := map[string]any{
			"name": "app-name",
		}
		result := varsProvider(appData, testDestination)
		assert.NotNil(t, t, result["app"])
		assert.Equal(t, result["app"], appData)
	})
	t.Run("Vars provider serves notification context data on context key", func(t *testing.T) {
		t.Parallel()
		expectedContext := map[string]string{
			testContextKey:     testContextKeyValue,
			"notificationType": testDestination.Service,
		}
		result := varsProvider(emptyAppData, testDestination)
		assert.NotNil(t, result["context"])
		assert.Equal(t, expectedContext, result["context"])
	})
	t.Run("Vars provider serves notification secrets on secrets key", func(t *testing.T) {
		t.Parallel()
		result := varsProvider(emptyAppData, testDestination)
		assert.NotNil(t, result["secrets"])
		assert.Equal(t, result["secrets"], notificationsSecret.Data)
	})
	t.Run("Vars provider serves empty appProject when AppProject not found", func(t *testing.T) {
		t.Parallel()
		appData := map[string]any{
			"spec": map[string]any{
				"project": "nonexistent-project",
			},
			"metadata": map[string]any{
				"namespace": testNamespace,
				"name":      "my-app",
			},
		}
		result := varsProvider(appData, testDestination)
		assert.NotNil(t, result["appProject"])
		assert.Equal(t, map[string]any{}, result["appProject"])
	})
}

func TestInitGetVarsAppProject(t *testing.T) {
	t.Parallel()
	notificationsCm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "argocd-notifications-cm",
		},
		Data: map[string]string{},
	}
	notificationsSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-notifications-secret",
			Namespace: testNamespace,
		},
	}
	kubeclientset := fake.NewClientset(&notificationsCm, &notificationsSecret)
	mockRepoClient := &mocks.Clientset{RepoServerServiceClient: &mocks.RepoServerServiceClient{}}

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.SchemeBuilder.AddToScheme(scheme))

	appProject := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-project",
			Namespace: testNamespace,
		},
		Spec: v1alpha1.AppProjectSpec{
			Description: "test project description",
		},
	}
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, appProject)
	argocdService, err := service.NewArgoCDService(kubeclientset, dynamicClient, testNamespace, mockRepoClient)
	require.NoError(t, err)
	t.Cleanup(argocdService.Close)

	config := api.Config{}
	testDestination := services.Destination{Service: "webhook"}
	varsProvider, _ := initGetVars(argocdService, nil, &config, &notificationsCm, &notificationsSecret)

	appData := map[string]any{
		"spec": map[string]any{
			"project": "my-project",
		},
		"metadata": map[string]any{
			"namespace": testNamespace,
			"name":      "my-app",
		},
	}

	t.Run("Vars provider serves AppProject data on appProject key", func(t *testing.T) {
		t.Parallel()
		result := varsProvider(appData, testDestination)
		assert.NotNil(t, result["appProject"])
		proj, ok := result["appProject"].(map[string]any)
		require.True(t, ok)
		projMeta, ok := proj["metadata"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "my-project", projMeta["name"])
		assert.Equal(t, testNamespace, projMeta["namespace"])
	})

	t.Run("Vars provider appProject key is always present", func(t *testing.T) {
		// Even with empty app data, appProject key always be set
		t.Parallel()
		result := varsProvider(map[string]any{}, testDestination)
		_, exists := result["appProject"]
		assert.True(t, exists)
	})
}

// TestGetAppProjectForTemplateUsesCache proves that when an AppProjectGetter is
// provided, the AppProject is read from the cache and no live API call is made.
// The mock Service has no GetAppProject expectation configured, so it would
// panic if getAppProjectForTemplate fell back to the live lookup.
func TestGetAppProjectForTemplateUsesCache(t *testing.T) {
	t.Parallel()

	appData := map[string]any{
		"spec": map[string]any{
			"project": "my-project",
		},
		"metadata": map[string]any{
			"namespace": testNamespace,
			"name":      "my-app",
		},
	}

	cachedProject := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name":      "my-project",
				"namespace": testNamespace,
			},
		},
	}

	var gotNamespace, gotName string
	getter := func(namespace, name string) (*unstructured.Unstructured, error) {
		gotNamespace, gotName = namespace, name
		return cachedProject, nil
	}

	// A mock with no expectations: any live GetAppProject call fails the test.
	mockService := servicemocks.NewService(t)

	result := getAppProjectForTemplate(mockService, getter, appData)

	require.NotNil(t, result)
	assert.Equal(t, testNamespace, gotNamespace)
	assert.Equal(t, "my-project", gotName)
	projMeta, ok := result["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "my-project", projMeta["name"])
	assert.Equal(t, testNamespace, projMeta["namespace"])
	mockService.AssertNotCalled(t, "GetAppProject")
}

// TestGetAppProjectForTemplateCacheMiss proves that a cache miss returns nil
// without falling back to a live API call.
func TestGetAppProjectForTemplateCacheMiss(t *testing.T) {
	t.Parallel()

	appData := map[string]any{
		"spec": map[string]any{
			"project": "missing-project",
		},
		"metadata": map[string]any{
			"namespace": testNamespace,
			"name":      "my-app",
		},
	}

	getter := func(_, _ string) (*unstructured.Unstructured, error) {
		return nil, nil
	}

	mockService := servicemocks.NewService(t)

	result := getAppProjectForTemplate(mockService, getter, appData)

	assert.Nil(t, result)
	mockService.AssertNotCalled(t, "GetAppProject")
}
