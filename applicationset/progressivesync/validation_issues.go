package progressivesync

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"
)

// ValidationIssues holds all validation problems detected during progressive sync
type ValidationIssues struct {
	InvalidMatchExpressions []InvalidMatchExpression
	DuplicateAppSelections  map[string][]int
	EmptySteps              []int // step indices (0-based) with no matching apps
	InvalidMaxUpdates       []InvalidMaxUpdate
}

// InvalidMatchExpression represents a step with an invalid matchExpression operator
type InvalidMatchExpression struct {
	StepIndex int    // 0-based step index
	Operator  string // the invalid operator value
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
	stepNums := make([]string, count)
	invalidOperators := make([]string, count)
	for i, invalidMatch := range v.InvalidMatchExpressions {
		stepNums[i] += strconv.Itoa(invalidMatch.StepIndex + 1)
		invalidOperators[i] = invalidMatch.Operator
	}
	if len(stepNums) == 1 {
		return fmt.Sprintf("Step %s has invalid matchExpression operators: %s. Supported Operators are 'In' and 'NotIn'", stepNums[0], strings.Join(invalidOperators, ", "))
	}
	return fmt.Sprintf("Steps %s have invalid matchExpression operators: %s. Supported Operators are 'In' and 'NotIn'", strings.Join(stepNums, ", "), strings.Join(invalidOperators, ", "))
}

// formatDuplicateAppSelectionMessage formats error message for duplicate app selections
func (v *ValidationIssues) formatDuplicateAppSelectionMessage() string {
	appNames := []string{}
	stepNums := []string{}
	// maps are unsorted
	keys := slices.Sorted(maps.Keys(v.DuplicateAppSelections))
	for _, key := range keys {
		appNames = append(appNames, key)
		steps := v.DuplicateAppSelections[key]
		stepNumsString := make([]string, len(steps))
		for i, step := range steps {
			stepNumsString[i] = strconv.Itoa(step + 1)
		}
		stepNums = append(stepNums, strings.Join(stepNumsString, "-"))
	}
	if len(appNames) == 1 {
		return fmt.Sprintf("Application '%s' is selected by multiple steps: (%s)", appNames[0], strings.Join(stepNums, ", "))
	}
	return fmt.Sprintf("Applications '%s' are selected by multiple steps: [(%s)]", strings.Join(appNames, "', '"), strings.Join(stepNums, "), ("))
}

// formatInvalidMaxUpdateMessage formats error message for invalid maxUpdate values
func (v *ValidationIssues) formatInvalidMaxUpdateMessage() string {
	count := len(v.InvalidMaxUpdates)
	stepNums := make([]string, count)
	values := make([]string, count)
	for i, invalidMaxUpdate := range v.InvalidMaxUpdates {
		stepNums[i] = strconv.Itoa(invalidMaxUpdate.StepIndex + 1)
		values[i] = invalidMaxUpdate.MaxUpdate.String()
	}
	if len(stepNums) == 1 {
		return fmt.Sprintf("Step %v has an invalid maxUpdate value: %v", strings.Join(stepNums, ", "), strings.Join(values, ", "))
	}
	return fmt.Sprintf("Steps %v have invalid maxUpdate values: [%v]", strings.Join(stepNums, ", "), strings.Join(values, ", "))
}

// formatEmptyStepsMessage formats warning message for empty steps
func (v *ValidationIssues) formatEmptyStepsMessage() string {
	count := len(v.EmptySteps)
	stepNums := make([]string, count)
	for i, step := range v.EmptySteps {
		stepNums[i] = strconv.Itoa(step + 1)
	}
	return fmt.Sprintf("Steps %v have no applications matching their criteria", stepNums)
}

// getConditionMessage formats condition message for the first validation Issue found. If appset has multiple vaildation issues, condition message only reports one of highest priority
func (v *ValidationIssues) getConditionMessage() string {
	var rolloutMessage string
	switch {
	case len(v.InvalidMatchExpressions) > 0:
		rolloutMessage = v.formatInvalidMatchExpressionMessage()
	case len(v.DuplicateAppSelections) > 0:
		rolloutMessage = v.formatDuplicateAppSelectionMessage()
	case len(v.InvalidMaxUpdates) > 0:
		rolloutMessage = v.formatInvalidMaxUpdateMessage()
	case len(v.EmptySteps) > 0:
		rolloutMessage = v.formatEmptyStepsMessage()
	}
	return rolloutMessage
}
