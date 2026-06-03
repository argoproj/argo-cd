package e2e

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/applicationsets"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	TransitionTimeout = 60 * time.Second
)

func TestApplicationSetProgressiveSyncStep(t *testing.T) {
	if os.Getenv("ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS") != "true" {
		t.Skip("Skipping progressive sync tests - env variable not set to enable progressive sync")
	}
	expectedDevApp := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app1-dev",
			Namespace: fixture.TestNamespace(),
			Labels: map[string]string{
				"environment": "dev",
			},
			Finalizers: []string{
				"resources-finalizer.argocd.argoproj.io",
			},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				Path:           "guestbook",
				TargetRevision: "HEAD",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "app1",
			},
		},
	}

	expectedStageApp := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app2-staging",
			Namespace: fixture.TestNamespace(),
			Labels: map[string]string{
				"environment": "staging",
			},
			Finalizers: []string{
				"resources-finalizer.argocd.argoproj.io",
			},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				Path:           "guestbook",
				TargetRevision: "HEAD",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "app2",
			},
		},
	}
	expectedProdApp := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app3-prod",
			Namespace: fixture.TestNamespace(),
			Labels: map[string]string{
				"environment": "prod",
			},
			Finalizers: []string{
				"resources-finalizer.argocd.argoproj.io",
			},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				Path:           "guestbook",
				TargetRevision: "HEAD",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "app3",
			},
		},
	}

	Given(t).
		When().
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "progressive-sync-apps",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				GoTemplate: true,
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
						Name:      "{{.name}}-{{.environment}}",
						Namespace: fixture.TestNamespace(),
						Labels: map[string]string{
							"environment": "{{.environment}}",
						},
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							Path:           "guestbook",
							TargetRevision: "HEAD",
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "{{.name}}",
						},
						SyncPolicy: &v1alpha1.SyncPolicy{
							SyncOptions: v1alpha1.SyncOptions{"CreateNamespace=true"},
						},
					},
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						List: &v1alpha1.ListGenerator{
							Elements: []apiextensionsv1.JSON{
								{Raw: []byte(`{"name": "app1", "environment": "dev"}`)},
								{Raw: []byte(`{"name": "app2", "environment": "staging"}`)},
								{Raw: []byte(`{"name": "app3", "environment": "prod"}`)},
							},
						},
					},
				},
				Strategy: &v1alpha1.ApplicationSetStrategy{
					Type: "RollingSync",
					RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
						Steps: generateStandardRolloutSyncSteps(),
					},
				},
			},
		}).
		Then().
		And(func() {
			t.Log("ApplicationSet created ")
		}).
		Expect(ApplicationsExist([]v1alpha1.Application{expectedDevApp, expectedStageApp, expectedProdApp})).
		And(func() {
			t.Log("All applications exist")
		}).
		ExpectWithDuration(CheckApplicationInRightSteps("1", []string{"app1-dev"}), TransitionTimeout).
		ExpectWithDuration(CheckApplicationInRightSteps("2", []string{"app2-staging"}), time.Second*5).
		ExpectWithDuration(CheckApplicationInRightSteps("3", []string{"app3-prod"}), time.Second*5).
		// cleanup
		When().
		Delete(metav1.DeletePropagationForeground).
		Then().
		ExpectWithDuration(ApplicationsDoNotExist([]v1alpha1.Application{expectedDevApp, expectedStageApp, expectedProdApp}), time.Minute)
}

func TestProgressiveSyncHealthGating(t *testing.T) {
	if os.Getenv("ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS") != "true" {
		t.Skip("Skipping progressive sync tests - ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS not enabled")
	}
	expectedDevApp := generateExpectedApp("prog-", "progressive-sync/", "dev", "dev", "")
	expectedStageApp := generateExpectedApp("prog-", "progressive-sync/", "staging", "staging", "")
	expectedProdApp := generateExpectedApp("prog-", "progressive-sync/", "prod", "prod", "")

	expectedStatusWave1 := map[string]v1alpha1.ApplicationSetApplicationStatus{
		"prog-dev": {
			Application: "prog-dev",
			Status:      v1alpha1.ProgressiveSyncProgressing,
		},
		"prog-staging": {
			Application: "prog-staging",
			Status:      v1alpha1.ProgressiveSyncWaiting,
		},
		"prog-prod": {
			Application: "prog-prod",
			Status:      v1alpha1.ProgressiveSyncWaiting,
		},
	}

	expectedStatusWave2 := map[string]v1alpha1.ApplicationSetApplicationStatus{
		"prog-dev": {
			Application: "prog-dev",
			Status:      v1alpha1.ProgressiveSyncHealthy,
		},
		"prog-staging": {
			Application: "prog-staging",
			Status:      v1alpha1.ProgressiveSyncProgressing,
		},
		"prog-prod": {
			Application: "prog-prod",
			Status:      v1alpha1.ProgressiveSyncWaiting,
		},
	}

	expectedStatusWave3 := map[string]v1alpha1.ApplicationSetApplicationStatus{
		"prog-dev": {
			Application: "prog-dev",
			Status:      v1alpha1.ProgressiveSyncHealthy,
		},
		"prog-staging": {
			Application: "prog-staging",
			Status:      v1alpha1.ProgressiveSyncHealthy,
		},
		"prog-prod": {
			Application: "prog-prod",
			Status:      v1alpha1.ProgressiveSyncProgressing,
		},
	}

	expectedAllHealthy := map[string]v1alpha1.ApplicationSetApplicationStatus{
		"prog-dev": {
			Application: "prog-dev",
			Status:      v1alpha1.ProgressiveSyncHealthy,
		},
		"prog-staging": {
			Application: "prog-staging",
			Status:      v1alpha1.ProgressiveSyncHealthy,
		},
		"prog-prod": {
			Application: "prog-prod",
			Status:      v1alpha1.ProgressiveSyncHealthy,
		},
	}

	Given(t).
		When().
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "progressive-sync-gating",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				GoTemplate: true,
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
						Name:      "prog-{{.environment}}",
						Namespace: fixture.TestNamespace(),
						Labels: map[string]string{
							"environment": "{{.environment}}",
						},
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        fixture.RepoURL(fixture.RepoURLTypeFile),
							Path:           "progressive-sync/{{.environment}}",
							TargetRevision: "HEAD",
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "prog-{{.environment}}",
						},
						SyncPolicy: &v1alpha1.SyncPolicy{
							SyncOptions: v1alpha1.SyncOptions{"CreateNamespace=true"},
						},
					},
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						List: &v1alpha1.ListGenerator{
							Elements: []apiextensionsv1.JSON{
								{Raw: []byte(`{"environment": "dev"}`)},
								{Raw: []byte(`{"environment": "staging"}`)},
								{Raw: []byte(`{"environment": "prod"}`)},
							},
						},
					},
				},
				Strategy: &v1alpha1.ApplicationSetStrategy{
					Type: "RollingSync",
					RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
						Steps: generateStandardRolloutSyncSteps(),
					},
				},
			},
		}).
		Then().
		Expect(ApplicationsExist([]v1alpha1.Application{expectedDevApp, expectedStageApp, expectedProdApp})).
		And(func() {
			t.Log("ApplicationSet created")
			t.Log("Checking Dev app should be stuck in Progressing (invalid image)")
			t.Log("Verifying staging and prod are Waiting")
		}).
		ExpectWithDuration(CheckProgressiveSyncStatusCodeOfApplications(expectedStatusWave1), TransitionTimeout).
		And(func() {
			// Patch deployment to use valid image
			fixture.Patch(t, "progressive-sync/dev/deployment.yaml", `[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value": "quay.io/argoprojlabs/argocd-e2e-container:0.1"}]`)
			// Refresh the app to detect git changes
			_, err := fixture.RunCli("app", "get", "prog-dev", "--refresh")
			require.NoError(t, err)
			t.Log("After patching image and refreshing, Dev app should progress to Healthy")
			t.Log("Staging app should now be in Progressing, and prod is waiting")
		}).
		ExpectWithDuration(CheckProgressiveSyncStatusCodeOfApplications(expectedStatusWave2), TransitionTimeout).
		And(func() {
			// Patch deployment to use valid image
			fixture.Patch(t, "progressive-sync/staging/deployment.yaml", `[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value": "quay.io/argoprojlabs/argocd-e2e-container:0.1"}]`)
			// Refresh the app to detect git changes
			_, err := fixture.RunCli("app", "get", "prog-staging", "--refresh")
			require.NoError(t, err)
			t.Log("Dev and staging are now Healthy")
			t.Log("check Prod app is progressing")
		}).
		ExpectWithDuration(CheckProgressiveSyncStatusCodeOfApplications(expectedStatusWave3), TransitionTimeout).
		And(func() {
			// Patch deployment to use valid image
			fixture.Patch(t, "progressive-sync/prod/deployment.yaml", `[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value": "quay.io/argoprojlabs/argocd-e2e-container:0.1"}]`)
			// Refresh the app to detect git changes
			_, err := fixture.RunCli("app", "get", "prog-prod", "--refresh")
			require.NoError(t, err)
		}).
		ExpectWithDuration(CheckProgressiveSyncStatusCodeOfApplications(expectedAllHealthy), TransitionTimeout).
		And(func() {
			t.Log("progressive sync verified")
			t.Log("Dev progressed first")
			t.Log("Staging waited until Dev was Healthy")
			t.Log("Prod waited until Staging was Healthy")
		}).
		// Cleanup
		When().
		Delete(metav1.DeletePropagationForeground).
		Then().
		ExpectWithDuration(ApplicationsDoNotExist([]v1alpha1.Application{expectedDevApp, expectedStageApp, expectedProdApp}), TransitionTimeout)
}

func TestNoApplicationStatusWhenNoSteps(t *testing.T) {
	if os.Getenv("ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS") != "true" {
		t.Skip("Skipping progressive sync tests - ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS not enabled")
	}

	expectedConditions := []v1alpha1.ApplicationSetCondition{
		{
			Type:    v1alpha1.ApplicationSetConditionErrorOccurred,
			Status:  v1alpha1.ApplicationSetConditionStatusFalse,
			Message: "All applications have been generated successfully",
			Reason:  v1alpha1.ApplicationSetReasonApplicationSetUpToDate,
		},
		{
			Type:    v1alpha1.ApplicationSetConditionParametersGenerated,
			Status:  v1alpha1.ApplicationSetConditionStatusTrue,
			Message: "Successfully generated parameters for all Applications",
			Reason:  v1alpha1.ApplicationSetReasonParametersGenerated,
		},
		{
			Type:    v1alpha1.ApplicationSetConditionResourcesUpToDate,
			Status:  v1alpha1.ApplicationSetConditionStatusTrue,
			Message: "All applications have been generated successfully",
			Reason:  v1alpha1.ApplicationSetReasonApplicationSetUpToDate,
		},
		{
			Type:    v1alpha1.ApplicationSetConditionRolloutProgressing,
			Status:  v1alpha1.ApplicationSetConditionStatusFalse,
			Message: "ApplicationSet Rollout has completed",
			Reason:  v1alpha1.ApplicationSetReasonApplicationSetRolloutComplete,
		},
	}

	expectedApps := []v1alpha1.Application{
		generateExpectedApp("prog-", "progressive-sync/", "dev", "dev", ""),
		generateExpectedApp("prog-", "progressive-sync/", "staging", "staging", ""),
		generateExpectedApp("prog-", "progressive-sync/", "prod", "prod", ""),
	}
	Given(t).
		When().
		Create(appSetInvalidStepConfiguration).
		Then().
		Expect(ApplicationSetHasConditions(expectedConditions)). // TODO: when no steps created, condition should reflect that.
		Expect(ApplicationSetDoesNotHaveApplicationStatus()).
		// Cleanup
		When().
		Delete(metav1.DeletePropagationForeground).
		Then().
		ExpectWithDuration(ApplicationsDoNotExist(expectedApps), TransitionTimeout)
}

func TestNoApplicationStatusWhenNoApplications(t *testing.T) {
	if os.Getenv("ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS") != "true" {
		t.Skip("Skipping progressive sync tests - ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS not enabled")
	}
	expectedApps := []v1alpha1.Application{
		generateExpectedApp("prog-", "progressive-sync/", "dev", "dev", ""),
		generateExpectedApp("prog-", "progressive-sync/", "staging", "staging", ""),
		generateExpectedApp("prog-", "progressive-sync/", "prod", "prod", ""),
	}
	Given(t).
		When().
		Create(appSetWithEmptyGenerator).
		Then().
		Expect(ApplicationsDoNotExist(expectedApps)).
		Expect(ApplicationSetDoesNotHaveApplicationStatus()).
		// Cleanup
		When().
		Delete(metav1.DeletePropagationForeground).
		Then().
		Expect(ApplicationsDoNotExist(expectedApps))
}

func TestProgressiveSyncMultipleAppsPerStepWithReverseDeletionOrder(t *testing.T) {
	if os.Getenv("ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS") != "true" {
		t.Skip("Skipping progressive sync tests - ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS not enabled")
	}
	// Define app groups by step (for reverse deletion: prod -> staging -> dev)
	prodApps := []string{"prog-ship", "prog-run"}
	stagingApps := []string{"prog-verify", "prog-validate"}
	devApps := []string{"prog-sketch", "prog-build"}
	testFinalizer := "test.e2e.argoproj.io/wait-for-verification"
	// Create expected app definitions for existence checks
	expectedProdApps := []v1alpha1.Application{
		generateExpectedApp("prog-", "progressive-sync/multiple-apps-in-step/prod/", "ship", "prod", testFinalizer),
		generateExpectedApp("prog-", "progressive-sync/multiple-apps-in-step/prod/", "run", "prod", testFinalizer),
	}
	expectedStagingApps := []v1alpha1.Application{
		generateExpectedApp("prog-", "progressive-sync/multiple-apps-in-step/staging/", "verify", "staging", testFinalizer),
		generateExpectedApp("prog-", "progressive-sync/multiple-apps-in-step/staging/", "validate", "staging", testFinalizer),
	}
	expectedDevApps := []v1alpha1.Application{
		generateExpectedApp("prog-", "progressive-sync/multiple-apps-in-step/dev/", "sketch", "dev", testFinalizer),
		generateExpectedApp("prog-", "progressive-sync/multiple-apps-in-step/dev/", "build", "dev", testFinalizer),
	}
	var allExpectedApps []v1alpha1.Application
	allExpectedApps = append(allExpectedApps, expectedProdApps...)
	allExpectedApps = append(allExpectedApps, expectedStagingApps...)
	allExpectedApps = append(allExpectedApps, expectedDevApps...)

	Given(t).
		When().
		Create(appSetWithReverseDeletionOrder).
		Then().
		And(func() {
			t.Log("ApplicationSet with reverse deletion order created")
		}).
		Expect(ApplicationsExist(allExpectedApps)).
		Expect(CheckApplicationInRightSteps("1", []string{"prog-sketch", "prog-build"})).
		Expect(CheckApplicationInRightSteps("2", []string{"prog-verify", "prog-validate"})).
		Expect(CheckApplicationInRightSteps("3", []string{"prog-ship", "prog-run"})).
		ExpectWithDuration(ApplicationSetHasApplicationStatus(6), TransitionTimeout).
		And(func() {
			t.Log("All 6 applications exist and are tracked in ApplicationSet status")
		}).
		// Delete the ApplicationSet
		When().
		Delete(metav1.DeletePropagationBackground).
		Then().
		And(func() {
			t.Log("Starting deletion - should happen in reverse order: prod -> staging -> dev")
			t.Log("Wave 1: Verifying prod apps (prog-ship, prog-run) are deleted first")
		}).
		// Wave 1: Prod apps should be deleted first, others untouched
		Expect(ApplicationDeletionStarted(prodApps)).
		Expect(ApplicationsExistAndNotBeingDeleted(append(stagingApps, devApps...))).
		And(func() {
			t.Log("Wave 1 confirmed: prod apps deleting/gone, staging and dev apps still exist and not being deleted")
		}).
		When().
		RemoveFinalizerFromApps(prodApps, testFinalizer).
		Then().
		And(func() {
			t.Log("removed finalizer from prod apps, confirm prod apps deleted")
			t.Log("Wave 2: Verifying staging apps (prog-verify, prog-validate) are deleted second")
		}).
		// Wave 2: Staging apps being deleted, dev untouched
		ExpectWithDuration(ApplicationsDoNotExist(expectedProdApps), TransitionTimeout).
		Expect(ApplicationDeletionStarted(stagingApps)).
		Expect(ApplicationsExistAndNotBeingDeleted(devApps)).
		And(func() {
			t.Log("Wave 2 confirmed: prod apps gone, staging apps deleting/gone, dev apps still exist and not being deleted")
		}).
		When().
		RemoveFinalizerFromApps(stagingApps, testFinalizer).
		Then().
		And(func() {
			t.Log("removed finalizer from staging apps, confirm staging apps deleted")
			t.Log("Wave 3: Verifying dev apps (prog-sketch, prog-build) are deleted last")
		}).
		// Wave 3: Dev apps deleted last
		ExpectWithDuration(ApplicationsDoNotExist(expectedStagingApps), TransitionTimeout).
		Expect(ApplicationDeletionStarted(devApps)).
		And(func() {
			t.Log("Wave 3 confirmed: all prod and staging apps gone, dev apps deleting/gone")
		}).
		When().
		RemoveFinalizerFromApps(devApps, testFinalizer).
		Then().
		And(func() {
			t.Log("removed finalizer from dev apps, confirm dev apps deleted")
			t.Log("Waiting for final cleanup - all applications should be deleted")
		}).
		// Final: All applications should be gone
		ExpectWithDuration(ApplicationsDoNotExist(allExpectedApps), time.Minute).
		And(func() {
			t.Log("Reverse deletion order verified successfully!")
			t.Log("Deletion sequence was: prod -> staging -> dev")
		})
}

var appSetInvalidStepConfiguration = v1alpha1.ApplicationSet{
	ObjectMeta: metav1.ObjectMeta{
		Name: "invalid-step-configuration",
	},
	TypeMeta: metav1.TypeMeta{
		Kind:       "ApplicationSet",
		APIVersion: "argoproj.io/v1alpha1",
	},
	Spec: v1alpha1.ApplicationSetSpec{
		GoTemplate: true,
		Template: v1alpha1.ApplicationSetTemplate{
			ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
				Name:      "prog-{{.environment}}",
				Namespace: fixture.TestNamespace(),
				Labels: map[string]string{
					"environment": "{{.environment}}",
				},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "default",
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeFile),
					Path:           "progressive-sync/{{.environment}}",
					TargetRevision: "HEAD",
				},
				Destination: v1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: "prog-{{.environment}}",
				},
				SyncPolicy: &v1alpha1.SyncPolicy{
					SyncOptions: v1alpha1.SyncOptions{"CreateNamespace=true"},
				},
			},
		},
		Generators: []v1alpha1.ApplicationSetGenerator{
			{
				List: &v1alpha1.ListGenerator{
					Elements: []apiextensionsv1.JSON{
						{Raw: []byte(`{"environment": "dev"}`)},
						{Raw: []byte(`{"environment": "staging"}`)},
						{Raw: []byte(`{"environment": "prod"}`)},
					},
				},
			},
		},
		Strategy: &v1alpha1.ApplicationSetStrategy{
			Type: "RollingSync",
			RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
				Steps: []v1alpha1.ApplicationSetRolloutStep{
					// Empty Steps with Rolling Sync shouldn't trigger
				},
			},
		},
	},
}

var appSetWithEmptyGenerator = v1alpha1.ApplicationSet{
	ObjectMeta: metav1.ObjectMeta{
		Name: "appset-empty-generator",
	},
	TypeMeta: metav1.TypeMeta{
		Kind:       "ApplicationSet",
		APIVersion: "argoproj.io/v1alpha1",
	},
	Spec: v1alpha1.ApplicationSetSpec{
		GoTemplate: true,
		Template: v1alpha1.ApplicationSetTemplate{
			ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
				Name:      "prog-{{.environment}}",
				Namespace: fixture.TestNamespace(),
				Labels: map[string]string{
					"environment": "{{.environment}}",
				},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "default",
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeFile),
					Path:           "progressive-sync/{{.environment}}",
					TargetRevision: "HEAD",
				},
				Destination: v1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: "prog-{{.environment}}",
				},
				SyncPolicy: &v1alpha1.SyncPolicy{
					SyncOptions: v1alpha1.SyncOptions{"CreateNamespace=true"},
				},
			},
		},
		Generators: []v1alpha1.ApplicationSetGenerator{
			{
				List: &v1alpha1.ListGenerator{
					Elements: []apiextensionsv1.JSON{
						// Empty Generator
					},
				},
			},
		},
		Strategy: &v1alpha1.ApplicationSetStrategy{
			Type: "RollingSync",
			RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
				Steps: generateStandardRolloutSyncSteps(),
			},
		},
	},
}

var appSetWithReverseDeletionOrder = v1alpha1.ApplicationSet{
	ObjectMeta: metav1.ObjectMeta{
		Name: "appset-reverse-deletion-order",
	},
	TypeMeta: metav1.TypeMeta{
		Kind:       "ApplicationSet",
		APIVersion: "argoproj.io/v1alpha1",
	},
	Spec: v1alpha1.ApplicationSetSpec{
		GoTemplate: true,
		Template: v1alpha1.ApplicationSetTemplate{
			ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
				Name:      "prog-{{.name}}",
				Namespace: fixture.TestNamespace(),
				Labels: map[string]string{
					"environment": "{{.environment}}",
				},
				Finalizers: []string{
					"resources-finalizer.argocd.argoproj.io",
					"test.e2e.argoproj.io/wait-for-verification",
				},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "default",
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeFile),
					Path:           "progressive-sync/multiple-apps-in-step/{{.environment}}/{{.name}}",
					TargetRevision: "HEAD",
				},
				Destination: v1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: "prog-{{.name}}",
				},
				SyncPolicy: &v1alpha1.SyncPolicy{
					SyncOptions: v1alpha1.SyncOptions{"CreateNamespace=true"},
				},
			},
		},
		Generators: []v1alpha1.ApplicationSetGenerator{
			{
				List: &v1alpha1.ListGenerator{
					Elements: []apiextensionsv1.JSON{
						{Raw: []byte(`{"environment": "dev", "name": "sketch"}`)},
						{Raw: []byte(`{"environment": "dev", "name": "build"}`)},
						{Raw: []byte(`{"environment": "staging", "name": "verify"}`)},
						{Raw: []byte(`{"environment": "staging", "name": "validate"}`)},
						{Raw: []byte(`{"environment": "prod", "name": "ship"}`)},
						{Raw: []byte(`{"environment": "prod", "name": "run"}`)},
					},
				},
			},
		},
		Strategy: &v1alpha1.ApplicationSetStrategy{
			Type: "RollingSync",
			RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
				Steps: generateStandardRolloutSyncSteps(),
			},
			DeletionOrder: "Reverse",
		},
	},
}

func generateExpectedApp(prefix string, path string, name string, envVar string, testFinalizer string) v1alpha1.Application {
	finalizers := []string{
		"resources-finalizer.argocd.argoproj.io",
	}
	if testFinalizer != "" {
		finalizers = append(finalizers, testFinalizer)
	}
	return v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      prefix + name,
			Namespace: fixture.TestNamespace(),
			Labels: map[string]string{
				"environment": envVar,
			},
			Finalizers: finalizers,
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        fixture.RepoURL(fixture.RepoURLTypeFile),
				Path:           path + name,
				TargetRevision: "HEAD",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: prefix + name,
			},
		},
	}
}

func generateStandardRolloutSyncSteps() []v1alpha1.ApplicationSetRolloutStep {
	return []v1alpha1.ApplicationSetRolloutStep{
		{
			MatchExpressions: []v1alpha1.ApplicationMatchExpression{
				{
					Key:      "environment",
					Operator: "In",
					Values:   []string{"dev"},
				},
			},
		},
		{
			MatchExpressions: []v1alpha1.ApplicationMatchExpression{
				{
					Key:      "environment",
					Operator: "In",
					Values:   []string{"staging"},
				},
			},
		},
		{
			MatchExpressions: []v1alpha1.ApplicationMatchExpression{
				{
					Key:      "environment",
					Operator: "In",
					Values:   []string{"prod"},
				},
			},
		},
	}
}
