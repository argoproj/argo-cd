package cache

import (
	"testing"

	"github.com/argoproj/argo-cd/common"

	"github.com/stretchr/testify/assert"
)

var c = &clusterInfo{cacheSettingsSrc: func() *cacheSettings {
	return &cacheSettings{AppInstanceLabelKey: common.LabelKeyAppInstance}
}}

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
	assert.False(t, parent.isParentOf(nonMatchingNameEndPoint))
}
