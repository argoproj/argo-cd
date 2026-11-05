package kube

import (
	"runtime"
	"testing"
	"weak"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2/textlogger"
	"k8s.io/kube-openapi/pkg/util/proto"
	"k8s.io/kubectl/pkg/util/openapi"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/tracing"
)

// weakOpenAPIClient serves the test OpenAPI document and keeps a weak pointer to the last
// document it returned, so tests can check whether anything still references it.
type weakOpenAPIClient struct {
	doc weak.Pointer[openapi_v2.Document]
}

func (c *weakOpenAPIClient) OpenAPISchema() (*openapi_v2.Document, error) {
	doc, err := (&fakeOpenAPIClient{}).OpenAPISchema()
	if err != nil {
		return nil, err
	}
	c.doc = weak.Make(doc)
	return doc, nil
}

func newTestKubectl() *KubectlCmd {
	return &KubectlCmd{
		Log:    textlogger.NewLogger(textlogger.NewConfig()),
		Tracer: tracing.NopTracer{},
	}
}

func TestLoadOpenAPISchema(t *testing.T) {
	t.Parallel()

	resources, gvkParser, err := newTestKubectl().loadOpenAPISchema(&fakeOpenAPIClient{})
	require.NoError(t, err)
	require.NotNil(t, resources)
	assert.NotNil(t, gvkParser)

	t.Run("LookupResource matches kubectl's implementation", func(t *testing.T) {
		t.Parallel()
		doc, err := (&fakeOpenAPIClient{}).OpenAPISchema()
		require.NoError(t, err)
		expected, err := openapi.NewOpenAPIData(doc)
		require.NoError(t, err)
		models, err := proto.NewOpenAPIData(doc)
		require.NoError(t, err)

		var gvks []schema.GroupVersionKind
		for _, modelName := range models.ListModels() {
			gvks = append(gvks, parseGroupVersionKind(models.LookupModel(modelName))...)
		}
		require.NotEmpty(t, gvks, "test document should contain models with GVKs")

		for _, gvk := range gvks {
			want := expected.LookupResource(gvk)
			got := resources.LookupResource(gvk)
			require.NotNil(t, got, gvk.String())
			assert.Equal(t, want.GetPath().String(), got.GetPath().String(), gvk.String())
		}
	})

	t.Run("LookupResource returns nil for an unknown GVK", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, resources.LookupResource(schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Missing"}))
	})

	t.Run("GetConsumes returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, resources.GetConsumes(schema.GroupVersionKind{Version: "v1", Kind: "APIResourceList"}, "PATCH"))
	})
}

// The cluster cache keeps the openapi.Resources and GVK parser for every cluster, so they must not
// keep the decoded OpenAPI document alive.
func TestLoadOpenAPISchema_DoesNotRetainDocument(t *testing.T) {
	t.Parallel()

	t.Run("kubectl's implementation retains the document", func(t *testing.T) {
		t.Parallel()
		// Control case: shows the weak-pointer check detects retention. If this starts failing after a
		// kubectl upgrade, kubectl stopped retaining the document and openAPIResources may be unneeded.
		client := &weakOpenAPIClient{}
		resources, err := openapi.NewOpenAPIParser(client).Parse()
		require.NoError(t, err)
		runtime.GC()
		assert.NotNil(t, client.doc.Value())
		runtime.KeepAlive(resources)
	})

	t.Run("loadOpenAPISchema does not retain the document", func(t *testing.T) {
		t.Parallel()
		client := &weakOpenAPIClient{}
		resources, gvkParser, err := newTestKubectl().loadOpenAPISchema(client)
		require.NoError(t, err)
		runtime.GC()
		assert.Nil(t, client.doc.Value())
		runtime.KeepAlive(resources)
		runtime.KeepAlive(gvkParser)
	})
}
