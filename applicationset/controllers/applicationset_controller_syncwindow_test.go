package controllers

import (
	"fmt"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	crtclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj/argo-cd/v3/applicationset/generators"
	appsetmetrics "github.com/argoproj/argo-cd/v3/applicationset/metrics"
	"github.com/argoproj/argo-cd/v3/applicationset/progressivesync"
	"github.com/argoproj/argo-cd/v3/applicationset/utils"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// Regression test for https://github.com/argoproj/argo-cd/issues/29307: RollingSync must not
// write an automated sync Operation to an Application whose sync windows deny automated syncs.
func TestHoldSyncsBlockedBySyncWindows(t *testing.T) {
	t.Parallel()

	alwaysActive := func(kind string, manualSync bool, apps ...string) *v1alpha1.InlineSyncWindow {
		return &v1alpha1.InlineSyncWindow{Kind: kind, Schedule: "* * * * *", Duration: "1h", ManualSync: manualSync, Applications: apps}
	}
	// An allow window that is not active now, so automated syncs are denied outside of it.
	inactiveAllow := &v1alpha1.InlineSyncWindow{
		Kind:         "allow",
		Schedule:     fmt.Sprintf("0 %d * * *", time.Now().UTC().Add(12*time.Hour).Hour()),
		Duration:     "1h",
		Applications: []string{"*"},
	}

	for _, tc := range []struct {
		name string
		// project is the name of the AppProject the Application uses
		project string
		// windows are the sync windows of that AppProject; nil means the project does not exist
		windows     v1alpha1.SyncWindows
		projMissing bool
		// notPending marks the Application as already Healthy in the rollout, so no sync is due
		notPending bool
		wantHeld   bool
	}{
		{
			name:     "no sync windows",
			project:  "team",
			windows:  v1alpha1.SyncWindows{},
			wantHeld: false,
		},
		{
			name:     "active deny window",
			project:  "team",
			windows:  v1alpha1.SyncWindows{alwaysActive("deny", false, "*")},
			wantHeld: true,
		},
		{
			// manualSync only lets manual syncs through; RollingSync triggers automated syncs.
			name:     "active deny window that allows manual syncs",
			project:  "team",
			windows:  v1alpha1.SyncWindows{alwaysActive("deny", true, "*")},
			wantHeld: true,
		},
		{
			// The "default" AppProject can have sync windows like any other project.
			name:     "active deny window on the default project",
			project:  "default",
			windows:  v1alpha1.SyncWindows{alwaysActive("deny", true, "*")},
			wantHeld: true,
		},
		{
			name:     "active deny window for another application",
			project:  "team",
			windows:  v1alpha1.SyncWindows{alwaysActive("deny", false, "other-app")},
			wantHeld: false,
		},
		{
			name:     "active allow window",
			project:  "team",
			windows:  v1alpha1.SyncWindows{alwaysActive("allow", false, "*")},
			wantHeld: false,
		},
		{
			name:     "inactive allow window",
			project:  "team",
			windows:  v1alpha1.SyncWindows{inactiveAllow},
			wantHeld: true,
		},
		{
			// The application controller treats an invalid window as blocking.
			name:     "invalid sync window",
			project:  "team",
			windows:  v1alpha1.SyncWindows{{Kind: "deny", Schedule: "not a schedule", Duration: "1h", Applications: []string{"*"}}},
			wantHeld: true,
		},
		{
			// Only Applications whose sync RollingSync is about to trigger are checked.
			name:       "application is not pending",
			project:    "team",
			windows:    v1alpha1.SyncWindows{alwaysActive("deny", false, "*")},
			notPending: true,
			wantHeld:   false,
		},
		{
			name:        "project cannot be read",
			project:     "team",
			projMissing: true,
			wantHeld:    true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			scheme := runtime.NewScheme()
			require.NoError(t, v1alpha1.AddToScheme(scheme))

			appStatus := v1alpha1.ProgressiveSyncPending
			if tc.notPending {
				appStatus = v1alpha1.ProgressiveSyncHealthy
			}
			appSet := &v1alpha1.ApplicationSet{
				Name: "appset", Namespace: "argocd",
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{Application: "app", Status: appStatus, Step: "1"},
						{Application: "unrelated", Status: v1alpha1.ProgressiveSyncHealthy, Step: "1"},
					},
				},
			}
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			if !tc.projMissing {
				clientBuilder = clientBuilder.WithObjects(&v1alpha1.AppProject{
					Name: tc.project, Namespace: "argocd",
					Spec: v1alpha1.AppProjectSpec{SyncWindows: tc.windows},
				})
			}
			r := &ApplicationSetReconciler{
				Client:          clientBuilder.Build(),
				ArgoCDNamespace: "argocd",
				Metrics:         appsetmetrics.NewFakeAppsetMetrics(),
			}
			apps := []v1alpha1.Application{
				{
					Name: "app", Namespace: "argocd",
					Spec: v1alpha1.ApplicationSpec{
						Project:     tc.project,
						Destination: v1alpha1.ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "app"},
					},
				},
				{
					// Not selected for sync in this reconcile, so it must be left alone.
					Name: "unrelated", Namespace: "argocd",
					Spec: v1alpha1.ApplicationSpec{Project: tc.project},
				},
			}
			appSyncMap := map[string]bool{"app": true, "unrelated": false}
			logCtx := log.NewEntry(log.New())

			held := r.holdSyncsBlockedBySyncWindows(t.Context(), logCtx, appSet, appSyncMap, apps)
			assert.Equal(t, tc.wantHeld, held)
			assert.Equal(t, !tc.wantHeld, appSyncMap["app"])
			if tc.notPending {
				return
			}
			assert.False(t, appSyncMap["unrelated"])

			// The filtered map is what RollingSync uses to decide which Applications get an Operation.
			result := progressivesync.NewManager(r.Client, r.Client, nil, r).SyncDesiredApplications(logCtx, appSet, appSyncMap, apps)
			require.Len(t, result, 2)
			if tc.wantHeld {
				assert.Nil(t, result[0].Operation, "no sync operation must be written while a sync window blocks automated syncs")
			} else {
				require.NotNil(t, result[0].Operation)
				assert.True(t, result[0].Operation.InitiatedBy.Automated)
			}
			assert.Nil(t, result[1].Operation)
		})
	}
}

// Regression test for https://github.com/argoproj/argo-cd/issues/29307 through a full reconcile:
// while the project's sync windows deny automated syncs, RollingSync leaves the Application Pending
// without an Operation and requeues, and it triggers the sync once the window no longer applies.
func TestReconcileRollingSyncRespectsSyncWindows(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	project := &v1alpha1.AppProject{
		Name: "default", Namespace: "argocd",
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos:  []string{"*"},
			Destinations: []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
			SyncWindows: v1alpha1.SyncWindows{{
				Kind: "deny", Schedule: "* * * * *", Duration: "1h", ManualSync: true, Applications: []string{"*"},
			}},
		},
	}
	appSet := &v1alpha1.ApplicationSet{
		Name: "appset", Namespace: "argocd",
		Spec: v1alpha1.ApplicationSetSpec{
			GoTemplate: true,
			Generators: []v1alpha1.ApplicationSetGenerator{{
				List: &v1alpha1.ListGenerator{
					Elements: []apiextensionsv1.JSON{{Raw: []byte(`{"name": "app"}`)}},
				},
			}},
			Strategy: &v1alpha1.ApplicationSetStrategy{
				Type: "RollingSync",
				RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
					Steps: []v1alpha1.ApplicationSetRolloutStep{{
						MatchExpressions: []v1alpha1.ApplicationMatchExpression{{Key: "env", Operator: "In", Values: []string{"prod"}}},
					}},
				},
			},
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
					Name:      "{{.name}}",
					Namespace: "argocd",
					Labels:    map[string]string{"env": "prod"},
				},
				Spec: v1alpha1.ApplicationSpec{
					Source:      &v1alpha1.ApplicationSource{RepoURL: "https://github.com/argoproj/argocd-example-apps", Path: "guestbook"},
					Project:     "default",
					Destination: v1alpha1.ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "guestbook"},
				},
			},
		},
	}

	kubeclientset := getDefaultTestClientSet()
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(appSet, project).
		WithStatusSubresource(appSet).
		WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).
		Build()
	argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)
	clusterInformer, err := settings.NewClusterInformer(kubeclientset, "argocd")
	require.NoError(t, err)

	r := &ApplicationSetReconciler{
		Client:                 client,
		Scheme:                 scheme,
		Renderer:               &utils.Render{},
		Recorder:               record.NewFakeRecorder(10),
		Generators:             map[string]generators.Generator{"List": generators.NewListGenerator()},
		ArgoDB:                 argodb,
		KubeClientset:          kubeclientset,
		Policy:                 v1alpha1.ApplicationsSyncPolicySync,
		ArgoCDNamespace:        "argocd",
		Metrics:                appsetmetrics.NewFakeAppsetMetrics(),
		ClusterInformer:        clusterInformer,
		EnableProgressiveSyncs: true,
	}
	r.ProgressiveSyncManager = progressivesync.NewManager(client, client, nil, r)

	req := ctrl.Request{NamespacedName: crtclient.ObjectKeyFromObject(appSet)}
	appKey := crtclient.ObjectKey{Namespace: "argocd", Name: "app"}

	// The first reconcile creates the Application and the second one moves it to Pending. Without a
	// sync window, the third one would write the sync Operation.
	var res ctrl.Result
	for range 3 {
		var err error
		res, err = r.Reconcile(t.Context(), req)
		require.NoError(t, err)
	}
	assert.Equal(t, ReconcileRequeueOnSyncWindow, res.RequeueAfter, "the appset must be requeued so the sync starts once the window opens")

	var app v1alpha1.Application
	require.NoError(t, client.Get(t.Context(), appKey, &app))
	assert.Nil(t, app.Operation, "RollingSync must not start an operation that the sync window blocks")

	var updatedAppSet v1alpha1.ApplicationSet
	require.NoError(t, client.Get(t.Context(), req.NamespacedName, &updatedAppSet))
	require.Len(t, updatedAppSet.Status.ApplicationStatus, 1)
	assert.Equal(t, v1alpha1.ProgressiveSyncPending, updatedAppSet.Status.ApplicationStatus[0].Status)

	// Once the window no longer denies the sync, the next reconcile triggers it.
	require.NoError(t, client.Get(t.Context(), crtclient.ObjectKeyFromObject(project), project))
	project.Spec.SyncWindows = nil
	require.NoError(t, client.Update(t.Context(), project))

	res, err = r.Reconcile(t.Context(), req)
	require.NoError(t, err)
	assert.Zero(t, res.RequeueAfter)
	require.NoError(t, client.Get(t.Context(), appKey, &app))
	require.NotNil(t, app.Operation)
	assert.True(t, app.Operation.InitiatedBy.Automated)
}
