package v1alpha1

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIsDestinationPermitted(t *testing.T) {
	t.Run("allowed", func(t *testing.T) {
		pr := AppProject{
			Spec: AppProjectSpec{
				Destinations: []ApplicationDestination{
					{
						Server: KubernetesInternalAPIServerAddr,
					},
				},
			},
		}

		rs := pr.IsDestinationPermitted(ApplicationDestination{Server: KubernetesInternalAPIServerAddr}, "test", []string{"test"})
		assert.True(t, rs)
	})
	t.Run("rejected", func(t *testing.T) {
		pr := AppProject{
			Spec: AppProjectSpec{
				Destinations: []ApplicationDestination{
					{
						Server: KubernetesInternalAPIServerAddr,
					},
				},
			},
		}

		rs := pr.IsDestinationPermitted(ApplicationDestination{Server: KubernetesInternalAPIServerAddr}, "test", []string{"test2"})
		assert.False(t, rs)
	})
}
