package session

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"

	util "github.com/argoproj/argo-cd/util/io"
)

const (
	revokedTokenPrefix = "revoked-token|"
	newRevokedTokenKey = "new-revoked-token"
)

type userStateStorage struct {
	attempts       map[string]LoginAttempts
	redis          *redis.Client
	revokedTokens  map[string]bool
	lock           sync.RWMutex
	resyncDuration time.Duration
}

func NewUserStateStorage(redis *redis.Client) *userStateStorage {
	return &userStateStorage{
		attempts:       map[string]LoginAttempts{},
		revokedTokens:  map[string]bool{},
		resyncDuration: time.Hour,
		redis:          redis,
	}
}

func (storage *userStateStorage) Init(ctx context.Context) {
	go storage.watchRevokedTokens(ctx)
	ticker := time.NewTicker(storage.resyncDuration)
	go func() {
		storage.loadRevokedTokensSafe()
		for range ticker.C {
			storage.loadRevokedTokensSafe()
		}
	}()
	go func() {
		<-ctx.Done()
		ticker.Stop()
	}()
}

func (storage *userStateStorage) watchRevokedTokens(ctx context.Context) {
	pubsub := storage.redis.Subscribe(ctx, newRevokedTokenKey)
	defer util.Close(pubsub)

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case val := <-ch:
			storage.lock.Lock()
			storage.revokedTokens[val.Payload] = true
			storage.lock.Unlock()
		}
	}
}

func (storage *userStateStorage) loadRevokedTokensSafe() {
	for err := storage.loadRevokedTokens(); err != nil; {
		log.Warnf("Failed to resync revoked tokens. retrying again in 1 minute: %v", err)
		time.Sleep(time.Minute)
	}
}

func (storage *userStateStorage) loadRevokedTokens() error {
	storage.lock.Lock()
	defer storage.lock.Unlock()
	storage.revokedTokens = map[string]bool{}
	iterator := storage.redis.Scan(context.Background(), 0, revokedTokenPrefix+"*", -1).Iterator()
	for iterator.Next(context.Background()) {
		parts := strings.Split(iterator.Val(), "|")
		if len(parts) != 2 {
			log.Warnf("Unexpected redis key prefixed with '%s'. Must have token id after the prefix but got: '%s'.",
				revokedTokenPrefix,
				iterator.Val())
			continue
		}
		storage.revokedTokens[parts[1]] = true
	}
	if iterator.Err() != nil {
		return iterator.Err()
	}

	return nil
}

func (storage *userStateStorage) GetLoginAttempts(attempts *map[string]LoginAttempts) error {
	*attempts = storage.attempts
	return nil
}

func (storage *userStateStorage) SetLoginAttempts(attempts map[string]LoginAttempts) error {
	storage.attempts = attempts
	return nil
}

func (storage *userStateStorage) RevokeToken(ctx context.Context, id string, expiringAt time.Duration) error {
	storage.lock.Lock()
	storage.revokedTokens[id] = true
	storage.lock.Unlock()
	if err := storage.redis.Set(ctx, revokedTokenPrefix+id, "", expiringAt).Err(); err != nil {
		return err
	}
	return storage.redis.Publish(ctx, newRevokedTokenKey, id).Err()
}

func (storage *userStateStorage) IsTokenRevoked(id string) bool {
	storage.lock.RLock()
	defer storage.lock.RUnlock()
	return storage.revokedTokens[id]
}

type UserStateStorage interface {
	Init(ctx context.Context)
	// GetLoginAttempts return number of concurrent login attempts
	GetLoginAttempts(attempts *map[string]LoginAttempts) error
	// SetLoginAttempts sets number of concurrent login attempts
	SetLoginAttempts(attempts map[string]LoginAttempts) error
	// RevokeToken revokes token with given id (information about revocation expires after specified timeout)
	RevokeToken(ctx context.Context, id string, expiringAt time.Duration) error
	// IsTokenRevoked checks if given token is revoked
	IsTokenRevoked(id string) bool
}
