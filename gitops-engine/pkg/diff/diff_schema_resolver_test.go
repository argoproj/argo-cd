package diff_test

import (
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/openapi/openapitest"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/diff"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/diff/mocks"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/diff/testdata"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube/kubetest"
)

const widgetV2 = `{
  "swagger": "2.0",
  "info": {"title": "test", "version": "v1"},
  "paths": {},
  "definitions": {
    "com.example.v1.Widget": {
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
            "labels": {"type": "object", "additionalProperties": {"type": "string"}},
            "items": {
              "type": "array",
              "x-kubernetes-list-type": "map",
              "x-kubernetes-list-map-keys": ["name"],
              "items": {"type": "object", "properties": {"name": {"type": "string"}, "value": {"type": "string"}}}
            }
          }
        }
      }
    }
  }
}`

// widgetV3 is widgetV2 as served by the OpenAPI v3 endpoint.
const widgetV3 = `{
  "openapi": "3.0.0",
  "info": {"title": "test", "version": "v1"},
  "paths": {},
  "components": {"schemas": {
    "com.example.v1.Widget": {
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
            "labels": {"type": "object", "additionalProperties": {"type": "string"}},
            "items": {
              "type": "array",
              "x-kubernetes-list-type": "map",
              "x-kubernetes-list-map-keys": ["name"],
              "items": {"type": "object", "properties": {"name": {"type": "string"}, "value": {"type": "string"}}}
            }
          }
        }
      }
    }
  }}
}`

const widgetLive = `
apiVersion: example.com/v1
kind: Widget
metadata:
  name: w
  managedFields:
  - manager: argocd-controller
    operation: Apply
    apiVersion: example.com/v1
    fieldsType: FieldsV1
    fieldsV1: {"f:spec": {"f:replicas": {}, "f:items": {"k:{\"name\":\"a\"}": {".": {}, "f:name": {}, "f:value": {}}}}}
spec:
  replicas: 1
  labels: {team: x}
  items:
  - {name: a, value: "1"}
`

const widgetConfig = `
apiVersion: example.com/v1
kind: Widget
metadata:
  name: w
spec:
  replicas: 2
  items:
  - {name: a, value: "2"}
  - {name: b, value: "3"}
`

const widgetPredictedLive = `{
  "apiVersion": "example.com/v1",
  "kind": "Widget",
  "metadata": {
    "name": "w",
    "managedFields": [
      {"manager": "argocd-controller", "operation": "Apply", "apiVersion": "example.com/v1", "fieldsType": "FieldsV1",
       "fieldsV1": {"f:spec": {"f:replicas": {}, "f:items": {"k:{\"name\":\"a\"}": {".": {}, "f:name": {}, "f:value": {}}, "k:{\"name\":\"b\"}": {".": {}, "f:name": {}, "f:value": {}}}}}},
      {"manager": "webhook", "operation": "Update", "apiVersion": "example.com/v1", "fieldsType": "FieldsV1",
       "fieldsV1": {"f:spec": {"f:labels": {"f:injected": {}}}}}
    ]
  },
  "spec": {
    "replicas": 2,
    "labels": {"team": "x", "injected": "true"},
    "items": [{"name": "a", "value": "2"}, {"name": "b", "value": "3"}]
  }
}`

func toUnstructured(t *testing.T, s string) *unstructured.Unstructured {
	t.Helper()
	obj := map[string]any{}
	require.NoError(t, yaml.Unmarshal([]byte(s), &obj))
	return &unstructured.Unstructured{Object: obj}
}

// schemaResolvers returns resolvers backed by the OpenAPI v3 documents and by the v2 fallback.
func schemaResolvers(t *testing.T) map[string]kube.OpenAPISchemaResolver {
	t.Helper()
	v2 := &openapi_v2.Document{}
	require.NoError(t, proto.Unmarshal(testdata.OpenAPIV2Doc, v2))
	widget, err := openapi_v2.ParseDocument([]byte(widgetV2))
	require.NoError(t, err)
	v2.Definitions.AdditionalProperties = append(v2.Definitions.AdditionalProperties, widget.Definitions.AdditionalProperties...)

	paths, err := openapitest.NewEmbeddedFileClient().Paths()
	require.NoError(t, err)
	v3 := map[string][]byte{"apis/example.com/v1": []byte(widgetV3)}
	for path, gv := range paths {
		v3[path], err = gv.Schema("application/json")
		require.NoError(t, err)
	}
	return map[string]kube.OpenAPISchemaResolver{
		"v3": (&kubetest.FakeOpenAPIClient{V3: v3}).NewOpenAPISchemaResolver(),
		"v2": (&kubetest.FakeOpenAPIClient{V2: v2}).NewOpenAPISchemaResolver(),
	}
}

// TestServerSideDiffSchemaResolvers checks that server-side diffs are the same whether the schema
// comes from the OpenAPI v3 group version documents or from the v2 document.
func TestServerSideDiffSchemaResolvers(t *testing.T) {
	t.Parallel()
	fixtures := []struct {
		name                  string
		config, live, predict string
	}{
		{"service", testdata.ServiceConfigYAMLSSD, testdata.ServiceLiveYAMLSSD, testdata.ServicePredictedLiveJSONSSD},
		{"deployment field removal", testdata.Deployment2ConfigYAML, testdata.Deployment2LiveYAML, testdata.Deployment2PredictedLiveJSONSSD},
		{"deployment nested", testdata.DeploymentNestedConfigYAMLSSD, testdata.DeploymentNestedLiveYAMLSSD, testdata.DeploymentNestedPredictedLiveJSONSSD},
		{"deployment manual apply", testdata.DeploymentApplyConfigYAMLSSD, testdata.DeploymentApplyLiveYAMLSSD, testdata.DeploymentApplyPredictedLiveJSONSSD},
		{"deployment composite key", testdata.DeploymentCompositeKeyConfigYAMLSSD, testdata.DeploymentCompositeKeyLiveYAMLSSD, testdata.DeploymentCompositeKeyPredictedLiveJSONSSD},
		{"service label removed by other manager", testdata.ServiceConfigNoLabelYAMLSSD, testdata.ServiceLiveLabelYAMLSSD, testdata.ServicePredictedLiveNoLabelJSONSSD},
		{"configmap", testdata.ConfigMapConfigYAMLSSD, testdata.ConfigMapLiveYAMLSSD, testdata.ConfigMapPredictedLiveJSONSSD},
		{"custom resource", widgetConfig, widgetLive, widgetPredictedLive},
	}
	resolvers := schemaResolvers(t)
	for _, f := range fixtures {
		for _, ignoreMutationWebhook := range []bool{true, false} {
			results := map[string]*diff.DiffResult{}
			for name, resolver := range resolvers {
				dryRunner := mocks.NewServerSideDryRunner(t)
				dryRunner.EXPECT().Run(mock.Anything, mock.Anything, "argocd-controller").Return(f.predict, nil)
				result, err := diff.ServerSideDiff(t.Context(), toUnstructured(t, f.config), toUnstructured(t, f.live),
					diff.WithTypeResolver(resolver),
					diff.WithManager("argocd-controller"),
					diff.WithServerSideDryRunner(dryRunner),
					diff.WithIgnoreMutationWebhook(ignoreMutationWebhook))
				require.NoError(t, err, "%s %s", f.name, name)
				results[name] = result
			}
			assert.True(t, results["v3"].Modified, f.name)
			assert.Equal(t, results["v2"].Modified, results["v3"].Modified, f.name)
			assert.JSONEq(t, string(results["v2"].PredictedLive), string(results["v3"].PredictedLive), "%s ignoreMutationWebhook=%v", f.name, ignoreMutationWebhook)
			assert.JSONEq(t, string(results["v2"].NormalizedLive), string(results["v3"].NormalizedLive), "%s ignoreMutationWebhook=%v", f.name, ignoreMutationWebhook)
		}
	}

	t.Run("ServerSideApply sync option", func(t *testing.T) {
		t.Parallel()
		results := map[string]*diff.DiffResult{}
		for name, resolver := range resolvers {
			config := toUnstructured(t, testdata.DeploymentNestedConfigYAMLSSD)
			config.SetAnnotations(map[string]string{"argocd.argoproj.io/sync-options": "ServerSideApply=true"})
			dryRunner := mocks.NewServerSideDryRunner(t)
			dryRunner.EXPECT().Run(mock.Anything, mock.Anything, "argocd-controller").Return(testdata.DeploymentNestedPredictedLiveJSONSSD, nil)
			result, err := diff.Diff(t.Context(), config, toUnstructured(t, testdata.DeploymentNestedLiveYAMLSSD),
				diff.WithTypeResolver(resolver),
				diff.WithManager("argocd-controller"),
				diff.WithServerSideDryRunner(dryRunner))
			require.NoError(t, err, name)
			results[name] = result
		}
		assert.True(t, results["v3"].Modified)
		assert.JSONEq(t, string(results["v2"].PredictedLive), string(results["v3"].PredictedLive))
		assert.JSONEq(t, string(results["v2"].NormalizedLive), string(results["v3"].NormalizedLive))
	})

	t.Run("custom resource webhook mutation is removed", func(t *testing.T) {
		t.Parallel()
		dryRunner := mocks.NewServerSideDryRunner(t)
		dryRunner.EXPECT().Run(mock.Anything, mock.Anything, "argocd-controller").Return(widgetPredictedLive, nil)
		result, err := diff.ServerSideDiff(t.Context(), toUnstructured(t, widgetConfig), toUnstructured(t, widgetLive),
			diff.WithTypeResolver(resolvers["v3"]),
			diff.WithManager("argocd-controller"),
			diff.WithServerSideDryRunner(dryRunner))
		require.NoError(t, err)
		predicted := toUnstructured(t, string(result.PredictedLive))
		labels, _, err := unstructured.NestedStringMap(predicted.Object, "spec", "labels")
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"team": "x"}, labels)
		items, _, err := unstructured.NestedSlice(predicted.Object, "spec", "items")
		require.NoError(t, err)
		assert.Len(t, items, 2)
	})
}

func TestServerSideDiffSchemaResolverErrors(t *testing.T) {
	t.Parallel()
	run := func(t *testing.T, resolver kube.OpenAPISchemaResolver) error {
		t.Helper()
		dryRunner := mocks.NewServerSideDryRunner(t)
		dryRunner.EXPECT().Run(mock.Anything, mock.Anything, "argocd-controller").Return(widgetPredictedLive, nil)
		opts := []diff.Option{diff.WithManager("argocd-controller"), diff.WithServerSideDryRunner(dryRunner)}
		if resolver != nil {
			opts = append(opts, diff.WithTypeResolver(resolver))
		}
		_, err := diff.ServerSideDiff(t.Context(), toUnstructured(t, widgetConfig), toUnstructured(t, widgetLive), opts...)
		return err
	}

	t.Run("schema fetch fails", func(t *testing.T) {
		t.Parallel()
		err := run(t, (&kubetest.FakeOpenAPIClient{Err: assert.AnError}).NewOpenAPISchemaResolver())
		require.ErrorContains(t, err, "failed to load OpenAPI schema for example.com/v1")
		require.ErrorIs(t, err, assert.AnError)
	})
	t.Run("no schema for kind", func(t *testing.T) {
		t.Parallel()
		err := run(t, (&kubetest.FakeOpenAPIClient{V3: map[string][]byte{}}).NewOpenAPISchemaResolver())
		require.ErrorContains(t, err, "unable to resolve parseableType for GroupVersionKind")
	})
	t.Run("no resolver", func(t *testing.T) {
		t.Parallel()
		err := run(t, nil)
		require.ErrorContains(t, err, "no schema resolver")
	})
}
