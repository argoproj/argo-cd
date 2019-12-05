package cache

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetServerVersion(t *testing.T) {
	now := time.Now()
	cache := &liveStateCache{
		lock: &sync.Mutex{},
		clusters: map[string]*clusterInfo{
			"http://localhost": {
				syncTime:      &now,
				syncLock:      &sync.Mutex{},
				lock:          &sync.Mutex{},
				serverVersion: "123",
			},
		}}

	version, err := cache.GetServerVersion("http://localhost")
	assert.NoError(t, err)
	assert.Equal(t, "123", version)
}
