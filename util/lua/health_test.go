package lua

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/health"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type TestStructure struct {
	Tests []IndividualTest `yaml:"tests"`
}

type IndividualTest struct {
	InputPath    string              `yaml:"inputPath"`
	HealthStatus health.HealthStatus `yaml:"healthStatus"`
}

type healthTestCase struct {
	name     string
	obj      *unstructured.Unstructured
	expected health.HealthStatus
}

func parseObj(t *testing.T, yamlBytes []byte) *unstructured.Unstructured {
	t.Helper()
	obj := make(map[string]any)
	err := yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)
	return &unstructured.Unstructured{Object: obj}
}

func collectHealthTestCases(t *testing.T) []healthTestCase {
	t.Helper()
	var cases []healthTestCase
	err := filepath.Walk("../../resource_customizations", func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !strings.Contains(path, "health.lua") {
			return nil
		}
		dir := filepath.Dir(path)
		yamlBytes, err := os.ReadFile(filepath.Join(dir, "health_test.yaml"))
		if err != nil {
			return err
		}
		var resourceTest TestStructure
		if err := yaml.Unmarshal(yamlBytes, &resourceTest); err != nil {
			return err
		}
		resourcePrefix := strings.TrimPrefix(dir, "../../resource_customizations/")
		for _, test := range resourceTest.Tests {
			inputBytes, err := os.ReadFile(filepath.Join(dir, test.InputPath))
			if err != nil {
				return err
			}
			cases = append(cases, healthTestCase{
				name:     filepath.Join(resourcePrefix, test.InputPath),
				obj:      parseObj(t, inputBytes),
				expected: test.HealthStatus,
			})
		}
		return nil
	})
	require.NoError(t, err)
	return cases
}

func TestLuaHealthScript(t *testing.T) {
	t.Parallel()
	cases := collectHealthTestCases(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			vm := VM{
				UseOpenLibs: true,
			}
			script, _, err := vm.GetHealthScript(tc.obj)
			require.NoError(t, err)
			result, err := vm.ExecuteHealthLua(tc.obj, script)
			require.NoError(t, err)
			assert.Equal(t, &tc.expected, result)
		})
	}
}
