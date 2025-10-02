package errors

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ClusterHealthIssueType represents different types of cluster health issues
type ClusterHealthIssueType string

const (
	IssueTypeConversionWebhook ClusterHealthIssueType = "conversion_webhook"
	IssueTypeUnavailableTypes  ClusterHealthIssueType = "unavailable_types"
	IssueTypeListFailure       ClusterHealthIssueType = "list_failure"
	IssueTypeResourceExpired   ClusterHealthIssueType = "resource_expired"
	IssueTypeTaintedResources  ClusterHealthIssueType = "tainted_resources"
	IssueTypePaginationToken   ClusterHealthIssueType = "pagination_token"
	IssueTypeConnectionFailure ClusterHealthIssueType = "connection_failure"
	IssueTypeTimeout           ClusterHealthIssueType = "timeout"
	IssueTypeUnknown           ClusterHealthIssueType = "unknown"
)

// ErrorContext provides additional context about the error
type ErrorContext string

const (
	ContextAllTargetsFailed    ErrorContext = "all_targets_failed"    // All target resources have failures
	ContextListOperationFailed ErrorContext = "list_operation_failed" // Failed during list operation
	ContextGenericFailure      ErrorContext = "generic_failure"       // Generic or unspecified failure
)

// ClusterHealthSeverity represents the severity of cluster health issues
type ClusterHealthSeverity string

const (
	SeverityNone     ClusterHealthSeverity = "none"
	SeverityWarning  ClusterHealthSeverity = "warning"
	SeverityDegraded ClusterHealthSeverity = "degraded"
	SeverityCritical ClusterHealthSeverity = "critical"
)

// ClusterHealthAnalysis contains the analysis of an error or message
type ClusterHealthAnalysis struct {
	HasIssue        bool                   `json:"hasIssue"`
	IssueType       ClusterHealthIssueType `json:"issueType"`
	ErrorContext    ErrorContext           `json:"errorContext"`
	Severity        ClusterHealthSeverity  `json:"severity"`
	IsPartialCache  bool                   `json:"isPartialCache"`
	IsCacheTainting bool                   `json:"isCacheTainting"`
	Message         string                 `json:"message"`
	ExtractedGVK    string                 `json:"extractedGVK,omitempty"`
	SuggestedAction string                 `json:"suggestedAction,omitempty"`
}

// GetConditionMessage returns an appropriate application condition message based on the error analysis
func (a *ClusterHealthAnalysis) GetConditionMessage() string {
	if !a.HasIssue {
		return ""
	}

	switch a.IssueType {
	case IssueTypeConversionWebhook:
		switch a.ErrorContext {
		case ContextAllTargetsFailed:
			return fmt.Sprintf("This application cannot be synced because it contains resources with conversion webhook failures: %v. Check cluster status for more details about the affected resource types.", a.Message)
		case ContextListOperationFailed:
			return fmt.Sprintf("Failed to list resources due to conversion webhook error: %v. Check cluster status for more details about the affected resource types.", a.Message)
		default:
			return "Application contains resources affected by conversion webhook failures. Some resources may not be properly synchronized. Check cluster status for more details about the affected resource types."
		}
	case IssueTypeResourceExpired:
		return fmt.Sprintf("Resource version expired: %v. This is a transient error that should resolve on retry.", a.Message)
	case IssueTypeListFailure:
		return fmt.Sprintf("Failed to list resources: %v", a.Message)
	case IssueTypeUnavailableTypes:
		return fmt.Sprintf("Some resource types are unavailable: %v", a.Message)
	case IssueTypeConnectionFailure:
		return fmt.Sprintf("Connection failure: %v", a.Message)
	case IssueTypeTimeout:
		return fmt.Sprintf("Operation timed out: %v", a.Message)
	default:
		return fmt.Sprintf("Error retrieving live state: %v", a.Message)
	}
}

// Pre-compiled regex patterns for better performance
var (
	// Conversion webhook related patterns
	conversionWebhookPattern = regexp.MustCompile(`(?i)conversion\s+webhook.*(?:failed|error|failures)|conversion\s+webhook\s+for.*failed`)
	webhookFailuresPattern   = regexp.MustCompile(`(?i)known\s+conversion\s+webhook\s+failures`)
	unavailableTypesPattern  = regexp.MustCompile(`(?i)unavailable\s+resource\s+types`)

	// Pagination and expiration patterns
	paginationExpiredPattern = regexp.MustCompile(`(?i)expired.*too\s+old\s+resource\s+version`)
	resourceExpiredPattern   = regexp.MustCompile(`(?i)the\s+resourceVersion.*is\s+too\s+old`)

	// List and API discovery patterns
	listFailurePattern  = regexp.MustCompile(`(?i)failed\s+to\s+list.*resources`)
	apiDiscoveryPattern = regexp.MustCompile(`(?i)unable\s+to\s+retrieve.*(?:openapi|discovery)`)

	// Connection and network patterns
	connectionRefusedPattern = regexp.MustCompile(`(?i)connection\s+refused`)
	connectionResetPattern   = regexp.MustCompile(`(?i)connection\s+reset\s+by\s+peer`)
	ioTimeoutPattern         = regexp.MustCompile(`(?i)i/o\s+timeout`)
	contextDeadlinePattern   = regexp.MustCompile(`(?i)context\s+deadline\s+exceeded`)

	// GVK extraction patterns - matches different formats:
	// 1. Group:"example.io" Version:"v1" Kind:"Example" (structured)
	gvkExtractionPattern = regexp.MustCompile(`(?:Group|group):"([^"]*)".*(?:Version|version):"([^"]*)".*(?:Kind|kind):"([^"]*)"`)
	// 2. "example.io/v1, Kind=Example" (conversion webhook format - may have quotes)
	conversionWebhookGVKPattern = regexp.MustCompile(`"?([^"\s]+)/([^"\s,]+),\s*Kind=([^"\s]+)"?`)
)

// AnalyzeError analyzes an error and returns structured health information
func AnalyzeError(err error) *ClusterHealthAnalysis {
	if err == nil {
		return &ClusterHealthAnalysis{
			HasIssue:     false,
			IssueType:    IssueTypeUnknown,
			ErrorContext: ContextGenericFailure,
			Severity:     SeverityNone,
		}
	}
	return AnalyzeMessage(err.Error())
}

// AnalyzeMessage analyzes an error message string and returns structured health information
func AnalyzeMessage(message string) *ClusterHealthAnalysis {
	if message == "" {
		return &ClusterHealthAnalysis{
			HasIssue:     false,
			IssueType:    IssueTypeUnknown,
			ErrorContext: ContextGenericFailure,
			Severity:     SeverityNone,
		}
	}

	analysis := &ClusterHealthAnalysis{
		HasIssue: true,
		Message:  message,
	}

	// Check for conversion webhook issues (highest priority)
	if conversionWebhookPattern.MatchString(message) ||
		webhookFailuresPattern.MatchString(message) {
		analysis.IssueType = IssueTypeConversionWebhook
		analysis.Severity = SeverityDegraded
		analysis.IsPartialCache = true
		analysis.IsCacheTainting = true
		analysis.SuggestedAction = "Check conversion webhooks for affected CRDs. Consider upgrading CRDs or fixing webhook endpoints."
		analysis.ExtractedGVK = ExtractGVK(message)

		// Determine the specific context
		switch {
		case strings.Contains(message, "cannot sync application because all target resources"):
			analysis.ErrorContext = ContextAllTargetsFailed
		case strings.Contains(message, "failed to list resources"):
			analysis.ErrorContext = ContextListOperationFailed
		default:
			analysis.ErrorContext = ContextGenericFailure
		}
		return analysis
	}

	// Check for unavailable resource types
	if unavailableTypesPattern.MatchString(message) {
		analysis.IssueType = IssueTypeUnavailableTypes
		analysis.ErrorContext = ContextGenericFailure
		analysis.Severity = SeverityDegraded
		analysis.IsPartialCache = true
		analysis.IsCacheTainting = true
		analysis.SuggestedAction = "Check API server availability and resource definitions."
		return analysis
	}

	// Check for pagination token expiration (recoverable)
	if paginationExpiredPattern.MatchString(message) ||
		resourceExpiredPattern.MatchString(message) {
		analysis.IssueType = IssueTypeResourceExpired
		analysis.ErrorContext = ContextGenericFailure
		analysis.Severity = SeverityWarning
		analysis.IsPartialCache = true
		analysis.IsCacheTainting = true
		analysis.SuggestedAction = "Retry the operation. This is a transient error."
		return analysis
	}

	// Check for list failures
	if listFailurePattern.MatchString(message) {
		analysis.IssueType = IssueTypeListFailure
		analysis.ErrorContext = ContextListOperationFailed
		// If it also contains conversion webhook, it's partial
		if strings.Contains(message, "conversion") {
			analysis.Severity = SeverityDegraded
			analysis.IsPartialCache = true
			analysis.IsCacheTainting = true
		} else {
			analysis.Severity = SeverityCritical
			analysis.IsPartialCache = false
		}
		analysis.ExtractedGVK = ExtractGVK(message)
		return analysis
	}

	// Check for API discovery issues
	if apiDiscoveryPattern.MatchString(message) {
		analysis.IssueType = IssueTypeUnavailableTypes
		analysis.ErrorContext = ContextGenericFailure
		analysis.Severity = SeverityDegraded
		analysis.IsPartialCache = true
		analysis.IsCacheTainting = true
		analysis.SuggestedAction = "Check API server availability and network connectivity."
		return analysis
	}

	// Check for connection failures
	if connectionRefusedPattern.MatchString(message) {
		// Connection refused is a total failure
		analysis.IssueType = IssueTypeConnectionFailure
		analysis.ErrorContext = ContextGenericFailure
		analysis.Severity = SeverityCritical
		analysis.IsPartialCache = false
		analysis.IsCacheTainting = false
		analysis.SuggestedAction = "Check cluster connectivity and network configuration."
		return analysis
	}

	// Check for timeout issues and connection resets (partial failures)
	if ioTimeoutPattern.MatchString(message) ||
		contextDeadlinePattern.MatchString(message) ||
		connectionResetPattern.MatchString(message) {
		analysis.IssueType = IssueTypeTimeout
		analysis.ErrorContext = ContextGenericFailure
		analysis.Severity = SeverityDegraded
		analysis.IsPartialCache = true // These are transient issues, so partial cache
		analysis.IsCacheTainting = true
		analysis.SuggestedAction = "Check network latency and increase timeout values if necessary."
		return analysis
	}

	// Default case - no recognized issue pattern
	// For normal messages, mark as no issue
	analysis.HasIssue = false
	analysis.IssueType = IssueTypeUnknown
	analysis.ErrorContext = ContextGenericFailure
	analysis.Severity = SeverityNone
	analysis.IsPartialCache = false
	analysis.IsCacheTainting = false
	return analysis
}

// IsPartialCacheError determines if an error should result in partial cache being returned
// This replaces the multiple implementations across the codebase
func IsPartialCacheError(err error) bool {
	if err == nil {
		return false
	}
	analysis := AnalyzeError(err)
	return analysis.IsPartialCache
}

// IsCacheTaintingError determines if an error should taint the cache for a specific GVK
func IsCacheTaintingError(err error) bool {
	if err == nil {
		return false
	}
	analysis := AnalyzeError(err)
	return analysis.IsCacheTainting
}

// IsConversionWebhookError checks if error is specifically a conversion webhook issue
func IsConversionWebhookError(input any) bool {
	var message string

	switch v := input.(type) {
	case error:
		if v == nil {
			return false
		}
		message = v.Error()
	case string:
		message = v
	default:
		return false
	}

	analysis := AnalyzeMessage(message)
	return analysis.IssueType == IssueTypeConversionWebhook
}

// extractGVK attempts to extract GVK information from error messages
// ExtractGVKObject extracts GroupVersionKind information from error messages
// and returns it as a schema.GroupVersionKind object
func ExtractGVKObject(message string) *schema.GroupVersionKind {
	// Try structured format first: Group:"example.io" Version:"v1" Kind:"Example"
	matches := gvkExtractionPattern.FindStringSubmatch(message)
	if len(matches) == 4 {
		return &schema.GroupVersionKind{
			Group:   matches[1],
			Version: matches[2],
			Kind:    matches[3],
		}
	}

	// Try conversion webhook format: example.io/v1, Kind=Example
	matches = conversionWebhookGVKPattern.FindStringSubmatch(message)
	if len(matches) == 4 {
		return &schema.GroupVersionKind{
			Group:   matches[1],
			Version: matches[2],
			Kind:    matches[3],
		}
	}

	return nil
}

// ExtractGVK extracts GroupVersionKind information from error messages
// and returns it in the standard Kubernetes GVK.String() format
func ExtractGVK(message string) string {
	gvk := ExtractGVKObject(message)
	if gvk != nil {
		return gvk.String()
	}
	return ""
}

// GetIssueTypes analyzes a message and returns all detected issue types
// This is useful when multiple issues might be present in a single message
func GetIssueTypes(message string) []ClusterHealthIssueType {
	var types []ClusterHealthIssueType

	analysis := AnalyzeMessage(message)
	if analysis.HasIssue {
		types = append(types, analysis.IssueType)
	}

	// Check for secondary issues that might be present
	if strings.Contains(message, "tainted") {
		types = appendUnique(types, IssueTypeTaintedResources)
	}

	return types
}

// appendUnique adds an issue type if it's not already present
func appendUnique(types []ClusterHealthIssueType, issueType ClusterHealthIssueType) []ClusterHealthIssueType {
	for _, t := range types {
		if t == issueType {
			return types
		}
	}
	return append(types, issueType)
}
