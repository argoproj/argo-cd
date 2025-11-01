package e2e

import (
	"log"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/applicationsets"
)

func init() {
	// Enable progressive sync feature for all tests in this file
	if err := fixture.SetParamInSettingConfigMap("applicationsetcontroller.enable.progressive.syncs", "true"); err != nil {
		log.Fatalf("failed to enable progressive sync: %v", err)
	}
}

func TestProgressiveSyncBasicTwoStepRollout(t *testing.T) {
	expectedConditionSuccess := []v1alpha1.ApplicationSetCondition{
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
	}

	appStep1 := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "progressive-sync-dev",
			Namespace:  "argocd",
			Finalizers: []string{v1alpha1.ResourcesFinalizerName},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
		},
	}

	appStep2 := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "progressive-sync-prod",
			Namespace:  "argocd",
			Finalizers: []string{v1alpha1.ResourcesFinalizerName},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
		},
	}

	Given(t).
		When().
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "progressive-sync-basic",
				Namespace: "argocd-e2e",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				GoTemplate: true,
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						List: &v1alpha1.ListGenerator{
							Elements: []apiextensionsv1.JSON{
								{
									Raw: []byte(`{"name":"progressive-sync-dev","env":"dev","index":"1"}`),
								},
								{
									Raw: []byte(`{"name":"progressive-sync-prod","env":"prod","index":"2"}`),
								},
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
										Key:      "env",
										Operator: "In",
										Values:   []string{"dev"},
									},
								},
								MaxUpdate: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
							},
							{
								MatchExpressions: []v1alpha1.ApplicationMatchExpression{
									{
										Key:      "env",
										Operator: "In",
										Values:   []string{"prod"},
									},
								},
								MaxUpdate: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
							},
						},
					},
				},
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
						Name: "{{.name}}",
						Labels: map[string]string{
							"env": "{{.env}}",
						},
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "guestbook",
						},
					},
				},
			},
		}).
		Then().
		Expect(OnlyApplicationsExist([]v1alpha1.Application{appStep1}, []v1alpha1.Application{appStep2})).
		Expect(ApplicationSetHasProgressiveStatus("progressive-sync-basic", "progressive-sync-dev", v1alpha1.ProgressiveSyncHealthy)).
		Expect(ApplicationSetHasConditions("progressive-sync-basic", expectedConditionSuccess)).
		When().
		Then().
		Expect(ApplicationsExist([]v1alpha1.Application{appStep1, appStep2})).
		Expect(ApplicationSetHasProgressiveStatus("progressive-sync-basic", "progressive-sync-prod", v1alpha1.ProgressiveSyncHealthy))
}

func TestProgressiveSyncThreeStepRollout(t *testing.T) {
	expectedConditionSuccess := []v1alpha1.ApplicationSetCondition{
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
	}

	appStep1 := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "three-step-dev",
			Namespace:  "argocd",
			Finalizers: []string{v1alpha1.ResourcesFinalizerName},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
		},
	}

	appStep2 := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "three-step-staging",
			Namespace:  "argocd",
			Finalizers: []string{v1alpha1.ResourcesFinalizerName},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
		},
	}

	appStep3 := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "three-step-prod",
			Namespace:  "argocd",
			Finalizers: []string{v1alpha1.ResourcesFinalizerName},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
		},
	}

	Given(t).
		When().
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "progressive-sync-three-step",
				Namespace: "argocd-e2e",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				GoTemplate: true,
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						List: &v1alpha1.ListGenerator{
							Elements: []apiextensionsv1.JSON{
								{
									Raw: []byte(`{"name":"three-step-dev","env":"dev","index":"1"}`),
								},
								{
									Raw: []byte(`{"name":"three-step-staging","env":"staging","index":"2"}`),
								},
								{
									Raw: []byte(`{"name":"three-step-prod","env":"prod","index":"3"}`),
								},
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
										Key:      "env",
										Operator: "In",
										Values:   []string{"dev"},
									},
								},
								MaxUpdate: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
							},
							{
								MatchExpressions: []v1alpha1.ApplicationMatchExpression{
									{
										Key:      "env",
										Operator: "In",
										Values:   []string{"staging"},
									},
								},
								MaxUpdate: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
							},
							{
								MatchExpressions: []v1alpha1.ApplicationMatchExpression{
									{
										Key:      "env",
										Operator: "In",
										Values:   []string{"prod"},
									},
								},
								MaxUpdate: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
							},
						},
					},
				},
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
						Name: "{{.name}}",
						Labels: map[string]string{
							"env": "{{.env}}",
						},
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "guestbook",
						},
					},
				},
			},
		}).
		Then().
		Expect(OnlyApplicationsExist([]v1alpha1.Application{appStep1}, []v1alpha1.Application{appStep2, appStep3})).
		Expect(ApplicationSetHasProgressiveStatus("progressive-sync-three-step", "three-step-dev", v1alpha1.ProgressiveSyncHealthy)).
		When().
		Then().
		Expect(OnlyApplicationsExist([]v1alpha1.Application{appStep1, appStep2}, []v1alpha1.Application{appStep3})).
		Expect(ApplicationSetHasProgressiveStatus("progressive-sync-three-step", "three-step-staging", v1alpha1.ProgressiveSyncHealthy)).
		When().
		Then().
		Expect(ApplicationsExist([]v1alpha1.Application{appStep1, appStep2, appStep3})).
		Expect(ApplicationSetHasProgressiveStatus("progressive-sync-three-step", "three-step-prod", v1alpha1.ProgressiveSyncHealthy)).
		Expect(ApplicationSetHasConditions("progressive-sync-three-step", expectedConditionSuccess))
}

func TestProgressiveSyncWithMaxUpdatePercentage(t *testing.T) {
	expectedConditionSuccess := []v1alpha1.ApplicationSetCondition{
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
	}

	app1 := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "max-update-percent-1",
			Namespace:  "argocd",
			Finalizers: []string{v1alpha1.ResourcesFinalizerName},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
		},
	}

	app2 := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "max-update-percent-2",
			Namespace:  "argocd",
			Finalizers: []string{v1alpha1.ResourcesFinalizerName},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
		},
	}

	app3 := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "max-update-percent-3",
			Namespace:  "argocd",
			Finalizers: []string{v1alpha1.ResourcesFinalizerName},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
		},
	}

	Given(t).
		When().
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "progressive-sync-max-update-percent",
				Namespace: "argocd-e2e",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				GoTemplate: true,
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						List: &v1alpha1.ListGenerator{
							Elements: []apiextensionsv1.JSON{
								{
									Raw: []byte(`{"name":"max-update-percent-1","env":"prod"}`),
								},
								{
									Raw: []byte(`{"name":"max-update-percent-2","env":"prod"}`),
								},
								{
									Raw: []byte(`{"name":"max-update-percent-3","env":"prod"}`),
								},
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
										Key:      "env",
										Operator: "In",
										Values:   []string{"prod"},
									},
								},
								MaxUpdate: &intstr.IntOrString{Type: intstr.String, StrVal: "50%"},
							},
						},
					},
				},
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
						Name: "{{.name}}",
						Labels: map[string]string{
							"env": "{{.env}}",
						},
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "guestbook",
						},
					},
				},
			},
		}).
		Then().
		Expect(ApplicationsExist([]v1alpha1.Application{app1, app2, app3})).
		Expect(ApplicationSetHasConditions("progressive-sync-max-update-percent", expectedConditionSuccess))
}

func TestProgressiveSyncDeletionAllAtOnce(t *testing.T) {
	appDev := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "deletion-allatonce-dev",
			Namespace:  "argocd",
			Finalizers: []string{v1alpha1.ResourcesFinalizerName},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
		},
	}

	appProd := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "deletion-allatonce-prod",
			Namespace:  "argocd",
			Finalizers: []string{v1alpha1.ResourcesFinalizerName},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
		},
	}

	Given(t).
		When().
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "progressive-sync-deletion-allatonce",
				Namespace: "argocd-e2e",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				GoTemplate: true,
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						List: &v1alpha1.ListGenerator{
							Elements: []apiextensionsv1.JSON{
								{
									Raw: []byte(`{"name":"deletion-allatonce-dev","env":"dev"}`),
								},
								{
									Raw: []byte(`{"name":"deletion-allatonce-prod","env":"prod"}`),
								},
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
										Key:      "env",
										Operator: "In",
										Values:   []string{"dev"},
									},
								},
								MaxUpdate: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
							},
							{
								MatchExpressions: []v1alpha1.ApplicationMatchExpression{
									{
										Key:      "env",
										Operator: "In",
										Values:   []string{"prod"},
									},
								},
								MaxUpdate: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
							},
						},
					},
				},
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
						Name: "{{.name}}",
						Labels: map[string]string{
							"env": "{{.env}}",
						},
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "guestbook",
						},
					},
				},
			},
		}).
		Then().
		Expect(ApplicationsExist([]v1alpha1.Application{appDev, appProd})).
		When().
		Delete().
		Then().
		Expect(ApplicationsDoNotExist([]v1alpha1.Application{appDev, appProd}))
}

func TestProgressiveSyncDeletionReverse(t *testing.T) {
	appDev := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "deletion-reverse-dev",
			Namespace:  "argocd",
			Finalizers: []string{v1alpha1.ResourcesFinalizerName},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
		},
	}

	appProd := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "deletion-reverse-prod",
			Namespace:  "argocd",
			Finalizers: []string{v1alpha1.ResourcesFinalizerName},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
		},
	}

	Given(t).
		When().
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "progressive-sync-deletion-reverse",
				Namespace: "argocd-e2e",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				GoTemplate: true,
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						List: &v1alpha1.ListGenerator{
							Elements: []apiextensionsv1.JSON{
								{
									Raw: []byte(`{"name":"deletion-reverse-dev","env":"dev"}`),
								},
								{
									Raw: []byte(`{"name":"deletion-reverse-prod","env":"prod"}`),
								},
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
										Key:      "env",
										Operator: "In",
										Values:   []string{"dev"},
									},
								},
								MaxUpdate: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
							},
							{
								MatchExpressions: []v1alpha1.ApplicationMatchExpression{
									{
										Key:      "env",
										Operator: "In",
										Values:   []string{"prod"},
									},
								},
								MaxUpdate: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
							},
						},
					},
				},
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
						Name: "{{.name}}",
						Labels: map[string]string{
							"env": "{{.env}}",
						},
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "guestbook",
						},
					},
				},
			},
		}).
		Then().
		Expect(ApplicationsExist([]v1alpha1.Application{appDev, appProd})).
		When().
		Delete().
		Then().
		Expect(ApplicationsDoNotExist([]v1alpha1.Application{appDev, appProd}))
}
