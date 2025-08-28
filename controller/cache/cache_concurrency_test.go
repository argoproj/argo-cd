package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterTaintManagerConcurrency(t *testing.T) {
	manager := newClusterTaintManager()
	
	const numGoroutines = 10
	const numOperations = 100
	const server = "https://test-server"
	
	var wg sync.WaitGroup
	
	// Concurrently mark taints
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				gvk := fmt.Sprintf("test/v1/Resource%d_%d", id, j)
				manager.markTainted(server, gvk, "ConversionError", "test error")
			}
		}(i)
	}
	
	// Concurrently read taints
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_ = manager.isTainted(server)
				_ = manager.getTaintedGVKs(server)
				_ = manager.getAllTaints()
				time.Sleep(time.Microsecond) // Small delay to increase chance of race
			}
		}()
	}
	
	// Concurrently clear taints
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numOperations/10; i++ {
			time.Sleep(time.Millisecond * 10)
			manager.clearTaints(server)
		}
	}()
	
	wg.Wait()
	
	// Test should complete without race conditions
	// Final state may be empty or have some taints depending on timing
	gvks := manager.getTaintedGVKs(server)
	t.Logf("Final tainted GVKs count: %d", len(gvks))
}

func TestClusterTaintManagerDataIntegrity(t *testing.T) {
	manager := newClusterTaintManager()
	server := "https://test-server"
	
	// Test that concurrent operations maintain data integrity
	const numWorkers = 5
	const numGVKsPerWorker = 20
	
	var wg sync.WaitGroup
	
	// Each worker marks unique GVKs
	for worker := 0; worker < numWorkers; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < numGVKsPerWorker; i++ {
				gvk := fmt.Sprintf("worker%d/v1/Resource%d", workerID, i)
				manager.markTainted(server, gvk, "ConversionError", "test")
			}
		}(worker)
	}
	
	wg.Wait()
	
	// Verify all GVKs were stored correctly
	gvks := manager.getTaintedGVKs(server)
	assert.Equal(t, numWorkers*numGVKsPerWorker, len(gvks), "Should have all GVKs")
	
	// Verify no duplicates
	gvkSet := make(map[string]bool)
	for _, gvk := range gvks {
		assert.False(t, gvkSet[gvk], "Should not have duplicate GVK: %s", gvk)
		gvkSet[gvk] = true
	}
}

func TestClusterTaintManagerIsolation(t *testing.T) {
	manager := newClusterTaintManager()
	
	server1 := "https://server1"
	server2 := "https://server2"
	
	var wg sync.WaitGroup
	
	// Concurrent operations on different servers should not interfere
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			gvk := fmt.Sprintf("server1/v1/Resource%d", i)
			manager.markTainted(server1, gvk, "ConversionError", "test")
		}
	}()
	
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			gvk := fmt.Sprintf("server2/v1/Resource%d", i)
			manager.markTainted(server2, gvk, "NetworkError", "test")
		}
	}()
	
	wg.Wait()
	
	// Verify isolation
	gvks1 := manager.getTaintedGVKs(server1)
	gvks2 := manager.getTaintedGVKs(server2)
	
	assert.Equal(t, 100, len(gvks1), "Server1 should have 100 GVKs")
	assert.Equal(t, 100, len(gvks2), "Server2 should have 100 GVKs")
	
	// Verify no cross-contamination
	for _, gvk := range gvks1 {
		assert.Contains(t, gvk, "server1", "Server1 GVKs should contain 'server1'")
	}
	for _, gvk := range gvks2 {
		assert.Contains(t, gvk, "server2", "Server2 GVKs should contain 'server2'")
	}
	
	// Clear one server shouldn't affect the other
	manager.clearTaints(server1)
	
	assert.False(t, manager.isTainted(server1), "Server1 should not be tainted after clear")
	assert.True(t, manager.isTainted(server2), "Server2 should still be tainted")
	assert.Equal(t, 100, len(manager.getTaintedGVKs(server2)), "Server2 should still have all GVKs")
}

func TestInstanceTaintManagerFunctions(t *testing.T) {
	// Test the instance-based functions via LiveStateCache
	cache := newTestLiveStateCache(t)
	server := "https://instance-test-server"
	
	// Clean state - should be clean already since it's a new instance
	require.False(t, cache.IsClusterTainted(server))
	
	// Mark as tainted
	cache.MarkClusterTainted(server, "test error", "test/v1/Resource", "ConversionError")
	assert.True(t, cache.IsClusterTainted(server))
	
	gvks := cache.GetTaintedGVKs(server)
	assert.Equal(t, 1, len(gvks))
	assert.Contains(t, gvks, "test/v1/Resource")
	
	// Test getAllTaints via the taint manager directly
	allTaints := cache.taintManager.getAllTaints()
	assert.Contains(t, allTaints, server)
	assert.Contains(t, allTaints[server], "test/v1/Resource")
	assert.Equal(t, "ConversionError", allTaints[server]["test/v1/Resource"])
	
	// Clear and verify
	cache.ClearClusterTaints(server)
	assert.False(t, cache.IsClusterTainted(server))
	assert.Empty(t, cache.GetTaintedGVKs(server))
}

func BenchmarkClusterTaintManager(b *testing.B) {
	manager := newClusterTaintManager()
	server := "https://benchmark-server"
	
	b.Run("MarkTainted", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				gvk := fmt.Sprintf("test/v1/Resource%d", i)
				manager.markTainted(server, gvk, "ConversionError", "benchmark")
				i++
			}
		})
	})
	
	// Setup some data for read benchmarks
	for i := 0; i < 1000; i++ {
		gvk := fmt.Sprintf("test/v1/Resource%d", i)
		manager.markTainted(server, gvk, "ConversionError", "setup")
	}
	
	b.Run("IsTainted", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = manager.isTainted(server)
			}
		})
	})
	
	b.Run("GetTaintedGVKs", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = manager.getTaintedGVKs(server)
			}
		})
	})
	
	b.Run("GetAllTaints", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = manager.getAllTaints()
			}
		})
	})
}