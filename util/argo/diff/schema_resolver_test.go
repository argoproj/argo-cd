package diff_test

import (
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube/kubetest"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	testutil "github.com/argoproj/argo-cd/v3/test"
	argo "github.com/argoproj/argo-cd/v3/util/argo/diff"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
	"github.com/argoproj/argo-cd/v3/util/argo/testdata"
)

const widgetSchema = `{
  "type": "object",
  "x-kubernetes-group-version-kind": [{"group": "example.com", "version": "v1", "kind": "Widget"}],
  "properties": {
    "apiVersion": {"type": "string"},
    "kind": {"type": "string"},
    "metadata": {"type": "object"},
    "spec": {
      "type": "object",
      "properties": {
        "replicas": {"type": "integer"},
        "image": {"type": "string"},
        "items": {
          "type": "array",
          "x-kubernetes-list-type": "map",
          "x-kubernetes-list-map-keys": ["name"],
          "items": {"type": "object", "properties": {"name": {"type": "string"}, "value": {"type": "string"}}}
        }
      }
    }
  }
}`

const widgetLive = `
apiVersion: example.com/v1
kind: Widget
metadata:
  name: w
  namespace: default
  managedFields:
  - manager: argocd-controller
    operation: Apply
    apiVersion: example.com/v1
    fieldsType: FieldsV1
    fieldsV1: {"f:spec": {"f:image": {}, "f:items": {"k:{\"name\":\"a\"}": {".": {}, "f:name": {}}}}}
  - manager: autoscaler
    operation: Update
    apiVersion: example.com/v1
    fieldsType: FieldsV1
    fieldsV1: {"f:spec": {"f:replicas": {}, "f:items": {"k:{\"name\":\"a\"}": {"f:value": {}}}}}
spec:
  replicas: 5
  image: v1
  items:
  - {name: a, value: scaled}
`

const widgetDesired = `
apiVersion: example.com/v1
kind: Widget
metadata:
  name: w
  namespace: default
spec:
  replicas: 1
  image: v2
  items:
  - {name: a, value: desired}
`

func widgetResolvers(t *testing.T) map[string]kube.OpenAPISchemaResolver {
	t.Helper()
	v2, err := openapi_v2.ParseDocument([]byte(`{"swagger": "2.0", "info": {"title": "t", "version": "v1"}, "paths": {}, "definitions": {"com.example.v1.Widget": ` + widgetSchema + `}}`))
	require.NoError(t, err)
	v3 := []byte(`{"openapi": "3.0.0", "info": {"title": "t", "version": "v1"}, "paths": {}, "components": {"schemas": {"com.example.v1.Widget": ` + widgetSchema + `}}}`)
	return map[string]kube.OpenAPISchemaResolver{
		"v2": (&kubetest.FakeOpenAPIClient{V2: v2}).NewOpenAPISchemaResolver(),
		"v3": (&kubetest.FakeOpenAPIClient{V3: map[string][]byte{"apis/example.com/v1": v3}}).NewOpenAPISchemaResolver(),
	}
}

func managedFieldsDiffConfig(t *testing.T, resolver kube.OpenAPISchemaResolver, manager string) argo.DiffConfig {
	t.Helper()
	dc, err := argo.NewDiffConfigBuilder().
		WithDiffSettings([]v1alpha1.ResourceIgnoreDifferences{{Group: "*", Kind: "*", ManagedFieldsManagers: []string{manager}}}, nil, true, normalizers.IgnoreNormalizerOpts{}).
		WithNoCache().
		WithOpenAPISchemaResolver(resolver).
		Build()
	require.NoError(t, err)
	return dc
}

func TestStateDiffManagedFieldsManagersCustomResource(t *testing.T) {
	t.Parallel()
	normalize := func(t *testing.T, resolver kube.OpenAPISchemaResolver) (*unstructured.Unstructured, *unstructured.Unstructured) {
		t.Helper()
		result, err := argo.StateDiff(t.Context(), testutil.YamlToUnstructured(widgetLive), testutil.YamlToUnstructured(widgetDesired), managedFieldsDiffConfig(t, resolver, "autoscaler"))
		require.NoError(t, err)
		require.True(t, result.Modified)
		return testutil.YamlToUnstructured(string(result.NormalizedLive)), testutil.YamlToUnstructured(string(result.PredictedLive))
	}

	results := map[string][2]*unstructured.Unstructured{}
	for name, resolver := range widgetResolvers(t) {
		normalized, predicted := normalize(t, resolver)
		// Fields owned by the trusted manager are removed from both sides, including the field of
		// the list map entry, which can only be matched by key with the CRD schema.
		for _, obj := range []*unstructured.Unstructured{normalized, predicted} {
			_, found, err := unstructured.NestedFieldNoCopy(obj.Object, "spec", "replicas")
			require.NoError(t, err)
			assert.False(t, found, name)
			items, _, err := unstructured.NestedSlice(obj.Object, "spec", "items")
			require.NoError(t, err)
			assert.Equal(t, []any{map[string]any{"name": "a"}}, items, name)
		}
		image, _, err := unstructured.NestedString(predicted.Object, "spec", "image")
		require.NoError(t, err)
		assert.Equal(t, "v2", image, name)
		results[name] = [2]*unstructured.Unstructured{normalized, predicted}
	}
	assert.Equal(t, results["v2"][0].Object, results["v3"][0].Object)
	assert.Equal(t, results["v2"][1].Object, results["v3"][1].Object)

	// Without a schema the list is atomic and its entries keep the trusted manager's value.
	normalized, _ := normalize(t, nil)
	items, _, err := unstructured.NestedSlice(normalized.Object, "spec", "items")
	require.NoError(t, err)
	assert.Equal(t, []any{map[string]any{"name": "a", "value": "scaled"}}, items)
}

func TestStateDiffManagedFieldsManagersSchemaErrors(t *testing.T) {
	t.Parallel()

	t.Run("built-in types don't fetch the schema", func(t *testing.T) {
		t.Parallel()
		client := &kubetest.FakeOpenAPIClient{Err: assert.AnError}
		result, err := argo.StateDiff(t.Context(), testutil.YamlToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml), testutil.YamlToUnstructured(testdata.DesiredDeploymentYaml), managedFieldsDiffConfig(t, client.NewOpenAPISchemaResolver(), "kube-controller-manager"))
		require.NoError(t, err)
		normalized := testutil.YamlToUnstructured(string(result.NormalizedLive))
		replicas, _, err := unstructured.NestedFloat64(normalized.Object, "spec", "replicas")
		require.NoError(t, err)
		assert.InEpsilon(t, float64(1), replicas, 0.0001)
		assert.Zero(t, client.V3PathsRequests.Load()+client.V3GVRequests.Load()+client.V2Requests.Load())
	})

	t.Run("custom resource schema fetch failure fails the diff", func(t *testing.T) {
		t.Parallel()
		resolver := (&kubetest.FakeOpenAPIClient{Err: assert.AnError}).NewOpenAPISchemaResolver()
		_, err := argo.StateDiff(t.Context(), testutil.YamlToUnstructured(widgetLive), testutil.YamlToUnstructured(widgetDesired), managedFieldsDiffConfig(t, resolver, "autoscaler"))
		require.ErrorContains(t, err, "error resolving schema of Widget/w")
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("one resolver per diff fetches each group version once", func(t *testing.T) {
		t.Parallel()
		client := &kubetest.FakeOpenAPIClient{V3: map[string][]byte{"apis/example.com/v1": []byte(`{"openapi": "3.0.0", "components": {"schemas": {"com.example.v1.Widget": ` + widgetSchema + `}}}`)}}
		dc := managedFieldsDiffConfig(t, client.NewOpenAPISchemaResolver(), "autoscaler")
		lives := []*unstructured.Unstructured{testutil.YamlToUnstructured(widgetLive), testutil.YamlToUnstructured(widgetLive)}
		desired := []*unstructured.Unstructured{testutil.YamlToUnstructured(widgetDesired), testutil.YamlToUnstructured(widgetDesired)}
		_, err := argo.StateDiffs(t.Context(), lives, desired, dc)
		require.NoError(t, err)
		assert.Equal(t, int32(1), client.V3GVRequests.Load())
	})
}
