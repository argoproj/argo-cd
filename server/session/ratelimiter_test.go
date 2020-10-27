package session

import (
	"testing"

	"github.com/stretchr/testify/assert"

	util "github.com/argoproj/argo-cd/util/io"
	"github.com/argoproj/argo-cd/util/session"
)

func TestRateLimiter(t *testing.T) {
	var closers []util.Closer
	limiter := NewLoginRateLimiter(10)
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
