package kube

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/openapi"
	"k8s.io/client-go/rest"
	"k8s.io/kube-openapi/pkg/schemaconv"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"
	kubectlopenapi "k8s.io/kubectl/pkg/util/openapi"
	smdschema "sigs.k8s.io/structured-merge-diff/v6/schema"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
)

// OpenAPISchemaResolver resolves the OpenAPI schema of individual GVKs.
//
// Schemas are fetched from the cluster the first time a GVK of a group version is requested and
// kept for the lifetime of the resolver, so create one per operation (e.g. one app comparison)
// instead of holding on to it.
type OpenAPISchemaResolver interface {
	// Type returns the type used to build structured merge diff values for gvk, or nil if the
	// cluster serves no schema for it.
	Type(gvk schema.GroupVersionKind) (*typed.ParseableType, error)
	// LookupPatchMeta returns the strategic merge patch metadata for gvk, or nil if the cluster
	// serves no schema for it.
	LookupPatchMeta(gvk schema.GroupVersionKind) (strategicpatch.LookupPatchMeta, error)
}

// OpenAPIClient fetches OpenAPI documents from a cluster.
type OpenAPIClient interface {
	discovery.OpenAPISchemaInterface
	discovery.OpenAPIV3SchemaInterface
}

// OpenAPISchemaCache keeps the structured merge diff types of a cluster's v3 group version
// documents across resolvers, but not the documents, which are several times larger. An entry is
// reused while the cluster serves its group version under the same hash.
type OpenAPISchemaCache struct {
	mu            sync.Mutex
	groupVersions map[schema.GroupVersion]cachedGroupVersion
}

type cachedGroupVersion struct {
	url    string              // the document's ServerRelativeURL, which includes its hash
	schema *groupVersionSchema // without schemas
}

func NewOpenAPISchemaCache() *OpenAPISchemaCache {
	return &OpenAPISchemaCache{groupVersions: map[schema.GroupVersion]cachedGroupVersion{}}
}

// NewOpenAPISchemaResolver returns a resolver that fetches schemas using config. Nothing is fetched
// until a schema is requested.
func NewOpenAPISchemaResolver(config *rest.Config, log logr.Logger) OpenAPISchemaResolver {
	return NewCachedOpenAPISchemaResolver(config, log, nil)
}

// NewCachedOpenAPISchemaResolver is NewOpenAPISchemaResolver that reuses types from cache, which
// may be nil. It still fetches the v3 index once, to learn the current hashes, and fetches the
// document for LookupPatchMeta.
func NewCachedOpenAPISchemaResolver(config *rest.Config, log logr.Logger, cache *OpenAPISchemaCache) OpenAPISchemaResolver {
	r := newOpenAPISchemaResolver(func() (OpenAPIClient, error) {
		return discovery.NewDiscoveryClientForConfig(config)
	}, log)
	r.cache = cache
	return r
}

// NewOpenAPISchemaResolverFromClient returns a resolver that fetches schemas using client.
func NewOpenAPISchemaResolverFromClient(client OpenAPIClient, log logr.Logger) OpenAPISchemaResolver {
	return newOpenAPISchemaResolver(func() (OpenAPIClient, error) { return client, nil }, log)
}

func newOpenAPISchemaResolver(newClient func() (OpenAPIClient, error), log logr.Logger) *openAPISchemaResolver {
	return &openAPISchemaResolver{newClient: newClient, log: log, groupVersions: map[schema.GroupVersion]*groupVersionSchema{}}
}

type openAPISchemaResolver struct {
	newClient func() (OpenAPIClient, error)
	log       logr.Logger

	mu     sync.Mutex
	client OpenAPIClient
	// v3Paths maps OpenAPI v3 paths (e.g. "apis/apps/v1") to their documents.
	v3Paths map[string]openapi.GroupVersion
	// groupVersions caches loaded v3 documents. A nil value means the group version isn't served.
	groupVersions map[schema.GroupVersion]*groupVersionSchema
	// v2 is set instead of v3Paths when the server does not serve OpenAPI v3.
	v2    *openAPIV2Schema
	cache *OpenAPISchemaCache
}

type groupVersionSchema struct {
	schemas map[string]*spec.Schema // nil if taken from the cache
	// models maps each GVK to the name of its schema.
	models map[schema.GroupVersionKind]string

	// The parser is built on first use; a cached document is shared between resolvers.
	parserOnce sync.Once
	parser     *typed.Parser
	parserErr  error
}

type openAPIV2Schema struct {
	resources kubectlopenapi.Resources
	parser    *managedfields.GvkParser
}

func (r *openAPISchemaResolver) Type(gvk schema.GroupVersionKind) (*typed.ParseableType, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	gv, err := r.load(gvk, false)
	if err != nil {
		return nil, err
	}
	if r.v2 != nil {
		return r.v2.parser.Type(gvk), nil
	}
	if gv == nil {
		return nil, nil
	}
	name, ok := gv.models[gvk]
	if !ok {
		return nil, nil
	}
	parser, err := gv.typeParser(gvk.GroupVersion())
	if err != nil {
		return nil, err
	}
	pt := parser.Type(name)
	return &pt, nil
}

func (r *openAPISchemaResolver) LookupPatchMeta(gvk schema.GroupVersionKind) (strategicpatch.LookupPatchMeta, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	gv, err := r.load(gvk, true)
	if err != nil {
		return nil, err
	}
	if r.v2 != nil {
		s := r.v2.resources.LookupResource(gvk)
		if s == nil {
			return nil, nil
		}
		return strategicpatch.NewPatchMetaFromOpenAPI(s), nil
	}
	if gv == nil {
		return nil, nil
	}
	name, ok := gv.models[gvk]
	if !ok {
		return nil, nil
	}
	return patchMetaV3{schemas: gv.schemas, schema: gv.schemas[name]}, nil
}

// patchMetaV3 is strategicpatch.PatchMetaFromOpenAPIV3 that also traverses additionalProperties and
// fields that allow unknown keys, such as x-kubernetes-preserve-unknown-fields. Like the v2 Arbitrary
// schema, it returns unspecified metadata below the latter.
type patchMetaV3 struct {
	schemas map[string]*spec.Schema
	// schema is nil if the patch metadata is unspecified.
	schema *spec.Schema
}

func (m patchMetaV3) LookupPatchMetadataForStruct(key string) (strategicpatch.LookupPatchMeta, strategicpatch.PatchMeta, error) {
	return m.lookup(key, false)
}

func (m patchMetaV3) LookupPatchMetadataForSlice(key string) (strategicpatch.LookupPatchMeta, strategicpatch.PatchMeta, error) {
	return m.lookup(key, true)
}

func (m patchMetaV3) Name() string {
	if m.schema != nil && len(m.schema.Type) > 0 {
		return strings.Join(m.schema.Type, "")
	}
	return "Struct"
}

func (m patchMetaV3) lookup(key string, slice bool) (strategicpatch.LookupPatchMeta, strategicpatch.PatchMeta, error) {
	unspecified := patchMetaV3{schemas: m.schemas}
	if m.schema == nil {
		return unspecified, strategicpatch.PatchMeta{}, nil
	}
	var field *spec.Schema
	additional := m.schema.AdditionalProperties
	property, declared := m.schema.Properties[key]
	switch {
	case declared:
		field = &property
	case additional != nil && additional.Schema != nil:
		field = additional.Schema
	case len(m.schema.Properties) == 0 || (additional != nil && additional.Allows) || preservesUnknownFields(m.schema):
		return unspecified, strategicpatch.PatchMeta{}, nil
	default:
		return nil, strategicpatch.PatchMeta{}, fmt.Errorf("unable to find api field %q", key)
	}

	var meta strategicpatch.PatchMeta
	if mergeKey, ok := field.Extensions.GetString("x-kubernetes-patch-merge-key"); ok {
		meta.SetPatchMergeKey(mergeKey)
	}
	if strategy, ok := field.Extensions.GetString("x-kubernetes-patch-strategy"); ok {
		meta.SetPatchStrategies(strings.Split(strategy, ","))
	}
	if slice && field.Items != nil {
		if field.Items.Schema == nil {
			return unspecified, meta, nil
		}
		field = field.Items.Schema
	}
	if len(field.AllOf) > 0 {
		field = &field.AllOf[0]
	}
	if ref := field.Ref.String(); ref != "" {
		resolved, ok := m.schemas[strings.TrimPrefix(ref, "#/components/schemas/")]
		if !ok {
			return nil, meta, fmt.Errorf("unable to resolve %s in OpenAPI V3", ref)
		}
		field = resolved
	}
	return patchMetaV3{schemas: m.schemas, schema: field}, meta, nil
}

func (s *groupVersionSchema) typeParser(gv schema.GroupVersion) (*typed.Parser, error) {
	s.parserOnce.Do(func() {
		converted, err := schemaconv.ToSchemaFromOpenAPI(s.schemas, false)
		if err != nil {
			s.parserErr = fmt.Errorf("failed to convert OpenAPI schema of %s: %w", gv, err)
			return
		}
		s.parser = &typed.Parser{Schema: smdschema.Schema{Types: converted.Types}}
	})
	return s.parser, s.parserErr
}

func preservesUnknownFields(s *spec.Schema) bool {
	preserve, _ := s.Extensions.GetBool("x-kubernetes-preserve-unknown-fields")
	return preserve
}

// load returns the v3 document of gvk's group version, loading it if needed; withSchemas loads the
// document even if the cache has its types. It returns nil if the group version isn't served, or if
// the server only serves OpenAPI v2, in which case r.v2 is set.
func (r *openAPISchemaResolver) load(gvk schema.GroupVersionKind, withSchemas bool) (*groupVersionSchema, error) {
	if r.v2 != nil {
		return nil, nil
	}
	if gv, ok := r.groupVersions[gvk.GroupVersion()]; ok && (gv == nil || gv.schemas != nil || !withSchemas) {
		return gv, nil
	}
	gv, err := r.loadGroupVersion(gvk.GroupVersion(), withSchemas)
	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI schema for %s: %w", gvk.GroupVersion(), err)
	}
	return gv, nil
}

func (r *openAPISchemaResolver) loadGroupVersion(gv schema.GroupVersion, withSchemas bool) (*groupVersionSchema, error) {
	if r.client == nil {
		client, err := r.newClient()
		if err != nil {
			return nil, fmt.Errorf("failed to create discovery client: %w", err)
		}
		r.client = client
	}
	if r.v3Paths == nil {
		paths, err := r.client.OpenAPIV3().Paths()
		if apierrors.IsNotFound(err) {
			return nil, r.loadV2()
		}
		if err != nil {
			return nil, fmt.Errorf("error getting OpenAPI v3 paths: %w", err)
		}
		r.v3Paths = paths
	}
	path, ok := r.v3Paths[openAPIV3Path(gv)]
	if !ok {
		r.groupVersions[gv] = nil
		return nil, nil
	}
	url := path.ServerRelativeURL()
	if r.cache != nil && !withSchemas {
		r.cache.mu.Lock()
		cached, ok := r.cache.groupVersions[gv]
		r.cache.mu.Unlock()
		if ok && cached.url == url {
			r.groupVersions[gv] = cached.schema
			return cached.schema, nil
		}
	}
	data, err := path.Schema(runtime.ContentTypeJSON)
	if err != nil {
		return nil, fmt.Errorf("error getting OpenAPI v3 document: %w", err)
	}
	var doc spec3.OpenAPI
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("error parsing OpenAPI v3 document: %w", err)
	}
	s := &groupVersionSchema{models: map[schema.GroupVersionKind]string{}}
	if doc.Components != nil {
		s.schemas = doc.Components.Schemas
	}
	var duplicates []schema.GroupVersionKind
	s.models, duplicates = indexModelsByGVK(s.schemas)
	if len(duplicates) > 0 {
		r.log.Info("Duplicate GVKs detected in OpenAPI schema. This could cause inaccurate diffs.", "gvks", duplicates)
	}
	r.groupVersions[gv] = s
	if r.cache != nil {
		if _, err := s.typeParser(gv); err == nil {
			types := &groupVersionSchema{models: s.models, parser: s.parser}
			types.parserOnce.Do(func() {})
			r.cache.mu.Lock()
			r.cache.groupVersions[gv] = cachedGroupVersion{url: url, schema: types}
			r.cache.mu.Unlock()
		}
	}
	return s, nil
}

func (r *openAPISchemaResolver) loadV2() error {
	resources, parser, err := loadOpenAPISchemaV2(r.client, r.log)
	if err != nil {
		return err
	}
	r.v2 = &openAPIV2Schema{resources: resources, parser: parser}
	return nil
}

func openAPIV3Path(gv schema.GroupVersion) string {
	if gv.Group == "" {
		return "api/" + gv.Version
	}
	return "apis/" + gv.Group + "/" + gv.Version
}

// indexModelsByGVK maps each GVK to the name of the schema that declares it. Like newUniqueModels,
// it walks schemas in name order and skips a schema if any of its GVKs was already declared, and
// returns the GVKs of the skipped schemas.
func indexModelsByGVK(schemas map[string]*spec.Schema) (map[schema.GroupVersionKind]string, []schema.GroupVersionKind) {
	names := make([]string, 0, len(schemas))
	for name := range schemas {
		names = append(names, name)
	}
	sort.Strings(names)

	models := map[schema.GroupVersionKind]string{}
	var duplicates []schema.GroupVersionKind
	for _, name := range names {
		gvks := specGroupVersionKinds(schemas[name])
		duplicate := false
		for _, gvk := range gvks {
			if _, ok := models[gvk]; ok {
				duplicates = append(duplicates, gvk)
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		for _, gvk := range gvks {
			models[gvk] = name
		}
	}
	return models, duplicates
}

// specGroupVersionKinds returns the GVKs of the x-kubernetes-group-version-kind extension, skipping
// entries without a kind like parseGroupVersionKind does.
func specGroupVersionKinds(s *spec.Schema) []schema.GroupVersionKind {
	if s == nil {
		return nil
	}
	var list []map[string]string
	if err := s.Extensions.GetObject(groupVersionKindExtensionKey, &list); err != nil {
		return nil
	}
	var gvks []schema.GroupVersionKind
	for _, item := range list {
		if item["kind"] == "" {
			continue
		}
		gvks = append(gvks, schema.GroupVersionKind{Group: item["group"], Version: item["version"], Kind: item["kind"]})
	}
	return gvks
}
