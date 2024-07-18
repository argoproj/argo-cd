package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
)

var cacheTest = NewClusterCache(&rest.Config{})

func TestIsParentOf(t *testing.T) {
	child := cacheTest.newResource(mustToUnstructured(testPod1()))
	parent := cacheTest.newResource(mustToUnstructured(testRS()))
	grandParent := cacheTest.newResource(mustToUnstructured(testDeploy()))

	assert.True(t, parent.isParentOf(child))
	assert.False(t, grandParent.isParentOf(child))
}

func TestIsParentOfSameKindDifferentGroupAndUID(t *testing.T) {
	rs := testRS()
	rs.APIVersion = "somecrd.io/v1"
	rs.SetUID("123")
	child := cacheTest.newResource(mustToUnstructured(testPod1()))
	invalidParent := cacheTest.newResource(mustToUnstructured(rs))

	assert.False(t, invalidParent.isParentOf(child))
}

func TestIsServiceParentOfEndPointWithTheSameName(t *testing.T) {
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
