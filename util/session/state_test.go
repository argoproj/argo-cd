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
