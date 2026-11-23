package cache

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
)

func newMiniRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

// Regression: freshness and request identity must come from the SAME record.
// Reading them separately lets an overlapping writer renew freshness for
// identical body bytes between the two reads, authorizing a body whose own
// validation time had already expired.
func TestFreshRecordSingleRead(t *testing.T) {
	testCache := NewCache(cacheutil.NewCache(cacheutil.NewRedisCache(newMiniRedis(t), time.Minute, cacheutil.RedisCompressionNone)), time.Minute, time.Minute, time.Minute)
	testCache.SetHelmIndexCacheExpiration(time.Minute)
	const repo = "https://charts.example.com"

	bodyA := []byte("apiVersion: v1\nentries:\n  child:\n    - name: child\n      version: 1.0.0\n")
	recordA := &HelmIndexRecord{Body: bodyA, ETag: `"1.0.0"`, StoredAt: time.Now().UTC().Format(time.RFC3339Nano), URLIdentity: "helm-index-request-v1|agent-a|url-a"}
	require.NoError(t, testCache.SetHelmIndexRetainedRecord(repo, recordA))

	// Read 1 (raw): fresh, returns body A.
	var data []byte
	require.NoError(t, testCache.GetHelmIndex(repo, &data))
	require.Equal(t, bodyA, data)

	// Overlapping writer (agent B) renews freshness for the SAME bytes
	// between read 1 and read 2.
	recordB := &HelmIndexRecord{Body: bodyA, ETag: `"1.0.0"`, StoredAt: time.Now().UTC().Format(time.RFC3339Nano), URLIdentity: "helm-index-request-v1|agent-b|url-b"}
	require.NoError(t, testCache.SetHelmIndexRetainedRecord(repo, recordB))

	// The single-read fresh getter returns a record whose freshness and
	// identity are from the same generation: the newest record, agent B's.
	fresh, err := testCache.GetHelmIndexFreshRecordContext(t.Context(), repo)
	require.NoError(t, err)
	require.Equal(t, "helm-index-request-v1|agent-b|url-b", fresh.URLIdentity)

	// Expired record: the fresh getter refuses even though the record exists
	// (the retained reader still returns it for conditional revalidation).
	expired := *recordB
	expired.StoredAt = time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano)
	require.NoError(t, testCache.SetHelmIndexRetainedRecord(repo, &expired))
	_, err = testCache.GetHelmIndexFreshRecordContext(t.Context(), repo)
	require.ErrorIs(t, err, ErrCacheMiss)
	retained, err := testCache.GetHelmIndexRetainedRecord(repo)
	require.NoError(t, err)
	require.Equal(t, "helm-index-request-v1|agent-b|url-b", retained.URLIdentity)
}
