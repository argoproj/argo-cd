package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
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

	"github.com/argoproj/argo-cd/v2/applicationset/generators"
	"github.com/argoproj/argo-cd/v2/applicationset/generators/mocks"
	"github.com/argoproj/argo-cd/v2/applicationset/utils"

	appsetmetrics "github.com/argoproj/argo-cd/v2/applicationset/metrics"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	dbmocks "github.com/argoproj/argo-cd/v2/util/db/mocks"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application"
)

func TestCreateOrUpdateInCluster(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	err = v1alpha1.AddToScheme(scheme)
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
			metrics := appsetmetrics.NewFakeAppsetMetrics(client)

			r := ApplicationSetReconciler{
				Client:   client,
				Scheme:   scheme,
				Recorder: record.NewFakeRecorder(len(initObjs) + len(c.expected)),
				Metrics:  metrics,
			}

			err = r.createOrUpdateInCluster(context.TODO(), log.NewEntry(log.StandardLogger()), c.appSet, c.desiredApps)
			require.NoError(t, err)

			for _, obj := range c.expected {
				got := &v1alpha1.Application{}
				_ = client.Get(context.Background(), crtclient.ObjectKey{
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

	err = v1alpha1.AddToScheme(scheme)
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
						generators.ArgoCDSecretTypeLabel: generators.ArgoCDSecretTypeCluster,
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
			metrics := appsetmetrics.NewFakeAppsetMetrics(client)

			r := ApplicationSetReconciler{
				Client:        client,
				Scheme:        scheme,
				Recorder:      record.NewFakeRecorder(10),
				KubeClientset: kubeclientset,
				Metrics:       metrics,
			}
			// settingsMgr := settings.NewSettingsManager(context.TODO(), kubeclientset, "namespace")
			// argoDB := db.NewDB("namespace", settingsMgr, r.KubeClientset)
			// clusterList, err := argoDB.ListClusters(context.Background())
			clusterList, err := utils.ListClusters(context.Background(), kubeclientset, "namespace")
			require.NoError(t, err, "Unexpected error")

			appLog := log.WithFields(log.Fields{"app": app.Name, "appSet": ""})

			appInputParam := app.DeepCopy()

			err = r.removeFinalizerOnInvalidDestination(context.Background(), appSet, appInputParam, clusterList, appLog)
			require.NoError(t, err, "Unexpected error")

			retrievedApp := v1alpha1.Application{}
			err = client.Get(context.Background(), crtclient.ObjectKeyFromObject(&app), &retrievedApp)
			require.NoError(t, err, "Unexpected error")

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

	err = v1alpha1.AddToScheme(scheme)
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
					Namespace: "namespace",
					Labels: map[string]string{
						generators.ArgoCDSecretTypeLabel: generators.ArgoCDSecretTypeCluster,
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
			metrics := appsetmetrics.NewFakeAppsetMetrics(client)

			r := ApplicationSetReconciler{
				Client:        client,
				Scheme:        scheme,
				Recorder:      record.NewFakeRecorder(10),
				KubeClientset: kubeclientset,
				Metrics:       metrics,
			}
			// settingsMgr := settings.NewSettingsManager(context.TODO(), kubeclientset, "argocd")
			// argoDB := db.NewDB("argocd", settingsMgr, r.KubeClientset)
			// clusterList, err := argoDB.ListClusters(context.Background())
			clusterList, err := utils.ListClusters(context.Background(), kubeclientset, "namespace")
			require.NoError(t, err, "Unexpected error")

			appLog := log.WithFields(log.Fields{"app": app.Name, "appSet": ""})

			appInputParam := app.DeepCopy()

			err = r.removeFinalizerOnInvalidDestination(context.Background(), appSet, appInputParam, clusterList, appLog)
			require.NoError(t, err, "Unexpected error")

			retrievedApp := v1alpha1.Application{}
			err = client.Get(context.Background(), crtclient.ObjectKeyFromObject(&app), &retrievedApp)
			require.NoError(t, err, "Unexpected error")

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

	err = v1alpha1.AddToScheme(scheme)
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
			require.NoError(t, err, "Unexpected error")

			initObjs := []crtclient.Object{&app, &appSet}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics(client)

			r := ApplicationSetReconciler{
				Client:        client,
				Scheme:        scheme,
				Recorder:      record.NewFakeRecorder(10),
				KubeClientset: nil,
				Metrics:       metrics,
			}

			err = r.removeOwnerReferencesOnDeleteAppSet(context.Background(), appSet)
			require.NoError(t, err, "Unexpected error")

			retrievedApp := v1alpha1.Application{}
			err = client.Get(context.Background(), crtclient.ObjectKeyFromObject(&app), &retrievedApp)
			require.NoError(t, err, "Unexpected error")

			ownerReferencesRemoved := len(retrievedApp.OwnerReferences) == 0
			assert.True(t, ownerReferencesRemoved)
		})
	}
}

func TestCreateApplications(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	err = v1alpha1.AddToScheme(scheme)
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
			metrics := appsetmetrics.NewFakeAppsetMetrics(client)

			r := ApplicationSetReconciler{
				Client:   client,
				Scheme:   scheme,
				Recorder: record.NewFakeRecorder(len(initObjs) + len(c.expected)),
				Metrics:  metrics,
			}

			err = r.createInCluster(context.TODO(), log.NewEntry(log.StandardLogger()), c.appSet, c.apps)
			require.NoError(t, err)

			for _, obj := range c.expected {
				got := &v1alpha1.Application{}
				_ = client.Get(context.Background(), crtclient.ObjectKey{
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
	err = v1alpha1.AddToScheme(scheme)
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
		metrics := appsetmetrics.NewFakeAppsetMetrics(client)

		r := ApplicationSetReconciler{
			Client:        client,
			Scheme:        scheme,
			Recorder:      record.NewFakeRecorder(len(initObjs) + len(c.expected)),
			KubeClientset: kubefake.NewSimpleClientset(),
			Metrics:       metrics,
		}

		err = r.deleteInCluster(context.TODO(), log.NewEntry(log.StandardLogger()), c.appSet, c.desiredApps)
		require.NoError(t, err)

		// For each of the expected objects, verify they exist on the cluster
		for _, obj := range c.expected {
			got := &v1alpha1.Application{}
			_ = client.Get(context.Background(), crtclient.ObjectKey{
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
			err := client.Get(context.Background(), crtclient.ObjectKey{
				Namespace: obj.Namespace,
				Name:      obj.Name,
			}, got)

			assert.EqualError(t, err, fmt.Sprintf("applications.argoproj.io \"%s\" not found", obj.Name))
		}
	}
}

func TestGetMinRequeueAfter(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)
	err = v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	metrics := appsetmetrics.NewFakeAppsetMetrics(client)

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
		Return([]map[string]interface{}{}, fmt.Errorf("Simulated error generating params that could be related to an external service/API call"))

	metrics := appsetmetrics.NewFakeAppsetMetrics(client)

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

	res, err := r.Reconcile(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, ReconcileRequeueOnValidationError, res.RequeueAfter)
}

func TestValidateGeneratedApplications(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	err = v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	metrics := appsetmetrics.NewFakeAppsetMetrics(client)

	// Valid cluster
	myCluster := v1alpha1.Cluster{
		Server: "https://kubernetes.default.svc",
		Name:   "my-cluster",
	}

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

	// Test a subset of the validations that 'validateGeneratedApplications' performs
	for _, cc := range []struct {
		name             string
		apps             []v1alpha1.Application
		expectedErrors   []string
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
			expectedErrors:   []string{},
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
			expectedErrors:   []string{"application destination can't have both name and server defined"},
			validationErrors: map[int]error{0: fmt.Errorf("application destination spec is invalid: application destination can't have both name and server defined: my-cluster my-server")},
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
			expectedErrors:   []string{"application references project DOES-NOT-EXIST which does not exist"},
			validationErrors: map[int]error{0: fmt.Errorf("application references project DOES-NOT-EXIST which does not exist")},
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
			expectedErrors:   []string{},
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
			expectedErrors:   []string{"there are no clusters with this name: nonexistent-cluster"},
			validationErrors: map[int]error{0: fmt.Errorf("application destination spec is invalid: unable to find destination server: there are no clusters with this name: nonexistent-cluster")},
		},
	} {
		t.Run(cc.name, func(t *testing.T) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: "namespace",
					Labels: map[string]string{
						generators.ArgoCDSecretTypeLabel: generators.ArgoCDSecretTypeCluster,
					},
				},
				Data: map[string][]byte{
					"name":   []byte("my-cluster"),
					"server": []byte("https://kubernetes.default.svc"),
					"config": []byte("{\"username\":\"foo\",\"password\":\"foo\"}"),
				},
			}

			objects := append([]runtime.Object{}, secret)
			kubeclientset := kubefake.NewSimpleClientset(objects...)

			argoDBMock := dbmocks.ArgoDB{}
			argoDBMock.On("GetCluster", mock.Anything, "https://kubernetes.default.svc").Return(&myCluster, nil)
			argoDBMock.On("ListClusters", mock.Anything).Return(&v1alpha1.ClusterList{Items: []v1alpha1.Cluster{
				myCluster,
			}}, nil)

			argoObjs := []runtime.Object{myProject}
			for _, app := range cc.apps {
				argoObjs = append(argoObjs, &app)
			}

			r := ApplicationSetReconciler{
				Client:           client,
				Scheme:           scheme,
				Recorder:         record.NewFakeRecorder(1),
				Generators:       map[string]generators.Generator{},
				ArgoDB:           &argoDBMock,
				ArgoCDNamespace:  "namespace",
				ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
				KubeClientset:    kubeclientset,
				Metrics:          metrics,
			}

			appSetInfo := v1alpha1.ApplicationSet{}

			validationErrors, _ := r.validateGeneratedApplications(context.TODO(), cc.apps, appSetInfo)
			var errorMessages []string
			for _, v := range validationErrors {
				errorMessages = append(errorMessages, v.Error())
			}

			if len(errorMessages) == 0 {
				assert.Empty(t, cc.expectedErrors, "Expected errors but none were seen")
			} else {
				// An error was returned: it should be expected
				matched := false
				for _, expectedErr := range cc.expectedErrors {
					foundMatch := strings.Contains(strings.Join(errorMessages, ";"), expectedErr)
					assert.True(t, foundMatch, "Unble to locate expected error: %s", cc.expectedErrors)
					matched = matched || foundMatch
				}
				assert.True(t, matched, "An unexpected error occurrred: %v", err)
				// validation message was returned: it should be expected
				matched = false
				foundMatch := reflect.DeepEqual(validationErrors, cc.validationErrors)
				var message string
				for _, v := range validationErrors {
					message = v.Error()
					break
				}
				assert.True(t, foundMatch, "Unble to locate validation message: %s", message)
				matched = matched || foundMatch
				assert.True(t, matched, "An unexpected error occurrred: %v", err)
			}
		})
	}
}

func TestReconcilerValidationProjectErrorBehaviour(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)
	err = v1alpha1.AddToScheme(scheme)
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

	kubeclientset := kubefake.NewSimpleClientset()
	argoDBMock := dbmocks.ArgoDB{}
	argoObjs := []runtime.Object{&project}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet).WithStatusSubresource(&appSet).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
	metrics := appsetmetrics.NewFakeAppsetMetrics(client)
	goodCluster := v1alpha1.Cluster{Server: "https://good-cluster", Name: "good-cluster"}
	badCluster := v1alpha1.Cluster{Server: "https://bad-cluster", Name: "bad-cluster"}
	argoDBMock.On("GetCluster", mock.Anything, "https://good-cluster").Return(&goodCluster, nil)
	argoDBMock.On("GetCluster", mock.Anything, "https://bad-cluster").Return(&badCluster, nil)
	argoDBMock.On("ListClusters", mock.Anything).Return(&v1alpha1.ClusterList{Items: []v1alpha1.Cluster{
		goodCluster,
	}}, nil)

	r := ApplicationSetReconciler{
		Client:   client,
		Scheme:   scheme,
		Renderer: &utils.Render{},
		Recorder: record.NewFakeRecorder(1),
		Generators: map[string]generators.Generator{
			"List": generators.NewListGenerator(),
		},
		ArgoDB:           &argoDBMock,
		ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
		KubeClientset:    kubeclientset,
		Policy:           v1alpha1.ApplicationsSyncPolicySync,
		ArgoCDNamespace:  "argocd",
		Metrics:          metrics,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "argocd",
			Name:      "name",
		},
	}

	// Verify that on validation error, no error is returned, but the object is requeued
	res, err := r.Reconcile(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, ReconcileRequeueOnValidationError, res.RequeueAfter)

	var app v1alpha1.Application

	// make sure good app got created
	err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "good-project"}, &app)
	require.NoError(t, err)
	assert.Equal(t, "good-project", app.Name)

	// make sure bad app was not created
	err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "bad-project"}, &app)
	require.Error(t, err)
}

func TestSetApplicationSetStatusCondition(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)
	err = v1alpha1.AddToScheme(scheme)
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
	argoDBMock := dbmocks.ArgoDB{}
	argoObjs := []runtime.Object{}

	for _, testCase := range testCases {
		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&testCase.appset).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).WithStatusSubresource(&testCase.appset).Build()
		metrics := appsetmetrics.NewFakeAppsetMetrics(client)

		r := ApplicationSetReconciler{
			Client:   client,
			Scheme:   scheme,
			Renderer: &utils.Render{},
			Recorder: record.NewFakeRecorder(1),
			Generators: map[string]generators.Generator{
				"List": generators.NewListGenerator(),
			},
			ArgoDB:           &argoDBMock,
			ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
			KubeClientset:    kubeclientset,
			Metrics:          metrics,
		}

		for _, condition := range testCase.conditions {
			err = r.setApplicationSetStatusCondition(context.TODO(), &testCase.appset, condition, true)
			require.NoError(t, err)
		}

		testCase.testfunc(t, testCase.appset)
	}
}

func applicationsUpdateSyncPolicyTest(t *testing.T, applicationsSyncPolicy v1alpha1.ApplicationsSyncPolicy, recordBuffer int, allowPolicyOverride bool) v1alpha1.Application {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)
	err = v1alpha1.AddToScheme(scheme)
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

	kubeclientset := kubefake.NewSimpleClientset()
	argoDBMock := dbmocks.ArgoDB{}
	argoObjs := []runtime.Object{&defaultProject}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet).WithStatusSubresource(&appSet).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
	metrics := appsetmetrics.NewFakeAppsetMetrics(client)
	goodCluster := v1alpha1.Cluster{Server: "https://good-cluster", Name: "good-cluster"}
	argoDBMock.On("GetCluster", mock.Anything, "https://good-cluster").Return(&goodCluster, nil)
	argoDBMock.On("ListClusters", mock.Anything).Return(&v1alpha1.ClusterList{Items: []v1alpha1.Cluster{
		goodCluster,
	}}, nil)

	r := ApplicationSetReconciler{
		Client:   client,
		Scheme:   scheme,
		Renderer: &utils.Render{},
		Recorder: record.NewFakeRecorder(recordBuffer),
		Generators: map[string]generators.Generator{
			"List": generators.NewListGenerator(),
		},
		ArgoDB:               &argoDBMock,
		ArgoCDNamespace:      "argocd",
		ArgoAppClientset:     appclientset.NewSimpleClientset(argoObjs...),
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
	resCreate, err := r.Reconcile(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), resCreate.RequeueAfter)

	var app v1alpha1.Application

	// make sure good app got created
	err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "good-cluster"}, &app)
	require.NoError(t, err)
	assert.Equal(t, "good-cluster", app.Name)

	// Update resource
	var retrievedApplicationSet v1alpha1.ApplicationSet
	err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "name"}, &retrievedApplicationSet)
	require.NoError(t, err)

	retrievedApplicationSet.Spec.Template.Annotations = map[string]string{"annotation-key": "annotation-value"}
	retrievedApplicationSet.Spec.Template.Labels = map[string]string{"label-key": "label-value"}

	retrievedApplicationSet.Spec.Template.Spec.Source.Helm = &v1alpha1.ApplicationSourceHelm{
		Values: "global.test: test",
	}

	err = r.Client.Update(context.TODO(), &retrievedApplicationSet)
	require.NoError(t, err)

	resUpdate, err := r.Reconcile(context.Background(), req)
	require.NoError(t, err)

	err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "good-cluster"}, &app)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), resUpdate.RequeueAfter)
	assert.Equal(t, "good-cluster", app.Name)

	return app
}

func TestUpdateNotPerformedWithSyncPolicyCreateOnly(t *testing.T) {
	applicationsSyncPolicy := v1alpha1.ApplicationsSyncPolicyCreateOnly

	app := applicationsUpdateSyncPolicyTest(t, applicationsSyncPolicy, 1, true)

	assert.Nil(t, app.Spec.Source.Helm)
	assert.Nil(t, app.ObjectMeta.Annotations)
}

func TestUpdateNotPerformedWithSyncPolicyCreateDelete(t *testing.T) {
	applicationsSyncPolicy := v1alpha1.ApplicationsSyncPolicyCreateDelete

	app := applicationsUpdateSyncPolicyTest(t, applicationsSyncPolicy, 1, true)

	assert.Nil(t, app.Spec.Source.Helm)
	assert.Nil(t, app.ObjectMeta.Annotations)
}

func TestUpdatePerformedWithSyncPolicyCreateUpdate(t *testing.T) {
	applicationsSyncPolicy := v1alpha1.ApplicationsSyncPolicyCreateUpdate

	app := applicationsUpdateSyncPolicyTest(t, applicationsSyncPolicy, 2, true)

	assert.Equal(t, "global.test: test", app.Spec.Source.Helm.Values)
	assert.Equal(t, map[string]string{"annotation-key": "annotation-value"}, app.ObjectMeta.Annotations)
	assert.Equal(t, map[string]string{"label-key": "label-value"}, app.ObjectMeta.Labels)
}

func TestUpdatePerformedWithSyncPolicySync(t *testing.T) {
	applicationsSyncPolicy := v1alpha1.ApplicationsSyncPolicySync

	app := applicationsUpdateSyncPolicyTest(t, applicationsSyncPolicy, 2, true)

	assert.Equal(t, "global.test: test", app.Spec.Source.Helm.Values)
	assert.Equal(t, map[string]string{"annotation-key": "annotation-value"}, app.ObjectMeta.Annotations)
	assert.Equal(t, map[string]string{"label-key": "label-value"}, app.ObjectMeta.Labels)
}

func TestUpdatePerformedWithSyncPolicyCreateOnlyAndAllowPolicyOverrideFalse(t *testing.T) {
	applicationsSyncPolicy := v1alpha1.ApplicationsSyncPolicyCreateOnly

	app := applicationsUpdateSyncPolicyTest(t, applicationsSyncPolicy, 2, false)

	assert.Equal(t, "global.test: test", app.Spec.Source.Helm.Values)
	assert.Equal(t, map[string]string{"annotation-key": "annotation-value"}, app.ObjectMeta.Annotations)
	assert.Equal(t, map[string]string{"label-key": "label-value"}, app.ObjectMeta.Labels)
}

func applicationsDeleteSyncPolicyTest(t *testing.T, applicationsSyncPolicy v1alpha1.ApplicationsSyncPolicy, recordBuffer int, allowPolicyOverride bool) v1alpha1.ApplicationList {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)
	err = v1alpha1.AddToScheme(scheme)
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

	kubeclientset := kubefake.NewSimpleClientset()
	argoDBMock := dbmocks.ArgoDB{}
	argoObjs := []runtime.Object{&defaultProject}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet).WithStatusSubresource(&appSet).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
	metrics := appsetmetrics.NewFakeAppsetMetrics(client)
	goodCluster := v1alpha1.Cluster{Server: "https://good-cluster", Name: "good-cluster"}
	argoDBMock.On("GetCluster", mock.Anything, "https://good-cluster").Return(&goodCluster, nil)
	argoDBMock.On("ListClusters", mock.Anything).Return(&v1alpha1.ClusterList{Items: []v1alpha1.Cluster{
		goodCluster,
	}}, nil)

	r := ApplicationSetReconciler{
		Client:   client,
		Scheme:   scheme,
		Renderer: &utils.Render{},
		Recorder: record.NewFakeRecorder(recordBuffer),
		Generators: map[string]generators.Generator{
			"List": generators.NewListGenerator(),
		},
		ArgoDB:               &argoDBMock,
		ArgoCDNamespace:      "argocd",
		ArgoAppClientset:     appclientset.NewSimpleClientset(argoObjs...),
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
	resCreate, err := r.Reconcile(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), resCreate.RequeueAfter)

	var app v1alpha1.Application

	// make sure good app got created
	err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "good-cluster"}, &app)
	require.NoError(t, err)
	assert.Equal(t, "good-cluster", app.Name)

	// Update resource
	var retrievedApplicationSet v1alpha1.ApplicationSet
	err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "name"}, &retrievedApplicationSet)
	require.NoError(t, err)
	retrievedApplicationSet.Spec.Generators = []v1alpha1.ApplicationSetGenerator{
		{
			List: &v1alpha1.ListGenerator{
				Elements: []apiextensionsv1.JSON{},
			},
		},
	}

	err = r.Client.Update(context.TODO(), &retrievedApplicationSet)
	require.NoError(t, err)

	resUpdate, err := r.Reconcile(context.Background(), req)
	require.NoError(t, err)

	var apps v1alpha1.ApplicationList

	err = r.Client.List(context.TODO(), &apps)
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

	assert.Empty(t, apps.Items)
}

func TestDeletePerformedWithSyncPolicySync(t *testing.T) {
	applicationsSyncPolicy := v1alpha1.ApplicationsSyncPolicySync

	apps := applicationsDeleteSyncPolicyTest(t, applicationsSyncPolicy, 3, true)

	assert.Empty(t, apps.Items)
}

func TestDeletePerformedWithSyncPolicyCreateOnlyAndAllowPolicyOverrideFalse(t *testing.T) {
	applicationsSyncPolicy := v1alpha1.ApplicationsSyncPolicyCreateOnly

	apps := applicationsDeleteSyncPolicyTest(t, applicationsSyncPolicy, 3, false)

	assert.Empty(t, apps.Items)
}

func TestPolicies(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	err = v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	defaultProject := v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "argocd"},
		Spec:       v1alpha1.AppProjectSpec{SourceRepos: []string{"*"}, Destinations: []v1alpha1.ApplicationDestination{{Namespace: "*", Server: "https://kubernetes.default.svc"}}},
	}
	myCluster := v1alpha1.Cluster{
		Server: "https://kubernetes.default.svc",
		Name:   "my-cluster",
	}

	kubeclientset := kubefake.NewSimpleClientset()
	argoDBMock := dbmocks.ArgoDB{}
	argoDBMock.On("GetCluster", mock.Anything, "https://kubernetes.default.svc").Return(&myCluster, nil)
	argoObjs := []runtime.Object{&defaultProject}

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

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet).WithStatusSubresource(&appSet).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics(client)

			r := ApplicationSetReconciler{
				Client:   client,
				Scheme:   scheme,
				Renderer: &utils.Render{},
				Recorder: record.NewFakeRecorder(10),
				Generators: map[string]generators.Generator{
					"List": generators.NewListGenerator(),
				},
				ArgoDB:           &argoDBMock,
				ArgoCDNamespace:  "argocd",
				ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
				KubeClientset:    kubeclientset,
				Policy:           policy,
				Metrics:          metrics,
			}

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "argocd",
					Name:      "name",
				},
			}

			// Check if Application is created
			res, err := r.Reconcile(context.Background(), req)
			require.NoError(t, err)
			assert.Equal(t, time.Duration(0), res.RequeueAfter)

			var app v1alpha1.Application
			err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "my-app"}, &app)
			require.NoError(t, err)
			assert.Equal(t, "value", app.Annotations["key"])

			// Check if Application is updated
			app.Annotations["key"] = "edited"
			err = r.Client.Update(context.TODO(), &app)
			require.NoError(t, err)

			res, err = r.Reconcile(context.Background(), req)
			require.NoError(t, err)
			assert.Equal(t, time.Duration(0), res.RequeueAfter)

			err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "my-app"}, &app)
			require.NoError(t, err)

			if c.allowedUpdate {
				assert.Equal(t, "value", app.Annotations["key"])
			} else {
				assert.Equal(t, "edited", app.Annotations["key"])
			}

			// Check if Application is deleted
			err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "name"}, &appSet)
			require.NoError(t, err)
			appSet.Spec.Generators[0] = v1alpha1.ApplicationSetGenerator{
				List: &v1alpha1.ListGenerator{
					Elements: []apiextensionsv1.JSON{},
				},
			}
			err = r.Client.Update(context.TODO(), &appSet)
			require.NoError(t, err)

			res, err = r.Reconcile(context.Background(), req)
			require.NoError(t, err)
			assert.Equal(t, time.Duration(0), res.RequeueAfter)

			err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "my-app"}, &app)
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
	err = v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	kubeclientset := kubefake.NewSimpleClientset([]runtime.Object{}...)
	argoDBMock := dbmocks.ArgoDB{}
	argoObjs := []runtime.Object{}

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
			metrics := appsetmetrics.NewFakeAppsetMetrics(client)

			r := ApplicationSetReconciler{
				Client:   client,
				Scheme:   scheme,
				Renderer: &utils.Render{},
				Recorder: record.NewFakeRecorder(1),
				Generators: map[string]generators.Generator{
					"List": generators.NewListGenerator(),
				},
				ArgoDB:           &argoDBMock,
				ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
				KubeClientset:    kubeclientset,
				Metrics:          metrics,
			}

			err = r.setAppSetApplicationStatus(context.TODO(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.appStatuses)
			require.NoError(t, err)

			assert.Equal(t, cc.expectedAppStatuses, cc.appSet.Status.ApplicationStatus)
		})
	}
}

func TestBuildAppDependencyList(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	err = v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	metrics := appsetmetrics.NewFakeAppsetMetrics(client)

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
			argoDBMock := dbmocks.ArgoDB{}
			argoObjs := []runtime.Object{}

			r := ApplicationSetReconciler{
				Client:           client,
				Scheme:           scheme,
				Recorder:         record.NewFakeRecorder(1),
				Generators:       map[string]generators.Generator{},
				ArgoDB:           &argoDBMock,
				ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
				KubeClientset:    kubeclientset,
				Metrics:          metrics,
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

	err = v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	metrics := appsetmetrics.NewFakeAppsetMetrics(client)

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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
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
						Health: v1alpha1.HealthStatus{
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
						Health: v1alpha1.HealthStatus{
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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
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
						Health: v1alpha1.HealthStatus{
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
						Health: v1alpha1.HealthStatus{
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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
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
						Health: v1alpha1.HealthStatus{
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
						Health: v1alpha1.HealthStatus{
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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
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
						Health: v1alpha1.HealthStatus{
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
						Health: v1alpha1.HealthStatus{
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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
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
						Health: v1alpha1.HealthStatus{
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
						Health: v1alpha1.HealthStatus{
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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
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
						Health: v1alpha1.HealthStatus{
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
						Health: v1alpha1.HealthStatus{
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
						Health: v1alpha1.HealthStatus{
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
						Health: v1alpha1.HealthStatus{
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
						Health: v1alpha1.HealthStatus{
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
			argoDBMock := dbmocks.ArgoDB{}
			argoObjs := []runtime.Object{}

			r := ApplicationSetReconciler{
				Client:           client,
				Scheme:           scheme,
				Recorder:         record.NewFakeRecorder(1),
				Generators:       map[string]generators.Generator{},
				ArgoDB:           &argoDBMock,
				ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
				KubeClientset:    kubeclientset,
				Metrics:          metrics,
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

	err = v1alpha1.AddToScheme(scheme)
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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.HealthStatus{
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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
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
						Health: v1alpha1.HealthStatus{
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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
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
						Health: v1alpha1.HealthStatus{
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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
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
						Health: v1alpha1.HealthStatus{
							Status: health.HealthStatusProgressing,
						},
					},
				},
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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
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
						Health: v1alpha1.HealthStatus{
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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
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
						Health: v1alpha1.HealthStatus{
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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
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
						Health: v1alpha1.HealthStatus{
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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: v1alpha1.ApplicationStatus{
						Health: v1alpha1.HealthStatus{
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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
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
						Health: v1alpha1.HealthStatus{
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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
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
						Health: v1alpha1.HealthStatus{
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
			name: "does not progresses a pending application with a successful sync triggered by controller with invalid revision to progressing",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
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
						Health: v1alpha1.HealthStatus{
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
								Revision: "Previous",
							},
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					Message:         "",
					Status:          "Pending",
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
						Type:        "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{},
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
						Health: v1alpha1.HealthStatus{
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
			argoDBMock := dbmocks.ArgoDB{}
			argoObjs := []runtime.Object{}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&cc.appSet).WithStatusSubresource(&cc.appSet).Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics(client)

			r := ApplicationSetReconciler{
				Client:           client,
				Scheme:           scheme,
				Recorder:         record.NewFakeRecorder(1),
				Generators:       map[string]generators.Generator{},
				ArgoDB:           &argoDBMock,
				ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
				KubeClientset:    kubeclientset,
				Metrics:          metrics,
			}

			appStatuses, err := r.updateApplicationSetApplicationStatus(context.TODO(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.apps, cc.appStepMap)

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

	err = v1alpha1.AddToScheme(scheme)
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
			argoDBMock := dbmocks.ArgoDB{}
			argoObjs := []runtime.Object{}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&cc.appSet).WithStatusSubresource(&cc.appSet).Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics(client)

			r := ApplicationSetReconciler{
				Client:           client,
				Scheme:           scheme,
				Recorder:         record.NewFakeRecorder(1),
				Generators:       map[string]generators.Generator{},
				ArgoDB:           &argoDBMock,
				ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
				KubeClientset:    kubeclientset,
				Metrics:          metrics,
			}

			appStatuses, err := r.updateApplicationSetApplicationStatusProgress(context.TODO(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.appSyncMap, cc.appStepMap)

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

	err = v1alpha1.AddToScheme(scheme)
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
						Health: v1alpha1.HealthStatus{
							Status:  health.HealthStatusHealthy,
							Message: "OK",
						},
					},
				},
			},
			expectedResources: []v1alpha1.ResourceStatus{
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
								Status:  health.HealthStatusHealthy,
								Message: "OK",
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
						Health: v1alpha1.HealthStatus{
							Status:  health.HealthStatusHealthy,
							Message: "OK",
						},
					},
				},
			},
			expectedResources: []v1alpha1.ResourceStatus{
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
						Health: v1alpha1.HealthStatus{
							Status:  health.HealthStatusHealthy,
							Message: "OK",
						},
					},
				},
			},
			expectedResources: []v1alpha1.ResourceStatus{
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
			argoDBMock := dbmocks.ArgoDB{}
			argoObjs := []runtime.Object{}

			client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&cc.appSet).WithObjects(&cc.appSet).Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics(client)

			r := ApplicationSetReconciler{
				Client:           client,
				Scheme:           scheme,
				Recorder:         record.NewFakeRecorder(1),
				Generators:       map[string]generators.Generator{},
				ArgoDB:           &argoDBMock,
				ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
				KubeClientset:    kubeclientset,
				Metrics:          metrics,
			}

			err := r.updateResourcesStatus(context.TODO(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.apps)

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
				Status:  health.HealthStatusHealthy,
				Message: "OK",
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
				Health: v1alpha1.HealthStatus{
					Status:  health.HealthStatusHealthy,
					Message: "OK",
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
			argoDBMock := dbmocks.ArgoDB{}
			argoObjs := []runtime.Object{}

			client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&cc.appSet).WithObjects(&cc.appSet).Build()
			metrics := appsetmetrics.NewFakeAppsetMetrics(client)

			r := ApplicationSetReconciler{
				Client:           client,
				Scheme:           scheme,
				Recorder:         record.NewFakeRecorder(1),
				Generators:       map[string]generators.Generator{},
				ArgoDB:           &argoDBMock,
				ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
				KubeClientset:    kubeclientset,
				Metrics:          metrics,
			}

			err := r.updateResourcesStatus(context.TODO(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.apps)
			require.NoError(t, err, "expected no errors, but errors occurred")

			err = r.updateResourcesStatus(context.TODO(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.apps)
			require.NoError(t, err, "expected no errors, but errors occurred")

			err = r.updateResourcesStatus(context.TODO(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.apps)
			require.NoError(t, err, "expected no errors, but errors occurred")

			assert.Equal(t, cc.expectedResources, cc.appSet.Status.Resources, "expected resources did not match actual")
		})
	}
}

func TestOwnsHandler(t *testing.T) {
	// progressive syncs do not affect create, delete, or generic
	ownsHandler := getOwnsHandlerPredicates(true)
	assert.False(t, ownsHandler.CreateFunc(event.CreateEvent{}))
	assert.True(t, ownsHandler.DeleteFunc(event.DeleteEvent{}))
	assert.True(t, ownsHandler.GenericFunc(event.GenericEvent{}))
	ownsHandler = getOwnsHandlerPredicates(false)
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
					Health: v1alpha1.HealthStatus{
						Status: "Unknown",
					},
				}},
				ObjectNew: &v1alpha1.Application{Status: v1alpha1.ApplicationStatus{
					Health: v1alpha1.HealthStatus{
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
			ownsHandler = getOwnsHandlerPredicates(tt.args.enableProgressiveSyncs)
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

			err := r.migrateStatus(context.Background(), &tc.appset)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, tc.appset.Status)
		})
	}
}
