package hostscan

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheReusesValueWithinTTL(t *testing.T) {
	t.Parallel()
	var loads atomic.Int32
	cache := Cache[int]{TTL: time.Hour}
	load := func() (int, error) {
		loads.Add(1)
		return 7, nil
	}

	first, err := cache.Get("cluster-a", load)
	require.NoError(t, err)
	second, err := cache.Get("cluster-a", load)
	require.NoError(t, err)
	assert.Equal(t, 7, first)
	assert.Equal(t, first, second)
	assert.Equal(t, int32(1), loads.Load())

	_, err = cache.Get("cluster-b", load)
	require.NoError(t, err)
	assert.Equal(t, int32(2), loads.Load())
}

func TestCacheRefreshesAfterTTL(t *testing.T) {
	t.Parallel()
	var loads atomic.Int32
	cache := Cache[int]{TTL: 20 * time.Millisecond}
	_, err := cache.Get("cluster", func() (int, error) {
		loads.Add(1)
		return 1, nil
	})
	require.NoError(t, err)
	time.Sleep(40 * time.Millisecond)
	_, err = cache.Get("cluster", func() (int, error) {
		loads.Add(1)
		return 2, nil
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), loads.Load())
}

func TestCacheDoesNotStoreErrors(t *testing.T) {
	t.Parallel()
	var loads atomic.Int32
	cache := Cache[int]{TTL: time.Hour}
	_, err := cache.Get("cluster", func() (int, error) {
		loads.Add(1)
		return 0, errors.New("scan failed")
	})
	require.EqualError(t, err, "scan failed")
	got, err := cache.Get("cluster", func() (int, error) {
		loads.Add(1)
		return 3, nil
	})
	require.NoError(t, err)
	assert.Equal(t, 3, got)
	assert.Equal(t, int32(2), loads.Load())
}

func TestCacheSingleflightsConcurrentLoads(t *testing.T) {
	t.Parallel()
	var loads atomic.Int32
	cache := Cache[int]{TTL: time.Hour}
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once

	const n = 8
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			_, err := cache.Get("cluster", func() (int, error) {
				loads.Add(1)
				once.Do(func() { close(started) })
				<-release
				return 1, nil
			})
			errCh <- err
		}()
	}
	<-started
	close(release)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}
	assert.Equal(t, int32(1), loads.Load())
}
