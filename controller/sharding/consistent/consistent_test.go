package consistent

import (
	"encoding/binary"
	"fmt"
	"sync"
	"testing"

	"github.com/google/btree"
	
	blake2b "github.com/minio/blake2b-simd"
)

const (
	testNumShards = 3
	testRepFactor = 1000
)

// 旧版非泛型 BTree 实现的包装器
type OldConsistent struct {
	servers           map[uint64]string
	clients           *btree.BTree
	loadMap           map[string]*Host
	totalLoad         int64
	replicationFactor int
	lock              sync.RWMutex
}

type item struct {
	value uint64
}

func (i item) Less(than btree.Item) bool {
	return i.value < than.(item).value
}

func NewOld() *OldConsistent {
	return &OldConsistent{
		servers:           map[uint64]string{},
		clients:           btree.New(2),
		loadMap:           map[string]*Host{},
		replicationFactor: 1000,
	}
}

func (c *OldConsistent) Add(server string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if _, ok := c.loadMap[server]; ok {
		return
	}

	c.loadMap[server] = &Host{Name: server, Load: 0}
	for i := 0; i < c.replicationFactor; i++ {
		h := c.hash(fmt.Sprintf("%s%d", server, i))
		c.servers[h] = server
		c.clients.ReplaceOrInsert(item{h})
	}
}

func (c *OldConsistent) Get(client string) (string, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if c.clients.Len() == 0 {
		return "", ErrNoHosts
	}

	h := c.hash(client)
	var foundItem btree.Item
	c.clients.AscendGreaterOrEqual(item{h}, func(i btree.Item) bool {
		foundItem = i
		return false // stop the iteration
	})

	if foundItem == nil {
		// If no host found, wrap around to the first one.
		foundItem = c.clients.Min()
	}

	host := c.servers[foundItem.(item).value]

	return host, nil
}

func (c *OldConsistent) hash(key string) uint64 {
	out := blake2b.Sum512([]byte(key))
	return binary.LittleEndian.Uint64(out[:])
}

// 基准测试：测试旧版 BTree 的 Add 方法
func BenchmarkOldBTreeAdd(b *testing.B) {
	for i := 0; i < b.N; i++ {
		c := NewOld()
		for j := 0; j < testNumShards; j++ {
			c.Add(fmt.Sprintf("server%d", j))
		}
	}
}

// 基准测试：测试新版 BTreeG 的 Add 方法
func BenchmarkBTreeGAdd(b *testing.B) {
	for i := 0; i < b.N; i++ {
		c := New()
		for j := 0; j < testNumShards; j++ {
			c.Add(fmt.Sprintf("server%d", j))
		}
	}
}

// 基准测试：测试旧版 BTree 的 Get 方法
func BenchmarkOldBTreeGet(b *testing.B) {
	c := NewOld()
	for j := 0; j < testNumShards; j++ {
		c.Add(fmt.Sprintf("server%d", j))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Get(fmt.Sprintf("client%d", i))
	}
}

// 基准测试：测试新版 BTreeG 的 Get 方法
func BenchmarkBTreeGGet(b *testing.B) {
	c := New()
	for j := 0; j < testNumShards; j++ {
		c.Add(fmt.Sprintf("server%d", j))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Get(fmt.Sprintf("client%d", i))
	}
}

// 基准测试：测试旧版 BTree 的 Add 和 Get 组合操作
func BenchmarkOldBTreeAddAndGet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		c := NewOld()
		for j := 0; j < testNumShards; j++ {
			c.Add(fmt.Sprintf("server%d", j))
		}
		for k := 0; k < 10; k++ {
			_, _ = c.Get(fmt.Sprintf("client%d", k))
		}
	}
}

// 基准测试：测试新版 BTreeG 的 Add 和 Get 组合操作
func BenchmarkBTreeGAddAndGet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		c := New()
		for j := 0; j < testNumShards; j++ {
			c.Add(fmt.Sprintf("server%d", j))
		}
		for k := 0; k < 10; k++ {
			_, _ = c.Get(fmt.Sprintf("client%d", k))
		}
	}
}

// 基准测试：测试更大数据集上的性能
func BenchmarkLargeOldBTreeAddAndGet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		c := NewOld()
		for j := 0; j < 100; j++ {
			c.Add(fmt.Sprintf("server%03d", j))
		}
		for k := 0; k < 1000; k++ {
			_, _ = c.Get(fmt.Sprintf("client%04d", k))
		}
	}
}

func BenchmarkLargeBTreeGAddAndGet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		c := New()
		for j := 0; j < 100; j++ {
			c.Add(fmt.Sprintf("server%03d", j))
		}
		for k := 0; k < 1000; k++ {
			_, _ = c.Get(fmt.Sprintf("client%04d", k))
		}
	}
}