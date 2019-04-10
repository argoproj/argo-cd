package resource_customizations

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/yudai/gojsondiff/formatter"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/errors"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/diff"
	"github.com/argoproj/argo-cd/util/lua"
)

type testNormalizer struct{}

func (t testNormalizer) Normalize(un *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return un, nil
}

type ActionTestStructure struct {
	DiscoveryTests []IndividualDiscoveryTest `yaml:"discoveryTests"`
	ActionTests    []IndividualActionTest    `yaml:"actionTests"`
}

type IndividualDiscoveryTest struct {
	InputPath string                  `yaml:"inputPath"`
	Result    []appsv1.ResourceAction `yaml:"result"`
}

type IndividualActionTest struct {
	Action             string `yaml:"action"`
	InputPath          string `yaml:"inputPath"`
	ExpectedOutputPath string `yaml:"expectedOutputPath"`
	InputStr           string `yaml:"input"`
}

func TestLuaCustomActionsScript(t *testing.T) {
	err := filepath.Walk(".", func(path string, f os.FileInfo, err error) error {
		if !strings.Contains(path, "action_test.yaml") {
			return nil
		}
		errors.CheckError(err)
		dir := filepath.Dir(path)
		//TODO: Change to path
		yamlBytes, err := ioutil.ReadFile(dir + "/action_test.yaml")
		errors.CheckError(err)
		var resourceTest ActionTestStructure
		err = yaml.Unmarshal(yamlBytes, &resourceTest)
		errors.CheckError(err)
		for i := range resourceTest.DiscoveryTests {
			test := resourceTest.DiscoveryTests[i]
			testName := fmt.Sprintf("discovery/%s", test.InputPath)
			t.Run(testName, func(t *testing.T) {
				vm := lua.VM{
					UseOpenLibs: true,
				}
				obj := getObj(filepath.Join(dir, test.InputPath))
				discoveryLua, err := vm.GetCustomActionDiscovery(obj)
				errors.CheckError(err)
				result, err := vm.ExecuteCustomActionDiscovery(obj, discoveryLua)
				errors.CheckError(err)
				assert.Equal(t, test.Result, result)
			})
		}
		for i := range resourceTest.ActionTests {
			test := resourceTest.ActionTests[i]
			testName := fmt.Sprintf("actions/%s/%s", test.Action, test.InputPath)
			t.Run(testName, func(t *testing.T) {
				vm := lua.VM{
					UseOpenLibs: true,
				}
				obj := getObj(filepath.Join(dir, test.InputPath))
				customAction, err := vm.GetCustomAction(obj, test.Action)
				errors.CheckError(err)
				result, err := vm.ExecuteCustomAction(obj, customAction.ActionLua)
				errors.CheckError(err)
				expectedObj := getObj(filepath.Join(dir, test.ExpectedOutputPath))
				// Ideally, we would use a assert.Equal to detect the difference, but the Lua VM returns a object with float64 instead of the originial int32.  As a result, the assert.Equal is never true despite that the change has been applied.
				diffResult := diff.Diff(expectedObj, result, testNormalizer{})
				if diffResult.Modified {
					output, err := diffResult.ASCIIFormat(expectedObj, formatter.AsciiFormatterConfig{})
					errors.CheckError(err)
					assert.Fail(t, "Output does not match input:", output)
				}
			})
		}

		return nil
	})
	assert.Nil(t, err)
}
