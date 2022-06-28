package application

import (
	"testing"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func TestParseResourceSyncResultErrors(t *testing.T) {
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
		assert.Equal(t, errors[0].Message, "error message")
		assert.Equal(t, errors[0].Type, "sync")
		assert.Equal(t, errors[0].Level, "error")
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

		assert.Len(t, errors, 0)
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
		assert.Equal(t, errors[0].Message, "error message")
		assert.Equal(t, errors[0].Type, "sync")
		assert.Equal(t, errors[0].Level, "error")
	})
}

func TestParseApplicationSyncResultErrorsFromConditions(t *testing.T) {
	t.Run("conditions exists", func(t *testing.T) {
		errors := parseApplicationSyncResultErrorsFromConditions([]v1alpha1.ApplicationCondition{
			{
				Type:    "error",
				Message: "error message",
			},
		})

		assert.Len(t, errors, 1)
		assert.Equal(t, errors[0].Message, "error message")
		assert.Equal(t, errors[0].Type, "sync")
		assert.Equal(t, errors[0].Level, "error")
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
		assert.Len(t, errs, 0)
	})
}
