package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/util/settings"
)

var c = &clusterInfo{settings: &settings.ArgoCDSettings{}}

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
