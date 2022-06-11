package errors

type credentialsConfigurationError struct {
	causingError error
}

func (err *credentialsConfigurationError) Error() string {
	return err.causingError.Error()
}

// NewCredentialsConfigurationError wraps any error into a credentials configuration error.
func NewCredentialsConfigurationError(err error) error {
	return &credentialsConfigurationError{causingError: err}
}

// IsCredentialsConfigurationError checks if the given error is a wrapped credentials configuration error.
func IsCredentialsConfigurationError(err error) bool {
	_, ok := err.(*credentialsConfigurationError)
	return ok
}
