package v1alpha1

import (
	"testing"
	"time"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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

func TestApplicationsSyncPolicy(t *testing.T) {
	assert.False(t, ApplicationsSyncPolicyCreateOnly.AllowDelete())
	assert.False(t, ApplicationsSyncPolicyCreateOnly.AllowUpdate())

	assert.False(t, ApplicationsSyncPolicyCreateUpdate.AllowDelete())
	assert.True(t, ApplicationsSyncPolicyCreateUpdate.AllowUpdate())

	assert.True(t, ApplicationsSyncPolicySync.AllowDelete())
	assert.True(t, ApplicationsSyncPolicySync.AllowUpdate())
}

func TestApplicationSetRBACName(t *testing.T) {
	testRepo := "https://github.com/org/repo"

	t.Run("Test RBAC name with namespace", func(t *testing.T) {
		namespace := "guestbook"
		a := newTestAppSet("test-appset", namespace, testRepo)
		a.Spec.Template.Spec.Project = "test"
		assert.Equal(t, "test/guestbook/test-appset", a.RBACName("argocd"))
	})

	t.Run("Test RBAC name default ns", func(t *testing.T) {
		namespace := "argocd"
		a := newTestAppSet("test-appset", namespace, testRepo)
		a.Spec.Template.Spec.Project = "test"
		assert.Equal(t, "test/test-appset", a.RBACName("argocd"))
	})

	t.Run("Test RBAC no ns", func(t *testing.T) {
		a := newTestAppSet("test-appset", "", testRepo)
		a.Spec.Template.Spec.Project = "test"
		assert.Equal(t, "test/test-appset", a.RBACName("argocd"))
	})
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
				t.Helper()
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
				t.Helper()
				// SetConditions should add timestamps for new conditions.
				assert.True(t, a.Status.Conditions[0].LastTransitionTime.After(fiveMinsAgo.Time))
				assert.True(t, a.Status.Conditions[1].LastTransitionTime.After(fiveMinsAgo.Time))
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
				t.Helper()
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
	t.Helper()
	assert.Len(t, actual, len(expected))
	for i := range expected {
		assert.Equal(t, expected[i].Type, actual[i].Type)
		assert.Equal(t, expected[i].Message, actual[i].Message)
	}
}

func TestApplicationSetCalculateHealth(t *testing.T) {
	tests := []struct {
		name           string
		conditions     []ApplicationSetCondition
		expectedHealth health.HealthStatusCode
		expectedMsg    string
	}{
		{
			name:           "no conditions returns unknown",
			conditions:     []ApplicationSetCondition{},
			expectedHealth: health.HealthStatusUnknown,
			expectedMsg:    "No status conditions found for ApplicationSet",
		},
		{
			name: "error occurred returns degraded",
			conditions: []ApplicationSetCondition{
				{Type: ApplicationSetConditionErrorOccurred, Status: ApplicationSetConditionStatusTrue, Message: "generator failed"},
			},
			expectedHealth: health.HealthStatusDegraded,
			expectedMsg:    "generator failed",
		},
		{
			name: "error occurred false does not indicate degraded",
			conditions: []ApplicationSetCondition{
				{Type: ApplicationSetConditionErrorOccurred, Status: ApplicationSetConditionStatusFalse, Message: "no error"},
				{Type: ApplicationSetConditionResourcesUpToDate, Status: ApplicationSetConditionStatusTrue, Message: "all good"},
			},
			expectedHealth: health.HealthStatusHealthy,
			expectedMsg:    "all good",
		},
		{
			name: "rollout progressing returns progressing",
			conditions: []ApplicationSetCondition{
				{Type: ApplicationSetConditionRolloutProgressing, Status: ApplicationSetConditionStatusTrue, Message: "rolling out 2/5"},
			},
			expectedHealth: health.HealthStatusProgressing,
			expectedMsg:    "rolling out 2/5",
		},
		{
			name: "resources up to date returns healthy",
			conditions: []ApplicationSetCondition{
				{Type: ApplicationSetConditionResourcesUpToDate, Status: ApplicationSetConditionStatusTrue, Message: "all applications synced"},
			},
			expectedHealth: health.HealthStatusHealthy,
			expectedMsg:    "all applications synced",
		},
		{
			name: "error takes priority over resources up to date",
			conditions: []ApplicationSetCondition{
				{Type: ApplicationSetConditionResourcesUpToDate, Status: ApplicationSetConditionStatusTrue, Message: "synced"},
				{Type: ApplicationSetConditionErrorOccurred, Status: ApplicationSetConditionStatusTrue, Message: "validation error"},
			},
			expectedHealth: health.HealthStatusDegraded,
			expectedMsg:    "validation error",
		},
		{
			name: "error takes priority over progressing",
			conditions: []ApplicationSetCondition{
				{Type: ApplicationSetConditionRolloutProgressing, Status: ApplicationSetConditionStatusTrue, Message: "rolling"},
				{Type: ApplicationSetConditionErrorOccurred, Status: ApplicationSetConditionStatusTrue, Message: "error during rollout"},
			},
			expectedHealth: health.HealthStatusDegraded,
			expectedMsg:    "error during rollout",
		},
		{
			name: "progressing takes priority over resources up to date",
			conditions: []ApplicationSetCondition{
				{Type: ApplicationSetConditionResourcesUpToDate, Status: ApplicationSetConditionStatusTrue, Message: "synced"},
				{Type: ApplicationSetConditionRolloutProgressing, Status: ApplicationSetConditionStatusTrue, Message: "rolling out"},
			},
			expectedHealth: health.HealthStatusProgressing,
			expectedMsg:    "rolling out",
		},
		{
			name: "parameters generated only returns unknown",
			conditions: []ApplicationSetCondition{
				{Type: ApplicationSetConditionParametersGenerated, Status: ApplicationSetConditionStatusTrue, Message: "params ok"},
			},
			expectedHealth: health.HealthStatusUnknown,
			expectedMsg:    "Waiting for health status to be determined",
		},
		{
			name: "all conditions false returns unknown",
			conditions: []ApplicationSetCondition{
				{Type: ApplicationSetConditionErrorOccurred, Status: ApplicationSetConditionStatusFalse},
				{Type: ApplicationSetConditionResourcesUpToDate, Status: ApplicationSetConditionStatusFalse},
			},
			expectedHealth: health.HealthStatusUnknown,
			expectedMsg:    "Waiting for health status to be determined",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := &ApplicationSetStatus{Conditions: tt.conditions}
			healthStatus := status.CalculateHealth()
			assert.Equal(t, tt.expectedHealth, healthStatus.Status)
			assert.Equal(t, tt.expectedMsg, healthStatus.Message)
		})
	}
}

func TestSetConditionsUpdatesHealth(t *testing.T) {
	testRepo := "https://github.com/org/repo"
	namespace := "test"
	a := newTestAppSet("sample-app-set", namespace, testRepo)

	// Initially no conditions, health should be unknown
	assert.Equal(t, health.HealthStatusCode(""), a.Status.Health.Status)

	// Set ResourcesUpToDate condition
	a.Status.SetConditions([]ApplicationSetCondition{
		{Type: ApplicationSetConditionResourcesUpToDate, Status: ApplicationSetConditionStatusTrue, Message: "all synced"},
	}, map[ApplicationSetConditionType]bool{
		ApplicationSetConditionResourcesUpToDate: true,
	})
	assert.Equal(t, health.HealthStatusHealthy, a.Status.Health.Status)
	assert.Equal(t, "all synced", a.Status.Health.Message)

	// Add error condition, health should become degraded
	a.Status.SetConditions([]ApplicationSetCondition{
		{Type: ApplicationSetConditionErrorOccurred, Status: ApplicationSetConditionStatusTrue, Message: "something broke"},
	}, map[ApplicationSetConditionType]bool{
		ApplicationSetConditionErrorOccurred: true,
	})
	assert.Equal(t, health.HealthStatusDegraded, a.Status.Health.Status)
	assert.Equal(t, "something broke", a.Status.Health.Message)

	// Clear error, health should return to healthy
	a.Status.SetConditions([]ApplicationSetCondition{
		{Type: ApplicationSetConditionErrorOccurred, Status: ApplicationSetConditionStatusFalse, Message: ""},
	}, map[ApplicationSetConditionType]bool{
		ApplicationSetConditionErrorOccurred: true,
	})
	assert.Equal(t, health.HealthStatusHealthy, a.Status.Health.Status)
}

func TestSCMProviderGeneratorGitlab_WillIncludeSharedProjects(t *testing.T) {
	settings := SCMProviderGeneratorGitlab{}
	assert.True(t, settings.WillIncludeSharedProjects())

	settings.IncludeSharedProjects = ptr.To(false)
	assert.False(t, settings.WillIncludeSharedProjects())

	settings.IncludeSharedProjects = ptr.To(true)
	assert.True(t, settings.WillIncludeSharedProjects())
}
