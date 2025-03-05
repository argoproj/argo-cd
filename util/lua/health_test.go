package lua

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/util/errors"
)

type TestStructure struct {
	Tests []IndividualTest `yaml:"tests"`
}

type IndividualTest struct {
	InputPath    string              `yaml:"inputPath"`
	HealthStatus health.HealthStatus `yaml:"healthStatus"`
}

func getObj(t *testing.T, path string) *unstructured.Unstructured {
	t.Helper()
	yamlBytes, err := os.ReadFile(path)
	errors.NewHandler(t).CheckForErr(err)
	obj := make(map[string]any)
	err = yaml.Unmarshal(yamlBytes, &obj)
	errors.NewHandler(t).CheckForErr(err)

	return &unstructured.Unstructured{Object: obj}
}

func TestLuaHealthScript(t *testing.T) {
	err := filepath.Walk("../../resource_customizations", func(path string, _ os.FileInfo, err error) error {
		if !strings.Contains(path, "health.lua") {
			return nil
		}
		errors.NewHandler(t).CheckForErr(err)
		dir := filepath.Dir(path)
		yamlBytes, err := os.ReadFile(dir + "/health_test.yaml")
		errors.NewHandler(t).CheckForErr(err)
		var resourceTest TestStructure
		err = yaml.Unmarshal(yamlBytes, &resourceTest)
		errors.NewHandler(t).CheckForErr(err)
		for i := range resourceTest.Tests {
			test := resourceTest.Tests[i]
			t.Run(test.InputPath, func(t *testing.T) {
				vm := VM{
					UseOpenLibs: true,
				}
				obj := getObj(t, filepath.Join(dir, test.InputPath))
				script, _, err := vm.GetHealthScript(obj)
				errors.NewHandler(t).CheckForErr(err)
				result, err := vm.ExecuteHealthLua(obj, script)
				errors.NewHandler(t).CheckForErr(err)
				assert.Equal(t, &test.HealthStatus, result)
			})
		}
		return nil
	})
	assert.NoError(t, err)
}
