package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_IsNamespaceEnabled(t *testing.T) {
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
		{
			"argocd namespace is not in the enabled list",
			"argocd",
			"argocd",
			[]string{"my-namespace"},
			false,
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
