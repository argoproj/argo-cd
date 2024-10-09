package cache

import (
	"context"
	"errors"
	"net"

	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

type argoRedisHooks struct {
	reconnectCallback func()
}

func NewArgoRedisHook(reconnectCallback func()) *argoRedisHooks {
	return &argoRedisHooks{reconnectCallback: reconnectCallback}
}

func (hook *argoRedisHooks) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := next(ctx, network, addr)
		return conn, err
	}
}

func (hook *argoRedisHooks) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		var dnsError *net.DNSError
		err := next(ctx, cmd)
		if err != nil && errors.As(err, &dnsError) {
			log.Warnf("Reconnect to redis because error: \"%v\"", err)
			hook.reconnectCallback()
		}
		return err
	}
}

func (hook *argoRedisHooks) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return nil
}
