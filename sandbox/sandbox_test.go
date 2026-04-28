package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetModules(t *testing.T) {
	type testCase struct {
		name   string
		spec   ArgocdSandboxConfig
		errmsg string
	}
	newTestCase := func(name string, spec ArgocdSandboxConfig, errmsg string) testCase {
		result := testCase{}
		result.name = name
		result.spec = spec
		result.errmsg = errmsg
		return result
	}

	testCases := []testCase{
		newTestCase("landlock", ArgocdSandboxConfig{Landlock: &LandlockConfig{DefaultFSDeny: "execute, read_file, read_dir"}}, ""),
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			modules, err := getModules(&testCase.spec)
			assert.NoError(t, err)
			assert.True(t, len(modules) == 1)
			assert.Equal(t, "landlock", modules[0].Name())
		})
	}
}
