package e2e

import (
	"encoding/json"
	"fmt"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/applicationsets"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture/applicationsets/utils"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application"
)

func TestListMergeGenerator(t *testing.T) {
	generateExpectedApp := func(name, nameSuffix string) argov1alpha1.Application {
		return argov1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       fmt.Sprintf("%s-%s", name, nameSuffix),
				Namespace:  utils.TestNamespace(),
				Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			},
			Spec: argov1alpha1.ApplicationSpec{
				Project: "default",
				Source: &argov1alpha1.ApplicationSource{
					RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
					TargetRevision: "HEAD",
					Path:           name,
				},
				Destination: argov1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: name,
				},
			},
		}
	}

	expectedApps := []argov1alpha1.Application{
		generateExpectedApp("kustomize-guestbook", "1"),
		generateExpectedApp("helm-guestbook", "2"),
	}

	var expectedAppsNewNamespace []argov1alpha1.Application
	var expectedAppsNewMetadata []argov1alpha1.Application

	Given(t).
		// Create a ClusterGenerator-based ApplicationSet
		When().
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "merge-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{path.basename}}-{{name-suffix}}"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: &argov1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "{{path}}",
						},
						Destination: argov1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "{{path.basename}}",
						},
					},
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						Merge: &v1alpha1.MergeGenerator{
							MergeKeys: []string{"path.basename"},
							Generators: []v1alpha1.ApplicationSetNestedGenerator{
								{
									Git: &v1alpha1.GitGenerator{
										RepoURL: "https://github.com/argoproj/argocd-example-apps.git",
										Directories: []v1alpha1.GitDirectoryGeneratorItem{
											{
												Path: "*guestbook*",
											},
										},
									},
								},
								{
									List: &v1alpha1.ListGenerator{
										Elements: []apiextensionsv1.JSON{
											{Raw: []byte(`{"path.basename": "kustomize-guestbook", "name-suffix": "1"}`)},
											{Raw: []byte(`{"path.basename": "helm-guestbook", "name-suffix": "2"}`)},
										},
									},
								},
							},
						},
					},
				},
			},
		}).Then().Expect(ApplicationsExist(expectedApps)).

		// Update the ApplicationSet template namespace, and verify it updates the Applications
		When().
		And(func() {
			for _, expectedApp := range expectedApps {
				newExpectedApp := expectedApp.DeepCopy()
				newExpectedApp.Spec.Destination.Namespace = "guestbook2"
				expectedAppsNewNamespace = append(expectedAppsNewNamespace, *newExpectedApp)
			}
		}).
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Template.Spec.Destination.Namespace = "guestbook2"
		}).Then().Expect(ApplicationsExist(expectedAppsNewNamespace)).

		// Update the metadata fields in the appset template, and make sure it propagates to the apps
		When().
		And(func() {
			for _, expectedApp := range expectedAppsNewNamespace {
				expectedAppNewMetadata := expectedApp.DeepCopy()
				expectedAppNewMetadata.ObjectMeta.Annotations = map[string]string{"annotation-key": "annotation-value"}
				expectedAppNewMetadata.ObjectMeta.Labels = map[string]string{"label-key": "label-value"}
				expectedAppsNewMetadata = append(expectedAppsNewMetadata, *expectedAppNewMetadata)
			}
		}).
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Template.Annotations = map[string]string{"annotation-key": "annotation-value"}
			appset.Spec.Template.Labels = map[string]string{"label-key": "label-value"}
		}).Then().Expect(ApplicationsExist(expectedAppsNewMetadata)).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist(expectedAppsNewNamespace))
}

func TestClusterMergeGenerator(t *testing.T) {
	generateExpectedApp := func(cluster, name, nameSuffix string) argov1alpha1.Application {
		return argov1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       fmt.Sprintf("%s-%s-%s", cluster, name, nameSuffix),
				Namespace:  utils.TestNamespace(),
				Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			},
			Spec: argov1alpha1.ApplicationSpec{
				Project: "default",
				Source: &argov1alpha1.ApplicationSource{
					RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
					TargetRevision: "HEAD",
					Path:           name,
				},
				Destination: argov1alpha1.ApplicationDestination{
					Name:      cluster,
					Namespace: name,
				},
			},
		}
	}

	expectedApps := []argov1alpha1.Application{
		generateExpectedApp("cluster1", "kustomize-guestbook", "1"),
		generateExpectedApp("cluster1", "helm-guestbook", "0"),
		generateExpectedApp("cluster1", "ksonnet-guestbook", "0"),

		generateExpectedApp("cluster2", "kustomize-guestbook", "0"),
		generateExpectedApp("cluster2", "helm-guestbook", "2"),
		generateExpectedApp("cluster2", "ksonnet-guestbook", "0"),
	}

	var expectedAppsNewNamespace []argov1alpha1.Application
	var expectedAppsNewMetadata []argov1alpha1.Application

	Given(t).
		// Create a ClusterGenerator-based ApplicationSet
		When().
		CreateClusterSecret("my-secret", "cluster1", "https://kubernetes.default.svc").
		CreateClusterSecret("my-secret2", "cluster2", "https://kubernetes.default.svc").
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "merge-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{name}}-{{path.basename}}-{{values.name-suffix}}"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: &argov1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "{{path}}",
						},
						Destination: argov1alpha1.ApplicationDestination{
							Name:      "{{name}}",
							Namespace: "{{path.basename}}",
						},
					},
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						Merge: &v1alpha1.MergeGenerator{
							MergeKeys: []string{"name", "path.basename"},
							Generators: []v1alpha1.ApplicationSetNestedGenerator{
								{
									Matrix: toAPIExtensionsJSON(t, &v1alpha1.NestedMatrixGenerator{
										Generators: []v1alpha1.ApplicationSetTerminalGenerator{
											{
												Clusters: &v1alpha1.ClusterGenerator{
													Selector: metav1.LabelSelector{
														MatchLabels: map[string]string{
															"argocd.argoproj.io/secret-type": "cluster",
														},
													},
													Values: map[string]string{
														"name-suffix": "0",
													},
												},
											},
											{
												Git: &v1alpha1.GitGenerator{
													RepoURL: "https://github.com/argoproj/argocd-example-apps.git",
													Directories: []v1alpha1.GitDirectoryGeneratorItem{
														{
															Path: "*guestbook*",
														},
													},
												},
											},
										},
									}),
								},
								{
									List: &v1alpha1.ListGenerator{
										Elements: []apiextensionsv1.JSON{
											{Raw: []byte(`{"name": "cluster1", "path.basename": "kustomize-guestbook", "values": {"name-suffix": "1"}}`)},
											{Raw: []byte(`{"name": "cluster2", "path.basename": "helm-guestbook", "values": {"name-suffix": "2"}}`)},
										},
									},
								},
							},
						},
					},
				},
			},
		}).Then().Expect(ApplicationsExist(expectedApps)).

		// Update the ApplicationSet template namespace, and verify it updates the Applications
		When().
		And(func() {
			for _, expectedApp := range expectedApps {
				newExpectedApp := expectedApp.DeepCopy()
				newExpectedApp.Spec.Destination.Namespace = "guestbook2"
				expectedAppsNewNamespace = append(expectedAppsNewNamespace, *newExpectedApp)
			}
		}).
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Template.Spec.Destination.Namespace = "guestbook2"
		}).Then().Expect(ApplicationsExist(expectedAppsNewNamespace)).

		// Update the metadata fields in the appset template, and make sure it propagates to the apps
		When().
		And(func() {
			for _, expectedApp := range expectedAppsNewNamespace {
				expectedAppNewMetadata := expectedApp.DeepCopy()
				expectedAppNewMetadata.ObjectMeta.Annotations = map[string]string{"annotation-key": "annotation-value"}
				expectedAppNewMetadata.ObjectMeta.Labels = map[string]string{"label-key": "label-value"}
				expectedAppsNewMetadata = append(expectedAppsNewMetadata, *expectedAppNewMetadata)
			}
		}).
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Template.Annotations = map[string]string{"annotation-key": "annotation-value"}
			appset.Spec.Template.Labels = map[string]string{"label-key": "label-value"}
		}).Then().Expect(ApplicationsExist(expectedAppsNewMetadata)).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist(expectedAppsNewNamespace))
}

func TestMergeTerminalMergeGeneratorSelector(t *testing.T) {
	generateExpectedApp := func(name, nameSuffix string) argov1alpha1.Application {
		return argov1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       fmt.Sprintf("%s-%s", name, nameSuffix),
				Namespace:  utils.TestNamespace(),
				Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			},
			Spec: argov1alpha1.ApplicationSpec{
				Project: "default",
				Source: &argov1alpha1.ApplicationSource{
					RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
					TargetRevision: "HEAD",
					Path:           name,
				},
				Destination: argov1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: name,
				},
			},
		}
	}

	expectedApps1 := []argov1alpha1.Application{
		generateExpectedApp("kustomize-guestbook", "1"),
	}
	expectedApps2 := []argov1alpha1.Application{
		generateExpectedApp("helm-guestbook", "2"),
	}

	Given(t).
		// Create ApplicationSet with LabelSelector on an ApplicationSetTerminalGenerator
		When().
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "merge-generator-nested-merge",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				ApplyNestedSelectors: true,
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{path.basename}}-{{name-suffix}}"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: &argov1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "{{path}}",
						},
						Destination: argov1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "{{path.basename}}",
						},
					},
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						Merge: &v1alpha1.MergeGenerator{
							MergeKeys: []string{"path.basename"},
							Generators: []v1alpha1.ApplicationSetNestedGenerator{
								{
									Merge: toAPIExtensionsJSON(t, &v1alpha1.NestedMergeGenerator{
										MergeKeys: []string{"path.basename"},
										Generators: []v1alpha1.ApplicationSetTerminalGenerator{
											{
												Git: &v1alpha1.GitGenerator{
													RepoURL: "https://github.com/argoproj/argocd-example-apps.git",
													Directories: []v1alpha1.GitDirectoryGeneratorItem{
														{
															Path: "*guestbook*",
														},
													},
												},
												Selector: &metav1.LabelSelector{
													MatchLabels: map[string]string{
														"path.basename": "kustomize-guestbook",
													},
												},
											},
											{
												List: &v1alpha1.ListGenerator{
													Elements: []apiextensionsv1.JSON{
														{Raw: []byte(`{"path.basename": "kustomize-guestbook", "name-suffix": "1"}`)},
														{Raw: []byte(`{"path.basename": "helm-guestbook", "name-suffix": "2"}`)},
													},
												},
											},
										},
									}),
								},
								{
									List: &v1alpha1.ListGenerator{
										Elements: []apiextensionsv1.JSON{
											{Raw: []byte(`{}`)},
										},
									},
								},
							},
						},
					},
				},
			},
		}).Then().Expect(ApplicationsExist(expectedApps1)).Expect(ApplicationsDoNotExist(expectedApps2)).

		// Update the ApplicationSetTerminalGenerator LabelSelector, and verify the Applications are deleted and created
		When().
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Generators[0].Merge.Generators[0].Merge = toAPIExtensionsJSON(t, &v1alpha1.NestedMergeGenerator{
				MergeKeys: []string{"path.basename"},
				Generators: []v1alpha1.ApplicationSetTerminalGenerator{
					{
						Git: &v1alpha1.GitGenerator{
							RepoURL: "https://github.com/argoproj/argocd-example-apps.git",
							Directories: []v1alpha1.GitDirectoryGeneratorItem{
								{
									Path: "*guestbook*",
								},
							},
						},
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"path.basename": "helm-guestbook",
							},
						},
					},
					{
						List: &v1alpha1.ListGenerator{
							Elements: []apiextensionsv1.JSON{
								{Raw: []byte(`{"path.basename": "kustomize-guestbook", "name-suffix": "1"}`)},
								{Raw: []byte(`{"path.basename": "helm-guestbook", "name-suffix": "2"}`)},
							},
						},
					},
				},
			})
		}).Then().Expect(ApplicationsExist(expectedApps2)).Expect(ApplicationsDoNotExist(expectedApps1)).

		// Set ApplyNestedSelector to false and verify all Applications are created
		When().
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.ApplyNestedSelectors = false
		}).Then().Expect(ApplicationsExist(expectedApps1)).Expect(ApplicationsExist(expectedApps2)).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist(expectedApps1)).Expect(ApplicationsDoNotExist(expectedApps2))
}

func toAPIExtensionsJSON(t *testing.T, g interface{}) *apiextensionsv1.JSON {
	resVal, err := json.Marshal(g)
	if err != nil {
		t.Error("unable to unmarshal json", g)
		return nil
	}

	res := &apiextensionsv1.JSON{Raw: resVal}

	return res
}
