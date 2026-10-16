package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_IsNamespaceEnabled(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		namespace         string
		serverNamespace   string
		enabledNamespaces []string
		expectedResult    bool
	}{
		{
			"namespace is empty",
			"argocd",
			"argocd",
			[]string{},
			true,
		},
		{
			"namespace is explicitly server namespace",
			"argocd",
			"argocd",
			[]string{},
			true,
		},
		{
			"namespace is allowed namespace",
			"allowed",
			"argocd",
			[]string{"allowed"},
			true,
		},
		{
			"namespace matches pattern",
			"test-ns",
			"argocd",
			[]string{"test-*"},
			true,
		},
		{
			"namespace is not allowed namespace",
			"disallowed",
			"argocd",
			[]string{"allowed"},
			false,
		},
		{
			"match everything but specified word: fail",
			"disallowed",
			"argocd",
			[]string{"/^((?!disallowed).)*$/"},
			false,
		},
		{
			"match everything but specified word: pass",
			"allowed",
			"argocd",
			[]string{"/^((?!disallowed).)*$/"},
			true,
		},
	}

	for _, tc := range testCases {
		tcc := tc
		t.Run(tcc.name, func(t *testing.T) {
			t.Parallel()
			result := IsNamespaceEnabled(tcc.namespace, tcc.serverNamespace, tcc.enabledNamespaces)
			assert.Equal(t, tcc.expectedResult, result)
		})
	}
}

func Test_ApplicationNamespaceSet_Add(t *testing.T) {
	nsSet := ApplicationNamespaceSet{
		namespaces: []string{},
	}

	nsSet.Add("test-ns1")
	nsSet.Add("test-ns2")
	nsSet.Add("test-ns1")

	assert.Len(t, nsSet.namespaces, 2)
	assert.Equal(t, "test-ns1", nsSet.namespaces[0])
	assert.Equal(t, "test-ns2", nsSet.namespaces[1])
}

func Test_ApplicationNamespaceSet_List(t *testing.T) {
	namespaces := []string{"test-ns1", "test-ns2"}
	nsSet := ApplicationNamespaceSet{
		namespaces: namespaces,
	}

	assert.Equal(t, namespaces, nsSet.List())
}

func Test_ApplicationNamespaceSet_Delete(t *testing.T) {
	nsSet := ApplicationNamespaceSet{
		namespaces: []string{"test-ns1", "test-ns3"},
	}

	nsSet.Delete("test-ns2")

	assert.Len(t, nsSet.namespaces, 2)
	assert.Equal(t, "test-ns1", nsSet.namespaces[0])
	assert.Equal(t, "test-ns3", nsSet.namespaces[1])
}
