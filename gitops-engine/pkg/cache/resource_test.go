package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
)

var cacheTest = NewClusterCache(&rest.Config{})

func TestIsParentOf(t *testing.T) {
	t.Parallel()
	child := cacheTest.newResource(mustToUnstructured(testPod1()))
	parent := cacheTest.newResource(mustToUnstructured(testRS()))
	grandParent := cacheTest.newResource(mustToUnstructured(testDeploy()))

	assert.True(t, parent.isParentOf(child))
	assert.False(t, grandParent.isParentOf(child))
}

func TestIsParentOfSameKindDifferentGroupAndUID(t *testing.T) {
	t.Parallel()
	rs := testRS()
	rs.APIVersion = "somecrd.io/v1"
	rs.SetUID("123")
	child := cacheTest.newResource(mustToUnstructured(testPod1()))
	invalidParent := cacheTest.newResource(mustToUnstructured(rs))

	assert.False(t, invalidParent.isParentOf(child))
}

func TestIsServiceParentOfEndPointWithTheSameName(t *testing.T) {
	t.Parallel()
	nonMatchingNameEndPoint := cacheTest.newResource(strToUnstructured(`
apiVersion: v1
kind: Endpoints
metadata:
  name: not-matching-name
  namespace: default
`))

	matchingNameEndPoint := cacheTest.newResource(strToUnstructured(`
apiVersion: v1
kind: Endpoints
metadata:
  name: helm-guestbook
  namespace: default
`))

	parent := cacheTest.newResource(testService)

	assert.True(t, parent.isParentOf(matchingNameEndPoint))
	assert.Equal(t, parent.Ref.UID, matchingNameEndPoint.OwnerRefs[0].UID)
	assert.False(t, parent.isParentOf(nonMatchingNameEndPoint))
}

func TestIsServiceAccountParentOfSecret(t *testing.T) {
	t.Parallel()
	serviceAccount := cacheTest.newResource(strToUnstructured(`
apiVersion: v1
kind: ServiceAccount
metadata:
  name: default
  namespace: default
  uid: '123'
secrets:
- name: default-token-123
`))
	tokenSecret := cacheTest.newResource(strToUnstructured(`
apiVersion: v1
kind: Secret
metadata:
  annotations:
    kubernetes.io/service-account.name: default
    kubernetes.io/service-account.uid: '123'
  name: default-token-123
  namespace: default
  uid: '345'
type: kubernetes.io/service-account-token
`))

	assert.True(t, serviceAccount.isParentOf(tokenSecret))
}

func TestCompressedManifest_MsgPackWithIntegerValues(t *testing.T) {
	un := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "int-test",
			"namespace": "default",
		},
		"spec": map[string]any{
			"replicas":    int64(3),
			"minReplicas": int64(1),
			"nested": map[string]any{
				"count": int64(42),
			},
		},
	}}

	r := &Resource{}
	require.NoError(t, r.SetManifestWithCodec(un, ManifestStorageMsgPack, ManifestCompressionGZipBestSpeed))
	assert.True(t, r.HasManifest())

	got, err := r.GetManifest()
	require.NoError(t, err)
	require.NotNil(t, got)

	replicas, found, err := unstructured.NestedFloat64(got.Object, "spec", "replicas")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, float64(3), replicas)

	count, found, err := unstructured.NestedFloat64(got.Object, "spec", "nested", "count")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, float64(42), count)
}

func TestCompressedManifest_AllCodecs(t *testing.T) {
	un := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "codec-test",
			"namespace": "default",
			"uid":       "test-uid-123",
		},
		"spec": map[string]any{
			"replicas": float64(3),
			"selector": map[string]any{
				"matchLabels": map[string]any{
					"app": "test",
				},
			},
		},
	}}

	storageTypes := []ManifestStorageType{
		ManifestStorageJSON,
		ManifestStorageJSONIter,
		ManifestStorageMsgPack,
	}

	compressionTypes := []ManifestCompressionType{
		ManifestCompressionGZipBestSpeed,
		ManifestCompressionGZipDefault,
		ManifestCompressionS2Encode,
		ManifestCompressionS2EncodeBetter,
		ManifestCompressionZLib,
		ManifestCompressionNone,
	}

	for _, st := range storageTypes {
		for _, ct := range compressionTypes {
			t.Run(string(st)+"/"+string(ct), func(t *testing.T) {
				r := &Resource{}
				require.NoError(t, r.SetManifestWithCodec(un, st, ct))
				assert.True(t, r.HasManifest())
				assert.Equal(t, st, r.manifestStorage)
				assert.Equal(t, ct, r.manifestCompression)

				got, err := r.GetManifest()
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, "Deployment", got.GetKind())
				assert.Equal(t, "codec-test", got.GetName())
				assert.Equal(t, "default", got.GetNamespace())

				replicas, found, err := unstructured.NestedFloat64(got.Object, "spec", "replicas")
				require.NoError(t, err)
				assert.True(t, found)
				assert.Equal(t, float64(3), replicas)
			})
		}
	}
}
