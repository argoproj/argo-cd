package normalizers

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test"
)

func TestNormalizeObjectWithMatchedGroupKind(t *testing.T) {
	normalizer, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:        "apps",
		Kind:         "Deployment",
		JSONPointers: []string{"/not-matching-path", "/spec/template/spec/containers"},
	}}, make(map[string]v1alpha1.ResourceOverride), IgnoreNormalizerOpts{})

	require.NoError(t, err)

	deployment := test.NewDeployment()

	_, has, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	require.NoError(t, err)
	assert.True(t, has)

	err = normalizer.Normalize(deployment)
	require.NoError(t, err)
	_, has, err = unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	require.NoError(t, err)
	assert.False(t, has)

	err = normalizer.Normalize(nil)
	require.Error(t, err)
}

func TestNormalizeNoMatchedGroupKinds(t *testing.T) {
	normalizer, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:        "",
		Kind:         "Service",
		JSONPointers: []string{"/spec"},
	}}, make(map[string]v1alpha1.ResourceOverride), IgnoreNormalizerOpts{})

	require.NoError(t, err)

	deployment := test.NewDeployment()

	err = normalizer.Normalize(deployment)
	require.NoError(t, err)

	_, hasSpec, err := unstructured.NestedMap(deployment.Object, "spec")
	require.NoError(t, err)
	assert.True(t, hasSpec)
}

func TestNormalizeMatchedResourceOverrides(t *testing.T) {
	normalizer, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{}, map[string]v1alpha1.ResourceOverride{
		"apps/Deployment": {
			IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{JSONPointers: []string{"/spec/template/spec/containers"}},
		},
	}, IgnoreNormalizerOpts{})

	require.NoError(t, err)

	deployment := test.NewDeployment()

	_, has, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	require.NoError(t, err)
	assert.True(t, has)

	err = normalizer.Normalize(deployment)
	require.NoError(t, err)
	_, has, err = unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	require.NoError(t, err)
	assert.False(t, has)
}

const testCRDYAML = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: certificates.cert-manager.io
spec:
  conversion:
    strategy: None
  group: cert-manager.io
  names:
    kind: Certificate
    listKind: CertificateList
    plural: certificates
    shortNames:
    - cert
    - certs
    singular: certificate
  scope: Namespaced
  version: v1alpha1
  versions:
  - name: v1alpha1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          json:
            x-kubernetes-preserve-unknown-fields: true`

func TestNormalizeMissingJsonPointer(t *testing.T) {
	normalizer, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{}, map[string]v1alpha1.ResourceOverride{
		"apps/Deployment": {
			IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{JSONPointers: []string{"/garbage"}},
		},
		"apiextensions.k8s.io/CustomResourceDefinition": {
			IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{JSONPointers: []string{"/spec/additionalPrinterColumns/0/priority"}},
		},
	}, IgnoreNormalizerOpts{})
	require.NoError(t, err)

	deployment := test.NewDeployment()

	err = normalizer.Normalize(deployment)
	require.NoError(t, err)

	crd := unstructured.Unstructured{}
	err = yaml.Unmarshal([]byte(testCRDYAML), &crd)
	require.NoError(t, err)

	err = normalizer.Normalize(&crd)
	require.NoError(t, err)
}

func TestNormalizeGlobMatch(t *testing.T) {
	normalizer, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{}, map[string]v1alpha1.ResourceOverride{
		"*/*": {
			IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{JSONPointers: []string{"/spec/template/spec/containers"}},
		},
	}, IgnoreNormalizerOpts{})

	require.NoError(t, err)

	deployment := test.NewDeployment()

	_, has, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	require.NoError(t, err)
	assert.True(t, has)

	err = normalizer.Normalize(deployment)
	require.NoError(t, err)
	_, has, err = unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	require.NoError(t, err)
	assert.False(t, has)
}

func TestNormalizeJQPathExpression(t *testing.T) {
	normalizer, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:             "apps",
		Kind:              "Deployment",
		JQPathExpressions: []string{".spec.template.spec.initContainers[] | select(.name == \"init-container-0\")"},
	}}, make(map[string]v1alpha1.ResourceOverride), IgnoreNormalizerOpts{})

	require.NoError(t, err)

	deployment := test.NewDeployment()

	var initContainers []interface{}
	initContainers = append(initContainers, map[string]interface{}{"name": "init-container-0"})
	initContainers = append(initContainers, map[string]interface{}{"name": "init-container-1"})
	err = unstructured.SetNestedSlice(deployment.Object, initContainers, "spec", "template", "spec", "initContainers")
	require.NoError(t, err)

	actualInitContainers, has, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "initContainers")
	require.NoError(t, err)
	assert.True(t, has)
	assert.Len(t, actualInitContainers, 2)

	err = normalizer.Normalize(deployment)
	require.NoError(t, err)
	actualInitContainers, has, err = unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "initContainers")
	require.NoError(t, err)
	assert.True(t, has)
	assert.Len(t, actualInitContainers, 1)

	actualInitContainerName, has, err := unstructured.NestedString(actualInitContainers[0].(map[string]interface{}), "name")
	require.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, "init-container-1", actualInitContainerName)
}

func TestNormalizeIllegalJQPathExpression(t *testing.T) {
	_, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:             "apps",
		Kind:              "Deployment",
		JQPathExpressions: []string{".spec.template.spec.containers[] | select(.name == \"missing-quote)"},
		// JSONPointers: []string{"no-starting-slash"},
	}}, make(map[string]v1alpha1.ResourceOverride), IgnoreNormalizerOpts{})

	require.Error(t, err)
}

func TestNormalizeJQPathExpressionWithError(t *testing.T) {
	normalizer, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:             "apps",
		Kind:              "Deployment",
		JQPathExpressions: []string{".spec.fakeField.foo[]"},
	}}, make(map[string]v1alpha1.ResourceOverride), IgnoreNormalizerOpts{})

	require.NoError(t, err)

	deployment := test.NewDeployment()
	originalDeployment, err := deployment.MarshalJSON()
	require.NoError(t, err)

	err = normalizer.Normalize(deployment)
	require.NoError(t, err)

	normalizedDeployment, err := deployment.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, originalDeployment, normalizedDeployment)
}

func TestNormalizeExpectedErrorAreSilenced(t *testing.T) {
	normalizer, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{}, map[string]v1alpha1.ResourceOverride{
		"*/*": {
			IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{
				JSONPointers: []string{"/invalid", "/invalid/json/path"},
			},
		},
	}, IgnoreNormalizerOpts{})
	require.NoError(t, err)

	ignoreNormalizer := normalizer.(*ignoreNormalizer)
	assert.Len(t, ignoreNormalizer.patches, 2)
	jsonPatch := ignoreNormalizer.patches[0]
	jqPatch := ignoreNormalizer.patches[1]

	deployment := test.NewDeployment()
	deploymentData, err := json.Marshal(deployment)
	require.NoError(t, err)

	// Error: "error in remove for path: '/invalid': Unable to remove nonexistent key: invalid: missing value"
	_, err = jsonPatch.Apply(deploymentData)
	assert.False(t, shouldLogError(err))

	// Error: "remove operation does not apply: doc is missing path: \"/invalid/json/path\": missing value"
	_, err = jqPatch.Apply(deploymentData)
	assert.False(t, shouldLogError(err))

	assert.True(t, shouldLogError(fmt.Errorf("An error that should not be ignored")))
}

func TestJqPathExpressionFailWithTimeout(t *testing.T) {
	normalizer, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{}, map[string]v1alpha1.ResourceOverride{
		"*/*": {
			IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{
				JQPathExpressions: []string{"until(true==false; [.] + [1])"},
			},
		},
	}, IgnoreNormalizerOpts{})
	require.NoError(t, err)

	ignoreNormalizer := normalizer.(*ignoreNormalizer)
	assert.Len(t, ignoreNormalizer.patches, 1)
	jqPatch := ignoreNormalizer.patches[0]

	deployment := test.NewDeployment()
	deploymentData, err := json.Marshal(deployment)
	require.NoError(t, err)

	_, err = jqPatch.Apply(deploymentData)
	assert.ErrorContains(t, err, "JQ patch execution timed out")
}

func TestJQPathExpressionReturnsHelpfulError(t *testing.T) {
	normalizer, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Kind: "ConfigMap",
		// This is a really wild expression, but it does trigger the desired error.
		JQPathExpressions: []string{`.nothing) | .data["config.yaml"] |= (fromjson | del(.auth) | tojson`},
	}}, nil, IgnoreNormalizerOpts{})

	require.NoError(t, err)

	configMap := test.NewConfigMap()
	require.NoError(t, err)

	out := test.CaptureLogEntries(func() {
		err = normalizer.Normalize(configMap)
		require.NoError(t, err)
	})
	assert.Contains(t, out, "fromjson cannot be applied")
}
