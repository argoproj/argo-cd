package status

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"

	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func newApp(name string) argov1alpha1.Application {
	return argov1alpha1.Application{
		Name: name,
		Status: argov1alpha1.ApplicationStatus{
			Sync: argov1alpha1.SyncStatus{
				Status: argov1alpha1.SyncStatusCodeSynced,
			},
			Health: argov1alpha1.AppHealthStatus{
				Status: health.HealthStatusHealthy,
			},
		},
	}
}

func TestBuildResourceStatusMarksAbandonedApplications(t *testing.T) {
	apps := []argov1alpha1.Application{newApp("app1"), newApp("app2")}
	generated := []argov1alpha1.Application{newApp("app1")}

	statusMap := BuildResourceStatus(map[string]argov1alpha1.ResourceStatus{}, apps, generated)

	assert.Len(t, statusMap, 2)
	assert.False(t, statusMap["app1"].Abandoned, "generated application must not be marked as abandoned")
	assert.True(t, statusMap["app2"].Abandoned, "application no longer generated must be marked as abandoned")
}

func TestBuildResourceStatusDoesNotMarkDeletingApplicationsAbandoned(t *testing.T) {
	deleting := newApp("app1")
	now := metav1.Now()
	deleting.DeletionTimestamp = &now

	statusMap := BuildResourceStatus(map[string]argov1alpha1.ResourceStatus{}, []argov1alpha1.Application{deleting}, nil)

	assert.False(t, statusMap["app1"].Abandoned, "application that is already being deleted must not be marked as abandoned")
}

func TestBuildResourceStatusClearsAbandonedMarkWhenGeneratedAgain(t *testing.T) {
	apps := []argov1alpha1.Application{newApp("app1")}

	statusMap := BuildResourceStatus(map[string]argov1alpha1.ResourceStatus{}, apps, nil)
	assert.True(t, statusMap["app1"].Abandoned)

	statusMap = BuildResourceStatus(statusMap, apps, apps)
	assert.False(t, statusMap["app1"].Abandoned, "abandoned mark must clear once the application is generated again")
}
