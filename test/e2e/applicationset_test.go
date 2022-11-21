package e2e

import (
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/applicationsets"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture/applicationsets/utils"
	. "github.com/argoproj/argo-cd/v2/util/errors"
)

var (
	ExpectedConditions = []v1alpha1.ApplicationSetCondition{
		{
			Type:    v1alpha1.ApplicationSetConditionErrorOccurred,
			Status:  v1alpha1.ApplicationSetConditionStatusFalse,
			Message: "Successfully generated parameters for all Applications",
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
			Message: "ApplicationSet up to date",
			Reason:  v1alpha1.ApplicationSetReasonApplicationSetUpToDate,
		},
	}
)

func TestSimpleListGenerator(t *testing.T) {

	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "my-cluster-guestbook",
			Namespace:  utils.ArgoCDNamespace,
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: argov1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: argov1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
		},
	}
	var expectedAppNewNamespace *argov1alpha1.Application
	var expectedAppNewMetadata *argov1alpha1.Application

	Given(t).
		// Create a ListGenerator-based ApplicationSet
		When().Create(v1alpha1.ApplicationSet{ObjectMeta: metav1.ObjectMeta{
		Name: "simple-list-generator",
	},
		Spec: v1alpha1.ApplicationSetSpec{
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{cluster}}-guestbook"},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: argov1alpha1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						TargetRevision: "HEAD",
						Path:           "guestbook",
					},
					Destination: argov1alpha1.ApplicationDestination{
						Server:    "{{url}}",
						Namespace: "guestbook",
					},
				},
			},
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					List: &v1alpha1.ListGenerator{
						Elements: []apiextensionsv1.JSON{{
							Raw: []byte(`{"cluster": "my-cluster","url": "https://kubernetes.default.svc"}`),
						}},
					},
				},
			},
		},
	}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{expectedApp})).

		// Update the ApplicationSet template namespace, and verify it updates the Applications
		When().
		And(func() {
			expectedAppNewNamespace = expectedApp.DeepCopy()
			expectedAppNewNamespace.Spec.Destination.Namespace = "guestbook2"
		}).
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Template.Spec.Destination.Namespace = "guestbook2"
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{*expectedAppNewNamespace})).

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
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{*expectedAppNewMetadata})).

		// verify the ApplicationSet status conditions were set correctly
		Expect(ApplicationSetHasConditions("simple-list-generator", ExpectedConditions)).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist([]argov1alpha1.Application{*expectedAppNewMetadata}))

}

func TestSimpleListGeneratorGoTemplate(t *testing.T) {

	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "my-cluster-guestbook",
			Namespace:  utils.ArgoCDNamespace,
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: argov1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: argov1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
		},
	}
	var expectedAppNewNamespace *argov1alpha1.Application
	var expectedAppNewMetadata *argov1alpha1.Application

	Given(t).
		// Create a ListGenerator-based ApplicationSet
		When().Create(v1alpha1.ApplicationSet{ObjectMeta: metav1.ObjectMeta{
		Name: "simple-list-generator",
	},
		Spec: v1alpha1.ApplicationSetSpec{
			GoTemplate: true,
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{.cluster}}-guestbook"},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: argov1alpha1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						TargetRevision: "HEAD",
						Path:           "guestbook",
					},
					Destination: argov1alpha1.ApplicationDestination{
						Server:    "{{.url}}",
						Namespace: "guestbook",
					},
				},
			},
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					List: &v1alpha1.ListGenerator{
						Elements: []apiextensionsv1.JSON{{
							Raw: []byte(`{"cluster": "my-cluster","url": "https://kubernetes.default.svc"}`),
						}},
					},
				},
			},
		},
	}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{expectedApp})).

		// Update the ApplicationSet template namespace, and verify it updates the Applications
		When().
		And(func() {
			expectedAppNewNamespace = expectedApp.DeepCopy()
			expectedAppNewNamespace.Spec.Destination.Namespace = "guestbook2"
		}).
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Template.Spec.Destination.Namespace = "guestbook2"
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{*expectedAppNewNamespace})).

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
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{*expectedAppNewMetadata})).

		// verify the ApplicationSet status conditions were set correctly
		Expect(ApplicationSetHasConditions("simple-list-generator", ExpectedConditions)).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist([]argov1alpha1.Application{*expectedAppNewMetadata}))

}

func TestSimpleGitDirectoryGenerator(t *testing.T) {
	generateExpectedApp := func(name string) argov1alpha1.Application {
		return argov1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Application",
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  utils.ArgoCDNamespace,
				Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			},
			Spec: argov1alpha1.ApplicationSpec{
				Project: "default",
				Source: argov1alpha1.ApplicationSource{
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
		generateExpectedApp("kustomize-guestbook"),
		generateExpectedApp("helm-guestbook"),
		generateExpectedApp("ksonnet-guestbook"),
	}

	var expectedAppsNewNamespace []argov1alpha1.Application
	var expectedAppsNewMetadata []argov1alpha1.Application

	Given(t).
		When().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{ObjectMeta: metav1.ObjectMeta{
			Name: "simple-git-generator",
		},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{path.basename}}"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: argov1alpha1.ApplicationSource{
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

		// verify the ApplicationSet status conditions were set correctly
		Expect(ApplicationSetHasConditions("simple-git-generator", ExpectedConditions)).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist(expectedAppsNewNamespace))
}

func TestSimpleGitDirectoryGeneratorGoTemplate(t *testing.T) {
	generateExpectedApp := func(name string) argov1alpha1.Application {
		return argov1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Application",
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  utils.ArgoCDNamespace,
				Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			},
			Spec: argov1alpha1.ApplicationSpec{
				Project: "default",
				Source: argov1alpha1.ApplicationSource{
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
		generateExpectedApp("kustomize-guestbook"),
		generateExpectedApp("helm-guestbook"),
		generateExpectedApp("ksonnet-guestbook"),
	}

	var expectedAppsNewNamespace []argov1alpha1.Application
	var expectedAppsNewMetadata []argov1alpha1.Application

	Given(t).
		When().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{ObjectMeta: metav1.ObjectMeta{
			Name: "simple-git-generator",
		},
			Spec: v1alpha1.ApplicationSetSpec{
				GoTemplate: true,
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{.path.basename}}"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: argov1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "{{.path.path}}",
						},
						Destination: argov1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "{{.path.basename}}",
						},
					},
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
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

		// verify the ApplicationSet status conditions were set correctly
		Expect(ApplicationSetHasConditions("simple-git-generator", ExpectedConditions)).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist(expectedAppsNewNamespace))
}

func TestSimpleGitFilesGenerator(t *testing.T) {

	generateExpectedApp := func(name string) argov1alpha1.Application {
		return argov1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Application",
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  utils.ArgoCDNamespace,
				Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			},
			Spec: argov1alpha1.ApplicationSpec{
				Project: "default",
				Source: argov1alpha1.ApplicationSource{
					RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
					TargetRevision: "HEAD",
					Path:           "guestbook",
				},
				Destination: argov1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: "guestbook",
				},
			},
		}
	}

	expectedApps := []argov1alpha1.Application{
		generateExpectedApp("engineering-dev-guestbook"),
		generateExpectedApp("engineering-prod-guestbook"),
	}

	var expectedAppsNewNamespace []argov1alpha1.Application
	var expectedAppsNewMetadata []argov1alpha1.Application

	Given(t).
		When().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{ObjectMeta: metav1.ObjectMeta{
			Name: "simple-git-generator",
		},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{cluster.name}}-guestbook"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: argov1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: argov1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "guestbook",
						},
					},
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						Git: &v1alpha1.GitGenerator{
							RepoURL: "https://github.com/argoproj/applicationset.git",
							Files: []v1alpha1.GitFileGeneratorItem{
								{
									Path: "examples/git-generator-files-discovery/cluster-config/**/config.json",
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

		// verify the ApplicationSet status conditions were set correctly
		Expect(ApplicationSetHasConditions("simple-git-generator", ExpectedConditions)).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist(expectedAppsNewNamespace))
}

func TestSimpleGitFilesGeneratorGoTemplate(t *testing.T) {

	generateExpectedApp := func(name string) argov1alpha1.Application {
		return argov1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Application",
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  utils.ArgoCDNamespace,
				Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			},
			Spec: argov1alpha1.ApplicationSpec{
				Project: "default",
				Source: argov1alpha1.ApplicationSource{
					RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
					TargetRevision: "HEAD",
					Path:           "guestbook",
				},
				Destination: argov1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: "guestbook",
				},
			},
		}
	}

	expectedApps := []argov1alpha1.Application{
		generateExpectedApp("engineering-dev-guestbook"),
		generateExpectedApp("engineering-prod-guestbook"),
	}

	var expectedAppsNewNamespace []argov1alpha1.Application
	var expectedAppsNewMetadata []argov1alpha1.Application

	Given(t).
		When().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{ObjectMeta: metav1.ObjectMeta{
			Name: "simple-git-generator",
		},
			Spec: v1alpha1.ApplicationSetSpec{
				GoTemplate: true,
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{.cluster.name}}-guestbook"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: argov1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: argov1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "guestbook",
						},
					},
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						Git: &v1alpha1.GitGenerator{
							RepoURL: "https://github.com/argoproj/applicationset.git",
							Files: []v1alpha1.GitFileGeneratorItem{
								{
									Path: "examples/git-generator-files-discovery/cluster-config/**/config.json",
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

		// verify the ApplicationSet status conditions were set correctly
		Expect(ApplicationSetHasConditions("simple-git-generator", ExpectedConditions)).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist(expectedAppsNewNamespace))
}

func TestSimpleGitFilesPreserveResourcesOnDeletion(t *testing.T) {

	Given(t).
		When().
		CreateNamespace().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{ObjectMeta: metav1.ObjectMeta{
			Name: "simple-git-generator",
		},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{cluster.name}}-guestbook"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: argov1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: argov1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: utils.ApplicationSetNamespace,
						},

						// Automatically create resources
						SyncPolicy: &argov1alpha1.SyncPolicy{
							Automated: &argov1alpha1.SyncPolicyAutomated{},
						},
					},
				},
				SyncPolicy: &v1alpha1.ApplicationSetSyncPolicy{
					PreserveResourcesOnDeletion: true,
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						Git: &v1alpha1.GitGenerator{
							RepoURL: "https://github.com/argoproj/applicationset.git",
							Files: []v1alpha1.GitFileGeneratorItem{
								{
									Path: "examples/git-generator-files-discovery/cluster-config/**/config.json",
								},
							},
						},
					},
				},
			},
			// We use an extra-long duration here, as we might need to wait for image pull.
		}).Then().ExpectWithDuration(Pod(func(p corev1.Pod) bool { return strings.Contains(p.Name, "guestbook-ui") }), 6*time.Minute).
		When().
		Delete().
		And(func() {
			t.Log("Waiting 30 seconds to give the cluster a chance to delete the pods.")
			// Wait 30 seconds to give the cluster a chance to deletes the pods, if it is going to do so.
			// It should NOT delete the pods; to do so would be an ApplicationSet bug, and
			// that is what we are testing here.
			time.Sleep(30 * time.Second)
			// The pod should continue to exist after 30 seconds.
		}).Then().Expect(Pod(func(p corev1.Pod) bool { return strings.Contains(p.Name, "guestbook-ui") }))
}

func TestSimpleGitFilesPreserveResourcesOnDeletionGoTemplate(t *testing.T) {

	Given(t).
		When().
		CreateNamespace().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{ObjectMeta: metav1.ObjectMeta{
			Name: "simple-git-generator",
		},
			Spec: v1alpha1.ApplicationSetSpec{
				GoTemplate: true,
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{.cluster.name}}-guestbook"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: argov1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: argov1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: utils.ApplicationSetNamespace,
						},

						// Automatically create resources
						SyncPolicy: &argov1alpha1.SyncPolicy{
							Automated: &argov1alpha1.SyncPolicyAutomated{},
						},
					},
				},
				SyncPolicy: &v1alpha1.ApplicationSetSyncPolicy{
					PreserveResourcesOnDeletion: true,
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						Git: &v1alpha1.GitGenerator{
							RepoURL: "https://github.com/argoproj/applicationset.git",
							Files: []v1alpha1.GitFileGeneratorItem{
								{
									Path: "examples/git-generator-files-discovery/cluster-config/**/config.json",
								},
							},
						},
					},
				},
			},
			// We use an extra-long duration here, as we might need to wait for image pull.
		}).Then().ExpectWithDuration(Pod(func(p corev1.Pod) bool { return strings.Contains(p.Name, "guestbook-ui") }), 6*time.Minute).
		When().
		Delete().
		And(func() {
			t.Log("Waiting 30 seconds to give the cluster a chance to delete the pods.")
			// Wait 30 seconds to give the cluster a chance to deletes the pods, if it is going to do so.
			// It should NOT delete the pods; to do so would be an ApplicationSet bug, and
			// that is what we are testing here.
			time.Sleep(30 * time.Second)
			// The pod should continue to exist after 30 seconds.
		}).Then().Expect(Pod(func(p corev1.Pod) bool { return strings.Contains(p.Name, "guestbook-ui") }))
}

func TestSimpleSCMProviderGenerator(t *testing.T) {
	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "argocd-example-apps-guestbook",
			Namespace:  utils.ArgoCDNamespace,
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: argov1alpha1.ApplicationSource{
				RepoURL:        "git@github.com:argoproj/argocd-example-apps.git",
				TargetRevision: "master",
				Path:           "guestbook",
			},
			Destination: argov1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
		},
	}

	// Because you can't &"".
	repoMatch := "example-apps"

	Given(t).
		// Create an SCMProviderGenerator-based ApplicationSet
		When().Create(v1alpha1.ApplicationSet{ObjectMeta: metav1.ObjectMeta{
		Name: "simple-scm-provider-generator",
	},
		Spec: v1alpha1.ApplicationSetSpec{
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{ repository }}-guestbook"},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: argov1alpha1.ApplicationSource{
						RepoURL:        "{{ url }}",
						TargetRevision: "{{ branch }}",
						Path:           "guestbook",
					},
					Destination: argov1alpha1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "guestbook",
					},
				},
			},
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					SCMProvider: &v1alpha1.SCMProviderGenerator{
						Github: &v1alpha1.SCMProviderGeneratorGithub{
							Organization: "argoproj",
						},
						Filters: []v1alpha1.SCMProviderGeneratorFilter{
							{
								RepositoryMatch: &repoMatch,
							},
						},
					},
				},
			},
		},
	}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{expectedApp}))
}

func TestSimpleSCMProviderGeneratorGoTemplate(t *testing.T) {
	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "argocd-example-apps-guestbook",
			Namespace:  utils.ArgoCDNamespace,
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: argov1alpha1.ApplicationSource{
				RepoURL:        "git@github.com:argoproj/argocd-example-apps.git",
				TargetRevision: "master",
				Path:           "guestbook",
			},
			Destination: argov1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
		},
	}

	// Because you can't &"".
	repoMatch := "example-apps"

	Given(t).
		// Create an SCMProviderGenerator-based ApplicationSet
		When().Create(v1alpha1.ApplicationSet{ObjectMeta: metav1.ObjectMeta{
		Name: "simple-scm-provider-generator",
	},
		Spec: v1alpha1.ApplicationSetSpec{
			GoTemplate: true,
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{ .repository }}-guestbook"},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: argov1alpha1.ApplicationSource{
						RepoURL:        "{{ .url }}",
						TargetRevision: "{{ .branch }}",
						Path:           "guestbook",
					},
					Destination: argov1alpha1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "guestbook",
					},
				},
			},
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					SCMProvider: &v1alpha1.SCMProviderGenerator{
						Github: &v1alpha1.SCMProviderGeneratorGithub{
							Organization: "argoproj",
						},
						Filters: []v1alpha1.SCMProviderGeneratorFilter{
							{
								RepositoryMatch: &repoMatch,
							},
						},
					},
				},
			},
		},
	}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{expectedApp}))
}

func TestCustomApplicationFinalizers(t *testing.T) {
	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "my-cluster-guestbook",
			Namespace:  utils.ArgoCDNamespace,
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io/background"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: argov1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: argov1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
		},
	}

	Given(t).
		// Create a ListGenerator-based ApplicationSet
		When().Create(v1alpha1.ApplicationSet{ObjectMeta: metav1.ObjectMeta{
		Name: "simple-list-generator",
	},
		Spec: v1alpha1.ApplicationSetSpec{
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
					Name:       "{{cluster}}-guestbook",
					Finalizers: []string{"resources-finalizer.argocd.argoproj.io/background"},
				},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: argov1alpha1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						TargetRevision: "HEAD",
						Path:           "guestbook",
					},
					Destination: argov1alpha1.ApplicationDestination{
						Server:    "{{url}}",
						Namespace: "guestbook",
					},
				},
			},
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					List: &v1alpha1.ListGenerator{
						Elements: []apiextensionsv1.JSON{{
							Raw: []byte(`{"cluster": "my-cluster","url": "https://kubernetes.default.svc"}`),
						}},
					},
				},
			},
		},
	}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{expectedApp})).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist([]argov1alpha1.Application{expectedApp}))
}

func TestCustomApplicationFinalizersGoTemplate(t *testing.T) {
	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "my-cluster-guestbook",
			Namespace:  utils.ArgoCDNamespace,
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io/background"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: argov1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: argov1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
		},
	}

	Given(t).
		// Create a ListGenerator-based ApplicationSet
		When().Create(v1alpha1.ApplicationSet{ObjectMeta: metav1.ObjectMeta{
		Name: "simple-list-generator",
	},
		Spec: v1alpha1.ApplicationSetSpec{
			GoTemplate: true,
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
					Name:       "{{.cluster}}-guestbook",
					Finalizers: []string{"resources-finalizer.argocd.argoproj.io/background"},
				},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: argov1alpha1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						TargetRevision: "HEAD",
						Path:           "guestbook",
					},
					Destination: argov1alpha1.ApplicationDestination{
						Server:    "{{.url}}",
						Namespace: "guestbook",
					},
				},
			},
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					List: &v1alpha1.ListGenerator{
						Elements: []apiextensionsv1.JSON{{
							Raw: []byte(`{"cluster": "my-cluster","url": "https://kubernetes.default.svc"}`),
						}},
					},
				},
			},
		},
	}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{expectedApp})).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist([]argov1alpha1.Application{expectedApp}))
}

func TestSimplePullRequestGenerator(t *testing.T) {

	if utils.IsGitHubAPISkippedTest(t) {
		return
	}

	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "guestbook-1",
			Namespace:  utils.ArgoCDNamespace,
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: argov1alpha1.ApplicationSource{
				RepoURL:        "git@github.com:applicationset-test-org/argocd-example-apps.git",
				TargetRevision: "824a5c987fdfb2b0629e9dbf5f31636c69ba4772",
				Path:           "kustomize-guestbook",
				Kustomize: &argov1alpha1.ApplicationSourceKustomize{
					NamePrefix: "guestbook-1",
				},
			},
			Destination: argov1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook-pull-request",
			},
		},
	}

	Given(t).
		// Create an PullRequestGenerator-based ApplicationSet
		When().Create(v1alpha1.ApplicationSet{ObjectMeta: metav1.ObjectMeta{
		Name: "simple-pull-request-generator",
	},
		Spec: v1alpha1.ApplicationSetSpec{
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "guestbook-{{ number }}"},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: argov1alpha1.ApplicationSource{
						RepoURL:        "git@github.com:applicationset-test-org/argocd-example-apps.git",
						TargetRevision: "{{ head_sha }}",
						Path:           "kustomize-guestbook",
						Kustomize: &argov1alpha1.ApplicationSourceKustomize{
							NamePrefix: "guestbook-{{ number }}",
						},
					},
					Destination: argov1alpha1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "guestbook-{{ branch }}",
					},
				},
			},
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					PullRequest: &v1alpha1.PullRequestGenerator{
						Github: &v1alpha1.PullRequestGeneratorGithub{
							Owner: "applicationset-test-org",
							Repo:  "argocd-example-apps",
							Labels: []string{
								"preview",
							},
						},
					},
				},
			},
		},
	}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{expectedApp}))
}

func TestSimplePullRequestGeneratorGoTemplate(t *testing.T) {

	if utils.IsGitHubAPISkippedTest(t) {
		return
	}

	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "guestbook-1",
			Namespace:  utils.ArgoCDNamespace,
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: argov1alpha1.ApplicationSource{
				RepoURL:        "git@github.com:applicationset-test-org/argocd-example-apps.git",
				TargetRevision: "824a5c987fdfb2b0629e9dbf5f31636c69ba4772",
				Path:           "kustomize-guestbook",
				Kustomize: &argov1alpha1.ApplicationSourceKustomize{
					NamePrefix: "guestbook-1",
				},
			},
			Destination: argov1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook-pull-request",
			},
		},
	}

	Given(t).
		// Create an PullRequestGenerator-based ApplicationSet
		When().Create(v1alpha1.ApplicationSet{ObjectMeta: metav1.ObjectMeta{
		Name: "simple-pull-request-generator",
	},
		Spec: v1alpha1.ApplicationSetSpec{
			GoTemplate: true,
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "guestbook-{{ .number }}"},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: argov1alpha1.ApplicationSource{
						RepoURL:        "git@github.com:applicationset-test-org/argocd-example-apps.git",
						TargetRevision: "{{ .head_sha }}",
						Path:           "kustomize-guestbook",
						Kustomize: &argov1alpha1.ApplicationSourceKustomize{
							NamePrefix: "guestbook-{{ .number }}",
						},
					},
					Destination: argov1alpha1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "guestbook-{{ .branch }}",
					},
				},
			},
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					PullRequest: &v1alpha1.PullRequestGenerator{
						Github: &v1alpha1.PullRequestGeneratorGithub{
							Owner: "applicationset-test-org",
							Repo:  "argocd-example-apps",
							Labels: []string{
								"preview",
							},
						},
					},
				},
			},
		},
	}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{expectedApp}))
}

func TestGitGeneratorPrivateRepo(t *testing.T) {
	FailOnErr(fixture.RunCli("repo", "add", fixture.RepoURL(fixture.RepoURLTypeHTTPS), "--username", fixture.GitUsername, "--password", fixture.GitPassword, "--insecure-skip-server-verification"))
	generateExpectedApp := func(name string) argov1alpha1.Application {
		return argov1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Application",
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  utils.ArgoCDNamespace,
				Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			},
			Spec: argov1alpha1.ApplicationSpec{
				Project: "default",
				Source: argov1alpha1.ApplicationSource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHTTPS),
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
		generateExpectedApp("https-kustomize-base"),
	}

	var expectedAppsNewNamespace []argov1alpha1.Application

	Given(t).
		When().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{ObjectMeta: metav1.ObjectMeta{
			Name: "simple-git-generator-private",
		},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{path.basename}}"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: argov1alpha1.ApplicationSource{
							RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHTTPS),
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
						Git: &v1alpha1.GitGenerator{
							RepoURL: fixture.RepoURL(fixture.RepoURLTypeHTTPS),
							Directories: []v1alpha1.GitDirectoryGeneratorItem{
								{
									Path: "*kustomize*",
								},
							},
						},
					},
				},
			},
		}).Then().Expect(ApplicationsExist(expectedApps)).
		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist(expectedAppsNewNamespace))
}

func TestGitGeneratorPrivateRepoGoTemplate(t *testing.T) {
	FailOnErr(fixture.RunCli("repo", "add", fixture.RepoURL(fixture.RepoURLTypeHTTPS), "--username", fixture.GitUsername, "--password", fixture.GitPassword, "--insecure-skip-server-verification"))
	generateExpectedApp := func(name string) argov1alpha1.Application {
		return argov1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Application",
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  utils.ArgoCDNamespace,
				Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			},
			Spec: argov1alpha1.ApplicationSpec{
				Project: "default",
				Source: argov1alpha1.ApplicationSource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHTTPS),
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
		generateExpectedApp("https-kustomize-base"),
	}

	var expectedAppsNewNamespace []argov1alpha1.Application

	Given(t).
		When().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{ObjectMeta: metav1.ObjectMeta{
			Name: "simple-git-generator-private",
		},
			Spec: v1alpha1.ApplicationSetSpec{
				GoTemplate: true,
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{.path.basename}}"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: argov1alpha1.ApplicationSource{
							RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHTTPS),
							TargetRevision: "HEAD",
							Path:           "{{.path.path}}",
						},
						Destination: argov1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "{{.path.basename}}",
						},
					},
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						Git: &v1alpha1.GitGenerator{
							RepoURL: fixture.RepoURL(fixture.RepoURLTypeHTTPS),
							Directories: []v1alpha1.GitDirectoryGeneratorItem{
								{
									Path: "*kustomize*",
								},
							},
						},
					},
				},
			},
		}).Then().Expect(ApplicationsExist(expectedApps)).
		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist(expectedAppsNewNamespace))
}
