package normalizers

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test"
)

func TestNormalizeObjectWithMatchedGroupKind(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	normalizer, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:             "apps",
		Kind:              "Deployment",
		JQPathExpressions: []string{".spec.template.spec.initContainers[] | select(.name == \"init-container-0\")"},
	}}, make(map[string]v1alpha1.ResourceOverride), IgnoreNormalizerOpts{})

	require.NoError(t, err)

	deployment := test.NewDeployment()

	var initContainers []any
	initContainers = append(initContainers, map[string]any{"name": "init-container-0"})
	initContainers = append(initContainers, map[string]any{"name": "init-container-1"})
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

	actualInitContainerName, has, err := unstructured.NestedString(actualInitContainers[0].(map[string]any), "name")
	require.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, "init-container-1", actualInitContainerName)
}

func TestNormalizeIllegalJQPathExpression(t *testing.T) {
	t.Parallel()
	_, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:             "apps",
		Kind:              "Deployment",
		JQPathExpressions: []string{".spec.template.spec.containers[] | select(.name == \"missing-quote)"},
		// JSONPointers: []string{"no-starting-slash"},
	}}, make(map[string]v1alpha1.ResourceOverride), IgnoreNormalizerOpts{})

	require.Error(t, err)
}

func TestNormalizeJQPathExpressionWithError(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

	assert.True(t, shouldLogError(errors.New("An error that should not be ignored")))
}

func TestJqPathExpressionFailWithTimeout(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestNormalizeFailureLogIncludesResourceContext(t *testing.T) {
	t.Parallel()
	// When a normalization patch fails with a non-silenced error, the log entry
	// must identify which resource was being normalized so operators can act on it.
	// Regression test for https://github.com/argoproj/argo-cd/issues/14148.
	normalizer, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Kind:              "ConfigMap",
		JQPathExpressions: []string{`.nothing) | .data["config.yaml"] |= (fromjson | del(.auth) | tojson`},
	}}, nil, IgnoreNormalizerOpts{})
	require.NoError(t, err)

	configMap := test.NewConfigMap()

	out := test.CaptureLogEntries(func() {
		err = normalizer.Normalize(configMap)
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Failed to apply normalization patch")
	assert.Contains(t, out, "kind=ConfigMap")
	assert.Contains(t, out, "name=my-configmap")
}

func TestIgnoredPaths(t *testing.T) {
	t.Parallel()
	newNormalizer := func(t *testing.T, ignore v1alpha1.ResourceIgnoreDifferences) IgnoreNormalizer {
		t.Helper()
		n, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{ignore}, nil, IgnoreNormalizerOpts{})
		require.NoError(t, err)
		return n
	}

	t.Run("json pointers resolve to map keys and list indexes", func(t *testing.T) {
		t.Parallel()
		n := newNormalizer(t, v1alpha1.ResourceIgnoreDifferences{
			Group: "apps", Kind: "Deployment",
			JSONPointers: []string{"/spec/replicas", "/spec/template/spec/containers/0/image", "/spec/does/not/exist"},
		})
		deployment := test.NewDeployment()

		paths, err := n.IgnoredPaths(deployment)

		require.NoError(t, err)
		assert.ElementsMatch(t, [][]any{
			{"spec", "replicas"},
			{"spec", "template", "spec", "containers", 0, "image"},
		}, paths)
	})

	t.Run("escaped json pointer segments are unescaped", func(t *testing.T) {
		t.Parallel()
		n := newNormalizer(t, v1alpha1.ResourceIgnoreDifferences{
			Group: "apps", Kind: "Deployment",
			JSONPointers: []string{"/metadata/labels/example.com~1role"},
		})
		deployment := test.NewDeployment()
		deployment.SetLabels(map[string]string{"example.com/role": "web"})

		paths, err := n.IgnoredPaths(deployment)

		require.NoError(t, err)
		assert.Equal(t, [][]any{{"metadata", "labels", "example.com/role"}}, paths)
	})

	t.Run("jq expressions resolve every selected path", func(t *testing.T) {
		t.Parallel()
		n := newNormalizer(t, v1alpha1.ResourceIgnoreDifferences{
			Group: "apps", Kind: "Deployment",
			JQPathExpressions: []string{
				`.spec.template.spec.containers[] | select(.name == "nginx") | .image`,
				`.spec.template.spec.containers[] | select(.name == "missing")`,
				`.metadata.annotations["example.com/absent"]`,
			},
		})
		deployment := test.NewDeployment()

		paths, err := n.IgnoredPaths(deployment)

		require.NoError(t, err)
		assert.Equal(t, [][]any{{"spec", "template", "spec", "containers", 0, "image"}}, paths)
	})

	t.Run("negative json pointer index counts from the end, as json-patch does", func(t *testing.T) {
		t.Parallel()
		n := newNormalizer(t, v1alpha1.ResourceIgnoreDifferences{
			Group: "apps", Kind: "Deployment",
			JSONPointers: []string{"/spec/template/spec/containers/-1/image", "/spec/template/spec/containers/-2"},
		})
		deployment := test.NewDeployment()

		paths, err := n.IgnoredPaths(deployment)

		require.NoError(t, err)
		assert.Equal(t, [][]any{{"spec", "template", "spec", "containers", 0, "image"}}, paths, "-1 is the only container; -2 is out of range")
	})

	t.Run("jq slice expands to every selected index, as del() removes them", func(t *testing.T) {
		t.Parallel()
		n := newNormalizer(t, v1alpha1.ResourceIgnoreDifferences{
			Group: "apps", Kind: "Deployment",
			JQPathExpressions: []string{
				`.spec.template.spec.containers[1:]`,
				`.spec.template.spec.containers[-1:][].image`,
				`.spec.template.spec.containers[-1].name`,
				`.spec.template.spec.containers[-4]`,
			},
		})
		deployment := test.NewDeployment()
		containers, _, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
		require.NoError(t, err)
		containers = append(containers, map[string]any{"name": "sidecar", "image": "sidecar:1"}, map[string]any{"name": "third", "image": "third:1"})
		require.NoError(t, unstructured.SetNestedSlice(deployment.Object, containers, "spec", "template", "spec", "containers"))

		paths, err := n.IgnoredPaths(deployment)

		require.NoError(t, err)
		assert.ElementsMatch(t, [][]any{
			{"spec", "template", "spec", "containers", 1},
			{"spec", "template", "spec", "containers", 2},
			{"spec", "template", "spec", "containers", 2, "image"},
			{"spec", "template", "spec", "containers", 2, "name"},
		}, paths, "negative indexes count from the end; -4 is out of range")
	})

	t.Run("jq expression that is not a path expression is skipped", func(t *testing.T) {
		t.Parallel()
		n := newNormalizer(t, v1alpha1.ResourceIgnoreDifferences{
			Group: "apps", Kind: "Deployment",
			JQPathExpressions: []string{`.spec.replicas + 1`},
		})

		paths, err := n.IgnoredPaths(test.NewDeployment())

		require.NoError(t, err)
		assert.Empty(t, paths)
	})

	t.Run("resources not matched by any rule yield no paths", func(t *testing.T) {
		t.Parallel()
		n := newNormalizer(t, v1alpha1.ResourceIgnoreDifferences{
			Kind: "Service", JSONPointers: []string{"/spec"},
		})

		paths, err := n.IgnoredPaths(test.NewDeployment())

		require.NoError(t, err)
		assert.Empty(t, paths)
	})

	t.Run("nil resource is an error", func(t *testing.T) {
		t.Parallel()
		n := newNormalizer(t, v1alpha1.ResourceIgnoreDifferences{Kind: "Service", JSONPointers: []string{"/spec"}})

		_, err := n.IgnoredPaths(nil)

		require.Error(t, err)
	})
}
