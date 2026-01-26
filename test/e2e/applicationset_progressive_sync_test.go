package e2e

import (
	"os"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/applicationsets"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestApplicationSetProgressiveSync(t *testing.T) {
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
			time.Sleep(5 * time.Second)
		}).
		Expect(ApplicationsExist([]v1alpha1.Application{expectedDevApp, expectedStageApp, expectedProdApp})).
		And(func() {
			t.Log("All applications exist")
			time.Sleep(30 * time.Second) // added to see the applications on UI
		}).
		ExpectWithDuration(CheckApplicationInRightSteps("1", []string{"app1-dev"}), time.Minute).
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
	expectedDevApp := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prog-dev",
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
				RepoURL:        fixture.RepoURL(fixture.RepoURLTypeFile),
				Path:           "progressive-sync/dev",
				TargetRevision: "HEAD",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "prog-dev",
			},
		},
	}

	expectedStageApp := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prog-staging",
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
				RepoURL:        fixture.RepoURL(fixture.RepoURLTypeFile),
				Path:           "progressive-sync/staging",
				TargetRevision: "HEAD",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "prog-staging",
			},
		},
	}
	expectedProdApp := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prog-prod",
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
				RepoURL:        fixture.RepoURL(fixture.RepoURLTypeFile),
				Path:           "progressive-sync/prod",
				TargetRevision: "HEAD",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "prog-prod",
			},
		},
	}

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
			time.Sleep(30 * time.Second)
		}).
		ExpectWithDuration(CheckProgressiveSyncStatusCodeOfApplications(expectedStatusWave1), time.Second*60).
		And(func() {
			t.Log("Dev app stuck in Progressing (invalid image)")
			t.Log("Verifying staging and prod are Waiting")
			// Patch deployment to use valid image
			fixture.Patch(t, "progressive-sync/dev/deployment.yaml", `[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value": "quay.io/argoprojlabs/argocd-e2e-container:0.1"}]`)
			// Refresh the app to detect git changes
			fixture.RunCli("app", "get", "prog-dev", "--refresh")
		}).
		ExpectWithDuration(CheckProgressiveSyncStatusCodeOfApplications(expectedStatusWave2), time.Second*60).
		And(func() {
			t.Log("Dev app is Healthy")
			t.Log("Staging app should now be in Progressing, and prod is waiting")
			// Patch deployment to use valid image
			fixture.Patch(t, "progressive-sync/staging/deployment.yaml", `[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value": "quay.io/argoprojlabs/argocd-e2e-container:0.1"}]`)
			// Refresh the app to detect git changes
			fixture.RunCli("app", "get", "prog-staging", "--refresh")
		}).
		ExpectWithDuration(CheckProgressiveSyncStatusCodeOfApplications(expectedStatusWave3), time.Second*60).
		And(func() {
			t.Log("Dev and staging are now Healthy")
			t.Log("prod app is progressing")
			// Patch deployment to use valid image
			fixture.Patch(t, "progressive-sync/prod/deployment.yaml", `[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value": "quay.io/argoprojlabs/argocd-e2e-container:0.1"}]`)
			// Refresh the app to detect git changes
			fixture.RunCli("app", "get", "prog-prod", "--refresh")
		}).
		ExpectWithDuration(CheckProgressiveSyncStatusCodeOfApplications(expectedAllHealthy), time.Second*60).
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
		ExpectWithDuration(ApplicationsDoNotExist([]v1alpha1.Application{expectedDevApp, expectedStageApp, expectedProdApp}), time.Minute)
}
