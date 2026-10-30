package controller

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sort"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	smdschema "sigs.k8s.io/structured-merge-diff/v6/schema"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
)

// fieldSchema answers, for one field, how server-side apply merges it: how
// list items are identified and whether a map or list is applied as one value.
// It reads the structured-merge-diff schema the API server uses for
// server-side apply, resolved for the cluster the same way the
// managedFieldsManagers ignore resolves it. A zero fieldSchema means the type
// is unknown: lists are then treated as atomic and maps as granular, which is
// also what the API server deduces for an unknown type.
type fieldSchema struct {
	schema *smdschema.Schema
	ref    smdschema.TypeRef
}

func newFieldSchema(pt *typed.ParseableType) fieldSchema {
	if pt == nil || !pt.IsValid() {
		return fieldSchema{}
	}
	return fieldSchema{schema: pt.Schema, ref: pt.TypeRef}
}

func (f fieldSchema) atom() (smdschema.Atom, bool) {
	if f.schema == nil {
		return smdschema.Atom{}, false
	}
	return f.schema.Resolve(f.ref)
}

// child returns the schema of a struct field or map entry.
func (f fieldSchema) child(name string) fieldSchema {
	a, ok := f.atom()
	if !ok || a.Map == nil {
		return fieldSchema{}
	}
	if sf, ok := a.FindField(name); ok {
		return fieldSchema{schema: f.schema, ref: sf.Type}
	}
	return fieldSchema{schema: f.schema, ref: a.Map.ElementType}
}

// item returns the schema of a list element.
func (f fieldSchema) item() fieldSchema {
	a, ok := f.atom()
	if !ok || a.List == nil {
		return fieldSchema{}
	}
	return fieldSchema{schema: f.schema, ref: a.List.ElementType}
}

// listKeys returns the merge keys of a keyed list (x-kubernetes-list-map-keys
// or the legacy patch merge key), nil for every other list.
func (f fieldSchema) listKeys() []string {
	a, ok := f.atom()
	if !ok || a.List == nil || a.List.ElementRelationship != smdschema.Associative {
		return nil
	}
	return a.Keys
}

// atomicList reports whether the API server replaces the whole list on apply.
func (f fieldSchema) atomicList() bool {
	a, ok := f.atom()
	if !ok || a.List == nil {
		return true
	}
	return a.List.ElementRelationship == smdschema.Atomic
}

// atomicMap reports whether the API server replaces the whole map on apply.
func (f fieldSchema) atomicMap() bool {
	a, ok := f.atom()
	return ok && a.Map != nil && a.Map.ElementRelationship == smdschema.Atomic
}

// foldSecretStringData moves stringData into data the way the API server does
// on write, so ignore rules on data keys see the same object the server will.
func foldSecretStringData(un *unstructured.Unstructured) {
	gvk := un.GroupVersionKind()
	if gvk.Group != "" || gvk.Kind != "Secret" {
		return
	}
	stringData, found, err := unstructured.NestedMap(un.Object, "stringData")
	if !found || err != nil {
		return
	}
	data, found, err := unstructured.NestedMap(un.Object, "data")
	if !found || err != nil {
		data = map[string]any{}
	}
	for k, v := range stringData {
		if s, ok := v.(string); ok {
			data[k] = base64.StdEncoding.EncodeToString([]byte(s))
		} else {
			data[k] = v
		}
	}
	if err := unstructured.SetNestedField(un.Object, data, "data"); err == nil {
		delete(un.Object, "stringData")
	}
}

// buildApplyTarget returns the object a RespectIgnoreDifferences sync applies:
// the target without the paths the ignore rules remove from it, plus the live
// values at the paths the ignore rules remove from live. Where git still
// asserts a value, git wins. A container the manifest does not declare is only
// created for a carried value if the API server merges it granularly: an
// atomic map or list git does not declare is left to its other managers,
// because applying a partial one would replace the whole field.
//
// Paths are sequences of map keys (string) and list indexes (int) into
// live.Object and target.Object.
func buildApplyTarget(live, target *unstructured.Unstructured, livePaths, targetPaths [][]any, sch fieldSchema) (*unstructured.Unstructured, error) {
	// Work on JSON-decoded copies so every number is an int64 or a float64,
	// as in an object decoded from the API server.
	result, err := jsonCopy(target)
	if err != nil {
		return nil, fmt.Errorf("failed to copy target %s %s/%s: %w", target.GroupVersionKind(), target.GetNamespace(), target.GetName(), err)
	}
	live, err = jsonCopy(live)
	if err != nil {
		return nil, fmt.Errorf("failed to copy live %s %s/%s: %w", live.GroupVersionKind(), live.GetNamespace(), live.GetName(), err)
	}
	// Metadata the API server populates is never carried, whether a rule names
	// one of these fields or all of metadata: an apply that sends them is
	// rejected (managedFields) or raced (resourceVersion, uid).
	for _, field := range serverMetadataFields {
		unstructured.RemoveNestedField(live.Object, "metadata", field)
	}

	// Drop the target's ignored paths, deepest and highest index first so list
	// removals do not shift the indexes of paths still to be removed.
	sort.SliceStable(targetPaths, func(i, j int) bool { return comparePaths(targetPaths[i], targetPaths[j]) > 0 })
	for _, p := range targetPaths {
		removePath(result.Object, p)
	}

	// Carry the live values, parents before children and lower indexes first
	// so inserted list items keep their live order.
	sort.SliceStable(livePaths, func(i, j int) bool { return comparePaths(livePaths[i], livePaths[j]) < 0 })
	for _, p := range livePaths {
		if len(p) == 0 {
			continue
		}
		// status is owned by the resource's own controller, never by the sync.
		// Carrying it would make the sync field manager a co-owner of status
		// and, for resources without a /status subresource, freeze it.
		if p[0] == "status" {
			continue
		}
		v, ok := getPath(live.Object, p)
		if !ok {
			continue
		}
		if out, ok := carryInto(result.Object, live.Object, p, v, sch).(map[string]any); ok {
			result.Object = out
		}
	}
	return result, nil
}

// serverMetadataFields are the metadata fields the API server owns.
var serverMetadataFields = []string{
	"creationTimestamp",
	"deletionGracePeriodSeconds",
	"deletionTimestamp",
	"generation",
	"managedFields",
	"resourceVersion",
	"selfLink",
	"uid",
}

func jsonCopy(un *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	raw, err := json.Marshal(un.Object)
	if err != nil {
		return nil, err
	}
	out := &unstructured.Unstructured{}
	if _, _, err := unstructured.UnstructuredJSONScheme.Decode(raw, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// carryInto places v, the live value at path p, into res (the corresponding
// container of the apply target) and returns the possibly re-allocated res.
// liv is the live container corresponding to res.
func carryInto(res, liv any, p []any, v any, sch fieldSchema) any {
	if len(p) == 0 {
		return res
	}
	last := len(p) == 1
	switch e := p[0].(type) {
	case string:
		rm, ok := res.(map[string]any)
		lm, lok := liv.(map[string]any)
		if !ok || !lok {
			return res
		}
		if last {
			// git asserts a value here: git wins.
			if _, exists := rm[e]; !exists {
				rm[e] = runtime.DeepCopyJSONValue(v)
			}
			return rm
		}
		childSch := sch.child(e)
		next, exists := rm[e]
		if !exists {
			// git does not declare this container. Create it only if the server
			// merges it granularly; an atomic container git does not declare is
			// left to its other managers.
			switch lm[e].(type) {
			case map[string]any:
				if childSch.atomicMap() {
					return rm
				}
				next = map[string]any{}
			case []any:
				if childSch.atomicList() {
					return rm
				}
				next = []any{}
			default:
				return rm
			}
		}
		if !sameContainerKind(next, lm[e]) {
			// git asserts something of a different shape here: git wins.
			return rm
		}
		rm[e] = carryInto(next, lm[e], p[1:], v, childSch)
		return rm
	case int:
		rl, ok := res.([]any)
		ll, lok := liv.([]any)
		if !ok || !lok || e < 0 || e >= len(ll) {
			return res
		}
		liveItem := ll[e]
		keys := sch.listKeys()
		if !scalarKeyValues(liveItem, keys) {
			// Keys that are not scalars (or not present on the item) cannot
			// identify the item; fall back to position.
			keys = nil
		}
		if last {
			// A whole list item. Skip it if git declares the same item.
			if keys != nil {
				if findByKeys(rl, liveItem, keys) >= 0 {
					return rl
				}
			} else if slices.ContainsFunc(rl, func(x any) bool { return reflect.DeepEqual(x, liveItem) }) {
				return rl
			}
			return insertAt(rl, min(e, len(rl)), runtime.DeepCopyJSONValue(liveItem))
		}
		// A field inside a list item: find the item in git by key, or by
		// position when the list has no keys.
		pos := -1
		if keys != nil {
			pos = findByKeys(rl, liveItem, keys)
		} else if e < len(rl) {
			pos = e
		}
		if pos < 0 {
			liveMap, ok := liveItem.(map[string]any)
			if !ok {
				return rl
			}
			// git does not declare the item: create it with its identity only,
			// the carried field is added below.
			newItem := map[string]any{}
			for _, k := range keys {
				if kv, ok := liveMap[k]; ok {
					newItem[k] = runtime.DeepCopyJSONValue(kv)
				}
			}
			pos = min(e, len(rl))
			rl = insertAt(rl, pos, newItem)
		}
		if !sameContainerKind(rl[pos], liveItem) {
			return rl
		}
		rl[pos] = carryInto(rl[pos], liveItem, p[1:], v, sch.item())
		return rl
	}
	return res
}

func sameContainerKind(a, b any) bool {
	switch a.(type) {
	case map[string]any:
		_, ok := b.(map[string]any)
		return ok
	case []any:
		_, ok := b.([]any)
		return ok
	}
	return false
}

func insertAt(l []any, pos int, v any) []any {
	out := make([]any, 0, len(l)+1)
	out = append(out, l[:pos]...)
	out = append(out, v)
	return append(out, l[pos:]...)
}

// scalarKeyValues reports whether item is a map whose key fields are all
// present and scalar, so that they can identify it in a list.
func scalarKeyValues(item any, keys []string) bool {
	if keys == nil {
		return false
	}
	im, ok := item.(map[string]any)
	if !ok {
		return false
	}
	for _, k := range keys {
		switch im[k].(type) {
		case string, bool, int64, float64:
		default:
			return false
		}
	}
	return true
}

// findByKeys returns the index of the item in l whose key fields equal those of item.
func findByKeys(l []any, item any, keys []string) int {
	im, ok := item.(map[string]any)
	if !ok {
		return -1
	}
	for i, x := range l {
		xm, ok := x.(map[string]any)
		if !ok {
			continue
		}
		match := true
		for _, k := range keys {
			if !reflect.DeepEqual(xm[k], im[k]) {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// getPath returns the value at path p in root.
func getPath(root any, p []any) (any, bool) {
	cur := root
	for _, e := range p {
		switch c := cur.(type) {
		case map[string]any:
			k, ok := e.(string)
			if !ok {
				return nil, false
			}
			cur, ok = c[k]
			if !ok {
				return nil, false
			}
		case []any:
			i, ok := e.(int)
			if !ok || i < 0 || i >= len(c) {
				return nil, false
			}
			cur = c[i]
		default:
			return nil, false
		}
	}
	return cur, true
}

// removePath deletes the value at path p from root, if present.
func removePath(root map[string]any, p []any) {
	if len(p) == 0 {
		return
	}
	parentPath := p[:len(p)-1]
	parent, ok := getPath(root, parentPath)
	if !ok {
		return
	}
	switch c := parent.(type) {
	case map[string]any:
		if k, ok := p[len(p)-1].(string); ok {
			delete(c, k)
		}
	case []any:
		i, ok := p[len(p)-1].(int)
		if !ok || i < 0 || i >= len(c) {
			return
		}
		setPath(root, parentPath, slices.Delete(slices.Clone(c), i, i+1))
	}
}

// setPath replaces the value at path p in root. The path must exist.
func setPath(root map[string]any, p []any, v any) {
	parent, ok := getPath(root, p[:len(p)-1])
	if !ok {
		return
	}
	switch c := parent.(type) {
	case map[string]any:
		if k, ok := p[len(p)-1].(string); ok {
			c[k] = v
		}
	case []any:
		if i, ok := p[len(p)-1].(int); ok && i >= 0 && i < len(c) {
			c[i] = v
		}
	}
}

// comparePaths orders paths so that a parent sorts before its children and
// lower list indexes sort before higher ones.
func comparePaths(a, b []any) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		ai, aIsInt := a[i].(int)
		bi, bIsInt := b[i].(int)
		switch {
		case aIsInt && bIsInt:
			if ai != bi {
				return ai - bi
			}
		case aIsInt:
			return -1
		case bIsInt:
			return 1
		default:
			as, _ := a[i].(string)
			bs, _ := b[i].(string)
			if as != bs {
				if as < bs {
					return -1
				}
				return 1
			}
		}
	}
	return len(a) - len(b)
}
