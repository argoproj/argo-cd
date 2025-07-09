package pull_request

import "errors"

// RepositoryNotFoundError represents an error when a repository is not found by a pull request provider
type RepositoryNotFoundError struct {
	causingError error
}

func (e *RepositoryNotFoundError) Error() string {
	return e.causingError.Error()
}

// NewRepositoryNotFoundError creates a new repository not found error
func NewRepositoryNotFoundError(err error) error {
	return &RepositoryNotFoundError{causingError: err}
}

// IsRepositoryNotFoundError checks if the given error is a repository not found error
func IsRepositoryNotFoundError(err error) bool {
	var repoErr *RepositoryNotFoundError
	return errors.As(err, &repoErr)
}
