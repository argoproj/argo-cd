package e2e

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/pkg/rand"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/applicationsets"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture/applicationsets/utils"
	. "github.com/argoproj/argo-cd/v2/util/errors"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application"
)

var ExpectedConditions = []v1alpha1.ApplicationSetCondition{
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

func TestSimpleListGeneratorExternalNamespace(t *testing.T) {
	externalNamespace := string(utils.ArgoCDExternalNamespace)

	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "my-cluster-guestbook",
			Namespace:  externalNamespace,
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
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
		When().
		SwitchToExternalNamespace(utils.ArgoCDExternalNamespace).
		CreateNamespace(externalNamespace).Create(v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple-list-generator-external",
			Namespace: externalNamespace,
		},
		Spec: v1alpha1.ApplicationSetSpec{
			GoTemplate: true,
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{.cluster}}-guestbook"},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: &argov1alpha1.ApplicationSource{
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
			expectedAppNewMetadata.ObjectMeta.Labels = map[string]string{
				"label-key": "label-value",
			}
		}).
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Template.Annotations = map[string]string{"annotation-key": "annotation-value"}
			appset.Spec.Template.Labels = map[string]string{
				"label-key": "label-value",
			}
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{*expectedAppNewMetadata})).

		// verify the ApplicationSet status conditions were set correctly
		Expect(ApplicationSetHasConditions("simple-list-generator-external", ExpectedConditions)).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist([]argov1alpha1.Application{*expectedAppNewMetadata}))
}

func TestSimpleListGeneratorExternalNamespaceNoConflict(t *testing.T) {
	externalNamespace := string(utils.ArgoCDExternalNamespace)
	externalNamespace2 := string(utils.ArgoCDExternalNamespace2)

	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "my-cluster-guestbook",
			Namespace:  externalNamespace,
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
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

	expectedAppExternalNamespace2 := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "my-cluster-guestbook",
			Namespace:  externalNamespace2,
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
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
		When().
		SwitchToExternalNamespace(utils.ArgoCDExternalNamespace2).
		CreateNamespace(externalNamespace2).Create(v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple-list-generator-external",
			Namespace: externalNamespace2,
		},
		Spec: v1alpha1.ApplicationSetSpec{
			GoTemplate: true,
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{.cluster}}-guestbook"},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: &argov1alpha1.ApplicationSource{
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
	}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{expectedAppExternalNamespace2})).
		When().
		SwitchToExternalNamespace(utils.ArgoCDExternalNamespace).
		CreateNamespace(externalNamespace).Create(v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple-list-generator-external",
			Namespace: externalNamespace,
		},
		Spec: v1alpha1.ApplicationSetSpec{
			GoTemplate: true,
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{.cluster}}-guestbook"},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: &argov1alpha1.ApplicationSource{
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
		When().
		SwitchToExternalNamespace(utils.ArgoCDExternalNamespace2).
		Then().
		Expect(ApplicationsExist([]argov1alpha1.Application{expectedAppExternalNamespace2})).
		When().
		SwitchToExternalNamespace(utils.ArgoCDExternalNamespace).
		Then().
		// Update the ApplicationSet template namespace, and verify it updates the Applications
		When().
		And(func() {
			expectedAppNewNamespace = expectedApp.DeepCopy()
			expectedAppNewNamespace.Spec.Destination.Namespace = "guestbook2"
		}).
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Template.Spec.Destination.Namespace = "guestbook2"
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{*expectedAppNewNamespace})).
		When().
		SwitchToExternalNamespace(utils.ArgoCDExternalNamespace2).
		Then().
		Expect(ApplicationsExist([]argov1alpha1.Application{expectedAppExternalNamespace2})).
		When().
		SwitchToExternalNamespace(utils.ArgoCDExternalNamespace).
		Then().
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
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{*expectedAppNewMetadata})).

		// verify the ApplicationSet status conditions were set correctly
		Expect(ApplicationSetHasConditions("simple-list-generator-external", ExpectedConditions)).
		When().
		SwitchToExternalNamespace(utils.ArgoCDExternalNamespace2).
		Then().
		Expect(ApplicationsExist([]argov1alpha1.Application{expectedAppExternalNamespace2})).
		When().
		SwitchToExternalNamespace(utils.ArgoCDExternalNamespace).
		Then().
		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist([]argov1alpha1.Application{*expectedAppNewMetadata})).
		When().
		SwitchToExternalNamespace(utils.ArgoCDExternalNamespace2).
		Then().
		Expect(ApplicationsExist([]argov1alpha1.Application{expectedAppExternalNamespace2})).
		When().
		Delete().Then().Expect(ApplicationsDoNotExist([]argov1alpha1.Application{expectedAppExternalNamespace2}))
}

func TestSimpleListGenerator(t *testing.T) {
	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "my-cluster-guestbook",
			Namespace:  fixture.TestNamespace(),
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
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
		When().Create(v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "simple-list-generator",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{cluster}}-guestbook"},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: &argov1alpha1.ApplicationSource{
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
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "my-cluster-guestbook",
			Namespace:  fixture.TestNamespace(),
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
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
		When().Create(v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "simple-list-generator",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			GoTemplate: true,
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{.cluster}}-guestbook"},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: &argov1alpha1.ApplicationSource{
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

func TestRenderHelmValuesObject(t *testing.T) {
	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "my-cluster-guestbook",
			Namespace:  fixture.TestNamespace(),
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "helm-guestbook",
				Helm: &argov1alpha1.ApplicationSourceHelm{
					ValuesObject: &runtime.RawExtension{
						// This will always be converted as yaml
						Raw: []byte(`{"some":{"string":"Hello world"}}`),
					},
				},
			},
			Destination: argov1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
		},
	}

	Given(t).
		// Create a ListGenerator-based ApplicationSet
		When().Create(v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-values-object",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			GoTemplate: true,
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{.cluster}}-guestbook"},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: &argov1alpha1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						TargetRevision: "HEAD",
						Path:           "helm-guestbook",
						Helm: &argov1alpha1.ApplicationSourceHelm{
							ValuesObject: &runtime.RawExtension{
								Raw: []byte(`{"some":{"string":"{{.test}}"}}`),
							},
						},
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
							Raw: []byte(`{"cluster": "my-cluster","url": "https://kubernetes.default.svc", "test": "Hello world"}`),
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

func TestTemplatePatch(t *testing.T) {
	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "my-cluster-guestbook",
			Namespace:  fixture.TestNamespace(),
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			Annotations: map[string]string{
				"annotation-some-key": "annotation-some-value",
			},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: argov1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
			SyncPolicy: &argov1alpha1.SyncPolicy{
				SyncOptions: argov1alpha1.SyncOptions{"CreateNamespace=true"},
			},
		},
	}

	templatePatch := `{
		"metadata": {
			"annotations": {
				{{- range $k, $v := .annotations }}
				"{{ $k }}": "{{ $v }}"
				{{- end }}
			}
		},
		{{- if .createNamespace }}
		"spec": {
			"syncPolicy": {
				"syncOptions": [
					"CreateNamespace=true"
				]
			}
		}
		{{- end }}
	}
	`

	var expectedAppNewNamespace *argov1alpha1.Application
	var expectedAppNewMetadata *argov1alpha1.Application

	Given(t).
		// Create a ListGenerator-based ApplicationSet
		When().Create(v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "patch-template",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			GoTemplate: true,
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{.cluster}}-guestbook"},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: &argov1alpha1.ApplicationSource{
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
			TemplatePatch: &templatePatch,
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					List: &v1alpha1.ListGenerator{
						Elements: []apiextensionsv1.JSON{{
							Raw: []byte(`{
									"cluster": "my-cluster",
									"url": "https://kubernetes.default.svc",
									"createNamespace": true,
									"annotations": {
										"annotation-some-key": "annotation-some-value"
									}
								}`),
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
			expectedAppNewMetadata.ObjectMeta.Labels = map[string]string{
				"label-key": "label-value",
			}
		}).
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Template.Labels = map[string]string{"label-key": "label-value"}
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{*expectedAppNewMetadata})).

		// verify the ApplicationSet status conditions were set correctly
		Expect(ApplicationSetHasConditions("patch-template", ExpectedConditions)).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist([]argov1alpha1.Application{*expectedAppNewMetadata}))
}

func TestUpdateHelmValuesObject(t *testing.T) {
	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "my-cluster-guestbook",
			Namespace:  fixture.TestNamespace(),
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "helm-guestbook",
				Helm: &argov1alpha1.ApplicationSourceHelm{
					ValuesObject: &runtime.RawExtension{
						// This will always be converted as yaml
						Raw: []byte(`{"some":{"foo":"bar"}}`),
					},
				},
			},
			Destination: argov1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "guestbook",
			},
		},
	}

	Given(t).
		// Create a ListGenerator-based ApplicationSet
		When().Create(v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-values-object-patch",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			GoTemplate: true,
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{.cluster}}-guestbook"},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: &argov1alpha1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						TargetRevision: "HEAD",
						Path:           "helm-guestbook",
						Helm: &argov1alpha1.ApplicationSourceHelm{
							ValuesObject: &runtime.RawExtension{
								Raw: []byte(`{"some":{"string":"{{.test}}"}}`),
							},
						},
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
							Raw: []byte(`{"cluster": "my-cluster","url": "https://kubernetes.default.svc", "test": "Hello world"}`),
						}},
					},
				},
			},
		},
	}).Then().
		Expect(ApplicationSetHasConditions("test-values-object-patch", ExpectedConditions)).
		When().
		// Update the app spec with some knew ValuesObject to force a merge
		Update(func(as *argov1alpha1.ApplicationSet) {
			as.Spec.Template.Spec.Source.Helm.ValuesObject = &runtime.RawExtension{
				Raw: []byte(`{"some":{"foo":"bar"}}`),
			}
		}).
		Then().
		Expect(ApplicationsExist([]argov1alpha1.Application{expectedApp})).
		When().
		// Delete the ApplicationSet, and verify it deletes the Applications
		Delete().Then().Expect(ApplicationsDoNotExist([]argov1alpha1.Application{expectedApp}))
}

func TestSyncPolicyCreateUpdate(t *testing.T) {
	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "my-cluster-guestbook-sync-policy-create-update",
			Namespace:  utils.ArgoCDNamespace,
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
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
		When().Create(v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sync-policy-create-update",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			GoTemplate: true,
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
					Name:       "{{.cluster}}-guestbook-sync-policy-create-update",
					Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
				},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: &argov1alpha1.ApplicationSource{
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

		// Update the metadata fields in the appset template
		// Update as well the policy
		// As policy is create-update, updates must reflected
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
			applicationsSyncPolicy := argov1alpha1.ApplicationsSyncPolicyCreateUpdate
			appset.Spec.SyncPolicy = &argov1alpha1.ApplicationSetSyncPolicy{
				ApplicationsSync: &applicationsSyncPolicy,
			}
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{*expectedAppNewMetadata})).

		// Update the list and remove element
		// As policy is create-update, app deletion must not be reflected
		When().
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Generators = []v1alpha1.ApplicationSetGenerator{}
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{*expectedAppNewMetadata})).

		// verify the ApplicationSet status conditions were set correctly
		Expect(ApplicationSetHasConditions("sync-policy-create-update", ExpectedConditions)).

		// Delete the ApplicationSet, and verify it not deletes the Applications
		// As policy is create-update, AppSet controller will remove all generated applications's ownerReferences on delete AppSet
		// So AppSet deletion will be reflected, but all the applications it generates will still exist
		When().
		Delete().Then().Expect(ApplicationsExist([]argov1alpha1.Application{*expectedAppNewMetadata}))
}

func TestSyncPolicyCreateDelete(t *testing.T) {
	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "my-cluster-guestbook-sync-policy-create-delete",
			Namespace:  utils.ArgoCDNamespace,
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
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

	Given(t).
		// Create a ListGenerator-based ApplicationSet
		When().Create(v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sync-policy-create-delete",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			GoTemplate: true,
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{.cluster}}-guestbook-sync-policy-create-delete"},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: &argov1alpha1.ApplicationSource{
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

		// Update the metadata fields in the appset template
		// Update as well the policy
		// As policy is create-delete, updates must not be reflected
		When().
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Template.Annotations = map[string]string{"annotation-key": "annotation-value"}
			appset.Spec.Template.Labels = map[string]string{"label-key": "label-value"}
			applicationsSyncPolicy := argov1alpha1.ApplicationsSyncPolicyCreateDelete
			appset.Spec.SyncPolicy = &argov1alpha1.ApplicationSetSyncPolicy{
				ApplicationsSync: &applicationsSyncPolicy,
			}
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{*expectedAppNewNamespace})).

		// Update the list and remove element
		// As policy is create-delete, app deletion must be reflected
		When().
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Generators = []v1alpha1.ApplicationSetGenerator{}
		}).Then().Expect(ApplicationsDoNotExist([]argov1alpha1.Application{*expectedAppNewNamespace})).

		// verify the ApplicationSet status conditions were set correctly
		Expect(ApplicationSetHasConditions("sync-policy-create-delete", ExpectedConditions)).

		// Delete the ApplicationSet
		When().
		Delete().Then().Expect(ApplicationsDoNotExist([]argov1alpha1.Application{*expectedAppNewNamespace}))
}

func TestSyncPolicyCreateOnly(t *testing.T) {
	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "my-cluster-guestbook-sync-policy-create-only",
			Namespace:  utils.ArgoCDNamespace,
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
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

	Given(t).
		// Create a ListGenerator-based ApplicationSet
		When().Create(v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sync-policy-create-only",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			GoTemplate: true,
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
					Name:       "{{.cluster}}-guestbook-sync-policy-create-only",
					Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
				},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: &argov1alpha1.ApplicationSource{
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

		// Update the metadata fields in the appset template
		// Update as well the policy
		// As policy is create-only, updates must not be reflected
		When().
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Template.Annotations = map[string]string{"annotation-key": "annotation-value"}
			appset.Spec.Template.Labels = map[string]string{"label-key": "label-value"}
			applicationsSyncPolicy := argov1alpha1.ApplicationsSyncPolicyCreateOnly
			appset.Spec.SyncPolicy = &argov1alpha1.ApplicationSetSyncPolicy{
				ApplicationsSync: &applicationsSyncPolicy,
			}
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{*expectedAppNewNamespace})).

		// Update the list and remove element
		// As policy is create-only, app deletion must not be reflected
		When().
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Generators = []v1alpha1.ApplicationSetGenerator{}
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{*expectedAppNewNamespace})).

		// verify the ApplicationSet status conditions were set correctly
		Expect(ApplicationSetHasConditions("sync-policy-create-only", ExpectedConditions)).

		// Delete the ApplicationSet, and verify it not deletes the Applications
		// As policy is create-update, AppSet controller will remove all generated applications's ownerReferences on delete AppSet
		// So AppSet deletion will be reflected, but all the applications it generates will still exist
		When().
		Delete().Then().Expect(ApplicationsExist([]argov1alpha1.Application{*expectedAppNewNamespace}))
}

func TestSimpleGitDirectoryGenerator(t *testing.T) {
	generateExpectedApp := func(name string) argov1alpha1.Application {
		return argov1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
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
		generateExpectedApp("kustomize-guestbook"),
		generateExpectedApp("helm-guestbook"),
		generateExpectedApp("ksonnet-guestbook"),
	}

	var expectedAppsNewNamespace []argov1alpha1.Application
	var expectedAppsNewMetadata []argov1alpha1.Application

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
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
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
		generateExpectedApp("kustomize-guestbook"),
		generateExpectedApp("helm-guestbook"),
		generateExpectedApp("ksonnet-guestbook"),
	}

	var expectedAppsNewNamespace []argov1alpha1.Application
	var expectedAppsNewMetadata []argov1alpha1.Application

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
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: &argov1alpha1.ApplicationSource{
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
			Reason:  v1alpha1.ApplicationSetReasonApplicationParamsGenerationError,
		},
	}
	generateExpectedApp := func(name string) argov1alpha1.Application {
		return argov1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
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
		generateExpectedApp("guestbook"),
	}
	project := "gpg"

	fixture.EnsureCleanState(t)
	Given(t).
		Project(project).
		When().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{path.basename}}"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: project,
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
			Reason:  v1alpha1.ApplicationSetReasonApplicationParamsGenerationError,
		},
	}
	generateExpectedApp := func(name string) argov1alpha1.Application {
		return argov1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
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
		generateExpectedApp("guestbook"),
	}

	project := "gpg"

	str, _ := rand.RandString(1)

	Given(t).
		Project(project).
		Path(guestbookPath).
		When().
		AddSignedFile("test.yaml", str).IgnoreErrors().
		IgnoreErrors().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{path.basename}}"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: project,
						Source: &argov1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "{{path}}",
						},
						Destination: argov1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "{{path.basename}}",
						},
						// Automatically create resources
						SyncPolicy: &argov1alpha1.SyncPolicy{
							Automated: &argov1alpha1.SyncPolicyAutomated{},
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
	generateExpectedApp := func(name string) argov1alpha1.Application {
		return argov1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
				Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			},
			Spec: argov1alpha1.ApplicationSpec{
				Project: "default",
				Source: &argov1alpha1.ApplicationSource{
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
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{cluster.name}}-guestbook"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: &argov1alpha1.ApplicationSource{
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
			Reason:  v1alpha1.ApplicationSetReasonApplicationParamsGenerationError,
		},
	}
	project := "gpg"
	generateExpectedApp := func(name string) argov1alpha1.Application {
		return argov1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
				Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			},
			Spec: argov1alpha1.ApplicationSpec{
				Project: project,
				Source: &argov1alpha1.ApplicationSource{
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

	fixture.EnsureCleanState(t)
	Given(t).
		Project(project).
		When().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{cluster.name}}-guestbook"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: project,
						Source: &argov1alpha1.ApplicationSource{
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
			Reason:  v1alpha1.ApplicationSetReasonApplicationParamsGenerationError,
		},
	}
	project := "gpg"
	generateExpectedApp := func(name string) argov1alpha1.Application {
		return argov1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
				Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			},
			Spec: argov1alpha1.ApplicationSpec{
				Project: project,
				Source: &argov1alpha1.ApplicationSource{
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

	str, _ := rand.RandString(1)

	expectedApps := []argov1alpha1.Application{
		generateExpectedApp("engineering-dev-guestbook"),
		generateExpectedApp("engineering-prod-guestbook"),
	}

	fixture.EnsureCleanState(t)
	Given(t).
		Project(project).
		Path(guestbookPath).
		When().
		AddSignedFile("test.yaml", str).IgnoreErrors().
		IgnoreErrors().
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{cluster.name}}-guestbook"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: project,
						Source: &argov1alpha1.ApplicationSource{
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
		}).Then().
		// verify the ApplicationSet error status conditions were set correctly
		Expect(ApplicationSetHasConditions("simple-git-generator", expectedConditionsParamsError)).
		Expect(ApplicationsDoNotExist(expectedApps)).
		When().
		Delete().Then().Expect(ApplicationsDoNotExist(expectedApps))
}

func TestSimpleGitFilesGeneratorGoTemplate(t *testing.T) {
	generateExpectedApp := func(name string) argov1alpha1.Application {
		return argov1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
				Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			},
			Spec: argov1alpha1.ApplicationSpec{
				Project: "default",
				Source: &argov1alpha1.ApplicationSource{
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
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				GoTemplate: true,
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{.cluster.name}}-guestbook"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: &argov1alpha1.ApplicationSource{
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
		CreateNamespace(utils.ApplicationsResourcesNamespace).
		// Create a GitGenerator-based ApplicationSet
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{cluster.name}}-guestbook"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: &argov1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: argov1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: utils.ApplicationsResourcesNamespace,
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
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: &argov1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: argov1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: utils.ApplicationsResourcesNamespace,
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

func githubSCMMockHandler(t *testing.T) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/api/v3/orgs/argoproj/repos?per_page=100":
			_, err := io.WriteString(w, `[
				{
				  "id": 1296269,
				  "node_id": "MDEwOlJlcG9zaXRvcnkxMjk2MjY5",
				  "name": "argo-cd",
				  "full_name": "argoproj/argo-cd",
				  "owner": {
					"login": "argoproj",
					"id": 1,
					"node_id": "MDQ6VXNlcjE=",
					"avatar_url": "https://github.com/images/error/argoproj_happy.gif",
					"gravatar_id": "",
					"url": "https://api.github.com/users/argoproj",
					"html_url": "https://github.com/argoproj",
					"followers_url": "https://api.github.com/users/argoproj/followers",
					"following_url": "https://api.github.com/users/argoproj/following{/other_user}",
					"gists_url": "https://api.github.com/users/argoproj/gists{/gist_id}",
					"starred_url": "https://api.github.com/users/argoproj/starred{/owner}{/repo}",
					"subscriptions_url": "https://api.github.com/users/argoproj/subscriptions",
					"organizations_url": "https://api.github.com/users/argoproj/orgs",
					"repos_url": "https://api.github.com/users/argoproj/repos",
					"events_url": "https://api.github.com/users/argoproj/events{/privacy}",
					"received_events_url": "https://api.github.com/users/argoproj/received_events",
					"type": "User",
					"site_admin": false
				  },
				  "private": false,
				  "html_url": "https://github.com/argoproj/argo-cd",
				  "description": "This your first repo!",
				  "fork": false,
				  "url": "https://api.github.com/repos/argoproj/argo-cd",
				  "archive_url": "https://api.github.com/repos/argoproj/argo-cd/{archive_format}{/ref}",
				  "assignees_url": "https://api.github.com/repos/argoproj/argo-cd/assignees{/user}",
				  "blobs_url": "https://api.github.com/repos/argoproj/argo-cd/git/blobs{/sha}",
				  "branches_url": "https://api.github.com/repos/argoproj/argo-cd/branches{/branch}",
				  "collaborators_url": "https://api.github.com/repos/argoproj/argo-cd/collaborators{/collaborator}",
				  "comments_url": "https://api.github.com/repos/argoproj/argo-cd/comments{/number}",
				  "commits_url": "https://api.github.com/repos/argoproj/argo-cd/commits{/sha}",
				  "compare_url": "https://api.github.com/repos/argoproj/argo-cd/compare/{base}...{head}",
				  "contents_url": "https://api.github.com/repos/argoproj/argo-cd/contents/{path}",
				  "contributors_url": "https://api.github.com/repos/argoproj/argo-cd/contributors",
				  "deployments_url": "https://api.github.com/repos/argoproj/argo-cd/deployments",
				  "downloads_url": "https://api.github.com/repos/argoproj/argo-cd/downloads",
				  "events_url": "https://api.github.com/repos/argoproj/argo-cd/events",
				  "forks_url": "https://api.github.com/repos/argoproj/argo-cd/forks",
				  "git_commits_url": "https://api.github.com/repos/argoproj/argo-cd/git/commits{/sha}",
				  "git_refs_url": "https://api.github.com/repos/argoproj/argo-cd/git/refs{/sha}",
				  "git_tags_url": "https://api.github.com/repos/argoproj/argo-cd/git/tags{/sha}",
				  "git_url": "git:github.com/argoproj/argo-cd.git",
				  "issue_comment_url": "https://api.github.com/repos/argoproj/argo-cd/issues/comments{/number}",
				  "issue_events_url": "https://api.github.com/repos/argoproj/argo-cd/issues/events{/number}",
				  "issues_url": "https://api.github.com/repos/argoproj/argo-cd/issues{/number}",
				  "keys_url": "https://api.github.com/repos/argoproj/argo-cd/keys{/key_id}",
				  "labels_url": "https://api.github.com/repos/argoproj/argo-cd/labels{/name}",
				  "languages_url": "https://api.github.com/repos/argoproj/argo-cd/languages",
				  "merges_url": "https://api.github.com/repos/argoproj/argo-cd/merges",
				  "milestones_url": "https://api.github.com/repos/argoproj/argo-cd/milestones{/number}",
				  "notifications_url": "https://api.github.com/repos/argoproj/argo-cd/notifications{?since,all,participating}",
				  "pulls_url": "https://api.github.com/repos/argoproj/argo-cd/pulls{/number}",
				  "releases_url": "https://api.github.com/repos/argoproj/argo-cd/releases{/id}",
				  "ssh_url": "git@github.com:argoproj/argo-cd.git",
				  "stargazers_url": "https://api.github.com/repos/argoproj/argo-cd/stargazers",
				  "statuses_url": "https://api.github.com/repos/argoproj/argo-cd/statuses/{sha}",
				  "subscribers_url": "https://api.github.com/repos/argoproj/argo-cd/subscribers",
				  "subscription_url": "https://api.github.com/repos/argoproj/argo-cd/subscription",
				  "tags_url": "https://api.github.com/repos/argoproj/argo-cd/tags",
				  "teams_url": "https://api.github.com/repos/argoproj/argo-cd/teams",
				  "trees_url": "https://api.github.com/repos/argoproj/argo-cd/git/trees{/sha}",
				  "clone_url": "https://github.com/argoproj/argo-cd.git",
				  "mirror_url": "git:git.example.com/argoproj/argo-cd",
				  "hooks_url": "https://api.github.com/repos/argoproj/argo-cd/hooks",
				  "svn_url": "https://svn.github.com/argoproj/argo-cd",
				  "homepage": "https://github.com",
				  "language": null,
				  "forks_count": 9,
				  "stargazers_count": 80,
				  "watchers_count": 80,
				  "size": 108,
				  "default_branch": "master",
				  "open_issues_count": 0,
				  "is_template": false,
				  "topics": [
					"argoproj",
					"atom",
					"electron",
					"api"
				  ],
				  "has_issues": true,
				  "has_projects": true,
				  "has_wiki": true,
				  "has_pages": false,
				  "has_downloads": true,
				  "archived": false,
				  "disabled": false,
				  "visibility": "public",
				  "pushed_at": "2011-01-26T19:06:43Z",
				  "created_at": "2011-01-26T19:01:12Z",
				  "updated_at": "2011-01-26T19:14:43Z",
				  "permissions": {
					"admin": false,
					"push": false,
					"pull": true
				  },
				  "template_repository": null
				}
			  ]`)
			if err != nil {
				t.Fail()
			}
		case "/api/v3/repos/argoproj/argo-cd/branches?per_page=100":
			_, err := io.WriteString(w, `[
				{
				  "name": "master",
				  "commit": {
					"sha": "c5b97d5ae6c19d5c5df71a34c7fbeeda2479ccbc",
					"url": "https://api.github.com/repos/argoproj/argo-cd/commits/c5b97d5ae6c19d5c5df71a34c7fbeeda2479ccbc"
				  },
				  "protected": true,
				  "protection": {
					"required_status_checks": {
					  "enforcement_level": "non_admins",
					  "contexts": [
						"ci-test",
						"linter"
					  ]
					}
				  },
				  "protection_url": "https://api.github.com/repos/argoproj/hello-world/branches/master/protection"
				}
			  ]
			`)
			if err != nil {
				t.Fail()
			}
		case "/api/v3/repos/argoproj/argo-cd/contents/pkg?ref=master":
			_, err := io.WriteString(w, `{
				"type": "file",
				"encoding": "base64",
				"size": 5362,
				"name": "pkg/",
				"path": "pkg/",
				"content": "encoded content ...",
				"sha": "3d21ec53a331a6f037a91c368710b99387d012c1",
				"url": "https://api.github.com/repos/octokit/octokit.rb/contents/README.md",
				"git_url": "https://api.github.com/repos/octokit/octokit.rb/git/blobs/3d21ec53a331a6f037a91c368710b99387d012c1",
				"html_url": "https://github.com/octokit/octokit.rb/blob/master/README.md",
				"download_url": "https://raw.githubusercontent.com/octokit/octokit.rb/master/README.md",
				"_links": {
				  "git": "https://api.github.com/repos/octokit/octokit.rb/git/blobs/3d21ec53a331a6f037a91c368710b99387d012c1",
				  "self": "https://api.github.com/repos/octokit/octokit.rb/contents/README.md",
				  "html": "https://github.com/octokit/octokit.rb/blob/master/README.md"
				}
			  }`)
			if err != nil {
				t.Fail()
			}
		case "/api/v3/repos/argoproj/argo-cd/branches/master":
			_, err := io.WriteString(w, `{
				"name": "master",
				"commit": {
				  "sha": "c5b97d5ae6c19d5c5df71a34c7fbeeda2479ccbc",
				  "url": "https://api.github.com/repos/octocat/Hello-World/commits/c5b97d5ae6c19d5c5df71a34c7fbeeda2479ccbc"
				},
				"protected": true,
				"protection": {
				  "required_status_checks": {
					"enforcement_level": "non_admins",
					"contexts": [
					  "ci-test",
					  "linter"
					]
				  }
				},
				"protection_url": "https://api.github.com/repos/octocat/hello-world/branches/master/protection"
			  }`)
			if err != nil {
				t.Fail()
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func testServerWithPort(t *testing.T, port int, handler http.Handler) *httptest.Server {
	// Use mocked API response to avoid rate-limiting.
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Error(fmt.Errorf("Unable to start server %w", err))
	}

	ts := httptest.NewUnstartedServer(handler)

	ts.Listener.Close()
	ts.Listener = l

	return ts
}

func TestSimpleSCMProviderGenerator(t *testing.T) {
	ts := testServerWithPort(t, 8341, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		githubSCMMockHandler(t)(w, r)
	}))
	ts.Start()
	defer ts.Close()

	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "argo-cd-guestbook",
			Namespace:  fixture.TestNamespace(),
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
				RepoURL:        "git@github.com:argoproj/argo-cd.git",
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
	repoMatch := "argo-cd"

	Given(t).
		// Create an SCMProviderGenerator-based ApplicationSet
		When().Create(v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "simple-scm-provider-generator",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{ repository }}-guestbook"},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: &argov1alpha1.ApplicationSource{
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
							API:          ts.URL,
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
	ts := testServerWithPort(t, 8342, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		githubSCMMockHandler(t)(w, r)
	}))
	ts.Start()
	defer ts.Close()

	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "argo-cd-guestbook",
			Namespace:  fixture.TestNamespace(),
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
				RepoURL:        "git@github.com:argoproj/argo-cd.git",
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
	repoMatch := "argo-cd"

	Given(t).
		// Create an SCMProviderGenerator-based ApplicationSet
		When().Create(v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "simple-scm-provider-generator",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			GoTemplate: true,
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{ .repository }}-guestbook"},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: &argov1alpha1.ApplicationSource{
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
							API:          ts.URL,
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

func TestSCMProviderGeneratorSCMProviderNotAllowed(t *testing.T) {
	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "argo-cd-guestbook",
			Namespace:  fixture.TestNamespace(),
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
				RepoURL:        "git@github.com:argoproj/argo-cd.git",
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
	repoMatch := "argo-cd"

	Given(t).
		// Create an SCMProviderGenerator-based ApplicationSet
		When().Create(v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "scm-provider-generator-scm-provider-not-allowed",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			GoTemplate: true,
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{ .repository }}-guestbook"},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: &argov1alpha1.ApplicationSource{
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
							API:          "http://myservice.mynamespace.svc.cluster.local",
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
	}).Then().Expect(ApplicationsDoNotExist([]argov1alpha1.Application{expectedApp})).
		And(func() {
			// app should be listed
			output, err := fixture.RunCli("appset", "get", "scm-provider-generator-scm-provider-not-allowed")
			require.NoError(t, err)
			assert.Contains(t, output, "scm provider not allowed")
		})
}

func TestCustomApplicationFinalizers(t *testing.T) {
	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "my-cluster-guestbook",
			Namespace:  fixture.TestNamespace(),
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io/background"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
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
		When().Create(v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
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
					Source: &argov1alpha1.ApplicationSource{
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
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "my-cluster-guestbook",
			Namespace:  fixture.TestNamespace(),
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io/background"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
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
		When().Create(v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
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
					Source: &argov1alpha1.ApplicationSource{
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

func githubPullMockHandler(t *testing.T) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/api/v3/repos/applicationset-test-org/argocd-example-apps/pulls?per_page=100":
			_, err := io.WriteString(w, `[
  {
    "number": 1,
    "title": "title1",
    "labels": [
      {
        "name": "preview"
      }
    ],
	"base": {
		"ref": "master",
		"sha": "7a4a5c987fdfb2b0629e9dbf5f31636c69ba4775"
	},
    "head": {
      "ref": "pull-request",
      "sha": "824a5c987fdfb2b0629e9dbf5f31636c69ba4772"
    },
	"user": {
	  "login": "testName"
	}
  }
]`)
			if err != nil {
				t.Fail()
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestSimplePullRequestGenerator(t *testing.T) {
	ts := testServerWithPort(t, 8343, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		githubPullMockHandler(t)(w, r)
	}))

	ts.Start()
	defer ts.Close()

	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "guestbook-1",
			Namespace:  fixture.TestNamespace(),
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
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
		When().Create(v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "simple-pull-request-generator",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "guestbook-{{ number }}"},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: &argov1alpha1.ApplicationSource{
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
							API:   ts.URL,
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
	ts := testServerWithPort(t, 8344, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		githubPullMockHandler(t)(w, r)
	}))

	ts.Start()
	defer ts.Close()

	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "guestbook-1",
			Namespace:  fixture.TestNamespace(),
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			Labels:     map[string]string{"app": "preview"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
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
		When().Create(v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "simple-pull-request-generator",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			GoTemplate: true,
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
					Name:   "guestbook-{{ .number }}",
					Labels: map[string]string{"app": "{{index .labels 0}}"},
				},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: &argov1alpha1.ApplicationSource{
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
							API:   ts.URL,
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

func TestPullRequestGeneratorNotAllowedSCMProvider(t *testing.T) {
	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "guestbook-1",
			Namespace:  fixture.TestNamespace(),
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			Labels: map[string]string{
				"app": "preview",
			},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
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
		When().Create(v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pull-request-generator-not-allowed-scm",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			GoTemplate: true,
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
					Name:   "guestbook-{{ .number }}",
					Labels: map[string]string{"app": "{{index .labels 0}}"},
				},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "default",
					Source: &argov1alpha1.ApplicationSource{
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
							API:   "http://myservice.mynamespace.svc.cluster.local",
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
	}).Then().Expect(ApplicationsDoNotExist([]argov1alpha1.Application{expectedApp})).
		And(func() {
			// app should be listed
			output, err := fixture.RunCli("appset", "get", "pull-request-generator-not-allowed-scm")
			require.NoError(t, err)
			assert.Contains(t, output, "scm provider not allowed")
		})
}

func TestGitGeneratorPrivateRepo(t *testing.T) {
	FailOnErr(fixture.RunCli("repo", "add", fixture.RepoURL(fixture.RepoURLTypeHTTPS), "--username", fixture.GitUsername, "--password", fixture.GitPassword, "--insecure-skip-server-verification"))
	generateExpectedApp := func(name string) argov1alpha1.Application {
		return argov1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
				Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			},
			Spec: argov1alpha1.ApplicationSpec{
				Project: "default",
				Source: &argov1alpha1.ApplicationSource{
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
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator-private",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{path.basename}}"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: &argov1alpha1.ApplicationSource{
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
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  fixture.TestNamespace(),
				Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			},
			Spec: argov1alpha1.ApplicationSpec{
				Project: "default",
				Source: &argov1alpha1.ApplicationSource{
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
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-git-generator-private",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				GoTemplate: true,
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{.path.basename}}"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: &argov1alpha1.ApplicationSource{
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
