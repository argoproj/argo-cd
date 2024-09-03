package reporter

import (
	"fmt"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func TestParseResourceSyncResultErrors(t *testing.T) {
	t.Run("Resource of app contain error that contain comma", func(t *testing.T) {
		errors := parseResourceSyncResultErrors(&v1alpha1.ResourceStatus{
			Group:     "group",
			Kind:      "kind",
			Namespace: "namespace",
			Name:      "name",
		}, &v1alpha1.OperationState{
			SyncResult: &v1alpha1.SyncOperationResult{
				Resources: v1alpha1.ResourceResults{
					{
						Group:     "group",
						Kind:      "kind",
						Namespace: "namespace",
						Name:      "name",
						SyncPhase: common.SyncPhaseSync,
						Message:   "error message, with comma",
						HookPhase: common.OperationFailed,
					},
					{
						Group:     "group",
						Kind:      "kind",
						Namespace: "namespace",
						Name:      "name-2",
						SyncPhase: common.SyncPhaseSync,
					},
				},
				Revision: "123",
				Source:   v1alpha1.ApplicationSource{},
			},
		})

		assert.Len(t, errors, 1)
		assert.Equal(t, "error message, with comma", errors[0].Message)
		assert.Equal(t, "sync", errors[0].Type)
		assert.Equal(t, "error", errors[0].Level)
	})
	t.Run("Resource of app contain error", func(t *testing.T) {
		errors := parseResourceSyncResultErrors(&v1alpha1.ResourceStatus{
			Group:     "group",
			Kind:      "kind",
			Namespace: "namespace",
			Name:      "name",
		}, &v1alpha1.OperationState{
			SyncResult: &v1alpha1.SyncOperationResult{
				Resources: v1alpha1.ResourceResults{
					{
						Group:     "group",
						Kind:      "kind",
						Namespace: "namespace",
						Name:      "name",
						SyncPhase: common.SyncPhaseSync,
						Message:   "error message",
						HookPhase: common.OperationFailed,
					},
					{
						Group:     "group",
						Kind:      "kind",
						Namespace: "namespace",
						Name:      "name-2",
						SyncPhase: common.SyncPhaseSync,
					},
				},
				Revision: "123",
				Source:   v1alpha1.ApplicationSource{},
			},
		})

		assert.Len(t, errors, 1)
		assert.Equal(t, "error message", errors[0].Message)
		assert.Equal(t, "sync", errors[0].Type)
		assert.Equal(t, "error", errors[0].Level)
	})
	t.Run("Resource of app not contain error", func(t *testing.T) {
		errors := parseResourceSyncResultErrors(&v1alpha1.ResourceStatus{
			Group:     "group",
			Kind:      "kind",
			Namespace: "namespace",
			Name:      "name",
		}, &v1alpha1.OperationState{
			SyncResult: &v1alpha1.SyncOperationResult{
				Resources: v1alpha1.ResourceResults{
					{
						Group:     "group",
						Kind:      "kind",
						Namespace: "namespace",
						Name:      "name",
						SyncPhase: common.SyncPhaseSync,
						Message:   "error message",
						HookPhase: common.OperationSucceeded,
					},
					{
						Group:     "group",
						Kind:      "kind",
						Namespace: "namespace",
						Name:      "name-2",
						SyncPhase: common.SyncPhaseSync,
					},
				},
				Revision: "123",
				Source:   v1alpha1.ApplicationSource{},
			},
		})

		assert.Empty(t, errors)
	})
	t.Run("App of app contain error", func(t *testing.T) {
		errors := parseApplicationSyncResultErrors(&v1alpha1.OperationState{
			Phase:   common.OperationError,
			Message: "error message",
			SyncResult: &v1alpha1.SyncOperationResult{
				Revision: "123",
				Source:   v1alpha1.ApplicationSource{},
			},
		})

		assert.Len(t, errors, 1)
		assert.Equal(t, "error message", errors[0].Message)
		assert.Equal(t, "sync", errors[0].Type)
		assert.Equal(t, "error", errors[0].Level)
	})
}

func TestParseApplicationSyncResultErrorsFromConditions(t *testing.T) {
	t.Run("conditions exists", func(t *testing.T) {
		errors := parseApplicationSyncResultErrorsFromConditions(v1alpha1.ApplicationStatus{
			Conditions: []v1alpha1.ApplicationCondition{
				{
					Type:    "error",
					Message: "error message",
				},
			},
		})

		assert.Len(t, errors, 1)
		assert.Equal(t, "error message", errors[0].Message)
		assert.Equal(t, "sync", errors[0].Type)
		assert.Equal(t, "error", errors[0].Level)
	})

	t.Run("conditions erorr replaced with sync result errors", func(t *testing.T) {
		errors := parseApplicationSyncResultErrorsFromConditions(v1alpha1.ApplicationStatus{
			Conditions: []v1alpha1.ApplicationCondition{
				{
					Type:    "error",
					Message: syncTaskUnsuccessfullErrorMessage,
				},
			},
			OperationState: &v1alpha1.OperationState{
				SyncResult: &v1alpha1.SyncOperationResult{
					Resources: v1alpha1.ResourceResults{
						&v1alpha1.ResourceResult{
							Kind:      "Job",
							Name:      "some-job",
							Message:   "job failed",
							HookPhase: common.OperationFailed,
						},
						&v1alpha1.ResourceResult{
							Kind:    "Pod",
							Name:    "some-pod",
							Message: "pod failed",
							Status:  common.ResultCodeSyncFailed,
						},
						&v1alpha1.ResourceResult{
							Kind:      "Job",
							Name:      "some-succeeded-hook",
							Message:   "job succeeded",
							HookPhase: common.OperationSucceeded,
						},
						&v1alpha1.ResourceResult{
							Kind:    "Pod",
							Name:    "synced-pod",
							Message: "pod synced",
							Status:  common.ResultCodeSynced,
						},
					},
				},
			},
		})

		assert.Len(t, errors, 2)
		assert.Equal(t, fmt.Sprintf("Resource %s(%s): \n %s", "Job", "some-job", "job failed"), errors[0].Message)
		assert.Equal(t, "sync", errors[0].Type)
		assert.Equal(t, "error", errors[0].Level)
		assert.Equal(t, fmt.Sprintf("Resource %s(%s): \n %s", "Pod", "some-pod", "pod failed"), errors[1].Message)
		assert.Equal(t, "sync", errors[1].Type)
		assert.Equal(t, "error", errors[1].Level)
	})
}

func TestParseAggregativeHealthErrors(t *testing.T) {
	t.Run("application tree is nil", func(t *testing.T) {
		errs := parseAggregativeHealthErrors(&v1alpha1.ResourceStatus{
			Group:     "group",
			Kind:      "application",
			Namespace: "namespace",
			Name:      "name",
		}, nil)
		assert.Empty(t, errs)
	})
}
