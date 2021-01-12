package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"

	"github.com/argoproj/gitops-engine/pkg/utils/kube/kubetest"
)

func TestSetSettings(t *testing.T) {
	cache := NewClusterCache(&rest.Config{}, SetKubectl(&kubetest.MockKubectlCmd{}))
	updatedHealth := &noopSettings{}
	updatedFilter := &noopSettings{}
	cache.Invalidate(SetSettings(Settings{ResourceHealthOverride: updatedHealth, ResourcesFilter: updatedFilter}))

	assert.Equal(t, updatedFilter, cache.settings.ResourcesFilter)
	assert.Equal(t, updatedHealth, cache.settings.ResourceHealthOverride)
}

func TestSetConfig(t *testing.T) {
	cache := NewClusterCache(&rest.Config{}, SetKubectl(&kubetest.MockKubectlCmd{}))
	updatedConfig := &rest.Config{Host: "http://newhost"}
	cache.Invalidate(SetConfig(updatedConfig))

	assert.Equal(t, updatedConfig, cache.config)
}

func TestSetNamespaces(t *testing.T) {
	cache := NewClusterCache(&rest.Config{}, SetKubectl(&kubetest.MockKubectlCmd{}), SetNamespaces([]string{"default"}))

	updatedNamespaces := []string{"updated"}
	cache.Invalidate(SetNamespaces(updatedNamespaces))

	assert.ElementsMatch(t, updatedNamespaces, cache.namespaces)
}

func TestSetResyncTimeout(t *testing.T) {
	cache := NewClusterCache(&rest.Config{})
	assert.Equal(t, clusterResyncTimeout, cache.syncStatus.resyncTimeout)

	timeout := 1 * time.Hour
	cache.Invalidate(SetResyncTimeout(timeout))

	assert.Equal(t, timeout, cache.syncStatus.resyncTimeout)
}
