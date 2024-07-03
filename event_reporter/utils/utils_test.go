package utils

import (
	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSetHealthStatusIfMissing(t *testing.T) {
	resource := appsv1.ResourceStatus{Status: appsv1.SyncStatusCodeSynced}
	SetHealthStatusIfMissing(&resource)
	assert.Equal(t, resource.Health.Status, health.HealthStatusHealthy)
}
