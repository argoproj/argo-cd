package session

import (
	"testing"
	"time"

	util "github.com/argoproj/gitops-engine/pkg/utils/io"
	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/util/session"
)

type fakeStorage struct {
	values map[string]int
}

func (f *fakeStorage) obtainLock(key string, ttl time.Duration) (util.Closer, error) {
	return util.NopCloser, nil
}

func (f *fakeStorage) set(key string, value interface{}, expiration time.Duration) error {
	f.values[key] = value.(int)
	return nil
}

func (f *fakeStorage) get(key string) (int, error) {
	return f.values[key], nil
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{values: map[string]int{}}
}

func TestRateLimiter(t *testing.T) {
	var closers []util.Closer
	limiter := NewLoginRateLimiter(newFakeStorage(), 10)
	for i := 0; i < 10; i++ {
		closer, err := limiter()
		assert.NoError(t, err)
		closers = append(closers, closer)
	}
	// 11 request should fail
	_, err := limiter()
	assert.Equal(t, err, session.InvalidLoginErr)

	if !assert.Equal(t, len(closers), 10) {
		return
	}
	// complete one request
	assert.NoError(t, closers[0].Close())
	_, err = limiter()
	assert.NoError(t, err)
}
