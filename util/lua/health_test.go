package lua

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
	name      string
	inputPath string
	expected  health.HealthStatus
}

func parseObj(t *testing.T, yamlBytes []byte) *unstructured.Unstructured {
	t.Helper()
	obj := make(map[string]any)
	err := yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)
	return &unstructured.Unstructured{Object: obj}
}

// getObj reads a YAML file and returns an unstructured object.
// Deprecated: Use parseObj with pre-read bytes instead, to support parallel subtests.
func getObj(t *testing.T, path string) *unstructured.Unstructured {
	t.Helper()
	yamlBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	return parseObj(t, yamlBytes)
}

func collectHealthTestCases(t *testing.T) []healthTestCase {
	t.Helper()
	var cases []healthTestCase
	err := filepath.WalkDir("../../resource_customizations", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Base(path) != "health.lua" {
			return nil
		}
		dir := filepath.Dir(path)
		testYAMLPath := filepath.Join(dir, "health_test.yaml")
		yamlBytes, err := os.ReadFile(testYAMLPath)
		if err != nil {
			return fmt.Errorf("reading %s: %w", testYAMLPath, err)
		}
		var resourceTest TestStructure
		if err := yaml.Unmarshal(yamlBytes, &resourceTest); err != nil {
			return fmt.Errorf("parsing %s: %w", testYAMLPath, err)
		}
		resourcePrefix, err := filepath.Rel("../../resource_customizations", dir)
		if err != nil {
			return fmt.Errorf("computing prefix from %s: %w", dir, err)
		}
		for _, test := range resourceTest.Tests {
			cases = append(cases, healthTestCase{
				name:      filepath.ToSlash(filepath.Join(resourcePrefix, test.InputPath)),
				inputPath: filepath.Join(dir, test.InputPath),
				expected:  test.HealthStatus,
			})
		}
		return nil
	})
	require.NoError(t, err)
	sort.Slice(cases, func(i, j int) bool { return cases[i].name < cases[j].name })
	return cases
}

func TestLuaHealthScript(t *testing.T) {
	cases := collectHealthTestCases(t)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			yamlBytes, err := os.ReadFile(tc.inputPath)
			require.NoError(t, err)
			obj := parseObj(t, yamlBytes)
			vm := VM{
				UseOpenLibs: true,
			}
			script, _, err := vm.GetHealthScript(obj)
			require.NoError(t, err)
			result, err := vm.ExecuteHealthLua(obj, script)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tc.expected, *result)
		})
	}
}
