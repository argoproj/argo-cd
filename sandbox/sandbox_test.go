package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetModulesFromConfig(t *testing.T) {
	type testCase struct {
		name   string
		spec   ArgocdSandboxConfig
		impls  []string
		errmsg string
	}
	newTestCase := func(name string, spec ArgocdSandboxConfig, impls []string, errmsg string) testCase {
		result := testCase{}
		result.name = name
		result.spec = spec
		result.impls = impls
		result.errmsg = errmsg
		return result
	}

	testCases := []testCase{
		newTestCase("landlock", ArgocdSandboxConfig{Landlock: &LandlockConfig{DefaultFSDeny: "execute, read_file, read_dir"}}, []string{}, ""),
		newTestCase("landlock", ArgocdSandboxConfig{Landlock: &LandlockConfig{DefaultFSDeny: "execute, read_file, read_dir"}}, []string{"landlock"}, ""),
		newTestCase("landlock", ArgocdSandboxConfig{Landlock: &LandlockConfig{DefaultFSDeny: "execute, read_file, read_dir"}}, []string{"dummy"}, "No such sandbox module: \"dummy\""),
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			modules, err := getModulesFromConfig(&testCase.spec, testCase.impls)
			if testCase.errmsg == "" {
				require.NoError(t, err)
				assert.True(t, len(modules) == 1)
				assert.Equal(t, "landlock", modules[0].Name())
			} else {
				require.Error(t, err)
				assert.ErrorContains(t, err, testCase.errmsg)
			}
		})
	}
}
