package controller

import (
	"reflect"

	synccommon "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	"go.opentelemetry.io/otel/attribute"
	otel_codes "go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func comparisonLevelTraceValue(level CompareWith) string {
	switch level {
	case ComparisonWithNothing:
		return "nothing"
	case CompareWithRecent:
		return "recent"
	case CompareWithLatest:
		return "latest"
	case CompareWithLatestForceResolve:
		return "latest_force_resolve"
	default:
		return "unknown"
	}
}

func operationReconciliationCause(app *v1alpha1.Application) string {
	if app == nil {
		return "unknown"
	}
	if isOperationInProgress(app) {
		state := app.Status.OperationState
		if state != nil && state.Phase == synccommon.OperationRunning && state.FinishedAt != nil {
			return "retry"
		}
		if state != nil && state.Phase == synccommon.OperationTerminating {
			return "termination"
		}
		return "resume"
	}
	if app.Operation != nil && app.Operation.InitiatedBy.Automated {
		return "auto_sync"
	}
	return "manual"
}

func operationReconciliationOutcome(phase synccommon.OperationPhase, requeuedForRetry bool) string {
	if requeuedForRetry {
		return "retry_wait"
	}
	switch phase {
	case synccommon.OperationSucceeded:
		return "succeeded"
	case synccommon.OperationFailed, synccommon.OperationError:
		return "error"
	case synccommon.OperationRunning:
		return "progressing"
	case synccommon.OperationTerminating:
		return "terminating"
	default:
		return "unknown"
	}
}

func refreshReconciliationOutcome(origStatus, finalStatus *v1alpha1.ApplicationStatus, failed bool) (string, bool) {
	if failed || origStatus == nil || finalStatus == nil {
		return "error", false
	}
	// SyncStatus contains the observed revision(s) and ComparedTo inputs, but no timestamps. Comparing
	// it as a value catches a new release even when both snapshots remain Synced. ReconciledAt is
	// intentionally excluded so periodic refreshes can still be classified as no_change.
	statusChanged := !reflect.DeepEqual(origStatus.Sync, finalStatus.Sync) || origStatus.Health.Status != finalStatus.Health.Status
	if statusChanged {
		return "state_changed", true
	}
	return "no_change", false
}

// operationTraceResult keeps the observed attempt and the retained operation state
// separate without changing either object for telemetry.
type operationTraceResult struct {
	phase        synccommon.OperationPhase
	outcome      string
	hasError     bool
	errorMessage string
}

func calculateOperationTraceResult(attempted, persisted *v1alpha1.OperationState, attemptPhase *synccommon.OperationPhase, requeuedForRetry bool) operationTraceResult {
	var result operationTraceResult
	attemptFailed := attemptPhase != nil && isFailedOperationPhase(*attemptPhase)
	usedFallback := !requeuedForRetry && persisted != nil && persisted != attempted

	if attempted != nil {
		if persisted == nil {
			result.hasError = true
			result.errorMessage = "operation state was not persisted"
		} else {
			result.phase = persisted.Phase
			if usedFallback {
				result.hasError = true
				result.errorMessage = persisted.Message
			}
		}
	}

	// A failed attempt remains an error when retry handling resets it to Running.
	// A persistence fallback that changes the phase takes precedence, as does the
	// absence of a known persisted state. Other outcomes follow the retained phase.
	outcomePhase := result.phase
	phaseChangedByPersistence := attempted != nil && (persisted == nil || persisted.Phase != attempted.Phase)
	if attemptFailed && !phaseChangedByPersistence {
		outcomePhase = *attemptPhase
	}
	result.outcome = operationReconciliationOutcome(outcomePhase, requeuedForRetry)

	if attemptFailed || (persisted != nil && isFailedOperationPhase(persisted.Phase)) {
		result.hasError = true
		result.errorMessage = ""
		if attempted != nil {
			result.errorMessage = attempted.Message
		}
		if usedFallback {
			result.errorMessage = persisted.Message
		}
	}
	return result
}

func finishOperationTrace(span oteltrace.Span, attempted, persisted *v1alpha1.OperationState, attemptPhase *synccommon.OperationPhase, requeuedForRetry bool) {
	defer span.End()
	if !span.IsRecording() {
		return
	}
	result := calculateOperationTraceResult(attempted, persisted, attemptPhase, requeuedForRetry)
	setOperationStateTraceAttrs(span, attempted, result.phase, attemptPhase)
	span.SetAttributes(attribute.String("argocd.reconciliation.outcome", result.outcome))
	if result.hasError {
		span.SetStatus(otel_codes.Error, result.errorMessage)
	}
}

// setComparisonResultTraceAttrs records the final result of an application comparison. Resolved
// revisions are attached only after manifest generation has resolved every source. Earlier return
// paths can still contain requested refs such as HEAD or main in the sync status.
func setComparisonResultTraceAttrs(span oteltrace.Span, result *comparisonResult) {
	if !span.IsRecording() || result == nil || result.syncStatus == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("argocd.app.sync.status", string(result.syncStatus.Status)),
		attribute.String("argocd.app.health.status", string(result.healthStatus)),
	}
	if result.hasResolvedRevisions {
		attrs = appendResolvedRevisionTraceAttrs(attrs, result.syncStatus.Revision, result.syncStatus.Revisions)
	}
	span.SetAttributes(attrs...)
}

// setApplicationStatusTraceAttrs records an explicit Application status snapshot for paths that
// intentionally return before comparison. A later comparison result can overwrite these
// attributes with its final values on paths that do perform a comparison.
func setApplicationStatusTraceAttrs(span oteltrace.Span, status *v1alpha1.ApplicationStatus) {
	if !span.IsRecording() || status == nil {
		return
	}

	span.SetAttributes(
		attribute.String("argocd.app.sync.status", string(status.Sync.Status)),
		attribute.String("argocd.app.health.status", string(status.Health.Status)),
	)
}

// setOperationStateTraceAttrs records attempt metadata with the retained phase supplied
// separately. attemptPhase is captured before retry handling can reset a failure to Running.
func setOperationStateTraceAttrs(span oteltrace.Span, state *v1alpha1.OperationState, persistedPhase synccommon.OperationPhase, attemptPhase *synccommon.OperationPhase) {
	if !span.IsRecording() || state == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.Int64("argocd.operation.retry_count", state.RetryCount),
		attribute.Bool("argocd.operation.initiated_by.automated", state.Operation.InitiatedBy.Automated),
	}
	if persistedPhase != "" {
		attrs = append(attrs, attribute.String("argocd.operation.phase", string(persistedPhase)))
	}
	if attemptPhase != nil {
		attrs = append(attrs, attribute.String("argocd.operation.attempt.phase", string(*attemptPhase)))
	}
	if syncOperation := state.Operation.Sync; syncOperation != nil {
		attrs = append(attrs,
			attribute.Bool("argocd.sync.dry_run", syncOperation.DryRun),
			attribute.Bool("argocd.sync.prune", syncOperation.Prune),
		)
	}
	span.SetAttributes(attrs...)
}

func setSyncOperationTraceAttrs(span oteltrace.Span, state *v1alpha1.OperationState) {
	if !span.IsRecording() || state == nil || state.Operation.Sync == nil {
		return
	}

	span.SetAttributes(
		attribute.Bool("argocd.sync.dry_run", state.Operation.Sync.DryRun),
		attribute.Bool("argocd.sync.prune", state.Operation.Sync.Prune),
	)
}

func setResolvedRevisionTraceAttrs(span oteltrace.Span, revision string, revisions []string) {
	if !span.IsRecording() {
		return
	}
	span.SetAttributes(appendResolvedRevisionTraceAttrs(nil, revision, revisions)...)
}

func isFailedOperationPhase(phase synccommon.OperationPhase) bool {
	return phase == synccommon.OperationFailed || phase == synccommon.OperationError
}

func appendResolvedRevisionTraceAttrs(attrs []attribute.KeyValue, revision string, revisions []string) []attribute.KeyValue {
	if revision != "" {
		return append(attrs, attribute.String("argocd.resolved_revision", revision))
	}
	if len(revisions) > 0 {
		return append(attrs, attribute.StringSlice("argocd.resolved_revisions", revisions))
	}
	return attrs
}
