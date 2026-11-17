package cache

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/helm"
)

func TestHelmIndexCacheExpirationIndependentFromRevisions(t *testing.T) {
	for _, expiration := range []time.Duration{0, 30 * time.Second, time.Minute, 3 * time.Minute, 48 * time.Hour} {
		t.Run(expiration.String(), func(t *testing.T) {
			cache, redisServer, _, _ := helmIndexContractSetup(t)
			require.Equal(t, time.Minute, cache.GetHelmIndexCacheExpiration())
			cache.revisionCacheExpiration = 7 * time.Minute
			cache.SetHelmIndexCacheExpiration(expiration)
			require.Equal(t, expiration, cache.GetHelmIndexCacheExpiration())
			validatedAt := time.Now().Add(-45 * time.Second)
			record := &helm.HelmIndexRecord{
				Body: helmIndexContractBody("1.0.0"), ETag: `"1.0.0"`,
				StoredAt: validatedAt.UTC().Format(time.RFC3339Nano),
			}
			require.NoError(t, cache.SetHelmIndexRetainedRecord("record", record))
			var body []byte
			err := cache.GetHelmIndex("record", &body)
			if expiration > 45*time.Second {
				require.NoError(t, err)
				require.Equal(t, record.Body, body)
			} else {
				require.ErrorIs(t, err, ErrCacheMiss, "expired Helm bodies must miss despite the longer Git revision interval")
			}
			retained, err := cache.GetHelmIndexRetainedRecord("record")
			require.NoError(t, err)
			require.Equal(t, record, retained, "validators and body must survive freshness expiry together")
			require.NoError(t, cache.SetGitReferences("git", nil))
			require.NoError(t, cache.SetOCITags("oci", []byte("tags")))
			require.NoError(t, cache.SetHelmIndex("legacy", record.Body))
			checked := 0
			for _, key := range redisServer.Keys() {
				switch {
				case strings.Contains(key, "helm-index-v2|record"):
					want := time.Until(validatedAt.Add(expiration + max(24*time.Hour, expiration)))
					require.InDelta(t, float64(want), float64(redisServer.TTL(key)), float64(time.Second))
					checked++
				case strings.Contains(key, "git-refs|git"), strings.Contains(key, "oci-tags|oci"):
					require.Equal(t, 7*time.Minute, redisServer.TTL(key), "Helm configuration must not change Git/OCI TTLs")
					checked++
				case strings.Contains(key, "helm-index|legacy") && expiration > 0:
					require.Equal(t, expiration, redisServer.TTL(key))
					checked++
				}
			}
			wantKeys := 4
			if expiration == 0 {
				wantKeys-- // Zero uses the Redis backend's default legacy-key TTL.
			}
			require.Equal(t, wantKeys, checked)
		})
	}
}
