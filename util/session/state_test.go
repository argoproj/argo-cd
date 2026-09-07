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
	t.Parallel()
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

func TestUserStateStorage_ResyncDuration(t *testing.T) {
	redis, closer := test.NewInMemoryRedis()
	defer closer()

	t.Run("defaults when unset", func(t *testing.T) {
		// Set explicitly rather than relying on the variable being absent from the ambient environment.
		t.Setenv(envRevokedTokenResyncDuration, "")
		assert.Equal(t, defaultRevokedTokenResyncDuration, NewUserStateStorage(redis).resyncDuration)
	})

	t.Run("honours a valid override", func(t *testing.T) {
		t.Setenv(envRevokedTokenResyncDuration, "5m")
		assert.Equal(t, 5*time.Minute, NewUserStateStorage(redis).resyncDuration)
	})

	t.Run("honours the maximum", func(t *testing.T) {
		t.Setenv(envRevokedTokenResyncDuration, "1h")
		assert.Equal(t, time.Hour, NewUserStateStorage(redis).resyncDuration)
	})

	t.Run("falls back to the default below the minimum", func(t *testing.T) {
		// Below the minimum, so it would increase the load the resync places on Redis.
		t.Setenv(envRevokedTokenResyncDuration, "1s")
		assert.Equal(t, defaultRevokedTokenResyncDuration, NewUserStateStorage(redis).resyncDuration)
	})

	t.Run("falls back to the default above the maximum", func(t *testing.T) {
		// Above the maximum, so it would widen the missed-revocation recovery window beyond the documented cap.
		t.Setenv(envRevokedTokenResyncDuration, "2h")
		assert.Equal(t, defaultRevokedTokenResyncDuration, NewUserStateStorage(redis).resyncDuration)
	})

	t.Run("falls back to the default when unparseable", func(t *testing.T) {
		t.Setenv(envRevokedTokenResyncDuration, "not-a-duration")
		assert.Equal(t, defaultRevokedTokenResyncDuration, NewUserStateStorage(redis).resyncDuration)
	})
}
