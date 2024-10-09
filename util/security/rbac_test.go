package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_AppRBACName(t *testing.T) {
	testCases := []struct {
		name           string
		defaultNS      string
		project        string
		namespace      string
		appName        string
		expectedResult string
	}{
		{
			"namespace is empty",
			"argocd",
			"default",
			"",
			"app",
			"default/app",
		},
		{
			"namespace is default namespace",
			"argocd",
			"default",
			"argocd",
			"app",
			"default/app",
		},
		{
			"namespace is not default namespace",
			"argocd",
			"default",
			"test",
			"app",
			"default/test/app",
		},
	}

	for _, tc := range testCases {
		tcc := tc
		t.Run(tcc.name, func(t *testing.T) {
			t.Parallel()
			result := RBACName(tcc.defaultNS, tcc.project, tcc.namespace, tcc.appName)
			assert.Equal(t, tcc.expectedResult, result)
		})
	}
}
