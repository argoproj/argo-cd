// Package jqpath reads and deletes values in a JSON document (nested
// map[string]any and []any, as decoded from the API server) at jq paths.
//
// A jq path is the representation jq's path() builtin produces and its
// getpath, setpath and delpaths builtins consume: a list whose elements are
// map keys (string) or list indexes (int), for example
// ["spec", "containers", 0, "image"]. It is positional. A rule that selects
// list items by content (`.webhooks[] | select(.name == "a")`) is a query,
// and resolving it against one concrete document is what turns the content
// into an index; the same rule resolves to different indexes in the live
// object and in the manifest, which is why every rule is resolved against
// both. gojq returns exactly this from `[path(<expr>)]`, so the ignore rules
// of an Application (jsonPointers, jqPathExpressions and
// managedFieldsManagers) all resolve to it: a JSON pointer is already such a
// path, and a structured-merge-diff field path (containers[name=web].image)
// becomes one once the keyed item is looked up in the document it refers to.
//
// Why this package exists rather than a dependency:
//
//   - gojq implements getpath and delpaths but exports neither as a Go
//     function; using them means compiling a query and running it per
//     resource, with errors arriving as values. The functions here are the
//     same semantics in direct Go, and TestAgreesWithJq pins that claim by
//     running gojq's builtins on the same inputs.
//   - structured-merge-diff's fieldpath.Path is the other path type in the
//     Kubernetes ecosystem, but its Key elements (containers[name=web]) only
//     mean something next to the document they index, and its TypedValue
//     operations cannot address anything inside an atomic list, because the
//     API server never tracks ownership below one. Ignore rules routinely do
//     (tolerations, args, Ingress paths), so the sync has to work on
//     positions in the concrete document.
//   - A resolved path is what a JSON pointer is, so a pointer library could
//     serve as the primitive. The two in the dependency tree do not fit:
//     go-openapi/jsonpointer gets and sets but cannot delete, and json-patch
//     works on serialized bytes. Neither orders deletions as delpaths does.
//
// Indexes are non-negative, as path() emits them; the negative-index
// convention of getpath is not supported. Numbers in a path are int, as gojq
// produces them.
package jqpath

import (
	"fmt"
	"slices"
	"strings"
)

// Path is a jq path: map keys as string, list indexes as int. It is an alias
// so that the [][]any gojq hands back needs no conversion.
type Path = []any

// Get returns the value at p in root, as getpath does, and whether the path
// exists. A missing key, an index past the end of a list, or an element whose
// type does not match the container (a string into a list) means it does not.
// An explicit null at the path is a value: Get returns (nil, true).
func Get(root any, p Path) (any, bool) {
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

// Delete removes the value at p from root in place, as delpaths does for one
// path. A path that does not exist is a no-op, and so is the empty path: jq
// returns null for it, but a document has no use for that.
func Delete(root map[string]any, p Path) {
	if len(p) == 0 {
		return
	}
	parentPath := p[:len(p)-1]
	parent, ok := Get(root, parentPath)
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
		set(root, parentPath, slices.Delete(slices.Clone(c), i, i+1))
	}
}

// DeleteAll removes every path in paths from root in place, as delpaths does:
// each distinct path once, children before their parents and higher list
// indexes before lower ones, so that deleting one list item does not shift
// the index of another still to be deleted. paths is left in that order.
func DeleteAll(root map[string]any, paths []Path) {
	slices.SortStableFunc(paths, func(a, b Path) int { return Compare(b, a) })
	for _, p := range slices.CompactFunc(paths, func(a, b Path) bool { return Compare(a, b) == 0 }) {
		Delete(root, p)
	}
}

// Compare orders paths as jq's sort orders them: element by element, with
// indexes before keys, indexes numerically, keys by their bytes, and a path
// before any longer path it is a prefix of. Parents therefore sort before
// their children and lower list indexes before higher ones. Elements of any
// other kind (which a well-formed path never has) sort after keys, by their
// printed form, so that only identical paths compare equal.
func Compare(a, b Path) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		ai, aIsInt := a[i].(int)
		bi, bIsInt := b[i].(int)
		as, aIsStr := a[i].(string)
		bs, bIsStr := b[i].(string)
		switch {
		case aIsInt && bIsInt:
			if ai != bi {
				return ai - bi
			}
		case aIsStr && bIsStr:
			if c := strings.Compare(as, bs); c != 0 {
				return c
			}
		default:
			if ra, rb := kindRank(a[i]), kindRank(b[i]); ra != rb {
				return ra - rb
			}
			if c := strings.Compare(fmt.Sprint(a[i]), fmt.Sprint(b[i])); c != 0 {
				return c
			}
		}
	}
	return len(a) - len(b)
}

func kindRank(e any) int {
	switch e.(type) {
	case int:
		return 0
	case string:
		return 1
	default:
		return 2
	}
}

// set replaces the value at an existing path; Delete needs it to store a
// list it has shortened back into the list's parent.
func set(root map[string]any, p Path, v any) {
	parent, ok := Get(root, p[:len(p)-1])
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
