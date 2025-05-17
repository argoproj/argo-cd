package lua

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/gitops-engine/pkg/diff"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	appsv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/cli"
)

type testNormalizer struct{}

func (t testNormalizer) Normalize(un *unstructured.Unstructured) error {
	if un == nil {
		return nil
	}
	if un.GetKind() == "Job" {
		err := unstructured.SetNestedField(un.Object, map[string]any{"name": "not sure why this works"}, "metadata")
		if err != nil {
			return fmt.Errorf("failed to normalize Job: %w", err)
		}
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
	case "ExternalSecret", "PushSecret":
		err := unstructured.SetNestedStringMap(un.Object, map[string]string{"force-sync": "0001-01-01T00:00:00Z"}, "metadata", "annotations")
		if err != nil {
			return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
		}
	case "Workflow":
		err := unstructured.SetNestedField(un.Object, nil, "metadata", "resourceVersion")
		if err != nil {
			return fmt.Errorf("failed to normalize Rollout: %w", err)
		}
		err = unstructured.SetNestedField(un.Object, nil, "metadata", "uid")
		if err != nil {
			return fmt.Errorf("failed to normalize Rollout: %w", err)
		}
		err = unstructured.SetNestedField(un.Object, nil, "metadata", "annotations", "workflows.argoproj.io/scheduled-time")
		if err != nil {
			return fmt.Errorf("failed to normalize Rollout: %w", err)
		}
	case "HelmRelease", "ImageRepository", "ImageUpdateAutomation", "Kustomization", "Receiver", "Bucket", "GitRepository", "HelmChart", "HelmRepository", "OCIRepository":
		err := unstructured.SetNestedStringMap(un.Object, map[string]string{"reconcile.fluxcd.io/requestedAt": "By Argo CD at: 0001-01-01T00:00:00"}, "metadata", "annotations")
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
	Action               string            `yaml:"action"`
	InputPath            string            `yaml:"inputPath"`
	ExpectedOutputPath   string            `yaml:"expectedOutputPath"`
	ExpectedErrorMessage string            `yaml:"expectedErrorMessage"`
	InputStr             string            `yaml:"input"`
	Parameters           map[string]string `yaml:"parameters"`
}

func TestLuaResourceActionsScript(t *testing.T) {
	err := filepath.Walk("../../resource_customizations", func(path string, _ os.FileInfo, err error) error {
		if !strings.Contains(path, "action_test.yaml") {
			return nil
		}
		require.NoError(t, err)
		dir := filepath.Dir(path)
		yamlBytes, err := os.ReadFile(filepath.Join(dir, "action_test.yaml"))
		require.NoError(t, err)
		var resourceTest ActionTestStructure
		err = yaml.Unmarshal(yamlBytes, &resourceTest)
		require.NoError(t, err)
		for i := range resourceTest.DiscoveryTests {
			test := resourceTest.DiscoveryTests[i]
			testName := "discovery/" + test.InputPath
			t.Run(testName, func(t *testing.T) {
				vm := VM{
					UseOpenLibs: true,
				}
				obj := getObj(t, filepath.Join(dir, test.InputPath))
				discoveryLua, err := vm.GetResourceActionDiscovery(obj)
				require.NoError(t, err)
				result, err := vm.ExecuteResourceActionDiscovery(obj, discoveryLua)
				require.NoError(t, err)
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
					// UseOpenLibs: true,
				}
				sourceObj := getObj(t, filepath.Join(dir, test.InputPath))
				action, err := vm.GetResourceAction(sourceObj, test.Action)

				require.NoError(t, err)

				// Log the action Lua script
				t.Logf("Action Lua script: %s", action.ActionLua)

				// Parse action parameters
				var params []*applicationpkg.ResourceActionParameters
				if test.Parameters != nil {
					for k, v := range test.Parameters {
						params = append(params, &applicationpkg.ResourceActionParameters{
							Name:  &k,
							Value: &v,
						})
					}
				}

				if len(params) > 0 {
					// Log the parameters
					t.Logf("Parameters: %+v", params)
				}

				require.NoError(t, err)
				impactedResources, err := vm.ExecuteResourceAction(sourceObj, action.ActionLua, params)

				// Handle expected errors
				if test.ExpectedErrorMessage != "" {
					assert.EqualError(t, err, test.ExpectedErrorMessage)
					return
				}

				require.NoError(t, err)

				// Treat the Lua expected output as a list
				expectedObjects := getExpectedObjectList(t, filepath.Join(dir, test.ExpectedOutputPath))

				for _, impactedResource := range impactedResources {
					result := impactedResource.UnstructuredObj

					// The expected output is a list of objects
					// Find the actual impacted resource in the expected output
					expectedObj := findFirstMatchingItem(expectedObjects.Items, func(u unstructured.Unstructured) bool {
						// Some resources' name is derived from the source object name, so the returned name is not actually equal to the testdata output name
						// Considering the resource found in the testdata output if its name starts with source object name
						// TODO: maybe this should use a normalizer function instead of hard-coding the resource specifics here
						if (result.GetKind() == "Job" && sourceObj.GetKind() == "CronJob") || (result.GetKind() == "Workflow" && (sourceObj.GetKind() == "CronWorkflow" || sourceObj.GetKind() == "WorkflowTemplate")) {
							return u.GroupVersionKind() == result.GroupVersionKind() && strings.HasPrefix(u.GetName(), sourceObj.GetName()) && u.GetNamespace() == result.GetNamespace()
						}
						return u.GroupVersionKind() == result.GroupVersionKind() && u.GetName() == result.GetName() && u.GetNamespace() == result.GetNamespace()
					})

					assert.NotNil(t, expectedObj)

					switch impactedResource.K8SOperation {
					// No default case since a not supported operation would have failed upon unmarshaling earlier
					case PatchOperation:
						// Patching is only allowed for the source resource, so the GVK + name + ns must be the same as the impacted resource
						assert.Equal(t, sourceObj.GroupVersionKind(), result.GroupVersionKind())
						assert.Equal(t, sourceObj.GetName(), result.GetName())
						assert.Equal(t, sourceObj.GetNamespace(), result.GetNamespace())
					case CreateOperation:
						switch result.GetKind() {
						case "Job":
						case "Workflow":
							// The name of the created resource is derived from the source object name, so the returned name is not actually equal to the testdata output name
							result.SetName(expectedObj.GetName())
						}
					}

					// Ideally, we would use a assert.Equal to detect the difference, but the Lua VM returns a object with float64 instead of the original int32.  As a result, the assert.Equal is never true despite that the change has been applied.
					diffResult, err := diff.Diff(expectedObj, result, diff.WithNormalizer(testNormalizer{}))
					require.NoError(t, err)
					if diffResult.Modified {
						t.Error("Output does not match input:")
						err = cli.PrintDiff(test.Action, expectedObj, result)
						require.NoError(t, err)
					}
				}
			})
		}

		return nil
	})
	require.NoError(t, err)
}

// Handling backward compatibility.
// The old-style actions return a single object in the expected output from testdata, so will wrap them in a list
func getExpectedObjectList(t *testing.T, path string) *unstructured.UnstructuredList {
	t.Helper()
	yamlBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	unstructuredList := &unstructured.UnstructuredList{}
	yamlString := bytes.NewBuffer(yamlBytes).String()
	if yamlString[0] == '-' {
		// The string represents a new-style action array output, where each member is a wrapper around a k8s unstructured resource
		objList := make([]map[string]any, 5)
		err = yaml.Unmarshal(yamlBytes, &objList)
		require.NoError(t, err)
		unstructuredList.Items = make([]unstructured.Unstructured, len(objList))
		// Append each map in objList to the Items field of the new object
		for i, obj := range objList {
			unstructuredObj, ok := obj["unstructuredObj"].(map[string]any)
			assert.True(t, ok, "Wrong type of unstructuredObj")
			unstructuredList.Items[i] = unstructured.Unstructured{Object: unstructuredObj}
		}
	} else {
		// The string represents an old-style action object output, which is a k8s unstructured resource
		obj := make(map[string]any)
		err = yaml.Unmarshal(yamlBytes, &obj)
		require.NoError(t, err)
		unstructuredList.Items = make([]unstructured.Unstructured, 1)
		unstructuredList.Items[0] = unstructured.Unstructured{Object: obj}
	}
	return unstructuredList
}

func findFirstMatchingItem(items []unstructured.Unstructured, f func(unstructured.Unstructured) bool) *unstructured.Unstructured {
	var matching *unstructured.Unstructured
	for _, item := range items {
		if f(item) {
			matching = &item
			break
		}
	}
	return matching
}
