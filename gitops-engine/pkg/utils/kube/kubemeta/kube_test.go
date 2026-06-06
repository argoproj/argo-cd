package kubemeta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// guestbook keeps nested objects/arrays/escapes so the "skip everything else"
// path is exercised. Both implementations must extract the same fields.
const guestbook = `{
  "apiVersion": "argoproj.io/v1alpha1",
  "kind": "Application",
  "metadata": {
    "name": "guestbook",
    "namespace": "argocd",
    "finalizers": ["resources-finalizer.argocd.argoproj.io"],
    "labels": {"name": "guestbook"}
  },
  "spec": {
    "project": "default",
    "source": {
      "repoURL": "https://github.com/argoproj/argocd-example-apps.git",
      "path": "guestbook",
      "helm": {
        "values": "ingress:\n  enabled: true\n  tls:\n    - secretName: mydomain-tls\n",
        "parameters": [{"name": "a\\.b/c", "value": "x", "forceString": true}]
      }
    },
    "destination": {"server": "https://kubernetes.default.svc", "namespace": "guestbook"}
  }
}`

func TestKube(t *testing.T) {
	k, err := NewKubeJson([]byte(guestbook))
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
	// "null" is the absent-resource sentinel seen in practice; the other empty
	// forms are covered by TestKube_emptyEdges.
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

// duplicateNamesInput exercises duplicate object keys. This cannot actually
// occur on the call-site data paths — every input there is json.Marshal output
// from a Go map.
const duplicateNamesInput = `{"kind":"A","kind":"Application","metadata":{"name":"g"}}`

// Empty/whitespace/{}/[] input is treated as an absent object rather than an
// error, unlike json.Unmarshal, which rejects them. These cases never occur in
// practice (state is "" or "null"); asserted so the behaviour is pinned.
func TestKube_emptyEdges(t *testing.T) {
	for _, in := range []string{"", "   ", "{}", "[]"} {
		k, err := NewKubeJson([]byte(in))
		require.NoErrorf(t, err, "input %q", in)
		assert.Truef(t, k.IsEmpty(), "input %q should be empty", in)
	}
}

// Duplicate object names take last-wins, matching the json.Unmarshal into
// unstructured this replaces. See the AllowDuplicateNames note in NewKubeJson.
func TestKube_duplicateNamesLastWins(t *testing.T) {
	k, err := NewKubeJson([]byte(duplicateNamesInput))
	require.NoError(t, err)
	assert.Equal(t, "Application", k.GetKind())
}
