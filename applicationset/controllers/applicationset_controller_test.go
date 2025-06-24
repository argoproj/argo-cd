package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	crtclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync/common"

	"github.com/argoproj/argo-cd/v3/applicationset/generators"
	"github.com/argoproj/argo-cd/v3/applicationset/generators/mocks"
	appsetmetrics "github.com/argoproj/argo-cd/v3/applicationset/metrics"
	"github.com/argoproj/argo-cd/v3/applicationset/utils"
	argocommon "github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	applog "github.com/argoproj/argo-cd/v3/util/app/log"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// getDefaultTestClientSet creates a Clientset with the default argo objects
// and objects specified in parameters
func getDefaultTestClientSet(obj ...runtime.Object) *kubefake.Clientset {
	argoCDSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      argocommon.ArgoCDSecretName,
			Namespace: "argocd",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string][]byte{
			"admin.password":   nil,
			"server.secretkey": nil,
		},
	}

	emptyArgoCDConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      argocommon.ArgoCDConfigMapName,
			Namespace: "argocd",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string]string{},
	}

	objects := append(obj, emptyArgoCDConfigMap, argoCDSecret)
	kubeclientset := kubefake.NewClientset(objects...)
	return kubeclientset
}

func TestCreateOrUpdateInCluster(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	for _, c := range []struct {
		// name is human-readable test name
		name string
		// appSet is the ApplicationSet we are generating resources for
		appSet v1alpha1.ApplicationSet
		// existingApps are the apps that already exist on the cluster
		existingApps []v1alpha1.Application
		// desiredApps are the generated apps to create/update
		desiredApps []v1alpha1.Application
		// expected is what we expect the cluster Applications to look like, after createOrUpdateInCluster
		expected []v1alpha1.Application
	}{
		{
			name: "Create an app that doesn't exist",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
			},
			existingApps: nil,
			desiredApps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app1",
						Namespace: "namespace",
					},
					Spec: v1alpha1.ApplicationSpec{Project: "default"},
				},
			},
			expected: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "1",
					},
					Spec: v1alpha1.ApplicationSpec{Project: "default"},
				},
			},
		},
		{
			name: "Update an existing app with a different project name",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			},
			existingApps: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "2",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "test",
					},
				},
			},
			desiredApps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app1",
						Namespace: "namespace",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			expected: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "3",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
		},
		{
			name: "Create a new app and check it doesn't replace the existing app",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			},
			existingApps: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "2",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "test",
					},
				},
			},
			desiredApps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app2",
						Namespace: "namespace",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			expected: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app2",
						Namespace:       "namespace",
						ResourceVersion: "1",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
		},
		{
			name: "Ensure that labels and annotations are added (via update) into an exiting application",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			},
			existingApps: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "2",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			desiredApps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "app1",
						Namespace:   "namespace",
						Labels:      map[string]string{"label-key": "label-value"},
						Annotations: map[string]string{"annot-key": "annot-value"},
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			expected: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						Labels:          map[string]string{"label-key": "label-value"},
						Annotations:     map[string]string{"annot-key": "annot-value"},
						ResourceVersion: "3",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
		},
		{
			name: "Ensure that labels and annotations are removed from an existing app",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			},
			existingApps: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "2",
						Labels:          map[string]string{"label-key": "label-value"},
						Annotations:     map[string]string{"annot-key": "annot-value"},
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			desiredApps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app1",
						Namespace: "namespace",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			expected: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "3",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
		},
		{
			name: "Ensure that status and operation fields are not overridden by an update, when removing labels/annotations",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			},
			existingApps: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "2",
						Labels:          map[string]string{"label-key": "label-value"},
						Annotations:     map[string]string{"annot-key": "annot-value"},
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
					Status: v1alpha1.ApplicationStatus{
						Resources: []v1alpha1.ResourceStatus{{Name: "sample-name"}},
					},
					Operation: &v1alpha1.Operation{
						Sync: &v1alpha1.SyncOperation{Revision: "sample-revision"},
					},
				},
			},
			desiredApps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app1",
						Namespace: "namespace",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			expected: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "3",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
					Status: v1alpha1.ApplicationStatus{
						Resources: []v1alpha1.ResourceStatus{{Name: "sample-name"}},
					},
					Operation: &v1alpha1.Operation{
						Sync: &v1alpha1.SyncOperation{Revision: "sample-revision"},
					},
				},
			},
		},
		{
			name: "Ensure that status and operation fields are not overridden by an update, when removing labels/annotations and adding other fields",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project:     "project",
							Source:      &v1alpha1.ApplicationSource{Path: "path", TargetRevision: "revision", RepoURL: "repoURL"},
							Destination: v1alpha1.ApplicationDestination{Server: "server", Namespace: "namespace"},
						},
					},
				},
			},
			existingApps: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "2",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
					Status: v1alpha1.ApplicationStatus{
						Resources: []v1alpha1.ResourceStatus{{Name: "sample-name"}},
					},
					Operation: &v1alpha1.Operation{
						Sync: &v1alpha1.SyncOperation{Revision: "sample-revision"},
					},
				},
			},
			desiredApps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "app1",
						Namespace:   "namespace",
						Labels:      map[string]string{"label-key": "label-value"},
						Annotations: map[string]string{"annot-key": "annot-value"},
					},
					Spec: v1alpha1.ApplicationSpec{
						Project:     "project",
						Source:      &v1alpha1.ApplicationSource{Path: "path", TargetRevision: "revision", RepoURL: "repoURL"},
						Destination: v1alpha1.ApplicationDestination{Server: "server", Namespace: "namespace"},
					},
				},
			},
			expected: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						Labels:          map[string]string{"label-key": "label-value"},
						Annotations:     map[string]string{"annot-key": "annot-value"},
						ResourceVersion: "3",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project:     "project",
						Source:      &v1alpha1.ApplicationSource{Path: "path", TargetRevision: "revision", RepoURL: "repoURL"},
						Destination: v1alpha1.ApplicationDestination{Server: "server", Namespace: "namespace"},
					},
					Status: v1alpha1.ApplicationStatus{
						Resources: []v1alpha1.ResourceStatus{{Name: "sample-name"}},
					},
					Operation: &v1alpha1.Operation{
						Sync: &v1alpha1.SyncOperation{Revision: "sample-revision"},
					},
				},
			},
		},
		{
			name: "Ensure that argocd notifications state and refresh annotation is preserved from an existing app",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			},
			existingApps: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "2",
						Labels:          map[string]string{"label-key": "label-value"},
						Annotations: map[string]string{
							"annot-key":                   "annot-value",
							NotifiedAnnotationKey:         `{"b620d4600c771a6f4cxxxxxxx:on-deployed:[0].y7b5sbwa2Q329JYHxxxxxx-fBs:slack:slack-test":1617144614}`,
							v1alpha1.AnnotationKeyRefresh: string(v1alpha1.RefreshTypeNormal),
						},
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			desiredApps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app1",
						Namespace: "namespace",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			expected: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "3",
						Annotations: map[string]string{
							NotifiedAnnotationKey:         `{"b620d4600c771a6f4cxxxxxxx:on-deployed:[0].y7b5sbwa2Q329JYHxxxxxx-fBs:slack:slack-test":1617144614}`,
							v1alpha1.AnnotationKeyRefresh: string(v1alpha1.RefreshTypeNormal),
						},
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
		},
		{
			name: "Ensure that configured preserved annotations are preserved from an existing app",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
					PreservedFields: &v1alpha1.ApplicationPreservedFields{
						Annotations: []string{"preserved-annot-key"},
					},
				},
			},
			existingApps: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Application",
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "2",
						Annotations: map[string]string{
							"annot-key":           "annot-value",
							"preserved-annot-key": "preserved-annot-value",
						},
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			desiredApps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app1",
						Namespace: "namespace",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			expected: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Application",
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "3",
						Annotations: map[string]string{
							"preserved-annot-key": "preserved-annot-value",
						},
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
		},
		{
			name: "Ensure that the app spec is normalized before applying",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "project",
							Source: &v1alpha1.ApplicationSource{
								Directory: &v1alpha1.ApplicationSourceDirectory{
									Jsonnet: v1alpha1.ApplicationSourceJsonnet{},
								},
							},
						},
					},
				},
			},
			desiredApps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app1",
						Namespace: "namespace",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
						Source: &v1alpha1.ApplicationSource{
							Directory: &v1alpha1.ApplicationSourceDirectory{
								Jsonnet: v1alpha1.ApplicationSourceJsonnet{},
							},
						},
					},
				},
			},
			expected: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Application",
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "1",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
						Source:  &v1alpha1.ApplicationSource{
							// Directory and jsonnet block are removed
						},
					},
				},
			},
		},
		{
			// For this use case: https://github.com/argoproj/argo-cd/issues/9101#issuecomment-1191138278
			name: "Ensure that ignored targetRevision difference doesn't cause an update, even if another field changes",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					IgnoreApplicationDifferences: v1alpha1.ApplicationSetIgnoreDifferences{
						{JQPathExpressions: []string{".spec.source.targetRevision"}},
					},
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "project",
							Source: &v1alpha1.ApplicationSource{
								RepoURL:        "https://git.example.com/test-org/test-repo.git",
								TargetRevision: "foo",
							},
						},
					},
				},
			},
			existingApps: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Application",
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "2",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://git.example.com/test-org/test-repo.git",
							TargetRevision: "bar",
						},
					},
				},
			},
			desiredApps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app1",
						Namespace: "namespace",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
						Source: &v1alpha1.ApplicationSource{
							RepoURL: "https://git.example.com/test-org/test-repo.git",
							// The targetRevision is ignored, so this should not be updated.
							TargetRevision: "foo",
							// This should be updated.
							Helm: &v1alpha1.ApplicationSourceHelm{
								Parameters: []v1alpha1.HelmParameter{
									{Name: "hi", Value: "there"},
								},
							},
						},
					},
				},
			},
			expected: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Application",
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "3",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
						Source: &v1alpha1.ApplicationSource{
							RepoURL: "https://git.example.com/test-org/test-repo.git",
							// This is the existing value from the cluster, which should not be updated because the field is ignored.
							TargetRevision: "bar",
							// This was missing on the cluster, so it should be added.
							Helm: &v1alpha1.ApplicationSourceHelm{
								Parameters: []v1alpha1.HelmParameter{
									{Name: "hi", Value: "there"},
								},
							},
						},
					},
				},
			},
		},
		{
			// For this use case: https://github.com/argoproj/argo-cd/pull/14743#issuecomment-1761954799
			name: "ignore parameters added to a multi-source app in the cluster",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					IgnoreApplicationDifferences: v1alpha1.ApplicationSetIgnoreDifferences{
						{JQPathExpressions: []string{`.spec.sources[] | select(.repoURL | contains("test-repo")).helm.parameters`}},
					},
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "project",
							Sources: []v1alpha1.ApplicationSource{
								{
									RepoURL: "https://git.example.com/test-org/test-repo.git",
									Helm: &v1alpha1.ApplicationSourceHelm{
										Values: "foo: bar",
									},
								},
							},
						},
					},
				},
			},
			existingApps: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Application",
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "2",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
						Sources: []v1alpha1.ApplicationSource{
							{
								RepoURL: "https://git.example.com/test-org/test-repo.git",
								Helm: &v1alpha1.ApplicationSourceHelm{
									Values: "foo: bar",
									Parameters: []v1alpha1.HelmParameter{
										{Name: "hi", Value: "there"},
									},
								},
							},
						},
					},
				},
			},
			desiredApps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app1",
						Namespace: "namespace",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
						Sources: []v1alpha1.ApplicationSource{
							{
								RepoURL: "https://git.example.com/test-org/test-repo.git",
								Helm: &v1alpha1.ApplicationSourceHelm{
									Values: "foo: bar",
								},
							},
						},
					},
				},
			},
			expected: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Application",
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app1",
						Namespace: "namespace",
						// This should not be updated, because reconciliation shouldn't modify the App.
						ResourceVersion: "2",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
						Sources: []v1alpha1.ApplicationSource{
							{
								RepoURL: "https://git.example.com/test-org/test-repo.git",
								Helm: &v1alpha1.ApplicationSourceHelm{
									Values: "foo: bar",
									Parameters: []v1alpha1.HelmParameter{
										// This existed only in the cluster, but it shouldn't be removed, because the field is ignored.
										{Name: "hi", Value: "there"},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Demonstrate limitation of MergePatch", // Maybe we can fix this in Argo CD 3.0: https://github.com/argoproj/argo-cd/issues/15975
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					IgnoreApplicationDifferences: v1alpha1.ApplicationSetIgnoreDifferences{
						{JQPathExpressions: []string{`.spec.sources[] | select(.repoURL | contains("test-repo")).helm.parameters`}},
					},
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "project",
							Sources: []v1alpha1.ApplicationSource{
								{
									RepoURL: "https://git.example.com/test-org/test-repo.git",
									Helm: &v1alpha1.ApplicationSourceHelm{
										Values: "new: values",
									},
								},
							},
						},
					},
				},
			},
			existingApps: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Application",
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "2",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
						Sources: []v1alpha1.ApplicationSource{
							{
								RepoURL: "https://git.example.com/test-org/test-repo.git",
								Helm: &v1alpha1.ApplicationSourceHelm{
									Values: "foo: bar",
									Parameters: []v1alpha1.HelmParameter{
										{Name: "hi", Value: "there"},
									},
								},
							},
						},
					},
				},
			},
			desiredApps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app1",
						Namespace: "namespace",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
						Sources: []v1alpha1.ApplicationSource{
							{
								RepoURL: "https://git.example.com/test-org/test-repo.git",
								Helm: &v1alpha1.ApplicationSourceHelm{
									Values: "new: values",
								},
							},
						},
					},
				},
			},
			expected: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Application",
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "3",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
						Sources: []v1alpha1.ApplicationSource{
							{
								RepoURL: "https://git.example.com/test-org/test-repo.git",
								Helm: &v1alpha1.ApplicationSourceHelm{
									Values: "new: values",
									// The Parameters field got blown away, because the values field changed. MergePatch
									// doesn't merge list items, it replaces the whole list if an item changes.
									// If we eventually add a `name` field to Sources, we can use StrategicMergePatch.
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Ensure that argocd post-delete finalizers are preserved from an existing app",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			},
			existingApps: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "2",
						Finalizers: []string{
							v1alpha1.PostDeleteFinalizerName,
							v1alpha1.PostDeleteFinalizerName + "/mystage",
						},
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			desiredApps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app1",
						Namespace: "namespace",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			expected: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "2",
						Finalizers: []string{
							v1alpha1.PostDeleteFinalizerName,
							v1alpha1.PostDeleteFinalizerName + "/mystage",
						},
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			initObjs := []crtclient.Object{&c.appSet}

			for _, a := range c.existingApps {
				err = controllerutil.SetControllerReference(&c.appSet, &a, scheme)
				require.NoError(t, err)
				initObjs = append(initObjs, &a)
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics()

			r := ApplicationSetReconciler{
				Client:   client,
				Scheme:   scheme,
				Recorder: record.NewFakeRecorder(len(initObjs) + len(c.expected)),
				Metrics:  metrics,
			}

			err = r.createOrUpdateInCluster(t.Context(), log.NewEntry(log.StandardLogger()), c.appSet, c.desiredApps)
			require.NoError(t, err)

			for _, obj := range c.expected {
				got := &v1alpha1.Application{}
				_ = client.Get(t.Context(), crtclient.ObjectKey{
					Namespace: obj.Namespace,
					Name:      obj.Name,
				}, got)

				err = controllerutil.SetControllerReference(&c.appSet, &obj, r.Scheme)
				assert.Equal(t, obj, *got)
			}
		})
	}
}

func TestRemoveFinalizerOnInvalidDestination_FinalizerTypes(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	for _, c := range []struct {
		// name is human-readable test name
		name               string
		existingFinalizers []string
		expectedFinalizers []string
	}{
		{
			name:               "no finalizers",
			existingFinalizers: []string{},
			expectedFinalizers: nil,
		},
		{
			name:               "contains only argo finalizer",
			existingFinalizers: []string{v1alpha1.ResourcesFinalizerName},
			expectedFinalizers: nil,
		},
		{
			name:               "contains only non-argo finalizer",
			existingFinalizers: []string{"non-argo-finalizer"},
			expectedFinalizers: []string{"non-argo-finalizer"},
		},
		{
			name:               "contains both argo and non-argo finalizer",
			existingFinalizers: []string{"non-argo-finalizer", v1alpha1.ResourcesFinalizerName},
			expectedFinalizers: []string{"non-argo-finalizer"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			appSet := v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			}

			app := v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "app1",
					Finalizers: c.existingFinalizers,
				},
				Spec: v1alpha1.ApplicationSpec{
					Project: "project",
					Source:  &v1alpha1.ApplicationSource{Path: "path", TargetRevision: "revision", RepoURL: "repoURL"},
					// Destination is always invalid, for this test:
					Destination: v1alpha1.ApplicationDestination{Name: "my-cluster", Namespace: "namespace"},
				},
			}

			initObjs := []crtclient.Object{&app, &appSet}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: "namespace",
					Labels: map[string]string{
						argocommon.LabelKeySecretType: argocommon.LabelValueSecretTypeCluster,
					},
				},
				Data: map[string][]byte{
					// Since this test requires the cluster to be an invalid destination, we
					// always return a cluster named 'my-cluster2' (different from app 'my-cluster', above)
					"name":   []byte("mycluster2"),
					"server": []byte("https://kubernetes.default.svc"),
					"config": []byte("{\"username\":\"foo\",\"password\":\"foo\"}"),
				},
			}

			objects := append([]runtime.Object{}, secret)
			kubeclientset := kubefake.NewSimpleClientset(objects...)
			metrics := appsetmetrics.NewFakeAppsetMetrics()

			argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)

			r := ApplicationSetReconciler{
				Client:        client,
				Scheme:        scheme,
				Recorder:      record.NewFakeRecorder(10),
				KubeClientset: kubeclientset,
				Metrics:       metrics,
				ArgoDB:        argodb,
			}
			clusterList, err := utils.ListClusters(t.Context(), kubeclientset, "namespace")
			require.NoError(t, err)

			appLog := log.WithFields(applog.GetAppLogFields(&app)).WithField("appSet", "")

			appInputParam := app.DeepCopy()

			err = r.removeFinalizerOnInvalidDestination(t.Context(), appSet, appInputParam, clusterList, appLog)
			require.NoError(t, err)

			retrievedApp := v1alpha1.Application{}
			err = client.Get(t.Context(), crtclient.ObjectKeyFromObject(&app), &retrievedApp)
			require.NoError(t, err)

			// App on the cluster should have the expected finalizers
			assert.ElementsMatch(t, c.expectedFinalizers, retrievedApp.Finalizers)

			// App object passed in as a parameter should have the expected finaliers
			assert.ElementsMatch(t, c.expectedFinalizers, appInputParam.Finalizers)

			bytes, _ := json.MarshalIndent(retrievedApp, "", "  ")
			t.Log("Contents of app after call:", string(bytes))
		})
	}
}

func TestRemoveFinalizerOnInvalidDestination_DestinationTypes(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	for _, c := range []struct {
		// name is human-readable test name
		name                   string
		destinationField       v1alpha1.ApplicationDestination
		expectFinalizerRemoved bool
	}{
		{
			name: "invalid cluster: empty destination",
			destinationField: v1alpha1.ApplicationDestination{
				Namespace: "namespace",
			},
			expectFinalizerRemoved: true,
		},
		{
			name: "invalid cluster: invalid server url",
			destinationField: v1alpha1.ApplicationDestination{
				Namespace: "namespace",
				Server:    "https://1.2.3.4",
			},
			expectFinalizerRemoved: true,
		},
		{
			name: "invalid cluster: invalid cluster name",
			destinationField: v1alpha1.ApplicationDestination{
				Namespace: "namespace",
				Name:      "invalid-cluster",
			},
			expectFinalizerRemoved: true,
		},
		{
			name: "invalid cluster by both valid",
			destinationField: v1alpha1.ApplicationDestination{
				Namespace: "namespace",
				Name:      "mycluster2",
				Server:    "https://kubernetes.default.svc",
			},
			expectFinalizerRemoved: true,
		},
		{
			name: "invalid cluster by both invalid",
			destinationField: v1alpha1.ApplicationDestination{
				Namespace: "namespace",
				Name:      "mycluster3",
				Server:    "https://4.5.6.7",
			},
			expectFinalizerRemoved: true,
		},
		{
			name: "valid cluster by name",
			destinationField: v1alpha1.ApplicationDestination{
				Namespace: "namespace",
				Name:      "mycluster2",
			},
			expectFinalizerRemoved: false,
		},
		{
			name: "valid cluster by server",
			destinationField: v1alpha1.ApplicationDestination{
				Namespace: "namespace",
				Server:    "https://kubernetes.default.svc",
			},
			expectFinalizerRemoved: false,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			appSet := v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			}

			app := v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "app1",
					Finalizers: []string{v1alpha1.ResourcesFinalizerName},
				},
				Spec: v1alpha1.ApplicationSpec{
					Project:     "project",
					Source:      &v1alpha1.ApplicationSource{Path: "path", TargetRevision: "revision", RepoURL: "repoURL"},
					Destination: c.destinationField,
				},
			}

			initObjs := []crtclient.Object{&app, &appSet}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: "argocd",
					Labels: map[string]string{
						argocommon.LabelKeySecretType: argocommon.LabelValueSecretTypeCluster,
					},
				},
				Data: map[string][]byte{
					// Since this test requires the cluster to be an invalid destination, we
					// always return a cluster named 'my-cluster2' (different from app 'my-cluster', above)
					"name":   []byte("mycluster2"),
					"server": []byte("https://kubernetes.default.svc"),
					"config": []byte("{\"username\":\"foo\",\"password\":\"foo\"}"),
				},
			}

			kubeclientset := getDefaultTestClientSet(secret)
			metrics := appsetmetrics.NewFakeAppsetMetrics()

			argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)

			r := ApplicationSetReconciler{
				Client:        client,
				Scheme:        scheme,
				Recorder:      record.NewFakeRecorder(10),
				KubeClientset: kubeclientset,
				Metrics:       metrics,
				ArgoDB:        argodb,
			}

			clusterList, err := utils.ListClusters(t.Context(), kubeclientset, "argocd")
			require.NoError(t, err)

			appLog := log.WithFields(applog.GetAppLogFields(&app)).WithField("appSet", "")

			appInputParam := app.DeepCopy()

			err = r.removeFinalizerOnInvalidDestination(t.Context(), appSet, appInputParam, clusterList, appLog)
			require.NoError(t, err)

			retrievedApp := v1alpha1.Application{}
			err = client.Get(t.Context(), crtclient.ObjectKeyFromObject(&app), &retrievedApp)
			require.NoError(t, err)

			finalizerRemoved := len(retrievedApp.Finalizers) == 0

			assert.Equal(t, c.expectFinalizerRemoved, finalizerRemoved)

			bytes, _ := json.MarshalIndent(retrievedApp, "", "  ")
			t.Log("Contents of app after call:", string(bytes))
		})
	}
}

func TestRemoveOwnerReferencesOnDeleteAppSet(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	for _, c := range []struct {
		// name is human-readable test name
		name string
	}{
		{
			name: "ownerReferences cleared",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			appSet := v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "name",
					Namespace:  "namespace",
					Finalizers: []string{v1alpha1.ResourcesFinalizerName},
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			}

			app := v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app1",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSpec{
					Project: "project",
					Source:  &v1alpha1.ApplicationSource{Path: "path", TargetRevision: "revision", RepoURL: "repoURL"},
					Destination: v1alpha1.ApplicationDestination{
						Namespace: "namespace",
						Server:    "https://kubernetes.default.svc",
					},
				},
			}

			err := controllerutil.SetControllerReference(&appSet, &app, scheme)
			require.NoError(t, err)

			initObjs := []crtclient.Object{&app, &appSet}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics()

			r := ApplicationSetReconciler{
				Client:        client,
				Scheme:        scheme,
				Recorder:      record.NewFakeRecorder(10),
				KubeClientset: nil,
				Metrics:       metrics,
			}

			err = r.removeOwnerReferencesOnDeleteAppSet(t.Context(), appSet)
			require.NoError(t, err)

			retrievedApp := v1alpha1.Application{}
			err = client.Get(t.Context(), crtclient.ObjectKeyFromObject(&app), &retrievedApp)
			require.NoError(t, err)

			ownerReferencesRemoved := len(retrievedApp.OwnerReferences) == 0
			assert.True(t, ownerReferencesRemoved)
		})
	}
}

func TestCreateApplications(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	testCases := []struct {
		name       string
		appSet     v1alpha1.ApplicationSet
		existsApps []v1alpha1.Application
		apps       []v1alpha1.Application
		expected   []v1alpha1.Application
	}{
		{
			name: "no existing apps",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
			},
			existsApps: nil,
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app1",
						Namespace: "namespace",
					},
				},
			},
			expected: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "1",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
					},
				},
			},
		},
		{
			name: "existing apps",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			},
			existsApps: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "2",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "test",
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app1",
						Namespace: "namespace",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			expected: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "2",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "test",
					},
				},
			},
		},
		{
			name: "existing apps with different project",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			},
			existsApps: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "2",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "test",
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app2",
						Namespace: "namespace",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			expected: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app2",
						Namespace:       "namespace",
						ResourceVersion: "1",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
		},
	}

	for _, c := range testCases {
		t.Run(c.name, func(t *testing.T) {
			initObjs := []crtclient.Object{&c.appSet}
			for _, a := range c.existsApps {
				err = controllerutil.SetControllerReference(&c.appSet, &a, scheme)
				require.NoError(t, err)
				initObjs = append(initObjs, &a)
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics()

			r := ApplicationSetReconciler{
				Client:   client,
				Scheme:   scheme,
				Recorder: record.NewFakeRecorder(len(initObjs) + len(c.expected)),
				Metrics:  metrics,
			}

			err = r.createInCluster(t.Context(), log.NewEntry(log.StandardLogger()), c.appSet, c.apps)
			require.NoError(t, err)

			for _, obj := range c.expected {
				got := &v1alpha1.Application{}
				_ = client.Get(t.Context(), crtclient.ObjectKey{
					Namespace: obj.Namespace,
					Name:      obj.Name,
				}, got)

				err = controllerutil.SetControllerReference(&c.appSet, &obj, r.Scheme)
				require.NoError(t, err)

				assert.Equal(t, obj, *got)
			}
		})
	}
}

func TestDeleteInCluster(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	for _, c := range []struct {
		// appSet is the application set on which the delete function is called
		appSet v1alpha1.ApplicationSet
		// existingApps is the current state of Applications on the cluster
		existingApps []v1alpha1.Application
		// desireApps is the apps generated by the generator that we wish to keep alive
		desiredApps []v1alpha1.Application
		// expected is the list of applications that we expect to exist after calling delete
		expected []v1alpha1.Application
		// notExpected is the list of applications that we expect not to exist after calling delete
		notExpected []v1alpha1.Application
	}{
		{
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			},
			existingApps: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "delete",
						Namespace:       "namespace",
						ResourceVersion: "2",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "keep",
						Namespace:       "namespace",
						ResourceVersion: "2",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			desiredApps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "keep",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			expected: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "keep",
						Namespace:       "namespace",
						ResourceVersion: "2",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			notExpected: []v1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "delete",
						Namespace:       "namespace",
						ResourceVersion: "1",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
		},
	} {
		initObjs := []crtclient.Object{&c.appSet}
		for _, a := range c.existingApps {
			temp := a
			err = controllerutil.SetControllerReference(&c.appSet, &temp, scheme)
			require.NoError(t, err)
			initObjs = append(initObjs, &temp)
		}

		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
		metrics := appsetmetrics.NewFakeAppsetMetrics()

		r := ApplicationSetReconciler{
			Client:        client,
			Scheme:        scheme,
			Recorder:      record.NewFakeRecorder(len(initObjs) + len(c.expected)),
			KubeClientset: kubefake.NewSimpleClientset(),
			Metrics:       metrics,
		}

		err = r.deleteInCluster(t.Context(), log.NewEntry(log.StandardLogger()), c.appSet, c.desiredApps)
		require.NoError(t, err)

		// For each of the expected objects, verify they exist on the cluster
		for _, obj := range c.expected {
			got := &v1alpha1.Application{}
			_ = client.Get(t.Context(), crtclient.ObjectKey{
				Namespace: obj.Namespace,
				Name:      obj.Name,
			}, got)

			err = controllerutil.SetControllerReference(&c.appSet, &obj, r.Scheme)
			require.NoError(t, err)

			assert.Equal(t, obj, *got)
		}

		// Verify each of the unexpected objs cannot be found
		for _, obj := range c.notExpected {
			got := &v1alpha1.Application{}
			err := client.Get(t.Context(), crtclient.ObjectKey{
				Namespace: obj.Namespace,
				Name:      obj.Name,
			}, got)

			assert.EqualError(t, err, fmt.Sprintf("applications.argoproj.io %q not found", obj.Name))
		}
	}
}

func TestGetMinRequeueAfter(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	metrics := appsetmetrics.NewFakeAppsetMetrics()

	generator := v1alpha1.ApplicationSetGenerator{
		List:     &v1alpha1.ListGenerator{},
		Git:      &v1alpha1.GitGenerator{},
		Clusters: &v1alpha1.ClusterGenerator{},
	}

	generatorMock0 := mocks.Generator{}
	generatorMock0.On("GetRequeueAfter", &generator).
		Return(generators.NoRequeueAfter)

	generatorMock1 := mocks.Generator{}
	generatorMock1.On("GetRequeueAfter", &generator).
		Return(time.Duration(1) * time.Second)

	generatorMock10 := mocks.Generator{}
	generatorMock10.On("GetRequeueAfter", &generator).
		Return(time.Duration(10) * time.Second)

	r := ApplicationSetReconciler{
		Client:   client,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(0),
		Metrics:  metrics,
		Generators: map[string]generators.Generator{
			"List":     &generatorMock10,
			"Git":      &generatorMock1,
			"Clusters": &generatorMock1,
		},
	}

	got := r.getMinRequeueAfter(&v1alpha1.ApplicationSet{
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{generator},
		},
	})

	assert.Equal(t, time.Duration(1)*time.Second, got)
}

func TestRequeueGeneratorFails(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)
	err = v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	appSet := v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "argocd",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{{
				PullRequest: &v1alpha1.PullRequestGenerator{},
			}},
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet).Build()

	generator := v1alpha1.ApplicationSetGenerator{
		PullRequest: &v1alpha1.PullRequestGenerator{},
	}

	generatorMock := mocks.Generator{}
	generatorMock.On("GetTemplate", &generator).
		Return(&v1alpha1.ApplicationSetTemplate{})
	generatorMock.On("GenerateParams", &generator, mock.AnythingOfType("*v1alpha1.ApplicationSet"), mock.Anything).
		Return([]map[string]any{}, errors.New("Simulated error generating params that could be related to an external service/API call"))

	metrics := appsetmetrics.NewFakeAppsetMetrics()

	r := ApplicationSetReconciler{
		Client:   client,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(0),
		Generators: map[string]generators.Generator{
			"PullRequest": &generatorMock,
		},
		Metrics: metrics,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "argocd",
			Name:      "name",
		},
	}

	res, err := r.Reconcile(t.Context(), req)
	require.NoError(t, err)
	assert.Equal(t, ReconcileRequeueOnValidationError, res.RequeueAfter)
}

func TestValidateGeneratedApplications(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	// Valid project
	myProject := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "namespace"},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos: []string{"*"},
			Destinations: []v1alpha1.ApplicationDestination{
				{
					Namespace: "*",
					Server:    "*",
				},
			},
			ClusterResourceWhitelist: []metav1.GroupKind{
				{
					Group: "*",
					Kind:  "*",
				},
			},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(myProject).Build()
	metrics := appsetmetrics.NewFakeAppsetMetrics()

	// Test a subset of the validations that 'validateGeneratedApplications' performs
	for _, cc := range []struct {
		name             string
		apps             []v1alpha1.Application
		validationErrors map[int]error
	}{
		{
			name: "valid app should return true",
			apps: []v1alpha1.Application{
				{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://url",
							Path:           "/",
							TargetRevision: "HEAD",
						},
						Destination: v1alpha1.ApplicationDestination{
							Namespace: "namespace",
							Name:      "my-cluster",
						},
					},
				},
			},
			validationErrors: map[int]error{},
		},
		{
			name: "can't have both name and server defined",
			apps: []v1alpha1.Application{
				{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://url",
							Path:           "/",
							TargetRevision: "HEAD",
						},
						Destination: v1alpha1.ApplicationDestination{
							Namespace: "namespace",
							Server:    "my-server",
							Name:      "my-cluster",
						},
					},
				},
			},
			validationErrors: map[int]error{0: errors.New("application destination spec is invalid: application destination can't have both name and server defined: my-cluster my-server")},
		},
		{
			name: "project mismatch should return error",
			apps: []v1alpha1.Application{
				{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: v1alpha1.ApplicationSpec{
						Project: "DOES-NOT-EXIST",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://url",
							Path:           "/",
							TargetRevision: "HEAD",
						},
						Destination: v1alpha1.ApplicationDestination{
							Namespace: "namespace",
							Name:      "my-cluster",
						},
					},
				},
			},
			validationErrors: map[int]error{0: errors.New("application references project DOES-NOT-EXIST which does not exist")},
		},
		{
			name: "valid app should return true",
			apps: []v1alpha1.Application{
				{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://url",
							Path:           "/",
							TargetRevision: "HEAD",
						},
						Destination: v1alpha1.ApplicationDestination{
							Namespace: "namespace",
							Name:      "my-cluster",
						},
					},
				},
			},
			validationErrors: map[int]error{},
		},
		{
			name: "cluster should match",
			apps: []v1alpha1.Application{
				{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://url",
							Path:           "/",
							TargetRevision: "HEAD",
						},
						Destination: v1alpha1.ApplicationDestination{
							Namespace: "namespace",
							Name:      "nonexistent-cluster",
						},
					},
				},
			},
			validationErrors: map[int]error{0: errors.New("application destination spec is invalid: there are no clusters with this name: nonexistent-cluster")},
		},
	} {
		t.Run(cc.name, func(t *testing.T) {
			t.Parallel()

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: "argocd",
					Labels: map[string]string{
						argocommon.LabelKeySecretType: argocommon.LabelValueSecretTypeCluster,
					},
				},
				Data: map[string][]byte{
					"name":   []byte("my-cluster"),
					"server": []byte("https://kubernetes.default.svc"),
					"config": []byte("{\"username\":\"foo\",\"password\":\"foo\"}"),
				},
			}

			kubeclientset := getDefaultTestClientSet(secret)

			argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)

			r := ApplicationSetReconciler{
				Client:          client,
				Scheme:          scheme,
				Recorder:        record.NewFakeRecorder(1),
				Generators:      map[string]generators.Generator{},
				ArgoDB:          argodb,
				ArgoCDNamespace: "namespace",
				KubeClientset:   kubeclientset,
				Metrics:         metrics,
			}

			appSetInfo := v1alpha1.ApplicationSet{}
			validationErrors, _ := r.validateGeneratedApplications(t.Context(), cc.apps, appSetInfo)
			assert.Equal(t, cc.validationErrors, validationErrors)
		})
	}
}

func TestReconcilerValidationProjectErrorBehaviour(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	project := v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "good-project", Namespace: "argocd"},
	}
	appSet := v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "argocd",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			GoTemplate: true,
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					List: &v1alpha1.ListGenerator{
						Elements: []apiextensionsv1.JSON{{
							Raw: []byte(`{"project": "good-project"}`),
						}, {
							Raw: []byte(`{"project": "bad-project"}`),
						}},
					},
				},
			},
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
					Name:      "{{.project}}",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSpec{
					Source:      &v1alpha1.ApplicationSource{RepoURL: "https://github.com/argoproj/argocd-example-apps", Path: "guestbook"},
					Project:     "{{.project}}",
					Destination: v1alpha1.ApplicationDestination{Server: "https://kubernetes.default.svc"},
				},
			},
		},
	}

	kubeclientset := getDefaultTestClientSet()

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet, &project).WithStatusSubresource(&appSet).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
	metrics := appsetmetrics.NewFakeAppsetMetrics()

	argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)

	r := ApplicationSetReconciler{
		Client:   client,
		Scheme:   scheme,
		Renderer: &utils.Render{},
		Recorder: record.NewFakeRecorder(1),
		Generators: map[string]generators.Generator{
			"List": generators.NewListGenerator(),
		},
		ArgoDB:          argodb,
		KubeClientset:   kubeclientset,
		Policy:          v1alpha1.ApplicationsSyncPolicySync,
		ArgoCDNamespace: "argocd",
		Metrics:         metrics,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "argocd",
			Name:      "name",
		},
	}

	// Verify that on validation error, no error is returned, but the object is requeued
	res, err := r.Reconcile(t.Context(), req)
	require.NoError(t, err)
	assert.Equal(t, ReconcileRequeueOnValidationError, res.RequeueAfter)

	var app v1alpha1.Application

	// make sure good app got created
	err = r.Get(t.Context(), crtclient.ObjectKey{Namespace: "argocd", Name: "good-project"}, &app)
	require.NoError(t, err)
	assert.Equal(t, "good-project", app.Name)

	// make sure bad app was not created
	err = r.Get(t.Context(), crtclient.ObjectKey{Namespace: "argocd", Name: "bad-project"}, &app)
	require.Error(t, err)
}

func TestSetApplicationSetStatusCondition(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	testCases := []struct {
		appset     v1alpha1.ApplicationSet
		conditions []v1alpha1.ApplicationSetCondition
		testfunc   func(t *testing.T, appset v1alpha1.ApplicationSet)
	}{
		{
			appset: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Generators: []v1alpha1.ApplicationSetGenerator{
						{List: &v1alpha1.ListGenerator{
							Elements: []apiextensionsv1.JSON{{
								Raw: []byte(`{"cluster": "my-cluster","url": "https://kubernetes.default.svc"}`),
							}},
						}},
					},
					Template: v1alpha1.ApplicationSetTemplate{},
				},
			},
			conditions: []v1alpha1.ApplicationSetCondition{
				{
					Type:    v1alpha1.ApplicationSetConditionResourcesUpToDate,
					Message: "All applications have been generated successfully",
					Reason:  v1alpha1.ApplicationSetReasonApplicationSetUpToDate,
					Status:  v1alpha1.ApplicationSetConditionStatusTrue,
				},
			},
			testfunc: func(t *testing.T, appset v1alpha1.ApplicationSet) {
				t.Helper()
				assert.Len(t, appset.Status.Conditions, 3)
			},
		},
		{
			appset: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Generators: []v1alpha1.ApplicationSetGenerator{
						{List: &v1alpha1.ListGenerator{
							Elements: []apiextensionsv1.JSON{{
								Raw: []byte(`{"cluster": "my-cluster","url": "https://kubernetes.default.svc"}`),
							}},
						}},
					},
					Template: v1alpha1.ApplicationSetTemplate{},
				},
			},
			conditions: []v1alpha1.ApplicationSetCondition{
				{
					Type:    v1alpha1.ApplicationSetConditionResourcesUpToDate,
					Message: "All applications have been generated successfully",
					Reason:  v1alpha1.ApplicationSetReasonApplicationSetUpToDate,
					Status:  v1alpha1.ApplicationSetConditionStatusTrue,
				},
				{
					Type:    v1alpha1.ApplicationSetConditionRolloutProgressing,
					Message: "ApplicationSet Rollout Rollout started",
					Reason:  v1alpha1.ApplicationSetReasonApplicationSetUpToDate,
					Status:  v1alpha1.ApplicationSetConditionStatusTrue,
				},
			},
			testfunc: func(t *testing.T, appset v1alpha1.ApplicationSet) {
				t.Helper()
				assert.Len(t, appset.Status.Conditions, 3)

				isProgressingCondition := false

				for _, condition := range appset.Status.Conditions {
					if condition.Type == v1alpha1.ApplicationSetConditionRolloutProgressing {
						isProgressingCondition = true
						break
					}
				}

				assert.False(t, isProgressingCondition, "no RolloutProgressing should be set for applicationsets that don't have rolling strategy")
			},
		},
		{
			appset: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Generators: []v1alpha1.ApplicationSetGenerator{
						{List: &v1alpha1.ListGenerator{
							Elements: []apiextensionsv1.JSON{{
								Raw: []byte(`{"cluster": "my-cluster","url": "https://kubernetes.default.svc"}`),
							}},
						}},
					},
					Template: v1alpha1.ApplicationSetTemplate{},
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "test",
											Operator: "In",
											Values:   []string{"test"},
										},
									},
								},
							},
						},
					},
				},
			},
			conditions: []v1alpha1.ApplicationSetCondition{
				{
					Type:    v1alpha1.ApplicationSetConditionResourcesUpToDate,
					Message: "All applications have been generated successfully",
					Reason:  v1alpha1.ApplicationSetReasonApplicationSetUpToDate,
					Status:  v1alpha1.ApplicationSetConditionStatusTrue,
				},
				{
					Type:    v1alpha1.ApplicationSetConditionRolloutProgressing,
					Message: "ApplicationSet Rollout Rollout started",
					Reason:  v1alpha1.ApplicationSetReasonApplicationSetUpToDate,
					Status:  v1alpha1.ApplicationSetConditionStatusTrue,
				},
			},
			testfunc: func(t *testing.T, appset v1alpha1.ApplicationSet) {
				t.Helper()
				assert.Len(t, appset.Status.Conditions, 4)

				isProgressingCondition := false

				for _, condition := range appset.Status.Conditions {
					if condition.Type == v1alpha1.ApplicationSetConditionRolloutProgressing {
						isProgressingCondition = true
						break
					}
				}

				assert.True(t, isProgressingCondition, "RolloutProgressing should be set for rollout strategy appset")
			},
		},
	}

	kubeclientset := kubefake.NewSimpleClientset([]runtime.Object{}...)

	for _, testCase := range testCases {
		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&testCase.appset).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).WithStatusSubresource(&testCase.appset).Build()
		metrics := appsetmetrics.NewFakeAppsetMetrics()

		argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)

		r := ApplicationSetReconciler{
			Client:   client,
			Scheme:   scheme,
			Renderer: &utils.Render{},
			Recorder: record.NewFakeRecorder(1),
			Generators: map[string]generators.Generator{
				"List": generators.NewListGenerator(),
			},
			ArgoDB:        argodb,
			KubeClientset: kubeclientset,
			Metrics:       metrics,
		}

		for _, condition := range testCase.conditions {
			err = r.setApplicationSetStatusCondition(t.Context(), &testCase.appset, condition, true)
			require.NoError(t, err)
		}

		testCase.testfunc(t, testCase.appset)
	}
}

func applicationsUpdateSyncPolicyTest(t *testing.T, applicationsSyncPolicy v1alpha1.ApplicationsSyncPolicy, recordBuffer int, allowPolicyOverride bool) v1alpha1.Application {
	t.Helper()
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	defaultProject := v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "argocd"},
		Spec:       v1alpha1.AppProjectSpec{SourceRepos: []string{"*"}, Destinations: []v1alpha1.ApplicationDestination{{Namespace: "*", Server: "https://good-cluster"}}},
	}
	appSet := v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "argocd",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					List: &v1alpha1.ListGenerator{
						Elements: []apiextensionsv1.JSON{{
							Raw: []byte(`{"cluster": "good-cluster","url": "https://good-cluster"}`),
						}},
					},
				},
			},
			SyncPolicy: &v1alpha1.ApplicationSetSyncPolicy{
				ApplicationsSync: &applicationsSyncPolicy,
			},
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
					Name:      "{{cluster}}",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSpec{
					Source:      &v1alpha1.ApplicationSource{RepoURL: "https://github.com/argoproj/argocd-example-apps", Path: "guestbook"},
					Project:     "default",
					Destination: v1alpha1.ApplicationDestination{Server: "{{url}}"},
				},
			},
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster",
			Namespace: "argocd",
			Labels: map[string]string{
				argocommon.LabelKeySecretType: argocommon.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			// Since this test requires the cluster to be an invalid destination, we
			// always return a cluster named 'my-cluster2' (different from app 'my-cluster', above)
			"name":   []byte("good-cluster"),
			"server": []byte("https://good-cluster"),
			"config": []byte("{\"username\":\"foo\",\"password\":\"foo\"}"),
		},
	}

	kubeclientset := getDefaultTestClientSet(secret)

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet, &defaultProject).WithStatusSubresource(&appSet).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
	metrics := appsetmetrics.NewFakeAppsetMetrics()

	argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)

	r := ApplicationSetReconciler{
		Client:   client,
		Scheme:   scheme,
		Renderer: &utils.Render{},
		Recorder: record.NewFakeRecorder(recordBuffer),
		Generators: map[string]generators.Generator{
			"List": generators.NewListGenerator(),
		},
		ArgoDB:               argodb,
		ArgoCDNamespace:      "argocd",
		KubeClientset:        kubeclientset,
		Policy:               v1alpha1.ApplicationsSyncPolicySync,
		EnablePolicyOverride: allowPolicyOverride,
		Metrics:              metrics,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "argocd",
			Name:      "name",
		},
	}

	// Verify that on validation error, no error is returned, but the object is requeued
	resCreate, err := r.Reconcile(t.Context(), req)
	require.NoErrorf(t, err, "Reconcile failed with error: %v", err)
	assert.Equal(t, time.Duration(0), resCreate.RequeueAfter)

	var app v1alpha1.Application

	// make sure good app got created
	err = r.Get(t.Context(), crtclient.ObjectKey{Namespace: "argocd", Name: "good-cluster"}, &app)
	require.NoError(t, err)
	assert.Equal(t, "good-cluster", app.Name)

	// Update resource
	var retrievedApplicationSet v1alpha1.ApplicationSet
	err = r.Get(t.Context(), crtclient.ObjectKey{Namespace: "argocd", Name: "name"}, &retrievedApplicationSet)
	require.NoError(t, err)

	retrievedApplicationSet.Spec.Template.Annotations = map[string]string{"annotation-key": "annotation-value"}
	retrievedApplicationSet.Spec.Template.Labels = map[string]string{"label-key": "label-value"}

	retrievedApplicationSet.Spec.Template.Spec.Source.Helm = &v1alpha1.ApplicationSourceHelm{
		Values: "global.test: test",
	}

	err = r.Update(t.Context(), &retrievedApplicationSet)
	require.NoError(t, err)

	resUpdate, err := r.Reconcile(t.Context(), req)
	require.NoError(t, err)

	err = r.Get(t.Context(), crtclient.ObjectKey{Namespace: "argocd", Name: "good-cluster"}, &app)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), resUpdate.RequeueAfter)
	assert.Equal(t, "good-cluster", app.Name)

	return app
}

func TestUpdateNotPerformedWithSyncPolicyCreateOnly(t *testing.T) {
	applicationsSyncPolicy := v1alpha1.ApplicationsSyncPolicyCreateOnly

	app := applicationsUpdateSyncPolicyTest(t, applicationsSyncPolicy, 1, true)

	assert.Nil(t, app.Spec.Source.Helm)
	assert.Nil(t, app.Annotations)
}

func TestUpdateNotPerformedWithSyncPolicyCreateDelete(t *testing.T) {
	applicationsSyncPolicy := v1alpha1.ApplicationsSyncPolicyCreateDelete

	app := applicationsUpdateSyncPolicyTest(t, applicationsSyncPolicy, 1, true)

	assert.Nil(t, app.Spec.Source.Helm)
	assert.Nil(t, app.Annotations)
}

func TestUpdatePerformedWithSyncPolicyCreateUpdate(t *testing.T) {
	applicationsSyncPolicy := v1alpha1.ApplicationsSyncPolicyCreateUpdate

	app := applicationsUpdateSyncPolicyTest(t, applicationsSyncPolicy, 2, true)

	assert.Equal(t, "global.test: test", app.Spec.Source.Helm.Values)
	assert.Equal(t, map[string]string{"annotation-key": "annotation-value"}, app.Annotations)
	assert.Equal(t, map[string]string{"label-key": "label-value"}, app.Labels)
}

func TestUpdatePerformedWithSyncPolicySync(t *testing.T) {
	applicationsSyncPolicy := v1alpha1.ApplicationsSyncPolicySync

	app := applicationsUpdateSyncPolicyTest(t, applicationsSyncPolicy, 2, true)

	assert.Equal(t, "global.test: test", app.Spec.Source.Helm.Values)
	assert.Equal(t, map[string]string{"annotation-key": "annotation-value"}, app.Annotations)
	assert.Equal(t, map[string]string{"label-key": "label-value"}, app.Labels)
}

func TestUpdatePerformedWithSyncPolicyCreateOnlyAndAllowPolicyOverrideFalse(t *testing.T) {
	applicationsSyncPolicy := v1alpha1.ApplicationsSyncPolicyCreateOnly

	app := applicationsUpdateSyncPolicyTest(t, applicationsSyncPolicy, 2, false)

	assert.Equal(t, "global.test: test", app.Spec.Source.Helm.Values)
	assert.Equal(t, map[string]string{"annotation-key": "annotation-value"}, app.Annotations)
	assert.Equal(t, map[string]string{"label-key": "label-value"}, app.Labels)
}

func applicationsDeleteSyncPolicyTest(t *testing.T, applicationsSyncPolicy v1alpha1.ApplicationsSyncPolicy, recordBuffer int, allowPolicyOverride bool) v1alpha1.ApplicationList {
	t.Helper()
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	defaultProject := v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "argocd"},
		Spec:       v1alpha1.AppProjectSpec{SourceRepos: []string{"*"}, Destinations: []v1alpha1.ApplicationDestination{{Namespace: "*", Server: "https://good-cluster"}}},
	}
	appSet := v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "argocd",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					List: &v1alpha1.ListGenerator{
						Elements: []apiextensionsv1.JSON{{
							Raw: []byte(`{"cluster": "good-cluster","url": "https://good-cluster"}`),
						}},
					},
				},
			},
			SyncPolicy: &v1alpha1.ApplicationSetSyncPolicy{
				ApplicationsSync: &applicationsSyncPolicy,
			},
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
					Name:      "{{cluster}}",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSpec{
					Source:      &v1alpha1.ApplicationSource{RepoURL: "https://github.com/argoproj/argocd-example-apps", Path: "guestbook"},
					Project:     "default",
					Destination: v1alpha1.ApplicationDestination{Server: "{{url}}"},
				},
			},
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster",
			Namespace: "argocd",
			Labels: map[string]string{
				argocommon.LabelKeySecretType: argocommon.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			// Since this test requires the cluster to be an invalid destination, we
			// always return a cluster named 'my-cluster2' (different from app 'my-cluster', above)
			"name":   []byte("good-cluster"),
			"server": []byte("https://good-cluster"),
			"config": []byte("{\"username\":\"foo\",\"password\":\"foo\"}"),
		},
	}

	kubeclientset := getDefaultTestClientSet(secret)

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet, &defaultProject).WithStatusSubresource(&appSet).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
	metrics := appsetmetrics.NewFakeAppsetMetrics()

	argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)

	r := ApplicationSetReconciler{
		Client:   client,
		Scheme:   scheme,
		Renderer: &utils.Render{},
		Recorder: record.NewFakeRecorder(recordBuffer),
		Generators: map[string]generators.Generator{
			"List": generators.NewListGenerator(),
		},
		ArgoDB:               argodb,
		ArgoCDNamespace:      "argocd",
		KubeClientset:        kubeclientset,
		Policy:               v1alpha1.ApplicationsSyncPolicySync,
		EnablePolicyOverride: allowPolicyOverride,
		Metrics:              metrics,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "argocd",
			Name:      "name",
		},
	}

	// Verify that on validation error, no error is returned, but the object is requeued
	resCreate, err := r.Reconcile(t.Context(), req)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), resCreate.RequeueAfter)

	var app v1alpha1.Application

	// make sure good app got created
	err = r.Get(t.Context(), crtclient.ObjectKey{Namespace: "argocd", Name: "good-cluster"}, &app)
	require.NoError(t, err)
	assert.Equal(t, "good-cluster", app.Name)

	// Update resource
	var retrievedApplicationSet v1alpha1.ApplicationSet
	err = r.Get(t.Context(), crtclient.ObjectKey{Namespace: "argocd", Name: "name"}, &retrievedApplicationSet)
	require.NoError(t, err)
	retrievedApplicationSet.Spec.Generators = []v1alpha1.ApplicationSetGenerator{
		{
			List: &v1alpha1.ListGenerator{
				Elements: []apiextensionsv1.JSON{},
			},
		},
	}

	err = r.Update(t.Context(), &retrievedApplicationSet)
	require.NoError(t, err)

	resUpdate, err := r.Reconcile(t.Context(), req)
	require.NoError(t, err)

	var apps v1alpha1.ApplicationList

	err = r.List(t.Context(), &apps)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), resUpdate.RequeueAfter)

	return apps
}

func TestDeleteNotPerformedWithSyncPolicyCreateOnly(t *testing.T) {
	applicationsSyncPolicy := v1alpha1.ApplicationsSyncPolicyCreateOnly

	apps := applicationsDeleteSyncPolicyTest(t, applicationsSyncPolicy, 1, true)

	assert.Equal(t, "good-cluster", apps.Items[0].Name)
}

func TestDeleteNotPerformedWithSyncPolicyCreateUpdate(t *testing.T) {
	applicationsSyncPolicy := v1alpha1.ApplicationsSyncPolicyCreateUpdate

	apps := applicationsDeleteSyncPolicyTest(t, applicationsSyncPolicy, 2, true)

	assert.Equal(t, "good-cluster", apps.Items[0].Name)
}

func TestDeletePerformedWithSyncPolicyCreateDelete(t *testing.T) {
	applicationsSyncPolicy := v1alpha1.ApplicationsSyncPolicyCreateDelete

	apps := applicationsDeleteSyncPolicyTest(t, applicationsSyncPolicy, 3, true)

	assert.NotNil(t, apps.Items[0].DeletionTimestamp)
}

func TestDeletePerformedWithSyncPolicySync(t *testing.T) {
	applicationsSyncPolicy := v1alpha1.ApplicationsSyncPolicySync

	apps := applicationsDeleteSyncPolicyTest(t, applicationsSyncPolicy, 3, true)

	assert.NotNil(t, apps.Items[0].DeletionTimestamp)
}

func TestDeletePerformedWithSyncPolicyCreateOnlyAndAllowPolicyOverrideFalse(t *testing.T) {
	applicationsSyncPolicy := v1alpha1.ApplicationsSyncPolicyCreateOnly

	apps := applicationsDeleteSyncPolicyTest(t, applicationsSyncPolicy, 3, false)

	assert.NotNil(t, apps.Items[0].DeletionTimestamp)
}

func TestPolicies(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	defaultProject := v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "argocd"},
		Spec:       v1alpha1.AppProjectSpec{SourceRepos: []string{"*"}, Destinations: []v1alpha1.ApplicationDestination{{Namespace: "*", Server: "https://kubernetes.default.svc"}}},
	}

	kubeclientset := getDefaultTestClientSet()

	for _, c := range []struct {
		name          string
		policyName    string
		allowedUpdate bool
		allowedDelete bool
	}{
		{
			name:          "Apps are allowed to update and delete",
			policyName:    "sync",
			allowedUpdate: true,
			allowedDelete: true,
		},
		{
			name:          "Apps are not allowed to update and delete",
			policyName:    "create-only",
			allowedUpdate: false,
			allowedDelete: false,
		},
		{
			name:          "Apps are allowed to update, not allowed to delete",
			policyName:    "create-update",
			allowedUpdate: true,
			allowedDelete: false,
		},
		{
			name:          "Apps are allowed to delete, not allowed to update",
			policyName:    "create-delete",
			allowedUpdate: false,
			allowedDelete: true,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			policy := utils.Policies[c.policyName]
			assert.NotNil(t, policy)

			appSet := v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					GoTemplate: true,
					Generators: []v1alpha1.ApplicationSetGenerator{
						{
							List: &v1alpha1.ListGenerator{
								Elements: []apiextensionsv1.JSON{
									{
										Raw: []byte(`{"name": "my-app"}`),
									},
								},
							},
						},
					},
					Template: v1alpha1.ApplicationSetTemplate{
						ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
							Name:      "{{.name}}",
							Namespace: "argocd",
							Annotations: map[string]string{
								"key": "value",
							},
						},
						Spec: v1alpha1.ApplicationSpec{
							Source:      &v1alpha1.ApplicationSource{RepoURL: "https://github.com/argoproj/argocd-example-apps", Path: "guestbook"},
							Project:     "default",
							Destination: v1alpha1.ApplicationDestination{Server: "https://kubernetes.default.svc"},
						},
					},
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet, &defaultProject).WithStatusSubresource(&appSet).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics()

			argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)

			r := ApplicationSetReconciler{
				Client:   client,
				Scheme:   scheme,
				Renderer: &utils.Render{},
				Recorder: record.NewFakeRecorder(10),
				Generators: map[string]generators.Generator{
					"List": generators.NewListGenerator(),
				},
				ArgoDB:          argodb,
				ArgoCDNamespace: "argocd",
				KubeClientset:   kubeclientset,
				Policy:          policy,
				Metrics:         metrics,
			}

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "argocd",
					Name:      "name",
				},
			}

			// Check if Application is created
			res, err := r.Reconcile(t.Context(), req)
			require.NoError(t, err)
			assert.Equal(t, time.Duration(0), res.RequeueAfter)

			var app v1alpha1.Application
			err = r.Get(t.Context(), crtclient.ObjectKey{Namespace: "argocd", Name: "my-app"}, &app)
			require.NoError(t, err)
			assert.Equal(t, "value", app.Annotations["key"])

			// Check if Application is updated
			app.Annotations["key"] = "edited"
			err = r.Update(t.Context(), &app)
			require.NoError(t, err)

			res, err = r.Reconcile(t.Context(), req)
			require.NoError(t, err)
			assert.Equal(t, time.Duration(0), res.RequeueAfter)

			err = r.Get(t.Context(), crtclient.ObjectKey{Namespace: "argocd", Name: "my-app"}, &app)
			require.NoError(t, err)

			if c.allowedUpdate {
				assert.Equal(t, "value", app.Annotations["key"])
			} else {
				assert.Equal(t, "edited", app.Annotations["key"])
			}

			// Check if Application is deleted
			err = r.Get(t.Context(), crtclient.ObjectKey{Namespace: "argocd", Name: "name"}, &appSet)
			require.NoError(t, err)
			appSet.Spec.Generators[0] = v1alpha1.ApplicationSetGenerator{
				List: &v1alpha1.ListGenerator{
					Elements: []apiextensionsv1.JSON{},
				},
			}
			err = r.Update(t.Context(), &appSet)
			require.NoError(t, err)

			res, err = r.Reconcile(t.Context(), req)
			require.NoError(t, err)
			assert.Equal(t, time.Duration(0), res.RequeueAfter)

			err = r.Get(t.Context(), crtclient.ObjectKey{Namespace: "argocd", Name: "my-app"}, &app)
			require.NoError(t, err)
			if c.allowedDelete {
				assert.NotNil(t, app.DeletionTimestamp)
			} else {
				assert.Nil(t, app.DeletionTimestamp)
			}
		})
	}
}

func TestSetApplicationSetApplicationStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	kubeclientset := kubefake.NewSimpleClientset([]runtime.Object{}...)

	for _, cc := range []struct {
		name                string
		appSet              v1alpha1.ApplicationSet
		appStatuses         []v1alpha1.ApplicationSetApplicationStatus
		expectedAppStatuses []v1alpha1.ApplicationSetApplicationStatus
	}{
		{
			name: "sets a single appstatus",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Generators: []v1alpha1.ApplicationSetGenerator{
						{List: &v1alpha1.ListGenerator{
							Elements: []apiextensionsv1.JSON{{
								Raw: []byte(`{"cluster": "my-cluster","url": "https://kubernetes.default.svc"}`),
							}},
						}},
					},
					Template: v1alpha1.ApplicationSetTemplate{},
				},
			},
			appStatuses: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application: "app1",
					Message:     "testing SetApplicationSetApplicationStatus to Healthy",
					Status:      "Healthy",
				},
			},
			expectedAppStatuses: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application: "app1",
					Message:     "testing SetApplicationSetApplicationStatus to Healthy",
					Status:      "Healthy",
				},
			},
		},
		{
			name: "removes an appstatus",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Generators: []v1alpha1.ApplicationSetGenerator{
						{List: &v1alpha1.ListGenerator{
							Elements: []apiextensionsv1.JSON{{
								Raw: []byte(`{"cluster": "my-cluster","url": "https://kubernetes.default.svc"}`),
							}},
						}},
					},
					Template: v1alpha1.ApplicationSetTemplate{},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Message:     "testing SetApplicationSetApplicationStatus to Healthy",
							Status:      "Healthy",
						},
					},
				},
			},
			appStatuses:         []v1alpha1.ApplicationSetApplicationStatus{},
			expectedAppStatuses: nil,
		},
	} {
		t.Run(cc.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&cc.appSet).WithStatusSubresource(&cc.appSet).Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics()

			argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)

			r := ApplicationSetReconciler{
				Client:   client,
				Scheme:   scheme,
				Renderer: &utils.Render{},
				Recorder: record.NewFakeRecorder(1),
				Generators: map[string]generators.Generator{
					"List": generators.NewListGenerator(),
				},
				ArgoDB:        argodb,
				KubeClientset: kubeclientset,
				Metrics:       metrics,
			}

			err = r.setAppSetApplicationStatus(t.Context(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.appStatuses)
			require.NoError(t, err)

			assert.Equal(t, cc.expectedAppStatuses, cc.appSet.Status.ApplicationStatus)
		})
	}
}

func TestBuildAppDependencyList(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	metrics := appsetmetrics.NewFakeAppsetMetrics()

	for _, cc := range []struct {
		name            string
		appSet          v1alpha1.ApplicationSet
		apps            []v1alpha1.Application
		expectedList    [][]string
		expectedStepMap map[string]int
	}{
		{
			name: "handles an empty set of applications and no strategy",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{},
			},
			apps:            []v1alpha1.Application{},
			expectedList:    [][]string{},
			expectedStepMap: map[string]int{},
		},
		{
			name: "handles an empty set of applications and ignores AllAtOnce strategy",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "AllAtOnce",
					},
				},
			},
			apps:            []v1alpha1.Application{},
			expectedList:    [][]string{},
			expectedStepMap: map[string]int{},
		},
		{
			name: "handles an empty set of applications with good 'In' selectors",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "In",
											Values: []string{
												"dev",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{},
			expectedList: [][]string{
				{},
			},
			expectedStepMap: map[string]int{},
		},
		{
			name: "handles selecting 1 application with 1 'In' selector",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "In",
											Values: []string{
												"dev",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-dev",
						Labels: map[string]string{
							"env": "dev",
						},
					},
				},
			},
			expectedList: [][]string{
				{"app-dev"},
			},
			expectedStepMap: map[string]int{
				"app-dev": 0,
			},
		},
		{
			name: "handles 'In' selectors that select no applications",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "In",
											Values: []string{
												"dev",
											},
										},
									},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "In",
											Values: []string{
												"qa",
											},
										},
									},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "In",
											Values: []string{
												"prod",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa",
						Labels: map[string]string{
							"env": "qa",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-prod",
						Labels: map[string]string{
							"env": "prod",
						},
					},
				},
			},
			expectedList: [][]string{
				{},
				{"app-qa"},
				{"app-prod"},
			},
			expectedStepMap: map[string]int{
				"app-qa":   1,
				"app-prod": 2,
			},
		},
		{
			name: "multiple 'In' selectors in the same matchExpression only select Applications that match all selectors",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "region",
											Operator: "In",
											Values: []string{
												"us-east-2",
											},
										},
										{
											Key:      "env",
											Operator: "In",
											Values: []string{
												"qa",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa1",
						Labels: map[string]string{
							"env": "qa",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa2",
						Labels: map[string]string{
							"env":    "qa",
							"region": "us-east-2",
						},
					},
				},
			},
			expectedList: [][]string{
				{"app-qa2"},
			},
			expectedStepMap: map[string]int{
				"app-qa2": 0,
			},
		},
		{
			name: "multiple values in the same 'In' matchExpression can match on any value",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "In",
											Values: []string{
												"qa",
												"prod",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-dev",
						Labels: map[string]string{
							"env": "dev",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa",
						Labels: map[string]string{
							"env": "qa",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-prod",
						Labels: map[string]string{
							"env":    "prod",
							"region": "us-east-2",
						},
					},
				},
			},
			expectedList: [][]string{
				{"app-qa", "app-prod"},
			},
			expectedStepMap: map[string]int{
				"app-qa":   0,
				"app-prod": 0,
			},
		},
		{
			name: "handles an empty set of applications with good 'NotIn' selectors",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "In",
											Values: []string{
												"dev",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{},
			expectedList: [][]string{
				{},
			},
			expectedStepMap: map[string]int{},
		},
		{
			name: "selects 1 application with 1 'NotIn' selector",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "NotIn",
											Values: []string{
												"qa",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-dev",
						Labels: map[string]string{
							"env": "dev",
						},
					},
				},
			},
			expectedList: [][]string{
				{"app-dev"},
			},
			expectedStepMap: map[string]int{
				"app-dev": 0,
			},
		},
		{
			name: "'NotIn' selectors that select no applications",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "NotIn",
											Values: []string{
												"dev",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa",
						Labels: map[string]string{
							"env": "qa",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-prod",
						Labels: map[string]string{
							"env": "prod",
						},
					},
				},
			},
			expectedList: [][]string{
				{"app-qa", "app-prod"},
			},
			expectedStepMap: map[string]int{
				"app-qa":   0,
				"app-prod": 0,
			},
		},
		{
			name: "multiple 'NotIn' selectors remove Applications with mising labels on any match",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "region",
											Operator: "NotIn",
											Values: []string{
												"us-east-2",
											},
										},
										{
											Key:      "env",
											Operator: "NotIn",
											Values: []string{
												"qa",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa1",
						Labels: map[string]string{
							"env": "qa",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa2",
						Labels: map[string]string{
							"env":    "qa",
							"region": "us-east-2",
						},
					},
				},
			},
			expectedList: [][]string{
				{},
			},
			expectedStepMap: map[string]int{},
		},
		{
			name: "multiple 'NotIn' selectors filter all matching Applications",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "region",
											Operator: "NotIn",
											Values: []string{
												"us-east-2",
											},
										},
										{
											Key:      "env",
											Operator: "NotIn",
											Values: []string{
												"qa",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa1",
						Labels: map[string]string{
							"env":    "qa",
							"region": "us-east-1",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa2",
						Labels: map[string]string{
							"env":    "qa",
							"region": "us-east-2",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-prod1",
						Labels: map[string]string{
							"env":    "prod",
							"region": "us-east-1",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-prod2",
						Labels: map[string]string{
							"env":    "prod",
							"region": "us-east-2",
						},
					},
				},
			},
			expectedList: [][]string{
				{"app-prod1"},
			},
			expectedStepMap: map[string]int{
				"app-prod1": 0,
			},
		},
		{
			name: "multiple values in the same 'NotIn' matchExpression exclude a match from any value",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "NotIn",
											Values: []string{
												"qa",
												"prod",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-dev",
						Labels: map[string]string{
							"env": "dev",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa",
						Labels: map[string]string{
							"env": "qa",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-prod",
						Labels: map[string]string{
							"env":    "prod",
							"region": "us-east-2",
						},
					},
				},
			},
			expectedList: [][]string{
				{"app-dev"},
			},
			expectedStepMap: map[string]int{
				"app-dev": 0,
			},
		},
		{
			name: "in a mix of 'In' and 'NotIn' selectors, 'NotIn' takes precedence",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "In",
											Values: []string{
												"qa",
												"prod",
											},
										},
										{
											Key:      "region",
											Operator: "NotIn",
											Values: []string{
												"us-west-2",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-dev",
						Labels: map[string]string{
							"env": "dev",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa1",
						Labels: map[string]string{
							"env":    "qa",
							"region": "us-west-2",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa2",
						Labels: map[string]string{
							"env":    "qa",
							"region": "us-east-2",
						},
					},
				},
			},
			expectedList: [][]string{
				{"app-qa2"},
			},
			expectedStepMap: map[string]int{
				"app-qa2": 0,
			},
		},
	} {
		t.Run(cc.name, func(t *testing.T) {
			kubeclientset := kubefake.NewSimpleClientset([]runtime.Object{}...)

			argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)

			r := ApplicationSetReconciler{
				Client:        client,
				Scheme:        scheme,
				Recorder:      record.NewFakeRecorder(1),
				Generators:    map[string]generators.Generator{},
				ArgoDB:        argodb,
				KubeClientset: kubeclientset,
				Metrics:       metrics,
			}

			appDependencyList, appStepMap := r.buildAppDependencyList(log.NewEntry(log.StandardLogger()), cc.appSet, cc.apps)
			assert.Equal(t, cc.expectedList, appDependencyList, "expected appDependencyList did not match actual")
			assert.Equal(t, cc.expectedStepMap, appStepMap, "expected appStepMap did not match actual")
		})
	}
}

func TestBuildAppSyncMap(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	metrics := appsetmetrics.NewFakeAppsetMetrics()

	for _, cc := range []struct {
		name              string
		appSet            v1alpha1.ApplicationSet
		appMap            map[string]v1alpha1.Application
		appDependencyList [][]string
		expectedMap       map[string]bool
	}{
		{
			name: "handles an empty app dependency list",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
			},
			appDependencyList: [][]string{},
			expectedMap:       map[string]bool{},
		},
		{
			name: "handles two applications with no statuses",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
			},
			appDependencyList: [][]string{
				{"app1"},
				{"app2"},
			},
			expectedMap: map[string]bool{
				"app1": true,
				"app2": false,
			},
		},
		{
			name: "handles applications after an empty selection",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
			},
			appDependencyList: [][]string{
				{},
				{"app1", "app2"},
			},
			expectedMap: map[string]bool{
				"app1": true,
				"app2": true,
			},
		},
		{
			name: "handles RollingSync applications that are healthy and have no changes",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Status:      "Healthy",
						},
						{
							Application: "app2",
							Status:      "Healthy",
						},
					},
				},
			},
			appMap: map[string]v1alpha1.Application{
				"app1": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				"app2": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app2",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			appDependencyList: [][]string{
				{"app1"},
				{"app2"},
			},
			expectedMap: map[string]bool{
				"app1": true,
				"app2": true,
			},
		},
		{
			name: "blocks RollingSync applications that are healthy and have no changes, but are still pending",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Status:      "Pending",
						},
						{
							Application: "app2",
							Status:      "Healthy",
						},
					},
				},
			},
			appMap: map[string]v1alpha1.Application{
				"app1": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				"app2": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app2",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			appDependencyList: [][]string{
				{"app1"},
				{"app2"},
			},
			expectedMap: map[string]bool{
				"app1": true,
				"app2": false,
			},
		},
		{
			name: "handles RollingSync applications that are up to date and healthy, but still syncing",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Status:      "Progressing",
						},
						{
							Application: "app2",
							Status:      "Progressing",
						},
					},
				},
			},
			appMap: map[string]v1alpha1.Application{
				"app1": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationRunning,
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				"app2": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app2",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationRunning,
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			appDependencyList: [][]string{
				{"app1"},
				{"app2"},
			},
			expectedMap: map[string]bool{
				"app1": true,
				"app2": false,
			},
		},
		{
			name: "handles RollingSync applications that are up to date and synced, but degraded",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Status:      "Progressing",
						},
						{
							Application: "app2",
							Status:      "Progressing",
						},
					},
				},
			},
			appMap: map[string]v1alpha1.Application{
				"app1": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusDegraded,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationRunning,
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				"app2": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app2",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusDegraded,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationRunning,
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			appDependencyList: [][]string{
				{"app1"},
				{"app2"},
			},
			expectedMap: map[string]bool{
				"app1": true,
				"app2": false,
			},
		},
		{
			name: "handles RollingSync applications that are OutOfSync and healthy",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Status:      "Healthy",
						},
						{
							Application: "app2",
							Status:      "Healthy",
						},
					},
				},
			},
			appDependencyList: [][]string{
				{"app1"},
				{"app2"},
			},
			appMap: map[string]v1alpha1.Application{
				"app1": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				"app2": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app2",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
			expectedMap: map[string]bool{
				"app1": true,
				"app2": false,
			},
		},
		{
			name: "handles a lot of applications",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Status:      "Healthy",
						},
						{
							Application: "app2",
							Status:      "Healthy",
						},
						{
							Application: "app3",
							Status:      "Healthy",
						},
						{
							Application: "app4",
							Status:      "Healthy",
						},
						{
							Application: "app5",
							Status:      "Healthy",
						},
						{
							Application: "app7",
							Status:      "Healthy",
						},
					},
				},
			},
			appMap: map[string]v1alpha1.Application{
				"app1": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				"app2": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app2",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				"app3": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app3",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				"app5": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app5",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				"app6": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app6",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusDegraded,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			appDependencyList: [][]string{
				{"app1", "app2", "app3"},
				{"app4", "app5", "app6"},
				{"app7", "app8", "app9"},
			},
			expectedMap: map[string]bool{
				"app1": true,
				"app2": true,
				"app3": true,
				"app4": true,
				"app5": true,
				"app6": true,
				"app7": false,
				"app8": false,
				"app9": false,
			},
		},
	} {
		t.Run(cc.name, func(t *testing.T) {
			kubeclientset := kubefake.NewSimpleClientset([]runtime.Object{}...)

			argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)

			r := ApplicationSetReconciler{
				Client:        client,
				Scheme:        scheme,
				Recorder:      record.NewFakeRecorder(1),
				Generators:    map[string]generators.Generator{},
				ArgoDB:        argodb,
				KubeClientset: kubeclientset,
				Metrics:       metrics,
			}

			appSyncMap := r.buildAppSyncMap(cc.appSet, cc.appDependencyList, cc.appMap)
			assert.Equal(t, cc.expectedMap, appSyncMap, "expected appSyncMap did not match actual")
		})
	}
}

func TestUpdateApplicationSetApplicationStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	for _, cc := range []struct {
		name              string
		appSet            v1alpha1.ApplicationSet
		apps              []v1alpha1.Application
		appStepMap        map[string]int
		expectedAppStatus []v1alpha1.ApplicationSetApplicationStatus
	}{
		{
			name: "handles a nil list of statuses and no applications",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
			},
			apps:              []v1alpha1.Application{},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{},
		},
		{
			name: "handles a nil list of statuses with a healthy application",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource is already Healthy, updating status from Waiting to Healthy.",
					Status:          "Healthy",
					Step:            "1",
					TargetRevisions: []string{},
				},
			},
		},
		{
			name: "handles an empty list of statuses with a healthy application",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource is already Healthy, updating status from Waiting to Healthy.",
					Status:          "Healthy",
					Step:            "1",
					TargetRevisions: []string{},
				},
			},
		},
		{
			name: "handles an outdated list of statuses with a healthy application, setting required variables",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Message:     "Application resource is already Healthy, updating status from Waiting to Healthy.",
							Status:      "Healthy",
							Step:        "1",
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource is already Healthy, updating status from Waiting to Healthy.",
					Status:          "Healthy",
					Step:            "1",
					TargetRevisions: []string{},
				},
			},
		},
		{
			name: "progresses an OutOfSync RollingSync application to waiting",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application:     "app1",
							Message:         "",
							Status:          "Healthy",
							Step:            "1",
							TargetRevisions: []string{"Previous"},
						},
						{
							Application:     "app2-multisource",
							Message:         "",
							Status:          "Healthy",
							Step:            "1",
							TargetRevisions: []string{"Previous", "OtherPrevious"},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Sync: v1alpha1.SyncStatus{
							Status:   v1alpha1.SyncStatusCodeOutOfSync,
							Revision: "Next",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app2-multisource",
					},
					Status: v1alpha1.ApplicationStatus{
						Sync: v1alpha1.SyncStatus{
							Status:    v1alpha1.SyncStatusCodeOutOfSync,
							Revisions: []string{"Next", "OtherNext"},
						},
					},
				},
			},
			appStepMap: map[string]int{
				"app1":             0,
				"app2-multisource": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application has pending changes, setting status to Waiting.",
					Status:          "Waiting",
					Step:            "1",
					TargetRevisions: []string{"Next"},
				},
				{
					Application:     "app2-multisource",
					Message:         "Application has pending changes, setting status to Waiting.",
					Status:          "Waiting",
					Step:            "1",
					TargetRevisions: []string{"Next", "OtherNext"},
				},
			},
		},
		{
			name: "progresses a pending progressing application to progressing",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application:     "app1",
							Message:         "",
							Status:          "Pending",
							Step:            "1",
							TargetRevisions: []string{"Next"},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusProgressing,
						},
						Sync: v1alpha1.SyncStatus{
							Revision: "Next",
						},
					},
				},
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource became Progressing, updating status from Pending to Progressing.",
					Status:          "Progressing",
					Step:            "1",
					TargetRevisions: []string{"Next"},
				},
			},
		},
		{
			name: "progresses a pending synced application to progressing",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application:     "app1",
							Message:         "",
							Status:          "Pending",
							Step:            "1",
							TargetRevisions: []string{"Current"},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationRunning,
						},
						Sync: v1alpha1.SyncStatus{
							Status:   v1alpha1.SyncStatusCodeSynced,
							Revision: "Current",
						},
					},
				},
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource became Progressing, updating status from Pending to Progressing.",
					Status:          "Progressing",
					Step:            "1",
					TargetRevisions: []string{"Current"},
				},
			},
		},
		{
			name: "progresses a progressing application to healthy",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application:     "app1",
							Message:         "",
							Status:          "Progressing",
							Step:            "1",
							TargetRevisions: []string{"Next"},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: v1alpha1.SyncStatus{
							Status:   v1alpha1.SyncStatusCodeSynced,
							Revision: "Next",
						},
					},
				},
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource became Healthy, updating status from Progressing to Healthy.",
					Status:          "Healthy",
					Step:            "1",
					TargetRevisions: []string{"Next"},
				},
			},
		},
		{
			name: "progresses a waiting healthy application to healthy",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application:     "app1",
							Message:         "",
							Status:          "Waiting",
							Step:            "1",
							TargetRevisions: []string{"Current"},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: v1alpha1.SyncStatus{
							Revision: "Current",
							Status:   v1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource is already Healthy, updating status from Waiting to Healthy.",
					Status:          "Healthy",
					Step:            "1",
					TargetRevisions: []string{"Current"},
				},
			},
		},
		{
			name: "progresses a new outofsync application in a later step to waiting",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationSucceeded,
							SyncResult: &v1alpha1.SyncOperationResult{
								Revision: "Previous",
							},
						},
						Sync: v1alpha1.SyncStatus{
							Status:   v1alpha1.SyncStatusCodeOutOfSync,
							Revision: "Next",
						},
					},
				},
			},
			appStepMap: map[string]int{
				"app1": 1,
				"app2": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "No Application status found, defaulting status to Waiting.",
					Status:          "Waiting",
					Step:            "2",
					TargetRevisions: []string{"Next"},
				},
			},
		},
		{
			name: "progresses a pending application with a successful sync triggered by controller to progressing",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							LastTransitionTime: &metav1.Time{
								Time: time.Now().Add(time.Duration(-1) * time.Minute),
							},
							Message:         "",
							Status:          "Pending",
							Step:            "1",
							TargetRevisions: []string{"Next"},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusDegraded,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationSucceeded,
							StartedAt: metav1.Time{
								Time: time.Now(),
							},
							Operation: v1alpha1.Operation{
								InitiatedBy: v1alpha1.OperationInitiator{
									Username:  "applicationset-controller",
									Automated: true,
								},
							},
							SyncResult: &v1alpha1.SyncOperationResult{
								Revision: "Next",
							},
						},
						Sync: v1alpha1.SyncStatus{
							Status:   v1alpha1.SyncStatusCodeSynced,
							Revision: "Next",
						},
					},
				},
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource completed a sync successfully, updating status from Pending to Progressing.",
					Status:          "Progressing",
					Step:            "1",
					TargetRevisions: []string{"Next"},
				},
			},
		},
		{
			name: "progresses a pending application with a successful sync trigger by applicationset-controller <1s ago to progressing",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							LastTransitionTime: &metav1.Time{
								Time: time.Now(),
							},
							Message:         "",
							Status:          "Pending",
							Step:            "1",
							TargetRevisions: []string{"Next"},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusDegraded,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationSucceeded,
							StartedAt: metav1.Time{
								Time: time.Now().Add(time.Duration(-1) * time.Second),
							},
							Operation: v1alpha1.Operation{
								InitiatedBy: v1alpha1.OperationInitiator{
									Username:  "applicationset-controller",
									Automated: true,
								},
							},
							SyncResult: &v1alpha1.SyncOperationResult{
								Revision: "Next",
							},
						},
						Sync: v1alpha1.SyncStatus{
							Status:   v1alpha1.SyncStatusCodeSynced,
							Revision: "Next",
						},
					},
				},
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource completed a sync successfully, updating status from Pending to Progressing.",
					Status:          "Progressing",
					Step:            "1",
					TargetRevisions: []string{"Next"},
				},
			},
		},
		{
			name: "removes the appStatus for applications that no longer exist",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application:     "app1",
							Message:         "Application has pending changes, setting status to Waiting.",
							Status:          "Waiting",
							Step:            "1",
							TargetRevisions: []string{"Current"},
						},
						{
							Application:     "app2",
							Message:         "Application has pending changes, setting status to Waiting.",
							Status:          "Waiting",
							Step:            "1",
							TargetRevisions: []string{"Current"},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: v1alpha1.SyncStatus{
							Status:   v1alpha1.SyncStatusCodeSynced,
							Revision: "Current",
						},
					},
				},
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource is already Healthy, updating status from Waiting to Healthy.",
					Status:          "Healthy",
					Step:            "1",
					TargetRevisions: []string{"Current"},
				},
			},
		},
		{
			name: "progresses a pending synced application with an old revision to progressing with the Current one",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application:     "app1",
							Message:         "",
							Status:          "Pending",
							Step:            "1",
							TargetRevisions: []string{"Old"},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationSucceeded,
							SyncResult: &v1alpha1.SyncOperationResult{
								Revision: "Current",
							},
						},
						Sync: v1alpha1.SyncStatus{
							Status:    v1alpha1.SyncStatusCodeSynced,
							Revisions: []string{"Current"},
						},
					},
				},
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "Application resource is already Healthy, updating status from Waiting to Healthy.",
					Status:          "Healthy",
					Step:            "1",
					TargetRevisions: []string{"Current"},
				},
			},
		},
	} {
		t.Run(cc.name, func(t *testing.T) {
			kubeclientset := kubefake.NewSimpleClientset([]runtime.Object{}...)

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&cc.appSet).WithStatusSubresource(&cc.appSet).Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics()

			argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)

			r := ApplicationSetReconciler{
				Client:        client,
				Scheme:        scheme,
				Recorder:      record.NewFakeRecorder(1),
				Generators:    map[string]generators.Generator{},
				ArgoDB:        argodb,
				KubeClientset: kubeclientset,
				Metrics:       metrics,
			}

			appStatuses, err := r.updateApplicationSetApplicationStatus(t.Context(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.apps, cc.appStepMap)

			// opt out of testing the LastTransitionTime is accurate
			for i := range appStatuses {
				appStatuses[i].LastTransitionTime = nil
			}

			require.NoError(t, err, "expected no errors, but errors occurred")
			assert.Equal(t, cc.expectedAppStatus, appStatuses, "expected appStatuses did not match actual")
		})
	}
}

func TestUpdateApplicationSetApplicationStatusProgress(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	for _, cc := range []struct {
		name              string
		appSet            v1alpha1.ApplicationSet
		appSyncMap        map[string]bool
		appStepMap        map[string]int
		appMap            map[string]v1alpha1.Application
		expectedAppStatus []v1alpha1.ApplicationSetApplicationStatus
	}{
		{
			name: "handles an empty appSync and appStepMap",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{},
				},
			},
			appSyncMap:        map[string]bool{},
			appStepMap:        map[string]int{},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{},
		},
		{
			name: "handles an empty strategy",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{},
				},
			},
			appSyncMap:        map[string]bool{},
			appStepMap:        map[string]int{},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{},
		},
		{
			name: "handles an empty applicationset strategy",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{},
				},
			},
			appSyncMap:        map[string]bool{},
			appStepMap:        map[string]int{},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{},
		},
		{
			name: "handles an appSyncMap with no existing statuses",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{},
				},
			},
			appSyncMap: map[string]bool{
				"app1": true,
				"app2": false,
			},
			appStepMap: map[string]int{
				"app1": 0,
				"app2": 1,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{},
		},
		{
			name: "handles updating a RollingSync status from Waiting to Pending",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application:     "app1",
							Message:         "Application is out of date with the current AppSet generation, setting status to Waiting.",
							Status:          "Waiting",
							TargetRevisions: []string{"Next"},
						},
					},
				},
			},
			appSyncMap: map[string]bool{
				"app1": true,
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					LastTransitionTime: nil,
					Message:            "Application moved to Pending status, watching for the Application resource to start Progressing.",
					Status:             "Pending",
					Step:               "1",
					TargetRevisions:    []string{"Next"},
				},
			},
		},
		{
			name: "does not update a RollingSync status if appSyncMap is false",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting.",
							Status:      "Waiting",
							Step:        "1",
						},
					},
				},
			},
			appSyncMap: map[string]bool{
				"app1": false,
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					LastTransitionTime: nil,
					Message:            "Application is out of date with the current AppSet generation, setting status to Waiting.",
					Status:             "Waiting",
					Step:               "1",
				},
			},
		},
		{
			name: "does not update a status if status is not pending",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Message:     "Application Pending status timed out while waiting to become Progressing, reset status to Healthy.",
							Status:      "Healthy",
							Step:        "1",
						},
					},
				},
			},
			appSyncMap: map[string]bool{
				"app1": true,
			},
			appStepMap: map[string]int{
				"app1": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					LastTransitionTime: nil,
					Message:            "Application Pending status timed out while waiting to become Progressing, reset status to Healthy.",
					Status:             "Healthy",
					Step:               "1",
				},
			},
		},
		{
			name: "does not update a status if maxUpdate has already been reached with RollingSync",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
									MaxUpdate: &intstr.IntOrString{
										Type:   intstr.Int,
										IntVal: 3,
									},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Message:     "Application resource became Progressing, updating status from Pending to Progressing.",
							Status:      "Progressing",
							Step:        "1",
						},
						{
							Application: "app2",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting.",
							Status:      "Waiting",
							Step:        "1",
						},
						{
							Application: "app3",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting.",
							Status:      "Waiting",
							Step:        "1",
						},
						{
							Application: "app4",
							Message:     "Application moved to Pending status, watching for the Application resource to start Progressing.",
							Status:      "Pending",
							Step:        "1",
						},
					},
				},
			},
			appSyncMap: map[string]bool{
				"app1": true,
				"app2": true,
				"app3": true,
				"app4": true,
			},
			appStepMap: map[string]int{
				"app1": 0,
				"app2": 0,
				"app3": 0,
				"app4": 0,
			},
			appMap: map[string]v1alpha1.Application{
				"app1": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				"app2": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app2",
					},
					Status: v1alpha1.ApplicationStatus{
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				"app3": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app3",
					},
					Status: v1alpha1.ApplicationStatus{
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				"app4": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app4",
					},
					Status: v1alpha1.ApplicationStatus{
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					LastTransitionTime: nil,
					Message:            "Application resource became Progressing, updating status from Pending to Progressing.",
					Status:             "Progressing",
					Step:               "1",
				},
				{
					Application:        "app2",
					LastTransitionTime: nil,
					Message:            "Application moved to Pending status, watching for the Application resource to start Progressing.",
					Status:             "Pending",
					Step:               "1",
				},
				{
					Application:        "app3",
					LastTransitionTime: nil,
					Message:            "Application is out of date with the current AppSet generation, setting status to Waiting.",
					Status:             "Waiting",
					Step:               "1",
				},
				{
					Application:        "app4",
					LastTransitionTime: nil,
					Message:            "Application moved to Pending status, watching for the Application resource to start Progressing.",
					Status:             "Pending",
					Step:               "1",
				},
			},
		},
		{
			name: "rounds down for maxUpdate set to percentage string",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
									MaxUpdate: &intstr.IntOrString{
										Type:   intstr.String,
										StrVal: "50%",
									},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting.",
							Status:      "Waiting",
							Step:        "1",
						},
						{
							Application: "app2",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting.",
							Status:      "Waiting",
							Step:        "1",
						},
						{
							Application: "app3",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting.",
							Status:      "Waiting",
							Step:        "1",
						},
					},
				},
			},
			appSyncMap: map[string]bool{
				"app1": true,
				"app2": true,
				"app3": true,
			},
			appStepMap: map[string]int{
				"app1": 0,
				"app2": 0,
				"app3": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					LastTransitionTime: nil,
					Message:            "Application moved to Pending status, watching for the Application resource to start Progressing.",
					Status:             "Pending",
					Step:               "1",
				},
				{
					Application:        "app2",
					LastTransitionTime: nil,
					Message:            "Application is out of date with the current AppSet generation, setting status to Waiting.",
					Status:             "Waiting",
					Step:               "1",
				},
				{
					Application:        "app3",
					LastTransitionTime: nil,
					Message:            "Application is out of date with the current AppSet generation, setting status to Waiting.",
					Status:             "Waiting",
					Step:               "1",
				},
			},
		},
		{
			name: "does not update any applications with maxUpdate set to 0",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
									MaxUpdate: &intstr.IntOrString{
										Type:   intstr.Int,
										IntVal: 0,
									},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting.",
							Status:      "Waiting",
							Step:        "1",
						},
						{
							Application: "app2",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting.",
							Status:      "Waiting",
							Step:        "1",
						},
						{
							Application: "app3",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting.",
							Status:      "Waiting",
							Step:        "1",
						},
					},
				},
			},
			appSyncMap: map[string]bool{
				"app1": true,
				"app2": true,
				"app3": true,
			},
			appStepMap: map[string]int{
				"app1": 0,
				"app2": 0,
				"app3": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					LastTransitionTime: nil,
					Message:            "Application is out of date with the current AppSet generation, setting status to Waiting.",
					Status:             "Waiting",
					Step:               "1",
				},
				{
					Application:        "app2",
					LastTransitionTime: nil,
					Message:            "Application is out of date with the current AppSet generation, setting status to Waiting.",
					Status:             "Waiting",
					Step:               "1",
				},
				{
					Application:        "app3",
					LastTransitionTime: nil,
					Message:            "Application is out of date with the current AppSet generation, setting status to Waiting.",
					Status:             "Waiting",
					Step:               "1",
				},
			},
		},
		{
			name: "updates all applications with maxUpdate set to 100%",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
									MaxUpdate: &intstr.IntOrString{
										Type:   intstr.String,
										StrVal: "100%",
									},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting.",
							Status:      "Waiting",
							Step:        "1",
						},
						{
							Application: "app2",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting.",
							Status:      "Waiting",
							Step:        "1",
						},
						{
							Application: "app3",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting.",
							Status:      "Waiting",
							Step:        "1",
						},
					},
				},
			},
			appSyncMap: map[string]bool{
				"app1": true,
				"app2": true,
				"app3": true,
			},
			appStepMap: map[string]int{
				"app1": 0,
				"app2": 0,
				"app3": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					LastTransitionTime: nil,
					Message:            "Application moved to Pending status, watching for the Application resource to start Progressing.",
					Status:             "Pending",
					Step:               "1",
				},
				{
					Application:        "app2",
					LastTransitionTime: nil,
					Message:            "Application moved to Pending status, watching for the Application resource to start Progressing.",
					Status:             "Pending",
					Step:               "1",
				},
				{
					Application:        "app3",
					LastTransitionTime: nil,
					Message:            "Application moved to Pending status, watching for the Application resource to start Progressing.",
					Status:             "Pending",
					Step:               "1",
				},
			},
		},
		{
			name: "updates at least 1 application with maxUpdate >0%",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
									MaxUpdate: &intstr.IntOrString{
										Type:   intstr.String,
										StrVal: "1%",
									},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting.",
							Status:      "Waiting",
							Step:        "1",
						},
						{
							Application: "app2",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting.",
							Status:      "Waiting",
							Step:        "1",
						},
						{
							Application: "app3",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting.",
							Status:      "Waiting",
							Step:        "1",
						},
					},
				},
			},
			appSyncMap: map[string]bool{
				"app1": true,
				"app2": true,
				"app3": true,
			},
			appStepMap: map[string]int{
				"app1": 0,
				"app2": 0,
				"app3": 0,
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:        "app1",
					LastTransitionTime: nil,
					Message:            "Application moved to Pending status, watching for the Application resource to start Progressing.",
					Status:             "Pending",
					Step:               "1",
				},
				{
					Application:        "app2",
					LastTransitionTime: nil,
					Message:            "Application is out of date with the current AppSet generation, setting status to Waiting.",
					Status:             "Waiting",
					Step:               "1",
				},
				{
					Application:        "app3",
					LastTransitionTime: nil,
					Message:            "Application is out of date with the current AppSet generation, setting status to Waiting.",
					Status:             "Waiting",
					Step:               "1",
				},
			},
		},
	} {
		t.Run(cc.name, func(t *testing.T) {
			kubeclientset := kubefake.NewSimpleClientset([]runtime.Object{}...)

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&cc.appSet).WithStatusSubresource(&cc.appSet).Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics()

			argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)

			r := ApplicationSetReconciler{
				Client:        client,
				Scheme:        scheme,
				Recorder:      record.NewFakeRecorder(1),
				Generators:    map[string]generators.Generator{},
				ArgoDB:        argodb,
				KubeClientset: kubeclientset,
				Metrics:       metrics,
			}

			appStatuses, err := r.updateApplicationSetApplicationStatusProgress(t.Context(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.appSyncMap, cc.appStepMap)

			// opt out of testing the LastTransitionTime is accurate
			for i := range appStatuses {
				appStatuses[i].LastTransitionTime = nil
			}

			require.NoError(t, err, "expected no errors, but errors occurred")
			assert.Equal(t, cc.expectedAppStatus, appStatuses, "expected appStatuses did not match actual")
		})
	}
}

func TestUpdateResourceStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	for _, cc := range []struct {
		name              string
		appSet            v1alpha1.ApplicationSet
		apps              []v1alpha1.Application
		expectedResources []v1alpha1.ResourceStatus
	}{
		{
			name: "handles an empty application list",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Status: v1alpha1.ApplicationSetStatus{
					Resources: []v1alpha1.ResourceStatus{},
				},
			},
			apps:              []v1alpha1.Application{},
			expectedResources: nil,
		},
		{
			name: "adds status if no existing statuses",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
					},
				},
			},
			expectedResources: []v1alpha1.ResourceStatus{
				{
					Name:   "app1",
					Status: v1alpha1.SyncStatusCodeSynced,
					Health: &v1alpha1.HealthStatus{
						Status: health.HealthStatusHealthy,
					},
				},
			},
		},
		{
			name: "handles an applicationset with existing and up-to-date status",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Status: v1alpha1.ApplicationSetStatus{
					Resources: []v1alpha1.ResourceStatus{
						{
							Name:   "app1",
							Status: v1alpha1.SyncStatusCodeSynced,
							Health: &v1alpha1.HealthStatus{
								Status: health.HealthStatusHealthy,
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
					},
				},
			},
			expectedResources: []v1alpha1.ResourceStatus{
				{
					Name:   "app1",
					Status: v1alpha1.SyncStatusCodeSynced,
					Health: &v1alpha1.HealthStatus{
						Status: health.HealthStatusHealthy,
					},
				},
			},
		},
		{
			name: "updates an applicationset with existing and out of date status",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Status: v1alpha1.ApplicationSetStatus{
					Resources: []v1alpha1.ResourceStatus{
						{
							Name:   "app1",
							Status: v1alpha1.SyncStatusCodeOutOfSync,
							Health: &v1alpha1.HealthStatus{
								Status:  health.HealthStatusProgressing,
								Message: "Progressing",
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
						Health: v1alpha1.AppHealthStatus{
							Status: health.HealthStatusHealthy,
						},
					},
				},
			},
			expectedResources: []v1alpha1.ResourceStatus{
				{
					Name:   "app1",
					Status: v1alpha1.SyncStatusCodeSynced,
					Health: &v1alpha1.HealthStatus{
						Status: health.HealthStatusHealthy,
					},
				},
			},
		},
		{
			name: "deletes an applicationset status if the application no longer exists",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Status: v1alpha1.ApplicationSetStatus{
					Resources: []v1alpha1.ResourceStatus{
						{
							Name:   "app1",
							Status: v1alpha1.SyncStatusCodeSynced,
							Health: &v1alpha1.HealthStatus{
								Status:  health.HealthStatusHealthy,
								Message: "OK",
							},
						},
					},
				},
			},
			apps:              []v1alpha1.Application{},
			expectedResources: nil,
		},
	} {
		t.Run(cc.name, func(t *testing.T) {
			kubeclientset := kubefake.NewSimpleClientset([]runtime.Object{}...)

			client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&cc.appSet).WithObjects(&cc.appSet).Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics()

			argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)

			r := ApplicationSetReconciler{
				Client:        client,
				Scheme:        scheme,
				Recorder:      record.NewFakeRecorder(1),
				Generators:    map[string]generators.Generator{},
				ArgoDB:        argodb,
				KubeClientset: kubeclientset,
				Metrics:       metrics,
			}

			err := r.updateResourcesStatus(t.Context(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.apps)

			require.NoError(t, err, "expected no errors, but errors occurred")
			assert.Equal(t, cc.expectedResources, cc.appSet.Status.Resources, "expected resources did not match actual")
		})
	}
}

func generateNAppResourceStatuses(n int) []v1alpha1.ResourceStatus {
	var r []v1alpha1.ResourceStatus
	for i := 0; i < n; i++ {
		r = append(r, v1alpha1.ResourceStatus{
			Name:   "app" + strconv.Itoa(i),
			Status: v1alpha1.SyncStatusCodeSynced,
			Health: &v1alpha1.HealthStatus{
				Status: health.HealthStatusHealthy,
			},
		},
		)
	}
	return r
}

func generateNHealthyApps(n int) []v1alpha1.Application {
	var r []v1alpha1.Application
	for i := 0; i < n; i++ {
		r = append(r, v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name: "app" + strconv.Itoa(i),
			},
			Status: v1alpha1.ApplicationStatus{
				Sync: v1alpha1.SyncStatus{
					Status: v1alpha1.SyncStatusCodeSynced,
				},
				Health: v1alpha1.AppHealthStatus{
					Status: health.HealthStatusHealthy,
				},
			},
		})
	}
	return r
}

func TestResourceStatusAreOrdered(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	err = v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)
	for _, cc := range []struct {
		name              string
		appSet            v1alpha1.ApplicationSet
		apps              []v1alpha1.Application
		expectedResources []v1alpha1.ResourceStatus
	}{
		{
			name: "Ensures AppSet is always ordered",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Status: v1alpha1.ApplicationSetStatus{
					Resources: []v1alpha1.ResourceStatus{},
				},
			},
			apps:              generateNHealthyApps(10),
			expectedResources: generateNAppResourceStatuses(10),
		},
	} {
		t.Run(cc.name, func(t *testing.T) {
			kubeclientset := kubefake.NewSimpleClientset([]runtime.Object{}...)

			client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&cc.appSet).WithObjects(&cc.appSet).Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics()

			argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)

			r := ApplicationSetReconciler{
				Client:        client,
				Scheme:        scheme,
				Recorder:      record.NewFakeRecorder(1),
				Generators:    map[string]generators.Generator{},
				ArgoDB:        argodb,
				KubeClientset: kubeclientset,
				Metrics:       metrics,
			}

			err := r.updateResourcesStatus(t.Context(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.apps)
			require.NoError(t, err, "expected no errors, but errors occurred")

			err = r.updateResourcesStatus(t.Context(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.apps)
			require.NoError(t, err, "expected no errors, but errors occurred")

			err = r.updateResourcesStatus(t.Context(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.apps)
			require.NoError(t, err, "expected no errors, but errors occurred")

			assert.Equal(t, cc.expectedResources, cc.appSet.Status.Resources, "expected resources did not match actual")
		})
	}
}

func TestApplicationOwnsHandler(t *testing.T) {
	// progressive syncs do not affect create, delete, or generic
	ownsHandler := getApplicationOwnsHandler(true)
	assert.False(t, ownsHandler.CreateFunc(event.CreateEvent{}))
	assert.True(t, ownsHandler.DeleteFunc(event.DeleteEvent{}))
	assert.True(t, ownsHandler.GenericFunc(event.GenericEvent{}))
	ownsHandler = getApplicationOwnsHandler(false)
	assert.False(t, ownsHandler.CreateFunc(event.CreateEvent{}))
	assert.True(t, ownsHandler.DeleteFunc(event.DeleteEvent{}))
	assert.True(t, ownsHandler.GenericFunc(event.GenericEvent{}))

	now := metav1.Now()
	type args struct {
		e                      event.UpdateEvent
		enableProgressiveSyncs bool
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "SameApplicationReconciledAtDiff", args: args{e: event.UpdateEvent{
			ObjectOld: &v1alpha1.Application{Status: v1alpha1.ApplicationStatus{ReconciledAt: &now}},
			ObjectNew: &v1alpha1.Application{Status: v1alpha1.ApplicationStatus{ReconciledAt: &now}},
		}}, want: false},
		{name: "SameApplicationResourceVersionDiff", args: args{e: event.UpdateEvent{
			ObjectOld: &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: "foo",
			}},
			ObjectNew: &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: "bar",
			}},
		}}, want: false},
		{name: "ApplicationHealthStatusDiff", args: args{
			e: event.UpdateEvent{
				ObjectOld: &v1alpha1.Application{Status: v1alpha1.ApplicationStatus{
					Health: v1alpha1.AppHealthStatus{
						Status: "Unknown",
					},
				}},
				ObjectNew: &v1alpha1.Application{Status: v1alpha1.ApplicationStatus{
					Health: v1alpha1.AppHealthStatus{
						Status: "Healthy",
					},
				}},
			},
			enableProgressiveSyncs: true,
		}, want: true},
		{name: "ApplicationSyncStatusDiff", args: args{
			e: event.UpdateEvent{
				ObjectOld: &v1alpha1.Application{Status: v1alpha1.ApplicationStatus{
					Sync: v1alpha1.SyncStatus{
						Status: "OutOfSync",
					},
				}},
				ObjectNew: &v1alpha1.Application{Status: v1alpha1.ApplicationStatus{
					Sync: v1alpha1.SyncStatus{
						Status: "Synced",
					},
				}},
			},
			enableProgressiveSyncs: true,
		}, want: true},
		{name: "ApplicationOperationStateDiff", args: args{
			e: event.UpdateEvent{
				ObjectOld: &v1alpha1.Application{Status: v1alpha1.ApplicationStatus{
					OperationState: &v1alpha1.OperationState{
						Phase: "foo",
					},
				}},
				ObjectNew: &v1alpha1.Application{Status: v1alpha1.ApplicationStatus{
					OperationState: &v1alpha1.OperationState{
						Phase: "bar",
					},
				}},
			},
			enableProgressiveSyncs: true,
		}, want: true},
		{name: "ApplicationOperationStartedAtDiff", args: args{
			e: event.UpdateEvent{
				ObjectOld: &v1alpha1.Application{Status: v1alpha1.ApplicationStatus{
					OperationState: &v1alpha1.OperationState{
						StartedAt: now,
					},
				}},
				ObjectNew: &v1alpha1.Application{Status: v1alpha1.ApplicationStatus{
					OperationState: &v1alpha1.OperationState{
						StartedAt: metav1.NewTime(now.Add(time.Minute * 1)),
					},
				}},
			},
			enableProgressiveSyncs: true,
		}, want: true},
		{name: "SameApplicationGeneration", args: args{e: event.UpdateEvent{
			ObjectOld: &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{
				Generation: 1,
			}},
			ObjectNew: &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{
				Generation: 2,
			}},
		}}, want: false},
		{name: "DifferentApplicationSpec", args: args{e: event.UpdateEvent{
			ObjectOld: &v1alpha1.Application{Spec: v1alpha1.ApplicationSpec{Project: "default"}},
			ObjectNew: &v1alpha1.Application{Spec: v1alpha1.ApplicationSpec{Project: "not-default"}},
		}}, want: true},
		{name: "DifferentApplicationLabels", args: args{e: event.UpdateEvent{
			ObjectOld: &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}}},
			ObjectNew: &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"bar": "foo"}}},
		}}, want: true},
		{name: "DifferentApplicationLabelsNil", args: args{e: event.UpdateEvent{
			ObjectOld: &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{}}},
			ObjectNew: &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Labels: nil}},
		}}, want: false},
		{name: "DifferentApplicationAnnotations", args: args{e: event.UpdateEvent{
			ObjectOld: &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"foo": "bar"}}},
			ObjectNew: &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"bar": "foo"}}},
		}}, want: true},
		{name: "DifferentApplicationAnnotationsNil", args: args{e: event.UpdateEvent{
			ObjectOld: &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}},
			ObjectNew: &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Annotations: nil}},
		}}, want: false},
		{name: "DifferentApplicationFinalizers", args: args{e: event.UpdateEvent{
			ObjectOld: &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{"argo"}}},
			ObjectNew: &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{"none"}}},
		}}, want: true},
		{name: "DifferentApplicationFinalizersNil", args: args{e: event.UpdateEvent{
			ObjectOld: &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			ObjectNew: &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Finalizers: nil}},
		}}, want: false},
		{name: "ApplicationDestinationSame", args: args{
			e: event.UpdateEvent{
				ObjectOld: &v1alpha1.Application{
					Spec: v1alpha1.ApplicationSpec{
						Destination: v1alpha1.ApplicationDestination{
							Server:    "server",
							Namespace: "ns",
							Name:      "name",
						},
					},
				},
				ObjectNew: &v1alpha1.Application{
					Spec: v1alpha1.ApplicationSpec{
						Destination: v1alpha1.ApplicationDestination{
							Server:    "server",
							Namespace: "ns",
							Name:      "name",
						},
					},
				},
			},
			enableProgressiveSyncs: true,
		}, want: false},
		{name: "ApplicationDestinationDiff", args: args{
			e: event.UpdateEvent{
				ObjectOld: &v1alpha1.Application{
					Spec: v1alpha1.ApplicationSpec{
						Destination: v1alpha1.ApplicationDestination{
							Server:    "server",
							Namespace: "ns",
							Name:      "name",
						},
					},
				},
				ObjectNew: &v1alpha1.Application{
					Spec: v1alpha1.ApplicationSpec{
						Destination: v1alpha1.ApplicationDestination{
							Server:    "notSameServer",
							Namespace: "ns",
							Name:      "name",
						},
					},
				},
			},
			enableProgressiveSyncs: true,
		}, want: true},
		{name: "NotAnAppOld", args: args{e: event.UpdateEvent{
			ObjectOld: &v1alpha1.AppProject{},
			ObjectNew: &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"bar": "foo"}}},
		}}, want: false},
		{name: "NotAnAppNew", args: args{e: event.UpdateEvent{
			ObjectOld: &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}}},
			ObjectNew: &v1alpha1.AppProject{},
		}}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ownsHandler = getApplicationOwnsHandler(tt.args.enableProgressiveSyncs)
			assert.Equalf(t, tt.want, ownsHandler.UpdateFunc(tt.args.e), "UpdateFunc(%v)", tt.args.e)
		})
	}
}

func TestMigrateStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	err = v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	for _, tc := range []struct {
		name           string
		appset         v1alpha1.ApplicationSet
		expectedStatus v1alpha1.ApplicationSetStatus
	}{
		{
			name: "status without applicationstatus target revisions set will default to empty list",
			appset: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{},
					},
				},
			},
			expectedStatus: v1alpha1.ApplicationSetStatus{
				ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
					{
						TargetRevisions: []string{},
					},
				},
			},
		},
		{
			name: "status with applicationstatus target revisions set will do nothing",
			appset: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							TargetRevisions: []string{"Current"},
						},
					},
				},
			},
			expectedStatus: v1alpha1.ApplicationSetStatus{
				ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
					{
						TargetRevisions: []string{"Current"},
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&tc.appset).WithObjects(&tc.appset).Build()
			r := ApplicationSetReconciler{
				Client: client,
			}

			err := r.migrateStatus(t.Context(), &tc.appset)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, tc.appset.Status)
		})
	}
}

func TestApplicationSetOwnsHandlerUpdate(t *testing.T) {
	buildAppSet := func(annotations map[string]string) *v1alpha1.ApplicationSet {
		return &v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: annotations,
			},
		}
	}

	tests := []struct {
		name                   string
		appSetOld              crtclient.Object
		appSetNew              crtclient.Object
		enableProgressiveSyncs bool
		want                   bool
	}{
		{
			name: "Different Spec",
			appSetOld: &v1alpha1.ApplicationSet{
				Spec: v1alpha1.ApplicationSetSpec{
					Generators: []v1alpha1.ApplicationSetGenerator{
						{List: &v1alpha1.ListGenerator{}},
					},
				},
			},
			appSetNew: &v1alpha1.ApplicationSet{
				Spec: v1alpha1.ApplicationSetSpec{
					Generators: []v1alpha1.ApplicationSetGenerator{
						{Git: &v1alpha1.GitGenerator{}},
					},
				},
			},
			enableProgressiveSyncs: false,
			want:                   true,
		},
		{
			name:                   "Different Annotations",
			appSetOld:              buildAppSet(map[string]string{"key1": "value1"}),
			appSetNew:              buildAppSet(map[string]string{"key1": "value2"}),
			enableProgressiveSyncs: false,
			want:                   true,
		},
		{
			name: "Different Labels",
			appSetOld: &v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"key1": "value1"},
				},
			},
			appSetNew: &v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"key1": "value2"},
				},
			},
			enableProgressiveSyncs: false,
			want:                   true,
		},
		{
			name: "Different Finalizers",
			appSetOld: &v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{"finalizer1"},
				},
			},
			appSetNew: &v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{"finalizer2"},
				},
			},
			enableProgressiveSyncs: false,
			want:                   true,
		},
		{
			name: "No Changes",
			appSetOld: &v1alpha1.ApplicationSet{
				Spec: v1alpha1.ApplicationSetSpec{
					Generators: []v1alpha1.ApplicationSetGenerator{
						{List: &v1alpha1.ListGenerator{}},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"key1": "value1"},
					Labels:      map[string]string{"key1": "value1"},
					Finalizers:  []string{"finalizer1"},
				},
			},
			appSetNew: &v1alpha1.ApplicationSet{
				Spec: v1alpha1.ApplicationSetSpec{
					Generators: []v1alpha1.ApplicationSetGenerator{
						{List: &v1alpha1.ListGenerator{}},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"key1": "value1"},
					Labels:      map[string]string{"key1": "value1"},
					Finalizers:  []string{"finalizer1"},
				},
			},
			enableProgressiveSyncs: false,
			want:                   false,
		},
		{
			name: "annotation removed",
			appSetOld: buildAppSet(map[string]string{
				argocommon.AnnotationApplicationSetRefresh: "true",
			}),
			appSetNew:              buildAppSet(map[string]string{}),
			enableProgressiveSyncs: false,
			want:                   false,
		},
		{
			name: "annotation not removed",
			appSetOld: buildAppSet(map[string]string{
				argocommon.AnnotationApplicationSetRefresh: "true",
			}),
			appSetNew: buildAppSet(map[string]string{
				argocommon.AnnotationApplicationSetRefresh: "true",
			}),
			enableProgressiveSyncs: false,
			want:                   false,
		},
		{
			name:      "annotation added",
			appSetOld: buildAppSet(map[string]string{}),
			appSetNew: buildAppSet(map[string]string{
				argocommon.AnnotationApplicationSetRefresh: "true",
			}),
			enableProgressiveSyncs: false,
			want:                   true,
		},
		{
			name:                   "old object is not an appset",
			appSetOld:              &v1alpha1.Application{},
			appSetNew:              buildAppSet(map[string]string{}),
			enableProgressiveSyncs: false,
			want:                   false,
		},
		{
			name:                   "new object is not an appset",
			appSetOld:              buildAppSet(map[string]string{}),
			appSetNew:              &v1alpha1.Application{},
			enableProgressiveSyncs: false,
			want:                   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ownsHandler := getApplicationSetOwnsHandler(tt.enableProgressiveSyncs)
			requeue := ownsHandler.UpdateFunc(event.UpdateEvent{
				ObjectOld: tt.appSetOld,
				ObjectNew: tt.appSetNew,
			})
			assert.Equalf(t, tt.want, requeue, "ownsHandler.UpdateFunc(%v, %v, %t)", tt.appSetOld, tt.appSetNew, tt.enableProgressiveSyncs)
		})
	}
}

func TestApplicationSetOwnsHandlerGeneric(t *testing.T) {
	ownsHandler := getApplicationSetOwnsHandler(false)
	tests := []struct {
		name string
		obj  crtclient.Object
		want bool
	}{
		{
			name: "Object is ApplicationSet",
			obj:  &v1alpha1.ApplicationSet{},
			want: true,
		},
		{
			name: "Object is not ApplicationSet",
			obj:  &v1alpha1.Application{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requeue := ownsHandler.GenericFunc(event.GenericEvent{
				Object: tt.obj,
			})
			assert.Equalf(t, tt.want, requeue, "ownsHandler.GenericFunc(%v)", tt.obj)
		})
	}
}

func TestApplicationSetOwnsHandlerCreate(t *testing.T) {
	ownsHandler := getApplicationSetOwnsHandler(false)
	tests := []struct {
		name string
		obj  crtclient.Object
		want bool
	}{
		{
			name: "Object is ApplicationSet",
			obj:  &v1alpha1.ApplicationSet{},
			want: true,
		},
		{
			name: "Object is not ApplicationSet",
			obj:  &v1alpha1.Application{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requeue := ownsHandler.CreateFunc(event.CreateEvent{
				Object: tt.obj,
			})
			assert.Equalf(t, tt.want, requeue, "ownsHandler.CreateFunc(%v)", tt.obj)
		})
	}
}

func TestApplicationSetOwnsHandlerDelete(t *testing.T) {
	ownsHandler := getApplicationSetOwnsHandler(false)
	tests := []struct {
		name string
		obj  crtclient.Object
		want bool
	}{
		{
			name: "Object is ApplicationSet",
			obj:  &v1alpha1.ApplicationSet{},
			want: true,
		},
		{
			name: "Object is not ApplicationSet",
			obj:  &v1alpha1.Application{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requeue := ownsHandler.DeleteFunc(event.DeleteEvent{
				Object: tt.obj,
			})
			assert.Equalf(t, tt.want, requeue, "ownsHandler.DeleteFunc(%v)", tt.obj)
		})
	}
}

func TestShouldRequeueForApplicationSet(t *testing.T) {
	type args struct {
		appSetOld              *v1alpha1.ApplicationSet
		appSetNew              *v1alpha1.ApplicationSet
		enableProgressiveSyncs bool
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "NilAppSet",
			args: args{
				appSetNew:              &v1alpha1.ApplicationSet{},
				appSetOld:              nil,
				enableProgressiveSyncs: false,
			},
			want: false,
		},
		{
			name: "ApplicationSetApplicationStatusChanged",
			args: args{
				appSetOld: &v1alpha1.ApplicationSet{
					Status: v1alpha1.ApplicationSetStatus{
						ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
							{
								Application: "app1",
								Status:      "Healthy",
							},
						},
					},
				},
				appSetNew: &v1alpha1.ApplicationSet{
					Status: v1alpha1.ApplicationSetStatus{
						ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
							{
								Application: "app1",
								Status:      "Waiting",
							},
						},
					},
				},
				enableProgressiveSyncs: true,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, shouldRequeueForApplicationSet(tt.args.appSetOld, tt.args.appSetNew, tt.args.enableProgressiveSyncs), "shouldRequeueForApplicationSet(%v, %v)", tt.args.appSetOld, tt.args.appSetNew)
		})
	}
}

func TestIgnoreNotAllowedNamespaces(t *testing.T) {
	tests := []struct {
		name       string
		namespaces []string
		objectNS   string
		expected   bool
	}{
		{
			name:       "Namespace allowed",
			namespaces: []string{"allowed-namespace"},
			objectNS:   "allowed-namespace",
			expected:   true,
		},
		{
			name:       "Namespace not allowed",
			namespaces: []string{"allowed-namespace"},
			objectNS:   "not-allowed-namespace",
			expected:   false,
		},
		{
			name:       "Empty allowed namespaces",
			namespaces: []string{},
			objectNS:   "any-namespace",
			expected:   false,
		},
		{
			name:       "Multiple allowed namespaces",
			namespaces: []string{"allowed-namespace-1", "allowed-namespace-2"},
			objectNS:   "allowed-namespace-2",
			expected:   true,
		},
		{
			name:       "Namespace not in multiple allowed namespaces",
			namespaces: []string{"allowed-namespace-1", "allowed-namespace-2"},
			objectNS:   "not-allowed-namespace",
			expected:   false,
		},
		{
			name:       "Namespace matched by glob pattern",
			namespaces: []string{"allowed-namespace-*"},
			objectNS:   "allowed-namespace-1",
			expected:   true,
		},
		{
			name:       "Namespace matched by regex pattern",
			namespaces: []string{"/^allowed-namespace-[^-]+$/"},
			objectNS:   "allowed-namespace-1",
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			predicate := ignoreNotAllowedNamespaces(tt.namespaces)
			object := &v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: tt.objectNS,
				},
			}

			t.Run(tt.name+":Create", func(t *testing.T) {
				result := predicate.Create(event.CreateEvent{Object: object})
				assert.Equal(t, tt.expected, result)
			})

			t.Run(tt.name+":Update", func(t *testing.T) {
				result := predicate.Update(event.UpdateEvent{ObjectNew: object})
				assert.Equal(t, tt.expected, result)
			})

			t.Run(tt.name+":Delete", func(t *testing.T) {
				result := predicate.Delete(event.DeleteEvent{Object: object})
				assert.Equal(t, tt.expected, result)
			})

			t.Run(tt.name+":Generic", func(t *testing.T) {
				result := predicate.Generic(event.GenericEvent{Object: object})
				assert.Equal(t, tt.expected, result)
			})
		})
	}
}

func TestIsRollingSyncStrategy(t *testing.T) {
	tests := []struct {
		name     string
		appset   *v1alpha1.ApplicationSet
		expected bool
	}{
		{
			name: "RollingSync strategy is explicitly set",
			appset: &v1alpha1.ApplicationSet{
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "AllAtOnce strategy is explicitly set",
			appset: &v1alpha1.ApplicationSet{
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "AllAtOnce",
					},
				},
			},
			expected: false,
		},
		{
			name: "Strategy is empty",
			appset: &v1alpha1.ApplicationSet{
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{},
				},
			},
			expected: false,
		},
		{
			name: "Strategy is nil",
			appset: &v1alpha1.ApplicationSet{
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: nil,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRollingSyncStrategy(tt.appset)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSyncApplication(t *testing.T) {
	tests := []struct {
		name     string
		input    v1alpha1.Application
		prune    bool
		expected v1alpha1.Application
	}{
		{
			name: "Default retry limit with no SyncPolicy",
			input: v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{},
			},
			prune: false,
			expected: v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{},
				Operation: &v1alpha1.Operation{
					InitiatedBy: v1alpha1.OperationInitiator{
						Username:  "applicationset-controller",
						Automated: true,
					},
					Info: []*v1alpha1.Info{
						{
							Name:  "Reason",
							Value: "ApplicationSet RollingSync triggered a sync of this Application resource.",
						},
					},
					Sync: &v1alpha1.SyncOperation{
						Prune: false,
					},
					Retry: v1alpha1.RetryStrategy{
						Limit: 5,
					},
				},
			},
		},
		{
			name: "Retry and SyncOptions from SyncPolicy are applied",
			input: v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{
					SyncPolicy: &v1alpha1.SyncPolicy{
						Retry: &v1alpha1.RetryStrategy{
							Limit: 10,
						},
						SyncOptions: []string{"CreateNamespace=true"},
					},
				},
			},
			prune: true,
			expected: v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{
					SyncPolicy: &v1alpha1.SyncPolicy{
						Retry: &v1alpha1.RetryStrategy{
							Limit: 10,
						},
						SyncOptions: []string{"CreateNamespace=true"},
					},
				},
				Operation: &v1alpha1.Operation{
					InitiatedBy: v1alpha1.OperationInitiator{
						Username:  "applicationset-controller",
						Automated: true,
					},
					Info: []*v1alpha1.Info{
						{
							Name:  "Reason",
							Value: "ApplicationSet RollingSync triggered a sync of this Application resource.",
						},
					},
					Sync: &v1alpha1.SyncOperation{
						SyncOptions: []string{"CreateNamespace=true"},
						Prune:       true,
					},
					Retry: v1alpha1.RetryStrategy{
						Limit: 10,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := syncApplication(tt.input, tt.prune)
			assert.Equal(t, tt.expected, result)
		})
	}
}
