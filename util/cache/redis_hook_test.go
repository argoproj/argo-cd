package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/go-redis/redis/v8"
)

func Test_ReconnectCallbackHookCalled(t *testing.T) {
	called := false
	hook := NewArgoRedisHook(func() {
		called = true
	})

	cmd := &redis.StringCmd{}
	cmd.SetErr(errors.New("Failed to resync revoked tokens. retrying again in 1 minute: dial tcp: lookup argocd-redis on 10.179.0.10:53: no such host"))

	_ = hook.AfterProcess(context.Background(), cmd)

	assert.Equal(t, called, true)
}

func Test_ReconnectCallbackHookNotCalled(t *testing.T) {
	called := false
	hook := NewArgoRedisHook(func() {
		called = true
	})
	cmd := &redis.StringCmd{}
	cmd.SetErr(errors.New("Something wrong"))

	_ = hook.AfterProcess(context.Background(), cmd)

	assert.Equal(t, called, false)
}
