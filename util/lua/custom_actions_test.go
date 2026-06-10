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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/diff"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	appsv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/cli"
)

type testNormalizer struct{}

func (t testNormalizer) Normalize(un *unstructured.Unstructured) error {
	if un == nil {
		return nil
	}
	// Disambiguate resources by apiVersion group to avoid collisions on Kind names
	gv, err := schema.ParseGroupVersion(un.GetAPIVersion())
	if err != nil {
		return fmt.Errorf("failed to parse apiVersion for %s: %w", un.GetKind(), err)
	}
	group := gv.Group
	// First, group-specific, then kind-specific normalization
	switch group {
	case "batch":
		if un.GetKind() == "Job" {
			return t.normalizeJob(un)
		}
	case "apps":
		switch un.GetKind() {
		case "DaemonSet", "Deployment", "StatefulSet":
			if err := setRestartedAtAnnotationOnPodTemplate(un); err != nil {
				return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
			}
		}
		if un.GetKind() == "Deployment" {
			if err := unstructured.SetNestedField(un.Object, nil, "status"); err != nil {
				return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
			}
			if err := unstructured.SetNestedField(un.Object, nil, "metadata", "creationTimestamp"); err != nil {
				return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
			}
			if err := unstructured.SetNestedField(un.Object, nil, "metadata", "generation"); err != nil {
				return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
			}
		}
	case "argoproj.io":
		switch un.GetKind() {
		case "Rollout":
			if err := unstructured.SetNestedField(un.Object, nil, "spec", "restartAt"); err != nil {
				return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
			}
		case "Workflow":
			if err := unstructured.SetNestedField(un.Object, nil, "metadata", "resourceVersion"); err != nil {
				return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
			}
			if err := unstructured.SetNestedField(un.Object, nil, "metadata", "uid"); err != nil {
				return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
			}
			if err := unstructured.SetNestedField(un.Object, nil, "metadata", "annotations", "workflows.argoproj.io/scheduled-time"); err != nil {
				return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
			}
		}
	case "external-secrets.io":
		switch un.GetKind() {
		case "ExternalSecret", "PushSecret":
			if err := unstructured.SetNestedStringMap(un.Object, map[string]string{"force-sync": "0001-01-01T00:00:00Z"}, "metadata", "annotations"); err != nil {
				return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
			}
		}
	case "postgresql.cnpg.io":
		if un.GetKind() == "Cluster" {
			if err := setPgClusterAnnotations(un); err != nil {
				return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
			}
			if err := unstructured.SetNestedField(un.Object, nil, "status", "targetPrimaryTimestamp"); err != nil {
				return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
			}
		}
	case "helm.toolkit.fluxcd.io":
		if un.GetKind() == "HelmRelease" {
			if err := setFluxRequestedAtAnnotation(un); err != nil {
				return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
			}
		}
	case "source.toolkit.fluxcd.io":
		switch un.GetKind() {
		case "Bucket", "GitRepository", "HelmChart", "HelmRepository", "OCIRepository":
			if err := setFluxRequestedAtAnnotation(un); err != nil {
				return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
			}
		}
	case "image.toolkit.fluxcd.io":
		switch un.GetKind() {
		case "ImageRepository", "ImageUpdateAutomation":
			if err := setFluxRequestedAtAnnotation(un); err != nil {
				return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
			}
		}
	case "kustomize.toolkit.fluxcd.io":
		if un.GetKind() == "Kustomization" {
			if err := setFluxRequestedAtAnnotation(un); err != nil {
				return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
			}
		}
	case "notification.toolkit.fluxcd.io":
		if un.GetKind() == "Receiver" {
			if err := setFluxRequestedAtAnnotation(un); err != nil {
				return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
			}
		}
	}
	return nil
}

// Helper: normalize restart annotation on pod template used by apps workloads
func setRestartedAtAnnotationOnPodTemplate(un *unstructured.Unstructured) error {
	return unstructured.SetNestedStringMap(un.Object, map[string]string{"kubectl.kubernetes.io/restartedAt": "0001-01-01T00:00:00Z"}, "spec", "template", "metadata", "annotations")
}

// Helper: normalize Flux requestedAt annotation across FluxCD kinds
func setFluxRequestedAtAnnotation(un *unstructured.Unstructured) error {
	return unstructured.SetNestedStringMap(un.Object, map[string]string{"reconcile.fluxcd.io/requestedAt": "By Argo CD at: 0001-01-01T00:00:00"}, "metadata", "annotations")
}

// Helper: normalize PostgreSQL CNPG Cluster annotations while preserving existing ones
func setPgClusterAnnotations(un *unstructured.Unstructured) error {
	// Get existing annotations or create an empty map
	existingAnnotations, _, _ := unstructured.NestedStringMap(un.Object, "metadata", "annotations")
	if existingAnnotations == nil {
		existingAnnotations = make(map[string]string)
	}

	// Update only the specific keys
	existingAnnotations["cnpg.io/reloadedAt"] = "0001-01-01T00:00:00Z"
	existingAnnotations["kubectl.kubernetes.io/restartedAt"] = "0001-01-01T00:00:00Z"

	// Set the updated annotations back
	return unstructured.SetNestedStringMap(un.Object, existingAnnotations, "metadata", "annotations")
}

func (t testNormalizer) normalizeJob(un *unstructured.Unstructured) error {
	if conditions, exist, err := unstructured.NestedSlice(un.Object, "status", "conditions"); err != nil {
		return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
	} else if exist {
		changed := false
		for i := range conditions {
			condition := conditions[i].(map[string]any)
			cType := condition["type"].(string)
			if cType == "FailureTarget" {
				condition["lastTransitionTime"] = "0001-01-01T00:00:00Z"
				changed = true
			}
		}
		if changed {
			if err := unstructured.SetNestedSlice(un.Object, conditions, "status", "conditions"); err != nil {
				return fmt.Errorf("failed to normalize %s: %w", un.GetKind(), err)
			}
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

type discoveryActionTestCase struct {
	name     string
	obj      *unstructured.Unstructured
	expected []appsv1.ResourceAction
}

type actionScriptTestCase struct {
	name                 string
	action               string
	sourceObj            *unstructured.Unstructured
	parameters           map[string]string
	expectedErrorMessage string
	expectedObjects      *unstructured.UnstructuredList
}

func collectActionTestCases(t *testing.T) (discoveryCases []discoveryActionTestCase, actionCases []actionScriptTestCase) {
	t.Helper()
	err := filepath.Walk("../../resource_customizations", func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !strings.Contains(path, "action_test.yaml") {
			return nil
		}
		dir := filepath.Dir(path)
		yamlBytes, err := os.ReadFile(filepath.Join(dir, "action_test.yaml"))
		if err != nil {
			return err
		}
		var resourceTest ActionTestStructure
		if err := yaml.Unmarshal(yamlBytes, &resourceTest); err != nil {
			return err
		}
		for _, test := range resourceTest.DiscoveryTests {
			inputBytes, err := os.ReadFile(filepath.Join(dir, test.InputPath))
			if err != nil {
				return err
			}
			discoveryCases = append(discoveryCases, discoveryActionTestCase{
				name:     "discovery/" + test.InputPath,
				obj:      parseObj(t, inputBytes),
				expected: test.Result,
			})
		}
		for _, test := range resourceTest.ActionTests {
			inputBytes, err := os.ReadFile(filepath.Join(dir, test.InputPath))
			if err != nil {
				return err
			}
			tc := actionScriptTestCase{
				name:                 fmt.Sprintf("actions/%s/%s", test.Action, test.InputPath),
				action:               test.Action,
				sourceObj:            parseObj(t, inputBytes),
				parameters:           test.Parameters,
				expectedErrorMessage: test.ExpectedErrorMessage,
			}
			if test.ExpectedOutputPath != "" {
				expectedBytes, err := os.ReadFile(filepath.Join(dir, test.ExpectedOutputPath))
				if err != nil {
					return err
				}
				tc.expectedObjects = parseExpectedObjectList(t, expectedBytes)
			}
			actionCases = append(actionCases, tc)
		}
		return nil
	})
	require.NoError(t, err)
	return discoveryCases, actionCases
}

func runDiscoveryActionTestCase(t *testing.T, tc discoveryActionTestCase) {
	t.Helper()
	vm := VM{
		UseOpenLibs: true,
	}
	discoveryLua, err := vm.GetResourceActionDiscovery(tc.obj)
	require.NoError(t, err)
	result, err := vm.ExecuteResourceActionDiscovery(tc.obj, discoveryLua)
	require.NoError(t, err)
	for i := range result {
		assert.Contains(t, tc.expected, result[i])
	}
}

func runActionScriptTestCase(t *testing.T, tc actionScriptTestCase) {
	t.Helper()
	vm := VM{
		// Uncomment the following line if you need to use lua libraries debugging
		// purposes. Otherwise, leave this false to ensure tests reflect the same
		// privileges that API server has.
		// UseOpenLibs: true,
	}
	action, err := vm.GetResourceAction(tc.sourceObj, tc.action)
	require.NoError(t, err)

	t.Logf("Action Lua script: %s", action.ActionLua)

	var params []*applicationpkg.ResourceActionParameters
	if tc.parameters != nil {
		for k, v := range tc.parameters {
			params = append(params, &applicationpkg.ResourceActionParameters{
				Name:  &k,
				Value: &v,
			})
		}
	}
	if len(params) > 0 {
		t.Logf("Parameters: %+v", params)
	}

	impactedResources, err := vm.ExecuteResourceAction(tc.sourceObj, action.ActionLua, params)
	if tc.expectedErrorMessage != "" {
		assert.EqualError(t, err, tc.expectedErrorMessage)
		return
	}
	require.NoError(t, err)

	for _, impactedResource := range impactedResources {
		result := impactedResource.UnstructuredObj
		expectedObj := findFirstMatchingItem(tc.expectedObjects.Items, func(u unstructured.Unstructured) bool {
			if (result.GetKind() == "Job" && tc.sourceObj.GetKind() == "CronJob") || (result.GetKind() == "Workflow" && (tc.sourceObj.GetKind() == "CronWorkflow" || tc.sourceObj.GetKind() == "WorkflowTemplate")) {
				return u.GroupVersionKind() == result.GroupVersionKind() && strings.HasPrefix(u.GetName(), tc.sourceObj.GetName()) && u.GetNamespace() == result.GetNamespace()
			}
			return u.GroupVersionKind() == result.GroupVersionKind() && u.GetName() == result.GetName() && u.GetNamespace() == result.GetNamespace()
		})
		assert.NotNil(t, expectedObj)

		switch impactedResource.K8SOperation {
		case PatchOperation:
			assert.Equal(t, tc.sourceObj.GroupVersionKind(), result.GroupVersionKind())
			assert.Equal(t, tc.sourceObj.GetName(), result.GetName())
			assert.Equal(t, tc.sourceObj.GetNamespace(), result.GetNamespace())
		case CreateOperation:
			switch result.GetKind() {
			case "Job", "Workflow":
				result.SetName(expectedObj.GetName())
			}
		}

		diffResult, err := diff.Diff(expectedObj, result, diff.WithNormalizer(testNormalizer{}))
		require.NoError(t, err)
		if diffResult.Modified {
			t.Error("Output does not match input:")
			err = cli.PrintDiff(tc.action, expectedObj, result)
			require.NoError(t, err)
		}
	}
}

func TestLuaResourceActionsScript(t *testing.T) {
	discoveryCases, actionCases := collectActionTestCases(t)
	for _, tc := range discoveryCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runDiscoveryActionTestCase(t, tc)
		})
	}

	for _, tc := range actionCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runActionScriptTestCase(t, tc)
		})
	}
}

// Handling backward compatibility.
// The old-style actions return a single object in the expected output from testdata, so will wrap them in a list
func parseExpectedObjectList(t *testing.T, yamlBytes []byte) *unstructured.UnstructuredList {
	t.Helper()
	unstructuredList := &unstructured.UnstructuredList{}
	yamlString := bytes.NewBuffer(yamlBytes).String()
	if yamlString[0] == '-' {
		objList := make([]map[string]any, 5)
		err := yaml.Unmarshal(yamlBytes, &objList)
		require.NoError(t, err)
		unstructuredList.Items = make([]unstructured.Unstructured, len(objList))
		for i, obj := range objList {
			unstructuredObj, ok := obj["unstructuredObj"].(map[string]any)
			assert.True(t, ok, "Wrong type of unstructuredObj")
			unstructuredList.Items[i] = unstructured.Unstructured{Object: unstructuredObj}
		}
	} else {
		obj := make(map[string]any)
		err := yaml.Unmarshal(yamlBytes, &obj)
		require.NoError(t, err)
		unstructuredList.Items = make([]unstructured.Unstructured, 1)
		unstructuredList.Items[0] = unstructured.Unstructured{Object: obj}
	}
	return unstructuredList
}

func getExpectedObjectList(t *testing.T, path string) *unstructured.UnstructuredList {
	t.Helper()
	yamlBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	return parseExpectedObjectList(t, yamlBytes)
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
