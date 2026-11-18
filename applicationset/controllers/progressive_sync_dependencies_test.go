package controllers

import (
	"testing"
	"time"

	appfake "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/fake"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
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
	t.Parallel()
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
			Name:      "name",
			Namespace: "argocd",
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

	newAppSetWithMaxUpdate := func(maxUpdate intstr.IntOrString, status []v1alpha1.ApplicationSetApplicationStatus) v1alpha1.ApplicationSet {
		appSet := newDefaultAppSet(1, status)
		appSet.Spec.Strategy.RollingSync.Steps[0].MaxUpdate = &maxUpdate
		return appSet
	}

	newApp := func(name string, health health.HealthStatusCode, sync v1alpha1.SyncStatusCode, revision string, opState *v1alpha1.OperationState) v1alpha1.Application {
		return v1alpha1.Application{
			Name: name,
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
					Name: "app1",
					Status: v1alpha1.ApplicationStatus{
						ReconciledAt: nil,
						Conditions: []v1alpha1.ApplicationCondition{
							{
								Type:               v1alpha1.ApplicationConditionInvalidSpecError,
								Message:            "Fake invalid specs preventing app updates and sync to be triggered",
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
					Name: "app1",
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
					Name: "app1",
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
		{
			name:   "keeps a second synced but unhealthy application waiting when maxUpdate is 1",
			appSet: newAppSetWithMaxUpdate(intstr.FromInt(1), []v1alpha1.ApplicationSetApplicationStatus{}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusDegraded, v1alpha1.SyncStatusCodeSynced, "current", newOperationState(common.OperationSucceeded)),
				newApp("app2", health.HealthStatusDegraded, v1alpha1.SyncStatusCodeSynced, "current", newOperationState(common.OperationSucceeded)),
			},
			appStepMap: map[string]int{
				"app1": 0,
				"app2": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource has synced, updating status to Progressing",
					Status:          v1alpha1.ProgressiveSyncProgressing,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
				{
					Application:     "app2",
					Message:         "Application resource has synced but is waiting for an available update slot in this step",
					Status:          v1alpha1.ProgressiveSyncWaiting,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
			},
		},
		{
			name:   "keeps a second synced but unhealthy application waiting when maxUpdate is 50 percent",
			appSet: newAppSetWithMaxUpdate(intstr.FromString("50%"), []v1alpha1.ApplicationSetApplicationStatus{}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusDegraded, v1alpha1.SyncStatusCodeSynced, "current", newOperationState(common.OperationSucceeded)),
				newApp("app2", health.HealthStatusDegraded, v1alpha1.SyncStatusCodeSynced, "current", newOperationState(common.OperationSucceeded)),
			},
			appStepMap: map[string]int{
				"app1": 0,
				"app2": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource has synced, updating status to Progressing",
					Status:          v1alpha1.ProgressiveSyncProgressing,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
				{
					Application:     "app2",
					Message:         "Application resource has synced but is waiting for an available update slot in this step",
					Status:          v1alpha1.ProgressiveSyncWaiting,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
			},
		},
		{
			name: "keeps a synced but unhealthy application waiting when the step already has a progressing application",
			appSet: newAppSetWithMaxUpdate(intstr.FromInt(1), []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					Message:            "Application resource has synced, updating status to Progressing",
					Status:             v1alpha1.ProgressiveSyncProgressing,
					Step:               "1",
					TargetRevisions:    []string{"current"},
					LastTransitionTime: &nowMinus5,
				},
			}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusDegraded, v1alpha1.SyncStatusCodeSynced, "current", newOperationState(common.OperationSucceeded)),
				newApp("app2", health.HealthStatusDegraded, v1alpha1.SyncStatusCodeSynced, "current", newOperationState(common.OperationSucceeded)),
			},
			appStepMap: map[string]int{
				"app1": 0,
				"app2": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource has synced, updating status to Progressing",
					Status:          v1alpha1.ProgressiveSyncProgressing,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
				{
					Application:     "app2",
					Message:         "Application resource has synced but is waiting for an available update slot in this step",
					Status:          v1alpha1.ProgressiveSyncWaiting,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
			},
		},
		{
			name:   "promotes both synced but unhealthy applications when maxUpdate is 2",
			appSet: newAppSetWithMaxUpdate(intstr.FromInt(2), []v1alpha1.ApplicationSetApplicationStatus{}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusDegraded, v1alpha1.SyncStatusCodeSynced, "current", newOperationState(common.OperationSucceeded)),
				newApp("app2", health.HealthStatusDegraded, v1alpha1.SyncStatusCodeSynced, "current", newOperationState(common.OperationSucceeded)),
			},
			appStepMap: map[string]int{
				"app1": 0,
				"app2": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource has synced, updating status to Progressing",
					Status:          v1alpha1.ProgressiveSyncProgressing,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
				{
					Application:     "app2",
					Message:         "Application resource has synced, updating status to Progressing",
					Status:          v1alpha1.ProgressiveSyncProgressing,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
			},
		},
		{
			name:   "does not let an unselected application consume a step update slot",
			appSet: newAppSetWithMaxUpdate(intstr.FromInt(1), []v1alpha1.ApplicationSetApplicationStatus{}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusDegraded, v1alpha1.SyncStatusCodeSynced, "current", newOperationState(common.OperationSucceeded)),
				newApp("app2", health.HealthStatusDegraded, v1alpha1.SyncStatusCodeSynced, "current", newOperationState(common.OperationSucceeded)),
			},
			appStepMap: map[string]int{
				"app2": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource has synced, updating status to Progressing",
					Status:          v1alpha1.ProgressiveSyncProgressing,
					Step:            "-1",
					TargetRevisions: []string{"current"},
				},
				{
					Application:     "app2",
					Message:         "Application resource has synced, updating status to Progressing",
					Status:          v1alpha1.ProgressiveSyncProgressing,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
			},
		},
		{
			name: "does not count an unselected application's progressing status against a step",
			appSet: newAppSetWithMaxUpdate(intstr.FromInt(1), []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					Message:            "Application resource has synced, updating status to Progressing",
					Status:             v1alpha1.ProgressiveSyncProgressing,
					Step:               "-1",
					TargetRevisions:    []string{"current"},
					LastTransitionTime: &nowMinus5,
				},
			}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusDegraded, v1alpha1.SyncStatusCodeSynced, "current", newOperationState(common.OperationSucceeded)),
				newApp("app2", health.HealthStatusDegraded, v1alpha1.SyncStatusCodeSynced, "current", newOperationState(common.OperationSucceeded)),
			},
			appStepMap: map[string]int{
				"app2": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource has synced, updating status to Progressing",
					Status:          v1alpha1.ProgressiveSyncProgressing,
					Step:            "-1",
					TargetRevisions: []string{"current"},
				},
				{
					Application:     "app2",
					Message:         "Application resource has synced, updating status to Progressing",
					Status:          v1alpha1.ProgressiveSyncProgressing,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
			},
		},
		{
			name:   "moves synced and healthy applications to healthy without consuming a maxUpdate slot",
			appSet: newAppSetWithMaxUpdate(intstr.FromInt(1), []v1alpha1.ApplicationSetApplicationStatus{}),
			apps: []v1alpha1.Application{
				newApp("app1", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeSynced, "current", newOperationState(common.OperationSucceeded)),
				newApp("app2", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeSynced, "current", newOperationState(common.OperationSucceeded)),
			},
			appStepMap: map[string]int{
				"app1": 0,
				"app2": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource has synced, updating status to Healthy",
					Status:          v1alpha1.ProgressiveSyncHealthy,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
				{
					Application:     "app2",
					Message:         "Application resource has synced, updating status to Healthy",
					Status:          v1alpha1.ProgressiveSyncHealthy,
					Step:            "1",
					TargetRevisions: []string{"current"},
				},
			},
		},
	} {
		t.Run(cc.name, func(t *testing.T) {
			t.Parallel()
			kubeclientset := kubefake.NewClientset([]runtime.Object{}...)

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&cc.appSet).WithStatusSubresource(&cc.appSet).Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics()
			appObjs := []runtime.Object{}
			appClientSet := appfake.NewSimpleClientset(appObjs...)
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

			r.ProgressiveSyncManager = appsetprogressiveSync.NewManager(r.Client, r.Client, appClientSet, r)

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
	t.Parallel()
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
		{
			name: "does not count unselected applications in a percentage maxUpdate total",
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
							Step:        "-1",
						},
						{
							Application: "app4",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting",
							Status:      v1alpha1.ProgressiveSyncWaiting,
							Step:        "-1",
						},
					},
				},
			},
			appSyncMap: map[string]bool{
				"app1": true,
				"app2": true,
			},
			appStepMap: map[string]int{
				"app1": 0,
				"app2": 0,
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
					Step:               "-1",
				},
				{
					Application:        "app4",
					LastTransitionTime: nil,
					Message:            "Application is out of date with the current AppSet generation, setting status to Waiting",
					Status:             v1alpha1.ProgressiveSyncWaiting,
					Step:               "-1",
				},
			},
		},
		{
			name: "does not count an unselected application's progressing status against a step",
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
										IntVal: 1,
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
							Message:     "Application resource became Progressing, updating status from Pending to Progressing",
							Status:      v1alpha1.ProgressiveSyncProgressing,
							Step:        "-1",
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
				},
				{
					Application:        "app2",
					LastTransitionTime: nil,
					Message:            "Application resource became Progressing, updating status from Pending to Progressing",
					Status:             v1alpha1.ProgressiveSyncProgressing,
					Step:               "-1",
				},
			},
		},
	} {
		t.Run(cc.name, func(t *testing.T) {
			t.Parallel()
			kubeclientset := kubefake.NewClientset([]runtime.Object{}...)

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&cc.appSet).WithStatusSubresource(&cc.appSet).Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics()
			appObjs := []runtime.Object{}
			appClientSet := appfake.NewSimpleClientset(appObjs...)
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
			r.ProgressiveSyncManager = appsetprogressiveSync.NewManager(r.Client, r.Client, appClientSet, r)

			// Live applications for the Progress call. Cases without appMap keep
			// the previous behaviour: applications missing here are treated as
			// not Synced, so their Waiting -> Pending admission is unchanged.
			liveApps := make([]v1alpha1.Application, 0, len(cc.appMap))
			for _, app := range cc.appMap {
				liveApps = append(liveApps, app)
			}

			appStatuses, err := r.ProgressiveSyncManager.UpdateApplicationSetApplicationStatusProgress(t.Context(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.appSyncMap, cc.appStepMap, liveApps)

			// opt out of testing the LastTransitionTime is accurate
			for i := range appStatuses {
				appStatuses[i].LastTransitionTime = nil
			}

			require.NoError(t, err, "expected no errors, but errors occurred")
			assert.Equal(t, cc.expectedAppStatus, appStatuses, "expected appStatuses did not match actual")
		})
	}
}

// TestSyncedAppWaitingForSlotSkipsPendingAndSync pins the behaviour ppapapetrou76
// asked about on #30988: a Synced but unhealthy application that waits for a
// maxUpdate slot must not be synced once the slot frees up. It goes straight
// to Progressing and never enters Pending, so SyncDesiredApplications attaches
// no sync operation to it — before the admission gate the appset never synced
// such an application, and waiting for a slot must not change that.
//
// The test drives UpdateApplicationSetApplicationStatus and
// UpdateApplicationSetApplicationStatusProgress in sequence, mirroring the order
// PerformProgressiveSyncs calls them in, across two reconciles: first with the
// step full, then with the slot freed by the other application becoming Healthy.
// The waiting application is processed before the freeing one in the second
// reconcile, so the status function still sees the slot as full and the
// Progress function must not second-guess it by moving the application to Pending.
func TestSyncedAppWaitingForSlotSkipsPendingAndSync(t *testing.T) {
	t.Parallel()
	nowMinus5 := metav1.Time{Time: time.Now().Add(-5 * time.Minute)}
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	newAppSet := func() v1alpha1.ApplicationSet {
		maxUpdate := intstr.FromInt(1)
		return v1alpha1.ApplicationSet{
			Name:      "name",
			Namespace: "argocd",
			Spec: v1alpha1.ApplicationSetSpec{
				Strategy: &v1alpha1.ApplicationSetStrategy{
					Type: "RollingSync",
					RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
						Steps: []v1alpha1.ApplicationSetRolloutStep{
							{MaxUpdate: &maxUpdate, MatchExpressions: []v1alpha1.ApplicationMatchExpression{}},
						},
					},
				},
			},
			Status: v1alpha1.ApplicationSetStatus{
				ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
					{
						Application:        "app1",
						Message:            "Application resource has synced, updating status to Progressing",
						Status:             v1alpha1.ProgressiveSyncProgressing,
						Step:               "1",
						TargetRevisions:    []string{"current"},
						LastTransitionTime: &nowMinus5,
					},
					{
						Application:        "app2",
						Message:            "Application resource has synced but is waiting for an available update slot in this step",
						Status:             v1alpha1.ProgressiveSyncWaiting,
						Step:               "1",
						TargetRevisions:    []string{"current"},
						LastTransitionTime: &nowMinus5,
					},
				},
			},
		}
	}

	newApp := func(name string, healthStatus health.HealthStatusCode, syncStatus v1alpha1.SyncStatusCode) v1alpha1.Application {
		return v1alpha1.Application{
			Name: name,
			Status: v1alpha1.ApplicationStatus{
				ReconciledAt: &metav1.Time{Time: time.Now()},
				Health:       v1alpha1.AppHealthStatus{Status: healthStatus},
				Sync: v1alpha1.SyncStatus{
					Status:   syncStatus,
					Revision: "current",
				},
				OperationState: &v1alpha1.OperationState{
					Phase:      common.OperationSucceeded,
					StartedAt:  metav1.Time{Time: time.Now().Add(-1 * time.Minute)},
					FinishedAt: &metav1.Time{Time: time.Now().Add(-1 * time.Second)},
				},
			},
		}
	}

	appSet := newAppSet()
	kubeclientset := kubefake.NewClientset([]runtime.Object{}...)
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet).WithStatusSubresource(&appSet).Build()
	metrics := appsetmetrics.NewFakeAppsetMetrics()
	appClientSet := appfake.NewSimpleClientset([]runtime.Object{}...)
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
	r.ProgressiveSyncManager = appsetprogressiveSync.NewManager(r.Client, r.Client, appClientSet, r)
	logCtx := log.NewEntry(log.StandardLogger())
	appStepMap := map[string]int{"app1": 0, "app2": 0}
	appsToSync := map[string]bool{"app1": true, "app2": true}

	statusOf := func(statuses []v1alpha1.ApplicationSetApplicationStatus, app string) v1alpha1.ProgressiveSyncStatusCode {
		for _, s := range statuses {
			if s.Application == app {
				return s.Status
			}
		}
		t.Fatalf("no status for application %q in %+v", app, statuses)
		return ""
	}

	// Reconcile 1: the step is full (app1 is Progressing), so app2 must stay Waiting.
	app1 := newApp("app1", health.HealthStatusDegraded, v1alpha1.SyncStatusCodeSynced)
	app2 := newApp("app2", health.HealthStatusDegraded, v1alpha1.SyncStatusCodeSynced)
	statuses, err := r.ProgressiveSyncManager.UpdateApplicationSetApplicationStatus(t.Context(), logCtx, &appSet, []v1alpha1.Application{app1, app2}, []v1alpha1.Application{app1, app2}, appStepMap)
	require.NoError(t, err)
	assert.Equal(t, v1alpha1.ProgressiveSyncWaiting, statusOf(statuses, "app2"), "app2 must wait while the step is full")
	appSet.Status.ApplicationStatus = statuses

	statuses, err = r.ProgressiveSyncManager.UpdateApplicationSetApplicationStatusProgress(t.Context(), logCtx, &appSet, appsToSync, appStepMap, []v1alpha1.Application{app1, app2})
	require.NoError(t, err)
	assert.Equal(t, v1alpha1.ProgressiveSyncWaiting, statusOf(statuses, "app2"), "app2 must not go Pending while the step is full")
	appSet.Status.ApplicationStatus = statuses

	// Reconcile 2: app1 becomes Healthy, freeing the slot. app2 is processed
	// first, so the status function still counts the slot as full here.
	app1 = newApp("app1", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeSynced)
	statuses, err = r.ProgressiveSyncManager.UpdateApplicationSetApplicationStatus(t.Context(), logCtx, &appSet, []v1alpha1.Application{app2, app1}, []v1alpha1.Application{app2, app1}, appStepMap)
	require.NoError(t, err)
	assert.Equal(t, v1alpha1.ProgressiveSyncHealthy, statusOf(statuses, "app1"), "app1 must become Healthy once it is healthy and synced")
	appSet.Status.ApplicationStatus = statuses

	statuses, err = r.ProgressiveSyncManager.UpdateApplicationSetApplicationStatusProgress(t.Context(), logCtx, &appSet, appsToSync, appStepMap, []v1alpha1.Application{app2, app1})
	require.NoError(t, err)
	assert.NotEqual(t, v1alpha1.ProgressiveSyncPending, statusOf(statuses, "app2"),
		"a Synced application that waited for a slot must never go through Pending once the slot frees up")
	appSet.Status.ApplicationStatus = statuses

	// Reconcile 3: the status function now sees the free slot and must promote
	// app2 straight to Progressing.
	statuses, err = r.ProgressiveSyncManager.UpdateApplicationSetApplicationStatus(t.Context(), logCtx, &appSet, []v1alpha1.Application{app2, app1}, []v1alpha1.Application{app2, app1}, appStepMap)
	require.NoError(t, err)
	assert.Equal(t, v1alpha1.ProgressiveSyncProgressing, statusOf(statuses, "app2"),
		"app2 must go straight to Progressing once admitted, skipping the sync queue")
	appSet.Status.ApplicationStatus = statuses

	// No sync may be triggered for app2: it is already Synced, and only Pending
	// applications get a sync operation attached.
	rolloutApps := r.ProgressiveSyncManager.SyncDesiredApplications(logCtx, &appSet, appsToSync, []v1alpha1.Application{app1, app2})
	for _, app := range rolloutApps {
		assert.Nil(t, app.Operation, "no sync operation must be triggered for already-Synced application %q", app.Name)
	}
}

// TestRevisionChangeReleasesMaxUpdateSlot pins the slot accounting when a
// new target revision arrives for an application that already holds a
// maxUpdate slot (recorded Pending or Progressing). The revision change
// resets the application to Waiting first, so the status function must
// release the slot on the same pass when the application leaves its counted
// status, and must not reserve a second slot when it stays active:
//
//   - a Progressing application that reports the new revision Synced and
//     Healthy releases its slot, so a waiter later in the same pass can be
//     admitted;
//   - a Progressing application that reports the new revision Synced but
//     still unhealthy keeps its slot and goes straight back to Progressing
//     instead of being denied by its own reservation and flickering to
//     Waiting.
//
// Both cases fail on the previous commit and pass with the fix.
func TestRevisionChangeReleasesMaxUpdateSlot(t *testing.T) {
	t.Parallel()
	nowMinus5 := metav1.Time{Time: time.Now().Add(-5 * time.Minute)}
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	newManager := func(t *testing.T, appSet v1alpha1.ApplicationSet) *ApplicationSetReconciler {
		t.Helper()
		kubeclientset := kubefake.NewClientset([]runtime.Object{}...)
		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet).WithStatusSubresource(&appSet).Build()
		metrics := appsetmetrics.NewFakeAppsetMetrics()
		appClientSet := appfake.NewSimpleClientset([]runtime.Object{}...)
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
		r.ProgressiveSyncManager = appsetprogressiveSync.NewManager(r.Client, r.Client, appClientSet, r)
		return r
	}

	newAppSet := func() v1alpha1.ApplicationSet {
		maxUpdate := intstr.FromInt(1)
		return v1alpha1.ApplicationSet{
			Name:      "name",
			Namespace: "argocd",
			Spec: v1alpha1.ApplicationSetSpec{
				Strategy: &v1alpha1.ApplicationSetStrategy{
					Type: "RollingSync",
					RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
						Steps: []v1alpha1.ApplicationSetRolloutStep{
							{MaxUpdate: &maxUpdate, MatchExpressions: []v1alpha1.ApplicationMatchExpression{}},
						},
					},
				},
			},
			Status: v1alpha1.ApplicationSetStatus{
				ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
					{
						Application:        "app1",
						Message:            "Application resource has synced, updating status to Progressing",
						Status:             v1alpha1.ProgressiveSyncProgressing,
						Step:               "0",
						TargetRevisions:    []string{"old"},
						LastTransitionTime: &nowMinus5,
					},
					{
						Application:        "app2",
						Message:            "Application resource has synced but is waiting for an available update slot in this step",
						Status:             v1alpha1.ProgressiveSyncWaiting,
						Step:               "0",
						TargetRevisions:    []string{"old"},
						LastTransitionTime: &nowMinus5,
					},
				},
			},
		}
	}

	newApp := func(name, revision string, healthStatus health.HealthStatusCode, syncStatus v1alpha1.SyncStatusCode) v1alpha1.Application {
		return v1alpha1.Application{
			Name: name,
			Status: v1alpha1.ApplicationStatus{
				ReconciledAt: &metav1.Time{Time: time.Now()},
				Health:       v1alpha1.AppHealthStatus{Status: healthStatus},
				Sync: v1alpha1.SyncStatus{
					Status:   syncStatus,
					Revision: revision,
				},
				OperationState: &v1alpha1.OperationState{
					Phase:      common.OperationSucceeded,
					StartedAt:  metav1.Time{Time: time.Now().Add(-1 * time.Minute)},
					FinishedAt: &metav1.Time{Time: time.Now().Add(-1 * time.Second)},
				},
			},
		}
	}

	statusOf := func(t *testing.T, statuses []v1alpha1.ApplicationSetApplicationStatus, app string) v1alpha1.ProgressiveSyncStatusCode {
		t.Helper()
		for _, s := range statuses {
			if s.Application == app {
				return s.Status
			}
		}
		t.Fatalf("no status for application %q in %+v", app, statuses)
		return ""
	}

	appStepMap := map[string]int{"app1": 0, "app2": 0}
	logCtx := log.NewEntry(log.StandardLogger())

	t.Run("healthy new revision releases the slot for a waiter in the same pass", func(t *testing.T) {
		t.Parallel()
		appSet := newAppSet()
		r := newManager(t, appSet)

		app1 := newApp("app1", "new", health.HealthStatusHealthy, v1alpha1.SyncStatusCodeSynced)
		app2 := newApp("app2", "old", health.HealthStatusDegraded, v1alpha1.SyncStatusCodeSynced)
		statuses, err := r.ProgressiveSyncManager.UpdateApplicationSetApplicationStatus(t.Context(), logCtx, &appSet, []v1alpha1.Application{app1, app2}, []v1alpha1.Application{app1, app2}, appStepMap)
		require.NoError(t, err)
		assert.Equal(t, v1alpha1.ProgressiveSyncHealthy, statusOf(t, statuses, "app1"), "app1 must become Healthy for the new revision")
		assert.Equal(t, v1alpha1.ProgressiveSyncProgressing, statusOf(t, statuses, "app2"), "app2 must be admitted in the same pass once app1 releases its slot")
	})

	t.Run("unhealthy new revision keeps its slot instead of flickering to Waiting", func(t *testing.T) {
		t.Parallel()
		appSet := newAppSet()
		r := newManager(t, appSet)

		app1 := newApp("app1", "new", health.HealthStatusDegraded, v1alpha1.SyncStatusCodeSynced)
		app2 := newApp("app2", "old", health.HealthStatusDegraded, v1alpha1.SyncStatusCodeSynced)
		statuses, err := r.ProgressiveSyncManager.UpdateApplicationSetApplicationStatus(t.Context(), logCtx, &appSet, []v1alpha1.Application{app1, app2}, []v1alpha1.Application{app1, app2}, appStepMap)
		require.NoError(t, err)
		assert.Equal(t, v1alpha1.ProgressiveSyncProgressing, statusOf(t, statuses, "app1"), "app1 must keep its slot and stay Progressing for the new revision")
		assert.Equal(t, v1alpha1.ProgressiveSyncWaiting, statusOf(t, statuses, "app2"), "app2 must keep waiting while app1 still holds the step's only slot")
	})
}
