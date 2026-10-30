package normalizers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/diff"
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/itchyny/gojq"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/glob"
)

const (
	// DefaultJQExecutionTimeout is the maximum time allowed for a JQ patch to execute
	DefaultJQExecutionTimeout = 1 * time.Second
)

type normalizerPatch interface {
	GetGroupKind() schema.GroupKind
	GetNamespace() string
	GetName() string
	// Apply(un *unstructured.Unstructured) (error)
	Apply(data []byte) ([]byte, error)
	// Paths returns the paths this patch removes from doc (a JSON-decoded
	// document), as sequences of map keys (string) and list indexes (int).
	// Paths are resolved against doc as given, before any removal.
	Paths(doc any) ([][]any, error)
}

type baseNormalizerPatch struct {
	groupKind schema.GroupKind
	namespace string
	name      string
}

func (np *baseNormalizerPatch) GetGroupKind() schema.GroupKind {
	return np.groupKind
}

func (np *baseNormalizerPatch) GetNamespace() string {
	return np.namespace
}

func (np *baseNormalizerPatch) GetName() string {
	return np.name
}

type jsonPatchNormalizerPatch struct {
	baseNormalizerPatch
	patch   *jsonpatch.Patch
	pointer string
}

func (np *jsonPatchNormalizerPatch) Paths(doc any) ([][]any, error) {
	p, ok := resolvePointer(doc, np.pointer)
	if !ok {
		return nil, nil
	}
	return [][]any{p}, nil
}

// resolvePointer turns an RFC 6901 JSON pointer into a path of map keys and
// list indexes into doc. It returns false if the pointer does not resolve.
func resolvePointer(doc any, pointer string) ([]any, bool) {
	if !strings.HasPrefix(pointer, "/") {
		return nil, false
	}
	parts := strings.Split(pointer[1:], "/")
	cur := doc
	out := make([]any, 0, len(parts))
	for _, part := range parts {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		switch c := cur.(type) {
		case map[string]any:
			v, ok := c[part]
			if !ok {
				return nil, false
			}
			cur = v
			out = append(out, part)
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(c) {
				return nil, false
			}
			cur = c[i]
			out = append(out, i)
		default:
			return nil, false
		}
	}
	return out, true
}

func (np *jsonPatchNormalizerPatch) Apply(data []byte) ([]byte, error) {
	patchedData, err := np.patch.Apply(data)
	if err != nil {
		return nil, err
	}
	return patchedData, nil
}

type jqNormalizerPatch struct {
	baseNormalizerPatch
	code               *gojq.Code
	jqExecutionTimeout time.Duration

	// pathExpression is the user's expression; pathCode, compiled on first use
	// because only RespectIgnoreDifferences needs it, is `[path(<expression>)]`:
	// the paths that `del(<expression>)` removes.
	pathExpression string
	pathOnce       sync.Once
	pathCode       *gojq.Code
	pathErr        error
}

func (np *jqNormalizerPatch) compilePathCode() (*gojq.Code, error) {
	np.pathOnce.Do(func() {
		query, err := gojq.Parse(fmt.Sprintf("[path(%s)]", np.pathExpression))
		if err != nil {
			np.pathErr = err
			return
		}
		np.pathCode, np.pathErr = gojq.Compile(query)
	})
	return np.pathCode, np.pathErr
}

func (np *jqNormalizerPatch) Paths(doc any) ([][]any, error) {
	code, err := np.compilePathCode()
	if err != nil {
		return nil, fmt.Errorf("JQ path query could not be compiled: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), np.jqExecutionTimeout)
	defer cancel()

	iter := code.RunWithContext(ctx, doc)
	first, ok := iter.Next()
	if !ok {
		return nil, errors.New("JQ path query did not return any data")
	}
	if err, ok := first.(error); ok {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("JQ path query timed out (%v)", np.jqExecutionTimeout.String())
		}
		return nil, fmt.Errorf("JQ path query returned error: %w", err)
	}
	raw, ok := first.([]any)
	if !ok {
		return nil, fmt.Errorf("JQ path query returned %T, expected an array of paths", first)
	}
	out := make([][]any, 0, len(raw))
	for _, r := range raw {
		elems, ok := r.([]any)
		if !ok {
			continue
		}
		path, ok := toPath(elems)
		if !ok || !pathExists(doc, path) {
			continue
		}
		out = append(out, path)
	}
	return out, nil
}

// toPath converts a jq path (as returned by path()) into map keys and list
// indexes. Slice paths and other non-addressable elements are rejected.
func toPath(elems []any) ([]any, bool) {
	path := make([]any, 0, len(elems))
	for _, e := range elems {
		switch e := e.(type) {
		case string:
			path = append(path, e)
		case int:
			path = append(path, e)
		case float64:
			path = append(path, int(e))
		default:
			return nil, false
		}
	}
	return path, true
}

func pathExists(doc any, path []any) bool {
	cur := doc
	for _, e := range path {
		switch c := cur.(type) {
		case map[string]any:
			k, ok := e.(string)
			if !ok {
				return false
			}
			cur, ok = c[k]
			if !ok {
				return false
			}
		case []any:
			i, ok := e.(int)
			if !ok || i < 0 || i >= len(c) {
				return false
			}
			cur = c[i]
		default:
			return false
		}
	}
	return true
}

func (np *jqNormalizerPatch) Apply(data []byte) ([]byte, error) {
	dataJSON := make(map[string]any)
	err := json.Unmarshal(data, &dataJSON)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), np.jqExecutionTimeout)
	defer cancel()

	iter := np.code.RunWithContext(ctx, dataJSON)
	first, ok := iter.Next()
	if !ok {
		return nil, errors.New("JQ patch did not return any data")
	}
	if err, ok = first.(error); ok {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("JQ patch execution timed out (%v)", np.jqExecutionTimeout.String())
		}
		return nil, fmt.Errorf("JQ patch returned error: %w", err)
	}
	_, ok = iter.Next()
	if ok {
		return nil, errors.New("JQ patch returned multiple objects")
	}

	patchedData, err := json.Marshal(first)
	if err != nil {
		return nil, err
	}
	return patchedData, err
}

type ignoreNormalizer struct {
	patches []normalizerPatch
}

type IgnoreNormalizerOpts struct {
	JQExecutionTimeout time.Duration
}

func (opts *IgnoreNormalizerOpts) getJQExecutionTimeout() time.Duration {
	if opts == nil || opts.JQExecutionTimeout == 0 {
		return DefaultJQExecutionTimeout
	}
	return opts.JQExecutionTimeout
}

// IgnoreNormalizer is a diff.Normalizer built from ignoreDifferences rules that
// can also report which paths it removes from an object.
type IgnoreNormalizer interface {
	diff.Normalizer
	// IgnoredPaths returns the paths the ignore rules remove from un, as
	// sequences of map keys (string) and list indexes (int) into un.Object.
	// All paths are resolved against un as given, before any removal.
	IgnoredPaths(un *unstructured.Unstructured) ([][]any, error)
}

// NewIgnoreNormalizer creates diff normalizer which removes ignored fields according to given application spec and resource overrides
func NewIgnoreNormalizer(ignore []v1alpha1.ResourceIgnoreDifferences, overrides map[string]v1alpha1.ResourceOverride, opts IgnoreNormalizerOpts) (IgnoreNormalizer, error) {
	for key, override := range overrides {
		group, kind, err := getGroupKindForOverrideKey(key)
		if err != nil {
			log.Warn(err)
		}
		if len(override.IgnoreDifferences.JSONPointers) > 0 || len(override.IgnoreDifferences.JQPathExpressions) > 0 {
			resourceIgnoreDifference := v1alpha1.ResourceIgnoreDifferences{
				Group: group,
				Kind:  kind,
			}
			if len(override.IgnoreDifferences.JSONPointers) > 0 {
				resourceIgnoreDifference.JSONPointers = override.IgnoreDifferences.JSONPointers
			}
			if len(override.IgnoreDifferences.JQPathExpressions) > 0 {
				resourceIgnoreDifference.JQPathExpressions = override.IgnoreDifferences.JQPathExpressions
			}
			ignore = append(ignore, resourceIgnoreDifference)
		}
	}
	patches := make([]normalizerPatch, 0)
	for i := range ignore {
		for _, path := range ignore[i].JSONPointers {
			patchData, err := json.Marshal([]map[string]string{{"op": "remove", "path": path}})
			if err != nil {
				return nil, err
			}
			patch, err := jsonpatch.DecodePatch(patchData)
			if err != nil {
				return nil, err
			}
			patches = append(patches, &jsonPatchNormalizerPatch{
				groupKind: schema.GroupKind{Group: ignore[i].Group, Kind: ignore[i].Kind},
				name:      ignore[i].Name,
				namespace: ignore[i].Namespace,
				patch:     &patch,
				pointer:   path,
			})
		}
		for _, pathExpression := range ignore[i].JQPathExpressions {
			jqDeletionQuery, err := gojq.Parse(fmt.Sprintf("del(%s)", pathExpression))
			if err != nil {
				return nil, err
			}
			jqDeletionCode, err := gojq.Compile(jqDeletionQuery)
			if err != nil {
				return nil, err
			}
			patches = append(patches, &jqNormalizerPatch{
				groupKind:          schema.GroupKind{Group: ignore[i].Group, Kind: ignore[i].Kind},
				name:               ignore[i].Name,
				namespace:          ignore[i].Namespace,
				code:               jqDeletionCode,
				pathExpression:     pathExpression,
				jqExecutionTimeout: opts.getJQExecutionTimeout(),
			})
		}
	}
	return &ignoreNormalizer{patches: patches}, nil
}

// matchedPatches returns the patches whose group/kind/name/namespace selectors match un.
func (n *ignoreNormalizer) matchedPatches(un *unstructured.Unstructured) []normalizerPatch {
	matched := make([]normalizerPatch, 0)
	for _, patch := range n.patches {
		groupKind := un.GroupVersionKind().GroupKind()

		if glob.Match(patch.GetGroupKind().Group, groupKind.Group) &&
			glob.Match(patch.GetGroupKind().Kind, groupKind.Kind) &&
			(patch.GetName() == "" || patch.GetName() == un.GetName()) &&
			(patch.GetNamespace() == "" || patch.GetNamespace() == un.GetNamespace()) {
			matched = append(matched, patch)
		}
	}
	return matched
}

// IgnoredPaths returns the paths the matching ignore rules remove from un.
func (n *ignoreNormalizer) IgnoredPaths(un *unstructured.Unstructured) ([][]any, error) {
	if un == nil {
		return nil, errors.New("invalid argument: unstructured is nil")
	}
	matched := n.matchedPatches(un)
	if len(matched) == 0 {
		return nil, nil
	}
	docData, err := json.Marshal(un)
	if err != nil {
		return nil, err
	}
	var doc any
	if err := json.Unmarshal(docData, &doc); err != nil {
		return nil, err
	}
	var out [][]any
	for _, patch := range matched {
		paths, err := patch.Paths(doc)
		if err != nil {
			if shouldLogError(err) {
				gvk := un.GroupVersionKind()
				log.WithFields(log.Fields{
					"group":     gvk.Group,
					"kind":      gvk.Kind,
					"name":      un.GetName(),
					"namespace": un.GetNamespace(),
				}).Debugf("Failed to resolve ignored paths: %v", err)
			}
			continue
		}
		out = append(out, paths...)
	}
	return out, nil
}

// Normalize removes fields from supplied resource using json paths from matching items of specified resources ignored differences list
func (n *ignoreNormalizer) Normalize(un *unstructured.Unstructured) error {
	if un == nil {
		return errors.New("invalid argument: unstructured is nil")
	}
	matched := n.matchedPatches(un)
	if len(matched) == 0 {
		return nil
	}

	docData, err := json.Marshal(un)
	if err != nil {
		return err
	}

	for _, patch := range matched {
		patchedDocData, err := patch.Apply(docData)
		if err != nil {
			if shouldLogError(err) {
				gvk := un.GroupVersionKind()
				log.WithFields(log.Fields{
					"group":     gvk.Group,
					"kind":      gvk.Kind,
					"name":      un.GetName(),
					"namespace": un.GetNamespace(),
				}).Debugf("Failed to apply normalization patch: %v", err)
			}
			continue
		}
		docData = patchedDocData
	}

	err = json.Unmarshal(docData, un)
	if err != nil {
		return err
	}
	return nil
}

func shouldLogError(e error) bool {
	if strings.Contains(e.Error(), "Unable to remove nonexistent key") {
		return false
	}
	if strings.Contains(e.Error(), "remove operation does not apply: doc is missing path") {
		return false
	}
	return true
}
