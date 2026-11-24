package cache

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
)

// Resolvers share the cluster's cached OpenAPI v3 index until an event says schemas may have changed.
func TestOpenAPISchemaCache_InvalidatedOnSchemaChanges(t *testing.T) {
	t.Parallel()
	var indexRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/openapi/v3":
			indexRequests.Add(1)
			_, _ = w.Write([]byte(`{"paths":{"apis/example.com/v1":{"serverRelativeURL":"/openapi/v3/apis/example.com/v1?hash=1"}}}`))
		case "/openapi/v3/apis/example.com/v1":
			_, _ = w.Write([]byte(`{"components":{"schemas":{}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	cluster := newCluster(t)
	resolve := func() int32 {
		t.Helper()
		resolver := kube.NewCachedOpenAPISchemaResolver(&rest.Config{Host: server.URL}, logr.Discard(), cluster.openAPISchemaCache)
		_, err := resolver.Type(schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Example"})
		require.NoError(t, err)
		return indexRequests.Load()
	}

	assert.Equal(t, int32(1), resolve())
	assert.Equal(t, int32(1), resolve())

	crd := strToUnstructured(`
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: examples.example.com
spec:
  group: example.com
  scope: Namespaced
  names: {kind: Example, plural: examples, singular: example}
  versions:
  - name: v1`)
	cluster.handleCRDEvent(legacyEngineOf(cluster), watch.Modified, crd)
	assert.Equal(t, int32(2), resolve())
	assert.Equal(t, int32(2), resolve())

	cluster.Invalidate()
	assert.Equal(t, int32(3), resolve())
}
