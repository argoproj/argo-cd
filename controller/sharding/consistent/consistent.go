// An implementation of Consistent Hashing and
// Consistent Hashing With Bounded Loads.
//
// https://en.wikipedia.org/wiki/Consistent_hashing
//
// https://research.googleblog.com/2017/04/consistent-hashing-with-bounded-loads.html
package consistent

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"

	"github.com/google/btree"

	blake2b "github.com/minio/blake2b-simd"
)

// OptimalExtraCapacityFactor extra factor capacity (1 + ε). The ideal balance
// between keeping the shards uniform while also keeping consistency when
// changing shard numbers.
const OptimalExtraCapacityFactor = 1.25

var ErrNoHosts = errors.New("no hosts added")

type Host struct {
	Name string
	Load int64
}

type Consistent struct {
	servers           map[uint64]string
	clients           *btree.BTreeG[uint64]
	loadMap           map[string]*Host
	totalLoad         int64
	replicationFactor int
	lock              sync.RWMutex
}

func New() *Consistent {
	return &Consistent{
		servers:           map[uint64]string{},
		clients:           btree.NewOrderedG[uint64](2),
		loadMap:           map[string]*Host{},
		replicationFactor: 1000,
	}
}

func NewWithReplicationFactor(replicationFactor int) *Consistent {
	return &Consistent{
		servers:           map[uint64]string{},
		clients:           btree.NewOrderedG[uint64](2),
		loadMap:           map[string]*Host{},
		replicationFactor: replicationFactor,
	}
}

func (c *Consistent) Add(server string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if _, ok := c.loadMap[server]; ok {
		return
	}

	c.loadMap[server] = &Host{Name: server, Load: 0}
	for i := 0; i < c.replicationFactor; i++ {
		h := c.hash(fmt.Sprintf("%s%d", server, i))
		c.servers[h] = server
		c.clients.ReplaceOrInsert(h)
	}
}

// Get returns the server that owns the given client.
// As described in https://en.wikipedia.org/wiki/Consistent_hashing
// It returns ErrNoHosts if the ring has no servers in it.
func (c *Consistent) Get(client string) (string, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if c.clients.Len() == 0 {
		return "", ErrNoHosts
	}

	h := c.hash(client)
	var foundKey uint64
	c.clients.AscendGreaterOrEqual(h, func(i uint64) bool {
		foundKey = i
		return false // stop the iteration
	})

	if foundKey == 0 {
		// If no key found, get the minimum key
		c.clients.Ascend(func(i uint64) bool {
			foundKey = i
			return false // stop the iteration
		})
	}

	host := c.servers[foundKey]

	return host, nil
}

// GetLeast returns the least loaded host that can serve the key.
// It uses Consistent Hashing With Bounded loads.
// https://research.googleblog.com/2017/04/consistent-hashing-with-bounded-loads.html
// It returns ErrNoHosts if the ring has no hosts in it.
func (c *Consistent) GetLeast(client string) (string, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if c.clients.Len() == 0 {
		return "", ErrNoHosts
	}
	h := c.hash(client)
	start := h
	for {
		var foundKey uint64
		c.clients.AscendGreaterOrEqual(h, func(i uint64) bool {
			if h != i {
				foundKey = i
				return false // stop the iteration
			}
			return true
		})

		if foundKey == 0 {
			// If no key found, get the minimum key
			c.clients.Ascend(func(i uint64) bool {
				foundKey = i
				return false // stop the iteration
			})
		}

		// Check if we have looped all the way around
		if foundKey == start {
			break
		}

		host, exists := c.servers[foundKey]
		if !exists {
			return "", ErrNoHosts
		}
		if c.loadOK(host) {
			return host, nil
		}
		// Start searching from the next point on the ring
		h = foundKey + 1
	}
	// If no suitable host is found, return the first one or an error
	host, exists := c.servers[start]
	if !exists {
		return "", ErrNoHosts
	}
	return host, nil
}

// Sets the load of `server` to the given `load`
func (c *Consistent) UpdateLoad(server string, load int64) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if _, ok := c.loadMap[server]; !ok {
		return
	}
	c.totalLoad -= c.loadMap[server].Load
	c.loadMap[server].Load = load
	c.totalLoad += load
}

// Increments the load of host by 1
//
// should only be used with if you obtained a host with GetLeast
func (c *Consistent) Inc(server string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if _, ok := c.loadMap[server]; !ok {
		return
	}
	atomic.AddInt64(&c.loadMap[server].Load, 1)
	atomic.AddInt64(&c.totalLoad, 1)
}

// Decrements the load of host by 1
//
// should only be used with if you obtained a host with GetLeast
func (c *Consistent) Done(server string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if _, ok := c.loadMap[server]; !ok {
		return
	}
	atomic.AddInt64(&c.loadMap[server].Load, -1)
	atomic.AddInt64(&c.totalLoad, -1)
}

// Deletes host from the ring
func (c *Consistent) Remove(server string) bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	for i := 0; i < c.replicationFactor; i++ {
		h := c.hash(fmt.Sprintf("%s%d", server, i))
		delete(c.servers, h)
		c.delSlice(h)
	}
	delete(c.loadMap, server)
	return true
}

// Return the list of servers in the ring
func (c *Consistent) Servers() (servers []string) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	for k := range c.loadMap {
		servers = append(servers, k)
	}
	return servers
}

// Returns the loads of all the hosts
func (c *Consistent) GetLoads() map[string]int64 {
	loads := map[string]int64{}

	for k, v := range c.loadMap {
		loads[k] = v.Load
	}
	return loads
}

// Returns the maximum load of the single host
// which is:
// (total_load/number_of_hosts)*1.25
// total_load = is the total number of active requests served by hosts
// for more info:
// https://research.googleblog.com/2017/04/consistent-hashing-with-bounded-loads.html
func (c *Consistent) MaxLoad() int64 {
	if c.totalLoad == 0 {
		c.totalLoad = 1
	}
	var avgLoadPerNode float64
	avgLoadPerNode = float64(c.totalLoad / int64(len(c.loadMap)))
	if avgLoadPerNode == 0 {
		avgLoadPerNode = 1
	}
	avgLoadPerNode = math.Ceil(avgLoadPerNode * OptimalExtraCapacityFactor)
	return int64(avgLoadPerNode)
}

func (c *Consistent) loadOK(server string) bool {
	// a safety check if someone performed c.Done more than needed
	if c.totalLoad < 0 {
		c.totalLoad = 0
	}

	var avgLoadPerNode float64
	avgLoadPerNode = float64((c.totalLoad + 1) / int64(len(c.loadMap)))
	if avgLoadPerNode == 0 {
		avgLoadPerNode = 1
	}
	avgLoadPerNode = math.Ceil(avgLoadPerNode * 1.25)

	bserver, ok := c.loadMap[server]
	if !ok {
		panic(fmt.Sprintf("given host(%s) not in loadsMap", bserver.Name))
	}

	return float64(bserver.Load) < avgLoadPerNode
}

func (c *Consistent) delSlice(val uint64) {
	c.clients.Delete(val)
}

func (c *Consistent) hash(key string) uint64 {
	out := blake2b.Sum512([]byte(key))
	return binary.LittleEndian.Uint64(out[:])
}
