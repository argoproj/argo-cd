package settings

import (
	"context"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

const (
	testNamespace     = "default"
	resourceOverrides = `{
    "jsonPointers": [
        ""
    ],
    "jqPathExpressions": [
        ""
    ],
    "managedFieldsManagers": [
        ""
    ]
}`
)

func fixtures(ctx context.Context, data map[string]string) (*fake.Clientset, *settings.SettingsManager) {
	kubeClient := fake.NewClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: testNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: data,
	}, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-secret",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"admin.password":   []byte("test"),
			"server.secretkey": []byte("test"),
		},
	})
	settingsManager := settings.NewSettingsManager(ctx, kubeClient, testNamespace)
	return kubeClient, settingsManager
}

func TestSettingsServer(t *testing.T) {
	t.Parallel()
	newServer := func(data map[string]string) *Server {
		_, settingsMgr := fixtures(t.Context(), data)
		return NewServer(settingsMgr, nil, nil, false, false, false, false)
	}

	t.Run("TestGetInstallationID", func(t *testing.T) {
		t.Parallel()
		settingsServer := newServer(map[string]string{
			"installationID": "1234567890",
		})
		resp, err := settingsServer.Get(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, "1234567890", resp.InstallationID)
	})

	t.Run("TestGetInstallationIDNotSet", func(t *testing.T) {
		t.Parallel()
		settingsServer := newServer(map[string]string{})
		resp, err := settingsServer.Get(t.Context(), nil)
		require.NoError(t, err)
		assert.Empty(t, resp.InstallationID)
	})

	t.Run("TestGetTrackingMethod", func(t *testing.T) {
		t.Parallel()
		settingsServer := newServer(map[string]string{
			"application.resourceTrackingMethod": "annotation+label",
		})
		resp, err := settingsServer.Get(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, "annotation+label", resp.TrackingMethod)
	})

	t.Run("TestGetAppLabelKey", func(t *testing.T) {
		t.Parallel()
		settingsServer := newServer(map[string]string{
			"application.instanceLabelKey": "instance",
		})
		resp, err := settingsServer.Get(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, "instance", resp.AppLabelKey)
	})

	t.Run("TestGetResourceOverridesNotLoggedIn", func(t *testing.T) {
		t.Parallel()
		settingsServer := newServer(map[string]string{
			"resource.customizations.ignoreResourceUpdates.all": resourceOverrides,
		})
		resp, err := settingsServer.Get(t.Context(), nil)
		require.NoError(t, err)
		assert.Nil(t, resp.ResourceOverrides)
	})

	t.Run("TestGetResourceOverridesLoggedIn", func(t *testing.T) {
		t.Parallel()
		//nolint:staticcheck // it's ok to use built-in type string as key for value for testing purposes
		loggedInContext := context.WithValue(t.Context(), "claims", &jwt.MapClaims{"iss": "qux", "sub": "foo", "email": "bar", "groups": []string{"baz"}})
		settingsServer := newServer(map[string]string{
			"resource.customizations.ignoreResourceUpdates.all": resourceOverrides,
		})
		resp, err := settingsServer.Get(loggedInContext, nil)
		require.NoError(t, err)
		assert.NotNil(t, resp.ResourceOverrides)
		assert.NotEmpty(t, resp.ResourceOverrides["*/*"])
	})

	t.Run("TestGetKustomizeOptionsDoesNotExposeVersionsWhenNotLoggedIn", func(t *testing.T) {
		t.Parallel()
		settingsServer := newServer(map[string]string{
			"kustomize.buildOptions":        "--global",
			"kustomize.path.v1.2.3":         "/custom-tools/kustomize_1_2_3",
			"kustomize.buildOptions.v1.2.3": "--enable-helm",
		})

		resp, err := settingsServer.Get(t.Context(), nil)
		require.NoError(t, err)
		require.NotNil(t, resp.KustomizeOptions)
		assert.Equal(t, "--global", resp.KustomizeOptions.BuildOptions)
		assert.Empty(t, resp.KustomizeOptions.Versions)
	})

	t.Run("TestGetKustomizeOptionsIncludesVersionsWhenLoggedIn", func(t *testing.T) {
		t.Parallel()
		//nolint:staticcheck // it's ok to use built-in type string as key for value for testing purposes
		loggedInContext := context.WithValue(t.Context(), "claims", &jwt.MapClaims{"iss": "qux", "sub": "foo", "email": "bar", "groups": []string{"baz"}})
		settingsServer := newServer(map[string]string{
			"kustomize.buildOptions":        "--global",
			"kustomize.path.v1.2.3":         "/custom-tools/kustomize_1_2_3",
			"kustomize.buildOptions.v1.2.3": "--enable-helm",
		})

		resp, err := settingsServer.Get(loggedInContext, nil)
		require.NoError(t, err)
		require.NotNil(t, resp.KustomizeOptions)
		assert.Equal(t, "--global", resp.KustomizeOptions.BuildOptions)
		assert.Equal(t, []v1alpha1.KustomizeVersion{
			{
				Name:         "v1.2.3",
				Path:         "/custom-tools/kustomize_1_2_3",
				BuildOptions: "--enable-helm",
			},
		}, resp.KustomizeOptions.Versions)
	})
}
