package normalizers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/argoproj/gitops-engine/pkg/diff"
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/itchyny/gojq"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/glob"
)

type NormalizerPatch interface {
	GetGroupKind() schema.GroupKind
	GetNamespace() string
	GetName() string
	// Apply(un *unstructured.Unstructured) (error)
	ApplyDeletion(data []byte) ([]byte, error)
	GetSelectedPath(entity map[string]interface{}) ([]interface{}, error)
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
	deletePatch *jsonpatch.Patch
	path        string
}

func (np *jsonPatchNormalizerPatch) GetSelectedPath(_ map[string]interface{}) ([]interface{}, error) {
	split := strings.Split(np.path, "/")

	var segments []interface{}

	for _, i := range split {
		segments = append(segments, i)
	}

	return segments, nil
}

func (np *jsonPatchNormalizerPatch) ApplyDeletion(data []byte) ([]byte, error) {
	patchedData, err := np.deletePatch.Apply(data)
	if err != nil {
		return nil, err
	}
	return patchedData, nil
}

type jqNormalizerPatch struct {
	baseNormalizerPatch
	deleteCode *gojq.Code
	selectCode *gojq.Code
}

func (np *jqNormalizerPatch) GetSelectedPath(entity map[string]interface{}) ([]interface{}, error) {
	iter := np.selectCode.Run(entity)
	first, ok := iter.Next()
	if !ok {
		return nil, fmt.Errorf("JQ GetSelectedPath did not return any data")
	}
	_, ok = iter.Next()
	if ok {
		return nil, fmt.Errorf("JQ GetSelectedPath returned multiple objects")
	}

	return first.([]interface{}), nil
}

func (np *jqNormalizerPatch) ApplyDeletion(data []byte) ([]byte, error) {
	dataJson := make(map[string]interface{})
	err := json.Unmarshal(data, &dataJson)
	if err != nil {
		return nil, err
	}

	iter := np.deleteCode.Run(dataJson)
	first, ok := iter.Next()
	if !ok {
		return nil, fmt.Errorf("JQ deletePatch did not return any data")
	}
	_, ok = iter.Next()
	if ok {
		return nil, fmt.Errorf("JQ deletePatch returned multiple objects")
	}

	patchedData, err := json.Marshal(first)
	if err != nil {
		return nil, err
	}
	return patchedData, err
}

type IgnoreNormalizer struct {
	patches []NormalizerPatch
}

// NewIgnoreNormalizer creates diff normalizer which removes ignored fields according to given application spec and resource overrides
func NewIgnoreNormalizer(ignore []v1alpha1.ResourceIgnoreDifferences, overrides map[string]v1alpha1.ResourceOverride) (diff.Normalizer, error) {
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
	patches := make([]NormalizerPatch, 0)
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
				deletePatch: &patch,
				path:        path,
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
			jqSelectQuery, err := gojq.Parse(fmt.Sprintf("path(%s)", pathExpression))
			if err != nil {
				return nil, err
			}
			jqSelectCode, err := gojq.Compile(jqSelectQuery)
			if err != nil {
				return nil, err
			}
			patches = append(patches, &jqNormalizerPatch{
				baseNormalizerPatch: baseNormalizerPatch{
					groupKind: schema.GroupKind{Group: ignore[i].Group, Kind: ignore[i].Kind},
					name:      ignore[i].Name,
					namespace: ignore[i].Namespace,
				},
				deleteCode: jqDeletionCode,
				selectCode: jqSelectCode,
			})
		}
	}
	return &IgnoreNormalizer{patches: patches}, nil
}

func (n *IgnoreNormalizer) Patches() []NormalizerPatch {
	return n.patches
}

// Normalize removes fields from supplied resource using json paths from matching items of specified resources ignored differences list
func (n *IgnoreNormalizer) Normalize(un *unstructured.Unstructured) error {
	if un == nil {
		return fmt.Errorf("invalid argument: unstructured is nil")
	}
	matched := getMatchingNormalizers(un, n)
	if len(matched) == 0 {
		return nil
	}

	docData, err := json.Marshal(un)
	if err != nil {
		return err
	}

	for _, patch := range matched {
		patchedDocData, err := patch.ApplyDeletion(docData)
		if err != nil {
			log.Debugf("Failed to apply normalization: %v", err)
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

func (n *IgnoreNormalizer) GetImmutablePaths(un *unstructured.Unstructured) ([][]interface{}, error) {
	if un == nil {
		return nil, fmt.Errorf("invalid argument: unstructured is nil")
	}

	matched := getMatchingNormalizers(un, n)
	if len(matched) == 0 {
		return [][]interface{}{}, nil
	}

	var immutablePaths [][]interface{}

	for _, patch := range matched {
		selectedPath, err := patch.GetSelectedPath(un.Object)
		if err != nil {
			log.Debugf("Failed to determine selected path: %v", err)
			continue
		}
		immutablePaths = append(immutablePaths, selectedPath)
	}

	return immutablePaths, nil
}

func getMatchingNormalizers(un *unstructured.Unstructured, n *IgnoreNormalizer) []NormalizerPatch {
	matched := make([]NormalizerPatch, 0)
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
