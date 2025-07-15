package pull_request

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepositoryNotFoundError(t *testing.T) {
	t.Run("NewRepositoryNotFoundError creates correct error type", func(t *testing.T) {
		originalErr := errors.New("repository does not exist")
		repoNotFoundErr := NewRepositoryNotFoundError(originalErr)

		require.Error(t, repoNotFoundErr)
		assert.Equal(t, "repository does not exist", repoNotFoundErr.Error())
	})

	t.Run("IsRepositoryNotFoundError identifies RepositoryNotFoundError", func(t *testing.T) {
		originalErr := errors.New("repository does not exist")
		repoNotFoundErr := NewRepositoryNotFoundError(originalErr)

		assert.True(t, IsRepositoryNotFoundError(repoNotFoundErr))
	})

	t.Run("IsRepositoryNotFoundError returns false for regular errors", func(t *testing.T) {
		regularErr := errors.New("some other error")

		assert.False(t, IsRepositoryNotFoundError(regularErr))
	})

	t.Run("IsRepositoryNotFoundError returns false for nil error", func(t *testing.T) {
		assert.False(t, IsRepositoryNotFoundError(nil))
	})

	t.Run("IsRepositoryNotFoundError works with wrapped errors", func(t *testing.T) {
		originalErr := errors.New("repository does not exist")
		repoNotFoundErr := NewRepositoryNotFoundError(originalErr)
		wrappedErr := errors.New("wrapped: " + repoNotFoundErr.Error())

		// Direct RepositoryNotFoundError should be identified
		assert.True(t, IsRepositoryNotFoundError(repoNotFoundErr))

		// Wrapped string error should not be identified (this is expected behavior)
		assert.False(t, IsRepositoryNotFoundError(wrappedErr))
	})
}
