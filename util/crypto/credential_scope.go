package crypto

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"sync"

	"github.com/golang/groupcache/lru"
	"golang.org/x/crypto/argon2"
)

var credentialScopes = newCredentialScopeCache(1024, deriveCredentialScope)

// CredentialScope derives a versioned cache identity from credential fields.
// Length-prefixed fields preserve arbitrary bytes and cannot collide through
// embedded separators. The same inputs produce the same identity in other
// processes; no per-process secret participates in the persisted result.
//
// Argon2id makes guessing credentials from a persisted fingerprint expensive.
// Its domain salt is deterministic because these are shared cache identities,
// not authentication password verifiers with separately stored random salts.
// Warm requests reuse a bounded in-memory result, indexed by a process-keyed
// MAC. Neither plaintext credential fields nor a public fast digest are retained
// as the memoization key.
func CredentialScope(namespace string, fields ...string) string {
	return credentialScopes.scope(namespace, fields...)
}

// CredentialScopeContext preserves the same identity while allowing a request
// to stop waiting for queued or running derivation work. At most one cold
// derivation runs at a time, including after its caller cancels.
func CredentialScopeContext(ctx context.Context, namespace string, fields ...string) (string, error) {
	return credentialScopes.scopeContext(ctx, namespace, fields...)
}

func deriveCredentialScope(material, salt []byte) []byte {
	// Argon2id's minimum recommended cost: 19 MiB, two passes, one lane.
	return argon2.IDKey(material, salt, 2, 19*1024, 1, 32)
}

type credentialScopeCache struct {
	mu        sync.Mutex
	values    *lru.Cache
	lookupKey [32]byte
	derive    func([]byte, []byte) []byte
	// Serialize cold derivations to bound additional KDF memory to 19 MiB.
	// Warm hits only take mu and are not blocked by this work.
	deriving chan struct{}
}

func newCredentialScopeCache(limit int, derive func([]byte, []byte) []byte) *credentialScopeCache {
	c := &credentialScopeCache{values: lru.New(limit), derive: derive, deriving: make(chan struct{}, 1)}
	if _, err := rand.Read(c.lookupKey[:]); err != nil {
		panic(err)
	}
	return c
}

func (c *credentialScopeCache) get(key [32]byte) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, ok := c.values.Get(key)
	if !ok {
		return "", false
	}
	return value.(string), true
}

func (c *credentialScopeCache) scope(namespace string, fields ...string) string {
	value, _ := c.scopeContext(context.Background(), namespace, fields...)
	return value
}

func (c *credentialScopeCache) scopeContext(ctx context.Context, namespace string, fields ...string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	material := binary.AppendUvarint(nil, uint64(len(namespace)))
	material = append(material, namespace...)
	for _, field := range fields {
		material = binary.AppendUvarint(material, uint64(len(field)))
		material = append(material, field...)
	}
	mac := hmac.New(sha256.New, c.lookupKey[:])
	_, _ = mac.Write(material)
	var key [32]byte
	copy(key[:], mac.Sum(nil))
	if value, ok := c.get(key); ok {
		return value, nil
	}

	select {
	case c.deriving <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		<-c.deriving
		return "", err
	}
	// A previous waiter may already have derived this credential view.
	if value, ok := c.get(key); ok {
		<-c.deriving
		return value, nil
	}
	result := make(chan string, 1)
	go func() {
		defer func() { <-c.deriving }()
		value := "argon2id-v1:" + hex.EncodeToString(c.derive(material, []byte("argo-cd/credential-scope/v1/"+namespace)))
		c.mu.Lock()
		c.values.Add(key, value)
		c.mu.Unlock()
		result <- value
	}()
	select {
	case value := <-result:
		return value, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
