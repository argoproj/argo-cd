package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	util "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/session"
)

func TestRateLimiter(t *testing.T) {
	var closers []util.Closer
	limiter := NewLoginRateLimiter(10)
	for i := 0; i < 10; i++ {
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
