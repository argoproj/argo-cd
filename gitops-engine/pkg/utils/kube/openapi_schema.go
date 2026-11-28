package kube

import (
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

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

const (
	// openAPIIndexTTL bounds how long a cached v3 index is trusted without an invalidation, e.g. when
	// an aggregated API changes its schema or CRD events aren't watched.
	openAPIIndexTTL = time.Minute
	// openAPIIndexSettle is how long after Invalidate the API server may still serve the old index,
	// as it regenerates OpenAPI documents asynchronously.
	openAPIIndexSettle = 10 * time.Second
)

// OpenAPISchemaCache keeps the structured merge diff types built for a cluster's kinds across
// resolvers, but not the documents they were built from. A group version's types are reused while
// the cluster serves it under the same hash. The v3 index that holds the hashes is cached too, until
// Invalidate is called or it expires.
type OpenAPISchemaCache struct {
	mu            sync.Mutex
	groupVersions map[schema.GroupVersion]*groupVersionTypes
	// urls maps v3 index paths (e.g. "apis/apps/v1") to their document URLs, which include the hash.
	urls        map[string]string
	urlsExpiry  time.Time
	settleUntil time.Time
	now         func() time.Time
}

func NewOpenAPISchemaCache() *OpenAPISchemaCache {
	return &OpenAPISchemaCache{groupVersions: map[schema.GroupVersion]*groupVersionTypes{}, now: time.Now}
}

// Invalidate drops the cached index, so that resolvers fetch it again to learn the current hashes.
// Call it when the cluster's schemas may have changed, e.g. on CRD and APIService events.
func (c *OpenAPISchemaCache) Invalidate() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.urls = nil
	c.settleUntil = c.now().Add(openAPIIndexSettle)
}

func (c *OpenAPISchemaCache) indexURLs() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.urls == nil || !c.now().Before(c.urlsExpiry) {
		return nil
	}
	return c.urls
}

func (c *OpenAPISchemaCache) setIndex(paths map[string]openapi.GroupVersion) {
	urls := make(map[string]string, len(paths))
	for path, gv := range paths {
		urls[path] = gv.ServerRelativeURL()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	c.urls = urls
	c.urlsExpiry = now.Add(openAPIIndexTTL)
	// An index fetched right after an invalidation may predate the change, so refetch it once the
	// API server had time to catch up.
	if now.Before(c.settleUntil) {
		c.urlsExpiry = c.settleUntil
	}
}

// NewOpenAPISchemaResolver returns a resolver that fetches schemas using config. Nothing is fetched
// until a schema is requested.
func NewOpenAPISchemaResolver(config *rest.Config, log logr.Logger) OpenAPISchemaResolver {
	return NewCachedOpenAPISchemaResolver(config, log, nil)
}

// NewCachedOpenAPISchemaResolver is NewOpenAPISchemaResolver that reuses the index and types from
// cache, which may be nil. It still fetches the document for LookupPatchMeta.
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

// groupVersionTypes is what resolvers share through the cache: a group version's GVK index and the
// types built so far.
type groupVersionTypes struct {
	url string // the document's ServerRelativeURL, which includes its hash
	// models maps each GVK to the name of its schema.
	models map[schema.GroupVersionKind]string

	mu sync.Mutex
	// parsers maps schema names to a parser built from that schema and the schemas it refers to.
	parsers map[string]*typed.Parser
}

// groupVersionSchema is a group version as loaded by one resolver.
type groupVersionSchema struct {
	gv    schema.GroupVersion
	types *groupVersionTypes
	// raw holds the document's schemas undecoded. It is nil until needed if types came from the cache.
	raw map[string]jsontext.Value
	// closures holds, for each schema name, that schema and every schema it refers to, decoded.
	closures map[string]map[string]*spec.Schema
}

type openAPIV2Schema struct {
	resources kubectlopenapi.Resources
	parser    *managedfields.GvkParser
}

func (r *openAPISchemaResolver) Type(gvk schema.GroupVersionKind) (*typed.ParseableType, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	gv, err := r.load(gvk.GroupVersion())
	if err != nil {
		return nil, err
	}
	if r.v2 != nil {
		return r.v2.parser.Type(gvk), nil
	}
	if gv == nil {
		return nil, nil
	}
	name, ok := gv.types.models[gvk]
	if !ok {
		return nil, nil
	}
	gv.types.mu.Lock()
	parser := gv.types.parsers[name]
	gv.types.mu.Unlock()
	if parser == nil {
		schemas, err := r.closure(gv, name)
		if err != nil {
			return nil, err
		}
		converted, err := schemaconv.ToSchemaFromOpenAPI(schemas, false)
		if err != nil {
			return nil, fmt.Errorf("failed to convert OpenAPI schema of %s: %w", gvk, err)
		}
		parser = &typed.Parser{Schema: smdschema.Schema{Types: converted.Types}}
		gv.types.mu.Lock()
		gv.types.parsers[name] = parser
		gv.types.mu.Unlock()
	}
	pt := parser.Type(name)
	return &pt, nil
}

func (r *openAPISchemaResolver) LookupPatchMeta(gvk schema.GroupVersionKind) (strategicpatch.LookupPatchMeta, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	gv, err := r.load(gvk.GroupVersion())
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
	name, ok := gv.types.models[gvk]
	if !ok {
		return nil, nil
	}
	schemas, err := r.closure(gv, name)
	if err != nil {
		return nil, err
	}
	return patchMetaV3{schemas: schemas, schema: schemas[name]}, nil
}

// closure decodes name's schema and every schema it refers to, fetching the document if needed.
func (r *openAPISchemaResolver) closure(gv *groupVersionSchema, name string) (map[string]*spec.Schema, error) {
	if schemas, ok := gv.closures[name]; ok {
		return schemas, nil
	}
	if gv.raw == nil {
		raw, err := r.fetchSchemas(gv.gv)
		if err != nil {
			return nil, err
		}
		gv.raw = raw
	}
	schemas := map[string]*spec.Schema{}
	todo := []string{name}
	for len(todo) > 0 {
		next := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if _, ok := schemas[next]; ok {
			continue
		}
		raw, ok := gv.raw[next]
		if !ok {
			continue
		}
		s := &spec.Schema{}
		if err := json.Unmarshal(raw, s); err != nil {
			return nil, fmt.Errorf("error parsing OpenAPI v3 schema %s: %w", next, err)
		}
		schemas[next] = s
		todo = appendRefs(todo, s)
	}
	gv.closures[name] = schemas
	return schemas, nil
}

// appendRefs appends the names of the component schemas that s refers to.
func appendRefs(names []string, s *spec.Schema) []string {
	if s == nil {
		return names
	}
	if ref := s.Ref.String(); ref != "" {
		names = append(names, strings.TrimPrefix(ref, "#/components/schemas/"))
	}
	for _, list := range [][]spec.Schema{s.AllOf, s.AnyOf, s.OneOf} {
		for i := range list {
			names = appendRefs(names, &list[i])
		}
	}
	names = appendRefs(names, s.Not)
	for _, schemas := range []map[string]spec.Schema{s.Properties, s.PatternProperties, s.Definitions} {
		for _, p := range schemas {
			names = appendRefs(names, &p)
		}
	}
	if s.AdditionalProperties != nil {
		names = appendRefs(names, s.AdditionalProperties.Schema)
	}
	if s.AdditionalItems != nil {
		names = appendRefs(names, s.AdditionalItems.Schema)
	}
	if s.Items != nil {
		names = appendRefs(names, s.Items.Schema)
		for i := range s.Items.Schemas {
			names = appendRefs(names, &s.Items.Schemas[i])
		}
	}
	return names
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

func preservesUnknownFields(s *spec.Schema) bool {
	preserve, _ := s.Extensions.GetBool("x-kubernetes-preserve-unknown-fields")
	return preserve
}

// load returns gv, loading its GVK index if needed. It returns nil if gv isn't served, or if the
// server only serves OpenAPI v2, in which case r.v2 is set.
func (r *openAPISchemaResolver) load(gv schema.GroupVersion) (*groupVersionSchema, error) {
	if r.v2 != nil {
		return nil, nil
	}
	if s, ok := r.groupVersions[gv]; ok {
		return s, nil
	}
	s, err := r.loadGroupVersion(gv)
	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI schema for %s: %w", gv, err)
	}
	return s, nil
}

func (r *openAPISchemaResolver) loadGroupVersion(gv schema.GroupVersion) (*groupVersionSchema, error) {
	var url string
	var ok bool
	if r.v3Paths == nil && r.cache != nil {
		url, ok = r.cache.indexURLs()[openAPIV3Path(gv)]
	}
	// Without a cached index, or if gv isn't in it because it was just added, use a fresh one.
	if !ok {
		if err := r.loadIndex(); err != nil {
			return nil, err
		}
		if r.v2 != nil {
			return nil, nil
		}
		path, served := r.v3Paths[openAPIV3Path(gv)]
		if !served {
			r.groupVersions[gv] = nil
			return nil, nil
		}
		url = path.ServerRelativeURL()
	}
	s := &groupVersionSchema{gv: gv, closures: map[string]map[string]*spec.Schema{}}
	if r.cache != nil {
		r.cache.mu.Lock()
		cached := r.cache.groupVersions[gv]
		r.cache.mu.Unlock()
		if cached != nil && cached.url == url {
			s.types = cached
			r.groupVersions[gv] = s
			return s, nil
		}
	}
	raw, err := r.fetchSchemas(gv)
	if err != nil {
		return nil, err
	}
	// The document comes from a fresh index, which may be newer than the cached one.
	url = r.v3Paths[openAPIV3Path(gv)].ServerRelativeURL()
	models, duplicates := indexModelsByGVK(raw)
	if len(duplicates) > 0 {
		r.log.Info("Duplicate GVKs detected in OpenAPI schema. This could cause inaccurate diffs.", "gvks", duplicates)
	}
	s.raw = raw
	s.types = &groupVersionTypes{url: url, models: models, parsers: map[string]*typed.Parser{}}
	if r.cache != nil {
		r.cache.mu.Lock()
		r.cache.groupVersions[gv] = s.types
		r.cache.mu.Unlock()
	}
	r.groupVersions[gv] = s
	return s, nil
}

// loadIndex fetches the v3 index once per resolver, or sets r.v2 if the server doesn't serve v3.
func (r *openAPISchemaResolver) loadIndex() error {
	if r.v3Paths != nil || r.v2 != nil {
		return nil
	}
	if r.client == nil {
		client, err := r.newClient()
		if err != nil {
			return fmt.Errorf("failed to create discovery client: %w", err)
		}
		r.client = client
	}
	paths, err := r.client.OpenAPIV3().Paths()
	if apierrors.IsNotFound(err) {
		return r.loadV2()
	}
	if err != nil {
		return fmt.Errorf("error getting OpenAPI v3 paths: %w", err)
	}
	r.v3Paths = paths
	if r.cache != nil {
		r.cache.setIndex(paths)
	}
	return nil
}

// fetchSchemas fetches gv's v3 document and returns its component schemas without decoding them.
func (r *openAPISchemaResolver) fetchSchemas(gv schema.GroupVersion) (map[string]jsontext.Value, error) {
	if err := r.loadIndex(); err != nil {
		return nil, err
	}
	path, ok := r.v3Paths[openAPIV3Path(gv)]
	if !ok {
		return nil, fmt.Errorf("OpenAPI v3 document of %s is no longer served", gv)
	}
	data, err := path.Schema(runtime.ContentTypeJSON)
	if err != nil {
		return nil, fmt.Errorf("error getting OpenAPI v3 document: %w", err)
	}
	var doc struct {
		Components struct {
			Schemas map[string]jsontext.Value `json:"schemas"`
		} `json:"components"`
	}
	if err := jsonv2.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("error parsing OpenAPI v3 document: %w", err)
	}
	if doc.Components.Schemas == nil {
		return map[string]jsontext.Value{}, nil
	}
	return doc.Components.Schemas, nil
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
func indexModelsByGVK(schemas map[string]jsontext.Value) (map[schema.GroupVersionKind]string, []schema.GroupVersionKind) {
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

// specGroupVersionKinds decodes only the x-kubernetes-group-version-kind extension of a schema,
// skipping entries without a kind like parseGroupVersionKind does.
func specGroupVersionKinds(raw jsontext.Value) []schema.GroupVersionKind {
	var ext struct {
		GVKs []struct {
			Group   string `json:"group"`
			Version string `json:"version"`
			Kind    string `json:"kind"`
		} `json:"x-kubernetes-group-version-kind"`
	}
	if err := jsonv2.Unmarshal(raw, &ext); err != nil {
		return nil
	}
	var gvks []schema.GroupVersionKind
	for _, item := range ext.GVKs {
		if item.Kind == "" {
			continue
		}
		gvks = append(gvks, schema.GroupVersionKind{Group: item.Group, Version: item.Version, Kind: item.Kind})
	}
	return gvks
}
