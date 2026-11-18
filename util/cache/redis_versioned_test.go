package cache

import (
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRedisFetchOwnership(t *testing.T) {
	for _, compression := range []RedisCompressionType{RedisCompressionNone, RedisCompressionGZip} {
		t.Run(string(compression), func(t *testing.T) {
			t.Setenv("ARGOCD_REDIS_KEY_PREFIX", "ownership-test:")
			server := miniredis.RunT(t)
			connect := func() *Cache {
				client := redis.NewClient(&redis.Options{Addr: server.Addr(), ContextTimeoutEnabled: true})
				t.Cleanup(func() { require.NoError(t, client.Close()) })
				return NewCache(NewRedisCache(client, time.Minute, compression))
			}
			a, b := connect(), connect()
			type metadata struct {
				Generation string
				Value      int
			}
			bodyKey := "{index}|body"
			reference := strconv.Itoa
			item := func(owner string, value int, fenced bool) VersionedItem {
				return VersionedItem{Key: "{index}|metadata", ReferenceKey: "{index}|reference", ReferenceValue: reference(value), BodyKey: bodyKey, OwnerKey: "{index}|owner", Token: owner, Object: metadata{owner, value}, Expiration: 2 * time.Minute, RequireOwner: fenced}
			}
			begin := func(client *Cache, owner string) {
				ok, err := client.BeginUpdate(t.Context(), "{index}|owner", owner, time.Minute)
				require.NoError(t, err)
				require.True(t, ok)
			}
			publish := func(client *Cache, owner string, value int, want bool) {
				ok, err := client.StoreVersionedItem(t.Context(), item(owner, value, true))
				require.NoError(t, err)
				require.Equal(t, want, ok)
			}
			read := func(want int, generation string) {
				var got metadata
				require.NoError(t, a.GetItem("{index}|metadata", &got))
				require.Equal(t, metadata{generation, want}, got)
			}
			publish(a, "missing", 1, false)
			begin(a, "a")
			publish(a, "a", 1, true)
			read(1, "a")
			publish(a, "a", 99, false)
			begin(a, "old")
			begin(b, "new")
			publish(a, "old", 1, false)
			publish(b, "new", 2, true)
			read(2, "new")
			// Both prior-hit and prior-miss observations lose authority after a newer
			// publication, even if Redis subsequently evicts the entire cache.
			for _, cold := range []bool{false, true} {
				if cold {
					require.NoError(t, a.SetItem("{index}|metadata", 0, &CacheActionOpts{Delete: true}))
				}
				begin(a, "delayed")
				ok, err := b.StoreVersionedItem(t.Context(), item("maintenance", 3, false))
				require.NoError(t, err)
				require.True(t, ok)
				server.FlushAll()
				publish(a, "delayed", 1, false)
				var got metadata
				require.ErrorIs(t, a.GetItem("{index}|metadata", &got), ErrCacheMiss)
			}
			begin(a, "restore")
			publish(a, "restore", 4, true)
			ok, err := a.StoreVersionedBody(t.Context(), "{index}|reference", bodyKey, reference(4), 4, 2*time.Minute)
			require.NoError(t, err)
			require.True(t, ok)
			begin(a, "retained-owner")
			require.NoError(t, a.SetItem(bodyKey, 0, &CacheActionOpts{Delete: true}))
			publish(a, "retained-owner", 4, true)
			read(4, "retained-owner")
			begin(a, "expired")
			server.FastForward(time.Minute + time.Second)
			publish(a, "expired", 5, false)
			begin(a, "evicted")
			require.NoError(t, a.SetItem("{index}|owner", 0, &CacheActionOpts{Delete: true}))
			publish(a, "evicted", 5, false)
			begin(a, "single-use")
			results := make(chan bool, 8)
			failures := make(chan error, 8)
			for range 8 {
				go func() {
					ok, err := a.StoreVersionedItem(t.Context(), item("single-use", 6, true))
					results <- ok
					failures <- err
				}()
			}
			winners := 0
			for range 8 {
				if <-results {
					winners++
				}
				require.NoError(t, <-failures)
			}
			require.Equal(t, 1, winners)
			read(6, "single-use")
			// A delayed body cannot create or overwrite data after metadata moved on.
			ok, err = a.StoreVersionedBody(t.Context(), "{index}|reference", bodyKey, reference(4), 4, time.Minute)
			require.NoError(t, err)
			require.False(t, ok)
			ok, err = a.StoreVersionedBody(t.Context(), "{index}|reference", bodyKey, reference(6), 6, 2*time.Minute)
			require.NoError(t, err)
			require.True(t, ok)
			var body int
			require.NoError(t, a.GetItem(bodyKey, &body))
			require.Equal(t, 6, body)
			for _, key := range server.Keys() {
				require.Contains(t, key, "ownership-test:")
				require.Equal(t, 2*time.Minute, server.TTL(key))
			}
		})
	}
}
