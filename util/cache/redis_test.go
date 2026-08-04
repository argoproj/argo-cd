package cache

import (
	"bytes"
	"compress/gzip"
	"io"
	"strconv"
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
