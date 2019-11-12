package cache

import (
	"testing"

	"github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/engine/util/lua"

	"github.com/argoproj/argo-cd/engine/common"

	"github.com/stretchr/testify/assert"
)

var c = &clusterInfo{
	cacheSettingsSrc: func() *cacheSettings {
		return &cacheSettings{AppInstanceLabelKey: common.LabelKeyAppInstance}
	},
	luaVMFactory: func(overrides map[string]v1alpha1.ResourceOverride) *lua.VM {
		return &lua.VM{
			ResourceOverrides: overrides,
		}
	},
}

func TestIsParentOf(t *testing.T) {
	child := c.createObjInfo(testPod, "")
	parent := c.createObjInfo(testRS, "")
	grandParent := c.createObjInfo(testDeploy, "")

	assert.True(t, parent.isParentOf(child))
	assert.False(t, grandParent.isParentOf(child))
}

func TestIsParentOfSameKindDifferentGroupAndUID(t *testing.T) {
	rs := testRS.DeepCopy()
	rs.SetAPIVersion("somecrd.io/v1")
	rs.SetUID("123")
	child := c.createObjInfo(testPod, "")
	invalidParent := c.createObjInfo(rs, "")

	assert.False(t, invalidParent.isParentOf(child))
}

func TestIsServiceParentOfEndPointWithTheSameName(t *testing.T) {
	nonMatchingNameEndPoint := c.createObjInfo(strToUnstructured(`
apiVersion: v1
kind: Endpoints
metadata:
  name: not-matching-name
  namespace: default
`), "")

	matchingNameEndPoint := c.createObjInfo(strToUnstructured(`
apiVersion: v1
kind: Endpoints
metadata:
  name: helm-guestbook
  namespace: default
`), "")

	parent := c.createObjInfo(testService, "")

	assert.True(t, parent.isParentOf(matchingNameEndPoint))
	assert.Equal(t, parent.ref.UID, matchingNameEndPoint.ownerRefs[0].UID)
	assert.False(t, parent.isParentOf(nonMatchingNameEndPoint))
}

func TestIsServiceAccoountParentOfSecret(t *testing.T) {
	serviceAccount := c.createObjInfo(strToUnstructured(`
apiVersion: v1
kind: ServiceAccount
metadata:
  name: default
  namespace: default
  uid: '123'
secrets:
- name: default-token-123
`), "")
	tokenSecret := c.createObjInfo(strToUnstructured(`
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
