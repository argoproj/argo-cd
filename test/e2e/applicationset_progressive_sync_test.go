package e2e

import (
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/applicationsets"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"testing"
)

func TestApplicationSetProgressiveSync(t *testing.T) {
	if os.Getenv("ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS") != "true" {
		t.Skip("Skipping progressive sync tests - env variable not set to enable progressive sync")
	}
	expectedDevApp := v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app1-dev",
			Labels: map[string]string{
				"environment": "dev",
			},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL: "https://github.com/ranakan19/test-yamls",
				Path:    "apps/app1",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server: "https://kubernetes.default.svc",
			},
		},
	}

	expectedStageApp := v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app2-staging",
			Labels: map[string]string{
				"environment": "staging",
			},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL: "https://github.com/ranakan19/test-yamls",
				Path:    "apps/app2",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server: "https://kubernetes.default.svc",
			},
		},
	}
	expectedProdApp := v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app3-prod",
			Labels: map[string]string{
				"environment": "prod",
			},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL: "https://github.com/ranakan19/test-yamls",
				Path:    "apps/app3",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server: "https://kubernetes.default.svc",
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
						Name: "{{.name}}-{{.environment}}",
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
							Server: "https://kubernetes.default.svc",
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
		Expect(ApplicationsExist([]v1alpha1.Application{expectedDevApp, expectedStageApp, expectedProdApp})).
		// cleanup
		When().
		Delete().
		Then().
		Expect(ApplicationsDoNotExist([]v1alpha1.Application{expectedDevApp, expectedStageApp, expectedProdApp}))
}
