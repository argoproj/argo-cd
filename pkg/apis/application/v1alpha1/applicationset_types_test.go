package v1alpha1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testAppSetCond(t ApplicationSetConditionType, msg string, lastTransitionTime *metav1.Time, status ApplicationSetConditionStatus, reason string) ApplicationSetCondition {
	return ApplicationSetCondition{
		Type:               t,
		Message:            msg,
		LastTransitionTime: lastTransitionTime,
		Status:             status,
		Reason:             reason,
	}
}

func newTestAppSet(name, namespace, repo string) *ApplicationSet {
	a := &ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: ApplicationSetSpec{
			Generators: []ApplicationSetGenerator{
				{
					Git: &GitGenerator{
						RepoURL: repo,
					},
				},
			},
		},
	}

	return a
}

func TestApplicationSetSetConditions(t *testing.T) {
	fiveMinsAgo := &metav1.Time{Time: time.Now().Add(-5 * time.Minute)}
	tenMinsAgo := &metav1.Time{Time: time.Now().Add(-10 * time.Minute)}
	tests := []struct {
		name           string
		existing       []ApplicationSetCondition
		incoming       []ApplicationSetCondition
		evaluatedTypes map[ApplicationSetConditionType]bool
		expected       []ApplicationSetCondition
		validate       func(*testing.T, *ApplicationSet)
	}{
		{
			name:     "new conditions with lastTransitionTime",
			existing: []ApplicationSetCondition{},
			incoming: []ApplicationSetCondition{
				testAppSetCond(ApplicationSetConditionErrorOccurred, "foo", fiveMinsAgo, ApplicationSetConditionStatusTrue, ApplicationSetReasonApplicationValidationError),
				testAppSetCond(ApplicationSetConditionResourcesUpToDate, "bar", tenMinsAgo, ApplicationSetConditionStatusTrue, ApplicationSetReasonApplicationSetUpToDate),
			},
			evaluatedTypes: map[ApplicationSetConditionType]bool{
				ApplicationSetConditionErrorOccurred:     true,
				ApplicationSetConditionResourcesUpToDate: true,
			},
			expected: []ApplicationSetCondition{
				testAppSetCond(ApplicationSetConditionErrorOccurred, "foo", fiveMinsAgo, ApplicationSetConditionStatusTrue, ApplicationSetReasonApplicationValidationError),
				testAppSetCond(ApplicationSetConditionResourcesUpToDate, "bar", tenMinsAgo, ApplicationSetConditionStatusTrue, ApplicationSetReasonApplicationSetUpToDate),
			},
			validate: func(t *testing.T, a *ApplicationSet) {
				assert.Equal(t, fiveMinsAgo, a.Status.Conditions[0].LastTransitionTime)
				assert.Equal(t, tenMinsAgo, a.Status.Conditions[1].LastTransitionTime)
			},
		}, {
			name:     "new conditions without lastTransitionTime",
			existing: []ApplicationSetCondition{},
			incoming: []ApplicationSetCondition{
				testAppSetCond(ApplicationSetConditionErrorOccurred, "foo", nil, ApplicationSetConditionStatusTrue, ApplicationSetReasonApplicationValidationError),
				testAppSetCond(ApplicationSetConditionResourcesUpToDate, "bar", nil, ApplicationSetConditionStatusFalse, ApplicationSetReasonApplicationSetUpToDate),
			},
			evaluatedTypes: map[ApplicationSetConditionType]bool{
				ApplicationSetConditionErrorOccurred:     true,
				ApplicationSetConditionResourcesUpToDate: true,
			},
			expected: []ApplicationSetCondition{
				testAppSetCond(ApplicationSetConditionErrorOccurred, "foo", nil, ApplicationSetConditionStatusTrue, ApplicationSetReasonApplicationValidationError),
				testAppSetCond(ApplicationSetConditionResourcesUpToDate, "bar", nil, ApplicationSetConditionStatusFalse, ApplicationSetReasonApplicationSetUpToDate),
			},
			validate: func(t *testing.T, a *ApplicationSet) {
				// SetConditions should add timestamps for new conditions.
				assert.True(t, a.Status.Conditions[0].LastTransitionTime.Time.After(fiveMinsAgo.Time))
				assert.True(t, a.Status.Conditions[1].LastTransitionTime.Time.After(fiveMinsAgo.Time))
			},
		}, {
			name: "condition cleared",
			existing: []ApplicationSetCondition{
				testAppSetCond(ApplicationSetConditionErrorOccurred, "foo", fiveMinsAgo, ApplicationSetConditionStatusTrue, ApplicationSetReasonApplicationValidationError),
				testAppSetCond(ApplicationSetConditionResourcesUpToDate, "bar", tenMinsAgo, ApplicationSetConditionStatusFalse, ApplicationSetReasonApplicationSetUpToDate),
			},
			incoming: []ApplicationSetCondition{
				testAppSetCond(ApplicationSetConditionResourcesUpToDate, "bar", tenMinsAgo, ApplicationSetConditionStatusTrue, ApplicationSetReasonApplicationSetUpToDate),
			},
			evaluatedTypes: map[ApplicationSetConditionType]bool{
				ApplicationSetConditionErrorOccurred:     true,
				ApplicationSetConditionResourcesUpToDate: true,
			},
			expected: []ApplicationSetCondition{
				testAppSetCond(ApplicationSetConditionResourcesUpToDate, "bar", tenMinsAgo, ApplicationSetConditionStatusTrue, ApplicationSetReasonApplicationSetUpToDate),
			},
			validate: func(t *testing.T, a *ApplicationSet) {
				assert.Equal(t, tenMinsAgo.Time, a.Status.Conditions[0].LastTransitionTime.Time)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testRepo := "https://github.com/org/repo"
			namespace := "test"
			a := newTestAppSet("sample-app-set", namespace, testRepo)
			a.Status.Conditions = tt.existing
			a.Status.SetConditions(tt.incoming, tt.evaluatedTypes)
			assertAppSetConditions(t, tt.expected, a.Status.Conditions)
			if tt.validate != nil {
				tt.validate(t, a)
			}
		})
	}
}

func assertAppSetConditions(t *testing.T, expected []ApplicationSetCondition, actual []ApplicationSetCondition) {
	assert.Equal(t, len(expected), len(actual))
	for i := range expected {
		assert.Equal(t, expected[i].Type, actual[i].Type)
		assert.Equal(t, expected[i].Message, actual[i].Message)
	}
}
