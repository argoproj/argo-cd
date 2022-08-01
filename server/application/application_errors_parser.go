package application

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync/common"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/events"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func parseApplicationSyncResultErrors(os *appv1.OperationState) []*events.ObjectError {
	var errors []*events.ObjectError
	// mean that resource not found as sync result but application can contain error inside operation state itself,
	// for example app created with invalid yaml
	if os.Phase == common.OperationError || os.Phase == common.OperationFailed {
		errors = append(errors, &events.ObjectError{
			Type:     "sync",
			Level:    "error",
			Message:  os.Message,
			LastSeen: os.StartedAt,
		})
	}
	return errors
}

func parseApplicationSyncResultErrorsFromConditions(conditions []appv1.ApplicationCondition) []*events.ObjectError {
	var errs []*events.ObjectError
	for _, cnd := range conditions {
		if !strings.Contains(strings.ToLower(cnd.Type), "error") {
			continue
		}

		lastSeen := metav1.Now()
		if cnd.LastTransitionTime != nil {
			lastSeen = *cnd.LastTransitionTime
		}

		errs = append(errs, &events.ObjectError{
			Type:     "sync",
			Level:    "error",
			Message:  cnd.Message,
			LastSeen: lastSeen,
		})
	}
	return errs
}

func parseResourceSyncResultErrors(rs *appv1.ResourceStatus, os *appv1.OperationState) []*events.ObjectError {
	errors := []*events.ObjectError{}
	if os.SyncResult == nil {
		return errors
	}

	_, sr := os.SyncResult.Resources.Find(
		rs.Group,
		rs.Kind,
		rs.Namespace,
		rs.Name,
		common.SyncPhaseSync,
	)

	if sr == nil || !(sr.HookPhase == common.OperationFailed || sr.HookPhase == common.OperationError || sr.Status == common.ResultCodeSyncFailed) {
		return errors
	}

	errors = append(errors, &events.ObjectError{
		Type:     "sync",
		Level:    "error",
		Message:  sr.Message,
		LastSeen: os.StartedAt,
	})

	return errors
}

func parseAggregativeHealthErrors(rs *appv1.ResourceStatus, apptree *appv1.ApplicationTree) []*events.ObjectError {
	errs := make([]*events.ObjectError, 0)

	if apptree == nil {
		return errs
	}

	n := apptree.FindNode(rs.Group, rs.Kind, rs.Namespace, rs.Name)
	if n == nil {
		return errs
	}

	childNodes := n.GetAllChildNodes(apptree)

	for _, cn := range childNodes {
		if cn.Health != nil && cn.Health.Status == health.HealthStatusDegraded {
			errs = append(errs, &events.ObjectError{
				Type:     "health",
				Level:    "error",
				Message:  cn.Health.Message,
				LastSeen: *cn.CreatedAt,
			})
		}
	}

	return errs
}
