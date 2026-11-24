package controller

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"unicode/utf8"

	synccommon "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	"go.opentelemetry.io/otel/attribute"
	otel_codes "go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// These error classes are chosen at the failure site and documented in the observability guide.
const (
	errorTypeApplicationValidationFailed     = "application_validation_failed"
	errorTypeAutoSyncInitiationFailed        = "auto_sync_initiation_failed"
	errorTypeCacheFailed                     = "cache_failed"
	errorTypeComparisonFailed                = "comparison_failed"
	errorTypeDestinationLookupFailed         = "destination_lookup_failed"
	errorTypeOperationStatePersistenceFailed = "operation_state_persistence_failed"
	errorTypePanic                           = "panic"
	errorTypeResourceTreeFailed              = "resource_tree_failed"
	errorTypeStatusPersistenceFailed         = "status_persistence_failed"
	errorTypeSyncFailed                      = "sync_failed"

	controllerSpanErrorMessageLimit = 1024
	controllerPanicStackTraceLimit  = 2048
)

func boundedControllerErrorMessage(message string) string {
	const suffix = "..."
	if len(message) > controllerSpanErrorMessageLimit {
		end := controllerSpanErrorMessageLimit - len(suffix)
		for end > 0 && !utf8.RuneStart(message[end]) {
			end--
		}
		message = message[:end] + suffix
	}
	return strings.ToValidUTF8(message, "?")
}

func setControllerSpanError(span oteltrace.Span, errorType, message string) {
	if !span.IsRecording() {
		return
	}
	if errorType == "" {
		span.SetAttributes(semconv.ErrorTypeOther)
	} else {
		span.SetAttributes(semconv.ErrorTypeKey.String(errorType))
	}
	span.SetStatus(otel_codes.Error, boundedControllerErrorMessage(message))
}

func recordControllerSpanError(span oteltrace.Span, errorType string, err error) {
	if err == nil || !span.IsRecording() {
		return
	}
	message := boundedControllerErrorMessage(err.Error())
	setControllerSpanError(span, errorType, message)
	addControllerExceptionEvent(span, err, message, "")
}

func recordControllerPanic(span oteltrace.Span, recovered any) {
	if recovered == nil || !span.IsRecording() {
		return
	}
	message := boundedControllerErrorMessage(fmt.Sprint(recovered))
	var stack [controllerPanicStackTraceLimit]byte
	n := runtime.Stack(stack[:], false)
	setControllerSpanError(span, errorTypePanic, message)
	addControllerExceptionEvent(span, recovered, message, strings.ToValidUTF8(string(stack[:n]), "?"))
}

// RecordError takes its message from the original error. Construct the exception event directly so
// it carries the bounded message while retaining the original Go type.
func addControllerExceptionEvent(span oteltrace.Span, value any, message, stack string) {
	t := reflect.TypeOf(value)
	exceptionType := t.String()
	if t.PkgPath() != "" && t.Name() != "" {
		exceptionType = t.PkgPath() + "." + t.Name()
	}
	attrs := []attribute.KeyValue{semconv.ExceptionType(exceptionType), semconv.ExceptionMessage(message)}
	if stack != "" {
		attrs = append(attrs, semconv.ExceptionStacktrace(stack))
	}
	span.AddEvent(semconv.ExceptionEventName, oteltrace.WithAttributes(attrs...))
}

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
	errorType    string
	errorMessage string
}

func calculateOperationTraceResult(attempted, persisted *v1alpha1.OperationState, attemptPhase *synccommon.OperationPhase, requeuedForRetry bool) operationTraceResult {
	var result operationTraceResult
	attemptFailed := attemptPhase != nil && isFailedOperationPhase(*attemptPhase)
	usedFallback := !requeuedForRetry && attempted != nil && persisted != nil && persisted != attempted

	if attempted != nil && persisted != nil {
		result.phase = persisted.Phase
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

	switch {
	case attempted != nil && persisted == nil:
		result.errorType = errorTypeOperationStatePersistenceFailed
		result.errorMessage = "operation state was not persisted"
	case usedFallback:
		result.errorType = errorTypeOperationStatePersistenceFailed
		result.errorMessage = persisted.Message
	case attemptFailed:
		result.errorType = errorTypeSyncFailed
		if attempted != nil {
			result.errorMessage = attempted.Message
		}
	case persisted != nil && isFailedOperationPhase(persisted.Phase):
		result.errorType = semconv.ErrorTypeOther.Value.AsString()
		if attempted != nil {
			result.errorMessage = attempted.Message
		}
	}
	return result
}

func finishOperationTrace(span oteltrace.Span, attempted, persisted *v1alpha1.OperationState, attemptPhase *synccommon.OperationPhase, requeuedForRetry, panicked bool) {
	defer span.End()
	if !span.IsRecording() {
		return
	}
	result := calculateOperationTraceResult(attempted, persisted, attemptPhase, requeuedForRetry)
	setOperationStateTraceAttrs(span, attempted, result.phase, attemptPhase)
	span.SetAttributes(attribute.String("argocd.reconciliation.outcome", result.outcome))
	if result.errorType != "" && !panicked {
		setControllerSpanError(span, result.errorType, result.errorMessage)
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
