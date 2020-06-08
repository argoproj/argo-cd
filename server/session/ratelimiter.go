package session

import (
	"fmt"
	"time"

	"github.com/argoproj/argo-cd/util/session"

	util "github.com/argoproj/gitops-engine/pkg/utils/io"
	"github.com/bsm/redislock"
	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
)

const (
	lockKey                = "login:lock"
	inProgressCountKey     = "login:in-progress-count"
	inProgressTimeoutDelay = time.Minute
)

type stateStorage interface {
	obtainLock(key string, ttl time.Duration) (util.Closer, error)
	set(key string, value interface{}, expiration time.Duration) error
	get(key string) (int, error)
}

// NewRedisStateStorage creates storage which leverages redis to establish distributed lock and store number
// of incomplete login requests.
func NewRedisStateStorage(client *redis.Client) *redisStateStorage {
	return &redisStateStorage{client: client, locker: redislock.New(client)}
}

type redisStateStorage struct {
	client *redis.Client
	locker *redislock.Client
}

func (redis *redisStateStorage) obtainLock(key string, ttl time.Duration) (util.Closer, error) {
	lock, err := redis.locker.Obtain(key, ttl, nil)
	if err != nil {
		return nil, err
	}
	return util.NewCloser(lock.Release), nil
}

func (redis *redisStateStorage) set(key string, value interface{}, expiration time.Duration) error {
	return redis.client.Set(key, value, expiration).Err()
}

func (redis *redisStateStorage) get(key string) (int, error) {
	return redis.client.Get(key).Int()
}

// NewLoginRateLimiter creates a function which enforces max number of concurrent login requests.
// Function returns closer that should be closed when logging request has completed or error if number
// of incomplete requests exceeded max number.
func NewLoginRateLimiter(storage stateStorage, maxNumber int) func() (util.Closer, error) {
	runLocked := func(callback func() error) error {
		closer, err := storage.obtainLock(lockKey, 100*time.Millisecond)
		if err != nil {
			return fmt.Errorf("failed to enforce max concurrent logins limit: %v", err)
		}
		defer func() {
			if err = closer.Close(); err != nil {
				log.Warnf("failed to release redis lock: %v", err)
			}
		}()
		return callback()
	}

	return func() (util.Closer, error) {
		if err := runLocked(func() error {
			inProgressCount, err := storage.get(inProgressCountKey)
			if err != nil && err != redis.Nil {
				return err
			}
			if inProgressCount = inProgressCount + 1; inProgressCount > maxNumber {
				log.Warnf("Exceeded number of concurrent login requests")
				return session.InvalidLoginErr
			}
			return storage.set(inProgressCountKey, inProgressCount, inProgressTimeoutDelay)
		}); err != nil {
			return nil, err
		}
		return util.NewCloser(func() error {
			inProgressCount, err := storage.get(inProgressCountKey)
			if err != nil && err != redis.Nil {
				return err
			}
			return storage.set(inProgressCountKey, inProgressCount-1, inProgressTimeoutDelay)
		}), nil
	}
}
