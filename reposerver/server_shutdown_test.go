package reposerver

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	executil "github.com/argoproj/argo-cd/v3/util/exec"
)

func TestShutdownErrToGRPC(t *testing.T) {
	cmdErr := fmt.Errorf("failed to checkout: %w", executil.ErrShuttingDown)
	flattened := status.Errorf(codes.Internal, "Failed to checkout revision abc: %v", cmdErr)

	t.Run("nil stays nil", func(t *testing.T) {
		assert.NoError(t, shutdownErrToGRPC(nil, true))
	})
	t.Run("untouched before shutdown", func(t *testing.T) {
		assert.Equal(t, flattened, shutdownErrToGRPC(flattened, false))
	})
	t.Run("wrapped error becomes Unavailable", func(t *testing.T) {
		err := shutdownErrToGRPC(cmdErr, true)
		assert.Equal(t, codes.Unavailable, status.Code(err))
		assert.Equal(t, cmdErr.Error(), status.Convert(err).Message())
	})
	t.Run("flattened status becomes Unavailable keeping its message", func(t *testing.T) {
		err := shutdownErrToGRPC(flattened, true)
		assert.Equal(t, codes.Unavailable, status.Code(err))
		assert.Equal(t, status.Convert(flattened).Message(), status.Convert(err).Message())
	})
	t.Run("unrelated error during shutdown is retried too", func(t *testing.T) {
		assert.Equal(t, codes.Unavailable, status.Code(shutdownErrToGRPC(errors.New("boom"), true)))
	})
}
