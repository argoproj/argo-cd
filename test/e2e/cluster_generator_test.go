package e2e

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/applicationsets"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture/applicationsets/utils"
)

func TestSimpleClusterGeneratorExternalNamespace(t *testing.T) {
	externalNamespace := string(utils.ArgoCDExternalNamespace)

	expectedApp := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "cluster1-guestbook",
			Namespace:  externalNamespace,
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
				Name:      "cluster1",
				Namespace: "guestbook",
			},
		},
	}

	var expectedAppNewNamespace *v1alpha1.Application
	var expectedAppNewMetadata *v1alpha1.Application

	Given(t).
		// Create a ClusterGenerator-based ApplicationSet
		When().
		CreateClusterSecret("my-secret", "cluster1", "https://kubernetes.default.svc").
		SwitchToExternalNamespace(utils.ArgoCDExternalNamespace).
		CreateNamespace(externalNamespace).
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-cluster-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{name}}-guestbook"},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: v1alpha1.ApplicationDestination{
							Name: "{{name}}",
							// Server:    "{{server}}",
							Namespace: "guestbook",
						},
					},
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						Clusters: &v1alpha1.ClusterGenerator{
							Selector: metav1.LabelSelector{
								MatchLabels: map[string]string{
									"argocd.argoproj.io/secret-type": "cluster",
								},
							},
						},
					},
				},
			},
		}).Then().Expect(ApplicationsExist([]v1alpha1.Application{expectedApp})).

		// Update the ApplicationSet template namespace, and verify it updates the Applications
		When().
		And(func() {
			expectedAppNewNamespace = expectedApp.DeepCopy()
			expectedAppNewNamespace.Spec.Destination.Namespace = "guestbook2"
		}).
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Template.Spec.Destination.Namespace = "guestbook2"
		}).Then().Expect(ApplicationsExist([]v1alpha1.Application{*expectedAppNewNamespace})).

		// Update the metadata fields in the appset template, and make sure it propagates to the apps
		When().
		And(func() {
			expectedAppNewMetadata = expectedAppNewNamespace.DeepCopy()
			expectedAppNewMetadata.ObjectMeta.Annotations = map[string]string{"annotation-key": "annotation-value"}
			expectedAppNewMetadata.ObjectMeta.Labels = map[string]string{
				"label-key": "label-value",
			}
		}).
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Template.Annotations = map[string]string{"annotation-key": "annotation-value"}
			appset.Spec.Template.Labels = map[string]string{
				"label-key": "label-value",
			}
		}).Then().Expect(ApplicationsExist([]v1alpha1.Application{*expectedAppNewMetadata})).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist([]v1alpha1.Application{*expectedAppNewNamespace}))
}

func TestSimpleClusterGenerator(t *testing.T) {
	expectedApp := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "cluster1-guestbook",
			Namespace:  fixture.TestNamespace(),
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
				Name:      "cluster1",
				Namespace: "guestbook",
			},
		},
	}

	var expectedAppNewNamespace *v1alpha1.Application
	var expectedAppNewMetadata *v1alpha1.Application

	Given(t).
		// Create a ClusterGenerator-based ApplicationSet
		When().
		CreateClusterSecret("my-secret", "cluster1", "https://kubernetes.default.svc").
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-cluster-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{name}}-guestbook"},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: v1alpha1.ApplicationDestination{
							Name: "{{name}}",
							// Server:    "{{server}}",
							Namespace: "guestbook",
						},
					},
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						Clusters: &v1alpha1.ClusterGenerator{
							Selector: metav1.LabelSelector{
								MatchLabels: map[string]string{
									"argocd.argoproj.io/secret-type": "cluster",
								},
							},
						},
					},
				},
			},
		}).Then().Expect(ApplicationsExist([]v1alpha1.Application{expectedApp})).

		// Update the ApplicationSet template namespace, and verify it updates the Applications
		When().
		And(func() {
			expectedAppNewNamespace = expectedApp.DeepCopy()
			expectedAppNewNamespace.Spec.Destination.Namespace = "guestbook2"
		}).
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Template.Spec.Destination.Namespace = "guestbook2"
		}).Then().Expect(ApplicationsExist([]v1alpha1.Application{*expectedAppNewNamespace})).

		// Update the metadata fields in the appset template, and make sure it propagates to the apps
		When().
		And(func() {
			expectedAppNewMetadata = expectedAppNewNamespace.DeepCopy()
			expectedAppNewMetadata.ObjectMeta.Annotations = map[string]string{"annotation-key": "annotation-value"}
			expectedAppNewMetadata.ObjectMeta.Labels = map[string]string{"label-key": "label-value"}
		}).
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Template.Annotations = map[string]string{"annotation-key": "annotation-value"}
			appset.Spec.Template.Labels = map[string]string{"label-key": "label-value"}
		}).Then().Expect(ApplicationsExist([]v1alpha1.Application{*expectedAppNewMetadata})).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist([]v1alpha1.Application{*expectedAppNewNamespace}))
}

func TestClusterGeneratorWithLocalCluster(t *testing.T) {
	expectedAppTemplate := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "in-cluster-guestbook",
			Namespace:  fixture.TestNamespace(),
			Finalizers: []string{v1alpha1.ResourcesFinalizerName},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			// Destination comes from appDestination below
		},
	}

	tests := []struct {
		name              string
		appsetDestination v1alpha1.ApplicationDestination
		appDestination    v1alpha1.ApplicationDestination
	}{
		{
			name: "specify local cluster by server field",
			appDestination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
			appsetDestination: v1alpha1.ApplicationDestination{
				Server:    "{{server}}",
				Namespace: "guestbook",
			},
		},
		{
			name: "specify local cluster by name field",
			appDestination: v1alpha1.ApplicationDestination{
				Name:      "in-cluster",
				Namespace: "guestbook",
			},
			appsetDestination: v1alpha1.ApplicationDestination{
				Name:      "{{name}}",
				Namespace: "guestbook",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var expectedAppNewNamespace *v1alpha1.Application
			var expectedAppNewMetadata *v1alpha1.Application

			// Create the expected application from the template, and copy in the destination from the test case
			expectedApp := *expectedAppTemplate.DeepCopy()
			expectedApp.Spec.Destination = test.appDestination

			Given(t).
				// Create a ClusterGenerator-based ApplicationSet
				When().
				Create(v1alpha1.ApplicationSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "in-cluster-generator",
					},
					Spec: v1alpha1.ApplicationSetSpec{
						Template: v1alpha1.ApplicationSetTemplate{
							ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{name}}-guestbook"},
							Spec: v1alpha1.ApplicationSpec{
								Project: "default",
								Source: &v1alpha1.ApplicationSource{
									RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
									TargetRevision: "HEAD",
									Path:           "guestbook",
								},
								Destination: test.appsetDestination,
							},
						},
						Generators: []v1alpha1.ApplicationSetGenerator{
							{
								Clusters: &v1alpha1.ClusterGenerator{},
							},
						},
					},
				}).Then().ExpectWithDuration(ApplicationsExist([]v1alpha1.Application{expectedApp}), 8*time.Minute).

				// Update the ApplicationSet template namespace, and verify it updates the Applications
				When().
				And(func() {
					expectedAppNewNamespace = expectedApp.DeepCopy()
					expectedAppNewNamespace.Spec.Destination.Namespace = "guestbook2"
				}).
				Update(func(appset *v1alpha1.ApplicationSet) {
					appset.Spec.Template.Spec.Destination.Namespace = "guestbook2"
				}).Then().Expect(ApplicationsExist([]v1alpha1.Application{*expectedAppNewNamespace})).

				// Update the metadata fields in the appset template, and make sure it propagates to the apps
				When().
				And(func() {
					expectedAppNewMetadata = expectedAppNewNamespace.DeepCopy()
					expectedAppNewMetadata.ObjectMeta.Annotations = map[string]string{"annotation-key": "annotation-value"}
					expectedAppNewMetadata.ObjectMeta.Labels = map[string]string{"label-key": "label-value"}
				}).
				Update(func(appset *v1alpha1.ApplicationSet) {
					appset.Spec.Template.Annotations = map[string]string{"annotation-key": "annotation-value"}
					appset.Spec.Template.Labels = map[string]string{"label-key": "label-value"}
				}).Then().Expect(ApplicationsExist([]v1alpha1.Application{*expectedAppNewMetadata})).

				// Delete the ApplicationSet, and verify it deletes the Applications
				When().
				Delete().Then().Expect(ApplicationsDoNotExist([]v1alpha1.Application{*expectedAppNewNamespace}))
		})
	}
}

func TestSimpleClusterGeneratorAddingCluster(t *testing.T) {
	expectedAppTemplate := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "{{name}}-guestbook",
			Namespace:  fixture.TestNamespace(),
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
				Name:      "{{name}}",
				Namespace: "guestbook",
			},
		},
	}

	expectedAppCluster1 := *expectedAppTemplate.DeepCopy()
	expectedAppCluster1.Spec.Destination.Name = "cluster1"
	expectedAppCluster1.Name = "cluster1-guestbook"

	expectedAppCluster2 := *expectedAppTemplate.DeepCopy()
	expectedAppCluster2.Spec.Destination.Name = "cluster2"
	expectedAppCluster2.Name = "cluster2-guestbook"

	Given(t).
		// Create a ClusterGenerator-based ApplicationSet
		When().
		CreateClusterSecret("my-secret", "cluster1", "https://kubernetes.default.svc").
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-cluster-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{name}}-guestbook"},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: v1alpha1.ApplicationDestination{
							Name: "{{name}}",
							// Server:    "{{server}}",
							Namespace: "guestbook",
						},
					},
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						Clusters: &v1alpha1.ClusterGenerator{
							Selector: metav1.LabelSelector{
								MatchLabels: map[string]string{
									"argocd.argoproj.io/secret-type": "cluster",
								},
							},
						},
					},
				},
			},
		}).Then().Expect(ApplicationsExist([]v1alpha1.Application{expectedAppCluster1})).

		// Update the ApplicationSet template namespace, and verify it updates the Applications
		When().
		CreateClusterSecret("my-secret2", "cluster2", "https://kubernetes.default.svc").
		Then().Expect(ApplicationsExist([]v1alpha1.Application{expectedAppCluster1, expectedAppCluster2})).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist([]v1alpha1.Application{expectedAppCluster1, expectedAppCluster2}))
}

func TestSimpleClusterGeneratorDeletingCluster(t *testing.T) {
	expectedAppTemplate := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "{{name}}-guestbook",
			Namespace:  fixture.TestNamespace(),
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
				Name:      "{{name}}",
				Namespace: "guestbook",
			},
		},
	}

	expectedAppCluster1 := *expectedAppTemplate.DeepCopy()
	expectedAppCluster1.Spec.Destination.Name = "cluster1"
	expectedAppCluster1.Name = "cluster1-guestbook"

	expectedAppCluster2 := *expectedAppTemplate.DeepCopy()
	expectedAppCluster2.Spec.Destination.Name = "cluster2"
	expectedAppCluster2.Name = "cluster2-guestbook"

	Given(t).
		// Create a ClusterGenerator-based ApplicationSet
		When().
		CreateClusterSecret("my-secret", "cluster1", "https://kubernetes.default.svc").
		CreateClusterSecret("my-secret2", "cluster2", "https://kubernetes.default.svc").
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-cluster-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{name}}-guestbook"},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: v1alpha1.ApplicationDestination{
							Name: "{{name}}",
							// Server:    "{{server}}",
							Namespace: "guestbook",
						},
					},
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						Clusters: &v1alpha1.ClusterGenerator{
							Selector: metav1.LabelSelector{
								MatchLabels: map[string]string{
									"argocd.argoproj.io/secret-type": "cluster",
								},
							},
						},
					},
				},
			},
		}).Then().Expect(ApplicationsExist([]v1alpha1.Application{expectedAppCluster1, expectedAppCluster2})).

		// Update the ApplicationSet template namespace, and verify it updates the Applications
		When().
		DeleteClusterSecret("my-secret2").
		Then().Expect(ApplicationsExist([]v1alpha1.Application{expectedAppCluster1})).
		Expect(ApplicationsDoNotExist([]v1alpha1.Application{expectedAppCluster2})).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist([]v1alpha1.Application{expectedAppCluster1}))
}

func TestClusterGeneratorWithFlatListMode(t *testing.T) {
	expectedAppTemplate := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "flat-clusters",
			Namespace:  fixture.TestNamespace(),
			Finalizers: []string{v1alpha1.ResourcesFinalizerName},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "helm-guestbook",
			},
			Destination: v1alpha1.ApplicationDestination{
				Name:      "cluster1",
				Namespace: fixture.TestNamespace(),
			},
			SyncPolicy: &v1alpha1.SyncPolicy{
				Automated: &v1alpha1.SyncPolicyAutomated{},
			},
		},
	}

	expectedAppCluster1 := *expectedAppTemplate.DeepCopy()
	expectedAppCluster1.Spec.Source.Helm = &v1alpha1.ApplicationSourceHelm{
		Values: `clusters:
  - name: cluster1
`,
	}

	expectedAppCluster2 := *expectedAppTemplate.DeepCopy()
	expectedAppCluster2.Spec.Source.Helm = &v1alpha1.ApplicationSourceHelm{
		Values: `clusters:
  - name: cluster1
  - name: cluster2
`,
	}

	Given(t).
		// Create a ClusterGenerator-based ApplicationSet
		When().
		CreateClusterSecret("my-secret", "cluster1", "https://kubernetes.default.svc").
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-cluster-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				GoTemplate: true,
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "flat-clusters"},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "helm-guestbook",
							Helm: &v1alpha1.ApplicationSourceHelm{
								Values: `clusters:
{{- range .clusters }}
  - name: {{ .name }}
{{- end }}
`,
							},
						},
						Destination: v1alpha1.ApplicationDestination{
							Name:      "cluster1",
							Namespace: fixture.TestNamespace(),
						},
						SyncPolicy: &v1alpha1.SyncPolicy{
							Automated: &v1alpha1.SyncPolicyAutomated{},
						},
					},
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						Clusters: &v1alpha1.ClusterGenerator{
							Selector: metav1.LabelSelector{
								MatchLabels: map[string]string{
									"argocd.argoproj.io/secret-type": "cluster",
								},
							},
							FlatList: true,
						},
					},
				},
			},
		}).Then().Expect(ApplicationsExist([]v1alpha1.Application{expectedAppCluster1})).

		// Update the ApplicationSet template namespace, and verify it updates the Applications
		When().
		CreateClusterSecret("my-secret2", "cluster2", "https://kubernetes.default.svc").
		Then().
		Expect(ApplicationsExist([]v1alpha1.Application{expectedAppCluster2})).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist([]v1alpha1.Application{expectedAppCluster2}))
}
