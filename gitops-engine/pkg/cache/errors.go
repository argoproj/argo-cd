package cache

import (
	"strings"
)

// ErrorClassification provides structured information about cache sync errors
type ErrorClassification struct {
	IsCacheTainting bool   // Whether the error should taint the cache but allow continuation
	IsTransient     bool   // Whether the error is likely transient and can be retried
	ErrorType       string // Type of error for categorization
}

// ClassifyError analyzes an error and returns structured classification information
// This centralizes error analysis logic that was previously scattered across the codebase
func ClassifyError(err error) ErrorClassification {
	if err == nil {
		return ErrorClassification{
			IsCacheTainting: false,
			IsTransient:     false,
			ErrorType:       "none",
		}
	}

	errStr := err.Error()

	// Conversion webhook errors - these should taint the cache but allow continuation
	if strings.Contains(errStr, "conversion webhook") ||
		(strings.Contains(errStr, "failed") && strings.Contains(errStr, "convert")) {
		return ErrorClassification{
			IsCacheTainting: true,
			IsTransient:     false,
			ErrorType:       "conversion_webhook",
		}
	}

	// Pagination token expiration - transient and should taint cache
	if strings.Contains(errStr, "expired") && strings.Contains(errStr, "too old resource version") {
		return ErrorClassification{
			IsCacheTainting: true,
			IsTransient:     true,
			ErrorType:       "pagination_expired",
		}
	}

	// Connection errors - not cache tainting, indicates total failure
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "i/o timeout") {
		return ErrorClassification{
			IsCacheTainting: false,
			IsTransient:     true,
			ErrorType:       "connection_failure",
		}
	}

	// Default case - unknown error
	return ErrorClassification{
		IsCacheTainting: false,
		IsTransient:     false,
		ErrorType:       "unknown",
	}
}