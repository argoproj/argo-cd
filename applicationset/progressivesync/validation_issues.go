package progressivesync

import (
	"fmt"
	"k8s.io/apimachinery/pkg/util/intstr"
	"strconv"
	"strings"
)

// ValidationIssues holds all validation problems detected during progressive sync
type ValidationIssues struct {
	InvalidMatchExpressions []InvalidMatchExpression
	DuplicateAppSelections  []DuplicateAppSelection
	EmptySteps              []int // step indices (0-based) with no matching apps
	InvalidMaxUpdates       []InvalidMaxUpdate
}

// InvalidMatchExpression represents a step with an invalid matchExpression operator
type InvalidMatchExpression struct {
	StepIndex int    // 0-based step index
	Operator  string // the invalid operator value
}

// DuplicateAppSelection represents an application selected by multiple steps
type DuplicateAppSelection struct {
	AppName string
	Step1   int // 0-based
	Step2   int // 0-based
}

// InvalidMaxUpdate represents a step with an invalid maxUpdate value
type InvalidMaxUpdate struct {
	StepIndex int
	MaxUpdate *intstr.IntOrString
	Error     string
}

// HasIssues returns true if any validation issues exist
func (v *ValidationIssues) HasIssues() bool {
	return len(v.InvalidMatchExpressions) > 0 ||
		len(v.DuplicateAppSelections) > 0 ||
		len(v.EmptySteps) > 0 ||
		len(v.InvalidMaxUpdates) > 0
}

// formatInvalidMatchExpressionMessage formats error message for invalid match expressions
func (v *ValidationIssues) formatInvalidMatchExpressionMessage() string {
	count := len(v.InvalidMatchExpressions)
	if count == 1 {
		issue := v.InvalidMatchExpressions[0]
		return fmt.Sprintf("Step %d has invalid matchExpression operator '%s' (must be 'In' or 'NotIn')",
			issue.StepIndex+1, issue.Operator)
	}
	return fmt.Sprintf("Found %d steps with invalid matchExpression operators (must be 'In' or 'NotIn')", count)
}

// formatDuplicateAppSelectionMessage formats error message for duplicate app selections
func (v *ValidationIssues) formatDuplicateAppSelectionMessage() string {
	count := len(v.DuplicateAppSelections)
	if count == 1 {
		issue := v.DuplicateAppSelections[0]
		return fmt.Sprintf("Application '%s' is selected by multiple steps (%d and %d)",
			issue.AppName, issue.Step1+1, issue.Step2+1)
	}
	return fmt.Sprintf("Found %d applications selected by multiple steps", count)
}

// formatInvalidMaxUpdateMessage formats error message for invalid maxUpdate values
func (v *ValidationIssues) formatInvalidMaxUpdateMessage() string {
	count := len(v.InvalidMaxUpdates)
	if count == 1 {
		issue := v.InvalidMaxUpdates[0]
		return fmt.Sprintf("Step %d has invalid maxUpdate value '%v': %s",
			issue.StepIndex+1, issue.MaxUpdate, issue.Error)
	}
	return fmt.Sprintf("Found %d steps with invalid maxUpdate values", count)
}

// formatEmptyStepsMessage formats warning message for empty steps
func (v *ValidationIssues) formatEmptyStepsMessage() string {
	count := len(v.EmptySteps)
	if count == 1 {
		return fmt.Sprintf("Step %d has no applications matching its criteria", v.EmptySteps[0]+1)
	}
	stepNums := make([]string, len(v.EmptySteps))
	for i, step := range v.EmptySteps {
		stepNums[i] = strconv.Itoa(step + 1)
	}
	return fmt.Sprintf("Steps %s have no applications matching their criteria", strings.Join(stepNums, ", "))
}
