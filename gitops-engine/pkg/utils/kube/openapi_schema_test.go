package kube_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/openapi/openapitest"
	"k8s.io/client-go/rest"

	difftestdata "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/diff/testdata"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube/kubetest"
)

var (
	deploymentGVK  = schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	statefulSetGVK = schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}
	jobGVK         = schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}
)

// embeddedV3Docs returns the OpenAPI v3 documents embedded in client-go's openapitest package.
func embeddedV3Docs(t *testing.T) map[string][]byte {
	t.Helper()
	paths, err := openapitest.NewEmbeddedFileClient().Paths()
	require.NoError(t, err)
	docs := map[string][]byte{}
	for path, gv := range paths {
		docs[path], err = gv.Schema("application/json")
		require.NoError(t, err)
	}
	return docs
}

func loadV2Doc(t *testing.T) *openapi_v2.Document {
	t.Helper()
	doc := &openapi_v2.Document{}
	require.NoError(t, proto.Unmarshal(difftestdata.OpenAPIV2Doc, doc))
	return doc
}

// fakeAPIServer serves OpenAPI documents over HTTP and records the requested paths.
type fakeAPIServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []string
}

func newFakeAPIServer(t *testing.T, v3 map[string][]byte, v2 *openapi_v2.Document, gvStatus int) *fakeAPIServer {
	t.Helper()
	s := &fakeAPIServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, r.URL.Path)
		s.mu.Unlock()
		switch {
		case r.URL.Path == "/openapi/v2" && v2 != nil:
			data, err := proto.Marshal(v2)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(data)
		case r.URL.Path == "/openapi/v3" && v3 != nil:
			paths := map[string]any{}
			for path := range v3 {
				paths[path] = map[string]string{"serverRelativeURL": "/openapi/v3/" + path + "?hash=123"}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"paths": paths})
		case strings.HasPrefix(r.URL.Path, "/openapi/v3/") && v3 != nil:
			if gvStatus != http.StatusOK {
				http.Error(w, "unavailable", gvStatus)
				return
			}
			doc, ok := v3[strings.TrimPrefix(r.URL.Path, "/openapi/v3/")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(doc)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *fakeAPIServer) Requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.requests...)
}

func TestOpenAPISchemaResolver_FetchesOnlyRequestedGroupVersions(t *testing.T) {
	t.Parallel()
	server := newFakeAPIServer(t, embeddedV3Docs(t), nil, http.StatusOK)
	resolver := kube.NewOpenAPISchemaResolver(&rest.Config{Host: server.URL}, logr.Discard())

	assert.Empty(t, server.Requests(), "nothing must be fetched before a schema is requested")

	pt, err := resolver.Type(deploymentGVK)
	require.NoError(t, err)
	require.NotNil(t, pt)
	assert.True(t, pt.IsValid())
	assert.Equal(t, []string{"/openapi/v3", "/openapi/v3/apis/apps/v1"}, server.Requests())

	// Same group version: served from the resolver.
	pt, err = resolver.Type(statefulSetGVK)
	require.NoError(t, err)
	require.NotNil(t, pt)
	meta, err := resolver.LookupPatchMeta(deploymentGVK)
	require.NoError(t, err)
	require.NotNil(t, meta)
	assert.Len(t, server.Requests(), 2)

	pt, err = resolver.Type(jobGVK)
	require.NoError(t, err)
	require.NotNil(t, pt)
	assert.Equal(t, []string{"/openapi/v3", "/openapi/v3/apis/apps/v1", "/openapi/v3/apis/batch/v1"}, server.Requests())

	// A new resolver fetches again.
	_, err = kube.NewOpenAPISchemaResolver(&rest.Config{Host: server.URL}, logr.Discard()).Type(deploymentGVK)
	require.NoError(t, err)
	assert.Len(t, server.Requests(), 5)
}

func TestOpenAPISchemaResolver_UnknownGVK(t *testing.T) {
	t.Parallel()
	client := &kubetest.FakeOpenAPIClient{V3: embeddedV3Docs(t)}
	resolver := client.NewOpenAPISchemaResolver()

	for _, gvk := range []schema.GroupVersionKind{
		{Group: "example.com", Version: "v1", Kind: "Widget"},
		{Group: "apps", Version: "v1", Kind: "Widget"},
	} {
		pt, err := resolver.Type(gvk)
		require.NoError(t, err)
		assert.Nil(t, pt, gvk.String())
		meta, err := resolver.LookupPatchMeta(gvk)
		require.NoError(t, err)
		assert.Nil(t, meta, gvk.String())
	}
	assert.Equal(t, int32(1), client.V3GVRequests.Load(), "only the served apps/v1 document is fetched")
}

func TestOpenAPISchemaResolver_PatchMeta(t *testing.T) {
	t.Parallel()
	resolver := (&kubetest.FakeOpenAPIClient{V3: embeddedV3Docs(t)}).NewOpenAPISchemaResolver()
	meta, err := resolver.LookupPatchMeta(deploymentGVK)
	require.NoError(t, err)
	require.NotNil(t, meta)

	meta, _, err = meta.LookupPatchMetadataForStruct("spec")
	require.NoError(t, err)
	meta, _, err = meta.LookupPatchMetadataForStruct("template")
	require.NoError(t, err)
	meta, _, err = meta.LookupPatchMetadataForStruct("spec")
	require.NoError(t, err)
	_, patchMeta, err := meta.LookupPatchMetadataForSlice("containers")
	require.NoError(t, err)
	assert.Equal(t, "name", patchMeta.GetPatchMergeKey())
	assert.Equal(t, []string{"merge"}, patchMeta.GetPatchStrategies())
}

func TestOpenAPISchemaResolver_FallsBackToV2(t *testing.T) {
	t.Parallel()
	server := newFakeAPIServer(t, nil, loadV2Doc(t), http.StatusOK)
	resolver := kube.NewOpenAPISchemaResolver(&rest.Config{Host: server.URL}, logr.Discard())

	pt, err := resolver.Type(deploymentGVK)
	require.NoError(t, err)
	require.NotNil(t, pt)
	assert.True(t, pt.IsValid())
	meta, err := resolver.LookupPatchMeta(deploymentGVK)
	require.NoError(t, err)
	assert.NotNil(t, meta)
	pt, err = resolver.Type(schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Widget"})
	require.NoError(t, err)
	assert.Nil(t, pt)

	assert.Equal(t, []string{"/openapi/v3", "/openapi/v2"}, server.Requests(), "the v2 document is fetched once")
}

func TestOpenAPISchemaResolver_Errors(t *testing.T) {
	t.Parallel()

	t.Run("group version request fails", func(t *testing.T) {
		t.Parallel()
		server := newFakeAPIServer(t, embeddedV3Docs(t), nil, http.StatusServiceUnavailable)
		resolver := kube.NewOpenAPISchemaResolver(&rest.Config{Host: server.URL}, logr.Discard())

		_, err := resolver.Type(deploymentGVK)
		require.ErrorContains(t, err, "failed to load OpenAPI schema for apps/v1")
		_, err = resolver.LookupPatchMeta(deploymentGVK)
		require.ErrorContains(t, err, "failed to load OpenAPI schema for apps/v1")
		assert.Equal(t, []string{"/openapi/v3", "/openapi/v3/apis/apps/v1", "/openapi/v3/apis/apps/v1"}, server.Requests(), "failures are not cached")
	})

	t.Run("client fails", func(t *testing.T) {
		t.Parallel()
		resolver := (&kubetest.FakeOpenAPIClient{Err: errors.New("connection refused")}).NewOpenAPISchemaResolver()
		_, err := resolver.Type(deploymentGVK)
		require.ErrorContains(t, err, "connection refused")
	})

	t.Run("invalid document", func(t *testing.T) {
		t.Parallel()
		resolver := (&kubetest.FakeOpenAPIClient{V3: map[string][]byte{"apis/apps/v1": []byte("{")}}).NewOpenAPISchemaResolver()
		_, err := resolver.Type(deploymentGVK)
		require.ErrorContains(t, err, "error parsing OpenAPI v3 document")
	})
}

func TestOpenAPISchemaResolver_DuplicateGVKs(t *testing.T) {
	t.Parallel()
	// Both schemas declare example.com/v1 Widget. Like newUniqueModels, the first by name wins.
	doc := `{
  "openapi": "3.0.0",
  "components": {"schemas": {
    "com.example.v1.Widget": {
      "type": "object",
      "x-kubernetes-group-version-kind": [{"group": "example.com", "version": "v1", "kind": "Widget"}],
      "properties": {"spec": {"type": "object", "properties": {"a": {"type": "string"}}}}
    },
    "com.example.v1.Widget_v2": {
      "type": "object",
      "x-kubernetes-group-version-kind": [{"group": "example.com", "version": "v1", "kind": "Widget"}],
      "properties": {"spec": {"type": "object", "properties": {"b": {"type": "string"}}}}
    }
  }}
}`
	resolver := (&kubetest.FakeOpenAPIClient{V3: map[string][]byte{"apis/example.com/v1": []byte(doc)}}).NewOpenAPISchemaResolver()
	pt, err := resolver.Type(schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Widget"})
	require.NoError(t, err)
	require.NotNil(t, pt)

	_, err = pt.FromUnstructured(map[string]any{"spec": map[string]any{"a": "x"}})
	require.NoError(t, err)
	_, err = pt.FromUnstructured(map[string]any{"spec": map[string]any{"b": "x"}})
	require.ErrorContains(t, err, "field not declared in schema")
}

func TestOpenAPISchemaResolver_Concurrent(t *testing.T) {
	t.Parallel()
	client := &kubetest.FakeOpenAPIClient{V3: embeddedV3Docs(t)}
	resolver := client.NewOpenAPISchemaResolver()
	var wg sync.WaitGroup
	errs := make([]error, 10)
	for i := range errs {
		wg.Go(func() {
			_, errs[i] = resolver.Type(deploymentGVK)
		})
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	assert.Equal(t, int32(1), client.V3GVRequests.Load())
}
