package session

import (
	"fmt"
	"time"

	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/session"

	"github.com/bsm/redislock"
	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
)

const (
	lockKey                = "login:lock"
	inProgressCountKey     = "login:in-progress-count"
	inProgressTimeoutDelay = time.Minute
)

func NewLoginRateLimiter(client *redis.Client, maxNumber int) func() (util.Closer, error) {
	locker := redislock.New(client)
	runLocked := func(callback func() error) error {
		lock, err := locker.Obtain(lockKey, 100*time.Millisecond, nil)
		if err != nil {
			return fmt.Errorf("failed to enforce max concurrent logins limit: %v", err)
		}
		defer func() {
			if err = lock.Release(); err != nil {
				log.Warnf("failed to release redis lock: %v", err)
			}
		}()
		return callback()
	}

	return func() (util.Closer, error) {
		if err := runLocked(func() error {
			inProgressCount, err := client.Get(inProgressCountKey).Int()
			if err != nil && err != redis.Nil {
				return err
			}
			if inProgressCount = inProgressCount + 1; inProgressCount > maxNumber {
				log.Warnf("Exceeded number of concurrent login requests")
				return session.InvalidLoginErr
			}
			return client.Set(inProgressCountKey, inProgressCount, inProgressTimeoutDelay).Err()
		}); err != nil {
			return nil, err
		}
		return util.NewCloser(func() error {
			inProgressCount, err := client.Get(inProgressCountKey).Int()
			if err != nil && err != redis.Nil {
				return err
			}
			return client.Set(inProgressCountKey, inProgressCount-1, inProgressTimeoutDelay).Err()
		}), nil
	}
}
