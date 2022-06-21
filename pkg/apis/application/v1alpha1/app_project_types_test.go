package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	t.Run("allowed-name", func(t *testing.T) {
		pr := AppProject{
			Spec: AppProjectSpec{
				Destinations: []ApplicationDestination{
					{
						Name: "in-cluster",
					},
				},
			},
		}

		rs := pr.IsDestinationPermitted(ApplicationDestination{Name: "in-cluster"}, "test", []string{"test"})
		assert.True(t, rs)
	})
	t.Run("rejected-name", func(t *testing.T) {
		pr := AppProject{
			Spec: AppProjectSpec{
				Destinations: []ApplicationDestination{
					{
						Name: "in-cluster",
					},
				},
			},
		}

		rs := pr.IsDestinationPermitted(ApplicationDestination{Name: "in-cluster"}, "test", []string{"test2"})
		assert.False(t, rs)
	})
}
