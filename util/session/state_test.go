package session

import (
	"context"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v3/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserStateStorage_LoadRevokedTokens(t *testing.T) {
	redis, closer := test.NewInMemoryRedis()
	defer closer()

	err := redis.Set(t.Context(), revokedTokenPrefix+"abc", "", time.Hour).Err()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	storage := NewUserStateStorage(redis)
	storage.Init(ctx)
	time.Sleep(time.Millisecond * 100)

	assert.True(t, storage.IsTokenRevoked("abc"))
}

func TestUserStateStorage_RevokeOIDCSession(t *testing.T) {
	redis, closer := test.NewInMemoryRedis()
	defer closer()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	storage := NewUserStateStorage(redis)
	storage.Init(ctx)
	time.Sleep(time.Millisecond * 100)

	sid := "test-session-id"
	assert.False(t, storage.IsOIDCSessionRevoked(sid), "session should not be revoked before RevokeOIDCSession")

	err := storage.RevokeOIDCSession(t.Context(), sid, time.Hour)
	require.NoError(t, err)

	assert.True(t, storage.IsOIDCSessionRevoked(sid), "session should be revoked after RevokeOIDCSession")
}

func TestUserStateStorage_LoadRevokedOIDCSessions(t *testing.T) {
	// Pre-populate a revoked OIDC SID in Redis before the storage is initialised,
	// mirroring the pattern used in TestUserStateStorage_LoadRevokedTokens.
	redis, closer := test.NewInMemoryRedis()
	defer closer()

	sid := "pre-existing-session"
	err := redis.Set(t.Context(), revokedOIDCSIDPrefix+sid, "", time.Hour).Err()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	storage := NewUserStateStorage(redis)
	storage.Init(ctx)
	time.Sleep(time.Millisecond * 100)

	assert.True(t, storage.IsOIDCSessionRevoked(sid), "pre-existing revoked SID should be loaded on init")
}

func TestUserStateStorage_IsOIDCSessionRevoked_UnknownSID(t *testing.T) {
	storage := NewUserStateStorage(nil)

	assert.False(t, storage.IsOIDCSessionRevoked("unknown-sid"))
}

func TestUserStateStorage_RevokeOIDCSID_MalformedRedisKey(t *testing.T) {
	// A key that has the right prefix but too many pipe-separated parts should be skipped gracefully.
	redis, closer := test.NewInMemoryRedis()
	defer closer()

	// Write a key with the correct prefix but an extra "|" segment.
	malformed := revokedOIDCSIDPrefix + "part|extra"
	err := redis.Set(t.Context(), malformed, "", time.Hour).Err()
	require.NoError(t, err)

	// A normally revoked SID that should still load correctly.
	goodSID := "good-session"
	err = redis.Set(t.Context(), revokedOIDCSIDPrefix+goodSID, "", time.Hour).Err()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	storage := NewUserStateStorage(redis)
	storage.Init(ctx)
	time.Sleep(time.Millisecond * 100)

	// The malformed key must not crash and must not register a revocation.
	assert.False(t, storage.IsOIDCSessionRevoked("part|extra"), "malformed key should not be treated as revoked")
	assert.True(t, storage.IsOIDCSessionRevoked(goodSID), "good SID should still be loaded")
}

func TestUserStateStorage_RevokeOIDCSID_PubSubPropagation(t *testing.T) {
	// Two storage instances share the same Redis, simulating two argocd-server replicas.
	// Revoking a SID on instance A must be visible on instance B via the pub/sub channel.
	redis, closer := test.NewInMemoryRedis()
	defer closer()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	instanceA := NewUserStateStorage(redis)
	instanceA.Init(ctx)

	instanceB := NewUserStateStorage(redis)
	instanceB.Init(ctx)

	// Give the pub/sub watchers time to subscribe.
	time.Sleep(time.Millisecond * 100)

	sid := "cross-replica-session"
	assert.False(t, instanceB.IsOIDCSessionRevoked(sid))

	err := instanceA.RevokeOIDCSession(t.Context(), sid, time.Hour)
	require.NoError(t, err)

	// Allow the pub/sub message to propagate.
	time.Sleep(time.Millisecond * 100)

	assert.True(t, instanceB.IsOIDCSessionRevoked(sid), "instance B should see the revocation published by instance A")
}
