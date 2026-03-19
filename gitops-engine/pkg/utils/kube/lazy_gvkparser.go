package kube

import (
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	openapi_v3 "github.com/google/gnostic-models/openapiv3"
	"golang.org/x/sync/singleflight"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/managedfields"
	openapiclient "k8s.io/client-go/openapi"
	"k8s.io/kube-openapi/pkg/util/proto"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
)

// gvResult holds the result of loading a GroupVersion's schema.
// Both the parser and the error are cached so that a persistently
// failing GV (e.g. bad CRD) doesn't retry on every Type() call.
type gvResult struct {
	parser *managedfields.GvkParser
	err    error
}

// lazyGVKParser implements scheme.GVKParser by lazily loading OpenAPI v3
// schemas on a per-GroupVersion basis. Instead of fetching and parsing all
// GV schemas upfront, it only fetches a GV's schema when Type() is first
// called for a GVK in that GroupVersion. Parsed results are cached for
// subsequent calls.
//
// It also implements scheme.GVKErrorReporter, allowing external callers
// (e.g. the cluster cache) to inject per-GV errors that surface through
// Type(). This is used to report list/watch failures such as conversion
// webhook errors.
type lazyGVKParser struct {
	paths map[string]openapiclient.GroupVersion
	log   logr.Logger

	sf             singleflight.Group
	parsers        sync.Map // map[string]*gvResult
	reportedErrors sync.Map // map[string]error — injected by cluster cache
}

// newLazyGVKParser creates a lazy parser from the already-fetched GV paths.
// The paths map is obtained from a single cheap call to client.Paths().
func newLazyGVKParser(paths map[string]openapiclient.GroupVersion, log logr.Logger) *lazyGVKParser {
	return &lazyGVKParser{
		paths: paths,
		log:   log,
	}
}

// ReportError injects an error for a GVK's GroupVersion. Subsequent calls to
// Type() for any GVK in that GroupVersion will return this error. This is used
// by the cluster cache to report list/watch failures (e.g. conversion webhook
// errors) so they surface to consumers through the same Type() channel as
// schema errors.
func (p *lazyGVKParser) ReportError(gvk schema.GroupVersionKind, err error) {
	path := gvPathForGVK(gvk)
	p.reportedErrors.Store(path, err)
}

// Type resolves a GVK to its ParseableType by lazily loading the schema for
// the GVK's GroupVersion on first access. Returns an error if the schema
// could not be loaded (e.g. bad CRD, network failure), or if an external
// error was reported for this GV. Returns (nil, nil) if the GV is simply
// not known to the cluster.
func (p *lazyGVKParser) Type(gvk schema.GroupVersionKind) (*typed.ParseableType, error) {
	path := gvPathForGVK(gvk)

	// Check for externally reported errors first (e.g. list/watch failures).
	if reported, ok := p.reportedErrors.Load(path); ok {
		return nil, reported.(error)
	}

	// Check if this GV exists on the cluster.
	if _, ok := p.paths[path]; !ok {
		return nil, nil
	}

	// Fast path: check cache.
	if cached, ok := p.parsers.Load(path); ok {
		r := cached.(*gvResult)
		if r.err != nil {
			return nil, r.err
		}
		return r.parser.Type(gvk), nil
	}

	// Slow path: fetch and parse the GV schema. singleflight ensures only
	// one goroutine fetches a given GV even under concurrent requests.
	result, _, _ := p.sf.Do(path, func() (interface{}, error) {
		// Double-check cache after acquiring the singleflight slot.
		if cached, ok := p.parsers.Load(path); ok {
			return cached, nil
		}
		parser, loadErr := p.loadGV(path)
		r := &gvResult{parser: parser, err: loadErr}
		if loadErr != nil {
			p.log.Info("Failed to load OpenAPI v3 schema for GroupVersion, skipping", "path", path, "error", loadErr)
		}
		p.parsers.Store(path, r)
		// Always return the result (never an error) so singleflight
		// doesn't suppress it for waiting callers.
		return r, nil
	})

	r := result.(*gvResult)
	if r.err != nil {
		return nil, r.err
	}
	return r.parser.Type(gvk), nil
}

// loadGV fetches, parses, and builds a GvkParser for a single GroupVersion.
func (p *lazyGVKParser) loadGV(path string) (*managedfields.GvkParser, error) {
	gv, ok := p.paths[path]
	if !ok {
		return nil, fmt.Errorf("unknown path: %s", path)
	}

	jsonBytes, err := gv.Schema("application/json")
	if err != nil {
		return nil, fmt.Errorf("failed to get schema: %w", err)
	}

	doc, err := openapi_v3.ParseDocument(jsonBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse document: %w", err)
	}

	models, err := proto.NewOpenAPIV3Data(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to models: %w", err)
	}

	// The v3 proto path produces map[string]interface{} for nested extension
	// values, but managedfields.NewGVKParser expects map[interface{}]interface{}.
	normalizeV3Extensions(models)

	parser, err := managedfields.NewGVKParser(models, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create GVK parser: %w", err)
	}
	return parser, nil
}

// gvPathForGVK derives the OpenAPI v3 path for a GroupVersionKind.
// Core group (empty group) maps to "api/{version}".
// Named groups map to "apis/{group}/{version}".
func gvPathForGVK(gvk schema.GroupVersionKind) string {
	if gvk.Group == "" {
		return "api/" + gvk.Version
	}
	return "apis/" + gvk.Group + "/" + gvk.Version
}