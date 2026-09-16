package kubemeta

import (
	"testing"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube/kubemeta/internal/benchfixtures"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKube(t *testing.T) {
	k, err := NewKubeJson([]byte(benchfixtures.Guestbook))
	require.NoError(t, err)

	assert.Equal(t, "argoproj.io/v1alpha1", k.GetAPIVersion())
	assert.Equal(t, "Application", k.GetKind())
	assert.Equal(t, "argocd", k.GetNamespace())
	assert.Equal(t, "guestbook", k.GetName())
	assert.False(t, k.IsEmpty())

	gvk := k.GroupVersionKind()
	assert.Equal(t, "argoproj.io", gvk.Group)
	assert.Equal(t, "v1alpha1", gvk.Version)
	assert.Equal(t, "Application", gvk.Kind)

	rk := GetResourceKey(k)
	assert.Equal(t, "argoproj.io", rk.Group)
	assert.Equal(t, "Application", rk.Kind)
	assert.Equal(t, "argocd", rk.Namespace)
	assert.Equal(t, "guestbook", rk.Name)
}

func TestKube_coreGroup(t *testing.T) {
	// apiVersion without a slash => core group ("").
	k, err := NewKubeJson([]byte(`{"apiVersion":"v1","kind":"Secret","metadata":{"name":"s","namespace":"ns"}}`))
	require.NoError(t, err)
	assert.Equal(t, "v1", k.GetAPIVersion())
	assert.Equal(t, "Secret", k.GetKind())
	assert.Empty(t, k.GroupVersionKind().Group)
}

func TestKube_empty(t *testing.T) {
	// Other empty forms are covered by TestKube_emptyEdges.
	k, err := NewKubeJson([]byte("null"))
	require.NoError(t, err)
	assert.True(t, k.IsEmpty())
	assert.Empty(t, k.GetName())
	assert.Empty(t, k.GetKind())
}

func TestKube_invalid(t *testing.T) {
	_, err := NewKubeJson([]byte(`{"kind":`))
	require.Error(t, err)
}

// duplicateNamesInput cannot occur on the call-site paths: every input there is
// json.Marshal output from a Go map.
const duplicateNamesInput = `{"kind":"A","kind":"Application","metadata":{"name":"g"}}`

// Only "null" and whitespace read as absent; a kind-less object is an error, as
// it was when this decoded into unstructured.
func TestKube_emptyEdges(t *testing.T) {
	for _, in := range []string{"", "   ", "null", " null "} {
		k, err := NewKubeJson([]byte(in))
		require.NoErrorf(t, err, "input %q", in)
		assert.Truef(t, k.IsEmpty(), "input %q should be empty", in)
	}
	for _, in := range []string{"{}", "{ }", "[]"} {
		_, err := NewKubeJson([]byte(in))
		require.Errorf(t, err, "input %q", in)
	}
}

// v2 rejects what v1 read as ""; manifests that used to render must not start
// erroring. See the fallback in NewKubeJson.
func TestKube_toleratesWhatV1Tolerated(t *testing.T) {
	// An unquoted number out of a template.
	k, err := NewKubeJson([]byte(`{"apiVersion":"v1","kind":"Secret","metadata":{"name":123,"namespace":"ns"}}`))
	require.NoError(t, err)
	assert.Empty(t, k.GetName())
	assert.Equal(t, "ns", k.GetNamespace())
	assert.Equal(t, kube.NewResourceKey("", "Secret", "ns", ""), GetResourceKey(k))

	// Invalid UTF-8. json.Marshal substitutes U+FFFD too, so this cannot arrive
	// on the real paths; pinned anyway.
	k, err = NewKubeJson([]byte("{\"apiVersion\":\"v1\",\"kind\":\"Secret\",\"metadata\":{\"name\":\"a\xffb\"}}"))
	require.NoError(t, err)
	assert.Equal(t, "a\uFFFDb", k.GetName())
}

// An unresolvable kind errors rather than yielding a zero ResourceKey that
// matches nothing in the cluster cache.
func TestKube_unresolvableKind(t *testing.T) {
	for _, in := range []string{
		`{"apiVersion":"v1","metadata":{"name":"nokind"}}`,
		`{"apiVersion":"a/b/c","kind":"Foo","metadata":{"name":"n"}}`, // too many slashes
	} {
		_, err := NewKubeJson([]byte(in))
		require.Errorf(t, err, "input %q", in)
	}
}

// Last-wins, matching the unmarshal into unstructured this replaces.
func TestKube_duplicateNamesLastWins(t *testing.T) {
	k, err := NewKubeJson([]byte(duplicateNamesInput))
	require.NoError(t, err)
	assert.Equal(t, "Application", k.GetKind())
}
