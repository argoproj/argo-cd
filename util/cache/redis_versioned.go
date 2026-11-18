package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

func (r *redisCache) BeginUpdate(ctx context.Context, key, token string, expiration time.Duration) (bool, error) {
	if !r.client.Options().ContextTimeoutEnabled {
		return false, errors.New("redis client does not enforce context deadlines")
	}
	result, err := acquireVersionedScript.Run(ctx, r.client, []string{r.getKey(key)}, token, max(int64(1), expiration.Milliseconds())).Int64()
	return result == 1, err
}

// Replaying acquisition can confirm the same token, never replace a later
// owner's token. Contenders cannot preempt a live fetch before publication.
var acquireVersionedScript = redis.NewScript(`
local owner = redis.call('GET', KEYS[1])
if owner then
 if owner == ARGV[1] then return 1 end
 return 0
end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
return 1
`)

var releaseVersionedScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 0 end
return redis.call('DEL', KEYS[1])
`)

func (r *redisCache) ReleaseUpdate(ctx context.Context, key, token string) (bool, error) {
	if !r.client.Options().ContextTimeoutEnabled {
		return false, errors.New("redis client does not enforce context deadlines")
	}
	result, err := releaseVersionedScript.Run(ctx, r.client, []string{r.getKey(key)}, token).Int64()
	return result == 1, err
}

var renewVersionedScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 0 end
return redis.call('PEXPIRE', KEYS[1], ARGV[2])
`)

func (r *redisCache) RenewUpdate(ctx context.Context, key, token string, expiration time.Duration) (bool, error) {
	if !r.client.Options().ContextTimeoutEnabled {
		return false, errors.New("redis client does not enforce context deadlines")
	}
	result, err := renewVersionedScript.Run(ctx, r.client, []string{r.getKey(key)}, token, max(int64(1), expiration.Milliseconds())).Int64()
	return result == 1, err
}

// A missing owner is a lost claim, including when Redis evicts every key.
// Consumption makes each claim single-use; maintenance writes invalidate it.
var publishVersionedScript = redis.NewScript(`
if ARGV[5] == '1' and redis.call('GET', KEYS[3]) ~= ARGV[3] then return 0 end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[4])
redis.call('SET', KEYS[2], ARGV[2], 'PX', ARGV[4])
redis.call('DEL', KEYS[3])
if redis.call('GET', KEYS[5]) == ARGV[2] then
 local ttl = redis.call('PTTL', KEYS[4])
 if ttl >= 0 then
  if ttl < tonumber(ARGV[4]) then ttl = tonumber(ARGV[4]) end
  redis.call('PEXPIRE', KEYS[4], ttl)
  redis.call('PEXPIRE', KEYS[5], ttl)
 end
end
return 1
`)

func (r *redisCache) StoreVersioned(ctx context.Context, item VersionedItem) (bool, error) {
	if item.RequireOwner && !r.client.Options().ContextTimeoutEnabled {
		return false, errors.New("redis client does not enforce context deadlines")
	}
	metadata, err := r.marshal(item.Object)
	if err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	expiration := item.Expiration
	if expiration == 0 {
		expiration = r.expiration
	}
	fenced := "0"
	if item.RequireOwner {
		fenced = "1"
	}
	bodyKey := r.getKey(item.BodyKey)
	result, err := publishVersionedScript.Run(ctx, r.client, []string{r.getKey(item.Key), r.getKey(item.ReferenceKey), r.getKey(item.OwnerKey), bodyKey, r.getKey(item.BodyKey + "|digest")}, metadata, item.ReferenceValue, item.Token, max(int64(1), expiration.Milliseconds()), fenced).Int64()
	return result == 1, err
}

var shareVersionedBodyScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) ~= ARGV[3] then return 0 end
local ttl = tonumber(ARGV[2])
if redis.call('GET', KEYS[3]) == ARGV[3] then
 local existing = redis.call('PTTL', KEYS[2])
 if existing > ttl then ttl = existing end
end
redis.call('SET', KEYS[2], ARGV[1], 'PX', ttl)
redis.call('SET', KEYS[3], ARGV[3], 'PX', ttl)
return 1
`)

func (r *redisCache) StoreVersionedBody(ctx context.Context, referenceKey, bodyKey, referenceValue string, body any, expiration time.Duration) (bool, error) {
	if _, bounded := ctx.Deadline(); bounded && !r.client.Options().ContextTimeoutEnabled {
		return false, errors.New("redis client does not enforce context deadlines")
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	value, err := r.marshal(body)
	if err != nil {
		return false, err
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if expiration == 0 {
		expiration = r.expiration
	}
	result, err := shareVersionedBodyScript.Run(ctx, r.client, []string{r.getKey(referenceKey), r.getKey(bodyKey), r.getKey(bodyKey + "|digest")}, value, max(int64(1), expiration.Milliseconds()), referenceValue).Int64()
	return result == 1, err
}
