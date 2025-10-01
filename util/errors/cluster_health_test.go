package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeMessage(t *testing.T) {
	tests := []struct {
		name              string
		message           string
		expectedHasIssue  bool
		expectedIssueType ClusterHealthIssueType
		expectedContext   ErrorContext
		expectedSeverity  ClusterHealthSeverity
		expectedPartial   bool
		expectedTainting  bool
	}{
		// Conversion webhook errors
		{
			name:              "conversion webhook failed",
			message:           "conversion webhook for example.com/v1, Kind=Example failed: Post error",
			expectedHasIssue:  true,
			expectedIssueType: IssueTypeConversionWebhook,
			expectedContext:   ContextGenericFailure,
			expectedSeverity:  SeverityDegraded,
			expectedPartial:   true,
			expectedTainting:  true,
		},
		{
			name:              "conversion webhook with all targets failed",
			message:           "cannot sync application because all target resources have conversion webhook failures",
			expectedHasIssue:  true,
			expectedIssueType: IssueTypeConversionWebhook,
			expectedContext:   ContextAllTargetsFailed,
			expectedSeverity:  SeverityDegraded,
			expectedPartial:   true,
			expectedTainting:  true,
		},
		{
			name:              "conversion webhook with list operation",
			message:           "failed to list resources: conversion webhook error",
			expectedHasIssue:  true,
			expectedIssueType: IssueTypeConversionWebhook,
			expectedContext:   ContextListOperationFailed,
			expectedSeverity:  SeverityDegraded,
			expectedPartial:   true,
			expectedTainting:  true,
		},
		{
			name:              "known conversion webhook failures",
			message:           "cluster has known conversion webhook failures",
			expectedHasIssue:  true,
			expectedIssueType: IssueTypeConversionWebhook,
			expectedContext:   ContextGenericFailure,
			expectedSeverity:  SeverityDegraded,
			expectedPartial:   true,
			expectedTainting:  true,
		},

		// Unavailable resource types
		{
			name:              "unavailable resource types",
			message:           "found 2 unavailable resource types",
			expectedHasIssue:  true,
			expectedIssueType: IssueTypeUnavailableTypes,
			expectedContext:   ContextGenericFailure,
			expectedSeverity:  SeverityDegraded,
			expectedPartial:   true,
			expectedTainting:  true,
		},

		// Pagination/expiration errors
		{
			name:              "expired resource version",
			message:           "Expired: too old resource version: 123456",
			expectedHasIssue:  true,
			expectedIssueType: IssueTypeResourceExpired,
			expectedContext:   ContextGenericFailure,
			expectedSeverity:  SeverityWarning,
			expectedPartial:   true,
			expectedTainting:  true,
		},
		{
			name:              "resourceVersion too old",
			message:           "the resourceVersion is too old",
			expectedHasIssue:  true,
			expectedIssueType: IssueTypeResourceExpired,
			expectedContext:   ContextGenericFailure,
			expectedSeverity:  SeverityWarning,
			expectedPartial:   true,
			expectedTainting:  true,
		},

		// List failures
		{
			name:              "list failure with conversion",
			message:           "failed to list resources: conversion error",
			expectedHasIssue:  true,
			expectedIssueType: IssueTypeListFailure,
			expectedContext:   ContextListOperationFailed,
			expectedSeverity:  SeverityDegraded,
			expectedPartial:   true,
			expectedTainting:  true,
		},
		{
			name:              "list failure without conversion",
			message:           "failed to list resources for apps/v1",
			expectedHasIssue:  true,
			expectedIssueType: IssueTypeListFailure,
			expectedContext:   ContextListOperationFailed,
			expectedSeverity:  SeverityCritical,
			expectedPartial:   false,
			expectedTainting:  false,
		},

		// API discovery issues
		{
			name:              "unable to retrieve openapi",
			message:           "unable to retrieve openapi specification",
			expectedHasIssue:  true,
			expectedIssueType: IssueTypeUnavailableTypes,
			expectedContext:   ContextGenericFailure,
			expectedSeverity:  SeverityDegraded,
			expectedPartial:   true,
			expectedTainting:  true,
		},
		{
			name:              "unable to retrieve discovery",
			message:           "unable to retrieve discovery information",
			expectedHasIssue:  true,
			expectedIssueType: IssueTypeUnavailableTypes,
			expectedContext:   ContextGenericFailure,
			expectedSeverity:  SeverityDegraded,
			expectedPartial:   true,
			expectedTainting:  true,
		},

		// Connection failures
		{
			name:              "connection refused",
			message:           "connection refused",
			expectedHasIssue:  true,
			expectedIssueType: IssueTypeConnectionFailure,
			expectedContext:   ContextGenericFailure,
			expectedSeverity:  SeverityCritical,
			expectedPartial:   false,
			expectedTainting:  false,
		},
		{
			name:              "connection reset by peer",
			message:           "connection reset by peer",
			expectedHasIssue:  true,
			expectedIssueType: IssueTypeTimeout,
			expectedContext:   ContextGenericFailure,
			expectedSeverity:  SeverityDegraded,
			expectedPartial:   true,
			expectedTainting:  true,
		},
		{
			name:              "i/o timeout",
			message:           "i/o timeout",
			expectedHasIssue:  true,
			expectedIssueType: IssueTypeTimeout,
			expectedContext:   ContextGenericFailure,
			expectedSeverity:  SeverityDegraded,
			expectedPartial:   true,
			expectedTainting:  true,
		},
		{
			name:              "context deadline exceeded",
			message:           "context deadline exceeded",
			expectedHasIssue:  true,
			expectedIssueType: IssueTypeTimeout,
			expectedContext:   ContextGenericFailure,
			expectedSeverity:  SeverityDegraded,
			expectedPartial:   true,
			expectedTainting:  true,
		},

		// Normal messages
		{
			name:              "normal success message",
			message:           "application sync completed successfully",
			expectedHasIssue:  false,
			expectedIssueType: IssueTypeUnknown,
			expectedContext:   ContextGenericFailure,
			expectedSeverity:  SeverityNone,
			expectedPartial:   false,
			expectedTainting:  false,
		},
		{
			name:              "empty message",
			message:           "",
			expectedHasIssue:  false,
			expectedIssueType: IssueTypeUnknown,
			expectedContext:   ContextGenericFailure,
			expectedSeverity:  SeverityNone,
			expectedPartial:   false,
			expectedTainting:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := AnalyzeMessage(tt.message)
			assert.Equal(t, tt.expectedHasIssue, analysis.HasIssue, "HasIssue mismatch")
			assert.Equal(t, tt.expectedIssueType, analysis.IssueType, "IssueType mismatch")
			assert.Equal(t, tt.expectedContext, analysis.ErrorContext, "ErrorContext mismatch")
			assert.Equal(t, tt.expectedSeverity, analysis.Severity, "Severity mismatch")
			assert.Equal(t, tt.expectedPartial, analysis.IsPartialCache, "IsPartialCache mismatch")
			assert.Equal(t, tt.expectedTainting, analysis.IsCacheTainting, "IsCacheTainting mismatch")
			assert.Equal(t, tt.message, analysis.Message, "Message should be preserved")
		})
	}
}

func TestAnalyzeError(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		analysis := AnalyzeError(nil)
		assert.False(t, analysis.HasIssue)
		assert.Equal(t, SeverityNone, analysis.Severity)
	})

	t.Run("error with message", func(t *testing.T) {
		err := errors.New("conversion webhook failed")
		analysis := AnalyzeError(err)
		assert.True(t, analysis.HasIssue)
		assert.Equal(t, IssueTypeConversionWebhook, analysis.IssueType)
	})
}

func TestGetConditionMessage(t *testing.T) {
	tests := []struct {
		name            string
		analysis        *ClusterHealthAnalysis
		expectedMessage string
	}{
		{
			name: "no issue",
			analysis: &ClusterHealthAnalysis{
				HasIssue: false,
			},
			expectedMessage: "",
		},
		{
			name: "conversion webhook all targets failed",
			analysis: &ClusterHealthAnalysis{
				HasIssue:     true,
				IssueType:    IssueTypeConversionWebhook,
				ErrorContext: ContextAllTargetsFailed,
				Message:      "test error",
			},
			expectedMessage: "This application cannot be synced because it contains resources with conversion webhook failures: test error. Check cluster status for more details about the affected resource types.",
		},
		{
			name: "conversion webhook list operation failed",
			analysis: &ClusterHealthAnalysis{
				HasIssue:     true,
				IssueType:    IssueTypeConversionWebhook,
				ErrorContext: ContextListOperationFailed,
				Message:      "test error",
			},
			expectedMessage: "Failed to list resources due to conversion webhook error: test error. Check cluster status for more details about the affected resource types.",
		},
		{
			name: "conversion webhook generic",
			analysis: &ClusterHealthAnalysis{
				HasIssue:     true,
				IssueType:    IssueTypeConversionWebhook,
				ErrorContext: ContextGenericFailure,
				Message:      "test error",
			},
			expectedMessage: "Application contains resources affected by conversion webhook failures. Some resources may not be properly synchronized. Check cluster status for more details about the affected resource types.",
		},
		{
			name: "resource expired",
			analysis: &ClusterHealthAnalysis{
				HasIssue:  true,
				IssueType: IssueTypeResourceExpired,
				Message:   "version too old",
			},
			expectedMessage: "Resource version expired: version too old. This is a transient error that should resolve on retry.",
		},
		{
			name: "list failure",
			analysis: &ClusterHealthAnalysis{
				HasIssue:  true,
				IssueType: IssueTypeListFailure,
				Message:   "cannot list",
			},
			expectedMessage: "Failed to list resources: cannot list",
		},
		{
			name: "unavailable types",
			analysis: &ClusterHealthAnalysis{
				HasIssue:  true,
				IssueType: IssueTypeUnavailableTypes,
				Message:   "2 types unavailable",
			},
			expectedMessage: "Some resource types are unavailable: 2 types unavailable",
		},
		{
			name: "connection failure",
			analysis: &ClusterHealthAnalysis{
				HasIssue:  true,
				IssueType: IssueTypeConnectionFailure,
				Message:   "connection refused",
			},
			expectedMessage: "Connection failure: connection refused",
		},
		{
			name: "timeout",
			analysis: &ClusterHealthAnalysis{
				HasIssue:  true,
				IssueType: IssueTypeTimeout,
				Message:   "timed out",
			},
			expectedMessage: "Operation timed out: timed out",
		},
		{
			name: "unknown",
			analysis: &ClusterHealthAnalysis{
				HasIssue:  true,
				IssueType: IssueTypeUnknown,
				Message:   "some error",
			},
			expectedMessage: "Error retrieving live state: some error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := tt.analysis.GetConditionMessage()
			assert.Equal(t, tt.expectedMessage, message)
		})
	}
}

func TestIsPartialCacheError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "conversion webhook error",
			err:      errors.New("conversion webhook failed"),
			expected: true,
		},
		{
			name:     "expired resource version",
			err:      errors.New("Expired: too old resource version"),
			expected: true,
		},
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: false,
		},
		{
			name:     "i/o timeout",
			err:      errors.New("i/o timeout"),
			expected: true,
		},
		{
			name:     "normal error",
			err:      errors.New("something went wrong"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPartialCacheError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsCacheTaintingError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "conversion webhook error",
			err:      errors.New("conversion webhook failed"),
			expected: true,
		},
		{
			name:     "unavailable resource types",
			err:      errors.New("found 2 unavailable resource types"),
			expected: true,
		},
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: false,
		},
		{
			name:     "list failure without conversion",
			err:      errors.New("failed to list resources"),
			expected: false,
		},
		{
			name:     "list failure with conversion",
			err:      errors.New("failed to list resources: conversion error"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCacheTaintingError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsConversionWebhookError(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected bool
	}{
		{
			name:     "nil error",
			input:    error(nil),
			expected: false,
		},
		{
			name:     "conversion webhook error",
			input:    errors.New("conversion webhook failed"),
			expected: true,
		},
		{
			name:     "string with conversion webhook",
			input:    "conversion webhook error detected",
			expected: true,
		},
		{
			name:     "non-webhook error",
			input:    errors.New("connection refused"),
			expected: false,
		},
		{
			name:     "non-webhook string",
			input:    "normal message",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "invalid type",
			input:    123,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsConversionWebhookError(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetIssueTypes(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected []ClusterHealthIssueType
	}{
		{
			name:     "conversion webhook error",
			message:  "conversion webhook failed",
			expected: []ClusterHealthIssueType{IssueTypeConversionWebhook},
		},
		{
			name:     "message with tainted",
			message:  "cluster cache is tainted",
			expected: []ClusterHealthIssueType{IssueTypeTaintedResources},
		},
		{
			name:     "conversion webhook with tainted",
			message:  "conversion webhook failed and cache is tainted",
			expected: []ClusterHealthIssueType{IssueTypeConversionWebhook, IssueTypeTaintedResources},
		},
		{
			name:     "normal message",
			message:  "everything is fine",
			expected: []ClusterHealthIssueType{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetIssueTypes(tt.message)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestExtractGVK(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "full GVK",
			message:  `error with Group:"example.com" Version:"v1" Kind:"Example"`,
			expected: "example.com/v1, Kind=Example",
		},
		{
			name:     "GVK without group",
			message:  `error with Group:"" Version:"v1" Kind:"Pod"`,
			expected: "/v1, Kind=Pod",
		},
		{
			name:     "no GVK in message",
			message:  "normal error message",
			expected: "",
		},
		{
			name:     "lowercase fields",
			message:  `error with group:"apps" version:"v1" kind:"Deployment"`,
			expected: "apps/v1, Kind=Deployment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractGVK(tt.message)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCredentialsConfigurationError(t *testing.T) {
	originalErr := errors.New("original credential error")

	t.Run("NewCredentialsConfigurationError creates wrapped error", func(t *testing.T) {
		wrappedErr := NewCredentialsConfigurationError(originalErr)
		require.Error(t, wrappedErr)
		assert.Equal(t, "original credential error", wrappedErr.Error())
	})

	t.Run("IsCredentialsConfigurationError detects wrapped error", func(t *testing.T) {
		wrappedErr := NewCredentialsConfigurationError(originalErr)
		assert.True(t, IsCredentialsConfigurationError(wrappedErr))
	})

	t.Run("IsCredentialsConfigurationError returns false for regular error", func(t *testing.T) {
		regularErr := errors.New("regular error")
		assert.False(t, IsCredentialsConfigurationError(regularErr))
	})

	t.Run("IsCredentialsConfigurationError returns false for nil", func(t *testing.T) {
		assert.False(t, IsCredentialsConfigurationError(nil))
	})
}

func TestAppendUnique(t *testing.T) {
	tests := []struct {
		name         string
		initialTypes []ClusterHealthIssueType
		addType      ClusterHealthIssueType
		expected     []ClusterHealthIssueType
	}{
		{
			name:         "append to empty slice",
			initialTypes: []ClusterHealthIssueType{},
			addType:      IssueTypeConversionWebhook,
			expected:     []ClusterHealthIssueType{IssueTypeConversionWebhook},
		},
		{
			name:         "append unique type",
			initialTypes: []ClusterHealthIssueType{IssueTypeConversionWebhook},
			addType:      IssueTypeTimeout,
			expected:     []ClusterHealthIssueType{IssueTypeConversionWebhook, IssueTypeTimeout},
		},
		{
			name:         "append duplicate type",
			initialTypes: []ClusterHealthIssueType{IssueTypeConversionWebhook},
			addType:      IssueTypeConversionWebhook,
			expected:     []ClusterHealthIssueType{IssueTypeConversionWebhook},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := appendUnique(tt.initialTypes, tt.addType)
			assert.Equal(t, tt.expected, result)
		})
	}
}
