package admin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestProjectAllowListGen(t *testing.T) {
	res := metav1.APIResource{
		Name:       "services",
		Kind:       "Service",
		Namespaced: true,
	}
	resourceList := []*metav1.APIResourceList{{APIResources: []metav1.APIResource{res}}}

	globalProj, err := generateProjectAllowList(resourceList, "testdata/test_clusterrole.yaml", "testproj")
	require.NoError(t, err)
	assert.NotEmpty(t, globalProj.Spec.NamespaceResourceWhitelist)
}

func TestProjectAllowListGen_MultiAPIGroupsAndDeduplication(t *testing.T) {
	serverResources := []*metav1.APIResourceList{
		{
			GroupVersion: "networking.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{Name: "ingresses", Kind: "Ingress", Namespaced: true},
			},
		},
		{
			GroupVersion: "extensions/v1beta1",
			APIResources: []metav1.APIResource{
				{Name: "ingresses", Kind: "Ingress", Namespaced: true},
			},
		},
	}

	crYAML := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: test-cr
rules:
  - apiGroups:
      - extensions
      - networking.k8s.io
    resources:
      - ingresses
    verbs:
      - create
  - apiGroups:
      - networking.k8s.io
    resources:
      - ingresses
    verbs:
      - create
`
	tmpFile := filepath.Join(t.TempDir(), "cr.yaml")
	require.NoError(t, os.WriteFile(tmpFile, []byte(crYAML), 0o600))

	proj, err := generateProjectAllowList(serverResources, tmpFile, "testproj")
	require.NoError(t, err)

	expected := []metav1.GroupKind{
		{Group: "extensions", Kind: "Ingress"},
		{Group: "networking.k8s.io", Kind: "Ingress"},
	}
	assert.ElementsMatch(t, expected, proj.Spec.NamespaceResourceWhitelist)
}

func TestProjectAllowListGen_WildcardVerbAndResource(t *testing.T) {
	serverResources := []*metav1.APIResourceList{
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Kind: "Deployment", Namespaced: true},
				{Name: "statefulsets", Kind: "StatefulSet", Namespaced: true},
			},
		},
	}

	crYAML := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: test-cr-wildcard
rules:
  - apiGroups:
      - apps
    resources:
      - "*"
    verbs:
      - "*"
`
	tmpFile := filepath.Join(t.TempDir(), "cr.yaml")
	require.NoError(t, os.WriteFile(tmpFile, []byte(crYAML), 0o600))

	proj, err := generateProjectAllowList(serverResources, tmpFile, "testproj")
	require.NoError(t, err)

	expected := []metav1.GroupKind{
		{Group: "apps", Kind: "Deployment"},
		{Group: "apps", Kind: "StatefulSet"},
	}
	assert.ElementsMatch(t, expected, proj.Spec.NamespaceResourceWhitelist)
}

func TestProjectAllowListGen_WildcardAPIGroupAndSkipsClusterScoped(t *testing.T) {
	serverResources := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "services", Kind: "Service", Namespaced: true},
				{Name: "namespaces", Kind: "Namespace", Namespaced: false},
			},
		},
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Kind: "Deployment", Namespaced: true},
			},
		},
		{
			GroupVersion: "rbac.authorization.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{Name: "clusterroles", Kind: "ClusterRole", Namespaced: false},
			},
		},
	}

	crYAML := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: test-cr-wildcard-all
rules:
  - apiGroups:
      - "*"
    resources:
      - "*"
    verbs:
      - "*"
`
	tmpFile := filepath.Join(t.TempDir(), "cr.yaml")
	require.NoError(t, os.WriteFile(tmpFile, []byte(crYAML), 0o600))

	proj, err := generateProjectAllowList(serverResources, tmpFile, "testproj")
	require.NoError(t, err)

	expected := []metav1.GroupKind{
		{Group: "", Kind: "Service"},
		{Group: "apps", Kind: "Deployment"},
	}
	assert.ElementsMatch(t, expected, proj.Spec.NamespaceResourceWhitelist)
}
