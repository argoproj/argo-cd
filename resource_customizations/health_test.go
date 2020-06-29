package resource_customizations

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/utils/errors"
	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/util/lua"
)

type TestStructure struct {
	Tests []IndividualTest `yaml:"tests"`
}

type IndividualTest struct {
	InputPath    string              `yaml:"inputPath"`
	HealthStatus health.HealthStatus `yaml:"healthStatus"`
}

func getObj(path string) *unstructured.Unstructured {
	yamlBytes, err := ioutil.ReadFile(path)
	errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
	obj := make(map[string]interface{})
	err = yaml.Unmarshal(yamlBytes, &obj)
	errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
	return &unstructured.Unstructured{Object: obj}
}

func TestLuaHealthScript(t *testing.T) {
	err := filepath.Walk(".", func(path string, f os.FileInfo, err error) error {
		if !strings.Contains(path, "health.lua") {
			return nil
		}
		errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
		dir := filepath.Dir(path)
		yamlBytes, err := ioutil.ReadFile(dir + "/health_test.yaml")
		errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
		var resourceTest TestStructure
		err = yaml.Unmarshal(yamlBytes, &resourceTest)
		errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
		for i := range resourceTest.Tests {
			test := resourceTest.Tests[i]
			t.Run(test.InputPath, func(t *testing.T) {
				vm := lua.VM{
					UseOpenLibs: true,
				}
				obj := getObj(filepath.Join(dir, test.InputPath))
				script, err := vm.GetHealthScript(obj)
				errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
				result, err := vm.ExecuteHealthLua(obj, script)
				errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
				assert.Equal(t, &test.HealthStatus, result)
			})
		}
		return nil
	})
	assert.Nil(t, err)
}
