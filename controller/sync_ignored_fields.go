package controller

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	smdschema "sigs.k8s.io/structured-merge-diff/v6/schema"
	"sigs.k8s.io/structured-merge-diff/v6/typed"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/argo/diff"
	"github.com/argoproj/argo-cd/v3/util/argo/managedfields"
	"github.com/argoproj/argo-cd/v3/util/jqpath"
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

// listKeyDefaults returns the schema defaults of a keyed list's key fields. An
// item that omits a key field has the default, which is how the API server
// identifies it (a Service port without protocol is the TCP port).
func (f fieldSchema) listKeyDefaults(keys []string) map[string]any {
	a, ok := f.item().atom()
	if !ok || a.Map == nil {
		return nil
	}
	defaults := map[string]any{}
	for _, k := range keys {
		if sf, ok := a.FindField(k); ok && sf.Default != nil {
			defaults[k] = sf.Default
		}
	}
	return defaults
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

// secretStringData returns a Secret's stringData map, if it has one.
func secretStringData(un *unstructured.Unstructured) (map[string]any, bool) {
	gvk := un.GroupVersionKind()
	if gvk.Group != "" || gvk.Kind != "Secret" {
		return nil, false
	}
	stringData, found, err := unstructured.NestedMap(un.Object, "stringData")
	if !found || err != nil {
		return nil, false
	}
	return stringData, true
}

// foldSecretStringData returns the manifest as the API server will store it:
// for a Secret with stringData, a copy with stringData folded into data
// (stringData wins on conflict), so ignore rules on data keys see the same
// object the server will. Any other manifest is returned as is.
func foldSecretStringData(un *unstructured.Unstructured) *unstructured.Unstructured {
	stringData, ok := secretStringData(un)
	if !ok {
		return un
	}
	folded := un.DeepCopy()
	data, found, err := unstructured.NestedMap(folded.Object, "data")
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
	if err := unstructured.SetNestedField(folded.Object, data, "data"); err == nil {
		delete(folded.Object, "stringData")
	}
	return folded
}

// stringDataPaths resolves the rules against a Secret manifest that still has
// its stringData and returns, for every path that names a stringData key, the
// same key under data on both sides. The server stores stringData under data,
// so a rule written against either spelling protects the same key.
func stringDataPaths(resolver *diff.IgnoredPathsResolver, target *unstructured.Unstructured) (livePaths, targetPaths []jqpath.Path, err error) {
	stringData, ok := secretStringData(target)
	if !ok {
		return nil, nil, nil
	}
	rawPaths, err := resolver.RulePaths(target)
	if err != nil {
		return nil, nil, err
	}
	for _, p := range rawPaths {
		if len(p) == 0 || p[0] != "stringData" {
			continue
		}
		// A rule on all of stringData names the keys git writes there, not
		// the keys the same manifest writes under data.
		keys := []string{}
		if len(p) > 1 {
			if k, ok := p[1].(string); ok {
				keys = append(keys, k)
			}
		} else {
			keys = slices.Sorted(maps.Keys(stringData))
		}
		for _, k := range keys {
			livePaths = append(livePaths, jqpath.Path{"data", k})
			targetPaths = append(targetPaths, jqpath.Path{"data", k})
		}
	}
	return livePaths, targetPaths, nil
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
func buildApplyTarget(live, target *unstructured.Unstructured, livePaths, targetPaths []jqpath.Path, sch fieldSchema) (*unstructured.Unstructured, error) {
	// Work on JSON-decoded copies so every number is an int64 or a float64,
	// as in an object decoded from the API server.
	result, err := jsonCopy(target)
	if err != nil {
		return nil, fmt.Errorf("failed to copy target %s %s/%s: %w", target.GroupVersionKind(), target.GetNamespace(), target.GetName(), err)
	}
	liveCopy, err := jsonCopy(live)
	if err != nil {
		return nil, fmt.Errorf("failed to copy live %s %s/%s: %w", live.GroupVersionKind(), live.GetNamespace(), live.GetName(), err)
	}
	// Metadata the API server populates is never carried, whether a rule names
	// one of these fields or all of metadata: an apply that sends them is
	// rejected (managedFields) or raced (resourceVersion, uid).
	for _, field := range serverMetadataFields {
		unstructured.RemoveNestedField(liveCopy.Object, "metadata", field)
	}

	if err := checkKeyPaths(slices.Concat(targetPaths, livePaths), sch); err != nil {
		return nil, fmt.Errorf("%s %s/%s: %w", target.GetKind(), target.GetNamespace(), target.GetName(), err)
	}

	// Drop the target's ignored paths. The same path can arrive from several
	// rules (an Application rule and a resource.customizations rule); it is
	// removed once, as jq's delpaths would.
	jqpath.DeleteAll(result.Object, targetPaths)

	// Carry the live values, parents before children and lower indexes first
	// so inserted list items keep their live order.
	slices.SortStableFunc(livePaths, jqpath.Compare)
	livePaths = slices.CompactFunc(livePaths, func(a, b jqpath.Path) bool { return jqpath.Compare(a, b) == 0 })
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
		v, ok := jqpath.Get(liveCopy.Object, p)
		if !ok {
			continue
		}
		if out, ok := carryInto(result.Object, liveCopy.Object, p, v, sch).(map[string]any); ok {
			result.Object = out
		}
	}
	// The resource's identity is the manifest's, whatever the rules say: a rule
	// over apiVersion would otherwise apply the manifest's body under the
	// version the cluster happens to serve the live object at.
	result.SetAPIVersion(target.GetAPIVersion())
	result.SetKind(target.GetKind())
	result.SetName(target.GetName())
	result.SetNamespace(target.GetNamespace())
	return result, nil
}

// checkKeyPaths refuses a path that ends at a merge key: the key is how items
// are matched, so ignoring it leaves nothing to match on.
func checkKeyPaths(paths []jqpath.Path, sch fieldSchema) error {
	for _, p := range paths {
		s := sch
		var itemKeys []string // merge keys of the list whose item the path is in
		for i, e := range p {
			switch e := e.(type) {
			case string:
				if i == len(p)-1 && slices.Contains(itemKeys, e) {
					return fmt.Errorf("ignored path %s names the merge key %q of the list; items cannot be matched without it", jqpath.String(p), e)
				}
				itemKeys = nil
				s = s.child(e)
			case int:
				itemKeys = s.listKeys()
				s = s.item()
			}
		}
	}
	return nil
}

// checkIndexPaths refuses a positional path (a jsonPointers rule) that indexes
// a list with merge keys where the manifest and the cluster have different
// items at that index: the rule would strip a field from one item and carry
// the other's. Paths that select items by content resolve to each side's own
// index and are not checked.
func checkIndexPaths(live, target map[string]any, paths []jqpath.Path, sch fieldSchema) error {
	for _, p := range paths {
		l, g := any(live), any(target)
		s := sch
		for _, e := range p {
			switch e := e.(type) {
			case string:
				lm, _ := l.(map[string]any)
				gm, _ := g.(map[string]any)
				l, g = lm[e], gm[e]
				s = s.child(e)
			case int:
				keys := s.listKeys()
				ll, _ := l.([]any)
				gl, _ := g.([]any)
				if keys != nil && e >= 0 && e < len(ll) && e < len(gl) {
					defaults := s.listKeyDefaults(keys)
					if scalarKeyValues(ll[e], keys, defaults) && scalarKeyValues(gl[e], keys, defaults) &&
						managedfields.FindKeyedItem([]any{gl[e]}, keyValues(ll[e], keys, defaults), defaults) < 0 {
						return fmt.Errorf("ignored path %s indexes a list matched by %s: index %d is %s in the manifest and %s in the cluster; select the item by its key instead",
							jqpath.String(p), strings.Join(keys, ", "), e, describeKeys(gl[e], keys, defaults), describeKeys(ll[e], keys, defaults))
					}
				}
				l, g = nil, nil
				if e >= 0 && e < len(ll) {
					l = ll[e]
				}
				if e >= 0 && e < len(gl) {
					g = gl[e]
				}
				s = s.item()
			}
		}
	}
	return nil
}

// keyValues returns the key fields of a keyed-list item, defaulted.
func keyValues(item any, keys []string, defaults map[string]any) map[string]any {
	im, _ := item.(map[string]any)
	out := make(map[string]any, len(keys))
	for _, k := range keys {
		out[k] = keyValue(im, k, defaults)
	}
	return out
}

func describeKeys(item any, keys []string, defaults map[string]any) string {
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, keyValues(item, keys, defaults)[k]))
	}
	return strings.Join(parts, ",")
}

// restoreTrackingIdentity puts the application's own tracking annotation and
// label back on the apply target. The controller sets them on the manifest
// before the comparison, so a rule over metadata, labels or annotations
// removes them from git and carries the live values instead; a resource last
// synced by another application would then be handed to that application
// instead of taken over (#24150). Only the keys the tracking method uses are
// restored, and only when the manifest carries them.
func restoreTrackingIdentity(applied, target *unstructured.Unstructured, appLabelKey, trackingMethod string) {
	method := v1alpha1.TrackingMethod(trackingMethod)
	if argo.IsOldTrackingMethod(trackingMethod) {
		method = v1alpha1.TrackingMethodLabel
	}
	if method != v1alpha1.TrackingMethodLabel {
		annotations := applied.GetAnnotations()
		for key, val := range target.GetAnnotations() {
			if key != common.AnnotationKeyAppInstance && key != common.AnnotationInstallationID {
				continue
			}
			if annotations == nil {
				annotations = map[string]string{}
			}
			annotations[key] = val
		}
		if len(annotations) > 0 {
			applied.SetAnnotations(annotations)
		}
	}
	if method != v1alpha1.TrackingMethodAnnotation && appLabelKey != "" {
		if val, ok := target.GetLabels()[appLabelKey]; ok {
			labels := applied.GetLabels()
			if labels == nil {
				labels = map[string]string{}
			}
			labels[appLabelKey] = val
			applied.SetLabels(labels)
		}
	}
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
// carryInto is the one piece of this file that no library provides.
// structured-merge-diff can extract and merge typed values by field path, but
// it treats an atomic list as a single leaf, because the API server never
// tracks ownership below one: extracting a path into tolerations returns the
// whole list, and merging it would replace git's items with live's. Ignore
// rules point inside atomic lists all the time (tolerations, args, Ingress
// paths), and the contract is to carry the live-only item next to git's. So
// the walk below works on positions in the concrete documents, and asks the
// structured-merge-diff schema only how each container merges.
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
		carried := carryInto(next, lm[e], p[1:], v, childSch)
		if !exists && isEmptyContainer(carried) {
			// Nothing was carried into the container created above (the
			// path led to an item git does not declare): do not add an
			// empty one.
			return rm
		}
		rm[e] = carried
		return rm
	case int:
		rl, ok := res.([]any)
		ll, lok := liv.([]any)
		if !ok || !lok || e < 0 || e >= len(ll) {
			return res
		}
		liveItem := ll[e]
		keys := sch.listKeys()
		defaults := sch.listKeyDefaults(keys)
		if !scalarKeyValues(liveItem, keys, defaults) {
			// Keys that are not scalars (or not present on the item) cannot
			// identify the item; fall back to position.
			keys = nil
		}
		if last {
			// A whole list item. Skip it if git declares the same item.
			if keys != nil {
				if findByKeys(rl, liveItem, keys, defaults) >= 0 {
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
			pos = findByKeys(rl, liveItem, keys, defaults)
		} else if e < len(rl) {
			pos = e
		}
		if pos < 0 {
			// git does not declare the item, and the rule names a field of it,
			// not the item. A field rule never keeps an item: carrying a
			// skeleton (keyed list) or the whole item (keyless list) would keep
			// a container the user deleted from git alive, and would leave the
			// diff OutOfSync forever, since the diff sees the extra item too.
			// To keep an injected item, ignore the item itself.
			return rl
		}
		if !sameContainerKind(rl[pos], liveItem) {
			return rl
		}
		rl[pos] = carryInto(rl[pos], liveItem, p[1:], v, sch.item())
		return rl
	}
	return res
}

func isEmptyContainer(v any) bool {
	switch c := v.(type) {
	case map[string]any:
		return len(c) == 0
	case []any:
		return len(c) == 0
	}
	return false
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
	return slices.Insert(slices.Clone(l), pos, v)
}

// keyValue returns item's value for key field k, or the schema default when
// the item omits it.
func keyValue(item map[string]any, k string, defaults map[string]any) any {
	if v, ok := item[k]; ok {
		return v
	}
	return defaults[k]
}

// scalarKeyValues reports whether item is a map whose key fields are all
// scalar (present or defaulted), so that they can identify it in a list.
func scalarKeyValues(item any, keys []string, defaults map[string]any) bool {
	if keys == nil {
		return false
	}
	im, ok := item.(map[string]any)
	if !ok {
		return false
	}
	for _, k := range keys {
		switch keyValue(im, k, defaults).(type) {
		case string, bool, int, int64, float64:
		default:
			return false
		}
	}
	return true
}

// findByKeys returns the index of the item in l whose key fields equal those
// of item, each side defaulted; the same lookup managedFieldsManagers paths use.
func findByKeys(l []any, item any, keys []string, defaults map[string]any) int {
	if _, ok := item.(map[string]any); !ok {
		return -1
	}
	return managedfields.FindKeyedItem(l, keyValues(item, keys, defaults), defaults)
}
