package sharding

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	"github.com/stretchr/testify/assert"
)

func TestGetShardByID_NotEmptyID(t *testing.T) {
	assert.Equal(t, 0, GetShardByID("1", 2))
	assert.Equal(t, 1, GetShardByID("2", 2))
	assert.Equal(t, 0, GetShardByID("3", 2))
	assert.Equal(t, 1, GetShardByID("4", 2))
}

func TestGetShardByID_EmptyID(t *testing.T) {
	shard := GetShardByID("", 10)
	assert.Equal(t, 0, shard)
}

func TestGetClusterFilter(t *testing.T) {
	filter := GetClusterFilter(2, 1)
	assert.False(t, filter(&v1alpha1.Cluster{ID: "1"}))
	assert.True(t, filter(&v1alpha1.Cluster{ID: "2"}))
	assert.False(t, filter(&v1alpha1.Cluster{ID: "3"}))
	assert.True(t, filter(&v1alpha1.Cluster{ID: "4"}))
}
