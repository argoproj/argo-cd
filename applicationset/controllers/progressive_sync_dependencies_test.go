package controllers

import (
	"testing"
	"time"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/health"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj/argo-cd/v3/applicationset/generators"
	appsetmetrics "github.com/argoproj/argo-cd/v3/applicationset/metrics"
	appsetprogressiveSync "github.com/argoproj/argo-cd/v3/applicationset/progressivesync"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

func TestUpdateApplicationSetApplicationStatus(t *testing.T) {
	nowMinus5 := metav1.Time{Time: time.Now().Add(-5 * time.Minute)}
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	newDefaultAppSet := func(stepsCount int, status []v1alpha1.ApplicationSetApplicationStatus) v1alpha1.ApplicationSet {
		steps := []v1alpha1.ApplicationSetRolloutStep{}
		for range stepsCount {
			steps = append(steps, v1alpha1.ApplicationSetRolloutStep{MatchExpressions: []v1alpha1.ApplicationMatchExpression{}})
		}
		return v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "argocd",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Strategy: &v1alpha1.ApplicationSetStrategy{
					Type: "RollingSync",
					RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
						Steps: steps,
					},
				},
			},
			Status: v1alpha1.ApplicationSetStatus{
				ApplicationStatus: status,
			},
		}
	}

	newApp := func(name string, health health.HealthStatusCode, sync v1alpha1.SyncStatusCode, revision string, opState *v1alpha1.OperationState) v1alpha1.Application {
		return v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Status: v1alpha1.ApplicationStatus{
				ReconciledAt: &metav1.Time{Time: time.Now()},
				Health: v1alpha1.AppHealthStatus{
					Status: health,
				},
				OperationState: opState,
				Sync: v1alpha1.SyncStatus{
					Status:   sync,
					Revision: revision,
				},
			},
		}
	}

	newAppWithSpec := func(name string, health health.HealthStatusCode, sync v1alpha1.SyncStatusCode, revision string, opState *v1alpha1.OperationState, spec v1alpha1.ApplicationSpec) v1alpha1.Application {
		app := newApp(name, health, sync, revision, opState)
		app.Spec = spec
		return app
	}

	newOperationState := func(phase common.OperationPhase) *v1alpha1.OperationState {
		finishedAt := &metav1.Time{Time: time.Now().Add(-1 * time.Second)}
		if !phase.Completed() {
			finishedAt = nil
		}
		return &v1alpha1.OperationState{
			Phase:      phase,
			StartedAt:  metav1.Time{Time: time.Now().Add(-1 * time.Minute)},
			FinishedAt: finishedAt,
		}
	}

	for _, cc := range []struct {
		name              string
		appSet            v1alpha1.ApplicationSet
		apps              []v1alpha1.Application
		desiredApps       []v1alpha1.Application
		appStepMap        map[string]int
		expectedAppStatus []v1alpha1.ApplicationSetApplicationStatus
	}{
		{
			name:              "handles a nil list of statuses and no applications",
			appSet:            newDefaultAppSet(2, nil),
			apps:              []v1alpha1.Application{},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{},
		},
		{
			name:   "handles a nil list of statuses with a healthy application",
			appSet: newDefaultAppSet(2, nil),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeSynced, "next", newOperationState(common.OperationSucceeded)),
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource has synced, updating status to Healthy",
					Status:          v1alpha1.ProgressiveSyncHealthy,
					Step:            "1",
					TargetRevisions: []string{"next"},
				},
			},
		},
		{
			name:   "moves a new application to healthy when app is synced and healthy",
			appSet: newDefaultAppSet(2, []v1alpha1.ApplicationSetApplicationStatus{}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeSynced, "current", newOperationState(common.OperationSucceeded)),
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource has synced, updating status to Healthy",
					Status:          v1alpha1.ProgressiveSyncHealthy,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
			},
		},
		{
			name: "moves a waiting application to healthy when app is synced and healthy",
			appSet: newDefaultAppSet(2, []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					Message:            "",
					Status:             v1alpha1.ProgressiveSyncWaiting,
					Step:               "1",
					TargetRevisions:    []string{"current"},
					LastTransitionTime: &nowMinus5,
				},
			}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeSynced, "current", newOperationState(common.OperationSucceeded)),
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource has synced, updating status to Healthy",
					Status:          v1alpha1.ProgressiveSyncHealthy,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
			},
		},
		{
			name:   "moves a new application to progressing when app is synced but not healthy",
			appSet: newDefaultAppSet(2, []v1alpha1.ApplicationSetApplicationStatus{}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusDegraded, v1alpha1.SyncStatusCodeSynced, "current", newOperationState(common.OperationSucceeded)),
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource has synced, updating status to Progressing",
					Status:          v1alpha1.ProgressiveSyncProgressing,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
			},
		},
		{
			name: "moves an application with new revision to Healthy when it is not OutOfSync",
			appSet: newDefaultAppSet(2, []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					Message:            "Application resource has synced, updating status to Healthy",
					Status:             v1alpha1.ProgressiveSyncHealthy,
					Step:               "1",
					TargetRevisions:    []string{"previous"},
					LastTransitionTime: &nowMinus5,
				},
			}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeSynced, "next", newOperationState(common.OperationSucceeded)),
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource has synced, updating status to Healthy",
					Status:          v1alpha1.ProgressiveSyncHealthy,
					Step:            "1",
					TargetRevisions: []string{"next"},
				},
			},
		},
		{
			name: "moves an application with new version to waiting when it is OutOfSync",
			appSet: newDefaultAppSet(2, []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					Message:            "",
					Status:             v1alpha1.ProgressiveSyncHealthy,
					Step:               "1",
					TargetRevisions:    []string{"previous"},
					LastTransitionTime: &nowMinus5,
				},
				{
					Application:        "app2-multisource",
					Message:            "",
					Status:             v1alpha1.ProgressiveSyncHealthy,
					Step:               "1",
					TargetRevisions:    []string{"previous", "removed-source"},
					LastTransitionTime: &nowMinus5,
				},
			}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeOutOfSync, "next", nil),
				newApp("app2-multisource", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeOutOfSync, "next", nil),
			},
			appStepMap: map[string]int{
				"app1":             0,
				"app2-multisource": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         revisionChangedMsg,
					Status:          v1alpha1.ProgressiveSyncWaiting,
					Step:            "1",
					TargetRevisions: []string{"next"},
				},
				{
					Application:     "app2-multisource",
					Message:         revisionChangedMsg,
					Status:          v1alpha1.ProgressiveSyncWaiting,
					Step:            "1",
					TargetRevisions: []string{"next"},
				},
			},
		},
		{
			name: "does not move a Healthy application to another status if the revision has not changed",
			appSet: newDefaultAppSet(2, []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					Message:            "",
					Status:             v1alpha1.ProgressiveSyncHealthy,
					Step:               "1",
					TargetRevisions:    []string{"next"},
					LastTransitionTime: &nowMinus5,
				},
			}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeOutOfSync, "next", newOperationState(common.OperationSucceeded)),
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					Message:            "",
					Status:             v1alpha1.ProgressiveSyncHealthy,
					Step:               "1",
					TargetRevisions:    []string{"next"},
					LastTransitionTime: &nowMinus5,
				},
			},
		},
		{
			name: "moves a pending application to progressing when operation is running",
			appSet: newDefaultAppSet(2, []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					Message:            "",
					Status:             v1alpha1.ProgressiveSyncPending,
					Step:               "1",
					TargetRevisions:    []string{"next"},
					LastTransitionTime: &nowMinus5,
				},
			}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeOutOfSync, "next", newOperationState(common.OperationRunning)),
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource became Progressing, updating status from Pending to Progressing",
					Status:          v1alpha1.ProgressiveSyncProgressing,
					Step:            "1",
					TargetRevisions: []string{"next"},
				},
			},
		},
		{
			name: "moves a pending application to progressing when operation is successful",
			appSet: newDefaultAppSet(2, []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					Message:            "",
					Status:             v1alpha1.ProgressiveSyncPending,
					Step:               "1",
					TargetRevisions:    []string{"next"},
					LastTransitionTime: &nowMinus5,
				},
			}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeOutOfSync, "next", newOperationState(common.OperationSucceeded)),
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource completed a sync successfully, updating status from Pending to Progressing",
					Status:          v1alpha1.ProgressiveSyncProgressing,
					Step:            "1",
					TargetRevisions: []string{"next"},
				},
			},
		},
		{
			name: "moves a pending application to progressing when sync operation has failed",
			appSet: newDefaultAppSet(2, []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					Message:            "",
					Status:             v1alpha1.ProgressiveSyncPending,
					Step:               "1",
					TargetRevisions:    []string{"next"},
					LastTransitionTime: &nowMinus5,
				},
			}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeOutOfSync, "next", newOperationState(common.OperationFailed)),
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource completed a sync, updating status from Pending to Progressing",
					Status:          v1alpha1.ProgressiveSyncProgressing,
					Step:            "1",
					TargetRevisions: []string{"next"},
				},
			},
		},
		{
			name: "moves a pending application to progressing when sync operation had error",
			appSet: newDefaultAppSet(2, []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					Message:            "",
					Status:             v1alpha1.ProgressiveSyncPending,
					Step:               "1",
					TargetRevisions:    []string{"next"},
					LastTransitionTime: &nowMinus5,
				},
			}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeOutOfSync, "next", newOperationState(common.OperationError)),
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource completed a sync, updating status from Pending to Progressing",
					Status:          v1alpha1.ProgressiveSyncProgressing,
					Step:            "1",
					TargetRevisions: []string{"next"},
				},
			},
		},
		{
			// If an application is invalid, we move it to Progressing to avoid calling sync indefinitely.
			// It is the user responsibility to fix the error on the Application. This is different than
			// sync failures were the user is expected to configure retry as part of the sync policy.
			name: "moves a pending application with InvalidSpecError errors to progressing",
			appSet: newDefaultAppSet(2, []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					Message:            "",
					Status:             v1alpha1.ProgressiveSyncPending,
					Step:               "1",
					TargetRevisions:    []string{"next"},
					LastTransitionTime: &nowMinus5,
				},
			}),
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						ReconciledAt: nil,
						Conditions: []v1alpha1.ApplicationCondition{
							{
								Type:               v1alpha1.ApplicationConditionInvalidSpecError,
								Message:            "Fake invalid specs preventing app updates and sync to be trigerred",
								LastTransitionTime: &metav1.Time{Time: time.Now()},
							},
						},
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusUnknown,
						},
						OperationState: nil,
						Sync: v1alpha1.SyncStatus{
							Status:   v1alpha1.SyncStatusCodeUnknown,
							Revision: "next",
						},
					},
				},
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource has error and cannot sync, updating status to Progressing",
					Status:          v1alpha1.ProgressiveSyncProgressing,
					Step:            "1",
					TargetRevisions: []string{"next"},
				},
			},
		},
		{
			name: "does not move a pending application to progressing if sync happened before transition",
			appSet: newDefaultAppSet(2, []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					Message:            "",
					Status:             v1alpha1.ProgressiveSyncPending,
					Step:               "1",
					TargetRevisions:    []string{"next"},
					LastTransitionTime: &metav1.Time{Time: time.Now()},
				},
			}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusProgressing, v1alpha1.SyncStatusCodeSynced, "next", &v1alpha1.OperationState{
					Phase:      common.OperationSucceeded,
					StartedAt:  nowMinus5,
					FinishedAt: &metav1.Time{Time: nowMinus5.Add(5 * time.Second)},
				}),
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "",
					Status:          v1alpha1.ProgressiveSyncPending,
					Step:            "1",
					TargetRevisions: []string{"next"},
				},
			},
		},
		{
			name: "does not move a pending application to progressing if it has not been reconciled since transition",
			appSet: newDefaultAppSet(2, []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					Message:            "",
					Status:             v1alpha1.ProgressiveSyncPending,
					Step:               "1",
					TargetRevisions:    []string{"next"},
					LastTransitionTime: &metav1.Time{Time: time.Now().Add(-2 * time.Minute)},
				},
			}),
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						ReconciledAt: &nowMinus5, // This means data is stale and we cannot trust the information in the status.
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase:      common.OperationSucceeded,
							StartedAt:  metav1.Time{Time: time.Now().Add(-1 * time.Minute)},
							FinishedAt: &metav1.Time{Time: time.Now()},
						},
						Sync: v1alpha1.SyncStatus{
							Status:   v1alpha1.SyncStatusCodeSynced,
							Revision: "next",
						},
					},
				},
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "",
					Status:          v1alpha1.ProgressiveSyncPending,
					Step:            "1",
					TargetRevisions: []string{"next"},
				},
			},
		},
		{
			name: "moves a progressing application to healthy when it is synced and healthy",
			appSet: newDefaultAppSet(2, []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					Message:            "",
					Status:             v1alpha1.ProgressiveSyncProgressing,
					Step:               "1",
					TargetRevisions:    []string{"next"},
					LastTransitionTime: &nowMinus5,
				},
			}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeSynced, "next", newOperationState(common.OperationSucceeded)),
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource became Healthy, updating status from Progressing to Healthy",
					Status:          v1alpha1.ProgressiveSyncHealthy,
					Step:            "1",
					TargetRevisions: []string{"next"},
				},
			},
		},
		{
			name: "does not move a progressing application to healthy when it is synced and not healthy",
			appSet: newDefaultAppSet(2, []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					Message:            "",
					Status:             v1alpha1.ProgressiveSyncProgressing,
					Step:               "1",
					TargetRevisions:    []string{"next"},
					LastTransitionTime: &nowMinus5,
				},
			}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusDegraded, v1alpha1.SyncStatusCodeSynced, "next", newOperationState(common.OperationSucceeded)),
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "",
					Status:          v1alpha1.ProgressiveSyncProgressing,
					Step:            "1",
					TargetRevisions: []string{"next"},
				},
			},
		},
		{
			name: "application status is removed when applciation is deleted",
			appSet: newDefaultAppSet(2, []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "",
					Status:          v1alpha1.ProgressiveSyncPending,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
				{
					Application:     "app2",
					Message:         "",
					Status:          v1alpha1.ProgressiveSyncPending,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
			}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeOutOfSync, "current", nil),
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "",
					Status:          v1alpha1.ProgressiveSyncPending,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
			},
		},
		{
			name: "application status that is not in steps is updated to -1",
			appSet: newDefaultAppSet(2, []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "",
					Status:          v1alpha1.ProgressiveSyncPending,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
				{
					Application:     "app2",
					Message:         "",
					Status:          v1alpha1.ProgressiveSyncPending,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
			}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeOutOfSync, "current", nil),
				newApp("app2", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeOutOfSync, "current", nil),
			},
			appStepMap: map[string]int{
				"app1": 0,
				// app2 is removed from step selector
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "",
					Status:          v1alpha1.ProgressiveSyncPending,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
				{
					Application:     "app2",
					Message:         "",
					Status:          v1alpha1.ProgressiveSyncPending,
					Step:            "-1",
					TargetRevisions: []string{"current"},
				},
			},
		},
		{
			name: "update the steps of an existing status",
			appSet: newDefaultAppSet(2, []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "",
					Status:          v1alpha1.ProgressiveSyncPending,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
			}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeOutOfSync, "current", nil),
			},
			appStepMap: map[string]int{
				"app1": 1, // 1 is actually steps 2
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "",
					Status:          v1alpha1.ProgressiveSyncPending,
					Step:            "2",
					TargetRevisions: []string{"current"},
				},
			},
		},
		{
			name: "detects spec changes when image tag changes in generator (same Git revision)",
			appSet: newDefaultAppSet(2, []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "",
					Status:          v1alpha1.ProgressiveSyncHealthy,
					Step:            "1",
					TargetRevisions: []string{"abc123"},
				},
			}),
			apps: []v1alpha1.Application{
				newAppWithSpec("app1", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeOutOfSync, "abc123", nil, // Changed to OutOfSync
					v1alpha1.ApplicationSpec{
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://example.com/repo.git",
							TargetRevision: "master",
							Helm: &v1alpha1.ApplicationSourceHelm{
								Parameters: []v1alpha1.HelmParameter{
									{Name: "image.tag", Value: "v1.0.0"},
								},
							},
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "default",
						},
					}),
			},
			desiredApps: []v1alpha1.Application{
				newAppWithSpec("app1", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeOutOfSync, "abc123", nil, // Changed to OutOfSync
					v1alpha1.ApplicationSpec{
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://example.com/repo.git",
							TargetRevision: "master",
							Helm: &v1alpha1.ApplicationSourceHelm{
								Parameters: []v1alpha1.HelmParameter{
									{Name: "image.tag", Value: "v2.0.0"}, // Different value
								},
							},
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "default",
						},
					}),
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         specChangedMsg,
					Status:          v1alpha1.ProgressiveSyncWaiting,
					Step:            "1",
					TargetRevisions: []string{"abc123"},
				},
			},
		},
		{
			name: "does not detect changes when spec is identical (same Git revision)",
			appSet: newDefaultAppSet(2, []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "",
					Status:          v1alpha1.ProgressiveSyncHealthy,
					Step:            "1",
					TargetRevisions: []string{"abc123"},
				},
			}),
			apps: []v1alpha1.Application{
				newAppWithSpec("app1", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeSynced, "abc123", nil,
					v1alpha1.ApplicationSpec{
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://example.com/repo.git",
							TargetRevision: "master",
							Helm: &v1alpha1.ApplicationSourceHelm{
								Parameters: []v1alpha1.HelmParameter{
									{Name: "image.tag", Value: "v1.0.0"},
								},
							},
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "default",
						},
					}),
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			// Desired apps have identical spec
			desiredApps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Spec: v1alpha1.ApplicationSpec{
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://example.com/repo.git",
							TargetRevision: "master",
							Helm: &v1alpha1.ApplicationSourceHelm{
								Parameters: []v1alpha1.HelmParameter{
									{Name: "image.tag", Value: "v1.0.0"}, // Same value
								},
							},
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "default",
						},
					},
				},
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "",
					Status:          v1alpha1.ProgressiveSyncHealthy,
					Step:            "1",
					TargetRevisions: []string{"abc123"},
				},
			},
		},
		{
			name: "detects both spec and revision changes",
			appSet: newDefaultAppSet(2, []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "",
					Status:          v1alpha1.ProgressiveSyncHealthy,
					Step:            "1",
					TargetRevisions: []string{"abc123"}, // OLD revision in status
				},
			}),
			apps: []v1alpha1.Application{
				newAppWithSpec("app1", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeOutOfSync, "def456", nil, // NEW revision, but OutOfSync
					v1alpha1.ApplicationSpec{
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://example.com/repo.git",
							TargetRevision: "master",
							Helm: &v1alpha1.ApplicationSourceHelm{
								Parameters: []v1alpha1.HelmParameter{
									{Name: "image.tag", Value: "v1.0.0"},
								},
							},
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "default",
						},
					}),
			},
			desiredApps: []v1alpha1.Application{
				newAppWithSpec("app1", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeOutOfSync, "def456", nil,
					v1alpha1.ApplicationSpec{
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://example.com/repo.git",
							TargetRevision: "master",
							Helm: &v1alpha1.ApplicationSourceHelm{
								Parameters: []v1alpha1.HelmParameter{
									{Name: "image.tag", Value: "v2.0.0"}, // Changed value
								},
							},
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "default",
						},
					}),
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         revisionAndSpecChangedMsg,
					Status:          v1alpha1.ProgressiveSyncWaiting,
					Step:            "1",
					TargetRevisions: []string{"def456"},
				},
			},
		},
	} {
		t.Run(cc.name, func(t *testing.T) {
			kubeclientset := kubefake.NewClientset([]runtime.Object{}...)

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&cc.appSet).WithStatusSubresource(&cc.appSet).Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics()

			argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)

			r := &ApplicationSetReconciler{
				Client:        client,
				Scheme:        scheme,
				Recorder:      record.NewFakeRecorder(1),
				Generators:    map[string]generators.Generator{},
				ArgoDB:        argodb,
				KubeClientset: kubeclientset,
				Metrics:       metrics,
			}

			r.ProgressiveSyncManager = appsetprogressiveSync.NewManager(r.Client, r)

			desiredApps := cc.desiredApps
			if desiredApps == nil {
				desiredApps = cc.apps
			}
			appStatuses, err := r.ProgressiveSyncManager.UpdateApplicationSetApplicationStatus(t.Context(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.apps, desiredApps, cc.appStepMap)

			// opt out of testing the LastTransitionTime is accurate
			for i := range appStatuses {
				appStatuses[i].LastTransitionTime = nil
			}
			for i := range cc.expectedAppStatus {
				cc.expectedAppStatus[i].LastTransitionTime = nil
			}

			require.NoError(t, err, "expected no errors, but errors occurred")
			assert.Equal(t, cc.expectedAppStatus, appStatuses, "expected appStatuses did not match actual")
		})
	}
}

func TestUpdateApplicationSetApplicationStatusProgress(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	for _, cc := range []struct {
		name              string
		appSet            v1alpha1.ApplicationSet
		appSyncMap        map[string]bool
		appStepMap        map[string]int
		appMap            map[string]v1alpha1.Application
		expectedAppStatus []v1alpha1.ApplicationSetApplicationStatus
	}{
		{
			name: "handles an empty appSync and appStepMap",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{},
				},
			},
			appSyncMap:        map[string]bool{},
			appStepMap:        map[string]int{},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{},
		},
		{
			name: "handles an empty strategy",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{},
				},
			},
			appSyncMap:        map[string]bool{},
			appStepMap:        map[string]int{},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{},
		},
		{
			name: "handles an empty applicationset strategy",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{},
				},
			},
			appSyncMap:        map[string]bool{},
			appStepMap:        map[string]int{},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{},
		},
		{
			name: "handles an appSyncMap with no existing statuses",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{},
				},
			},
			appSyncMap: map[string]bool{
				"app1": true,
			},
			appStepMap: map[string]int{
				"app1": 0,
				"app2": 1,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{},
		},
		{
			name: "handles updating a RollingSync status from Waiting to Pending",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application:     "app1",
							Message:         "Application is out of date with the current AppSet generation, setting status to Waiting",
							Status:          v1alpha1.ProgressiveSyncWaiting,
							Step:            "1",
							TargetRevisions: []string{"next"},
						},
					},
				},
			},
			appSyncMap: map[string]bool{
				"app1": true,
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					LastTransitionTime: nil,
					Message:            "Application moved to Pending status, watching for the Application resource to start Progressing",
					Status:             v1alpha1.ProgressiveSyncPending,
					Step:               "1",
					TargetRevisions:    []string{"next"},
				},
			},
		},
		{
			name: "does not update a RollingSync status if appSyncMap is false",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting",
							Status:      v1alpha1.ProgressiveSyncWaiting,
							Step:        "1",
						},
					},
				},
			},
			appSyncMap: map[string]bool{
				"app1": false,
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					LastTransitionTime: nil,
					Message:            "Application is out of date with the current AppSet generation, setting status to Waiting",
					Status:             v1alpha1.ProgressiveSyncWaiting,
					Step:               "1",
				},
			},
		},
		{
			name: "does not update a status if status is not pending",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Message:     "Application Pending status timed out while waiting to become Progressing, reset status to Healthy",
							Status:      v1alpha1.ProgressiveSyncHealthy,
							Step:        "1",
						},
					},
				},
			},
			appSyncMap: map[string]bool{
				"app1": true,
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					LastTransitionTime: nil,
					Message:            "Application Pending status timed out while waiting to become Progressing, reset status to Healthy",
					Status:             v1alpha1.ProgressiveSyncHealthy,
					Step:               "1",
				},
			},
		},
		{
			name: "does not update a status if maxUpdate has already been reached with RollingSync",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
									MaxUpdate: &intstr.IntOrString{
										Type:   intstr.Int,
										IntVal: 3,
									},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Message:     "Application resource became Progressing, updating status from Pending to Progressing",
							Status:      v1alpha1.ProgressiveSyncProgressing,
							Step:        "1",
						},
						{
							Application: "app2",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting",
							Status:      v1alpha1.ProgressiveSyncWaiting,
							Step:        "1",
						},
						{
							Application: "app3",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting",
							Status:      v1alpha1.ProgressiveSyncWaiting,
							Step:        "1",
						},
						{
							Application: "app4",
							Message:     "Application moved to Pending status, watching for the Application resource to start Progressing",
							Status:      v1alpha1.ProgressiveSyncPending,
							Step:        "1",
						},
					},
				},
			},
			appSyncMap: map[string]bool{
				"app1": true,
				"app2": true,
				"app3": true,
				"app4": true,
			},
			appStepMap: map[string]int{
				"app1": 0,
				"app2": 0,
				"app3": 0,
				"app4": 0,
			},
			appMap: map[string]v1alpha1.Application{
				"app1": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				"app2": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app2",
					},
					Status: v1alpha1.ApplicationStatus{
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				"app3": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app3",
					},
					Status: v1alpha1.ApplicationStatus{
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				"app4": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app4",
					},
					Status: v1alpha1.ApplicationStatus{
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					LastTransitionTime: nil,
					Message:            "Application resource became Progressing, updating status from Pending to Progressing",
					Status:             v1alpha1.ProgressiveSyncProgressing,
					Step:               "1",
				},
				{
					Application:        "app2",
					LastTransitionTime: nil,
					Message:            "Application moved to Pending status, watching for the Application resource to start Progressing",
					Status:             v1alpha1.ProgressiveSyncPending,
					Step:               "1",
				},
				{
					Application:        "app3",
					LastTransitionTime: nil,
					Message:            "Application is out of date with the current AppSet generation, setting status to Waiting",
					Status:             v1alpha1.ProgressiveSyncWaiting,
					Step:               "1",
				},
				{
					Application:        "app4",
					LastTransitionTime: nil,
					Message:            "Application moved to Pending status, watching for the Application resource to start Progressing",
					Status:             v1alpha1.ProgressiveSyncPending,
					Step:               "1",
				},
			},
		},
		{
			name: "rounds down for maxUpdate set to percentage string",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
									MaxUpdate: &intstr.IntOrString{
										Type:   intstr.String,
										StrVal: "50%",
									},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting",
							Status:      v1alpha1.ProgressiveSyncWaiting,
							Step:        "1",
						},
						{
							Application: "app2",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting",
							Status:      v1alpha1.ProgressiveSyncWaiting,
							Step:        "1",
						},
						{
							Application: "app3",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting",
							Status:      v1alpha1.ProgressiveSyncWaiting,
							Step:        "1",
						},
					},
				},
			},
			appSyncMap: map[string]bool{
				"app1": true,
				"app2": true,
				"app3": true,
			},
			appStepMap: map[string]int{
				"app1": 0,
				"app2": 0,
				"app3": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					LastTransitionTime: nil,
					Message:            "Application moved to Pending status, watching for the Application resource to start Progressing",
					Status:             v1alpha1.ProgressiveSyncPending,
					Step:               "1",
				},
				{
					Application:        "app2",
					LastTransitionTime: nil,
					Message:            "Application is out of date with the current AppSet generation, setting status to Waiting",
					Status:             v1alpha1.ProgressiveSyncWaiting,
					Step:               "1",
				},
				{
					Application:        "app3",
					LastTransitionTime: nil,
					Message:            "Application is out of date with the current AppSet generation, setting status to Waiting",
					Status:             v1alpha1.ProgressiveSyncWaiting,
					Step:               "1",
				},
			},
		},
		{
			name: "does not update any applications with maxUpdate set to 0",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
									MaxUpdate: &intstr.IntOrString{
										Type:   intstr.Int,
										IntVal: 0,
									},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting",
							Status:      v1alpha1.ProgressiveSyncWaiting,
							Step:        "1",
						},
						{
							Application: "app2",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting",
							Status:      v1alpha1.ProgressiveSyncWaiting,
							Step:        "1",
						},
						{
							Application: "app3",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting",
							Status:      v1alpha1.ProgressiveSyncWaiting,
							Step:        "1",
						},
					},
				},
			},
			appSyncMap: map[string]bool{
				"app1": true,
				"app2": true,
				"app3": true,
			},
			appStepMap: map[string]int{
				"app1": 0,
				"app2": 0,
				"app3": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					LastTransitionTime: nil,
					Message:            "Application is out of date with the current AppSet generation, setting status to Waiting",
					Status:             v1alpha1.ProgressiveSyncWaiting,
					Step:               "1",
				},
				{
					Application:        "app2",
					LastTransitionTime: nil,
					Message:            "Application is out of date with the current AppSet generation, setting status to Waiting",
					Status:             v1alpha1.ProgressiveSyncWaiting,
					Step:               "1",
				},
				{
					Application:        "app3",
					LastTransitionTime: nil,
					Message:            "Application is out of date with the current AppSet generation, setting status to Waiting",
					Status:             v1alpha1.ProgressiveSyncWaiting,
					Step:               "1",
				},
			},
		},
		{
			name: "updates all applications with maxUpdate set to 100%",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
									MaxUpdate: &intstr.IntOrString{
										Type:   intstr.String,
										StrVal: "100%",
									},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting",
							Status:      v1alpha1.ProgressiveSyncWaiting,
							Step:        "1",
						},
						{
							Application: "app2",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting",
							Status:      v1alpha1.ProgressiveSyncWaiting,
							Step:        "1",
						},
						{
							Application: "app3",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting",
							Status:      v1alpha1.ProgressiveSyncWaiting,
							Step:        "1",
						},
					},
				},
			},
			appSyncMap: map[string]bool{
				"app1": true,
				"app2": true,
				"app3": true,
			},
			appStepMap: map[string]int{
				"app1": 0,
				"app2": 0,
				"app3": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					LastTransitionTime: nil,
					Message:            "Application moved to Pending status, watching for the Application resource to start Progressing",
					Status:             v1alpha1.ProgressiveSyncPending,
					Step:               "1",
				},
				{
					Application:        "app2",
					LastTransitionTime: nil,
					Message:            "Application moved to Pending status, watching for the Application resource to start Progressing",
					Status:             v1alpha1.ProgressiveSyncPending,
					Step:               "1",
				},
				{
					Application:        "app3",
					LastTransitionTime: nil,
					Message:            "Application moved to Pending status, watching for the Application resource to start Progressing",
					Status:             v1alpha1.ProgressiveSyncPending,
					Step:               "1",
				},
			},
		},
		{
			name: "updates at least 1 application with maxUpdate >0%",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
									MaxUpdate: &intstr.IntOrString{
										Type:   intstr.String,
										StrVal: "1%",
									},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting",
							Status:      v1alpha1.ProgressiveSyncWaiting,
							Step:        "1",
						},
						{
							Application: "app2",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting",
							Status:      v1alpha1.ProgressiveSyncWaiting,
							Step:        "1",
						},
						{
							Application: "app3",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting",
							Status:      v1alpha1.ProgressiveSyncWaiting,
							Step:        "1",
						},
					},
				},
			},
			appSyncMap: map[string]bool{
				"app1": true,
				"app2": true,
				"app3": true,
			},
			appStepMap: map[string]int{
				"app1": 0,
				"app2": 0,
				"app3": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					LastTransitionTime: nil,
					Message:            "Application moved to Pending status, watching for the Application resource to start Progressing",
					Status:             v1alpha1.ProgressiveSyncPending,
					Step:               "1",
				},
				{
					Application:        "app2",
					LastTransitionTime: nil,
					Message:            "Application is out of date with the current AppSet generation, setting status to Waiting",
					Status:             v1alpha1.ProgressiveSyncWaiting,
					Step:               "1",
				},
				{
					Application:        "app3",
					LastTransitionTime: nil,
					Message:            "Application is out of date with the current AppSet generation, setting status to Waiting",
					Status:             v1alpha1.ProgressiveSyncWaiting,
					Step:               "1",
				},
			},
		},
	} {
		t.Run(cc.name, func(t *testing.T) {
			kubeclientset := kubefake.NewClientset([]runtime.Object{}...)

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&cc.appSet).WithStatusSubresource(&cc.appSet).Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics()

			argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)

			r := &ApplicationSetReconciler{
				Client:        client,
				Scheme:        scheme,
				Recorder:      record.NewFakeRecorder(1),
				Generators:    map[string]generators.Generator{},
				ArgoDB:        argodb,
				KubeClientset: kubeclientset,
				Metrics:       metrics,
			}
			r.ProgressiveSyncManager = appsetprogressiveSync.NewManager(r.Client, r)

			appStatuses, err := r.ProgressiveSyncManager.UpdateApplicationSetApplicationStatusProgress(t.Context(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.appSyncMap, cc.appStepMap)

			// opt out of testing the LastTransitionTime is accurate
			for i := range appStatuses {
				appStatuses[i].LastTransitionTime = nil
			}

			require.NoError(t, err, "expected no errors, but errors occurred")
			assert.Equal(t, cc.expectedAppStatus, appStatuses, "expected appStatuses did not match actual")
		})
	}
}
