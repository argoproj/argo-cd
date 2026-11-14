package crypto

import (
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
	deriveMu sync.Mutex
}

func newCredentialScopeCache(limit int, derive func([]byte, []byte) []byte) *credentialScopeCache {
	c := &credentialScopeCache{values: lru.New(limit), derive: derive}
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
		return value
	}

	c.deriveMu.Lock()
	defer c.deriveMu.Unlock()
	// A previous waiter may already have derived this credential view.
	if value, ok := c.get(key); ok {
		return value
	}
	value := "argon2id-v1:" + hex.EncodeToString(c.derive(material, []byte("argo-cd/credential-scope/v1/"+namespace)))
	c.mu.Lock()
	c.values.Add(key, value)
	c.mu.Unlock()
	return value
}
