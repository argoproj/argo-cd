package argo

import (
	"encoding/json"
	"strings"

	"gopkg.in/yaml.v2"

	jsonpatch "github.com/evanphx/json-patch"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/diff"
	"github.com/argoproj/argo-cd/util/settings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type normalizerPatch struct {
	groupKind schema.GroupKind
	namespace string
	name      string
	patch     jsonpatch.Patch
}

type normalizer struct {
	patches []normalizerPatch
}

type overrideIgnoreDiff struct {
	Json6902Paths []string `yaml:"json6902Paths"`
}

// NewDiffNormalizer creates diff normalizer which removes ignored fields according to given application spec and resource overrides
func NewDiffNormalizer(ignore []v1alpha1.ResourceIgnoreDifferences, overrides map[string]settings.ResourceOverride) (diff.Normalizer, error) {
	for key, override := range overrides {
		parts := strings.Split(key, "/")
		if len(parts) < 2 {
			continue
		}
		group := parts[0]
		kind := parts[1]
		if override.IgnoreDifferences != "" {
			ignoreSettings := overrideIgnoreDiff{}
			err := yaml.Unmarshal([]byte(override.IgnoreDifferences), &ignoreSettings)
			if err != nil {
				return nil, err
			}

			ignore = append(ignore, v1alpha1.ResourceIgnoreDifferences{
				Group:         group,
				Kind:          kind,
				Json6902Paths: ignoreSettings.Json6902Paths,
			})
		}
	}
	patches := make([]normalizerPatch, len(ignore))
	for i := range ignore {
		ops := make([]map[string]string, 0)
		for _, path := range ignore[i].Json6902Paths {
			ops = append(ops, map[string]string{"op": "remove", "path": path})
		}
		patchData, err := json.Marshal(ops)
		if err != nil {
			return nil, err
		}
		patch, err := jsonpatch.DecodePatch(patchData)
		if err != nil {
			return nil, err
		}
		patches[i] = normalizerPatch{
			groupKind: schema.GroupKind{Group: ignore[i].Group, Kind: ignore[i].Kind},
			name:      ignore[i].Name,
			namespace: ignore[i].Namespace,
			patch:     patch,
		}
	}
	return &normalizer{patches: patches}, nil
}

func (n *normalizer) Normalize(un *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	matched := make([]normalizerPatch, 0)
	for _, patch := range n.patches {
		groupKind := un.GroupVersionKind().GroupKind()
		if groupKind == patch.groupKind &&
			(patch.name == "" || patch.name == un.GetName()) &&
			(patch.namespace == "" || patch.namespace == un.GetNamespace()) {

			matched = append(matched, patch)
		}
	}
	if len(matched) == 0 {
		return un, nil
	}

	docData, err := json.Marshal(un)
	if err != nil {
		return nil, err
	}

	for _, patch := range matched {
		docData, err = patch.patch.Apply(docData)
		if err != nil {
			return nil, err
		}
	}

	err = json.Unmarshal(docData, un)
	if err != nil {
		return nil, err
	}
	return un, nil
}
