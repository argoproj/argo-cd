package commands

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/util/assets"
)

func Test_isValidRBACAction(t *testing.T) {
	for k := range validRBACActions {
		t.Run(k, func(t *testing.T) {
			ok := isValidRBACAction(k)
			assert.True(t, ok)
		})
	}
	t.Run("invalid", func(t *testing.T) {
		ok := isValidRBACAction("invalid")
		assert.False(t, ok)
	})
}

func Test_isValidRBACResource(t *testing.T) {
	for k := range validRBACResources {
		t.Run(k, func(t *testing.T) {
			ok := isValidRBACResource(k)
			assert.True(t, ok)
		})
	}
	t.Run("invalid", func(t *testing.T) {
		ok := isValidRBACResource("invalid")
		assert.False(t, ok)
	})
}

func Test_PolicyFromCSV(t *testing.T) {
	uPol, dRole := getPolicy("testdata/rbac/policy.csv", nil, "")
	require.NotEmpty(t, uPol)
	require.Empty(t, dRole)
}

func Test_PolicyFromYAML(t *testing.T) {
	uPol, dRole := getPolicy("testdata/rbac/argocd-rbac-cm.yaml", nil, "")
	require.NotEmpty(t, uPol)
	require.Equal(t, "role:unknown", dRole)
}

func Test_PolicyFromK8s(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/rbac/policy.csv")
	require.NoError(t, err)
	kubeclientset := fake.NewSimpleClientset(&v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-rbac-cm",
			Namespace: "argocd",
		},
		Data: map[string]string{
			"policy.csv":     string(data),
			"policy.default": "role:unknown",
		},
	})
	uPol, dRole := getPolicy("", kubeclientset, "argocd")
	require.NotEmpty(t, uPol)
	require.Equal(t, "role:unknown", dRole)

	t.Run("get applications", func(t *testing.T) {
		ok := checkPolicy("role:user", "get", "applications", "*/*", assets.BuiltinPolicyCSV, uPol, dRole, true)
		require.True(t, ok)
	})
	t.Run("get clusters", func(t *testing.T) {
		ok := checkPolicy("role:user", "get", "clusters", "*", assets.BuiltinPolicyCSV, uPol, dRole, true)
		require.True(t, ok)
	})
	t.Run("get certificates", func(t *testing.T) {
		ok := checkPolicy("role:user", "get", "certificates", "*", assets.BuiltinPolicyCSV, uPol, dRole, true)
		require.False(t, ok)
	})
	t.Run("get certificates by default role", func(t *testing.T) {
		ok := checkPolicy("role:user", "get", "certificates", "*", assets.BuiltinPolicyCSV, uPol, "role:readonly", true)
		require.True(t, ok)
	})
	t.Run("get certificates by default role without builtin policy", func(t *testing.T) {
		ok := checkPolicy("role:user", "get", "certificates", "*", "", uPol, "role:readonly", true)
		require.False(t, ok)
	})
}
