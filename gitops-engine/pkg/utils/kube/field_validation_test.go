package kube

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/openapi"
	"k8s.io/client-go/openapi/openapitest"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
	kubectlopenapi "k8s.io/kubectl/pkg/util/openapi"
	"k8s.io/kubectl/pkg/validation"

	testingutils "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/testing"
)

var (
	configMapGVK = schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
	fooGVK       = schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Foo"}
	barGVK       = schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Bar"}
	// quxGVK is in fooSpec's served group-version but not listed in its document.
	quxGVK = schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Qux"}
)

// fooSpec is a group-version document whose Foo PATCH operation lacks the fieldValidation
// param and whose Bar PATCH operation has it.
const fooSpec = `{"openapi":"3.0.0","info":{"title":"t","version":"v1"},"paths":{` +
	`"/apis/example.com/v1/foos/{name}":{"patch":{"x-kubernetes-group-version-kind":{"group":"example.com","version":"v1","kind":"Foo"},"parameters":[{"name":"dryRun","in":"query"}]}},` +
	`"/apis/example.com/v1/bars/{name}":{"patch":{"x-kubernetes-group-version-kind":{"group":"example.com","version":"v1","kind":"Bar"},"parameters":[{"name":"fieldValidation","in":"query"}]}}}}`

// countingClient serves the test documents and fooSpec under hashed URLs and counts
// Schema calls.
type countingClient struct {
	hash      string
	pathsErr  error
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
	if c.pathsErr != nil {
		return nil, c.pathsErr
	}
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

var crdGVR = schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}

// newCRDDynamicClient returns a dynamic client that can list CRDs, which the verifiers
// query for group-versions missing from OpenAPI.
func newCRDDynamicClient() *dynamicfake.FakeDynamicClient {
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{crdGVR: "CustomResourceDefinitionList"})
}

func createCRD(t *testing.T, dyn *dynamicfake.FakeDynamicClient, gvk schema.GroupVersionKind) {
	t.Helper()
	crd := testingutils.Unstructured(fmt.Sprintf(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: %ss.%s
spec:
  group: %s
  names:
    kind: %s`, strings.ToLower(gvk.Kind), gvk.Group, gvk.Group, gvk.Kind))
	_, err := dyn.Resource(crdGVR).Create(t.Context(), crd, metav1.CreateOptions{})
	require.NoError(t, err)
}

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

	t.Run("kinds in one group-version are cached separately", func(t *testing.T) {
		t.Parallel()
		client := &countingClient{hash: "A"}
		cache := newFieldValidationCache()
		for range 2 {
			assert.True(t, resource.IsParamUnsupportedError(newTestVerifier(client, cache).HasSupport(fooGVK)))
			require.NoError(t, newTestVerifier(client, cache).HasSupport(barGVK))
		}
		assert.Equal(t, int64(2), client.calls.Load())
	})

	t.Run("group-version missing from OpenAPI is not cached", func(t *testing.T) {
		t.Parallel()
		// A CRD created in the same sync is listed before its group-version is served.
		dyn := newCRDDynamicClient()
		client := &countingClient{hash: "A"}
		cache := newFieldValidationCache()
		missingGVK := schema.GroupVersionKind{Group: "missing.example.com", Version: "v1", Kind: "Baz"}

		err := newFieldValidationVerifier(dyn, client, cache).HasSupport(missingGVK)
		assert.True(t, resource.IsParamUnsupportedError(err))

		createCRD(t, dyn, missingGVK)
		require.NoError(t, newFieldValidationVerifier(dyn, client, cache).HasSupport(missingGVK))
		assert.Empty(t, cache.entries)
	})

	t.Run("concurrent applies share the cache", func(t *testing.T) {
		t.Parallel()
		client := &countingClient{hash: "A"}
		cache := newFieldValidationCache()
		var wg sync.WaitGroup
		for range applies {
			wg.Go(func() {
				assert.NoError(t, newTestVerifier(client, cache).HasSupport(configMapGVK))
			})
		}
		wg.Wait()
		calls := client.calls.Load()
		require.NoError(t, newTestVerifier(client, cache).HasSupport(configMapGVK))
		assert.Equal(t, calls, client.calls.Load())
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

	t.Run("Paths error is not cached", func(t *testing.T) {
		t.Parallel()
		client := &countingClient{hash: "A", pathsErr: errors.New("boom")}
		cache := newFieldValidationCache()
		require.ErrorContains(t, newTestVerifier(client, cache).HasSupport(configMapGVK), "boom")
		assert.Empty(t, cache.entries)
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

	t.Run("verifier repopulates the cache after a reset", func(t *testing.T) {
		t.Parallel()
		client := &countingClient{hash: "A"}
		cache := newFieldValidationCache()
		require.NoError(t, newTestVerifier(client, cache).HasSupport(configMapGVK))
		for i := range maxFieldValidationCacheEntries - 1 {
			cache.set(fieldValidationCacheKey(strconv.Itoa(i)), nil)
		}
		cache.set("overflow", nil)
		require.Len(t, cache.entries, 1)

		for range 2 {
			require.NoError(t, newTestVerifier(client, cache).HasSupport(configMapGVK))
		}
		assert.Equal(t, int64(2), client.calls.Load())
		assert.Len(t, cache.entries, 2)
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
	// A unique hash keeps the shared cache's keys private to this run.
	client := &countingClient{hash: rand.Text()}
	f := cmdtesting.NewTestFactory()
	t.Cleanup(f.Cleanup)
	f.FakeDynamicClient = testDynamicClient
	f.OpenAPIV3ClientFunc = func() (openapi.Client, error) { return client, nil }
	k, _ := newTestKubectlResourceOperations(t)
	k.fact = f

	configMap := []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"cm"}}`)
	var v2Getter any
	for range 2 {
		v, err := k.newValidator(metav1.FieldValidationStrict)
		require.NoError(t, err)
		assert.NotEqual(t, validation.NullSchema{}, v)
		require.NoError(t, v.ValidateBytes(configMap))
		if v2Getter == nil {
			v2Getter = k.openAPIV2Getter
		}
		assert.Same(t, v2Getter, k.openAPIV2Getter, "v2 getter is built once per kubectlResourceOperations")
	}
	assert.Equal(t, int64(1), client.calls.Load())
}

func TestNewValidatorStrictClientSideValidation(t *testing.T) {
	t.Parallel()
	// Client-side schema validation reads the v2 models; failing that read shows whether it ran.
	errClientSide := errors.New("client-side validation ran")
	tests := []struct {
		name    string
		obj     string
		wantErr error
	}{
		{"supported kind skips client-side validation", `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"cm"}}`, nil},
		{"unsupported kind falls back to client-side validation", `{"apiVersion":"example.com/v1","kind":"Foo","metadata":{"name":"f"}}`, errClientSide},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := &countingClient{hash: rand.Text()}
			f := cmdtesting.NewTestFactory()
			t.Cleanup(f.Cleanup)
			f.FakeDynamicClient = testDynamicClient
			f.OpenAPIV3ClientFunc = func() (openapi.Client, error) { return client, nil }
			f.OpenAPISchemaFunc = func() (kubectlopenapi.Resources, error) { return nil, errClientSide }
			k, _ := newTestKubectlResourceOperations(t)
			k.fact = f

			v, err := k.newValidator(metav1.FieldValidationStrict)
			require.NoError(t, err)
			err = v.ValidateBytes([]byte(tt.obj))
			if tt.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

// TestFieldValidationV3VerifierRejectsUnlistedKind pins the v3 behaviour that the v2
// fallback in newValidator exists for: https://github.com/kubernetes/kubernetes/issues/143292.
func TestFieldValidationV3VerifierRejectsUnlistedKind(t *testing.T) {
	t.Parallel()
	err := newTestVerifier(&countingClient{hash: "A"}, newFieldValidationCache()).HasSupport(quxGVK)
	require.Error(t, err)
	assert.False(t, resource.IsParamUnsupportedError(err),
		"v3 verifier now handles kinds missing from a served group-version "+
			"(https://github.com/kubernetes/kubernetes/issues/143292); remove the v2 fallback in newValidator")
}

// v2Discovery serves only an OpenAPI v2 document in which Namespace PATCH supports
// fieldValidation, which is what the v2 verifier checks for CRDs.
type v2Discovery struct {
	discovery.CachedDiscoveryInterface
}

func (v2Discovery) OpenAPISchema() (*openapi_v2.Document, error) {
	return openapi_v2.ParseDocument([]byte(`{"swagger":"2.0","info":{"title":"t","version":"v1"},"paths":{` +
		`"/api/v1/namespaces/{name}":{"patch":{"x-kubernetes-group-version-kind":{"group":"","version":"v1","kind":"Namespace"},` +
		`"parameters":[{"name":"fieldValidation","in":"query","type":"string"}],"responses":{"200":{"description":"OK"}}}}}}`))
}

func TestNewValidatorStrictFallsBackToV2(t *testing.T) {
	t.Parallel()
	// Qux is a CRD kind whose group-version document is served but does not list it yet,
	// e.g. a CRD added to an existing group earlier in the same sync.
	tests := []struct {
		name      string
		schemaErr error
	}{
		// See TestFieldValidationV3VerifierRejectsUnlistedKind.
		{"kind not listed in v3 document", nil},
		{"v3 document fetch fails", errors.New("v3 unavailable")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dyn := newCRDDynamicClient()
			createCRD(t, dyn, quxGVK)

			client := &countingClient{hash: rand.Text(), schemaErr: tt.schemaErr}
			f := cmdtesting.NewTestFactory().WithDiscoveryClient(v2Discovery{})
			t.Cleanup(f.Cleanup)
			f.FakeDynamicClient = dyn
			f.OpenAPIV3ClientFunc = func() (openapi.Client, error) { return client, nil }
			k, _ := newTestKubectlResourceOperations(t)
			k.fact = f

			v, err := k.newValidator(metav1.FieldValidationStrict)
			require.NoError(t, err)
			assert.NoError(t, v.ValidateBytes([]byte(`{"apiVersion":"example.com/v1","kind":"Qux","metadata":{"name":"q"}}`)))
		})
	}
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
