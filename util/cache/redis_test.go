package cache

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus"
	promcm "github.com/prometheus/client_model/go"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	redisRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_redis_request_total",
		},
		[]string{"initiator", "command", "failed"},
	)
	redisRequestHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "argocd_redis_request_duration",
			Buckets: []float64{0.1, 0.25, .5, 1, 2},
		},
		[]string{"initiator"},
	)
)

type MockMetricsServer struct {
	redisRequestCounter   *prometheus.CounterVec
	redisRequestHistogram *prometheus.HistogramVec
}

func NewMockMetricsServer() *MockMetricsServer {
	registry := prometheus.NewRegistry()
	registry.MustRegister(redisRequestCounter)
	registry.MustRegister(redisRequestHistogram)
	return &MockMetricsServer{
		redisRequestCounter:   redisRequestCounter,
		redisRequestHistogram: redisRequestHistogram,
	}
}

func (m *MockMetricsServer) IncRedisRequest(command string, failed bool) {
	m.redisRequestCounter.WithLabelValues("mock", command, strconv.FormatBool(failed)).Inc()
}

func (m *MockMetricsServer) ObserveRedisRequestDuration(duration time.Duration) {
	m.redisRequestHistogram.WithLabelValues("mock").Observe(duration.Seconds())
}

func TestRedisSetCache(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer mr.Close()
	assert.NotNil(t, mr)

	t.Run("Successful set", func(t *testing.T) {
		client := NewRedisCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 60*time.Second, RedisCompressionNone)
		err = client.Set(&Item{Key: "foo", Object: "bar"})
		require.NoError(t, err)
	})

	t.Run("Successful get", func(t *testing.T) {
		var res string
		client := NewRedisCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 10*time.Second, RedisCompressionNone)
		err = client.Get("foo", &res)
		require.NoError(t, err)
		assert.Equal(t, "bar", res)
	})

	t.Run("Successful delete", func(t *testing.T) {
		client := NewRedisCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 10*time.Second, RedisCompressionNone)
		err = client.Delete("foo")
		require.NoError(t, err)
	})

	t.Run("Cache miss", func(t *testing.T) {
		var res string
		client := NewRedisCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 10*time.Second, RedisCompressionNone)
		err = client.Get("foo", &res)
		assert.ErrorContains(t, err, "cache: key is missing")
	})
}

func TestRedisSetCacheWithPrefix(t *testing.T) {
	prefix := "argocd-dev:"
	t.Setenv("ARGOCD_REDIS_KEY_PREFIX", prefix)
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer mr.Close()
	assert.NotNil(t, mr)

	t.Run("Successful set", func(t *testing.T) {
		client := NewRedisCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 60*time.Second, RedisCompressionNone)
		err = client.Set(&Item{Key: "foo", Object: "bar"})
		require.NoError(t, err)
		keys := mr.Keys()
		require.Len(t, keys, 1)
		assert.Equal(t, prefix+"foo", keys[0])
	})

	t.Run("Successful get", func(t *testing.T) {
		var res string
		client := NewRedisCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 10*time.Second, RedisCompressionNone)
		err = client.Get("foo", &res)
		require.NoError(t, err)
		assert.Equal(t, "bar", res)
	})

	t.Run("Successful delete", func(t *testing.T) {
		client := NewRedisCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 10*time.Second, RedisCompressionNone)
		err = client.Delete("foo")
		require.NoError(t, err)
	})

	t.Run("Cache miss", func(t *testing.T) {
		var res string
		client := NewRedisCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 10*time.Second, RedisCompressionNone)
		err = client.Get("foo", &res)
		assert.ErrorContains(t, err, "cache: key is missing")
	})
}

func TestRedisSetCacheCompressed(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer mr.Close()
	assert.NotNil(t, mr)

	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	client := NewRedisCache(redisClient, 10*time.Second, RedisCompressionGZip)
	testValue := "my-value"
	require.NoError(t, client.Set(&Item{Key: "my-key", Object: testValue}))

	compressedData, err := redisClient.Get(t.Context(), "my-key.gz").Bytes()
	require.NoError(t, err)

	assert.Greater(t, len(compressedData), len([]byte(testValue)), "compressed data is bigger than uncompressed")

	// trying to unzip compressed data
	gzipReader, err := gzip.NewReader(bytes.NewBuffer(compressedData))
	require.NoError(t, err)
	_, err = io.ReadAll(gzipReader)
	require.NoError(t, err)

	var result string
	require.NoError(t, client.Get("my-key", &result))

	assert.Equal(t, testValue, result)
}

func TestRedisMetrics(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer mr.Close()

	metric := &promcm.Metric{}
	ms := NewMockMetricsServer()
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	faultyRedisClient := redis.NewClient(&redis.Options{Addr: "invalidredishost.invalid:12345"})

	CollectMetrics(redisClient, ms, nil)
	CollectMetrics(faultyRedisClient, ms, nil)

	client := NewRedisCache(redisClient, 60*time.Second, RedisCompressionNone)
	faultyClient := NewRedisCache(faultyRedisClient, 60*time.Second, RedisCompressionNone)
	var res string

	assertCounter := func(command, failed string, expected float64) {
		t.Helper()
		m := &promcm.Metric{}
		c, err := ms.redisRequestCounter.GetMetricWithLabelValues("mock", command, failed)
		require.NoError(t, err)
		require.NoError(t, c.Write(m))
		assert.InEpsilon(t, expected, m.Counter.GetValue(), 0.0001)
	}

	// client successful request
	err = client.Set(&Item{Key: "foo", Object: "bar"})
	require.NoError(t, err)
	err = client.Get("foo", &res)
	require.NoError(t, err)

	// successful commands are recorded per command with failed="false"
	assertCounter("set", "false", 1)
	assertCounter("get", "false", 1)

	// Rename against a missing source key is a benign cache miss: it returns ErrCacheMiss and must
	// NOT be counted as a failed request.
	err = client.Rename("missing-old", "new", 0)
	require.ErrorIs(t, err, ErrCacheMiss)
	assertCounter("rename", "false", 1)

	// faulty client failed request
	err = faultyClient.Get("foo", &res)
	require.Error(t, err)
	assertCounter("get", "true", 1)

	// every client request observes a duration sample (set, get, rename, faulty get)
	o, err := ms.redisRequestHistogram.GetMetricWithLabelValues("mock")
	require.NoError(t, err)
	err = o.(prometheus.Metric).Write(metric)
	require.NoError(t, err)
	assert.Equal(t, 4, int(metric.Histogram.GetSampleCount()))
}

func TestRedisClientsEnableContextDeadlines(t *testing.T) {
	standalone := buildRedisClient("127.0.0.1:6379", "", "", 0, -1, nil)
	t.Cleanup(func() { require.NoError(t, standalone.Close()) })
	require.True(t, standalone.Options().ContextTimeoutEnabled)
	failover := buildFailoverRedisClient("master", "", "", "", "", 0, -1, nil, []string{"127.0.0.1:26379"})
	t.Cleanup(func() { require.NoError(t, failover.Close()) })
	require.True(t, failover.Options().ContextTimeoutEnabled)
}

type readDeadlineHook struct{ deadline time.Time }

func (h *readDeadlineHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h *readDeadlineHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func (h *readDeadlineHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, command redis.Cmder) error {
		if command.Name() == "get" {
			h.deadline, _ = ctx.Deadline()
		}
		return next(ctx, command)
	}
}

func TestRedisContextReadBoundsDeadlineFreeContext(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), ContextTimeoutEnabled: true, ReadTimeout: -1})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	hook := &readDeadlineHook{}
	client.AddHook(hook)
	cache := NewCache(NewRedisCache(client, time.Minute, RedisCompressionNone))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var body []byte
	require.ErrorIs(t, cache.GetItemContext(ctx, "index", &body), ErrCacheMiss)
	require.False(t, hook.deadline.IsZero(), "Redis must receive a real deadline even when socket timeouts are disabled")
	require.Positive(t, time.Until(hook.deadline))
	require.LessOrEqual(t, time.Until(hook.deadline), 5*time.Second)
	before := server.CommandCount()
	cancel()
	require.ErrorIs(t, cache.GetItemContext(ctx, "index", &body), context.Canceled)
	require.Equal(t, before, server.CommandCount(), "an already-cancelled request must not start Redis I/O")
}

func TestRedisContextReadRejectsUnboundedClient(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	cache := NewCache(NewRedisCache(client, time.Minute, RedisCompressionNone))
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	var body []byte
	require.ErrorContains(t, cache.GetItemContext(ctx, "index", &body), "does not enforce context deadlines")
	require.Zero(t, server.CommandCount(), "unsupported deadline enforcement must fail before I/O")
}

func TestRedisContextRejectsDisabledSocketDeadlines(t *testing.T) {
	for _, write := range []bool{false, true} {
		t.Run(strconv.FormatBool(write), func(t *testing.T) {
			server := miniredis.RunT(t)
			options := &redis.Options{Addr: server.Addr(), ContextTimeoutEnabled: true}
			if write {
				options.WriteTimeout = -2
			} else {
				options.ReadTimeout = -2
			}
			client := redis.NewClient(options)
			t.Cleanup(func() { require.NoError(t, client.Close()) })
			cache := NewCache(NewRedisCache(client, time.Minute, RedisCompressionNone))
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			var body []byte
			require.ErrorContains(t, cache.GetItemContext(ctx, "index", &body), "does not enforce context deadlines")
			_, err := cache.BeginUpdate(ctx, "owner", "token", time.Minute)
			require.ErrorContains(t, err, "does not enforce context deadlines")
			_, err = cache.StoreVersionedItem(ctx, VersionedItem{Key: "metadata", ReferenceKey: "reference", ReferenceValue: "digest", BodyKey: "body", OwnerKey: "owner", Token: "token", Object: "metadata", RequireOwner: true})
			require.ErrorContains(t, err, "does not enforce context deadlines")
			require.Zero(t, server.CommandCount(), "unsupported socket deadlines must be rejected before I/O")
		})
	}
}

type lostMaintenanceReplyConn struct {
	net.Conn
	armed atomic.Bool
	lost  *atomic.Bool
	after func()
}

func (c *lostMaintenanceReplyConn) Write(data []byte) (int, error) {
	if bytes.Contains(data, []byte("ARGV[5]")) {
		c.armed.Store(true)
	}
	return c.Conn.Write(data)
}

func (c *lostMaintenanceReplyConn) Read(data []byte) (int, error) {
	n, err := c.Conn.Read(data)
	if n > 0 && c.armed.Swap(false) && c.lost.CompareAndSwap(false, true) {
		c.after()
		return 0, io.EOF
	}
	return n, err
}

func TestRedisMaintenancePublicationDoesNotReplayAfterLostReply(t *testing.T) {
	server := miniredis.RunT(t)
	later := redis.NewClient(&redis.Options{Addr: server.Addr(), ContextTimeoutEnabled: true})
	t.Cleanup(func() { require.NoError(t, later.Close()) })
	newer := NewCache(NewRedisCache(later, time.Minute, RedisCompressionNone))
	item := VersionedItem{Key: "{index}|metadata", ReferenceKey: "{index}|reference", BodyKey: "{index}|body", OwnerKey: "{index}|owner", ReferenceValue: "old-body", Token: "old", Object: "old", Expiration: time.Minute}
	var lost atomic.Bool
	committed := make(chan error, 1)
	observed := make(chan string, 1)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), ContextTimeoutEnabled: true, MaxRetries: 3, Dialer: func(ctx context.Context, network, address string) (net.Conn, error) {
		connection, err := (&net.Dialer{}).DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return &lostMaintenanceReplyConn{Conn: connection, lost: &lost, after: func() {
			var initial string
			if err := newer.GetItem(item.Key, &initial); err != nil {
				observed <- ""
				committed <- err
				return
			}
			observed <- initial
			replacement := item
			replacement.Token, replacement.ReferenceValue, replacement.Object = "new", "new-body", "new"
			_, err := newer.StoreVersionedItem(t.Context(), replacement)
			committed <- err
		}}, nil
	}})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	cache := NewCache(NewRedisCache(client, time.Minute, RedisCompressionNone))
	_, err := cache.StoreVersionedItem(t.Context(), item)
	require.Error(t, err, "a lost maintenance reply must not replay the write")
	require.True(t, lost.Load(), "the first publication must commit before its reply is discarded")
	require.NoError(t, <-committed)
	require.Equal(t, "old", <-observed, "the first write must commit before the replacement")
	var current string
	require.NoError(t, newer.GetItem(item.Key, &current))
	require.Equal(t, "new", current, "retries cannot restore an older maintenance generation")
	item.Token, item.ReferenceValue, item.Object = "fresh", "fresh-body", "fresh"
	ok, err := cache.StoreVersionedItem(t.Context(), item)
	require.NoError(t, err)
	require.True(t, ok, "a new maintenance operation must still replace the current generation")
}

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
			ok, err := b.BeginUpdate(t.Context(), "{index}|owner", "new", time.Minute)
			require.NoError(t, err)
			require.False(t, ok)
			publish(a, "old", 1, true)
			begin(b, "new")
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
				publish(a, "delayed", 1, false)
				read(3, "maintenance")
				server.FlushAll()
				publish(a, "delayed", 1, false)
				var got metadata
				require.ErrorIs(t, a.GetItem("{index}|metadata", &got), ErrCacheMiss)
			}
			begin(a, "restore")
			publish(a, "restore", 4, true)
			ok, err = a.StoreVersionedBody(t.Context(), "{index}|reference", bodyKey, reference(4), 4, 2*time.Minute)
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

func TestRedisFetchClaimCannotBePreempted(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), ContextTimeoutEnabled: true})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	cache := NewCache(NewRedisCache(client, time.Minute, RedisCompressionNone))
	ok, err := cache.BeginUpdate(t.Context(), "{claim}|owner", "first", time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = cache.BeginUpdate(t.Context(), "{claim}|owner", "second", time.Minute)
	require.NoError(t, err)
	require.False(t, ok, "a later fetch must not revoke the first fetch before it can publish")
	ok, err = cache.StoreVersionedItem(t.Context(), VersionedItem{Key: "{claim}|meta", ReferenceKey: "{claim}|ref", ReferenceValue: "body-one", BodyKey: "{claim}|body", OwnerKey: "{claim}|owner", Token: "first", Object: "first", Expiration: time.Minute, RequireOwner: true})
	require.NoError(t, err)
	require.True(t, ok, "an overlapping successful fetch must still be able to warm the cache")
}

func TestRedisFetchClaimReplayAndRelease(t *testing.T) {
	for _, compression := range []RedisCompressionType{RedisCompressionNone, RedisCompressionGZip} {
		t.Run(string(compression), func(t *testing.T) {
			server := miniredis.RunT(t)
			client := redis.NewClient(&redis.Options{Addr: server.Addr(), ContextTimeoutEnabled: true})
			t.Cleanup(func() { require.NoError(t, client.Close()) })
			cache := NewCache(NewRedisCache(client, time.Minute, compression))
			key := "{replay}|owner"
			ok, err := cache.BeginUpdate(t.Context(), key, "first", time.Minute)
			require.NoError(t, err)
			require.True(t, ok)
			ok, err = cache.BeginUpdate(t.Context(), key, "first", time.Minute)
			require.NoError(t, err)
			require.True(t, ok, "a lost acquisition reply can confirm its existing token")
			ok, err = cache.ReleaseUpdate(t.Context(), key, "other")
			require.NoError(t, err)
			require.False(t, ok)
			ok, err = cache.ReleaseUpdate(t.Context(), key, "first")
			require.NoError(t, err)
			require.True(t, ok)
			ok, err = cache.BeginUpdate(t.Context(), key, "later", time.Minute)
			require.NoError(t, err)
			require.True(t, ok)
			ok, err = cache.BeginUpdate(t.Context(), key, "first", time.Minute)
			require.NoError(t, err)
			require.False(t, ok, "a replay cannot overwrite a later live claim")
			ok, err = cache.ReleaseUpdate(t.Context(), key, "first")
			require.NoError(t, err)
			require.False(t, ok, "late failure cleanup cannot release a different owner's claim")
			ok, err = cache.BeginUpdate(t.Context(), key, "later", time.Minute)
			require.NoError(t, err)
			require.True(t, ok)
		})
	}
}

func TestRedisMetadataExtendsOnlyMatchingBody(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(strconv.FormatBool(changed), func(t *testing.T) {
			server := miniredis.RunT(t)
			client := redis.NewClient(&redis.Options{Addr: server.Addr(), ContextTimeoutEnabled: true})
			t.Cleanup(func() { require.NoError(t, client.Close()) })
			cache := NewCache(NewRedisCache(client, time.Minute, RedisCompressionNone))
			item := VersionedItem{Key: "{ttl}|meta", ReferenceKey: "{ttl}|ref", ReferenceValue: "one", BodyKey: "{ttl}|body", OwnerKey: "{ttl}|owner", Token: "seed", Object: "seed", Expiration: time.Minute}
			ok, err := cache.StoreVersionedItem(t.Context(), item)
			require.NoError(t, err)
			require.True(t, ok)
			ok, err = cache.StoreVersionedBody(t.Context(), item.ReferenceKey, item.BodyKey, "one", []byte("one"), time.Minute)
			require.NoError(t, err)
			require.True(t, ok)
			server.FastForward(30 * time.Second)
			item.Expiration = 2 * time.Minute
			item.Token = "update"
			if changed {
				item.ReferenceValue = "two"
			}
			ok, err = cache.StoreVersionedItem(t.Context(), item)
			require.NoError(t, err)
			require.True(t, ok)
			want := 2 * time.Minute
			if changed {
				want = 30 * time.Second
			}
			require.Equal(t, want, server.TTL(cache.generateFullKey(item.BodyKey)), "a pending replacement must not renew the superseded body's lifetime")
		})
	}
}
