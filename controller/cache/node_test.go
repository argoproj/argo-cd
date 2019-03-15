package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsParentOf(t *testing.T) {
	child := createObjInfo(testPod, "")
	parent := createObjInfo(testRS, "")
	grandParent := createObjInfo(testDeploy, "")

	assert.True(t, parent.isParentOf(child))
	assert.False(t, grandParent.isParentOf(child))
}

func TestIsParentOfSameKindDifferentGroup(t *testing.T) {
	rs := testRS.DeepCopy()
	rs.SetAPIVersion("somecrd.io/v1")
	child := createObjInfo(testPod, "")
	invalidParent := createObjInfo(rs, "")

	assert.False(t, invalidParent.isParentOf(child))
}
