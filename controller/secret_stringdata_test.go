package controller

import (
	"testing"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test"
	"github.com/argoproj/argo-cd/v3/util/argo/diff"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
)

const liveSecretYaml = `
apiVersion: v1
kind: Secret
metadata:
  name: creds
  namespace: default
type: Opaque
data:
  ACCESS_KEY: bGl2ZS1hY2Nlc3M=
  SECRET_KEY: bGl2ZS1zZWNyZXQ=
`

const targetSecretYaml = `
apiVersion: v1
kind: Secret
metadata:
  name: creds
  namespace: default
type: Opaque
stringData:
  ACCESS_KEY: rendered-access
  SECRET_KEY: rendered-secret
`

func TestNormalizeTargetResourcesSecretStringData(t *testing.T) {
	setupSecret := func(t *testing.T, liveYaml, targetYaml string) *comparisonResult {
		t.Helper()
		dc, err := diff.NewDiffConfigBuilder().
			WithDiffSettings([]v1alpha1.ResourceIgnoreDifferences{{
				Group: "",
				Kind:  "Secret",
				JQPathExpressions: []string{
					`.data["SECRET_KEY"], .data["ACCESS_KEY"], .stringData["SECRET_KEY"], .stringData["ACCESS_KEY"]`,
				},
			}}, nil, true, normalizers.IgnoreNormalizerOpts{}).
			WithNoCache().
			Build()
		require.NoError(t, err)
		return &comparisonResult{
			reconciliationResult: sync.ReconciliationResult{
				Live:   []*unstructured.Unstructured{test.YamlToUnstructured(liveYaml)},
				Target: []*unstructured.Unstructured{test.YamlToUnstructured(targetYaml)},
			},
			diffConfig: dc,
		}
	}

	t.Run("ignored keys of a stringData Secret keep their live data values", func(t *testing.T) {
		// The rendered Secret only has stringData, so `data` is absent from the
		// original target. The ignore normalizer strips the two keys from live
		// `data` but leaves the (now empty) map behind. Pass 2 of
		// restoreNonIgnoredFields must not mistake the live-copied `data` for a
		// replace-strategy leak and drop it.
		cr := setupSecret(t, liveSecretYaml, targetSecretYaml)

		targets, err := normalizeTargetResources(nil, cr)
		require.NoError(t, err)
		require.Len(t, targets, 1)

		data, ok, err := unstructured.NestedStringMap(targets[0].Object, "data")
		require.NoError(t, err)
		require.True(t, ok, "data map was dropped from the apply target: %v", targets[0].Object)
		assert.Equal(t, "bGl2ZS1zZWNyZXQ=", data["SECRET_KEY"], "ignored key must keep the live value")
		assert.Equal(t, "bGl2ZS1hY2Nlc3M=", data["ACCESS_KEY"], "ignored key must keep the live value")

		// The rendered (non-live) values must not be applied.
		stringData, _, err := unstructured.NestedStringMap(targets[0].Object, "stringData")
		require.NoError(t, err)
		assert.NotContains(t, stringData, "SECRET_KEY")
		assert.NotContains(t, stringData, "ACCESS_KEY")
	})

	t.Run("non-ignored live-only data keys are still not copied", func(t *testing.T) {
		liveWithExtra := liveSecretYaml + "  STALE: c3RhbGU=\n"
		cr := setupSecret(t, liveWithExtra, targetSecretYaml)

		targets, err := normalizeTargetResources(nil, cr)
		require.NoError(t, err)
		require.Len(t, targets, 1)

		data, ok, err := unstructured.NestedStringMap(targets[0].Object, "data")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, "bGl2ZS1zZWNyZXQ=", data["SECRET_KEY"])
		assert.Equal(t, "bGl2ZS1hY2Nlc3M=", data["ACCESS_KEY"])
		assert.NotContains(t, data, "STALE", "live-only key that is not ignored must not leak into the apply target")
	})
}

const livePodWithTolerationsYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: p
  namespace: default
spec:
  containers:
    - name: app
      image: app:1
  tolerations:
    - key: dedicated
      operator: Equal
      value: ignored
      effect: NoSchedule
    - key: node.kubernetes.io/not-ready
      operator: Exists
      effect: NoExecute
`

const livePodWithOneTolerationYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: p
  namespace: default
spec:
  containers:
    - name: app
      image: app:1
  tolerations:
    - key: dedicated
      operator: Equal
      value: ignored
      effect: NoSchedule
`

const targetPodWithoutTolerationsYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: p
  namespace: default
spec:
  containers:
    - name: app
      image: app:1
`

// normalizePodWithIgnoredToleration runs normalizeTargetResources for a Pod
// whose rendered manifest has no `spec.tolerations` while live has some, with
// the first live toleration ignored. Tolerations carry no merge key, so the
// live patch copies the whole list into the target.
func normalizePodWithIgnoredToleration(t *testing.T, liveYaml string) *unstructured.Unstructured {
	t.Helper()
	dc, err := diff.NewDiffConfigBuilder().
		WithDiffSettings([]v1alpha1.ResourceIgnoreDifferences{{
			Group:             "",
			Kind:              "Pod",
			JQPathExpressions: []string{`.spec.tolerations[0]`},
		}}, nil, true, normalizers.IgnoreNormalizerOpts{}).
		WithNoCache().
		Build()
	require.NoError(t, err)

	cr := &comparisonResult{
		reconciliationResult: sync.ReconciliationResult{
			Live:   []*unstructured.Unstructured{test.YamlToUnstructured(liveYaml)},
			Target: []*unstructured.Unstructured{test.YamlToUnstructured(targetPodWithoutTolerationsYaml)},
		},
		diffConfig: dc,
	}

	targets, err := normalizeTargetResources(nil, cr)
	require.NoError(t, err)
	require.Len(t, targets, 1)

	containers, found, err := unstructured.NestedSlice(targets[0].Object, "spec", "containers")
	require.NoError(t, err)
	require.True(t, found)
	assert.Len(t, containers, 1)
	return targets[0]
}

func TestNormalizeTargetResourcesLiveOnlyArray(t *testing.T) {
	t.Run("partially ignored live-only list is dropped", func(t *testing.T) {
		// Live has two tolerations and only the first is ignored. Keeping the
		// list would leak the non-ignored second entry, and non-map values
		// cannot be pruned selectively, so the list is dropped as before.
		target := normalizePodWithIgnoredToleration(t, livePodWithTolerationsYaml)

		_, found, err := unstructured.NestedSlice(target.Object, "spec", "tolerations")
		require.NoError(t, err)
		assert.False(t, found, "live-only list with non-ignored entries must not be copied into the apply target")
	})

	t.Run("wholly ignored live-only list is kept", func(t *testing.T) {
		// Live has a single toleration and it is ignored. The normalizer empties
		// the live list, so everything in the patched list is ignored live state
		// that RespectIgnoreDifferences must carry over.
		target := normalizePodWithIgnoredToleration(t, livePodWithOneTolerationYaml)

		tolerations, found, err := unstructured.NestedSlice(target.Object, "spec", "tolerations")
		require.NoError(t, err)
		require.True(t, found, "wholly ignored live-only list must be kept in the apply target")
		require.Len(t, tolerations, 1)
		assert.Equal(t, "dedicated", tolerations[0].(map[string]any)["key"])
	})
}
