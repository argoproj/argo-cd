package sharding

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
)

func Test_hasShardingUpdates(t *testing.T) {
	tests := []struct {
		name     string
		old      *v1alpha1.Cluster
		new      *v1alpha1.Cluster
		expected bool
	}{
		{
			name:     "no cluster",
			old:      nil,
			new:      nil,
			expected: false,
		},
		{
			name:     "add cluster",
			old:      nil,
			new:      &v1alpha1.Cluster{Shard: nil},
			expected: true,
		},
		{
			name:     "remove cluster",
			old:      &v1alpha1.Cluster{Shard: nil},
			new:      nil,
			expected: true,
		},
		{
			name:     "no shard",
			old:      &v1alpha1.Cluster{Shard: nil},
			new:      &v1alpha1.Cluster{Shard: nil},
			expected: false,
		},
		{
			name:     "new shard",
			old:      &v1alpha1.Cluster{Shard: nil},
			new:      &v1alpha1.Cluster{Shard: pointer.Int64Ptr(int64(5))},
			expected: true,
		},
		{
			name:     "old shard",
			old:      &v1alpha1.Cluster{Shard: pointer.Int64Ptr(int64(5))},
			new:      &v1alpha1.Cluster{Shard: nil},
			expected: true,
		},
		{
			name:     "same shard",
			old:      &v1alpha1.Cluster{Shard: pointer.Int64Ptr(int64(5))},
			new:      &v1alpha1.Cluster{Shard: pointer.Int64Ptr(int64(5))},
			expected: false,
		},
		{
			name:     "changed shard",
			old:      &v1alpha1.Cluster{Shard: pointer.Int64Ptr(int64(1))},
			new:      &v1alpha1.Cluster{Shard: pointer.Int64Ptr(int64(5))},
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := hasShardingUpdates(tt.old, tt.new)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
