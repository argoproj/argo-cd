package configbus

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/v3/common"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

func TestCRDResourcesFilterExclusions(t *testing.T) {
	cfg := &appv1.ArgoCDConfiguration{
		Spec: appv1.ArgoCDConfigurationSpec{
			Controller: &appv1.ControllerConfig{
				Resource: &appv1.ResourceConfig{
					Exclusions: []appv1.FilteredResource{
						{APIGroups: []string{"group1"}, Kinds: []string{"Kind1"}, Clusters: []string{"cluster1"}},
					},
				},
			},
		},
	}
	got, ok := crdResourcesFilterExclusions(cfg)
	require.True(t, ok)
	require.NotNil(t, got)
	assert.Equal(t, []settings.FilteredResource{
		{APIGroups: []string{"group1"}, Kinds: []string{"Kind1"}, Clusters: []string{"cluster1"}},
	}, got.ResourceExclusions)
	assert.Empty(t, got.ResourceInclusions)
}

func TestCRDResourcesFilterInclusions(t *testing.T) {
	cfg := &appv1.ArgoCDConfiguration{
		Spec: appv1.ArgoCDConfigurationSpec{
			Controller: &appv1.ControllerConfig{
				Resource: &appv1.ResourceConfig{
					Inclusions: []appv1.FilteredResource{
						{APIGroups: []string{"group2"}, Kinds: []string{"Kind2"}, Clusters: []string{"cluster2"}},
					},
				},
			},
		},
	}
	got, ok := crdResourcesFilterInclusions(cfg)
	require.True(t, ok)
	assert.Equal(t, []settings.FilteredResource{
		{APIGroups: []string{"group2"}, Kinds: []string{"Kind2"}, Clusters: []string{"cluster2"}},
	}, got.ResourceInclusions)
}

func TestCRDAccountsIncludesAdmin(t *testing.T) {
	cfg := &appv1.ArgoCDConfiguration{
		Spec: appv1.ArgoCDConfigurationSpec{
			Server: &appv1.ServerConfig{
				Accounts: []appv1.AccountConfig{
					{Name: common.ArgoCDAdminUsername, Capabilities: []string{"login"}},
					{Name: "ci", Capabilities: []string{"apiKey"}, Enabled: true},
				},
			},
		},
	}
	got, ok := crdAccounts(cfg)
	require.True(t, ok)
	admin, ok := got[common.ArgoCDAdminUsername]
	require.True(t, ok)
	assert.True(t, admin.Enabled)
	assert.Equal(t, []settings.AccountCapability{settings.AccountCapabilityLogin}, admin.Capabilities)
	ci := got["ci"]
	assert.True(t, ci.Enabled)
	assert.Equal(t, []settings.AccountCapability{settings.AccountCapabilityApiKey}, ci.Capabilities)

	adminMap, ok := crdAccounts(cfg)
	require.True(t, ok)
	assert.Equal(t, got, adminMap)
}

func TestCRDKustomizeVersions(t *testing.T) {
	cfg := &appv1.ArgoCDConfiguration{
		Spec: appv1.ArgoCDConfigurationSpec{
			RepoServer: &appv1.RepoServerConfig{
				Kustomize: &appv1.KustomizeConfig{
					BuildOptions: "--load-restrictor LoadRestrictionsNone",
					Versions: []appv1.KustomizeVersion{
						{Name: "5.0.0", Path: "/usr/local/bin/kustomize-5", BuildOptions: "--enable-alpha-plugins"},
					},
				},
			},
		},
	}
	got, ok := crdKustomizeVersions(cfg)
	require.True(t, ok)
	assert.Equal(t, "--load-restrictor LoadRestrictionsNone", got.BuildOptions)
	require.Len(t, got.Versions, 1)
	assert.Equal(t, "5.0.0", got.Versions[0].Name)
	assert.Equal(t, "/usr/local/bin/kustomize-5", got.Versions[0].Path)
	assert.Equal(t, "--enable-alpha-plugins", got.Versions[0].BuildOptions)
}

func TestCRDDexAndOIDCConfigYAML(t *testing.T) {
	dexBytes, err := json.Marshal(map[string]any{"clientID": "abc"})
	require.NoError(t, err)

	cfg := &appv1.ArgoCDConfiguration{
		Spec: appv1.ArgoCDConfigurationSpec{
			Server: &appv1.ServerConfig{
				Dex: &appv1.DexConfig{
					Connectors: []appv1.DexConnector{
						{
							Type:   "github",
							ID:     "github",
							Name:   "GitHub",
							Config: runtime.RawExtension{Raw: dexBytes},
						},
					},
				},
				OIDC: &appv1.OIDCConfig{
					Name:      "Okta",
					IssuerURL: "https://example.okta.com",
					ClientID:  "oidc-client",
					ClientSecretRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "argocd-secret"},
						Key:                  "oidc.okta.clientSecret",
					},
					RequestedScopes: []string{"openid", "groups"},
					Azure: &appv1.AzureOIDCConfig{
						UseWorkloadIdentity: true,
					},
					RefreshTokenThreshold: &metav1.Duration{Duration: 5 * time.Minute},
				},
			},
		},
	}

	dexYAML, ok, err := crdDexConfigYAML(cfg)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Contains(t, dexYAML, "connectors:")
	assert.Contains(t, dexYAML, "type: github")
	assert.Contains(t, dexYAML, "clientID: abc")

	oidcYAML, ok, err := crdOIDCConfigYAML(cfg)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Contains(t, oidcYAML, "name: Okta")
	assert.Contains(t, oidcYAML, "issuer: https://example.okta.com")
	assert.Contains(t, oidcYAML, "clientSecret: $secret:argocd-secret/oidc.okta.clientSecret")
	assert.Contains(t, oidcYAML, "refreshTokenThreshold: 5m0s")
	assert.Contains(t, oidcYAML, "useWorkloadIdentity: true")
}

func TestCRDDeepLinks(t *testing.T) {
	desc := "docs"
	icon := "fa fa-link"
	cond := "app.status.health.status == 'Healthy'"
	cfg := &appv1.ArgoCDConfiguration{
		Spec: appv1.ArgoCDConfigurationSpec{
			Server: &appv1.ServerConfig{
				DeepLinks: &appv1.DeepLinksConfig{
					Application: []appv1.DeepLink{{
						URLTemplate:   "https://example/{{.app.metadata.name}}",
						Title:         "Example",
						Description:   desc,
						IconClass:     icon,
						ConditionExpr: cond,
					}},
				},
			},
		},
	}
	links, ok := crdApplicationLinks(cfg)
	require.True(t, ok)
	require.Len(t, links, 1)
	assert.Equal(t, "https://example/{{.app.metadata.name}}", links[0].URL)
	assert.Equal(t, "Example", links[0].Title)
	require.NotNil(t, links[0].Description)
	assert.Equal(t, desc, *links[0].Description)
	require.NotNil(t, links[0].IconClass)
	assert.Equal(t, icon, *links[0].IconClass)
	require.NotNil(t, links[0].Condition)
	assert.Equal(t, cond, *links[0].Condition)
}

func TestCRDHelpDownloadAndExtensionConfig(t *testing.T) {
	keepAlive := metav1.Duration{Duration: 11 * time.Second}
	cfg := &appv1.ArgoCDConfiguration{
		Spec: appv1.ArgoCDConfigurationSpec{
			Server: &appv1.ServerConfig{
				Help: &appv1.HelpConfig{
					BinaryURLs: map[string]string{
						"darwin/amd64": "https://example.com/argocd-darwin-amd64",
						"linux/amd64":  "/download/argocd-linux-amd64",
					},
				},
				Extensions: []appv1.ExtensionConfig{
					{
						Name:      "external-backend",
						KeepAlive: &keepAlive,
						Services: []appv1.ExtensionService{
							{
								URL: "https://httpbin.org",
								Headers: []appv1.ExtensionHeader{
									{Name: "Authorization", Value: "$extension.auth.header"},
								},
							},
						},
					},
				},
			},
		},
	}

	help, ok := crdHelpDownload(cfg)
	require.True(t, ok)
	assert.Equal(t, "https://example.com/argocd-darwin-amd64", help.BinaryURLs["darwin/amd64"])

	ext, ok, err := crdExtensionConfig(cfg)
	require.NoError(t, err)
	require.True(t, ok)
	require.Contains(t, ext, "external-backend")
	assert.Contains(t, ext["external-backend"], "https://httpbin.org")
	assert.Contains(t, ext["external-backend"], "keepAlive")
	assert.Contains(t, ext["external-backend"], "11s")
}

func TestCRDGlobalProjects(t *testing.T) {
	cfg := &appv1.ArgoCDConfiguration{
		Spec: appv1.ArgoCDConfigurationSpec{
			Controller: &appv1.ControllerConfig{
				GlobalProjects: []appv1.GlobalProjectConfig{
					{
						ProjectName: "global",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"team": "platform"},
						},
					},
				},
			},
		},
	}
	got, ok := crdGlobalProjects(cfg)
	require.True(t, ok)
	require.Len(t, got, 1)
	assert.Equal(t, "global", got[0].ProjectName)
	assert.Equal(t, map[string]string{"team": "platform"}, got[0].LabelSelector.MatchLabels)
}
