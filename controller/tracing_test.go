package controller

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"
	synccommon "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubetesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	mockrepoclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient/mocks"
	argotest "github.com/argoproj/argo-cd/v3/test"
	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
)

type panicCompareAppStateManager struct {
	AppStateManager
	panicValue any
}

func (m *panicCompareAppStateManager) CompareAppState(context.Context, *v1alpha1.Application, *v1alpha1.AppProject, []string, []v1alpha1.ApplicationSource, bool, bool, []string, bool) (*comparisonResult, error) {
	panic(m.panicValue)
}

type failingSetCacheClient struct {
	cacheutil.CacheClient
	err error
}

type fixedPhaseAppStateManager struct {
	AppStateManager
	phase synccommon.OperationPhase
}

func (m *fixedPhaseAppStateManager) SyncAppState(_ context.Context, _ *v1alpha1.Application, _ *v1alpha1.AppProject, state *v1alpha1.OperationState) {
	state.Phase = m.phase
	state.Message = "attempt completed"
}

func (c *failingSetCacheClient) Set(*cacheutil.Item) error {
	return c.err
}

func TestIsFailedOperationPhase(t *testing.T) {
	assert.False(t, isFailedOperationPhase(synccommon.OperationRunning))
	assert.False(t, isFailedOperationPhase(synccommon.OperationSucceeded))
	assert.True(t, isFailedOperationPhase(synccommon.OperationFailed))
	assert.True(t, isFailedOperationPhase(synccommon.OperationError))
}

func TestSetComparisonResultTraceAttrs(t *testing.T) {
	t.Run("single source", func(t *testing.T) {
		attrs := recordTraceAttrs(t, func(span oteltrace.Span) {
			setComparisonResultTraceAttrs(span, &comparisonResult{
				syncStatus: &v1alpha1.SyncStatus{
					Status:   v1alpha1.SyncStatusCodeSynced,
					Revision: "4f26ae8",
				},
				healthStatus:         health.HealthStatusHealthy,
				hasResolvedRevisions: true,
			})
		})

		assert.Equal(t, "Synced", attrs["argocd.app.sync.status"].AsString())
		assert.Equal(t, "Healthy", attrs["argocd.app.health.status"].AsString())
		assert.Equal(t, "4f26ae8", attrs["argocd.resolved_revision"].AsString())
		assert.NotContains(t, attrs, "argocd.resolved_revisions")
	})

	t.Run("multiple sources preserve order", func(t *testing.T) {
		attrs := recordTraceAttrs(t, func(span oteltrace.Span) {
			setComparisonResultTraceAttrs(span, &comparisonResult{
				syncStatus: &v1alpha1.SyncStatus{
					Status:    v1alpha1.SyncStatusCodeOutOfSync,
					Revisions: []string{"second-source-sha", "first-source-sha"},
				},
				healthStatus:         health.HealthStatusDegraded,
				hasResolvedRevisions: true,
			})
		})

		assert.Equal(t, "OutOfSync", attrs["argocd.app.sync.status"].AsString())
		assert.Equal(t, "Degraded", attrs["argocd.app.health.status"].AsString())
		assert.Equal(t, []string{"second-source-sha", "first-source-sha"}, attrs["argocd.resolved_revisions"].AsStringSlice())
		assert.NotContains(t, attrs, "argocd.resolved_revision")
	})

	t.Run("requested ref is not reported as resolved", func(t *testing.T) {
		attrs := recordTraceAttrs(t, func(span oteltrace.Span) {
			setComparisonResultTraceAttrs(span, &comparisonResult{
				syncStatus: &v1alpha1.SyncStatus{
					Status:   v1alpha1.SyncStatusCodeUnknown,
					Revision: "HEAD",
				},
				healthStatus: health.HealthStatusUnknown,
			})
		})

		assert.Equal(t, "Unknown", attrs["argocd.app.sync.status"].AsString())
		assert.Equal(t, "Unknown", attrs["argocd.app.health.status"].AsString())
		assert.NotContains(t, attrs, "argocd.resolved_revision")
		assert.NotContains(t, attrs, "argocd.resolved_revisions")
	})

	t.Run("nil result", func(t *testing.T) {
		attrs := recordTraceAttrs(t, func(span oteltrace.Span) {
			setComparisonResultTraceAttrs(span, nil)
		})

		assert.Empty(t, attrs)
	})

	t.Run("nil sync status", func(t *testing.T) {
		attrs := recordTraceAttrs(t, func(span oteltrace.Span) {
			setComparisonResultTraceAttrs(span, &comparisonResult{})
		})

		assert.Empty(t, attrs)
	})
}

func TestSetOperationStateTraceAttrs(t *testing.T) {
	for _, phase := range []synccommon.OperationPhase{
		synccommon.OperationRunning,
		synccommon.OperationSucceeded,
		synccommon.OperationFailed,
		synccommon.OperationError,
	} {
		t.Run(string(phase), func(t *testing.T) {
			attemptPhase := phase
			attrs := recordTraceAttrs(t, func(span oteltrace.Span) {
				setOperationStateTraceAttrs(span, &v1alpha1.OperationState{
					Phase:      phase,
					RetryCount: 2,
					Operation: v1alpha1.Operation{
						InitiatedBy: v1alpha1.OperationInitiator{Automated: true},
						Sync: &v1alpha1.SyncOperation{
							DryRun: true,
							Prune:  true,
						},
					},
				}, &attemptPhase)
			})

			assert.Equal(t, string(phase), attrs["argocd.operation.phase"].AsString())
			assert.Equal(t, string(phase), attrs["argocd.operation.attempt.phase"].AsString())
			assert.EqualValues(t, 2, attrs["argocd.operation.retry_count"].AsInt64())
			assert.True(t, attrs["argocd.operation.initiated_by.automated"].AsBool())
			assert.True(t, attrs["argocd.sync.dry_run"].AsBool())
			assert.True(t, attrs["argocd.sync.prune"].AsBool())
		})
	}

	t.Run("failed attempt scheduled for retry", func(t *testing.T) {
		attemptPhase := synccommon.OperationFailed
		attrs := recordTraceAttrs(t, func(span oteltrace.Span) {
			setOperationStateTraceAttrs(span, &v1alpha1.OperationState{
				Phase:      synccommon.OperationRunning,
				RetryCount: 1,
				Operation:  v1alpha1.Operation{Sync: &v1alpha1.SyncOperation{}},
			}, &attemptPhase)
		})

		assert.Equal(t, "Running", attrs["argocd.operation.phase"].AsString())
		assert.Equal(t, "Failed", attrs["argocd.operation.attempt.phase"].AsString())
		assert.EqualValues(t, 1, attrs["argocd.operation.retry_count"].AsInt64())
	})

	t.Run("nil state", func(t *testing.T) {
		attrs := recordTraceAttrs(t, func(span oteltrace.Span) {
			setOperationStateTraceAttrs(span, nil, nil)
		})

		assert.Empty(t, attrs)
	})

	t.Run("no sync attempt", func(t *testing.T) {
		attrs := recordTraceAttrs(t, func(span oteltrace.Span) {
			setOperationStateTraceAttrs(span, &v1alpha1.OperationState{}, nil)
		})

		assert.NotContains(t, attrs, "argocd.operation.attempt.phase")
	})
}

func TestSetSyncOperationTraceAttrs(t *testing.T) {
	attrs := recordTraceAttrs(t, func(span oteltrace.Span) {
		setSyncOperationTraceAttrs(span, &v1alpha1.OperationState{
			Operation:  v1alpha1.Operation{Sync: &v1alpha1.SyncOperation{DryRun: true, Prune: true}},
			SyncResult: &v1alpha1.SyncOperationResult{Revision: "HEAD"},
		})
	})

	assert.True(t, attrs["argocd.sync.dry_run"].AsBool())
	assert.True(t, attrs["argocd.sync.prune"].AsBool())
	assert.NotContains(t, attrs, "argocd.resolved_revision")
	assert.NotContains(t, attrs, "argocd.resolved_revisions")
}

func TestComparisonLevelTraceValue(t *testing.T) {
	testCases := map[CompareWith]string{
		ComparisonWithNothing:         "nothing",
		CompareWithRecent:             "recent",
		CompareWithLatest:             "latest",
		CompareWithLatestForceResolve: "latest_force_resolve",
		CompareWith(100):              "unknown",
	}
	for level, expected := range testCases {
		assert.Equal(t, expected, comparisonLevelTraceValue(level))
	}
}

func TestOperationReconciliationCauseAndOutcome(t *testing.T) {
	assert.Equal(t, "auto_sync", operationReconciliationCause(&v1alpha1.Application{
		Operation: &v1alpha1.Operation{InitiatedBy: v1alpha1.OperationInitiator{Automated: true}},
	}))
	assert.Equal(t, "manual", operationReconciliationCause(&v1alpha1.Application{Operation: &v1alpha1.Operation{}}))
	assert.Equal(t, "resume", operationReconciliationCause(&v1alpha1.Application{Status: v1alpha1.ApplicationStatus{
		OperationState: &v1alpha1.OperationState{Phase: synccommon.OperationRunning},
	}}))

	succeeded := synccommon.OperationSucceeded
	failed := synccommon.OperationFailed
	assert.Equal(t, "succeeded", operationReconciliationOutcome(&v1alpha1.OperationState{}, &succeeded, false))
	assert.Equal(t, "error", operationReconciliationOutcome(&v1alpha1.OperationState{}, &failed, false))
	assert.Equal(t, "retry_wait", operationReconciliationOutcome(&v1alpha1.OperationState{}, nil, true))
}

func TestRefreshReconciliationOutcome(t *testing.T) {
	original := &v1alpha1.ApplicationStatus{
		Sync:   v1alpha1.SyncStatus{Status: v1alpha1.SyncStatusCodeSynced},
		Health: v1alpha1.AppHealthStatus{Status: health.HealthStatusProgressing},
	}

	outcome, changed := refreshReconciliationOutcome(original, original.DeepCopy(), false)
	assert.Equal(t, "no_change", outcome)
	assert.False(t, changed)

	updated := original.DeepCopy()
	updated.Health.Status = health.HealthStatusHealthy
	outcome, changed = refreshReconciliationOutcome(original, updated, false)
	assert.Equal(t, "state_changed", outcome)
	assert.True(t, changed)

	revisionUpdated := original.DeepCopy()
	revisionUpdated.Sync.Revision = "new-release-revision"
	outcome, changed = refreshReconciliationOutcome(original, revisionUpdated, false)
	assert.Equal(t, "state_changed", outcome)
	assert.True(t, changed)

	outcome, changed = refreshReconciliationOutcome(original, updated, true)
	assert.Equal(t, "error", outcome)
	assert.False(t, changed)
}

func TestProcessRequestedAppOperationTracePreservesFailedAttemptBeforeRetry(t *testing.T) {
	recorder := installRecordingTracer(t)
	app := newFakeApp()
	app.Spec.Project = "invalid-project"
	app.Operation = &v1alpha1.Operation{
		Sync:  &v1alpha1.SyncOperation{},
		Retry: v1alpha1.RetryStrategy{Limit: 1},
	}
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)

	ctrl.processRequestedAppOperation(app)

	span := findRecordedSpan(t, recorder.Ended(), "controller.Operation")
	attrs := traceAttrs(span)
	assert.Equal(t, "manual", attrs["argocd.reconciliation.cause"].AsString())
	assert.Equal(t, "error", attrs["argocd.reconciliation.outcome"].AsString())
	assert.Equal(t, "Running", attrs["argocd.operation.phase"].AsString())
	assert.Equal(t, "Error", attrs["argocd.operation.attempt.phase"].AsString())
	assert.Equal(t, codes.Error, span.Status().Code)
}

func TestProcessRequestedAppOperationTraceUsesResolvedRevision(t *testing.T) {
	recorder := installRecordingTracer(t)
	app := newFakeApp()
	app.Spec.Project = "default"
	app.Operation = &v1alpha1.Operation{Sync: &v1alpha1.SyncOperation{Revision: "HEAD"}}
	ctrl := newFakeController(t.Context(), &fakeData{
		apps: []runtime.Object{app, &defaultProj},
		manifestResponses: []*apiclient.ManifestResponse{{
			Manifests: []string{},
			Revision:  "4f26ae8",
		}},
	}, nil)

	ctrl.processRequestedAppOperation(app)

	syncSpan := findRecordedSpan(t, recorder.Ended(), "controller.SyncAppState")
	assert.Equal(t, "4f26ae8", traceAttrs(syncSpan)["argocd.resolved_revision"].AsString())
	comparisonSpan := findRecordedSpan(t, recorder.Ended(), "controller.CompareAppState")
	assert.Equal(t, "4f26ae8", traceAttrs(comparisonSpan)["argocd.resolved_revision"].AsString())
	operationSpan := findRecordedSpan(t, recorder.Ended(), "controller.Operation")
	assert.NotContains(t, traceAttrs(operationSpan), "argocd.resolved_revision")
}

func TestProcessAppRefreshQueueItemTraceIncludesResult(t *testing.T) {
	recorder := installRecordingTracer(t)
	app := newFakeApp()
	reconciledAt := metav1.NewTime(time.Now().Add(-time.Second))
	app.Status = v1alpha1.ApplicationStatus{
		ReconciledAt: &reconciledAt,
		Sync: v1alpha1.SyncStatus{ComparedTo: v1alpha1.ComparedTo{
			Source:      app.Spec.GetSource(),
			Destination: app.Spec.Destination,
		}},
	}
	ctrl := newFakeController(t.Context(), &fakeData{
		apps: []runtime.Object{app, &defaultProj},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: argotest.FakeDestNamespace,
			Server:    argotest.FakeClusterURL,
			Revision:  "4f26ae8",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}, nil)
	key, err := cache.MetaNamespaceKeyFunc(app)
	require.NoError(t, err)
	ctrl.requestAppRefresh(app.Name, CompareWithLatest.Pointer(), nil)
	ctrl.appRefreshQueue.AddRateLimited(key)

	ctrl.processAppRefreshQueueItem()

	span := findRecordedSpan(t, recorder.Ended(), "controller.Refresh")
	attrs := traceAttrs(span)
	assert.Equal(t, "normal", attrs["argocd.refresh.type"].AsString())
	assert.Equal(t, "latest", attrs["argocd.refresh.comparison_level"].AsString())
	assert.Equal(t, "controller_request", attrs["argocd.reconciliation.cause"].AsString())
	assert.Equal(t, "state_changed", attrs["argocd.reconciliation.outcome"].AsString())
	assert.True(t, attrs["argocd.reconciliation.status_changed"].AsBool())
	assert.Equal(t, "4f26ae8", attrs["argocd.resolved_revision"].AsString())
	assert.Equal(t, "Synced", attrs["argocd.app.sync.status"].AsString())
	assert.Contains(t, attrs, "argocd.app.health.status")
}

func TestProcessAppRefreshQueueItemTraceIncludesStatusOnPreComparisonReturn(t *testing.T) {
	recorder := installRecordingTracer(t)
	app := newFakeApp()
	app.Spec.Project = "missing-project"
	reconciledAt := metav1.NewTime(time.Now().Add(-time.Second))
	app.Status.ReconciledAt = &reconciledAt
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
	key, err := cache.MetaNamespaceKeyFunc(app)
	require.NoError(t, err)
	ctrl.requestAppRefresh(app.Name, CompareWithLatest.Pointer(), nil)
	ctrl.appRefreshQueue.AddRateLimited(key)

	ctrl.processAppRefreshQueueItem()

	span := findRecordedSpan(t, recorder.Ended(), "controller.Refresh")
	attrs := traceAttrs(span)
	assert.Equal(t, "Unknown", attrs["argocd.app.sync.status"].AsString())
	assert.Equal(t, "Unknown", attrs["argocd.app.health.status"].AsString())
	assert.NotContains(t, attrs, "argocd.resolved_revision")
	assert.NotContains(t, attrs, "argocd.resolved_revisions")
	assert.False(t, hasRecordedSpan(recorder.Ended(), "controller.CompareAppState"))
	assert.Equal(t, "error", attrs["argocd.reconciliation.outcome"].AsString())
	assert.False(t, attrs["argocd.reconciliation.status_changed"].AsBool())
	assert.Equal(t, codes.Error, span.Status().Code)
}

func TestProcessAppRefreshQueueItemTraceDoesNotReportStaleStatusOnComparisonError(t *testing.T) {
	recorder := installRecordingTracer(t)
	app := newFakeApp()
	reconciledAt := metav1.NewTime(time.Now().Add(-time.Second))
	app.Status = v1alpha1.ApplicationStatus{
		ReconciledAt: &reconciledAt,
		Sync: v1alpha1.SyncStatus{
			Status:   v1alpha1.SyncStatusCodeSynced,
			Revision: "previous-revision",
			ComparedTo: v1alpha1.ComparedTo{
				Source:      app.Spec.GetSource(),
				Destination: app.Spec.Destination,
			},
		},
		Health: v1alpha1.AppHealthStatus{Status: health.HealthStatusHealthy},
	}
	ctrl := newFakeController(t.Context(), &fakeData{
		apps:             []runtime.Object{app, &defaultProj},
		manifestResponse: &apiclient.ManifestResponse{},
	}, errors.New("temporary repository failure"))
	key, err := cache.MetaNamespaceKeyFunc(app)
	require.NoError(t, err)
	ctrl.requestAppRefresh(app.Name, CompareWithLatest.Pointer(), nil)
	ctrl.appRefreshQueue.AddRateLimited(key)

	ctrl.processAppRefreshQueueItem()

	span := findRecordedSpan(t, recorder.Ended(), "controller.Refresh")
	attrs := traceAttrs(span)
	assert.Equal(t, "error", attrs["argocd.reconciliation.outcome"].AsString())
	assert.Equal(t, codes.Error, span.Status().Code)
	assert.NotContains(t, attrs, "argocd.app.sync.status")
	assert.NotContains(t, attrs, "argocd.app.health.status")
	assert.NotContains(t, attrs, "argocd.resolved_revision")
	assert.NotContains(t, attrs, "argocd.resolved_revisions")
}

func TestProcessAppRefreshQueueItemTraceRecordsPanic(t *testing.T) {
	recorder := installRecordingTracer(t)
	app := newFakeApp()
	reconciledAt := metav1.NewTime(time.Now().Add(-time.Second))
	app.Status = v1alpha1.ApplicationStatus{
		ReconciledAt: &reconciledAt,
		Sync: v1alpha1.SyncStatus{ComparedTo: v1alpha1.ComparedTo{
			Source:      app.Spec.GetSource(),
			Destination: app.Spec.Destination,
		}},
	}
	ctrl := newFakeController(t.Context(), &fakeData{
		apps:            []runtime.Object{app, &defaultProj},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}, nil)
	ctrl.appStateManager = &panicCompareAppStateManager{
		AppStateManager: ctrl.appStateManager,
		panicValue:      errors.New("comparison panic"),
	}
	key, err := cache.MetaNamespaceKeyFunc(app)
	require.NoError(t, err)
	ctrl.requestAppRefresh(app.Name, CompareWithLatest.Pointer(), nil)
	ctrl.appRefreshQueue.AddRateLimited(key)

	ctrl.processAppRefreshQueueItem()

	span := findRecordedSpan(t, recorder.Ended(), "controller.Refresh")
	attrs := traceAttrs(span)
	assert.Equal(t, "error", attrs["argocd.reconciliation.outcome"].AsString())
	assert.False(t, attrs["argocd.reconciliation.status_changed"].AsBool())
	assert.Equal(t, codes.Error, span.Status().Code)
	assert.Equal(t, "comparison panic", span.Status().Description)
	assert.NotEmpty(t, span.Events())
}

func TestProcessAppRefreshQueueItemTraceRecordsResourcesTreeCacheFailure(t *testing.T) {
	recorder := installRecordingTracer(t)
	app := newFakeApp()
	reconciledAt := metav1.Now()
	app.Status = v1alpha1.ApplicationStatus{
		ReconciledAt: &reconciledAt,
		Sync: v1alpha1.SyncStatus{
			Status: v1alpha1.SyncStatusCodeSynced,
			ComparedTo: v1alpha1.ComparedTo{
				Source:      app.Spec.GetSource(),
				Destination: app.Spec.Destination,
			},
		},
		Health: v1alpha1.AppHealthStatus{Status: health.HealthStatusHealthy},
	}
	ctrl := newFakeController(t.Context(), &fakeData{
		apps:            []runtime.Object{app, &defaultProj},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}, nil)
	instanceName := app.InstanceName(ctrl.namespace)
	require.NoError(t, ctrl.cache.SetAppManagedResources(instanceName, []*v1alpha1.ResourceDiff{}))
	cacheErr := errors.New("resources tree cache unavailable")
	baseCacheClient := ctrl.cache.Cache.GetClient()
	ctrl.cache.Cache.SetClient(&failingSetCacheClient{CacheClient: baseCacheClient, err: cacheErr})
	key, err := cache.MetaNamespaceKeyFunc(app)
	require.NoError(t, err)
	ctrl.requestAppRefresh(app.Name, ComparisonWithNothing.Pointer(), nil)
	ctrl.appRefreshQueue.AddRateLimited(key)

	ctrl.processAppRefreshQueueItem()

	span := findRecordedSpan(t, recorder.Ended(), "controller.Refresh")
	attrs := traceAttrs(span)
	assert.Equal(t, "error", attrs["argocd.reconciliation.outcome"].AsString())
	assert.False(t, attrs["argocd.reconciliation.status_changed"].AsBool())
	assert.Equal(t, codes.Error, span.Status().Code)
}

func TestSyncAppStateTraceRecordsAndPropagatesPanic(t *testing.T) {
	recorder := installRecordingTracer(t)
	manager := &appStateManager{}
	var recovered any
	func() {
		defer func() {
			recovered = recover()
		}()
		manager.SyncAppState(t.Context(), newFakeApp(), &defaultProj, nil)
	}()

	require.NotNil(t, recovered)
	span := findRecordedSpan(t, recorder.Ended(), "controller.SyncAppState")
	assert.Equal(t, codes.Error, span.Status().Code)
	assert.NotEmpty(t, span.Events())
}

func TestTraceAttrsDoNotExportUnfilteredApplicationMetadata(t *testing.T) {
	app := &v1alpha1.Application{
		Name:        "checkout",
		Namespace:   "argocd",
		Annotations: map[string]string{"example.com/secret": "do-not-export"},
		Operation: &v1alpha1.Operation{
			InitiatedBy: v1alpha1.OperationInitiator{Username: "alice@example.com"},
			Info:        []*v1alpha1.Info{{Name: "token", Value: "do-not-export"}},
		},
	}

	attrs := recordTraceAttrs(t, func(span oteltrace.Span) {
		setAppTraceAttrs(span, app)
		setOperationStateTraceAttrs(span, &v1alpha1.OperationState{Operation: *app.Operation}, nil)
	})

	assert.Equal(t, "checkout", attrs["argocd.app.name"].AsString())
	assert.NotContains(t, attrs, "example.com/secret")
	assert.NotContains(t, attrs, "argocd.operation.initiated_by.username")
	assert.NotContains(t, attrs, "token")
	for _, value := range attrs {
		assert.NotContains(t, value.String(), "do-not-export")
		assert.NotContains(t, value.String(), "alice@example.com")
	}
}

func TestProcessAppRefreshQueueItemTraceRecordsFullComparisonCacheFailure(t *testing.T) {
	recorder := installRecordingTracer(t)
	app := newFakeApp()
	ctrl := newFakeController(t.Context(), &fakeData{
		apps: []runtime.Object{app, &defaultProj},
		manifestResponse: &apiclient.ManifestResponse{
			Revision: "resolved-revision",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}, nil)
	baseClient := ctrl.cache.Cache.GetClient()
	ctrl.cache.Cache.SetClient(&failingSetCacheClient{CacheClient: baseClient, err: errors.New("managed resources cache unavailable")})
	key, err := cache.MetaNamespaceKeyFunc(app)
	require.NoError(t, err)
	ctrl.requestAppRefresh(app.Name, CompareWithLatest.Pointer(), nil)
	ctrl.appRefreshQueue.Add(key)

	ctrl.processAppRefreshQueueItem()

	span := findRecordedSpan(t, recorder.Ended(), "controller.Refresh")
	attrs := traceAttrs(span)
	assert.True(t, hasRecordedSpan(recorder.Ended(), "controller.CompareAppState"))
	assert.Equal(t, "error", attrs["argocd.reconciliation.outcome"].AsString())
	assert.False(t, attrs["argocd.reconciliation.status_changed"].AsBool())
	assert.Equal(t, codes.Error, span.Status().Code)
}

func TestCompareAppStateTraceRecordsMultiSourceResolution(t *testing.T) {
	for _, partialFailure := range []bool{false, true} {
		name := "all sources resolved"
		if partialFailure {
			name = "second source fails after first resolves"
		}
		t.Run(name, func(t *testing.T) {
			recorder := installRecordingTracer(t)
			app := newFakeApp()
			source := app.Spec.GetSource()
			app.Spec.Source = nil
			app.Spec.Sources = []v1alpha1.ApplicationSource{source, source}
			app.Spec.Sources[0].Path = "first"
			app.Spec.Sources[0].TargetRevision = "main"
			app.Spec.Sources[1].Path = "second"
			app.Spec.Sources[1].TargetRevision = "HEAD"
			ctrl := newFakeController(t.Context(), &fakeData{
				apps:            []runtime.Object{app, &defaultProj},
				managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
			}, nil)
			manager := ctrl.appStateManager.(*appStateManager)
			repo := &mockrepoclient.RepoServerServiceClient{}
			repo.EXPECT().GenerateManifest(mock.Anything, mock.MatchedBy(func(request *apiclient.ManifestRequest) bool {
				return request.ApplicationSource.Path == "first"
			})).Return(&apiclient.ManifestResponse{Revision: "first-resolved-sha"}, nil).Once()
			secondResponse := &apiclient.ManifestResponse{Revision: "second-resolved-sha"}
			var secondErr error
			if partialFailure {
				secondResponse = nil
				secondErr = errors.New("second source unavailable")
			}
			repo.EXPECT().GenerateManifest(mock.Anything, mock.MatchedBy(func(request *apiclient.ManifestRequest) bool {
				return request.ApplicationSource.Path == "second"
			})).Return(secondResponse, secondErr).Once()
			manager.repoClientset = &mockrepoclient.Clientset{RepoServerServiceClient: repo}

			result, err := manager.CompareAppState(t.Context(), app, &defaultProj, []string{"main", "HEAD"}, app.Spec.Sources, false, true, nil, true)

			require.NoError(t, err, "comparison errors are returned as Application conditions on this path")
			require.NotNil(t, result)
			repo.AssertExpectations(t)
			span := findRecordedSpan(t, recorder.Ended(), "controller.CompareAppState")
			attrs := traceAttrs(span)
			assert.NotContains(t, attrs, "argocd.resolved_revision")
			if partialFailure {
				assert.Equal(t, v1alpha1.SyncStatusCodeUnknown, result.syncStatus.Status)
				assert.False(t, result.hasResolvedRevisions)
				assert.NotEmpty(t, app.Status.GetConditions(map[v1alpha1.ApplicationConditionType]bool{
					v1alpha1.ApplicationConditionComparisonError: true,
				}))
				assert.NotContains(t, attrs, "argocd.resolved_revisions")
				assert.Equal(t, codes.Error, span.Status().Code)
			} else {
				assert.True(t, result.hasResolvedRevisions)
				assert.Equal(t, []string{"first-resolved-sha", "second-resolved-sha"}, attrs["argocd.resolved_revisions"].AsStringSlice())
				assert.Equal(t, codes.Unset, span.Status().Code)
			}
		})
	}
}

func TestProcessAppRefreshQueueItemTraceRecordsComparisonConditionFailure(t *testing.T) {
	recorder := installRecordingTracer(t)
	app := newFakeApp()
	ctrl := newFakeController(t.Context(), &fakeData{
		apps:             []runtime.Object{app, &defaultProj},
		manifestResponse: &apiclient.ManifestResponse{},
		managedLiveObjs:  make(map[kube.ResourceKey]*unstructured.Unstructured),
	}, errors.New("manifest generation failed"))
	key, err := cache.MetaNamespaceKeyFunc(app)
	require.NoError(t, err)
	ctrl.requestAppRefresh(app.Name, CompareWithLatestForceResolve.Pointer(), nil)
	ctrl.appRefreshQueue.Add(key)

	ctrl.processAppRefreshQueueItem()

	comparison := findRecordedSpan(t, recorder.Ended(), "controller.CompareAppState")
	assert.Equal(t, codes.Error, comparison.Status().Code)
	span := findRecordedSpan(t, recorder.Ended(), "controller.Refresh")
	attrs := traceAttrs(span)
	assert.Equal(t, "error", attrs["argocd.reconciliation.outcome"].AsString())
	assert.False(t, attrs["argocd.reconciliation.status_changed"].AsBool())
	assert.Equal(t, codes.Error, span.Status().Code)
	assert.NotContains(t, attrs, "argocd.resolved_revision")
}

func TestProcessRequestedAppOperationTraceRetryBackoffAndDisabledTracing(t *testing.T) {
	for _, recording := range []bool{true, false} {
		name := "recording"
		sampler := sdktrace.AlwaysSample()
		if !recording {
			name = "tracing disabled"
			sampler = sdktrace.NeverSample()
		}
		t.Run(name, func(t *testing.T) {
			recorder := installRecordingTracer(t, sdktrace.WithSampler(sampler))
			app := newFakeApp()
			app.Operation = &v1alpha1.Operation{
				Sync: &v1alpha1.SyncOperation{},
				Retry: v1alpha1.RetryStrategy{Limit: 1, Backoff: &v1alpha1.Backoff{
					Duration:    "1h",
					MaxDuration: "1h",
				}},
			}
			app.Status.OperationState.Operation = *app.Operation
			app.Status.OperationState.Phase = synccommon.OperationRunning
			app.Status.OperationState.RetryCount = 1
			app.Status.OperationState.FinishedAt = &metav1.Time{Time: time.Now().Add(-time.Second)}
			originalState := app.Status.OperationState.DeepCopy()
			ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app, &defaultProj}}, nil)
			queue := &recordingOperationQueue{TypedRateLimitingInterface: ctrl.appOperationQueue}
			ctrl.appOperationQueue = queue

			ctrl.processRequestedAppOperation(app)

			require.Len(t, queue.calls, 1)
			assert.Equal(t, ctrl.toAppKey(app.QualifiedName()), queue.calls[0].item)
			assert.Greater(t, queue.calls[0].delay, 50*time.Minute)
			assert.LessOrEqual(t, queue.calls[0].delay, time.Hour)
			stored, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Get(t.Context(), app.Name, metav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, originalState, stored.Status.OperationState)
			assert.NotNil(t, stored.Operation)
			assert.False(t, hasRecordedSpan(recorder.Ended(), "controller.SyncAppState"))
			if !recording {
				assert.Empty(t, recorder.Ended())
				return
			}
			span := findRecordedSpan(t, recorder.Ended(), "controller.Operation")
			attrs := traceAttrs(span)
			assert.Equal(t, "retry", attrs["argocd.reconciliation.cause"].AsString())
			assert.Equal(t, "retry_wait", attrs["argocd.reconciliation.outcome"].AsString())
			assert.Equal(t, "Running", attrs["argocd.operation.phase"].AsString())
			assert.EqualValues(t, 1, attrs["argocd.operation.retry_count"].AsInt64())
			assert.NotContains(t, attrs, "argocd.operation.attempt.phase")
			assert.Equal(t, codes.Unset, span.Status().Code)
		})
	}
}

func TestProcessRequestedAppOperationTraceRecordsConcurrentTermination(t *testing.T) {
	recorder := installRecordingTracer(t)
	app := newFakeApp()
	app.Operation = &v1alpha1.Operation{Sync: &v1alpha1.SyncOperation{}}
	app.Status.OperationState.Operation = *app.Operation
	app.Status.OperationState.Phase = synccommon.OperationRunning
	app.Status.OperationState.FinishedAt = nil
	storedApp := app.DeepCopy()
	storedApp.Status.OperationState.Phase = synccommon.OperationTerminating
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{storedApp, &defaultProj}}, nil)
	ctrl.appStateManager = &fixedPhaseAppStateManager{AppStateManager: ctrl.appStateManager, phase: synccommon.OperationRunning}
	queue := &recordingOperationQueue{TypedRateLimitingInterface: ctrl.appOperationQueue}
	ctrl.appOperationQueue = queue

	ctrl.processRequestedAppOperation(app)

	stored, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Get(t.Context(), app.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.NotNil(t, stored.Status.OperationState)
	assert.Equal(t, synccommon.OperationTerminating, stored.Status.OperationState.Phase)
	span := findRecordedSpan(t, recorder.Ended(), "controller.Operation")
	attrs := traceAttrs(span)
	assert.Equal(t, "Running", attrs["argocd.operation.attempt.phase"].AsString())
	assert.Equal(t, "Terminating", attrs["argocd.operation.phase"].AsString())
	assert.Equal(t, "terminating", attrs["argocd.reconciliation.outcome"].AsString())
	assert.Equal(t, codes.Unset, span.Status().Code)
	require.Len(t, queue.calls, 1)
	assert.Equal(t, ctrl.toAppKey(app.QualifiedName()), queue.calls[0].item)
	assert.Equal(t, appOperationMaxRequeueInterval, queue.calls[0].delay)
}

func TestProcessRequestedAppOperationTraceReportsPersistedFallback(t *testing.T) {
	cases := []struct {
		name           string
		initialPhase   synccommon.OperationPhase
		attemptPhase   synccommon.OperationPhase
		retryLimit     int64
		finalPhase     synccommon.OperationPhase
		outcome        string
		wantRetryCount int64
	}{
		{name: "successful attempt", initialPhase: synccommon.OperationRunning, attemptPhase: synccommon.OperationSucceeded, finalPhase: synccommon.OperationTerminating, outcome: "terminating"},
		{name: "new operation without prior state", attemptPhase: synccommon.OperationSucceeded, finalPhase: synccommon.OperationTerminating, outcome: "terminating"},
		{name: "failed attempt scheduled for retry", initialPhase: synccommon.OperationRunning, attemptPhase: synccommon.OperationFailed, retryLimit: 1, finalPhase: synccommon.OperationTerminating, outcome: "terminating", wantRetryCount: 1},
		{name: "failed terminating attempt", initialPhase: synccommon.OperationTerminating, attemptPhase: synccommon.OperationFailed, finalPhase: synccommon.OperationError, outcome: "error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := installRecordingTracer(t)
			app := newFakeApp()
			app.Operation = &v1alpha1.Operation{
				Sync:  &v1alpha1.SyncOperation{DryRun: true, Prune: true},
				Retry: v1alpha1.RetryStrategy{Limit: tc.retryLimit},
			}
			if tc.initialPhase == "" {
				app.Status.OperationState = nil
			} else {
				app.Status.OperationState.Operation = *app.Operation
				app.Status.OperationState.Phase = tc.initialPhase
				app.Status.OperationState.FinishedAt = nil
			}
			ctrl := newFakeControllerWithFailingPatch(t, &fakeData{apps: []runtime.Object{app, &defaultProj}}, func(action kubetesting.PatchAction) bool {
				return action.GetPatchType() == types.JSONPatchType && strings.HasPrefix(jsonPatchOperationStateMessage(action.GetPatch()), "attempt completed")
			})
			ctrl.appStateManager = &fixedPhaseAppStateManager{AppStateManager: ctrl.appStateManager, phase: tc.attemptPhase}

			ctrl.processRequestedAppOperation(app)

			stored, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Get(t.Context(), app.Name, metav1.GetOptions{})
			require.NoError(t, err)
			require.NotNil(t, stored.Status.OperationState)
			require.Equal(t, tc.finalPhase, stored.Status.OperationState.Phase)
			span := findRecordedSpan(t, recorder.Ended(), "controller.Operation")
			attrs := traceAttrs(span)
			assert.Equal(t, string(tc.attemptPhase), attrs["argocd.operation.attempt.phase"].AsString())
			assert.Equal(t, string(stored.Status.OperationState.Phase), attrs["argocd.operation.phase"].AsString())
			assert.Equal(t, tc.outcome, attrs["argocd.reconciliation.outcome"].AsString())
			assert.Equal(t, tc.wantRetryCount, attrs["argocd.operation.retry_count"].AsInt64())
			assert.True(t, attrs["argocd.sync.dry_run"].AsBool())
			assert.True(t, attrs["argocd.sync.prune"].AsBool())
		})
	}
}

func recordTraceAttrs(t *testing.T, setAttrs func(oteltrace.Span)) map[string]attribute.Value {
	t.Helper()

	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	_, span := provider.Tracer("controller-test").Start(t.Context(), "test")
	setAttrs(span)
	span.End()

	ended := recorder.Ended()
	require.Len(t, ended, 1)
	return traceAttrs(ended[0])
}

func installRecordingTracer(t *testing.T, options ...sdktrace.TracerProviderOption) *tracetest.SpanRecorder {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	options = append([]sdktrace.TracerProviderOption{sdktrace.WithSpanProcessor(recorder)}, options...)
	provider := sdktrace.NewTracerProvider(options...)
	// These tests must remain sequential while they temporarily replace the package tracer.
	originalTracer := tracer
	tracer = provider.Tracer("controller-test")
	t.Cleanup(func() {
		tracer = originalTracer
		require.NoError(t, provider.Shutdown(context.Background()))
	})
	return recorder
}

func findRecordedSpan(t *testing.T, spans []sdktrace.ReadOnlySpan, name string) sdktrace.ReadOnlySpan {
	t.Helper()
	for _, span := range spans {
		if span.Name() == name {
			return span
		}
	}
	require.FailNow(t, "span not found", name)
	return nil
}

func hasRecordedSpan(spans []sdktrace.ReadOnlySpan, name string) bool {
	for _, span := range spans {
		if span.Name() == name {
			return true
		}
	}
	return false
}

func traceAttrs(span sdktrace.ReadOnlySpan) map[string]attribute.Value {
	attrs := make(map[string]attribute.Value, len(span.Attributes()))
	for _, attr := range span.Attributes() {
		attrs[string(attr.Key)] = attr.Value
	}
	return attrs
}
