package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v3/applicationset/progressivesync"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	crtclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/health"

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

	kubeclientset := kubefake.NewClientset(append(obj, emptyArgoCDConfigMap, argoCDSecret)...)
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
			name: "Ensure that hydrate annotation is preserved from an existing app",
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
						Annotations: map[string]string{
							"annot-key":                   "annot-value",
							v1alpha1.AnnotationKeyHydrate: string(v1alpha1.RefreshTypeNormal),
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
							v1alpha1.AnnotationKeyHydrate: string(v1alpha1.RefreshTypeNormal),
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
			name: "Ensure that unnormalized live spec does not cause a spurious patch",
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
						// Without normalizing the live object, the equality check
						// sees &SyncPolicy{} vs nil and issues an unnecessary patch.
						SyncPolicy: &v1alpha1.SyncPolicy{},
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
						Project:    "project",
						SyncPolicy: nil,
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
						Project:    "project",
						SyncPolicy: &v1alpha1.SyncPolicy{},
					},
				},
			},
		},
		{
			name: "Ensure that argocd pre-delete and post-delete finalizers are preserved from an existing app",
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
							"non-argo-finalizer",
							v1alpha1.PreDeleteFinalizerName,
							v1alpha1.PreDeleteFinalizerName + "/stage1",
							v1alpha1.PostDeleteFinalizerName,
							v1alpha1.PostDeleteFinalizerName + "/stage2",
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
						Finalizers: []string{
							v1alpha1.PreDeleteFinalizerName,
							v1alpha1.PreDeleteFinalizerName + "/stage1",
							v1alpha1.PostDeleteFinalizerName,
							v1alpha1.PostDeleteFinalizerName + "/stage2",
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

func TestCreateOrUpdateInCluster_Concurrent(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	appSet := v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
	}

	t.Run("all apps are created correctly with concurrency > 1", func(t *testing.T) {
		desiredApps := make([]v1alpha1.Application, 5)
		for i := range desiredApps {
			desiredApps[i] = v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("app%d", i),
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSpec{Project: "project"},
			}
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(&appSet).
			WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).
			Build()
		metrics := appsetmetrics.NewFakeAppsetMetrics()

		r := ApplicationSetReconciler{
			Client:                       fakeClient,
			Scheme:                       scheme,
			Recorder:                     record.NewFakeRecorder(10),
			Metrics:                      metrics,
			ConcurrentApplicationUpdates: 5,
		}

		err = r.createOrUpdateInCluster(t.Context(), log.NewEntry(log.StandardLogger()), appSet, desiredApps)
		require.NoError(t, err)

		for _, desired := range desiredApps {
			got := &v1alpha1.Application{}
			require.NoError(t, fakeClient.Get(t.Context(), crtclient.ObjectKey{Namespace: desired.Namespace, Name: desired.Name}, got))
			assert.Equal(t, desired.Spec.Project, got.Spec.Project)
		}
	})

	t.Run("non-context errors from concurrent goroutines are collected and one is returned", func(t *testing.T) {
		existingApps := make([]v1alpha1.Application, 5)
		initObjs := []crtclient.Object{&appSet}
		for i := range existingApps {
			existingApps[i] = v1alpha1.Application{
				TypeMeta: metav1.TypeMeta{
					Kind:       application.ApplicationKind,
					APIVersion: "argoproj.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            fmt.Sprintf("app%d", i),
					Namespace:       "namespace",
					ResourceVersion: "1",
				},
				Spec: v1alpha1.ApplicationSpec{Project: "old"},
			}
			app := existingApps[i].DeepCopy()
			require.NoError(t, controllerutil.SetControllerReference(&appSet, app, scheme))
			initObjs = append(initObjs, app)
		}

		desiredApps := make([]v1alpha1.Application, 5)
		for i := range desiredApps {
			desiredApps[i] = v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("app%d", i),
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSpec{Project: "new"},
			}
		}

		patchErr := errors.New("some patch error")
		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(initObjs...).
			WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).
			WithInterceptorFuncs(interceptor.Funcs{
				Patch: func(_ context.Context, _ crtclient.WithWatch, _ crtclient.Object, _ crtclient.Patch, _ ...crtclient.PatchOption) error {
					return patchErr
				},
			}).
			Build()
		metrics := appsetmetrics.NewFakeAppsetMetrics()

		r := ApplicationSetReconciler{
			Client:                       fakeClient,
			Scheme:                       scheme,
			Recorder:                     record.NewFakeRecorder(10),
			Metrics:                      metrics,
			ConcurrentApplicationUpdates: 5,
		}

		err = r.createOrUpdateInCluster(t.Context(), log.NewEntry(log.StandardLogger()), appSet, desiredApps)
		require.ErrorIs(t, err, patchErr)
	})
}

func TestCreateOrUpdateInCluster_ContextCancellation(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	appSet := v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
	}
	existingApp := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "app1",
			Namespace:       "namespace",
			ResourceVersion: "1",
		},
		Spec: v1alpha1.ApplicationSpec{Project: "old"},
	}
	desiredApp := v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app1",
			Namespace: "namespace",
		},
		Spec: v1alpha1.ApplicationSpec{Project: "new"},
	}

	t.Run("context canceled on patch is returned directly", func(t *testing.T) {
		initObjs := []crtclient.Object{&appSet}
		app := existingApp.DeepCopy()
		err = controllerutil.SetControllerReference(&appSet, app, scheme)
		require.NoError(t, err)
		initObjs = append(initObjs, app)

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(initObjs...).
			WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).
			WithInterceptorFuncs(interceptor.Funcs{
				Patch: func(_ context.Context, _ crtclient.WithWatch, _ crtclient.Object, _ crtclient.Patch, _ ...crtclient.PatchOption) error {
					return context.Canceled
				},
			}).
			Build()
		metrics := appsetmetrics.NewFakeAppsetMetrics()

		r := ApplicationSetReconciler{
			Client:   fakeClient,
			Scheme:   scheme,
			Recorder: record.NewFakeRecorder(10),
			Metrics:  metrics,
		}

		err = r.createOrUpdateInCluster(t.Context(), log.NewEntry(log.StandardLogger()), appSet, []v1alpha1.Application{desiredApp})
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("context deadline exceeded on patch is returned directly", func(t *testing.T) {
		initObjs := []crtclient.Object{&appSet}
		app := existingApp.DeepCopy()
		err = controllerutil.SetControllerReference(&appSet, app, scheme)
		require.NoError(t, err)
		initObjs = append(initObjs, app)

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(initObjs...).
			WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).
			WithInterceptorFuncs(interceptor.Funcs{
				Patch: func(_ context.Context, _ crtclient.WithWatch, _ crtclient.Object, _ crtclient.Patch, _ ...crtclient.PatchOption) error {
					return context.DeadlineExceeded
				},
			}).
			Build()
		metrics := appsetmetrics.NewFakeAppsetMetrics()

		r := ApplicationSetReconciler{
			Client:   fakeClient,
			Scheme:   scheme,
			Recorder: record.NewFakeRecorder(10),
			Metrics:  metrics,
		}

		err = r.createOrUpdateInCluster(t.Context(), log.NewEntry(log.StandardLogger()), appSet, []v1alpha1.Application{desiredApp})
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("non-context error is collected and returned after all goroutines finish", func(t *testing.T) {
		initObjs := []crtclient.Object{&appSet}
		app := existingApp.DeepCopy()
		err = controllerutil.SetControllerReference(&appSet, app, scheme)
		require.NoError(t, err)
		initObjs = append(initObjs, app)

		patchErr := errors.New("some patch error")
		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(initObjs...).
			WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).
			WithInterceptorFuncs(interceptor.Funcs{
				Patch: func(_ context.Context, _ crtclient.WithWatch, _ crtclient.Object, _ crtclient.Patch, _ ...crtclient.PatchOption) error {
					return patchErr
				},
			}).
			Build()
		metrics := appsetmetrics.NewFakeAppsetMetrics()

		r := ApplicationSetReconciler{
			Client:   fakeClient,
			Scheme:   scheme,
			Recorder: record.NewFakeRecorder(10),
			Metrics:  metrics,
		}

		err = r.createOrUpdateInCluster(t.Context(), log.NewEntry(log.StandardLogger()), appSet, []v1alpha1.Application{desiredApp})
		require.ErrorIs(t, err, patchErr)
	})

	t.Run("context canceled on create is returned directly", func(t *testing.T) {
		initObjs := []crtclient.Object{&appSet}

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(initObjs...).
			WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).
			WithInterceptorFuncs(interceptor.Funcs{
				Create: func(_ context.Context, _ crtclient.WithWatch, _ crtclient.Object, _ ...crtclient.CreateOption) error {
					return context.Canceled
				},
			}).
			Build()
		metrics := appsetmetrics.NewFakeAppsetMetrics()

		r := ApplicationSetReconciler{
			Client:   fakeClient,
			Scheme:   scheme,
			Recorder: record.NewFakeRecorder(10),
			Metrics:  metrics,
		}

		newApp := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{Name: "newapp", Namespace: "namespace"},
			Spec:       v1alpha1.ApplicationSpec{Project: "default"},
		}
		err = r.createOrUpdateInCluster(t.Context(), log.NewEntry(log.StandardLogger()), appSet, []v1alpha1.Application{newApp})
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestDeleteInCluster_ContextCancellation(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)
	err = corev1.AddToScheme(scheme)
	require.NoError(t, err)

	appSet := v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
	}
	existingApp := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "delete-me",
			Namespace:       "namespace",
			ResourceVersion: "1",
		},
		Spec: v1alpha1.ApplicationSpec{Project: "project"},
	}

	makeReconciler := func(t *testing.T, fakeClient crtclient.Client) ApplicationSetReconciler {
		t.Helper()
		kubeclientset := kubefake.NewClientset()
		clusterInformer, err := settings.NewClusterInformer(kubeclientset, "namespace")
		require.NoError(t, err)
		cancel := startAndSyncInformer(t, clusterInformer)
		t.Cleanup(cancel)
		return ApplicationSetReconciler{
			Client:          fakeClient,
			Scheme:          scheme,
			Recorder:        record.NewFakeRecorder(10),
			KubeClientset:   kubeclientset,
			Metrics:         appsetmetrics.NewFakeAppsetMetrics(),
			ClusterInformer: clusterInformer,
		}
	}

	t.Run("context canceled on delete is returned directly", func(t *testing.T) {
		app := existingApp.DeepCopy()
		err = controllerutil.SetControllerReference(&appSet, app, scheme)
		require.NoError(t, err)

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(&appSet, app).
			WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).
			WithInterceptorFuncs(interceptor.Funcs{
				Delete: func(_ context.Context, _ crtclient.WithWatch, _ crtclient.Object, _ ...crtclient.DeleteOption) error {
					return context.Canceled
				},
			}).
			Build()

		r := makeReconciler(t, fakeClient)
		err = r.deleteInCluster(t.Context(), log.NewEntry(log.StandardLogger()), appSet, []v1alpha1.Application{})
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("context deadline exceeded on delete is returned directly", func(t *testing.T) {
		app := existingApp.DeepCopy()
		err = controllerutil.SetControllerReference(&appSet, app, scheme)
		require.NoError(t, err)

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(&appSet, app).
			WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).
			WithInterceptorFuncs(interceptor.Funcs{
				Delete: func(_ context.Context, _ crtclient.WithWatch, _ crtclient.Object, _ ...crtclient.DeleteOption) error {
					return context.DeadlineExceeded
				},
			}).
			Build()

		r := makeReconciler(t, fakeClient)
		err = r.deleteInCluster(t.Context(), log.NewEntry(log.StandardLogger()), appSet, []v1alpha1.Application{})
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("non-context delete error is collected and returned", func(t *testing.T) {
		app := existingApp.DeepCopy()
		err = controllerutil.SetControllerReference(&appSet, app, scheme)
		require.NoError(t, err)

		deleteErr := errors.New("delete failed")
		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(&appSet, app).
			WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).
			WithInterceptorFuncs(interceptor.Funcs{
				Delete: func(_ context.Context, _ crtclient.WithWatch, _ crtclient.Object, _ ...crtclient.DeleteOption) error {
					return deleteErr
				},
			}).
			Build()

		r := makeReconciler(t, fakeClient)
		err = r.deleteInCluster(t.Context(), log.NewEntry(log.StandardLogger()), appSet, []v1alpha1.Application{})
		require.ErrorIs(t, err, deleteErr)
	})
}

func TestRemoveFinalizerOnInvalidDestination_FinalizerTypes(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)
	err = corev1.AddToScheme(scheme)
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

			initObjs := []crtclient.Object{&app, &appSet, secret}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()

			objects := append([]runtime.Object{}, secret)
			kubeclientset := kubefake.NewClientset(objects...)
			metrics := appsetmetrics.NewFakeAppsetMetrics()

			settingsMgr := settings.NewSettingsManager(t.Context(), kubeclientset, "argocd")
			// Initialize the settings manager to ensure cluster cache is ready
			_ = settingsMgr.ResyncInformers()
			argodb := db.NewDB("argocd", settingsMgr, kubeclientset)

			clusterInformer, err := settings.NewClusterInformer(kubeclientset, "namespace")
			require.NoError(t, err)

			defer startAndSyncInformer(t, clusterInformer)()

			r := ApplicationSetReconciler{
				Client:        client,
				Scheme:        scheme,
				Recorder:      record.NewFakeRecorder(10),
				KubeClientset: kubeclientset,
				Metrics:       metrics,
				ArgoDB:        argodb,
			}
			clusterList, err := utils.ListClusters(clusterInformer)
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

			// App object passed in as a parameter should have the expected finalizers
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
	err = corev1.AddToScheme(scheme)
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

			initObjs := []crtclient.Object{&app, &appSet, secret}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()

			kubeclientset := getDefaultTestClientSet(secret)
			metrics := appsetmetrics.NewFakeAppsetMetrics()

			settingsMgr := settings.NewSettingsManager(t.Context(), kubeclientset, "argocd")
			// Initialize the settings manager to ensure cluster cache is ready
			_ = settingsMgr.ResyncInformers()
			argodb := db.NewDB("argocd", settingsMgr, kubeclientset)

			clusterInformer, err := settings.NewClusterInformer(kubeclientset, "argocd")
			require.NoError(t, err)

			defer startAndSyncInformer(t, clusterInformer)()

			r := ApplicationSetReconciler{
				Client:        client,
				Scheme:        scheme,
				Recorder:      record.NewFakeRecorder(10),
				KubeClientset: kubeclientset,
				Metrics:       metrics,
				ArgoDB:        argodb,
			}

			clusterList, err := utils.ListClusters(clusterInformer)
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
	err = corev1.AddToScheme(scheme)
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

		kubeclientset := kubefake.NewClientset()
		clusterInformer, err := settings.NewClusterInformer(kubeclientset, "namespace")
		require.NoError(t, err)

		defer startAndSyncInformer(t, clusterInformer)()

		r := ApplicationSetReconciler{
			Client:          client,
			Scheme:          scheme,
			Recorder:        record.NewFakeRecorder(len(initObjs) + len(c.expected)),
			KubeClientset:   kubeclientset,
			Metrics:         metrics,
			ClusterInformer: clusterInformer,
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

	generatorMock0 := &mocks.Generator{}
	generatorMock0.EXPECT().GetRequeueAfter(&generator).
		Return(generators.NoRequeueAfter)

	generatorMock1 := &mocks.Generator{}
	generatorMock1.EXPECT().GetRequeueAfter(&generator).
		Return(time.Duration(1) * time.Second)

	generatorMock10 := &mocks.Generator{}
	generatorMock10.EXPECT().GetRequeueAfter(&generator).
		Return(time.Duration(10) * time.Second)

	r := ApplicationSetReconciler{
		Client:   client,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(0),
		Metrics:  metrics,
		Generators: map[string]generators.Generator{
			"List":     generatorMock10,
			"Git":      generatorMock1,
			"Clusters": generatorMock1,
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

	generatorMock := &mocks.Generator{}
	generatorMock.EXPECT().GetTemplate(&generator).
		Return(&v1alpha1.ApplicationSetTemplate{})
	generatorMock.EXPECT().GenerateParams(&generator, mock.AnythingOfType("*v1alpha1.ApplicationSet"), mock.Anything).
		Return([]map[string]any{}, errors.New("Simulated error generating params that could be related to an external service/API call"))

	metrics := appsetmetrics.NewFakeAppsetMetrics()

	r := ApplicationSetReconciler{
		Client:   client,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(0),
		Generators: map[string]generators.Generator{
			"PullRequest": generatorMock,
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
			ClusterResourceWhitelist: []v1alpha1.ClusterResourceRestrictionItem{
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
		validationErrors map[string]error
	}{
		{
			name: "valid app should return true",
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app",
					},
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
			validationErrors: map[string]error{},
		},
		{
			name: "can't have both name and server defined",
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app",
					},
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
			validationErrors: map[string]error{"app": errors.New("application destination spec is invalid: application destination can't have both name and server defined: my-cluster my-server")},
		},
		{
			name: "project mismatch should return error",
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app",
					},
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
			validationErrors: map[string]error{"app": errors.New("application references project DOES-NOT-EXIST which does not exist")},
		},
		{
			name: "valid app should return true",
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app",
					},
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
			validationErrors: map[string]error{},
		},
		{
			name: "cluster should match",
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app",
					},
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
			validationErrors: map[string]error{"app": errors.New("application destination spec is invalid: there are no clusters with this name: nonexistent-cluster")},
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
	err = corev1.AddToScheme(scheme)
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

	clusterInformer, err := settings.NewClusterInformer(kubeclientset, "argocd")
	require.NoError(t, err)

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
		ClusterInformer: clusterInformer,
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
	kubeclientset := kubefake.NewClientset([]runtime.Object{}...)
	someTime := &metav1.Time{Time: time.Now().Add(-5 * time.Minute)}
	existingParameterGeneratedCondition := getParametersGeneratedCondition(true, "")
	existingParameterGeneratedCondition.LastTransitionTime = someTime

	for _, c := range []struct {
		name                string
		appset              v1alpha1.ApplicationSet
		condition           v1alpha1.ApplicationSetCondition
		parametersGenerated bool
		testfunc            func(t *testing.T, conditions []v1alpha1.ApplicationSetCondition)
	}{
		{
			name: "has parameters generated condition when false",
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
			condition: v1alpha1.ApplicationSetCondition{
				Type:    v1alpha1.ApplicationSetConditionResourcesUpToDate,
				Message: "This is a message",
				Reason:  "test",
				Status:  v1alpha1.ApplicationSetConditionStatusFalse,
			},
			parametersGenerated: false,
			testfunc: func(t *testing.T, conditions []v1alpha1.ApplicationSetCondition) {
				t.Helper()
				require.Len(t, conditions, 2)

				// Conditions are ordered by type, so the order is deterministic
				assert.Equal(t, v1alpha1.ApplicationSetConditionParametersGenerated, conditions[0].Type)
				assert.Equal(t, v1alpha1.ApplicationSetConditionStatusFalse, conditions[0].Status)

				assert.Equal(t, v1alpha1.ApplicationSetConditionResourcesUpToDate, conditions[1].Type)
				assert.Equal(t, v1alpha1.ApplicationSetConditionStatusFalse, conditions[1].Status)
				assert.Equal(t, "test", conditions[1].Reason)
			},
		},
		{
			name: "parameters generated condition is used when specified",
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
			condition: v1alpha1.ApplicationSetCondition{
				Type:    v1alpha1.ApplicationSetConditionParametersGenerated,
				Message: "This is a message",
				Reason:  "test",
				Status:  v1alpha1.ApplicationSetConditionStatusFalse,
			},
			parametersGenerated: true,
			testfunc: func(t *testing.T, conditions []v1alpha1.ApplicationSetCondition) {
				t.Helper()
				require.Len(t, conditions, 1)

				assert.Equal(t, v1alpha1.ApplicationSetConditionParametersGenerated, conditions[0].Type)
				assert.Equal(t, v1alpha1.ApplicationSetConditionStatusFalse, conditions[0].Status)
				assert.Equal(t, "test", conditions[0].Reason)
			},
		},
		{
			name: "has parameter conditions when true",
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
			condition: v1alpha1.ApplicationSetCondition{
				Type:    v1alpha1.ApplicationSetConditionResourcesUpToDate,
				Message: "This is a message",
				Reason:  "test",
				Status:  v1alpha1.ApplicationSetConditionStatusFalse,
			},
			parametersGenerated: true,
			testfunc: func(t *testing.T, conditions []v1alpha1.ApplicationSetCondition) {
				t.Helper()
				require.Len(t, conditions, 2)

				// Conditions are ordered by type, so the order is deterministic
				assert.Equal(t, v1alpha1.ApplicationSetConditionParametersGenerated, conditions[0].Type)
				assert.Equal(t, v1alpha1.ApplicationSetConditionStatusTrue, conditions[0].Status)

				assert.Equal(t, v1alpha1.ApplicationSetConditionResourcesUpToDate, conditions[1].Type)
				assert.Equal(t, v1alpha1.ApplicationSetConditionStatusFalse, conditions[1].Status)
				assert.Equal(t, "test", conditions[1].Reason)
			},
		},
		{
			name: "resource up to date sets error condition to false",
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
			condition: v1alpha1.ApplicationSetCondition{
				Type:    v1alpha1.ApplicationSetConditionResourcesUpToDate,
				Message: "Completed",
				Reason:  "test",
				Status:  v1alpha1.ApplicationSetConditionStatusTrue,
			},
			testfunc: func(t *testing.T, conditions []v1alpha1.ApplicationSetCondition) {
				t.Helper()
				require.Len(t, conditions, 3)

				assert.Equal(t, v1alpha1.ApplicationSetConditionErrorOccurred, conditions[0].Type)
				assert.Equal(t, v1alpha1.ApplicationSetConditionStatusFalse, conditions[0].Status)
				assert.Equal(t, "test", conditions[0].Reason)
				assert.Equal(t, "Completed", conditions[0].Message)

				assert.Equal(t, v1alpha1.ApplicationSetConditionParametersGenerated, conditions[1].Type)

				assert.Equal(t, v1alpha1.ApplicationSetConditionResourcesUpToDate, conditions[2].Type)
				assert.Equal(t, v1alpha1.ApplicationSetConditionStatusTrue, conditions[2].Status)
				assert.Equal(t, "test", conditions[2].Reason)
				assert.Equal(t, "Completed", conditions[2].Message)
			},
		},
		{
			name: "error condition sets resource up to date to false",
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
			condition: v1alpha1.ApplicationSetCondition{
				Type:    v1alpha1.ApplicationSetConditionErrorOccurred,
				Message: "Error",
				Reason:  "test",
				Status:  v1alpha1.ApplicationSetConditionStatusTrue,
			},
			testfunc: func(t *testing.T, conditions []v1alpha1.ApplicationSetCondition) {
				t.Helper()
				require.Len(t, conditions, 3)

				assert.Equal(t, v1alpha1.ApplicationSetConditionErrorOccurred, conditions[0].Type)
				assert.Equal(t, v1alpha1.ApplicationSetConditionStatusTrue, conditions[0].Status)
				assert.Equal(t, "test", conditions[0].Reason)
				assert.Equal(t, "Error", conditions[0].Message)

				assert.Equal(t, v1alpha1.ApplicationSetConditionParametersGenerated, conditions[1].Type)

				assert.Equal(t, v1alpha1.ApplicationSetConditionResourcesUpToDate, conditions[2].Type)
				assert.Equal(t, v1alpha1.ApplicationSetConditionStatusFalse, conditions[2].Status)
				assert.Equal(t, v1alpha1.ApplicationSetReasonErrorOccurred, conditions[2].Reason)
				assert.Equal(t, "Error", conditions[2].Message)
			},
		},
		{
			name: "updating an unchanged condition does not mutate existing conditions",
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
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
					},
					Template: v1alpha1.ApplicationSetTemplate{},
				},
				Status: v1alpha1.ApplicationSetStatus{
					Conditions: []v1alpha1.ApplicationSetCondition{
						{
							Type:               v1alpha1.ApplicationSetConditionErrorOccurred,
							Message:            "existing",
							LastTransitionTime: someTime,
						},
						existingParameterGeneratedCondition,
						{
							Type:               v1alpha1.ApplicationSetConditionResourcesUpToDate,
							Message:            "existing",
							Status:             v1alpha1.ApplicationSetConditionStatusFalse,
							LastTransitionTime: someTime,
						},
						{
							Type:               v1alpha1.ApplicationSetConditionRolloutProgressing,
							Message:            "existing",
							LastTransitionTime: someTime,
						},
					},
				},
			},
			condition: v1alpha1.ApplicationSetCondition{
				Type:    v1alpha1.ApplicationSetConditionResourcesUpToDate,
				Message: "existing",
				Status:  v1alpha1.ApplicationSetConditionStatusFalse,
			},
			parametersGenerated: true,
			testfunc: func(t *testing.T, conditions []v1alpha1.ApplicationSetCondition) {
				t.Helper()
				require.Len(t, conditions, 4)

				assert.Equal(t, v1alpha1.ApplicationSetConditionErrorOccurred, conditions[0].Type)
				assert.Equal(t, someTime, conditions[0].LastTransitionTime)

				assert.Equal(t, v1alpha1.ApplicationSetConditionParametersGenerated, conditions[1].Type)
				assert.Equal(t, someTime, conditions[1].LastTransitionTime)

				assert.Equal(t, v1alpha1.ApplicationSetConditionResourcesUpToDate, conditions[2].Type)
				assert.Equal(t, someTime, conditions[2].LastTransitionTime)

				assert.Equal(t, v1alpha1.ApplicationSetConditionRolloutProgressing, conditions[3].Type)
				assert.Equal(t, someTime, conditions[3].LastTransitionTime)
			},
		},
		{
			name: "progressing conditions is removed when AppSet is not configured",
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
					// Strategy removed
					// Strategy: &v1alpha1.ApplicationSetStrategy{
					// 	Type:        "RollingSync",
					// 	RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
					// },
					Template: v1alpha1.ApplicationSetTemplate{},
				},
				Status: v1alpha1.ApplicationSetStatus{
					Conditions: []v1alpha1.ApplicationSetCondition{
						{
							Type:               v1alpha1.ApplicationSetConditionErrorOccurred,
							Message:            "existing",
							LastTransitionTime: someTime,
						},
						existingParameterGeneratedCondition,
						{
							Type:               v1alpha1.ApplicationSetConditionResourcesUpToDate,
							Message:            "existing",
							Status:             v1alpha1.ApplicationSetConditionStatusFalse,
							LastTransitionTime: someTime,
						},
						{
							Type:               v1alpha1.ApplicationSetConditionRolloutProgressing,
							Message:            "existing",
							LastTransitionTime: someTime,
						},
					},
				},
			},
			condition: v1alpha1.ApplicationSetCondition{
				Type:    v1alpha1.ApplicationSetConditionResourcesUpToDate,
				Message: "existing",
				Status:  v1alpha1.ApplicationSetConditionStatusFalse,
			},
			parametersGenerated: true,
			testfunc: func(t *testing.T, conditions []v1alpha1.ApplicationSetCondition) {
				t.Helper()
				require.Len(t, conditions, 3)
				for _, c := range conditions {
					assert.NotEqual(t, v1alpha1.ApplicationSetConditionRolloutProgressing, c.Type)
				}
			},
		},
		{
			name: "progressing conditions is ignored when AppSet is not configured",
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
					// Strategy removed
					// Strategy: &v1alpha1.ApplicationSetStrategy{
					// 	Type:        "RollingSync",
					// 	RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
					// },
					Template: v1alpha1.ApplicationSetTemplate{},
				},
				Status: v1alpha1.ApplicationSetStatus{
					Conditions: []v1alpha1.ApplicationSetCondition{
						{
							Type:               v1alpha1.ApplicationSetConditionErrorOccurred,
							Message:            "existing",
							LastTransitionTime: someTime,
						},
						existingParameterGeneratedCondition,
						{
							Type:               v1alpha1.ApplicationSetConditionResourcesUpToDate,
							Message:            "existing",
							Status:             v1alpha1.ApplicationSetConditionStatusFalse,
							LastTransitionTime: someTime,
						},
					},
				},
			},
			condition: v1alpha1.ApplicationSetCondition{
				Type:    v1alpha1.ApplicationSetConditionRolloutProgressing,
				Message: "do not add me",
				Status:  v1alpha1.ApplicationSetConditionStatusTrue,
			},
			parametersGenerated: true,
			testfunc: func(t *testing.T, conditions []v1alpha1.ApplicationSetCondition) {
				t.Helper()
				require.Len(t, conditions, 3)
				for _, c := range conditions {
					assert.NotEqual(t, v1alpha1.ApplicationSetConditionRolloutProgressing, c.Type)
				}
			},
		},
		{
			name: "progressing conditions is updated correctly when configured",
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
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
					},
					Template: v1alpha1.ApplicationSetTemplate{},
				},
				Status: v1alpha1.ApplicationSetStatus{
					Conditions: []v1alpha1.ApplicationSetCondition{
						{
							Type:               v1alpha1.ApplicationSetConditionErrorOccurred,
							Message:            "existing",
							LastTransitionTime: someTime,
						},
						existingParameterGeneratedCondition,
						{
							Type:               v1alpha1.ApplicationSetConditionResourcesUpToDate,
							Message:            "existing",
							Status:             v1alpha1.ApplicationSetConditionStatusFalse,
							LastTransitionTime: someTime,
						},
						{
							Type:    v1alpha1.ApplicationSetConditionRolloutProgressing,
							Message: "old value",
							Status:  v1alpha1.ApplicationSetConditionStatusTrue,
						},
					},
				},
			},
			condition: v1alpha1.ApplicationSetCondition{
				Type:    v1alpha1.ApplicationSetConditionRolloutProgressing,
				Message: "new value",
				Status:  v1alpha1.ApplicationSetConditionStatusFalse,
			},
			parametersGenerated: true,
			testfunc: func(t *testing.T, conditions []v1alpha1.ApplicationSetCondition) {
				t.Helper()
				require.Len(t, conditions, 4)

				assert.Equal(t, v1alpha1.ApplicationSetConditionRolloutProgressing, conditions[3].Type)
				assert.Equal(t, v1alpha1.ApplicationSetConditionStatusFalse, conditions[3].Status)
				assert.Equal(t, "new value", conditions[3].Message)
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&c.appset).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).WithStatusSubresource(&c.appset).Build()
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

			err = r.setApplicationSetStatusCondition(t.Context(), &c.appset, c.condition, c.parametersGenerated)
			require.NoError(t, err)

			c.testfunc(t, c.appset.Status.Conditions)
		})
	}
}

func applicationsUpdateSyncPolicyTest(t *testing.T, applicationsSyncPolicy v1alpha1.ApplicationsSyncPolicy, recordBuffer int, allowPolicyOverride bool) v1alpha1.Application {
	t.Helper()
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)
	err = corev1.AddToScheme(scheme)
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

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet, &defaultProject, secret).WithStatusSubresource(&appSet).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
	metrics := appsetmetrics.NewFakeAppsetMetrics()

	argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)
	clusterInformer, err := settings.NewClusterInformer(kubeclientset, "argocd")
	require.NoError(t, err)

	defer startAndSyncInformer(t, clusterInformer)()

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
		ClusterInformer:      clusterInformer,
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

// TestReconcilePopulatesResourcesStatusOnFirstRun verifies that status.resources and status.resourcesCount
// are populated after the first reconcile, when applications are created.
func TestReconcilePopulatesResourcesStatusOnFirstRun(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)
	err = corev1.AddToScheme(scheme)
	require.NoError(t, err)

	defaultProject := v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "argocd"},
		Spec:       v1alpha1.AppProjectSpec{SourceRepos: []string{"*"}, Destinations: []v1alpha1.ApplicationDestination{{Namespace: "*", Server: "https://good-cluster"}}},
	}
	applicationsSyncPolicy := v1alpha1.ApplicationsSyncPolicySync
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
			"name":   []byte("good-cluster"),
			"server": []byte("https://good-cluster"),
			"config": []byte("{\"username\":\"foo\",\"password\":\"foo\"}"),
		},
	}

	kubeclientset := getDefaultTestClientSet(secret)
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(&appSet, &defaultProject, secret).
		WithStatusSubresource(&appSet).
		WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).
		Build()
	metrics := appsetmetrics.NewFakeAppsetMetrics()

	argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)
	clusterInformer, err := settings.NewClusterInformer(kubeclientset, "argocd")
	require.NoError(t, err)

	defer startAndSyncInformer(t, clusterInformer)()

	r := ApplicationSetReconciler{
		Client:          client,
		Scheme:          scheme,
		Renderer:        &utils.Render{},
		Recorder:        record.NewFakeRecorder(1),
		Generators:      map[string]generators.Generator{"List": generators.NewListGenerator()},
		ArgoDB:          argodb,
		ArgoCDNamespace: "argocd",
		KubeClientset:   kubeclientset,
		Policy:          v1alpha1.ApplicationsSyncPolicySync,
		Metrics:         metrics,
		ClusterInformer: clusterInformer,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "argocd", Name: "name"},
	}

	_, err = r.Reconcile(t.Context(), req)
	require.NoError(t, err)

	var retrievedAppSet v1alpha1.ApplicationSet
	err = r.Get(t.Context(), crtclient.ObjectKey{Namespace: "argocd", Name: "name"}, &retrievedAppSet)
	require.NoError(t, err)

	assert.Len(t, retrievedAppSet.Status.Resources, 1, "status.resources should have 1 item after first reconcile")
	assert.Equal(t, int64(1), retrievedAppSet.Status.ResourcesCount, "status.resourcesCount should be 1 after first reconcile")
	assert.Equal(t, "good-cluster", retrievedAppSet.Status.Resources[0].Name)
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
	err = corev1.AddToScheme(scheme)
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

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet, &defaultProject, secret).WithStatusSubresource(&appSet).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
	metrics := appsetmetrics.NewFakeAppsetMetrics()

	argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)

	clusterInformer, err := settings.NewClusterInformer(kubeclientset, "argocd")
	require.NoError(t, err)

	defer startAndSyncInformer(t, clusterInformer)()

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
		ClusterInformer:      clusterInformer,
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
	err = corev1.AddToScheme(scheme)
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

			clusterInformer, err := settings.NewClusterInformer(kubeclientset, "argocd")
			require.NoError(t, err)

			defer startAndSyncInformer(t, clusterInformer)()

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
				ClusterInformer: clusterInformer,
				Metrics:         metrics,
			}

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "argocd",
					Name:      "name",
				},
			}
			ctx := t.Context()
			// Check if the application is created
			res, err := r.Reconcile(ctx, req)
			require.NoError(t, err)
			assert.Equal(t, time.Duration(0), res.RequeueAfter)

			var app v1alpha1.Application
			err = r.Get(ctx, crtclient.ObjectKey{Namespace: "argocd", Name: "my-app"}, &app)
			require.NoError(t, err)
			assert.Equal(t, "value", app.Annotations["key"])

			// Check if the Application is updated
			app.Annotations["key"] = "edited"
			err = r.Update(ctx, &app)
			require.NoError(t, err)

			res, err = r.Reconcile(ctx, req)
			require.NoError(t, err)
			assert.Equal(t, time.Duration(0), res.RequeueAfter)

			err = r.Get(ctx, crtclient.ObjectKey{Namespace: "argocd", Name: "my-app"}, &app)
			require.NoError(t, err)

			if c.allowedUpdate {
				assert.Equal(t, "value", app.Annotations["key"])
			} else {
				assert.Equal(t, "edited", app.Annotations["key"])
			}

			// Check if the Application is deleted
			err = r.Get(ctx, crtclient.ObjectKey{Namespace: "argocd", Name: "name"}, &appSet)
			require.NoError(t, err)
			appSet.Spec.Generators[0] = v1alpha1.ApplicationSetGenerator{
				List: &v1alpha1.ListGenerator{
					Elements: []apiextensionsv1.JSON{},
				},
			}
			err = r.Update(ctx, &appSet)
			require.NoError(t, err)

			res, err = r.Reconcile(ctx, req)
			require.NoError(t, err)
			assert.Equal(t, time.Duration(0), res.RequeueAfter)

			err = r.Get(ctx, crtclient.ObjectKey{Namespace: "argocd", Name: "my-app"}, &app)
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

	kubeclientset := kubefake.NewClientset([]runtime.Object{}...)

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
					Status:      v1alpha1.ProgressiveSyncHealthy,
				},
			},
			expectedAppStatuses: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application: "app1",
					Message:     "testing SetApplicationSetApplicationStatus to Healthy",
					Status:      v1alpha1.ProgressiveSyncHealthy,
				},
			},
		},
		{
			name: "order appstatus by name",
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
					Application: "app2",
					Message:     "testing SetApplicationSetApplicationStatus to Healthy",
					Status:      v1alpha1.ProgressiveSyncHealthy,
				},
				{
					Application: "app1",
					Message:     "testing SetApplicationSetApplicationStatus to Healthy",
					Status:      v1alpha1.ProgressiveSyncHealthy,
				},
			},
			expectedAppStatuses: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application: "app1",
					Message:     "testing SetApplicationSetApplicationStatus to Healthy",
					Status:      v1alpha1.ProgressiveSyncHealthy,
				},
				{
					Application: "app2",
					Message:     "testing SetApplicationSetApplicationStatus to Healthy",
					Status:      v1alpha1.ProgressiveSyncHealthy,
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
							Status:      v1alpha1.ProgressiveSyncHealthy,
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

func TestUpdateResourceStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	for _, cc := range []struct {
		name                    string
		appSet                  v1alpha1.ApplicationSet
		apps                    []v1alpha1.Application
		expectedResources       []v1alpha1.ResourceStatus
		maxResourcesStatusCount int
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
								Message: "this is progressing",
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
		{
			name: "truncates resources status list to",
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
								Message: "this is progressing",
							},
						},
						{
							Name:   "app2",
							Status: v1alpha1.SyncStatusCodeOutOfSync,
							Health: &v1alpha1.HealthStatus{
								Status:  health.HealthStatusProgressing,
								Message: "this is progressing",
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
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app2",
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
			maxResourcesStatusCount: 1,
		},
	} {
		t.Run(cc.name, func(t *testing.T) {
			kubeclientset := kubefake.NewClientset([]runtime.Object{}...)

			client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&cc.appSet).WithObjects(&cc.appSet).Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics()

			argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)

			r := ApplicationSetReconciler{
				Client:                  client,
				Scheme:                  scheme,
				Recorder:                record.NewFakeRecorder(1),
				Generators:              map[string]generators.Generator{},
				ArgoDB:                  argodb,
				KubeClientset:           kubeclientset,
				Metrics:                 metrics,
				MaxResourcesStatusCount: cc.maxResourcesStatusCount,
			}

			err := r.updateResourcesStatus(t.Context(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.apps)

			require.NoError(t, err, "expected no errors, but errors occurred")
			assert.Equal(t, cc.expectedResources, cc.appSet.Status.Resources, "expected resources did not match actual")
		})
	}
}

func generateNAppResourceStatuses(n int) []v1alpha1.ResourceStatus {
	var r []v1alpha1.ResourceStatus
	for i := range n {
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
	for i := range n {
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
			kubeclientset := kubefake.NewClientset([]runtime.Object{}...)

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
						Status: health.HealthStatusUnknown,
					},
				}},
				ObjectNew: &v1alpha1.Application{Status: v1alpha1.ApplicationStatus{
					Health: v1alpha1.AppHealthStatus{
						Status: health.HealthStatusHealthy,
					},
				}},
			},
			enableProgressiveSyncs: true,
		}, want: true},
		{name: "ApplicationSyncStatusDiff", args: args{
			e: event.UpdateEvent{
				ObjectOld: &v1alpha1.Application{Status: v1alpha1.ApplicationStatus{
					Sync: v1alpha1.SyncStatus{
						Status: v1alpha1.SyncStatusCodeOutOfSync,
					},
				}},
				ObjectNew: &v1alpha1.Application{Status: v1alpha1.ApplicationStatus{
					Sync: v1alpha1.SyncStatus{
						Status: v1alpha1.SyncStatusCodeSynced,
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
							TargetRevisions: []string{"current"},
						},
					},
				},
			},
			expectedStatus: v1alpha1.ApplicationSetStatus{
				ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
					{
						TargetRevisions: []string{"current"},
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
		{
			name:      "deletionTimestamp present when progressive sync enabled",
			appSetOld: buildAppSet(map[string]string{}),
			appSetNew: &v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				},
			},
			enableProgressiveSyncs: true,
			want:                   true,
		},
		{
			name:      "deletionTimestamp present when progressive sync disabled",
			appSetOld: buildAppSet(map[string]string{}),
			appSetNew: &v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				},
			},
			enableProgressiveSyncs: false,
			want:                   true,
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
								Status:      v1alpha1.ProgressiveSyncHealthy,
							},
						},
					},
				},
				appSetNew: &v1alpha1.ApplicationSet{
					Status: v1alpha1.ApplicationSetStatus{
						ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
							{
								Application: "app1",
								Status:      v1alpha1.ProgressiveSyncWaiting,
							},
						},
					},
				},
				enableProgressiveSyncs: true,
			},
			want: true,
		},
		{
			name: "ApplicationSetWithDeletionTimestamp",
			args: args{
				appSetOld: &v1alpha1.ApplicationSet{
					Status: v1alpha1.ApplicationSetStatus{
						ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
							{
								Application: "app1",
								Status:      v1alpha1.ProgressiveSyncHealthy,
							},
						},
					},
				},
				appSetNew: &v1alpha1.ApplicationSet{
					ObjectMeta: metav1.ObjectMeta{
						DeletionTimestamp: &metav1.Time{Time: time.Now()},
					},
					Status: v1alpha1.ApplicationSetStatus{
						ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
							{
								Application: "app1",
								Status:      v1alpha1.ProgressiveSyncWaiting,
							},
						},
					},
				},
				enableProgressiveSyncs: false,
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

func TestFirstAppError(t *testing.T) {
	errA := errors.New("error from app-a")
	errB := errors.New("error from app-b")
	errC := errors.New("error from app-c")

	t.Run("returns nil for empty map", func(t *testing.T) {
		assert.NoError(t, firstAppError(map[string]error{}))
	})

	t.Run("returns the single error", func(t *testing.T) {
		assert.ErrorIs(t, firstAppError(map[string]error{"app-a": errA}), errA)
	})

	t.Run("returns error from lexicographically first app name", func(t *testing.T) {
		appErrors := map[string]error{
			"app-c": errC,
			"app-a": errA,
			"app-b": errB,
		}
		assert.ErrorIs(t, firstAppError(appErrors), errA)
	})

	t.Run("result is stable across multiple calls with same input", func(t *testing.T) {
		appErrors := map[string]error{
			"app-c": errC,
			"app-a": errA,
			"app-b": errB,
		}
		for range 10 {
			assert.ErrorIs(t, firstAppError(appErrors), errA, "firstAppError must return the same error on every call")
		}
	})
}

func TestReconcileAddsFinalizer_WhenDeletionOrderReverse(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	kubeclientset := kubefake.NewClientset([]runtime.Object{}...)

	for _, cc := range []struct {
		name                   string
		appSet                 v1alpha1.ApplicationSet
		progressiveSyncEnabled bool
		expectedFinalizers     []string
	}{
		{
			name: "adds finalizer when DeletionOrder is Reverse",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-appset",
					Namespace: "argocd",
					// No finalizers initially
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
											Values:   []string{"dev"},
										},
									},
								},
							},
						},
						DeletionOrder: ReverseDeletionOrder,
					},
					Template: v1alpha1.ApplicationSetTemplate{},
				},
			},
			progressiveSyncEnabled: true,
			expectedFinalizers:     []string{v1alpha1.ResourcesFinalizerName},
		},
		{
			name: "does not add finalizer when already exists and DeletionOrder is Reverse",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-appset",
					Namespace: "argocd",
					Finalizers: []string{
						v1alpha1.ResourcesFinalizerName,
					},
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
											Values:   []string{"dev"},
										},
									},
								},
							},
						},
						DeletionOrder: ReverseDeletionOrder,
					},
					Template: v1alpha1.ApplicationSetTemplate{},
				},
			},
			progressiveSyncEnabled: true,
			expectedFinalizers:     []string{v1alpha1.ResourcesFinalizerName},
		},
		{
			name: "does not add finalizer when DeletionOrder is AllAtOnce",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-appset",
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
											Values:   []string{"dev"},
										},
									},
								},
							},
						},
						DeletionOrder: AllAtOnceDeletionOrder,
					},
					Template: v1alpha1.ApplicationSetTemplate{},
				},
			},
			progressiveSyncEnabled: true,
			expectedFinalizers:     nil,
		},
		{
			name: "does not add finalizer when DeletionOrder is not set",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-appset",
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
											Values:   []string{"dev"},
										},
									},
								},
							},
						},
					},
					Template: v1alpha1.ApplicationSetTemplate{},
				},
			},
			progressiveSyncEnabled: true,
			expectedFinalizers:     nil,
		},
		{
			name: "does not add finalizer when progressive sync not enabled",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-appset",
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
											Values:   []string{"dev"},
										},
									},
								},
							},
						},
						DeletionOrder: ReverseDeletionOrder,
					},
					Template: v1alpha1.ApplicationSetTemplate{},
				},
			},
			progressiveSyncEnabled: false,
			expectedFinalizers:     nil,
		},
	} {
		t.Run(cc.name, func(t *testing.T) {
			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(&cc.appSet).
				WithStatusSubresource(&cc.appSet).
				WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).
				Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics()
			argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)

			r := ApplicationSetReconciler{
				Client:                 client,
				Scheme:                 scheme,
				Renderer:               &utils.Render{},
				Recorder:               record.NewFakeRecorder(1),
				Generators:             map[string]generators.Generator{},
				ArgoDB:                 argodb,
				KubeClientset:          kubeclientset,
				Metrics:                metrics,
				EnableProgressiveSyncs: cc.progressiveSyncEnabled,
			}
			r.ProgressiveSyncManager = progressivesync.NewManager(r.Client, &r)

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: cc.appSet.Namespace,
					Name:      cc.appSet.Name,
				},
			}

			// Run reconciliation
			_, err = r.Reconcile(t.Context(), req)
			require.NoError(t, err)

			// Fetch the updated ApplicationSet
			var updatedAppSet v1alpha1.ApplicationSet
			err = r.Get(t.Context(), req.NamespacedName, &updatedAppSet)
			require.NoError(t, err)

			// Verify the finalizers
			assert.Equal(t, cc.expectedFinalizers, updatedAppSet.Finalizers,
				"finalizers should match expected value")
		})
	}
}

func TestReconcileProgressiveSyncDisabled(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	kubeclientset := kubefake.NewClientset([]runtime.Object{}...)

	for _, cc := range []struct {
		name                   string
		appSet                 v1alpha1.ApplicationSet
		enableProgressiveSyncs bool
		expectedAppStatuses    []v1alpha1.ApplicationSetApplicationStatus
	}{
		{
			name: "clears applicationStatus when Progressive Sync is disabled",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-appset",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Generators: []v1alpha1.ApplicationSetGenerator{},
					Template:   v1alpha1.ApplicationSetTemplate{},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "test-appset-guestbook",
							Message:     "Application resource became Healthy, updating status from Progressing to Healthy.",
							Status:      "Healthy",
							Step:        "1",
						},
					},
				},
			},
			enableProgressiveSyncs: false,
			expectedAppStatuses:    nil,
		},
	} {
		t.Run(cc.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&cc.appSet).WithStatusSubresource(&cc.appSet).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics()

			argodb := db.NewDB("argocd", settings.NewSettingsManager(t.Context(), kubeclientset, "argocd"), kubeclientset)

			r := ApplicationSetReconciler{
				Client:                 client,
				Scheme:                 scheme,
				Renderer:               &utils.Render{},
				Recorder:               record.NewFakeRecorder(1),
				Generators:             map[string]generators.Generator{},
				ArgoDB:                 argodb,
				KubeClientset:          kubeclientset,
				Metrics:                metrics,
				EnableProgressiveSyncs: cc.enableProgressiveSyncs,
			}

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: cc.appSet.Namespace,
					Name:      cc.appSet.Name,
				},
			}

			// Run reconciliation
			_, err = r.Reconcile(t.Context(), req)
			require.NoError(t, err)

			// Fetch the updated ApplicationSet
			var updatedAppSet v1alpha1.ApplicationSet
			err = r.Get(t.Context(), req.NamespacedName, &updatedAppSet)
			require.NoError(t, err)

			// Verify the applicationStatus field
			assert.Equal(t, cc.expectedAppStatuses, updatedAppSet.Status.ApplicationStatus, "applicationStatus should match expected value")
		})
	}
}

func startAndSyncInformer(t *testing.T, informer cache.SharedIndexInformer) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	go informer.Run(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		cancel()
		t.Fatal("Timed out waiting for caches to sync")
	}
	return cancel
}
