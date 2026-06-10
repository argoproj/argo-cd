package progressivesync

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestFormatInvalidMatchExpressionMessage(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		issues                   ValidationIssues
		expectedConditionMessage string
	}{
		{
			name: "progressing condition when multiple steps have InvalidMatchExpressions found for RollingSync",
			issues: ValidationIssues{
				InvalidMatchExpressions: []InvalidMatchExpression{
					{
						StepIndex: 0,
						Operator:  "Invalid",
					},
					{
						StepIndex: 4,
						Operator:  "Another",
					},
				},
			},
			expectedConditionMessage: "Steps 1, 5 have invalid matchExpression operators: Invalid, Another. Supported Operators are 'In' and 'NotIn'",
		},
		{
			name: "single step has InvalidMatchExpressions found for RollingSync",
			issues: ValidationIssues{
				InvalidMatchExpressions: []InvalidMatchExpression{
					{
						StepIndex: 0,
						Operator:  "Invalid",
					},
				},
			},
			expectedConditionMessage: "Steps 1 have invalid matchExpression operators: Invalid. Supported Operators are 'In' and 'NotIn'",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			progressingConditionMessage := tc.issues.formatInvalidMatchExpressionMessage()
			assert.NotEmpty(t, progressingConditionMessage)
			assert.Equal(t, tc.expectedConditionMessage, progressingConditionMessage)
		})
	}
}

func TestFormatDuplicateAppSelectionMessage(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		issues                   ValidationIssues
		expectedConditionMessage string
	}{
		{
			name: "application duplicated in multiple steps have DuplicateAppSelections found for RollingSync",
			issues: ValidationIssues{
				DuplicateAppSelections: map[string][]int{
					"foo": {0, 1},
				},
			},
			expectedConditionMessage: "Applications '[foo]' are selected by multiple steps: [1,2]",
		},
		{
			name: "multiple applications selected in multiple steps have DuplicateAppSelections found for RollingSync",
			issues: ValidationIssues{
				DuplicateAppSelections: map[string][]int{
					"foo": {0, 1},
					"bar": {1, 2},
				},
			},
			// function sorts maps on keys to consistently test and avoid flakiness
			expectedConditionMessage: "Applications '[bar foo]' are selected by multiple steps: [2,3 1,2]",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			progressingConditionMessage := tc.issues.formatDuplicateAppSelectionMessage()
			assert.NotEmpty(t, progressingConditionMessage)
			assert.Equal(t, tc.expectedConditionMessage, progressingConditionMessage)
		})
	}
}

func TestFormatInvalidMaxUpdateMessage(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		issues                   ValidationIssues
		expectedConditionMessage string
	}{
		{
			name: "single step with invalid maxUpdate found for RollingSync",
			issues: ValidationIssues{
				InvalidMaxUpdates: []InvalidMaxUpdate{
					{
						StepIndex: 0,
						MaxUpdate: &intstr.IntOrString{
							Type:   intstr.String,
							StrVal: "Invalid",
						},
					},
				},
			},
			expectedConditionMessage: "Steps 1 have invalid maxUpdate values Invalid",
		},
		{
			name: "multiple steps with invalid maxUpdate found for RollingSync",
			issues: ValidationIssues{
				InvalidMaxUpdates: []InvalidMaxUpdate{
					{
						StepIndex: 0,
						MaxUpdate: &intstr.IntOrString{
							Type:   intstr.String,
							StrVal: "Invalid",
						},
					},
					{
						StepIndex: 4,
						MaxUpdate: &intstr.IntOrString{
							Type:   intstr.String,
							StrVal: "Another",
						},
					},
				},
			},
			expectedConditionMessage: "Steps 1, 5 have invalid maxUpdate values Invalid, Another",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			progressingConditionMessage := tc.issues.formatInvalidMaxUpdateMessage()
			assert.NotEmpty(t, progressingConditionMessage)
			assert.Equal(t, tc.expectedConditionMessage, progressingConditionMessage)
		})
	}
}

func TestFormatEmptyStepsMessage(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		issues                   ValidationIssues
		expectedConditionMessage string
	}{
		{
			name: "single step with no matching application found for RollingSync",
			issues: ValidationIssues{
				EmptySteps: []int{0},
			},
			expectedConditionMessage: "Steps [1] have no applications matching their criteria",
		},
		{
			name: "multiple steps with no matching applications found for RollingSync",
			issues: ValidationIssues{
				EmptySteps: []int{0, 4},
			},
			expectedConditionMessage: "Steps [1 5] have no applications matching their criteria",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			progressingConditionMessage := tc.issues.formatEmptyStepsMessage()
			assert.NotEmpty(t, progressingConditionMessage)
			assert.Equal(t, tc.expectedConditionMessage, progressingConditionMessage)
		})
	}
}
