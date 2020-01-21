package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var c = &clusterCache{}

func TestIsParentOf(t *testing.T) {
	child := c.newResource(testPod)
	parent := c.newResource(testRS)
	grandParent := c.newResource(testDeploy)

	assert.True(t, parent.isParentOf(child))
	assert.False(t, grandParent.isParentOf(child))
}

func TestIsParentOfSameKindDifferentGroupAndUID(t *testing.T) {
	rs := testRS.DeepCopy()
	rs.SetAPIVersion("somecrd.io/v1")
	rs.SetUID("123")
	child := c.newResource(testPod)
	invalidParent := c.newResource(rs)

	assert.False(t, invalidParent.isParentOf(child))
}

func TestIsServiceParentOfEndPointWithTheSameName(t *testing.T) {
	nonMatchingNameEndPoint := c.newResource(strToUnstructured(`
apiVersion: v1
kind: Endpoints
metadata:
  name: not-matching-name
  namespace: default
`))

	matchingNameEndPoint := c.newResource(strToUnstructured(`
apiVersion: v1
kind: Endpoints
metadata:
  name: helm-guestbook
  namespace: default
`))

	parent := c.newResource(testService)

	assert.True(t, parent.isParentOf(matchingNameEndPoint))
	assert.Equal(t, parent.Ref.UID, matchingNameEndPoint.OwnerRefs[0].UID)
	assert.False(t, parent.isParentOf(nonMatchingNameEndPoint))
}

func TestIsServiceAccoountParentOfSecret(t *testing.T) {
	serviceAccount := c.newResource(strToUnstructured(`
apiVersion: v1
kind: ServiceAccount
metadata:
  name: default
  namespace: default
  uid: '123'
secrets:
- name: default-token-123
`))
	tokenSecret := c.newResource(strToUnstructured(`
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
