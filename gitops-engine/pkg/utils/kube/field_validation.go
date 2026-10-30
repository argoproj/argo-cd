package kube

import (
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/openapi"
	kubectlopenapi "k8s.io/kubectl/pkg/util/openapi"
	"k8s.io/kubectl/pkg/validation"
)

// maxFieldValidationCacheEntries bounds the cache; it is reset when full, since keys only
// change with a schema.
const maxFieldValidationCacheEntries = 10000

// fieldValidationSupport records whether a GVK supports the fieldValidation query param.
// kubectl's Factory.Validator builds a new verifier per apply, so without it every apply
// parses its group-version's full OpenAPI v3 document.
var fieldValidationSupport = newFieldValidationCache()

// fieldValidationCacheKey is the group-version's OpenAPI v3 URL, which carries the
// document hash, plus the kind.
type fieldValidationCacheKey string

type fieldValidationCache struct {
	mu      sync.RWMutex
	entries map[fieldValidationCacheKey]error
}

func newFieldValidationCache() *fieldValidationCache {
	return &fieldValidationCache{entries: map[fieldValidationCacheKey]error{}}
}

func (c *fieldValidationCache) get(key fieldValidationCacheKey) (found bool, err error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	err, found = c.entries[key]
	return found, err
}

func (c *fieldValidationCache) set(key fieldValidationCacheKey, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= maxFieldValidationCacheEntries {
		c.entries = map[fieldValidationCacheKey]error{}
	}
	c.entries[key] = err
}

// hashKeyedVerifier caches next's definite answers by document hash; errors pass through
// uncached.
type hashKeyedVerifier struct {
	client openapi.Client
	next   resource.Verifier
	cache  *fieldValidationCache
}

// newFieldValidationVerifier wraps the OpenAPI v3 fieldValidation verifier with cache.
func newFieldValidationVerifier(dyn dynamic.Interface, client openapi.Client, cache *fieldValidationCache) resource.Verifier {
	return &hashKeyedVerifier{
		client: client,
		next:   resource.NewQueryParamVerifierV3(dyn, client, resource.QueryParamFieldValidation),
		cache:  cache,
	}
}

// HasSupport implements resource.Verifier, answering from the cache when the group-version's
// document hash and kind were seen before.
func (v *hashKeyedVerifier) HasSupport(gvk schema.GroupVersionKind) error {
	key, ok := v.cacheKey(gvk)
	if !ok {
		return v.next.HasSupport(gvk)
	}
	if found, err := v.cache.get(key); found {
		return err
	}
	err := v.next.HasSupport(gvk)
	if err == nil || resource.IsParamUnsupportedError(err) {
		v.cache.set(key, err)
	}
	return err
}

func (v *hashKeyedVerifier) cacheKey(gvk schema.GroupVersionKind) (fieldValidationCacheKey, bool) {
	paths, err := v.client.Paths()
	if err != nil {
		return "", false
	}
	gv := gvk.GroupVersion()
	apiPath := "api/" + gv.Version
	if gv.Group != "" {
		apiPath = "apis/" + gv.Group + "/" + gv.Version
	}
	gvDoc, ok := paths[apiPath]
	if !ok {
		return "", false
	}
	url := gvDoc.ServerRelativeURL()
	if !strings.Contains(url, "hash=") {
		return "", false
	}
	return fieldValidationCacheKey(url + " " + gvk.Kind), true
}

// newValidator is kubectl's Factory.Validator with the v3 verifier cached. The v2 fallback
// covers v3 errors other than ParamUnsupported, e.g. a CRD kind not yet in its served
// group-version document.
func (k *kubectlResourceOperations) newValidator(validationDirective string) (validation.Schema, error) {
	if validationDirective == metav1.FieldValidationIgnore {
		return validation.NullSchema{}, nil
	}

	schema := validation.ConjunctiveSchema{
		validation.NewSchemaValidation(k.fact),
		validation.NoDoubleKeySchema{},
	}

	dynamicClient, err := k.fact.DynamicClient()
	if err != nil {
		return nil, err
	}
	oapiV3Client, err := k.fact.OpenAPIV3Client()
	if err != nil {
		return nil, err
	}
	discoveryClient, err := k.fact.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	k.openAPIV2Once.Do(func() {
		k.openAPIV2Getter = kubectlopenapi.NewOpenAPIGetter(discoveryClient)
	})
	queryParam := resource.QueryParamFieldValidation
	primary := newFieldValidationVerifier(dynamicClient, oapiV3Client, fieldValidationSupport)
	secondary := resource.NewQueryParamVerifier(dynamicClient, k.openAPIV2Getter, queryParam)
	verifier := resource.NewFallbackQueryParamVerifier(primary, secondary)
	return validation.NewParamVerifyingSchema(schema, verifier, validationDirective), nil
}
