package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	utilio "github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/session"
)

func TestRateLimiter(t *testing.T) {
	var closers []utilio.Closer
	limiter := NewLoginRateLimiter(10)
	for range 10 {
		closer, err := limiter()
		require.NoError(t, err)
		closers = append(closers, closer)
	}
	// 11 request should fail
	_, err := limiter()
	assert.Equal(t, err, session.InvalidLoginErr)

	if !assert.Len(t, closers, 10) {
		return
	}
	// complete one request
	require.NoError(t, closers[0].Close())
	_, err = limiter()
	require.NoError(t, err)
}
