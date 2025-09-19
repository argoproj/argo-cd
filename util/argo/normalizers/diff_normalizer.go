package normalizers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/argoproj/gitops-engine/pkg/diff"
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
	patch *jsonpatch.Patch
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
}

type jqMultiPathNormalizerPatch struct {
	baseNormalizerPatch
	pathExpression     string
	jqExecutionTimeout time.Duration
}

type jqPathsNormalizerPatch struct {
	baseNormalizerPatch
	pathQueries        []string
	jqExecutionTimeout time.Duration
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

func (np *jqMultiPathNormalizerPatch) Apply(data []byte) ([]byte, error) {
	dataJSON := make(map[string]any)
	err := json.Unmarshal(data, &dataJSON)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), np.jqExecutionTimeout)
	defer cancel()

	// First, evaluate the path expression to get the paths to delete
	pathQuery, err := gojq.Parse(np.pathExpression)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path expression: %w", err)
	}
	pathCode, err := gojq.Compile(pathQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to compile path expression: %w", err)
	}

	// Collect all paths that match the expression
	var pathsToDelete []string
	iter := pathCode.RunWithContext(ctx, dataJSON)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, fmt.Errorf("JQ path evaluation timed out (%v)", np.jqExecutionTimeout.String())
			}
			// If the path expression fails (e.g., field doesn't exist), just continue
			continue
		}
		if pathStr, ok := v.(string); ok {
			pathsToDelete = append(pathsToDelete, pathStr)
		}
	}

	// If no paths to delete, return original data
	if len(pathsToDelete) == 0 {
		return data, nil
	}

	// For annotation-based expressions, we need to handle them specially
	// Check if this is an annotation key selection expression
	if strings.Contains(np.pathExpression, ".metadata.annotations") && strings.Contains(np.pathExpression, "keys[]") {
		// This is selecting annotation keys, so we need to delete those specific annotations
		result := dataJSON
		if metadata, ok := result["metadata"].(map[string]any); ok {
			if annotations, ok := metadata["annotations"].(map[string]any); ok {
				for _, key := range pathsToDelete {
					delete(annotations, key)
				}
				// If annotations is now empty, we can remove it entirely
				if len(annotations) == 0 {
					delete(metadata, "annotations")
				}
			}
		}

		patchedData, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		return patchedData, nil
	}

	// For other types of expressions, try to delete each path individually
	result := dataJSON
	for _, path := range pathsToDelete {
		deletionQuery, parseErr := gojq.Parse(fmt.Sprintf("del(.%s)", path))
		if parseErr != nil {
			continue // Skip invalid paths
		}
		deletionCode, compileErr := gojq.Compile(deletionQuery)
		if compileErr != nil {
			continue // Skip invalid paths
		}
		iter := deletionCode.RunWithContext(ctx, result)
		if v, ok := iter.Next(); ok {
			if _, isErr := v.(error); !isErr {
				result = v.(map[string]any)
			}
		}
	}

	patchedData, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return patchedData, nil
}

func (np *jqPathsNormalizerPatch) Apply(data []byte) ([]byte, error) {
	var dataJSON any
	if err := json.Unmarshal(data, &dataJSON); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Collect all paths from all queries
	var allPaths []any
	ctx, cancel := context.WithTimeout(context.Background(), np.jqExecutionTimeout)
	defer cancel()

	for _, pathQuery := range np.pathQueries {
		query, err := gojq.Parse(pathQuery)
		if err != nil {
			continue // Skip invalid queries
		}
		
		code, err := gojq.Compile(query)
		if err != nil {
			continue // Skip invalid queries
		}

		iter := code.RunWithContext(ctx, dataJSON)
		for {
			v, ok := iter.Next()
			if !ok {
				break
			}
			if err, ok := v.(error); ok {
				if errors.Is(err, context.DeadlineExceeded) {
					return nil, fmt.Errorf("JQ path evaluation timed out (%v)", np.jqExecutionTimeout.String())
				}
				continue // Skip errors
			}
			
			// The query should return an array of path arrays
			if pathArray, ok := v.([]any); ok {
				for _, path := range pathArray {
					if pathArr, ok := path.([]any); ok {
						allPaths = append(allPaths, pathArr)
					}
				}
			}
		}
	}

	if len(allPaths) == 0 {
		return data, nil
	}

	// Convert paths to JSON for embedding in query
	pathsJSON, err := json.Marshal(allPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal paths: %w", err)
	}

	// Create delpaths query with embedded paths
	delpathsQueryStr := fmt.Sprintf("delpaths(%s)", string(pathsJSON))
	delpathsQuery, err := gojq.Parse(delpathsQueryStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse delpaths query: %w", err)
	}

	code, err := gojq.Compile(delpathsQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to compile delpaths query: %w", err)
	}

	iter := code.RunWithContext(ctx, dataJSON)
	v, ok := iter.Next()
	if !ok {
		return data, nil
	}
	
	if err, ok := v.(error); ok {
		return nil, fmt.Errorf("delpaths execution failed: %w", err)
	}

	result, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return result, nil
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

// NewIgnoreNormalizer creates diff normalizer which removes ignored fields according to given application spec and resource overrides
func NewIgnoreNormalizer(ignore []v1alpha1.ResourceIgnoreDifferences, overrides map[string]v1alpha1.ResourceOverride, opts IgnoreNormalizerOpts) (diff.Normalizer, error) {
	for key, override := range overrides {
		group, kind, err := getGroupKindForOverrideKey(key)
		if err != nil {
			log.Warn(err)
		}
		if len(override.IgnoreDifferences.JSONPointers) > 0 || len(override.IgnoreDifferences.JQPathExpressions) > 0 || len(override.IgnoreDifferences.JQPaths) > 0 {
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
			if len(override.IgnoreDifferences.JQPaths) > 0 {
				resourceIgnoreDifference.JQPaths = override.IgnoreDifferences.JQPaths
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
				baseNormalizerPatch: baseNormalizerPatch{
					groupKind: schema.GroupKind{Group: ignore[i].Group, Kind: ignore[i].Kind},
					name:      ignore[i].Name,
					namespace: ignore[i].Namespace,
				},
				patch: &patch,
			})
		}
		for _, pathExpression := range ignore[i].JQPathExpressions {
			// For expressions that select multiple annotation keys, we need special handling
			if strings.Contains(pathExpression, ".metadata.annotations") &&
				strings.Contains(pathExpression, "keys[]") &&
				strings.Contains(pathExpression, "select") {
				// This is likely selecting multiple annotation keys
				patches = append(patches, &jqMultiPathNormalizerPatch{
					baseNormalizerPatch: baseNormalizerPatch{
						groupKind: schema.GroupKind{Group: ignore[i].Group, Kind: ignore[i].Kind},
						name:      ignore[i].Name,
						namespace: ignore[i].Namespace,
					},
					pathExpression:     pathExpression,
					jqExecutionTimeout: opts.getJQExecutionTimeout(),
				})
			} else {
				// Standard single-path deletion
				jqDeletionQuery, err := gojq.Parse(fmt.Sprintf("del(%s)", pathExpression))
				if err != nil {
					return nil, err
				}
				jqDeletionCode, err := gojq.Compile(jqDeletionQuery)
				if err != nil {
					return nil, err
				}
				patches = append(patches, &jqNormalizerPatch{
					baseNormalizerPatch: baseNormalizerPatch{
						groupKind: schema.GroupKind{Group: ignore[i].Group, Kind: ignore[i].Kind},
						name:      ignore[i].Name,
						namespace: ignore[i].Namespace,
					},
					code:               jqDeletionCode,
					jqExecutionTimeout: opts.getJQExecutionTimeout(),
				})
			}
		}
		// Handle new JQPaths field
		if len(ignore[i].JQPaths) > 0 {
			patches = append(patches, &jqPathsNormalizerPatch{
				baseNormalizerPatch: baseNormalizerPatch{
					groupKind: schema.GroupKind{Group: ignore[i].Group, Kind: ignore[i].Kind},
					name:      ignore[i].Name,
					namespace: ignore[i].Namespace,
				},
				pathQueries:        ignore[i].JQPaths,
				jqExecutionTimeout: opts.getJQExecutionTimeout(),
			})
		}
	}
	return &ignoreNormalizer{patches: patches}, nil
}

// Normalize removes fields from supplied resource using json paths from matching items of specified resources ignored differences list
func (n *ignoreNormalizer) Normalize(un *unstructured.Unstructured) error {
	if un == nil {
		return errors.New("invalid argument: unstructured is nil")
	}
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
				log.Debugf("Failed to apply normalization: %v", err)
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
