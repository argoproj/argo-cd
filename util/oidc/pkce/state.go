package pkce

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"

	util "github.com/argoproj/argo-cd/v2/util/io"
)

const (
	pkceEntryPrefix         = "pkce-auth|"
	newVerifierCodeWithAuth = "new-verifier-code-with-auth"
)

type pkceStateStorage struct {
	redis          *redis.Client
	pkceAuthCodes  map[string]string
	lock           sync.RWMutex
	resyncDuration time.Duration
}

var _ PKCEStateStorage = &pkceStateStorage{}

func NewPKCEStateStorage(redis *redis.Client) *pkceStateStorage {
	return &pkceStateStorage{
		pkceAuthCodes:  map[string]string{},
		resyncDuration: time.Hour,
		redis:          redis,
	}
}

func buildRedisEntry(pkceCodes *PKCECodes) string {
	return pkceEntryPrefix + buildRedisValue(pkceCodes)
}

func buildRedisValue(pkceCodes *PKCECodes) string {
	return pkceCodes.Nonce + "~" + pkceCodes.CodeVerifier
}

func (storage *pkceStateStorage) Init(ctx context.Context) {
	go storage.watchPKCEEntries(ctx)
	ticker := time.NewTicker(storage.resyncDuration)
	go func() {
		storage.loadPKCEEntriesSafe()
		for range ticker.C {
			storage.loadPKCEEntriesSafe()
		}
	}()
	go func() {
		<-ctx.Done()
		ticker.Stop()
	}()
}

func (storage *pkceStateStorage) watchPKCEEntries(ctx context.Context) {
	pubsub := storage.redis.Subscribe(ctx, newVerifierCodeWithAuth)
	defer util.Close(pubsub)

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case val := <-ch:
			storage.lock.Lock()
			pkceParts := strings.Split(val.Payload, "~")
			storage.pkceAuthCodes[pkceParts[0]] = strings.Join(pkceParts[1:], "")
			storage.lock.Unlock()
		}
	}
}

func (storage *pkceStateStorage) loadPKCEEntriesSafe() {
	err := storage.loadPKCEEntries()
	for err != nil {
		log.Warnf("Failed to resync pkce entries. retrying again in 1 minute: %v", err)
		time.Sleep(time.Minute)
		err = storage.loadPKCEEntries()
	}
}

func (storage *pkceStateStorage) loadPKCEEntries() error {
	storage.lock.Lock()
	defer storage.lock.Unlock()
	storage.pkceAuthCodes = map[string]string{}
	iterator := storage.redis.Scan(context.Background(), 0, pkceEntryPrefix+"*", -1).Iterator()
	for iterator.Next(context.Background()) {
		parts := strings.Split(iterator.Val(), "|")
		if len(parts) < 2 {
			log.Warnf("Unexpected redis key prefixed with '%s'. Must have nonce and code verifier, tilde separated, after the prefix but got: '%s'.",
				pkceEntryPrefix,
				iterator.Val())
			continue
		}
		pkceParts := strings.Split(parts[1], "~")
		storage.pkceAuthCodes[pkceParts[0]] = strings.Join(pkceParts[1:], "")
	}
	if iterator.Err() != nil {
		return iterator.Err()
	}

	return nil
}

func (storage *pkceStateStorage) StorePKCEEntry(ctx context.Context, pkceCodes *PKCECodes, expiringAt time.Duration) error {
	storage.lock.Lock()
	storage.pkceAuthCodes[pkceCodes.Nonce] = pkceCodes.CodeVerifier
	storage.lock.Unlock()
	if err := storage.redis.Set(ctx, buildRedisEntry(pkceCodes), "", expiringAt).Err(); err != nil {
		return err
	}
	return storage.redis.Publish(ctx, newVerifierCodeWithAuth, buildRedisValue(pkceCodes)).Err()
}

func (storage *pkceStateStorage) RetrieveCodeVerifier(nonce string) string {
	storage.lock.Lock()
	defer storage.lock.Unlock()
	if codeVerifier, ok := storage.pkceAuthCodes[nonce]; ok {
		return codeVerifier
	} else {
		//go to redis
		iterator := storage.redis.Scan(context.Background(), 0, pkceEntryPrefix+nonce+"*", 1).Iterator()
		if iterator.Next(context.Background()) {
			parts := strings.Split(iterator.Val(), "|")
			if len(parts) < 2 {
				log.Warnf("Unexpected redis key prefixed with '%s'. Must have nonce and code verifier, tilde separated, after the prefix but got: '%s'.",
					pkceEntryPrefix+nonce,
					iterator.Val())
				return ""
			}
			pkceParts := strings.Split(parts[1], "~")
			codeVerifier := strings.Join(pkceParts[1:], "")
			storage.pkceAuthCodes[nonce] = codeVerifier
			return codeVerifier
		}

		if iterator.Err() != nil {
			log.Warnf("Unexpected redis error when optimistically looking in redis for nonce '%s' : %v", nonce, iterator.Err())
		}
	}
	log.Warnf("Did not find code verifier for nonce '%s'", nonce)
	return ""
}

type PKCEStateStorage interface {
	Init(ctx context.Context)
	// StorePKCEEntry stores the verifier code and authorization code combination
	StorePKCEEntry(ctx context.Context, pkceEntry *PKCECodes, expiringAt time.Duration) error
	// RetrieveCodeVerifier gets the verifier code in memory
	RetrieveCodeVerifier(nonce string) string
}
