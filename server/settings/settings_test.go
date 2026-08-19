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
	settingspkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/settings"
	"github.com/argoproj/argo-cd/v3/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v3/test"
	"github.com/argoproj/argo-cd/v3/util/assets"
	rbac "github.com/argoproj/argo-cd/v3/util/rbac"
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
		return NewServer(settingsMgr, nil, nil, nil, false, false, false, false)
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

	t.Run("TestGetLoginButtonTextNotLoggedIn", func(t *testing.T) {
		t.Parallel()
		settingsServer := newServer(map[string]string{
			"ui.loginButtonText": "Sign in with SSO",
		})
		resp, err := settingsServer.Get(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, "Sign in with SSO", resp.UiLoginButtonText)
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
}

func TestGetDexConfig(t *testing.T) {
	t.Parallel()
	newServer := func(data map[string]string) *Server {
		_, settingsMgr := fixtures(t.Context(), data)
		return NewServer(settingsMgr, nil, nil, nil, false, false, false, false)
	}

	const dexConfig = `connectors:
- type: oidc
  id: okta
  name: Okta
  config:
    issuer: https://example.okta.com
    clientID: aaaa
    clientSecret: bbbb
- type: oidc
  id: github-actions
  name: GitHub Actions
  config:
    issuer: https://token.actions.githubusercontent.com
`

	tests := []struct {
		name                       string
		dexAuthConnectorID         string
		expectedDexAuthConnectorID string
	}{
		{
			name:                       "no connector ID configured returns empty DexAuthConnectorID",
			dexAuthConnectorID:         "",
			expectedDexAuthConnectorID: "",
		},
		{
			name:                       "valid connector ID is returned",
			dexAuthConnectorID:         "okta",
			expectedDexAuthConnectorID: "okta",
		},
		{
			name:                       "unknown connector ID is dropped",
			dexAuthConnectorID:         "does-not-exist",
			expectedDexAuthConnectorID: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data := map[string]string{
				"url":        "http://localhost", // required for IsDexConfigured
				"dex.config": dexConfig,
			}
			if tc.dexAuthConnectorID != "" {
				data["dex.auth.connectorId"] = tc.dexAuthConnectorID
			}
			resp, err := newServer(data).Get(t.Context(), nil)
			require.NoError(t, err)
			require.NotNil(t, resp.DexConfig)

			// All connectors are always returned; only the forced connector ID varies.
			var ids []string
			for _, c := range resp.DexConfig.Connectors {
				ids = append(ids, c.ID)
			}
			assert.ElementsMatch(t, []string{"okta", "github-actions"}, ids)
			assert.Equal(t, tc.expectedDexAuthConnectorID, resp.DexConfig.DexAuthConnectorID)
		})
	}
}

func TestGetHealthChecks(t *testing.T) {
	t.Parallel()
	newServer := func(data map[string]string) *Server {
		_, settingsMgr := fixtures(t.Context(), data)
		return NewServer(settingsMgr, nil, nil, nil, false, false, false, false)
	}

	t.Run("TestGetHealthChecks_BuiltinsAndOverrides_LoggedIn", func(t *testing.T) {
		t.Parallel()
		//nolint:staticcheck // built-in type string key for testing context
		loggedInContext := context.WithValue(t.Context(), "claims", &jwt.MapClaims{"iss": "qux", "sub": "foo", "email": "bar"})
		settingsServer := newServer(map[string]string{
			"resource.customizations.health.custom.io_Widget": "hs = {status = 'Healthy'}\nreturn hs",
			"resource.customizations.health.apps_Deployment":  "hs = {status = 'Healthy', message = 'Custom'}\nreturn hs",
		})
		resp, err := settingsServer.GetHealthChecks(loggedInContext, nil)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotEmpty(t, resp.HealthChecks)

		var widgetItem, deployItem, serviceItem *settingspkg.HealthCheckItem
		for _, item := range resp.HealthChecks {
			if item.Key == "custom.io/Widget" {
				widgetItem = item
			} else if item.Key == "apps/Deployment" {
				deployItem = item
			} else if item.Key == "Service" {
				serviceItem = item
			}
		}

		require.NotNil(t, widgetItem)
		assert.Equal(t, "CustomLua", widgetItem.Origin)
		assert.Equal(t, "custom.io", widgetItem.Group)
		assert.Equal(t, "Widget", widgetItem.Kind)
		assert.NotEmpty(t, widgetItem.LuaScript)

		require.NotNil(t, deployItem)
		assert.Equal(t, "OverrideLua", deployItem.Origin)
		assert.NotEmpty(t, deployItem.LuaScript)

		require.NotNil(t, serviceItem)
		assert.Equal(t, "BuiltinGo", serviceItem.Origin)
		assert.Empty(t, serviceItem.LuaScript)
	})

	t.Run("TestGetHealthChecks_NotLoggedIn_RedactsLuaSource", func(t *testing.T) {
		t.Parallel()
		settingsServer := newServer(map[string]string{
			"resource.customizations.health.custom.io_Widget": "hs = {status = 'Healthy'}\nreturn hs",
		})
		resp, err := settingsServer.GetHealthChecks(t.Context(), nil)
		require.NoError(t, err)
		require.NotNil(t, resp)

		var widgetItem *settingspkg.HealthCheckItem
		for _, item := range resp.HealthChecks {
			if item.Key == "custom.io/Widget" {
				widgetItem = item
			}
		}
		require.NotNil(t, widgetItem)
		assert.Equal(t, "CustomLua", widgetItem.Origin)
		assert.Equal(t, "custom.io", widgetItem.Group)
		assert.Equal(t, "Widget", widgetItem.Kind)
		assert.Empty(t, widgetItem.LuaScript, "LuaScript must be redacted for unauthenticated users")
	})

	t.Run("TestGetHealthChecks_RBACEnforcer_AuthorizedVsUnauthorized", func(t *testing.T) {
		t.Parallel()
		kubeClient, settingsMgr := fixtures(t.Context(), map[string]string{
			"resource.customizations.health.custom.io_Widget": "hs = {status = 'Healthy'}\nreturn hs",
		})
		rbacCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDRBACConfigMapName,
				Namespace: testNamespace,
			},
			Data: map[string]string{
				"policy.csv": "p, role:settings-reader, settings, get, *, allow",
			},
		}
		_, err := kubeClient.CoreV1().ConfigMaps(testNamespace).Create(t.Context(), rbacCM, metav1.CreateOptions{})
		require.NoError(t, err)

		enforcer := rbac.NewEnforcer(kubeClient, testNamespace, common.ArgoCDRBACConfigMapName, nil)
		_ = enforcer.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
		rbacEnf := rbacpolicy.NewRBACPolicyEnforcer(enforcer, test.NewFakeProjLister())
		enforcer.SetClaimsEnforcerFunc(rbacEnf.EnforceClaims)
		settingsServer := NewServer(settingsMgr, nil, nil, enforcer, false, false, false, false)



		// 1. Authorized user with settings,get permission
		//nolint:staticcheck
		authCtx := context.WithValue(t.Context(), "claims", &jwt.MapClaims{"sub": "user1", "groups": []string{"role:settings-reader"}})
		authResp, err := settingsServer.GetHealthChecks(authCtx, nil)
		require.NoError(t, err)
		require.NotNil(t, authResp)

		var authWidget *settingspkg.HealthCheckItem
		for _, item := range authResp.HealthChecks {
			if item.Key == "custom.io/Widget" {
				authWidget = item
			}
		}
		require.NotNil(t, authWidget)
		assert.NotEmpty(t, authWidget.LuaScript, "Authorized user with settings,get must receive Lua script source")

		// 2. Unauthorized user without settings,get permission
		//nolint:staticcheck
		unauthCtx := context.WithValue(t.Context(), "claims", &jwt.MapClaims{"sub": "user2", "groups": []string{"role:dev-no-settings"}})
		unauthResp, err := settingsServer.GetHealthChecks(unauthCtx, nil)
		require.NoError(t, err)
		require.NotNil(t, unauthResp)

		var unauthWidget *settingspkg.HealthCheckItem
		for _, item := range unauthResp.HealthChecks {
			if item.Key == "custom.io/Widget" {
				unauthWidget = item
			}
		}
		require.NotNil(t, unauthWidget)
		assert.Equal(t, "CustomLua", unauthWidget.Origin, "Metadata remains visible to unauthorized users")
		assert.Empty(t, unauthWidget.LuaScript, "LuaScript source must be redacted for unauthorized users")
	})
}
