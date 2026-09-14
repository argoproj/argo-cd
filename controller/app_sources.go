package controller

import (
	"bytes"
	"encoding/json"
	"maps"
	"reflect"

	jsonpatch "github.com/evanphx/json-patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
)

// preserveSourceOverridesOption is a sync option of the parent app or an annotation option on the child.
const preserveSourceOverridesOption = "PreserveSourceOverrides=true"

// mergeLiveApplicationSources lays each git spec.sources[i] of a child Application over the live one.
// kubectl applies CRDs with a JSON merge patch that replaces lists whole, so unlike spec.source the
// list loses keys git does not declare, such as helm.parameters set by "argocd app set". No
// last-applied baseline: the parent applies the merged object, so it would record carried keys as
// git's own and the next reconcile would delete them.
func mergeLiveApplicationSources(target, live *unstructured.Unstructured) *unstructured.Unstructured {
	if target == nil || live == nil || !isApplication(target) || !isApplication(live) {
		return target
	}
	targetSources, ok, err := unstructured.NestedSlice(target.Object, "spec", "sources")
	if err != nil || !ok || len(targetSources) == 0 {
		return target
	}
	liveSources, ok, err := unstructured.NestedSlice(live.Object, "spec", "sources")
	if err != nil || !ok || len(liveSources) != len(targetSources) || reflect.DeepEqual(liveSources, targetSources) {
		return target
	}

	merged := make([]any, len(targetSources))
	for i, targetSource := range targetSources {
		// A slot only merges when git and the cluster still describe the same source. A reorder in
		// git falls back to the plain target so an override can never land on another source.
		if !sameSource(targetSource, liveSources[i]) {
			merged[i] = targetSource
			continue
		}
		mergedSource, err := mergeJSON(withoutOtherSourceTypes(liveSources[i], targetSource), targetSource)
		if err != nil {
			mergedSource = targetSource
		}
		merged[i] = mergedSource
	}
	res := target.DeepCopy()
	if err := unstructured.SetNestedSlice(res.Object, merged, "spec", "sources"); err != nil {
		return target
	}
	return res
}

func isApplication(obj *unstructured.Unstructured) bool {
	gvk := obj.GroupVersionKind()
	return gvk.Group == application.Group && gvk.Kind == application.ApplicationKind
}

// sourceIdentityKeys leave out targetRevision so a chart or branch bump in git keeps the overrides.
var sourceIdentityKeys = []string{"repoURL", "chart", "path", "ref", "name"}

// sourceTypeKeys are the blocks of which a source may declare at most one.
var sourceTypeKeys = []string{"helm", "kustomize", "directory", "plugin"}

func sameSource(a, b any) bool {
	am, aok := a.(map[string]any)
	bm, bok := b.(map[string]any)
	if !aok || !bok {
		return false
	}
	for _, key := range sourceIdentityKeys {
		av, aIsString := am[key].(string)
		bv, bIsString := bm[key].(string)
		if (am[key] != nil && !aIsString) || (bm[key] != nil && !bIsString) || av != bv {
			return false
		}
	}
	return true
}

// sourceIdentity returns the identity keys joined, or "" when the element carries none of them.
func sourceIdentity(m map[string]any) string {
	var buf bytes.Buffer
	found := false
	for _, key := range sourceIdentityKeys {
		v, _ := m[key].(string)
		if v != "" {
			found = true
		}
		buf.WriteString(v)
		buf.WriteByte(0)
	}
	if !found {
		return ""
	}
	return buf.String()
}

// withoutOtherSourceTypes drops live type blocks git no longer declares when git declares another
// type, so a helm to kustomize switch does not leave two types on the merged source. A source with
// no type block in git keeps the live one: that is the override this option exists for.
func withoutOtherSourceTypes(live, target any) any {
	lm, lok := live.(map[string]any)
	tm, tok := target.(map[string]any)
	if !lok || !tok {
		return live
	}
	targetHasType := false
	for _, key := range sourceTypeKeys {
		if _, ok := tm[key]; ok {
			targetHasType = true
			break
		}
	}
	if !targetHasType {
		return live
	}
	res := maps.Clone(lm)
	for _, key := range sourceTypeKeys {
		if _, ok := tm[key]; !ok {
			delete(res, key)
		}
	}
	return res
}

// mergeJSON applies patch to base as an RFC 7396 merge patch: patch keys win, base-only keys stay.
func mergeJSON(base, patch any) (any, error) {
	baseJSON, err := json.Marshal(base)
	if err != nil {
		return nil, err
	}
	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}
	mergedJSON, err := jsonpatch.MergePatch(baseJSON, patchJSON)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(mergedJSON))
	dec.UseNumber()
	var merged any
	if err := dec.Decode(&merged); err != nil {
		return nil, err
	}
	return toUnstructuredValue(merged), nil
}

// toUnstructuredValue turns json.Number into int64 or float64 so whole numbers match the int64 the
// live object carries.
func toUnstructuredValue(v any) any {
	switch t := v.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i
		}
		f, _ := t.Float64()
		return f
	case map[string]any:
		for k, e := range t {
			t[k] = toUnstructuredValue(e)
		}
	case []any:
		for i, e := range t {
			t[i] = toUnstructuredValue(e)
		}
	}
	return v
}
