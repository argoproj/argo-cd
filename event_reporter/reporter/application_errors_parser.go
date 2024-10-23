package reporter

import (
	"fmt"
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

var (
	syncTaskUnsuccessfullErrorMessage = "one or more synchronization tasks completed unsuccessfully"
	syncTaskNotValidErrorMessage      = "one or more synchronization tasks are not valid"
)

func parseApplicationSyncResultErrorsFromConditions(status appv1.ApplicationStatus) []*events.ObjectError {
	var errs []*events.ObjectError
	if status.Conditions == nil {
		return errs
	}
	for _, cnd := range status.Conditions {
		if (strings.Contains(cnd.Message, syncTaskUnsuccessfullErrorMessage) || strings.Contains(cnd.Message, syncTaskNotValidErrorMessage)) && status.OperationState != nil && status.OperationState.SyncResult != nil && status.OperationState.SyncResult.Resources != nil {
			resourcesSyncErrors := parseAggregativeResourcesSyncErrors(status.OperationState.SyncResult.Resources)

			errs = append(errs, resourcesSyncErrors...)
			continue
		}

		if level := getConditionLevel(cnd); level != "" {
			errs = append(errs, &events.ObjectError{
				Type:     "sync",
				Level:    level,
				Message:  cnd.Message,
				LastSeen: getConditionTime(cnd),
			})
		}
	}
	return errs
}

func getConditionLevel(cnd appv1.ApplicationCondition) string {
	if cnd.IsWarning() {
		return "warning"
	}
	if cnd.IsError() {
		return "error"
	}
	return ""
}

func getConditionTime(cnd appv1.ApplicationCondition) metav1.Time {
	if cnd.LastTransitionTime != nil {
		return *cnd.LastTransitionTime
	}
	return metav1.Now()
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

func parseAggregativeHealthErrorsOfApplication(a *appv1.Application, appTree *appv1.ApplicationTree) []*events.ObjectError {
	var errors []*events.ObjectError
	if a.Status.Resources == nil {
		return errors
	}

	for _, rs := range a.Status.Resources {
		if rs.Health != nil {
			if rs.Health.Status != health.HealthStatusHealthy {
				errors = append(errors, parseAggregativeHealthErrors(&rs, appTree, true)...)
			}
		}
	}

	return errors
}

func parseAggregativeHealthErrors(rs *appv1.ResourceStatus, apptree *appv1.ApplicationTree, addReference bool) []*events.ObjectError {
	errs := make([]*events.ObjectError, 0)

	if apptree == nil {
		return errs
	}

	n := apptree.FindNode(rs.Group, rs.Kind, rs.Namespace, rs.Name)
	if n == nil {
		return errs
	}

	childNodes := n.GetAllChildNodes(apptree, "")

	for _, cn := range childNodes {
		if cn.Health != nil && cn.Health.Status == health.HealthStatusDegraded {
			newErr := events.ObjectError{
				Type:     "health",
				Level:    "error",
				Message:  cn.Health.Message,
				LastSeen: *cn.CreatedAt,
			}

			if addReference {
				newErr.SourceReference = &events.ErrorSourceReference{
					Group:     rs.Group,
					Version:   rs.Version,
					Kind:      rs.Kind,
					Namespace: rs.Namespace,
					Name:      rs.Name,
				}
			}

			errs = append(errs, &newErr)
		}
	}

	return errs
}

func parseAggregativeResourcesSyncErrors(resourceResults appv1.ResourceResults) []*events.ObjectError {
	var errs []*events.ObjectError

	if resourceResults == nil {
		return errs
	}

	for _, rr := range resourceResults {
		if rr.Message != "" {
			objectError := events.ObjectError{
				Type:     "sync",
				Level:    "error",
				LastSeen: metav1.Now(),
				Message:  fmt.Sprintf("Resource %s(%s): \n %s", rr.Kind, rr.Name, rr.Message),
			}
			if rr.Status == common.ResultCodeSyncFailed {
				errs = append(errs, &objectError)
			}
			if rr.HookPhase == common.OperationFailed || rr.HookPhase == common.OperationError {
				errs = append(errs, &objectError)
			}
		}
	}

	return errs
}
