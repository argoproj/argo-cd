package jqpath

import (
	"encoding/json"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/itchyny/gojq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestGet(t *testing.T) {
	doc := map[string]any{
		"spec": map[string]any{
			"replicas": int64(1),
			"nothing":  nil,
			"items":    []any{"a", "b", "c"},
		},
	}
	v, ok := Get(doc, Path{"spec", "items", 1})
	assert.True(t, ok)
	assert.Equal(t, "b", v)

	v, ok = Get(doc, Path{})
	assert.True(t, ok, "the empty path is the document")
	assert.Equal(t, doc, v)

	v, ok = Get(doc, Path{"spec", "nothing"})
	assert.True(t, ok, "an explicit null is a value")
	assert.Nil(t, v)

	for _, p := range []Path{{"spec", "missing"}, {"spec", "items", 3}, {"spec", "items", -1}, {"spec", "items", "x"}, {"spec", 0}, {"spec", "replicas", "deeper"}} {
		_, ok := Get(doc, p)
		assert.False(t, ok, "%v", p)
	}
}

func TestDelete(t *testing.T) {
	doc := map[string]any{
		"spec": map[string]any{
			"replicas": int64(1),
			"items":    []any{"a", "b", "c"},
		},
	}
	Delete(doc, Path{"spec", "replicas"})
	Delete(doc, Path{"spec", "items", 1})
	Delete(doc, Path{"spec", "missing", "deeper"})
	Delete(doc, Path{"spec", "items", 7})
	Delete(doc, Path{"spec", "items", "x"})
	Delete(doc, Path{})
	assert.Equal(t, map[string]any{"spec": map[string]any{"items": []any{"a", "c"}}}, doc)
}

func TestDeleteAll(t *testing.T) {
	doc := map[string]any{"items": []any{"a", "b", "c", "d"}, "m": map[string]any{"k": 1, "l": 2}}
	// Given in the wrong order and with a duplicate: lower index first would
	// shift the higher one, the duplicate would delete a neighbour.
	paths := []Path{{"items", 0}, {"items", 2}, {"items", 0}, {"m", "k"}, {"m"}}
	DeleteAll(doc, paths)
	assert.Equal(t, map[string]any{"items": []any{"b", "d"}}, doc)
}

func TestCompare(t *testing.T) {
	assert.Negative(t, Compare(Path{"spec"}, Path{"spec", "replicas"}), "parent sorts before child")
	assert.Negative(t, Compare(Path{"spec", "items", 0}, Path{"spec", "items", 1}), "lower index first")
	assert.Negative(t, Compare(Path{"spec", "items", 2}, Path{"spec", "items", 10}), "indexes compare numerically")
	assert.Negative(t, Compare(Path{0}, Path{"a"}), "indexes before keys, as jq orders numbers before strings")
	assert.Zero(t, Compare(Path{"a", 1}, Path{"a", 1}))
	assert.Positive(t, Compare(Path{"b"}, Path{"a"}))
}

// TestAgreesWithJq is the reason this package may exist: on random documents
// and random paths, valid and invalid, Get, DeleteAll and Compare give the
// answers jq's getpath, delpaths and sort give.
func TestAgreesWithJq(t *testing.T) {
	getpath := compile(t, "getpath($p)", "$p")
	delpaths := compile(t, "delpaths($ps)", "$ps")
	sortPaths := compile(t, "$ps | sort", "$ps")

	rng := rand.New(rand.NewPCG(1, 2))
	var found, missing, invalid int // so the corpus cannot quietly go trivial
	for range 2000 {
		doc := randomDoc(rng, 0).(map[string]any)
		paths := make([]Path, 1+rng.IntN(4))
		for j := range paths {
			paths[j] = randomPath(rng, doc)
		}

		for _, p := range paths {
			want, err := run(getpath, doc, p)
			got, ok := Get(doc, p)
			switch {
			case err != nil:
				// jq rejects a key into a list or an index into a map.
				invalid++
				assert.False(t, ok, "getpath(%v) errors with %v, Get found %v", p, err, got)
			case ok:
				found++
				assert.Equal(t, jsonOf(t, want), jsonOf(t, got), "getpath(%v)", p)
			default:
				missing++
				assert.Nil(t, want, "getpath(%v) = %v, Get found nothing", p, want)
			}
		}

		{
			// jq refuses to delete a key from a list or an index from a map,
			// and returns null for the empty path. Delete leaves those paths
			// alone, so compare against delpaths of the paths it acts on.
			valid := slices.DeleteFunc(slices.Clone(paths), func(p Path) bool { _, e := run(getpath, doc, p); return e != nil || len(p) == 0 })
			want, err := run(delpaths, doc, toAny(valid))
			require.NoError(t, err)
			got := runtime.DeepCopyJSON(doc)
			DeleteAll(got, slices.Clone(paths))
			assert.Equal(t, jsonOf(t, want), jsonOf(t, got), "delpaths(%v) on %s", paths, jsonOf(t, doc))
		}

		{
			want, err := run(sortPaths, nil, toAny(paths))
			require.NoError(t, err)
			got := slices.Clone(paths)
			slices.SortStableFunc(got, Compare)
			assert.Equal(t, jsonOf(t, want), jsonOf(t, toAny(got)), "sort(%v)", paths)
		}
	}
	t.Logf("paths: %d existing, %d missing, %d rejected by jq", found, missing, invalid)
	assert.Greater(t, found, 1000)
	assert.Greater(t, missing, 200)
	assert.Greater(t, invalid, 200)
}

func compile(t *testing.T, src string, vars ...string) *gojq.Code {
	t.Helper()
	q, err := gojq.Parse(src)
	require.NoError(t, err)
	code, err := gojq.Compile(q, gojq.WithVariables(vars))
	require.NoError(t, err)
	return code
}

func run(code *gojq.Code, input any, vars ...any) (any, error) {
	v, _ := code.Run(input, vars...).Next()
	if err, ok := v.(error); ok {
		return nil, err
	}
	return v, nil
}

func jsonOf(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

func toAny(paths []Path) []any {
	out := make([]any, len(paths))
	for i, p := range paths {
		out[i] = p
	}
	return out
}

var keys = []string{"a", "b", "c", "spec", "items"}

// randomDoc builds a small JSON document with nested maps and lists, decoded
// as the API server's objects are (numbers as int64). The root is a map.
func randomDoc(rng *rand.Rand, depth int) any {
	if depth > 2 {
		return int64(rng.IntN(100))
	}
	n := rng.IntN(5)
	if depth == 0 {
		n = 3
	}
	switch n {
	case 0:
		return int64(rng.IntN(100))
	case 1:
		return nil
	case 2:
		l := make([]any, rng.IntN(4))
		for i := range l {
			l[i] = randomDoc(rng, depth+1)
		}
		return l
	default:
		m := map[string]any{}
		for _, k := range keys[:rng.IntN(len(keys)+1)] {
			m[k] = randomDoc(rng, depth+1)
		}
		return m
	}
}

// randomPath mostly follows the document, so paths usually exist, and
// sometimes steps off it with a missing key, a past-the-end index or an
// element of the wrong type.
func randomPath(rng *rand.Rand, doc any) Path {
	var p Path
	cur := doc
	for len(p) < 4 {
		switch c := cur.(type) {
		case map[string]any:
			if rng.IntN(6) == 0 {
				return append(p, rng.IntN(3)) // an index into a map
			}
			k := keys[rng.IntN(len(keys))]
			p = append(p, k)
			next, ok := c[k]
			if !ok {
				return p
			}
			cur = next
		case []any:
			if rng.IntN(6) == 0 {
				return append(p, keys[rng.IntN(len(keys))]) // a key into a list
			}
			i := rng.IntN(len(c) + 2) // may be past the end
			p = append(p, i)
			if i >= len(c) {
				return p
			}
			cur = c[i]
		default:
			if rng.IntN(4) == 0 {
				return append(p, keys[0]) // deeper than the document goes
			}
			return p
		}
		if rng.IntN(3) == 0 {
			return p
		}
	}
	return p
}
