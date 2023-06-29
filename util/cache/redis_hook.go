package cache

import (
	"context"
	"strings"

	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

const NoSuchHostErr = "no such host"

type argoRedisHooks struct {
	reconnectCallback func()
}

func NewArgoRedisHook(reconnectCallback func()) *argoRedisHooks {
	return &argoRedisHooks{reconnectCallback: reconnectCallback}
}

func (hook *argoRedisHooks) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (hook *argoRedisHooks) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	if cmd.Err() != nil && strings.Contains(cmd.Err().Error(), NoSuchHostErr) {
		log.Warnf("Reconnect to redis because error: \"%v\"", cmd.Err())
		hook.reconnectCallback()
	}
	return nil
}

func (hook *argoRedisHooks) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (hook *argoRedisHooks) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	return nil
}

func (hook *argoRedisHooks) DialHook(next redis.DialHook) redis.DialHook {
	return nil
}

func (hook *argoRedisHooks) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return nil
}

func (hook *argoRedisHooks) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return nil
}
