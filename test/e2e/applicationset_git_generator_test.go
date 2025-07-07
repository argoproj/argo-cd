package e2e

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/applicationsets"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture/applicationsets/utils"
)

func randStr(t *testing.T) string {
	t.Helper()
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		require.NoError(t, err)
		return ""
	}
	return hex.EncodeToString(bytes)
}

func TestSimpleGitDirectoryGenerator(t *testing.T) {
	generateExpectedApp := func(name string) v1alpha1.Application {
		return v1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
				Finalizers: []string{v1alpha1.ResourcesFinalizerName},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "default",
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
					TargetRevision: "HEAD",
					Path:           name,
				},
				Destination: v1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: name,
				},
			},
		}
	}

	expectedApps := []v1alpha1.Application{
		generateExpectedApp("kustomize-guestbook"),
		generateExpectedApp("helm-guestbook"),
	}

	var expectedAppsNewNamespace []v1alpha1.Application
	var expectedAppsNewMetadata []v1alpha1.Application

	Given(t).
		When().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{path.basename}}"},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "{{path}}",
						},
						Destination: v1alpha1.ApplicationDestination{
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
	generateExpectedApp := func(name string) v1alpha1.Application {
		return v1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
				Finalizers: []string{v1alpha1.ResourcesFinalizerName},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "default",
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
					TargetRevision: "HEAD",
					Path:           name,
				},
				Destination: v1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: name,
				},
			},
		}
	}

	expectedApps := []v1alpha1.Application{
		generateExpectedApp("kustomize-guestbook"),
		generateExpectedApp("helm-guestbook"),
	}

	var expectedAppsNewNamespace []v1alpha1.Application
	var expectedAppsNewMetadata []v1alpha1.Application

	Given(t).
		When().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				GoTemplate: true,
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{.path.basename}}"},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "{{.path.path}}",
						},
						Destination: v1alpha1.ApplicationDestination{
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

func TestSimpleGitDirectoryGeneratorGPGEnabledUnsignedCommits(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	expectedErrorMessage := `error generating params from git: error getting directories from repo: error retrieving Git Directories: rpc error: code = Unknown desc = permission denied`
	expectedConditionsParamsError := []v1alpha1.ApplicationSetCondition{
		{
			Type:    v1alpha1.ApplicationSetConditionErrorOccurred,
			Status:  v1alpha1.ApplicationSetConditionStatusTrue,
			Message: expectedErrorMessage,
			Reason:  v1alpha1.ApplicationSetReasonApplicationParamsGenerationError,
		},
		{
			Type:    v1alpha1.ApplicationSetConditionParametersGenerated,
			Status:  v1alpha1.ApplicationSetConditionStatusFalse,
			Message: expectedErrorMessage,
			Reason:  v1alpha1.ApplicationSetReasonErrorOccurred,
		},
		{
			Type:    v1alpha1.ApplicationSetConditionResourcesUpToDate,
			Status:  v1alpha1.ApplicationSetConditionStatusFalse,
			Message: expectedErrorMessage,
			Reason:  v1alpha1.ApplicationSetReasonErrorOccurred,
		},
	}
	generateExpectedApp := func(name string) v1alpha1.Application {
		return v1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
				Finalizers: []string{v1alpha1.ResourcesFinalizerName},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "default",
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
					TargetRevision: "HEAD",
					Path:           name,
				},
				Destination: v1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: name,
				},
			},
		}
	}

	expectedApps := []v1alpha1.Application{
		generateExpectedApp("guestbook"),
	}
	project := "gpg"

	Given(t).
		When().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{path.basename}}"},
					Spec: v1alpha1.ApplicationSpec{
						Project: project,
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "{{path}}",
						},
						Destination: v1alpha1.ApplicationDestination{
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
									Path: guestbookPath,
								},
							},
						},
					},
				},
			},
		}).
		Then().Expect(ApplicationsDoNotExist(expectedApps)).
		// verify the ApplicationSet error status conditions were set correctly
		Expect(ApplicationSetHasConditions("simple-git-generator", expectedConditionsParamsError)).
		When().
		Delete().Then().Expect(ApplicationsDoNotExist(expectedApps))
}

func TestSimpleGitDirectoryGeneratorGPGEnabledWithoutKnownKeys(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	expectedErrorMessage := `error generating params from git: error getting directories from repo: error retrieving Git Directories: rpc error: code = Unknown desc = permission denied`
	expectedConditionsParamsError := []v1alpha1.ApplicationSetCondition{
		{
			Type:    v1alpha1.ApplicationSetConditionErrorOccurred,
			Status:  v1alpha1.ApplicationSetConditionStatusTrue,
			Message: expectedErrorMessage,
			Reason:  v1alpha1.ApplicationSetReasonApplicationParamsGenerationError,
		},
		{
			Type:    v1alpha1.ApplicationSetConditionParametersGenerated,
			Status:  v1alpha1.ApplicationSetConditionStatusFalse,
			Message: expectedErrorMessage,
			Reason:  v1alpha1.ApplicationSetReasonErrorOccurred,
		},
		{
			Type:    v1alpha1.ApplicationSetConditionResourcesUpToDate,
			Status:  v1alpha1.ApplicationSetConditionStatusFalse,
			Message: expectedErrorMessage,
			Reason:  v1alpha1.ApplicationSetReasonErrorOccurred,
		},
	}
	generateExpectedApp := func(name string) v1alpha1.Application {
		return v1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
				Finalizers: []string{v1alpha1.ResourcesFinalizerName},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "default",
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
					TargetRevision: "HEAD",
					Path:           name,
				},
				Destination: v1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: name,
				},
			},
		}
	}

	expectedApps := []v1alpha1.Application{
		generateExpectedApp("guestbook"),
	}

	project := "gpg"

	Given(t).
		Path(guestbookPath).
		When().
		AddSignedFile("test.yaml", randStr(t)).IgnoreErrors().
		IgnoreErrors().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{path.basename}}"},
					Spec: v1alpha1.ApplicationSpec{
						Project: project,
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "{{path}}",
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "{{path.basename}}",
						},
						// Automatically create resources
						SyncPolicy: &v1alpha1.SyncPolicy{
							Automated: &v1alpha1.SyncPolicyAutomated{},
						},
					},
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						Git: &v1alpha1.GitGenerator{
							RepoURL: "https://github.com/argoproj/argocd-example-apps.git",
							Directories: []v1alpha1.GitDirectoryGeneratorItem{
								{
									Path: guestbookPath,
								},
							},
						},
					},
				},
			},
		}).Then().
		// verify the ApplicationSet error status conditions were set correctly
		Expect(ApplicationSetHasConditions("simple-git-generator", expectedConditionsParamsError)).
		Expect(ApplicationsDoNotExist(expectedApps)).
		When().
		Delete().Then().Expect(ApplicationsDoNotExist(expectedApps))
}

func TestSimpleGitFilesGenerator(t *testing.T) {
	generateExpectedApp := func(name string) v1alpha1.Application {
		return v1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
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
					Server:    "https://kubernetes.default.svc",
					Namespace: "guestbook",
				},
			},
		}
	}

	expectedApps := []v1alpha1.Application{
		generateExpectedApp("engineering-dev-guestbook"),
		generateExpectedApp("engineering-prod-guestbook"),
	}

	var expectedAppsNewNamespace []v1alpha1.Application
	var expectedAppsNewMetadata []v1alpha1.Application

	Given(t).
		When().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{cluster.name}}-guestbook"},
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

func TestSimpleGitFilesGeneratorGPGEnabledUnsignedCommits(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	expectedErrorMessage := `error generating params from git: error retrieving Git files: rpc error: code = Unknown desc = permission denied`
	expectedConditionsParamsError := []v1alpha1.ApplicationSetCondition{
		{
			Type:    v1alpha1.ApplicationSetConditionErrorOccurred,
			Status:  v1alpha1.ApplicationSetConditionStatusTrue,
			Message: expectedErrorMessage,
			Reason:  v1alpha1.ApplicationSetReasonApplicationParamsGenerationError,
		},
		{
			Type:    v1alpha1.ApplicationSetConditionParametersGenerated,
			Status:  v1alpha1.ApplicationSetConditionStatusFalse,
			Message: expectedErrorMessage,
			Reason:  v1alpha1.ApplicationSetReasonErrorOccurred,
		},
		{
			Type:    v1alpha1.ApplicationSetConditionResourcesUpToDate,
			Status:  v1alpha1.ApplicationSetConditionStatusFalse,
			Message: expectedErrorMessage,
			Reason:  v1alpha1.ApplicationSetReasonErrorOccurred,
		},
	}
	project := "gpg"
	generateExpectedApp := func(name string) v1alpha1.Application {
		return v1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
				Finalizers: []string{v1alpha1.ResourcesFinalizerName},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: project,
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
	}

	expectedApps := []v1alpha1.Application{
		generateExpectedApp("engineering-dev-guestbook"),
		generateExpectedApp("engineering-prod-guestbook"),
	}

	Given(t).
		When().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{cluster.name}}-guestbook"},
					Spec: v1alpha1.ApplicationSpec{
						Project: project,
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
		}).Then().Expect(ApplicationsDoNotExist(expectedApps)).
		// verify the ApplicationSet error status conditions were set correctly
		Expect(ApplicationSetHasConditions("simple-git-generator", expectedConditionsParamsError)).
		When().
		Delete().Then().Expect(ApplicationsDoNotExist(expectedApps))
}

func TestSimpleGitFilesGeneratorGPGEnabledWithoutKnownKeys(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	expectedErrorMessage := `error generating params from git: error retrieving Git files: rpc error: code = Unknown desc = permission denied`
	expectedConditionsParamsError := []v1alpha1.ApplicationSetCondition{
		{
			Type:    v1alpha1.ApplicationSetConditionErrorOccurred,
			Status:  v1alpha1.ApplicationSetConditionStatusTrue,
			Message: expectedErrorMessage,
			Reason:  v1alpha1.ApplicationSetReasonApplicationParamsGenerationError,
		},
		{
			Type:    v1alpha1.ApplicationSetConditionParametersGenerated,
			Status:  v1alpha1.ApplicationSetConditionStatusFalse,
			Message: expectedErrorMessage,
			Reason:  v1alpha1.ApplicationSetReasonErrorOccurred,
		},
		{
			Type:    v1alpha1.ApplicationSetConditionResourcesUpToDate,
			Status:  v1alpha1.ApplicationSetConditionStatusFalse,
			Message: expectedErrorMessage,
			Reason:  v1alpha1.ApplicationSetReasonErrorOccurred,
		},
	}
	project := "gpg"
	generateExpectedApp := func(name string) v1alpha1.Application {
		return v1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
				Finalizers: []string{v1alpha1.ResourcesFinalizerName},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: project,
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
	}

	expectedApps := []v1alpha1.Application{
		generateExpectedApp("engineering-dev-guestbook"),
		generateExpectedApp("engineering-prod-guestbook"),
	}

	Given(t).
		Path(guestbookPath).
		When().
		AddSignedFile("test.yaml", randStr(t)).IgnoreErrors().
		IgnoreErrors().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{cluster.name}}-guestbook"},
					Spec: v1alpha1.ApplicationSpec{
						Project: project,
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
		}).Then().
		// verify the ApplicationSet error status conditions were set correctly
		Expect(ApplicationSetHasConditions("simple-git-generator", expectedConditionsParamsError)).
		Expect(ApplicationsDoNotExist(expectedApps)).
		When().
		Delete().Then().Expect(ApplicationsDoNotExist(expectedApps))
}

func TestSimpleGitFilesGeneratorGoTemplate(t *testing.T) {
	generateExpectedApp := func(name string) v1alpha1.Application {
		return v1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
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
					Server:    "https://kubernetes.default.svc",
					Namespace: "guestbook",
				},
			},
		}
	}

	expectedApps := []v1alpha1.Application{
		generateExpectedApp("engineering-dev-guestbook"),
		generateExpectedApp("engineering-prod-guestbook"),
	}

	var expectedAppsNewNamespace []v1alpha1.Application
	var expectedAppsNewMetadata []v1alpha1.Application

	Given(t).
		When().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				GoTemplate: true,
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{.cluster.name}}-guestbook"},
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
		CreateNamespace(utils.ApplicationsResourcesNamespace).
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{cluster.name}}-guestbook"},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: utils.ApplicationsResourcesNamespace,
						},

						// Automatically create resources
						SyncPolicy: &v1alpha1.SyncPolicy{
							Automated: &v1alpha1.SyncPolicyAutomated{},
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
		}).Then().ExpectWithDuration(Pod(t, func(p corev1.Pod) bool { return strings.Contains(p.Name, "guestbook-ui") }), 6*time.Minute).
		When().
		Delete().
		And(func() {
			t.Log("Waiting 15 seconds to give the cluster a chance to delete the pods.")
			// Wait 15 seconds to give the cluster a chance to deletes the pods, if it is going to do so.
			// It should NOT delete the pods; to do so would be an ApplicationSet bug, and
			// that is what we are testing here.
			time.Sleep(15 * time.Second)
			// The pod should continue to exist after 15 seconds.
		}).Then().Expect(Pod(t, func(p corev1.Pod) bool { return strings.Contains(p.Name, "guestbook-ui") }))
}

func TestSimpleGitFilesPreserveResourcesOnDeletionGoTemplate(t *testing.T) {
	Given(t).
		When().
		CreateNamespace(utils.ApplicationsResourcesNamespace).
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				GoTemplate: true,
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{.cluster.name}}-guestbook"},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: utils.ApplicationsResourcesNamespace,
						},

						// Automatically create resources
						SyncPolicy: &v1alpha1.SyncPolicy{
							Automated: &v1alpha1.SyncPolicyAutomated{},
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
		}).Then().ExpectWithDuration(Pod(t, func(p corev1.Pod) bool { return strings.Contains(p.Name, "guestbook-ui") }), 6*time.Minute).
		When().
		Delete().
		And(func() {
			t.Log("Waiting 15 seconds to give the cluster a chance to delete the pods.")
			// Wait 15 seconds to give the cluster a chance to deletes the pods, if it is going to do so.
			// It should NOT delete the pods; to do so would be an ApplicationSet bug, and
			// that is what we are testing here.
			time.Sleep(15 * time.Second)
			// The pod should continue to exist after 15 seconds.
		}).Then().Expect(Pod(t, func(p corev1.Pod) bool { return strings.Contains(p.Name, "guestbook-ui") }))
}

func TestGitGeneratorPrivateRepo(t *testing.T) {
	generateExpectedApp := func(name string) v1alpha1.Application {
		return v1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
				Finalizers: []string{v1alpha1.ResourcesFinalizerName},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "default",
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHTTPS),
					TargetRevision: "HEAD",
					Path:           name,
				},
				Destination: v1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: name,
				},
			},
		}
	}

	expectedApps := []v1alpha1.Application{
		generateExpectedApp("https-kustomize-base"),
	}

	var expectedAppsNewNamespace []v1alpha1.Application

	Given(t).
		HTTPSInsecureRepoURLAdded("").
		When().
		// Create a GitGenerator-based ApplicationSet

		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator-private",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{path.basename}}"},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHTTPS),
							TargetRevision: "HEAD",
							Path:           "{{path}}",
						},
						Destination: v1alpha1.ApplicationDestination{
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
	generateExpectedApp := func(name string) v1alpha1.Application {
		return v1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
				Finalizers: []string{v1alpha1.ResourcesFinalizerName},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "default",
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHTTPS),
					TargetRevision: "HEAD",
					Path:           name,
				},
				Destination: v1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: name,
				},
			},
		}
	}

	expectedApps := []v1alpha1.Application{
		generateExpectedApp("https-kustomize-base"),
	}

	var expectedAppsNewNamespace []v1alpha1.Application

	Given(t).
		HTTPSInsecureRepoURLAdded("").
		When().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator-private",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				GoTemplate: true,
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{.path.basename}}"},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHTTPS),
							TargetRevision: "HEAD",
							Path:           "{{.path.path}}",
						},
						Destination: v1alpha1.ApplicationDestination{
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

func TestSimpleGitGeneratorPrivateRepoWithNoRepo(t *testing.T) {
	generateExpectedApp := func(name string) v1alpha1.Application {
		return v1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
				Finalizers: []string{v1alpha1.ResourcesFinalizerName},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "default",
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHTTPS),
					TargetRevision: "HEAD",
					Path:           name,
				},
				Destination: v1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: name,
				},
			},
		}
	}

	expectedApps := []v1alpha1.Application{
		generateExpectedApp("https-kustomize-base"),
	}

	var expectedAppsNewNamespace []v1alpha1.Application

	Given(t).
		When().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator-private",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{path.basename}}"},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHTTPS),
							TargetRevision: "HEAD",
							Path:           "{{path}}",
						},
						Destination: v1alpha1.ApplicationDestination{
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
		}).Then().Expect(ApplicationsDoNotExist(expectedApps)).
		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist(expectedAppsNewNamespace))
}

func TestSimpleGitGeneratorPrivateRepoWithMatchingProject(t *testing.T) {
	generateExpectedApp := func(name string) v1alpha1.Application {
		return v1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
				Finalizers: []string{v1alpha1.ResourcesFinalizerName},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "default",
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHTTPS),
					TargetRevision: "HEAD",
					Path:           name,
				},
				Destination: v1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: name,
				},
			},
		}
	}

	expectedApps := []v1alpha1.Application{
		generateExpectedApp("https-kustomize-base"),
	}

	var expectedAppsNewNamespace []v1alpha1.Application

	Given(t).
		HTTPSInsecureRepoURLAdded("default").
		When().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator-private",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{path.basename}}"},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHTTPS),
							TargetRevision: "HEAD",
							Path:           "{{path}}",
						},
						Destination: v1alpha1.ApplicationDestination{
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

func TestSimpleGitGeneratorPrivateRepoWithMismatchingProject(t *testing.T) {
	generateExpectedApp := func(name string) v1alpha1.Application {
		return v1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
				Finalizers: []string{v1alpha1.ResourcesFinalizerName},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "default",
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHTTPS),
					TargetRevision: "HEAD",
					Path:           name,
				},
				Destination: v1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: name,
				},
			},
		}
	}

	expectedApps := []v1alpha1.Application{
		generateExpectedApp("https-kustomize-base"),
	}

	var expectedAppsNewNamespace []v1alpha1.Application

	Given(t).
		HTTPSInsecureRepoURLAdded("some-other-project").
		When().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator-private",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{path.basename}}"},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHTTPS),
							TargetRevision: "HEAD",
							Path:           "{{path}}",
						},
						Destination: v1alpha1.ApplicationDestination{
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
		}).Then().Expect(ApplicationsDoNotExist(expectedApps)).
		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist(expectedAppsNewNamespace))
}

func TestGitGeneratorPrivateRepoWithTemplatedProject(t *testing.T) {
	generateExpectedApp := func(name string) v1alpha1.Application {
		return v1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
				Finalizers: []string{v1alpha1.ResourcesFinalizerName},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "default",
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHTTPS),
					TargetRevision: "HEAD",
					Path:           name,
				},
				Destination: v1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: name,
				},
			},
		}
	}

	expectedApps := []v1alpha1.Application{
		generateExpectedApp("https-kustomize-base"),
	}

	var expectedAppsNewNamespace []v1alpha1.Application

	Given(t).
		HTTPSInsecureRepoURLAdded("").
		When().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator-private",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{path.basename}}"},
					Spec: v1alpha1.ApplicationSpec{
						Project: "{{values.project}}",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHTTPS),
							TargetRevision: "HEAD",
							Path:           "{{path}}",
						},
						Destination: v1alpha1.ApplicationDestination{
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
							Values: map[string]string{
								"project": "default",
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

func TestGitGeneratorPrivateRepoWithTemplatedProjectAndProjectScopedRepo(t *testing.T) {
	// Flush repo-server cache. Why? We want to ensure that the previous test has not already populated the repo-server
	// cache.
	r := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	all := r.FlushAll(t.Context())
	require.NoError(t, all.Err())

	generateExpectedApp := func(name string) v1alpha1.Application {
		return v1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
				Finalizers: []string{v1alpha1.ResourcesFinalizerName},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "default",
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHTTPS),
					TargetRevision: "HEAD",
					Path:           name,
				},
				Destination: v1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: name,
				},
			},
		}
	}

	expectedApps := []v1alpha1.Application{
		generateExpectedApp("https-kustomize-base"),
	}

	var expectedAppsNewNamespace []v1alpha1.Application

	Given(t).
		HTTPSInsecureRepoURLAdded("default").
		When().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator-private",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{path.basename}}"},
					Spec: v1alpha1.ApplicationSpec{
						Project: "{{values.project}}",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHTTPS),
							TargetRevision: "HEAD",
							Path:           "{{path}}",
						},
						Destination: v1alpha1.ApplicationDestination{
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
							Values: map[string]string{
								"project": "default",
							},
						},
					},
				},
			},
		}).Then().Expect(ApplicationsDoNotExist(expectedApps)).
		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist(expectedAppsNewNamespace))
}
