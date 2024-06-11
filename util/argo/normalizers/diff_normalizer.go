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

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/glob"
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

func (np *jqNormalizerPatch) Apply(data []byte) ([]byte, error) {
	dataJson := make(map[string]interface{})
	err := json.Unmarshal(data, &dataJson)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), np.jqExecutionTimeout)
	defer cancel()

	iter := np.code.RunWithContext(ctx, dataJson)
	first, ok := iter.Next()
	if !ok {
		return nil, fmt.Errorf("JQ patch did not return any data")
	}
	if err, ok = first.(error); ok {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("JQ patch execution timed out (%v)", np.jqExecutionTimeout.String())
		}
		return nil, fmt.Errorf("JQ patch returned error: %w", err)
	}
	_, ok = iter.Next()
	if ok {
		return nil, fmt.Errorf("JQ patch returned multiple objects")
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

// NewIgnoreNormalizer creates diff normalizer which removes ignored fields according to given application spec and resource overrides
func NewIgnoreNormalizer(ignore []v1alpha1.ResourceIgnoreDifferences, overrides map[string]v1alpha1.ResourceOverride, opts IgnoreNormalizerOpts) (diff.Normalizer, error) {
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
				baseNormalizerPatch: baseNormalizerPatch{
					groupKind: schema.GroupKind{Group: ignore[i].Group, Kind: ignore[i].Kind},
					name:      ignore[i].Name,
					namespace: ignore[i].Namespace,
				},
				patch: &patch,
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
	return &ignoreNormalizer{patches: patches}, nil
}

// Normalize removes fields from supplied resource using json paths from matching items of specified resources ignored differences list
func (n *ignoreNormalizer) Normalize(un *unstructured.Unstructured) error {
	if un == nil {
		return fmt.Errorf("invalid argument: unstructured is nil")
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
