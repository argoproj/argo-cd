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
	clients           *btree.BTree
	loadMap           map[string]*Host
	totalLoad         int64
	replicationFactor int

	sync.RWMutex
}

type item struct {
	value uint64
}

func (i item) Less(than btree.Item) bool {
	return i.value < than.(item).value
}

func New() *Consistent {
	return &Consistent{
		servers:           map[uint64]string{},
		clients:           btree.New(2),
		loadMap:           map[string]*Host{},
		replicationFactor: 1000,
	}
}

func NewWithReplicationFactor(replicationFactor int) *Consistent {
	return &Consistent{
		servers:           map[uint64]string{},
		clients:           btree.New(2),
		loadMap:           map[string]*Host{},
		replicationFactor: replicationFactor,
	}
}

func (c *Consistent) Add(server string) {
	c.Lock()
	defer c.Unlock()

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

// Get returns the server that owns the given client.
// As described in https://en.wikipedia.org/wiki/Consistent_hashing
// It returns ErrNoHosts if the ring has no servers in it.
func (c *Consistent) Get(client string) (string, error) {
	c.RLock()
	defer c.RUnlock()

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

// GetLeast returns the least loaded host that can serve the key.
// It uses Consistent Hashing With Bounded loads.
// https://research.googleblog.com/2017/04/consistent-hashing-with-bounded-loads.html
// It returns ErrNoHosts if the ring has no hosts in it.
func (c *Consistent) GetLeast(client string) (string, error) {
	c.RLock()
	defer c.RUnlock()

	if c.clients.Len() == 0 {
		return "", ErrNoHosts
	}
	h := c.hash(client)
	for {
		var foundItem btree.Item
		c.clients.AscendGreaterOrEqual(item{h}, func(bItem btree.Item) bool {
			if h != bItem.(item).value {
				foundItem = bItem
				return false // stop the iteration
			}
			return true
		})

		if foundItem == nil {
			// If no host found, wrap around to the first one.
			foundItem = c.clients.Min()
		}
		key := c.clients.Get(foundItem)
		if key != nil {
			host := c.servers[key.(item).value]
			if c.loadOK(host) {
				return host, nil
			}
			h = key.(item).value
		} else {
			return client, nil
		}
	}
}

// Sets the load of `server` to the given `load`
func (c *Consistent) UpdateLoad(server string, load int64) {
	c.Lock()
	defer c.Unlock()

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
	c.Lock()
	defer c.Unlock()

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
	c.Lock()
	defer c.Unlock()

	if _, ok := c.loadMap[server]; !ok {
		return
	}
	atomic.AddInt64(&c.loadMap[server].Load, -1)
	atomic.AddInt64(&c.totalLoad, -1)
}

// Deletes host from the ring
func (c *Consistent) Remove(server string) bool {
	c.Lock()
	defer c.Unlock()

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
	c.RLock()
	defer c.RUnlock()
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

	return float64(bserver.Load)+1 <= avgLoadPerNode
}

func (c *Consistent) delSlice(val uint64) {
	c.clients.Delete(item{val})
}

func (c *Consistent) hash(key string) uint64 {
	out := blake2b.Sum512([]byte(key))
	return binary.LittleEndian.Uint64(out[:])
}
