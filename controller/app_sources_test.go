package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func appWithSources(t *testing.T, sources string) *unstructured.Unstructured {
	t.Helper()
	manifest := `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: child
  namespace: argocd
spec:
  project: default
  sources:
` + sources
	obj := &unstructured.Unstructured{}
	require.NoError(t, yaml.Unmarshal([]byte(manifest), obj))
	return obj
}

const parentSources = `
  - repoURL: https://charts.example.com
    chart: app
    targetRevision: 1.0.0
    helm:
      valuesObject:
        image:
          tag: v1
  - repoURL: https://git.example.com/manifests.git
    targetRevision: main
    path: manifests
`

const liveSourcesWithOverride = `
  - repoURL: https://charts.example.com
    chart: app
    targetRevision: 1.0.0
    helm:
      valuesObject:
        image:
          tag: v1
      parameters:
        - name: image.tag
          value: v2
  - repoURL: https://git.example.com/manifests.git
    targetRevision: main
    path: manifests
`

func sourcesOf(t *testing.T, obj *unstructured.Unstructured) []any {
	t.Helper()
	sources, ok, err := unstructured.NestedSlice(obj.Object, "spec", "sources")
	require.NoError(t, err)
	require.True(t, ok)
	return sources
}

func helmParamsOf(t *testing.T, obj *unstructured.Unstructured) ([]any, bool) {
	t.Helper()
	src, ok := sourcesOf(t, obj)[0].(map[string]any)
	require.True(t, ok)
	params, found, err := unstructured.NestedSlice(src, "helm", "parameters")
	require.NoError(t, err)
	return params, found
}

func TestMergeLiveApplicationSources(t *testing.T) {
	t.Parallel()

	wantParams := []any{map[string]any{"name": "image.tag", "value": "v2"}}

	t.Run("keeps live-only helm parameters", func(t *testing.T) {
		t.Parallel()
		target := appWithSources(t, parentSources)
		before := target.DeepCopy()
		live := appWithSources(t, liveSourcesWithOverride)

		got := mergeLiveApplicationSources(target, live)

		params, _ := helmParamsOf(t, got)
		assert.Equal(t, wantParams, params)
		assert.Equal(t, before, target, "target must not be mutated")
		tag, _, err := unstructured.NestedString(sourcesOf(t, got)[0].(map[string]any), "helm", "valuesObject", "image", "tag")
		require.NoError(t, err)
		assert.Equal(t, "v1", tag)
	})

	t.Run("target values win over live", func(t *testing.T) {
		t.Parallel()
		target := appWithSources(t, parentSources)
		sources := sourcesOf(t, target)
		require.NoError(t, unstructured.SetNestedField(sources[0].(map[string]any), "v3", "helm", "valuesObject", "image", "tag"))
		require.NoError(t, unstructured.SetNestedSlice(target.Object, sources, "spec", "sources"))
		live := appWithSources(t, liveSourcesWithOverride)

		got := mergeLiveApplicationSources(target, live)

		tag, _, err := unstructured.NestedString(sourcesOf(t, got)[0].(map[string]any), "helm", "valuesObject", "image", "tag")
		require.NoError(t, err)
		assert.Equal(t, "v3", tag)
		params, _ := helmParamsOf(t, got)
		assert.Equal(t, wantParams, params)
	})

	t.Run("git declaring the key wins", func(t *testing.T) {
		t.Parallel()
		target := appWithSources(t, parentSources)
		sources := sourcesOf(t, target)
		require.NoError(t, unstructured.SetNestedSlice(sources[0].(map[string]any), []any{}, "helm", "parameters"))
		require.NoError(t, unstructured.SetNestedSlice(target.Object, sources, "spec", "sources"))
		live := appWithSources(t, liveSourcesWithOverride)

		got := mergeLiveApplicationSources(target, live)

		params, found := helmParamsOf(t, got)
		assert.True(t, found)
		assert.Empty(t, params)
	})

	t.Run("second reconcile after a sync keeps the override", func(t *testing.T) {
		t.Parallel()
		// After the parent applied the merged object the live one equals the merged result.
		target := appWithSources(t, parentSources)
		live := mergeLiveApplicationSources(target, appWithSources(t, liveSourcesWithOverride))

		got := mergeLiveApplicationSources(target, live)

		params, _ := helmParamsOf(t, got)
		assert.Equal(t, wantParams, params)
	})

	t.Run("source count changed uses target as is", func(t *testing.T) {
		t.Parallel()
		target := appWithSources(t, parentSources)
		live := appWithSources(t, liveSourcesWithOverride+`
  - repoURL: https://git.example.com/extra.git
    path: extra
`)

		assert.Same(t, target, mergeLiveApplicationSources(target, live))
	})

	t.Run("identical sources use target as is", func(t *testing.T) {
		t.Parallel()
		target := appWithSources(t, parentSources)
		assert.Same(t, target, mergeLiveApplicationSources(target, appWithSources(t, parentSources)))
	})

	t.Run("reordered sources use target as is", func(t *testing.T) {
		t.Parallel()
		reordered := `
  - repoURL: https://git.example.com/manifests.git
    targetRevision: main
    path: manifests
  - repoURL: https://charts.example.com
    chart: app
    targetRevision: 1.0.0
    helm:
      valuesObject:
        image:
          tag: v1
`
		target := appWithSources(t, reordered)
		got := mergeLiveApplicationSources(target, appWithSources(t, liveSourcesWithOverride))
		assert.Equal(t, target.Object["spec"], got.Object["spec"])
	})

	t.Run("whole numbers stay int64", func(t *testing.T) {
		t.Parallel()
		withReplicas := `
  - repoURL: https://git.example.com/manifests.git
    targetRevision: main
    path: manifests
    kustomize:
      replicas:
        - name: web
          count: 3
`
		target := appWithSources(t, withReplicas)
		live := appWithSources(t, withReplicas+`
    helm:
      parameters: []
`)

		got := mergeLiveApplicationSources(target, live)

		replicas, _, err := unstructured.NestedSlice(sourcesOf(t, got)[0].(map[string]any), "kustomize", "replicas")
		require.NoError(t, err)
		assert.Equal(t, int64(3), replicas[0].(map[string]any)["count"])
	})

	t.Run("ignores non application objects and nil live", func(t *testing.T) {
		t.Parallel()
		target := appWithSources(t, parentSources)
		assert.Same(t, target, mergeLiveApplicationSources(target, nil))

		pod := &unstructured.Unstructured{}
		require.NoError(t, yaml.Unmarshal([]byte("apiVersion: v1\nkind: Pod\nmetadata:\n  name: p\nspec:\n  sources: [{a: b}]\n"), pod))
		assert.Same(t, pod, mergeLiveApplicationSources(pod, pod.DeepCopy()))
	})
}

func TestSameSource(t *testing.T) {
	t.Parallel()
	assert.True(t, sameSource(map[string]any{"repoURL": "r", "path": "p"}, map[string]any{"repoURL": "r", "path": "p", "targetRevision": "v2"}))
	assert.False(t, sameSource(map[string]any{"repoURL": "r", "path": "p"}, map[string]any{"repoURL": "r", "path": "q"}))
	assert.False(t, sameSource(map[string]any{"repoURL": "r", "ref": "values"}, map[string]any{"repoURL": "r", "ref": "other"}))
	assert.False(t, sameSource("not a map", map[string]any{}))
	// Non-string identity values never match and must not panic, even when both sides carry the same type.
	assert.False(t, sameSource(map[string]any{"repoURL": []any{"a"}}, map[string]any{"repoURL": []any{"a"}}))
	assert.False(t, sameSource(map[string]any{"repoURL": []any{"a"}}, map[string]any{"repoURL": []any{"b"}}))
}
