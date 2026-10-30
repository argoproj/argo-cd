package kube

import (
	"errors"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/openapi"
	"k8s.io/client-go/openapi/openapitest"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
	"k8s.io/kubectl/pkg/validation"
)

var (
	configMapGVK = schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
	fooGVK       = schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Foo"}
)

// fooSpec is a group-version document whose PATCH operation lacks the fieldValidation param.
const fooSpec = `{"openapi":"3.0.0","info":{"title":"t","version":"v1"},"paths":{"/apis/example.com/v1/foos/{name}":{"patch":{"x-kubernetes-group-version-kind":{"group":"example.com","version":"v1","kind":"Foo"},"parameters":[{"name":"dryRun","in":"query"}]}}}}`

// countingClient serves the test documents and fooSpec under hashed URLs and counts
// Schema calls.
type countingClient struct {
	hash      string
	schemaErr error
	calls     atomic.Int64
}

type countingGroupVersion struct {
	c     *countingClient
	url   string
	inner openapi.GroupVersion
	raw   []byte
}

func (g *countingGroupVersion) Schema(contentType string) ([]byte, error) {
	g.c.calls.Add(1)
	if g.c.schemaErr != nil {
		return nil, g.c.schemaErr
	}
	if g.raw != nil {
		return g.raw, nil
	}
	return g.inner.Schema(contentType)
}

func (g *countingGroupVersion) ServerRelativeURL() string {
	return g.url
}

func (c *countingClient) Paths() (map[string]openapi.GroupVersion, error) {
	inner, err := openapitest.NewEmbeddedFileClient().Paths()
	if err != nil {
		return nil, err
	}
	url := func(path string) string {
		if c.hash == "" {
			return "/openapi/v3/" + path
		}
		return "/openapi/v3/" + path + "?hash=" + c.hash
	}
	paths := map[string]openapi.GroupVersion{}
	for path, gv := range inner {
		paths[path] = &countingGroupVersion{c: c, url: url(path), inner: gv}
	}
	paths["apis/example.com/v1"] = &countingGroupVersion{c: c, url: url("apis/example.com/v1"), raw: []byte(fooSpec)}
	return paths, nil
}

func newTestVerifier(client openapi.Client, cache *fieldValidationCache) resource.Verifier {
	return newFieldValidationVerifier(testDynamicClient, client, cache)
}

var testDynamicClient = dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())

func TestHashKeyedVerifier(t *testing.T) {
	t.Parallel()
	const applies = 20

	t.Run("supported GVK is parsed once across applies", func(t *testing.T) {
		t.Parallel()
		client := &countingClient{hash: "A"}
		cache := newFieldValidationCache()
		for range applies {
			require.NoError(t, newTestVerifier(client, cache).HasSupport(configMapGVK))
		}
		assert.Equal(t, int64(1), client.calls.Load())
	})

	t.Run("unsupported GVK is parsed once and stays unsupported", func(t *testing.T) {
		t.Parallel()
		client := &countingClient{hash: "A"}
		cache := newFieldValidationCache()
		for range applies {
			err := newTestVerifier(client, cache).HasSupport(fooGVK)
			require.Error(t, err)
			assert.True(t, resource.IsParamUnsupportedError(err))
		}
		assert.Equal(t, int64(1), client.calls.Load())
	})

	t.Run("same answer as the uncached verifier", func(t *testing.T) {
		t.Parallel()
		client := &countingClient{hash: "A"}
		cache := newFieldValidationCache()
		uncached := resource.NewQueryParamVerifierV3(testDynamicClient, client, resource.QueryParamFieldValidation)
		for _, gvk := range []schema.GroupVersionKind{configMapGVK, fooGVK, {Group: "apps", Version: "v1", Kind: "Deployment"}, {Version: "v1", Kind: "List"}} {
			want := uncached.HasSupport(gvk)
			for range 2 {
				got := newTestVerifier(client, cache).HasSupport(gvk)
				assert.Equal(t, want == nil, got == nil, gvk.String())
				assert.Equal(t, resource.IsParamUnsupportedError(want), resource.IsParamUnsupportedError(got), gvk.String())
			}
		}
	})

	t.Run("changed document hash is a cache miss", func(t *testing.T) {
		t.Parallel()
		client := &countingClient{hash: "A"}
		cache := newFieldValidationCache()
		require.NoError(t, newTestVerifier(client, cache).HasSupport(configMapGVK))
		client.hash = "B"
		require.NoError(t, newTestVerifier(client, cache).HasSupport(configMapGVK))
		require.NoError(t, newTestVerifier(client, cache).HasSupport(configMapGVK))
		assert.Equal(t, int64(2), client.calls.Load())
	})

	t.Run("URL without hash is not cached", func(t *testing.T) {
		t.Parallel()
		client := &countingClient{}
		cache := newFieldValidationCache()
		for range applies {
			require.NoError(t, newTestVerifier(client, cache).HasSupport(configMapGVK))
		}
		assert.Equal(t, int64(applies), client.calls.Load())
	})

	t.Run("errors are not cached", func(t *testing.T) {
		t.Parallel()
		client := &countingClient{hash: "A", schemaErr: errors.New("boom")}
		cache := newFieldValidationCache()
		for range applies {
			err := newTestVerifier(client, cache).HasSupport(configMapGVK)
			require.Error(t, err)
			assert.False(t, resource.IsParamUnsupportedError(err))
		}
		assert.Equal(t, int64(applies), client.calls.Load())
		client.schemaErr = nil
		require.NoError(t, newTestVerifier(client, cache).HasSupport(configMapGVK))
	})

	t.Run("cache is reset when full", func(t *testing.T) {
		t.Parallel()
		cache := newFieldValidationCache()
		for i := range maxFieldValidationCacheEntries {
			cache.set(fieldValidationCacheKey(strconv.Itoa(i)), nil)
		}
		cache.set("last", nil)
		assert.Len(t, cache.entries, 1)
	})
}

func TestNewValidatorIgnore(t *testing.T) {
	t.Parallel()
	k, _ := newTestKubectlResourceOperations(t)
	v, err := k.newValidator(metav1.FieldValidationIgnore)
	require.NoError(t, err)
	assert.IsType(t, validation.NullSchema{}, v)
}

func TestNewValidatorStrict(t *testing.T) {
	t.Parallel()
	// A unique hash keeps the shared cache's keys private to this test.
	client := &countingClient{hash: t.Name()}
	f := cmdtesting.NewTestFactory()
	t.Cleanup(f.Cleanup)
	f.FakeDynamicClient = testDynamicClient
	f.OpenAPIV3ClientFunc = func() (openapi.Client, error) { return client, nil }
	k, _ := newTestKubectlResourceOperations(t)
	k.fact = f

	configMap := []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"cm"}}`)
	for range 2 {
		v, err := k.newValidator(metav1.FieldValidationStrict)
		require.NoError(t, err)
		assert.NotEqual(t, validation.NullSchema{}, v)
		require.NoError(t, v.ValidateBytes(configMap))
	}
	assert.Equal(t, int64(1), client.calls.Load())
}

// BenchmarkHasSupport compares a fresh verifier per apply with the cache.
func BenchmarkHasSupport(b *testing.B) {
	client := &countingClient{hash: "A"}
	b.Run("uncached", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			v := resource.NewQueryParamVerifierV3(testDynamicClient, client, resource.QueryParamFieldValidation)
			if err := v.HasSupport(configMapGVK); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("hash-keyed", func(b *testing.B) {
		cache := newFieldValidationCache()
		b.ReportAllocs()
		for b.Loop() {
			if err := newTestVerifier(client, cache).HasSupport(configMapGVK); err != nil {
				b.Fatal(err)
			}
		}
	})
}
