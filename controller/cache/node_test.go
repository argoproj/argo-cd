package cache

import (
	"context"
	"testing"

	"github.com/argoproj/argo-cd/common"

	"github.com/stretchr/testify/assert"
)

var c = &clusterInfo{cacheSettingsSrc: func() *cacheSettings {
	return &cacheSettings{AppInstanceLabelKey: common.LabelKeyAppInstance}
}}

func TestIsParentOf(t *testing.T) {
	child := c.createObjInfo(context.TODO(),testPod, "")
	parent := c.createObjInfo(context.TODO(),testRS, "")
	grandParent := c.createObjInfo(context.TODO(),testDeploy, "")

	assert.True(t, parent.isParentOf(child))
	assert.False(t, grandParent.isParentOf(child))
}

func TestIsParentOfSameKindDifferentGroupAndUID(t *testing.T) {
	rs := testRS.DeepCopy()
	rs.SetAPIVersion("somecrd.io/v1")
	rs.SetUID("123")
	child := c.createObjInfo(context.TODO(),testPod, "")
	invalidParent := c.createObjInfo(context.TODO(),rs, "")

	assert.False(t, invalidParent.isParentOf(child))
}

func TestIsServiceParentOfEndPointWithTheSameName(t *testing.T) {
	nonMatchingNameEndPoint := c.createObjInfo(context.TODO(),strToUnstructured(`
apiVersion: v1
kind: Endpoints
metadata:
  name: not-matching-name
  namespace: default
`), "")

	matchingNameEndPoint := c.createObjInfo(context.TODO(),strToUnstructured(`
apiVersion: v1
kind: Endpoints
metadata:
  name: helm-guestbook
  namespace: default
`), "")

	parent := c.createObjInfo(context.TODO(),testService, "")

	assert.True(t, parent.isParentOf(matchingNameEndPoint))
	assert.Equal(t, parent.ref.UID, matchingNameEndPoint.ownerRefs[0].UID)
	assert.False(t, parent.isParentOf(nonMatchingNameEndPoint))
}

func TestIsServiceAccoountParentOfSecret(t *testing.T) {
	serviceAccount := c.createObjInfo(context.TODO(),strToUnstructured(`
apiVersion: v1
kind: ServiceAccount
metadata:
  name: default
  namespace: default
  uid: '123'
secrets:
- name: default-token-123
`), "")
	tokenSecret := c.createObjInfo(context.TODO(),strToUnstructured(`
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
`), "")

	assert.True(t, serviceAccount.isParentOf(tokenSecret))
}
