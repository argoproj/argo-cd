package session

import (
	"context"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserStateStorage_LoadRevokedTokens(t *testing.T) {
	redis, closer := test.NewInMemoryRedis()
	defer closer()

	err := redis.Set(context.Background(), revokedTokenPrefix+"abc", "", time.Hour).Err()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	storage := NewUserStateStorage(redis)
	storage.Init(ctx)
	time.Sleep(time.Millisecond * 100)

	assert.True(t, storage.IsTokenRevoked("abc"))
}
