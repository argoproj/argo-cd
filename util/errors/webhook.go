package errors

import "strings"

// IsConversionWebhookError checks if an error or message is related to conversion webhook failures
// This function handles both error objects and plain strings
func IsConversionWebhookError(input interface{}) bool {
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
	
	return strings.Contains(message, "conversion webhook") ||
		strings.Contains(message, "known conversion webhook failures") ||
		strings.Contains(message, "unavailable resource types")
}