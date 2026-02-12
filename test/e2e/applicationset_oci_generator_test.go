package e2e

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/applicationsets"
)

func TestSimpleOciDirectoryGenerator(t *testing.T) {
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
					RepoURL:        "oci://localhost:5000/testdata",
					TargetRevision: "1.0.0",
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

	Given(t).
		PushOCIArtifact("testdata", "1.0.0", "guestbook").
		AddOCIRepository("testdata", "testdata").
		When().
		Create(v1alpha1.ApplicationSet{
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{path.basename}}"},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "oci://localhost:5000/testdata",
							TargetRevision: "1.0.0",
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
						Oci: &v1alpha1.OciGenerator{
							RepoURL:  "oci://localhost:5000/testdata",
							Revision: "1.0.0",
							Directories: []v1alpha1.OciDirectoryGeneratorItem{
								{
									Path: "*",
								},
							},
						},
					},
				},
			},
		}).
		Then().Expect(ApplicationsExist(expectedApps)).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist(expectedApps))
}
