package e2e

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/applicationsets"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	TransitionTimeout = 60 * time.Second
)

// This test uses an external git repo instead of file repo in the other tests
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
				RepoURL:        "https://github.com/ranakan19/test-yamls",
				Path:           "apps/app1",
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
				RepoURL:        "https://github.com/ranakan19/test-yamls",
				Path:           "apps/app2",
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
				RepoURL:        "https://github.com/ranakan19/test-yamls",
				Path:           "apps/app3",
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
							RepoURL:        "https://github.com/ranakan19/test-yamls",
							Path:           "apps/{{.name}}",
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
							Elements: []v1.JSON{
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
						Steps: []v1alpha1.ApplicationSetRolloutStep{
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
						},
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
		Delete().
		Then().
		ExpectWithDuration(ApplicationsDoNotExist([]v1alpha1.Application{expectedDevApp, expectedStageApp, expectedProdApp}), time.Minute)
}

func TestProgressiveSyncHealthGating(t *testing.T) {
	if os.Getenv("ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS") != "true" {
		t.Skip("Skipping progressive sync tests - ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS not enabled")
	}
	expectedDevApp := generateExpectedApp("prog-", "progressive-sync/", "dev", "dev")
	expectedStageApp := generateExpectedApp("prog-", "progressive-sync/", "staging", "staging")
	expectedProdApp := generateExpectedApp("prog-", "progressive-sync/", "prod", "prod")

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
							Elements: []v1.JSON{
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
						},
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
		Delete().
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
		generateExpectedApp("prog-", "progressive-sync/", "dev", "dev"),
		generateExpectedApp("prog-", "progressive-sync/", "staging", "staging"),
		generateExpectedApp("prog-", "progressive-sync/", "prod", "prod"),
	}
	Given(t).
		When().
		Create(appSetInvalidStepConfiguration).
		Then().
		Expect(ApplicationSetHasConditions(expectedConditions)). // TODO: when no steps created, condition should reflect that.
		Expect(ApplicationSetDoesNotHaveApplicationStatus()).
		// Cleanup
		When().
		Delete().
		Then().
		ExpectWithDuration(ApplicationsDoNotExist(expectedApps), TransitionTimeout)
}

func TestNoApplicationStatusWhenNoApplications(t *testing.T) {
	if os.Getenv("ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS") != "true" {
		t.Skip("Skipping progressive sync tests - ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS not enabled")
	}
	expectedApps := []v1alpha1.Application{
		generateExpectedApp("prog-", "progressive-sync/", "dev", "dev"),
		generateExpectedApp("prog-", "progressive-sync/", "staging", "staging"),
		generateExpectedApp("prog-", "progressive-sync/", "prod", "prod"),
	}
	Given(t).
		When().
		Create(appSetWithEmptyGenerator).
		Then().
		Expect(ApplicationsDoNotExist(expectedApps)).
		Expect(ApplicationSetDoesNotHaveApplicationStatus()).
		// Cleanup
		When().
		Delete().
		Then().
		Expect(ApplicationsDoNotExist(expectedApps))
}

func TestProgressiveSyncMultipleAppsPerStep(t *testing.T) {
	if os.Getenv("ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS") != "true" {
		t.Skip("Skipping progressive sync tests - ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS not enabled")
	}
	expectedApps := []v1alpha1.Application{
		generateExpectedApp("prog-", "progressive-sync/multiple-apps-in-step/dev/", "sketch", "dev"),
		generateExpectedApp("prog-", "progressive-sync/multiple-apps-in-step/dev/", "build", "dev"),
		generateExpectedApp("prog-", "progressive-sync/multiple-apps-in-step/staging/", "verify", "staging"),
		generateExpectedApp("prog-", "progressive-sync/multiple-apps-in-step/staging/", "validate", "staging"),
		generateExpectedApp("prog-", "progressive-sync/multiple-apps-in-step/prod/", "ship", "prod"),
		generateExpectedApp("prog-", "progressive-sync/multiple-apps-in-step/prod/", "run", "prod"),
	}
	Given(t).
		When().
		Create(appSetWithMultipleAppsInEachStep).
		Then().
		Expect(ApplicationsExist(expectedApps)).
		Expect(CheckApplicationInRightSteps("1", []string{"prog-sketch", "prog-build"})).
		Expect(CheckApplicationInRightSteps("2", []string{"prog-verify", "prog-validate"})).
		Expect(CheckApplicationInRightSteps("3", []string{"prog-ship", "prog-run"})).
		ExpectWithDuration(ApplicationSetHasApplicationStatus(6), TransitionTimeout).
		// Cleanup
		When().
		Delete().
		Then().
		Expect(ApplicationsDoNotExist(expectedApps))
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
					Elements: []v1.JSON{
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
					Elements: []v1.JSON{
						// Empty Generator
					},
				},
			},
		},
		Strategy: &v1alpha1.ApplicationSetStrategy{
			Type: "RollingSync",
			RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
				Steps: []v1alpha1.ApplicationSetRolloutStep{
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
				},
			},
		},
	},
}

var appSetWithMultipleAppsInEachStep = v1alpha1.ApplicationSet{
	ObjectMeta: metav1.ObjectMeta{
		Name: "progressive-sync-multi-apps",
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
					Elements: []v1.JSON{
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
				Steps: []v1alpha1.ApplicationSetRolloutStep{
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
				},
			},
		},
	},
}

func generateExpectedApp(prefix string, path string, name string, envVar string) v1alpha1.Application {
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
			Finalizers: []string{
				"resources-finalizer.argocd.argoproj.io",
			},
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
