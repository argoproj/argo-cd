package application

import (
	"strings"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync/common"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/events"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func parseResourceSyncResultErrors(rs *appv1.ResourceStatus, os *appv1.OperationState, applicationEntity bool) []*events.ObjectError {
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

	// mean that resource not found as sync result but application can contain error inside operation state itself,
	// for example app created with invalid yaml
	if sr == nil && applicationEntity && os.Phase == common.OperationError {
		errors = append(errors, &events.ObjectError{
			Type:     "sync",
			Level:    "error",
			Message:  os.Message,
			LastSeen: os.StartedAt,
		})
		return errors
	}

	if sr == nil || !(sr.HookPhase == common.OperationFailed || sr.HookPhase == common.OperationError || sr.Status == common.ResultCodeSyncFailed) {
		return errors
	}

	for _, msg := range strings.Split(sr.Message, ",") {
		errors = append(errors, &events.ObjectError{
			Type:     "sync",
			Level:    "error",
			Message:  msg,
			LastSeen: os.StartedAt,
		})
	}

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
