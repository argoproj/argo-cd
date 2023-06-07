package lua

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/cli"
)

type testNormalizer struct{}

func (t testNormalizer) Normalize(un *unstructured.Unstructured) error {
	if un == nil {
		return nil
	}
	switch un.GetKind() {
	case "DaemonSet", "Deployment", "StatefulSet":
		err := unstructured.SetNestedStringMap(un.Object, map[string]string{"kubectl.kubernetes.io/restartedAt": "0001-01-01T00:00:00Z"}, "spec", "template", "metadata", "annotations")
		if err != nil {
			return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
		}
	}
	switch un.GetKind() {
	case "Deployment":
		err := unstructured.SetNestedField(un.Object, nil, "status")
		if err != nil {
			return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
		}
		err = unstructured.SetNestedField(un.Object, nil, "metadata", "creationTimestamp")
		if err != nil {
			return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
		}
		err = unstructured.SetNestedField(un.Object, nil, "metadata", "generation")
		if err != nil {
			return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
		}
	case "Rollout":
		err := unstructured.SetNestedField(un.Object, nil, "spec", "restartAt")
		if err != nil {
			return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
		}
	case "ExternalSecret":
		err := unstructured.SetNestedStringMap(un.Object, map[string]string{"force-sync": "0001-01-01T00:00:00Z"}, "metadata", "annotations")
		if err != nil {
			return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
		}
	}
	return nil
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

func TestLuaResourceActionsScript(t *testing.T) {
	err := filepath.Walk("../../resource_customizations", func(path string, f os.FileInfo, err error) error {
		if !strings.Contains(path, "action_test.yaml") {
			return nil
		}
		assert.NoError(t, err)
		dir := filepath.Dir(path)
		//TODO: Change to path
		yamlBytes, err := os.ReadFile(dir + "/action_test.yaml")
		assert.NoError(t, err)
		var resourceTest ActionTestStructure
		err = yaml.Unmarshal(yamlBytes, &resourceTest)
		assert.NoError(t, err)
		for i := range resourceTest.DiscoveryTests {
			test := resourceTest.DiscoveryTests[i]
			testName := fmt.Sprintf("discovery/%s", test.InputPath)
			t.Run(testName, func(t *testing.T) {
				vm := VM{
					UseOpenLibs: true,
				}
				obj := getObj(filepath.Join(dir, test.InputPath))
				discoveryLua, err := vm.GetResourceActionDiscovery(obj)
				assert.NoError(t, err)
				result, err := vm.ExecuteResourceActionDiscovery(obj, discoveryLua)
				assert.NoError(t, err)
				for i := range result {
					assert.Contains(t, test.Result, result[i])
				}
			})
		}
		for i := range resourceTest.ActionTests {
			test := resourceTest.ActionTests[i]
			testName := fmt.Sprintf("actions/%s/%s", test.Action, test.InputPath)
			t.Run(testName, func(t *testing.T) {
				vm := VM{
					// Uncomment the following line if you need to use lua libraries debugging
					// purposes. Otherwise, leave this false to ensure tests reflect the same
					// privileges that API server has.
					//UseOpenLibs: true,
				}
				obj := getObj(filepath.Join(dir, test.InputPath))
				action, err := vm.GetResourceAction(obj, test.Action)
				assert.NoError(t, err)

				assert.NoError(t, err)
				result, err := vm.ExecuteResourceAction(obj, action.ActionLua)
				assert.NoError(t, err)

				expectedObj := getObj(filepath.Join(dir, test.ExpectedOutputPath))
				// Ideally, we would use a assert.Equal to detect the difference, but the Lua VM returns a object with float64 instead of the original int32.  As a result, the assert.Equal is never true despite that the change has been applied.
				diffResult, err := diff.Diff(expectedObj, result, diff.WithNormalizer(testNormalizer{}))
				assert.NoError(t, err)
				if diffResult.Modified {
					t.Error("Output does not match input:")
					err = cli.PrintDiff(test.Action, expectedObj, result)
					assert.NoError(t, err)
				}
			})
		}

		return nil
	})
	assert.Nil(t, err)
}
