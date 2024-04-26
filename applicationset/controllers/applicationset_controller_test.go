package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8scache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	crtclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync/common"

	"github.com/argoproj/argo-cd/v2/applicationset/generators"
	"github.com/argoproj/argo-cd/v2/applicationset/utils"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/v2/util/collections"
	dbmocks "github.com/argoproj/argo-cd/v2/util/db/mocks"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application"
)

type fakeStore struct {
	k8scache.Store
}

func (f *fakeStore) Update(obj interface{}) error {
	return nil
}

type fakeInformer struct {
	k8scache.SharedInformer
}

func (f *fakeInformer) AddIndexers(indexers k8scache.Indexers) error {
	return nil
}

func (f *fakeInformer) GetStore() k8scache.Store {
	return &fakeStore{}
}

type fakeCache struct {
	cache.Cache
}

func (f *fakeCache) GetInformer(ctx context.Context, obj crtclient.Object) (cache.Informer, error) {
	return &fakeInformer{}, nil
}

type generatorMock struct {
	mock.Mock
}

func (g *generatorMock) GetTemplate(appSetGenerator *v1alpha1.ApplicationSetGenerator) *v1alpha1.ApplicationSetTemplate {
	args := g.Called(appSetGenerator)

	return args.Get(0).(*v1alpha1.ApplicationSetTemplate)
}

func (g *generatorMock) GenerateParams(appSetGenerator *v1alpha1.ApplicationSetGenerator, _ *v1alpha1.ApplicationSet) ([]map[string]interface{}, error) {
	args := g.Called(appSetGenerator)

	return args.Get(0).([]map[string]interface{}), args.Error(1)
}

func (g *generatorMock) Replace(tmpl string, replaceMap map[string]interface{}, useGoTemplate bool, goTemplateOptions []string) (string, error) {
	args := g.Called(tmpl, replaceMap, useGoTemplate, goTemplateOptions)

	return args.Get(0).(string), args.Error(1)
}

type rendererMock struct {
	mock.Mock
}

func (g *generatorMock) GetRequeueAfter(appSetGenerator *v1alpha1.ApplicationSetGenerator) time.Duration {
	args := g.Called(appSetGenerator)

	return args.Get(0).(time.Duration)
}

func (r *rendererMock) RenderTemplateParams(tmpl *v1alpha1.Application, syncPolicy *v1alpha1.ApplicationSetSyncPolicy, params map[string]interface{}, useGoTemplate bool, goTemplateOptions []string) (*v1alpha1.Application, error) {
	args := r.Called(tmpl, params, useGoTemplate, goTemplateOptions)

	if args.Error(1) != nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*v1alpha1.Application), args.Error(1)

}

func (r *rendererMock) Replace(tmpl string, replaceMap map[string]interface{}, useGoTemplate bool, goTemplateOptions []string) (string, error) {
	args := r.Called(tmpl, replaceMap, useGoTemplate, goTemplateOptions)

	return args.Get(0).(string), args.Error(1)
}

func TestExtractApplications(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	for _, c := range []struct {
		name                string
		params              []map[string]interface{}
		template            v1alpha1.ApplicationSetTemplate
		generateParamsError error
		rendererError       error
		expectErr           bool
		expectedReason      v1alpha1.ApplicationSetReasonType
	}{
		{
			name:   "Generate two applications",
			params: []map[string]interface{}{{"name": "app1"}, {"name": "app2"}},
			template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
					Name:      "name",
					Namespace: "namespace",
					Labels:    map[string]string{"label_name": "label_value"},
				},
				Spec: v1alpha1.ApplicationSpec{},
			},
			expectedReason: "",
		},
		{
			name:                "Handles error from the generator",
			generateParamsError: fmt.Errorf("error"),
			expectErr:           true,
			expectedReason:      v1alpha1.ApplicationSetReasonApplicationParamsGenerationError,
		},
		{
			name:   "Handles error from the render",
			params: []map[string]interface{}{{"name": "app1"}, {"name": "app2"}},
			template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
					Name:      "name",
					Namespace: "namespace",
					Labels:    map[string]string{"label_name": "label_value"},
				},
				Spec: v1alpha1.ApplicationSpec{},
			},
			rendererError:  fmt.Errorf("error"),
			expectErr:      true,
			expectedReason: v1alpha1.ApplicationSetReasonRenderTemplateParamsError,
		},
	} {
		cc := c
		app := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}

		t.Run(cc.name, func(t *testing.T) {

			appSet := &v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(appSet).Build()

			generatorMock := generatorMock{}
			generator := v1alpha1.ApplicationSetGenerator{
				List: &v1alpha1.ListGenerator{},
			}

			generatorMock.On("GenerateParams", &generator).
				Return(cc.params, cc.generateParamsError)

			generatorMock.On("GetTemplate", &generator).
				Return(&v1alpha1.ApplicationSetTemplate{})

			rendererMock := rendererMock{}

			var expectedApps []v1alpha1.Application

			if cc.generateParamsError == nil {
				for _, p := range cc.params {

					if cc.rendererError != nil {
						rendererMock.On("RenderTemplateParams", getTempApplication(cc.template), p, false, []string(nil)).
							Return(nil, cc.rendererError)
					} else {
						rendererMock.On("RenderTemplateParams", getTempApplication(cc.template), p, false, []string(nil)).
							Return(&app, nil)
						expectedApps = append(expectedApps, app)
					}
				}
			}

			r := ApplicationSetReconciler{
				Client:   client,
				Scheme:   scheme,
				Recorder: record.NewFakeRecorder(1),
				Generators: map[string]generators.Generator{
					"List": &generatorMock,
				},
				Renderer:      &rendererMock,
				KubeClientset: kubefake.NewSimpleClientset(),
				Cache:         &fakeCache{},
			}

			got, reason, err := r.generateApplications(log.NewEntry(log.StandardLogger()), v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Generators: []v1alpha1.ApplicationSetGenerator{generator},
					Template:   cc.template,
				},
			})

			if cc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, expectedApps, got)
			assert.Equal(t, cc.expectedReason, reason)
			generatorMock.AssertNumberOfCalls(t, "GenerateParams", 1)

			if cc.generateParamsError == nil {
				rendererMock.AssertNumberOfCalls(t, "RenderTemplateParams", len(cc.params))
			}

		})
	}

}

func TestMergeTemplateApplications(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	for _, c := range []struct {
		name             string
		params           []map[string]interface{}
		template         v1alpha1.ApplicationSetTemplate
		overrideTemplate v1alpha1.ApplicationSetTemplate
		expectedMerged   v1alpha1.ApplicationSetTemplate
		expectedApps     []v1alpha1.Application
	}{
		{
			name:   "Generate app",
			params: []map[string]interface{}{{"name": "app1"}},
			template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
					Name:      "name",
					Namespace: "namespace",
					Labels:    map[string]string{"label_name": "label_value"},
				},
				Spec: v1alpha1.ApplicationSpec{},
			},
			overrideTemplate: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
					Name:   "test",
					Labels: map[string]string{"foo": "bar"},
				},
				Spec: v1alpha1.ApplicationSpec{},
			},
			expectedMerged: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
					Name:      "test",
					Namespace: "namespace",
					Labels:    map[string]string{"label_name": "label_value", "foo": "bar"},
				},
				Spec: v1alpha1.ApplicationSpec{},
			},
			expectedApps: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
						Labels:    map[string]string{"foo": "bar"},
					},
					Spec: v1alpha1.ApplicationSpec{},
				},
			},
		},
	} {
		cc := c

		t.Run(cc.name, func(t *testing.T) {

			generatorMock := generatorMock{}
			generator := v1alpha1.ApplicationSetGenerator{
				List: &v1alpha1.ListGenerator{},
			}

			generatorMock.On("GenerateParams", &generator).
				Return(cc.params, nil)

			generatorMock.On("GetTemplate", &generator).
				Return(&cc.overrideTemplate)

			rendererMock := rendererMock{}

			rendererMock.On("RenderTemplateParams", getTempApplication(cc.expectedMerged), cc.params[0], false, []string(nil)).
				Return(&cc.expectedApps[0], nil)

			r := ApplicationSetReconciler{
				Client:   client,
				Scheme:   scheme,
				Recorder: record.NewFakeRecorder(1),
				Generators: map[string]generators.Generator{
					"List": &generatorMock,
				},
				Renderer:      &rendererMock,
				KubeClientset: kubefake.NewSimpleClientset(),
			}

			got, _, _ := r.generateApplications(log.NewEntry(log.StandardLogger()), v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Generators: []v1alpha1.ApplicationSetGenerator{generator},
					Template:   cc.template,
				},
			},
			)

			assert.Equal(t, cc.expectedApps, got)
		})
	}

}

func TestCreateOrUpdateInCluster(t *testing.T) {

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

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
						Name: "app1",
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
						Name: "app1",
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
						Name: "app2",
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
						Name: "app1",
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
						Name: "app1",
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
						Name: "app1",
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
		}, {
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
						Name: "app1",
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
		}, {
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
						Name: "app1",
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
		}, {
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
						Name: "app1",
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
		}, {
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
						Name: "app1",
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
		}, {
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
						Name: "app1",
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
		}, {
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
						Name: "app1",
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
				assert.Nil(t, err)
				initObjs = append(initObjs, &a)
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()

			r := ApplicationSetReconciler{
				Client:   client,
				Scheme:   scheme,
				Recorder: record.NewFakeRecorder(len(initObjs) + len(c.expected)),
				Cache:    &fakeCache{},
			}

			err = r.createOrUpdateInCluster(context.TODO(), log.NewEntry(log.StandardLogger()), c.appSet, c.desiredApps)
			assert.NoError(t, err)

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
	assert.Nil(t, err)

	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

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

			r := ApplicationSetReconciler{
				Client:        client,
				Scheme:        scheme,
				Recorder:      record.NewFakeRecorder(10),
				KubeClientset: kubeclientset,
				Cache:         &fakeCache{},
			}
			//settingsMgr := settings.NewSettingsManager(context.TODO(), kubeclientset, "namespace")
			//argoDB := db.NewDB("namespace", settingsMgr, r.KubeClientset)
			//clusterList, err := argoDB.ListClusters(context.Background())
			clusterList, err := utils.ListClusters(context.Background(), kubeclientset, "namespace")
			assert.NoError(t, err, "Unexpected error")

			appLog := log.WithFields(log.Fields{"app": app.Name, "appSet": ""})

			appInputParam := app.DeepCopy()

			err = r.removeFinalizerOnInvalidDestination(context.Background(), appSet, appInputParam, clusterList, appLog)
			assert.NoError(t, err, "Unexpected error")

			retrievedApp := v1alpha1.Application{}
			err = client.Get(context.Background(), crtclient.ObjectKeyFromObject(&app), &retrievedApp)
			assert.NoError(t, err, "Unexpected error")

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
	assert.Nil(t, err)

	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

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

			r := ApplicationSetReconciler{
				Client:        client,
				Scheme:        scheme,
				Recorder:      record.NewFakeRecorder(10),
				KubeClientset: kubeclientset,
				Cache:         &fakeCache{},
			}
			// settingsMgr := settings.NewSettingsManager(context.TODO(), kubeclientset, "argocd")
			// argoDB := db.NewDB("argocd", settingsMgr, r.KubeClientset)
			// clusterList, err := argoDB.ListClusters(context.Background())
			clusterList, err := utils.ListClusters(context.Background(), kubeclientset, "namespace")
			assert.NoError(t, err, "Unexpected error")

			appLog := log.WithFields(log.Fields{"app": app.Name, "appSet": ""})

			appInputParam := app.DeepCopy()

			err = r.removeFinalizerOnInvalidDestination(context.Background(), appSet, appInputParam, clusterList, appLog)
			assert.NoError(t, err, "Unexpected error")

			retrievedApp := v1alpha1.Application{}
			err = client.Get(context.Background(), crtclient.ObjectKeyFromObject(&app), &retrievedApp)
			assert.NoError(t, err, "Unexpected error")

			finalizerRemoved := len(retrievedApp.Finalizers) == 0

			assert.True(t, c.expectFinalizerRemoved == finalizerRemoved)

			bytes, _ := json.MarshalIndent(retrievedApp, "", "  ")
			t.Log("Contents of app after call:", string(bytes))

		})
	}
}

func TestRemoveOwnerReferencesOnDeleteAppSet(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

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
			assert.NoError(t, err, "Unexpected error")

			initObjs := []crtclient.Object{&app, &appSet}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()

			r := ApplicationSetReconciler{
				Client:        client,
				Scheme:        scheme,
				Recorder:      record.NewFakeRecorder(10),
				KubeClientset: nil,
				Cache:         &fakeCache{},
			}

			err = r.removeOwnerReferencesOnDeleteAppSet(context.Background(), appSet)
			assert.NoError(t, err, "Unexpected error")

			retrievedApp := v1alpha1.Application{}
			err = client.Get(context.Background(), crtclient.ObjectKeyFromObject(&app), &retrievedApp)
			assert.NoError(t, err, "Unexpected error")

			ownerReferencesRemoved := len(retrievedApp.OwnerReferences) == 0
			assert.True(t, ownerReferencesRemoved)
		})
	}
}

func TestCreateApplications(t *testing.T) {

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

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
						Name: "app1",
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
						Name: "app1",
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
						Name: "app2",
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
				assert.Nil(t, err)
				initObjs = append(initObjs, &a)
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()

			r := ApplicationSetReconciler{
				Client:   client,
				Scheme:   scheme,
				Recorder: record.NewFakeRecorder(len(initObjs) + len(c.expected)),
				Cache:    &fakeCache{},
			}

			err = r.createInCluster(context.TODO(), log.NewEntry(log.StandardLogger()), c.appSet, c.apps)
			assert.Nil(t, err)

			for _, obj := range c.expected {
				got := &v1alpha1.Application{}
				_ = client.Get(context.Background(), crtclient.ObjectKey{
					Namespace: obj.Namespace,
					Name:      obj.Name,
				}, got)

				err = controllerutil.SetControllerReference(&c.appSet, &obj, r.Scheme)
				assert.Nil(t, err)

				assert.Equal(t, obj, *got)
			}
		})
	}
}

func TestDeleteInCluster(t *testing.T) {

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)
	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

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
			assert.Nil(t, err)
			initObjs = append(initObjs, &temp)
		}

		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()

		r := ApplicationSetReconciler{
			Client:        client,
			Scheme:        scheme,
			Recorder:      record.NewFakeRecorder(len(initObjs) + len(c.expected)),
			KubeClientset: kubefake.NewSimpleClientset(),
		}

		err = r.deleteInCluster(context.TODO(), log.NewEntry(log.StandardLogger()), c.appSet, c.desiredApps)
		assert.Nil(t, err)

		// For each of the expected objects, verify they exist on the cluster
		for _, obj := range c.expected {
			got := &v1alpha1.Application{}
			_ = client.Get(context.Background(), crtclient.ObjectKey{
				Namespace: obj.Namespace,
				Name:      obj.Name,
			}, got)

			err = controllerutil.SetControllerReference(&c.appSet, &obj, r.Scheme)
			assert.Nil(t, err)

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
	assert.Nil(t, err)
	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	generator := v1alpha1.ApplicationSetGenerator{
		List:     &v1alpha1.ListGenerator{},
		Git:      &v1alpha1.GitGenerator{},
		Clusters: &v1alpha1.ClusterGenerator{},
	}

	generatorMock0 := generatorMock{}
	generatorMock0.On("GetRequeueAfter", &generator).
		Return(generators.NoRequeueAfter)

	generatorMock1 := generatorMock{}
	generatorMock1.On("GetRequeueAfter", &generator).
		Return(time.Duration(1) * time.Second)

	generatorMock10 := generatorMock{}
	generatorMock10.On("GetRequeueAfter", &generator).
		Return(time.Duration(10) * time.Second)

	r := ApplicationSetReconciler{
		Client:   client,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(0),
		Cache:    &fakeCache{},
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

func TestValidateGeneratedApplications(t *testing.T) {

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

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
				Cache:            &fakeCache{},
				Generators:       map[string]generators.Generator{},
				ArgoDB:           &argoDBMock,
				ArgoCDNamespace:  "namespace",
				ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
				KubeClientset:    kubeclientset,
			}

			appSetInfo := v1alpha1.ApplicationSet{}

			validationErrors, _ := r.validateGeneratedApplications(context.TODO(), cc.apps, appSetInfo)
			var errorMessages []string
			for _, v := range validationErrors {
				errorMessages = append(errorMessages, v.Error())
			}

			if len(errorMessages) == 0 {
				assert.Equal(t, len(cc.expectedErrors), 0, "Expected errors but none were seen")
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
	assert.Nil(t, err)
	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

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

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
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
		Cache:    &fakeCache{},
		Generators: map[string]generators.Generator{
			"List": generators.NewListGenerator(),
		},
		ArgoDB:           &argoDBMock,
		ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
		KubeClientset:    kubeclientset,
		Policy:           v1alpha1.ApplicationsSyncPolicySync,
		ArgoCDNamespace:  "argocd",
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "argocd",
			Name:      "name",
		},
	}

	// Verify that on validation error, no error is returned, but the object is requeued
	res, err := r.Reconcile(context.Background(), req)
	assert.Nil(t, err)
	assert.True(t, res.RequeueAfter == ReconcileRequeueOnValidationError)

	var app v1alpha1.Application

	// make sure good app got created
	err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "good-project"}, &app)
	assert.NoError(t, err)
	assert.Equal(t, app.Name, "good-project")

	// make sure bad app was not created
	err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "bad-project"}, &app)
	assert.Error(t, err)
}

func TestReconcilerCreateAppsRecoveringRenderError(t *testing.T) {

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)
	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	project := v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "argocd"},
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
							Raw: []byte(`{"name": "very-good-app"}`),
						}, {
							Raw: []byte(`{"name": "bad-app"}`),
						}},
					},
				},
			},
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
					Name:      "{{ index (splitList \"-\" .name ) 2 }}",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSpec{
					Source:      &v1alpha1.ApplicationSource{RepoURL: "https://github.com/argoproj/argocd-example-apps", Path: "guestbook"},
					Project:     "default",
					Destination: v1alpha1.ApplicationDestination{Server: "https://kubernetes.default.svc"},
				},
			},
		},
	}

	kubeclientset := kubefake.NewSimpleClientset()
	argoDBMock := dbmocks.ArgoDB{}
	argoObjs := []runtime.Object{&project}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()

	r := ApplicationSetReconciler{
		Client:   client,
		Scheme:   scheme,
		Renderer: &utils.Render{},
		Recorder: record.NewFakeRecorder(1),
		Cache:    &fakeCache{},
		Generators: map[string]generators.Generator{
			"List": generators.NewListGenerator(),
		},
		ArgoDB:           &argoDBMock,
		ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
		KubeClientset:    kubeclientset,
		Policy:           v1alpha1.ApplicationsSyncPolicySync,
		ArgoCDNamespace:  "argocd",
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "argocd",
			Name:      "name",
		},
	}

	// Verify that on generatorsError, no error is returned, but the object is requeued
	res, err := r.Reconcile(context.Background(), req)
	assert.Nil(t, err)
	assert.True(t, res.RequeueAfter == ReconcileRequeueOnValidationError)

	var app v1alpha1.Application

	// make sure good app got created
	err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "app"}, &app)
	assert.NoError(t, err)
	assert.Equal(t, app.Name, "app")
}

func TestSetApplicationSetStatusCondition(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)
	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	appSet := v1alpha1.ApplicationSet{
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
	}

	appCondition := v1alpha1.ApplicationSetCondition{
		Type:    v1alpha1.ApplicationSetConditionResourcesUpToDate,
		Message: "All applications have been generated successfully",
		Reason:  v1alpha1.ApplicationSetReasonApplicationSetUpToDate,
		Status:  v1alpha1.ApplicationSetConditionStatusTrue,
	}

	kubeclientset := kubefake.NewSimpleClientset([]runtime.Object{}...)
	argoDBMock := dbmocks.ArgoDB{}
	argoObjs := []runtime.Object{}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()

	r := ApplicationSetReconciler{
		Client:   client,
		Scheme:   scheme,
		Renderer: &utils.Render{},
		Recorder: record.NewFakeRecorder(1),
		Cache:    &fakeCache{},
		Generators: map[string]generators.Generator{
			"List": generators.NewListGenerator(),
		},
		ArgoDB:           &argoDBMock,
		ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
		KubeClientset:    kubeclientset,
	}

	err = r.setApplicationSetStatusCondition(context.TODO(), &appSet, appCondition, true)
	assert.Nil(t, err)

	assert.Len(t, appSet.Status.Conditions, 3)
}

func applicationsUpdateSyncPolicyTest(t *testing.T, applicationsSyncPolicy v1alpha1.ApplicationsSyncPolicy, recordBuffer int, allowPolicyOverride bool) v1alpha1.Application {

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)
	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

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

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
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
		Cache:    &fakeCache{},
		Generators: map[string]generators.Generator{
			"List": generators.NewListGenerator(),
		},
		ArgoDB:               &argoDBMock,
		ArgoCDNamespace:      "argocd",
		ArgoAppClientset:     appclientset.NewSimpleClientset(argoObjs...),
		KubeClientset:        kubeclientset,
		Policy:               v1alpha1.ApplicationsSyncPolicySync,
		EnablePolicyOverride: allowPolicyOverride,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "argocd",
			Name:      "name",
		},
	}

	// Verify that on validation error, no error is returned, but the object is requeued
	resCreate, err := r.Reconcile(context.Background(), req)
	assert.Nil(t, err)
	assert.True(t, resCreate.RequeueAfter == 0)

	var app v1alpha1.Application

	// make sure good app got created
	err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "good-cluster"}, &app)
	assert.Nil(t, err)
	assert.Equal(t, app.Name, "good-cluster")

	// Update resource
	var retrievedApplicationSet v1alpha1.ApplicationSet
	err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "name"}, &retrievedApplicationSet)
	assert.Nil(t, err)

	retrievedApplicationSet.Spec.Template.Annotations = map[string]string{"annotation-key": "annotation-value"}
	retrievedApplicationSet.Spec.Template.Labels = map[string]string{"label-key": "label-value"}

	retrievedApplicationSet.Spec.Template.Spec.Source.Helm = &v1alpha1.ApplicationSourceHelm{
		Values: "global.test: test",
	}

	err = r.Client.Update(context.TODO(), &retrievedApplicationSet)
	assert.Nil(t, err)

	resUpdate, err := r.Reconcile(context.Background(), req)
	assert.Nil(t, err)

	err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "good-cluster"}, &app)
	assert.Nil(t, err)
	assert.True(t, resUpdate.RequeueAfter == 0)
	assert.Equal(t, app.Name, "good-cluster")

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
	assert.Nil(t, err)
	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

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

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
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
		Cache:    &fakeCache{},
		Generators: map[string]generators.Generator{
			"List": generators.NewListGenerator(),
		},
		ArgoDB:               &argoDBMock,
		ArgoCDNamespace:      "argocd",
		ArgoAppClientset:     appclientset.NewSimpleClientset(argoObjs...),
		KubeClientset:        kubeclientset,
		Policy:               v1alpha1.ApplicationsSyncPolicySync,
		EnablePolicyOverride: allowPolicyOverride,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "argocd",
			Name:      "name",
		},
	}

	// Verify that on validation error, no error is returned, but the object is requeued
	resCreate, err := r.Reconcile(context.Background(), req)
	assert.Nil(t, err)
	assert.True(t, resCreate.RequeueAfter == 0)

	var app v1alpha1.Application

	// make sure good app got created
	err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "good-cluster"}, &app)
	assert.Nil(t, err)
	assert.Equal(t, app.Name, "good-cluster")

	// Update resource
	var retrievedApplicationSet v1alpha1.ApplicationSet
	err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "name"}, &retrievedApplicationSet)
	assert.Nil(t, err)
	retrievedApplicationSet.Spec.Generators = []v1alpha1.ApplicationSetGenerator{
		{
			List: &v1alpha1.ListGenerator{
				Elements: []apiextensionsv1.JSON{},
			},
		},
	}

	err = r.Client.Update(context.TODO(), &retrievedApplicationSet)
	assert.Nil(t, err)

	resUpdate, err := r.Reconcile(context.Background(), req)
	assert.Nil(t, err)

	var apps v1alpha1.ApplicationList

	err = r.Client.List(context.TODO(), &apps)
	assert.Nil(t, err)
	assert.True(t, resUpdate.RequeueAfter == 0)

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

	assert.Equal(t, 0, len(apps.Items))
}

func TestDeletePerformedWithSyncPolicySync(t *testing.T) {

	applicationsSyncPolicy := v1alpha1.ApplicationsSyncPolicySync

	apps := applicationsDeleteSyncPolicyTest(t, applicationsSyncPolicy, 3, true)

	assert.Equal(t, 0, len(apps.Items))
}

func TestDeletePerformedWithSyncPolicyCreateOnlyAndAllowPolicyOverrideFalse(t *testing.T) {

	applicationsSyncPolicy := v1alpha1.ApplicationsSyncPolicyCreateOnly

	apps := applicationsDeleteSyncPolicyTest(t, applicationsSyncPolicy, 3, false)

	assert.Equal(t, 0, len(apps.Items))
}

// Test app generation from a go template application set using a pull request generator
func TestGenerateAppsUsingPullRequestGenerator(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	for _, cases := range []struct {
		name        string
		params      []map[string]interface{}
		template    v1alpha1.ApplicationSetTemplate
		expectedApp []v1alpha1.Application
	}{
		{
			name: "Generate an application from a go template application set manifest using a pull request generator",
			params: []map[string]interface{}{{
				"number":                                "1",
				"branch":                                "branch1",
				"branch_slug":                           "branchSlug1",
				"head_sha":                              "089d92cbf9ff857a39e6feccd32798ca700fb958",
				"head_short_sha":                        "089d92cb",
				"branch_slugify_default":                "feat/a_really+long_pull_request_name_to_test_argo_slugification_and_branch_name_shortening_feature",
				"branch_slugify_smarttruncate_disabled": "feat/areallylongpullrequestnametotestargoslugificationandbranchnameshorteningfeature",
				"branch_slugify_smarttruncate_enabled":  "feat/testwithsmarttruncateenabledramdomlonglistofcharacters",
				"labels":                                []string{"label1"}},
			},
			template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
					Name: "AppSet-{{.branch}}-{{.number}}",
					Labels: map[string]string{
						"app1":         "{{index .labels 0}}",
						"branch-test1": "AppSet-{{.branch_slugify_default | slugify }}",
						"branch-test2": "AppSet-{{.branch_slugify_smarttruncate_disabled | slugify 49 false }}",
						"branch-test3": "AppSet-{{.branch_slugify_smarttruncate_enabled | slugify 50 true }}",
					},
				},
				Spec: v1alpha1.ApplicationSpec{
					Source: &v1alpha1.ApplicationSource{
						RepoURL:        "https://testurl/testRepo",
						TargetRevision: "{{.head_short_sha}}",
					},
					Destination: v1alpha1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "AppSet-{{.branch_slug}}-{{.head_sha}}",
					},
				},
			},
			expectedApp: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "AppSet-branch1-1",
						Labels: map[string]string{
							"app1":         "label1",
							"branch-test1": "AppSet-feat-a-really-long-pull-request-name-to-test-argo",
							"branch-test2": "AppSet-feat-areallylongpullrequestnametotestargoslugific",
							"branch-test3": "AppSet-feat",
						},
					},
					Spec: v1alpha1.ApplicationSpec{
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://testurl/testRepo",
							TargetRevision: "089d92cb",
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "AppSet-branchSlug1-089d92cbf9ff857a39e6feccd32798ca700fb958",
						},
					},
				},
			},
		},
	} {

		t.Run(cases.name, func(t *testing.T) {

			generatorMock := generatorMock{}
			generator := v1alpha1.ApplicationSetGenerator{
				PullRequest: &v1alpha1.PullRequestGenerator{},
			}

			generatorMock.On("GenerateParams", &generator).
				Return(cases.params, nil)

			generatorMock.On("GetTemplate", &generator).
				Return(&cases.template, nil)

			appSetReconciler := ApplicationSetReconciler{
				Client:   client,
				Scheme:   scheme,
				Recorder: record.NewFakeRecorder(1),
				Cache:    &fakeCache{},
				Generators: map[string]generators.Generator{
					"PullRequest": &generatorMock,
				},
				Renderer:      &utils.Render{},
				KubeClientset: kubefake.NewSimpleClientset(),
			}

			gotApp, _, _ := appSetReconciler.generateApplications(log.NewEntry(log.StandardLogger()), v1alpha1.ApplicationSet{
				Spec: v1alpha1.ApplicationSetSpec{
					GoTemplate: true,
					Generators: []v1alpha1.ApplicationSetGenerator{{
						PullRequest: &v1alpha1.PullRequestGenerator{},
					}},
					Template: cases.template,
				},
			},
			)
			assert.EqualValues(t, cases.expectedApp[0].ObjectMeta.Name, gotApp[0].ObjectMeta.Name)
			assert.EqualValues(t, cases.expectedApp[0].Spec.Source.TargetRevision, gotApp[0].Spec.Source.TargetRevision)
			assert.EqualValues(t, cases.expectedApp[0].Spec.Destination.Namespace, gotApp[0].Spec.Destination.Namespace)
			assert.True(t, collections.StringMapsEqual(cases.expectedApp[0].ObjectMeta.Labels, gotApp[0].ObjectMeta.Labels))
		})
	}
}

func TestPolicies(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

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

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet).WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()

			r := ApplicationSetReconciler{
				Client:   client,
				Scheme:   scheme,
				Renderer: &utils.Render{},
				Recorder: record.NewFakeRecorder(10),
				Cache:    &fakeCache{},
				Generators: map[string]generators.Generator{
					"List": generators.NewListGenerator(),
				},
				ArgoDB:           &argoDBMock,
				ArgoCDNamespace:  "argocd",
				ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
				KubeClientset:    kubeclientset,
				Policy:           policy,
			}

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "argocd",
					Name:      "name",
				},
			}

			// Check if Application is created
			res, err := r.Reconcile(context.Background(), req)
			assert.Nil(t, err)
			assert.True(t, res.RequeueAfter == 0)

			var app v1alpha1.Application
			err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "my-app"}, &app)
			assert.NoError(t, err)
			assert.Equal(t, app.Annotations["key"], "value")

			// Check if Application is updated
			app.Annotations["key"] = "edited"
			err = r.Client.Update(context.TODO(), &app)
			assert.NoError(t, err)

			res, err = r.Reconcile(context.Background(), req)
			assert.Nil(t, err)
			assert.True(t, res.RequeueAfter == 0)

			err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "my-app"}, &app)
			assert.NoError(t, err)

			if c.allowedUpdate {
				assert.Equal(t, app.Annotations["key"], "value")
			} else {
				assert.Equal(t, app.Annotations["key"], "edited")
			}

			// Check if Application is deleted
			err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "name"}, &appSet)
			assert.NoError(t, err)
			appSet.Spec.Generators[0] = v1alpha1.ApplicationSetGenerator{
				List: &v1alpha1.ListGenerator{
					Elements: []apiextensionsv1.JSON{},
				},
			}
			err = r.Client.Update(context.TODO(), &appSet)
			assert.NoError(t, err)

			res, err = r.Reconcile(context.Background(), req)
			assert.Nil(t, err)
			assert.True(t, res.RequeueAfter == 0)

			err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "my-app"}, &app)
			assert.NoError(t, err)
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
	assert.Nil(t, err)
	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

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

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&cc.appSet).Build()

			r := ApplicationSetReconciler{
				Client:   client,
				Scheme:   scheme,
				Renderer: &utils.Render{},
				Recorder: record.NewFakeRecorder(1),
				Cache:    &fakeCache{},
				Generators: map[string]generators.Generator{
					"List": generators.NewListGenerator(),
				},
				ArgoDB:           &argoDBMock,
				ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
				KubeClientset:    kubeclientset,
			}

			err = r.setAppSetApplicationStatus(context.TODO(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.appStatuses)
			assert.Nil(t, err)

			assert.Equal(t, cc.expectedAppStatuses, cc.appSet.Status.ApplicationStatus)
		})
	}
}

func TestBuildAppDependencyList(t *testing.T) {

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

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
				Cache:            &fakeCache{},
				Generators:       map[string]generators.Generator{},
				ArgoDB:           &argoDBMock,
				ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
				KubeClientset:    kubeclientset,
			}

			appDependencyList, appStepMap, err := r.buildAppDependencyList(log.NewEntry(log.StandardLogger()), cc.appSet, cc.apps)
			assert.Equal(t, err, nil, "expected no errors, but errors occured")
			assert.Equal(t, cc.expectedList, appDependencyList, "expected appDependencyList did not match actual")
			assert.Equal(t, cc.expectedStepMap, appStepMap, "expected appStepMap did not match actual")
		})
	}
}

func TestBuildAppSyncMap(t *testing.T) {

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

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
				Cache:            &fakeCache{},
				Generators:       map[string]generators.Generator{},
				ArgoDB:           &argoDBMock,
				ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
				KubeClientset:    kubeclientset,
			}

			appSyncMap, err := r.buildAppSyncMap(context.TODO(), cc.appSet, cc.appDependencyList, cc.appMap)
			assert.Equal(t, err, nil, "expected no errors, but errors occured")
			assert.Equal(t, cc.expectedMap, appSyncMap, "expected appSyncMap did not match actual")
		})
	}
}

func TestUpdateApplicationSetApplicationStatus(t *testing.T) {

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

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
					Application: "app1",
					Message:     "Application resource is already Healthy, updating status from Waiting to Healthy.",
					Status:      "Healthy",
					Step:        "1",
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
					Application: "app1",
					Message:     "Application resource is already Healthy, updating status from Waiting to Healthy.",
					Status:      "Healthy",
					Step:        "1",
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
							Application: "app1",
							Message:     "",
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
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application: "app1",
					Message:     "Application has pending changes, setting status to Waiting.",
					Status:      "Waiting",
					Step:        "1",
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
							Application: "app1",
							Message:     "",
							Status:      "Pending",
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
							Status: health.HealthStatusProgressing,
						},
					},
				},
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application: "app1",
					Message:     "Application resource became Progressing, updating status from Pending to Progressing.",
					Status:      "Progressing",
					Step:        "1",
				},
			},
		},
		{
			name: "progresses a pending syncing application to progressing",
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
							Message:     "",
							Status:      "Pending",
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
					Application: "app1",
					Message:     "Application resource became Progressing, updating status from Pending to Progressing.",
					Status:      "Progressing",
					Step:        "1",
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
							Application: "app1",
							Message:     "",
							Status:      "Progressing",
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
					Application: "app1",
					Message:     "Application resource became Healthy, updating status from Progressing to Healthy.",
					Status:      "Healthy",
					Step:        "1",
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
							Application: "app1",
							Message:     "",
							Status:      "Waiting",
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
					Application: "app1",
					Message:     "Application resource is already Healthy, updating status from Waiting to Healthy.",
					Status:      "Healthy",
					Step:        "1",
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
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeOutOfSync,
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
					Application: "app1",
					Message:     "No Application status found, defaulting status to Waiting.",
					Status:      "Waiting",
					Step:        "2",
				},
			},
		},
		{
			name: "progresses a pending application with a successful sync to progressing",
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
							Message: "",
							Status:  "Pending",
							Step:    "1",
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
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application: "app1",
					Message:     "Application resource completed a sync successfully, updating status from Pending to Progressing.",
					Status:      "Progressing",
					Step:        "1",
				},
			},
		},
		{
			name: "progresses a pending application with a successful sync <1s ago to progressing",
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
							Message: "",
							Status:  "Pending",
							Step:    "1",
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
						},
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			expectedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application: "app1",
					Message:     "Application resource completed a sync successfully, updating status from Pending to Progressing.",
					Status:      "Progressing",
					Step:        "1",
				},
			},
		},
		{
			name: "does not progresses a pending application with an old successful sync to progressing",
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
							Message: "Application moved to Pending status, watching for the Application resource to start Progressing.",
							Status:  "Pending",
							Step:    "1",
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
								Time: time.Now().Add(time.Duration(-11) * time.Second),
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
					Application: "app1",
					Message:     "Application moved to Pending status, watching for the Application resource to start Progressing.",
					Status:      "Pending",
					Step:        "1",
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
							Application: "app1",
							Message:     "Application has pending changes, setting status to Waiting.",
							Status:      "Waiting",
							Step:        "1",
						},
						{
							Application: "app2",
							Message:     "Application has pending changes, setting status to Waiting.",
							Status:      "Waiting",
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
					Application: "app1",
					Message:     "Application resource is already Healthy, updating status from Waiting to Healthy.",
					Status:      "Healthy",
					Step:        "1",
				},
			},
		},
	} {

		t.Run(cc.name, func(t *testing.T) {

			kubeclientset := kubefake.NewSimpleClientset([]runtime.Object{}...)
			argoDBMock := dbmocks.ArgoDB{}
			argoObjs := []runtime.Object{}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&cc.appSet).Build()

			r := ApplicationSetReconciler{
				Client:           client,
				Scheme:           scheme,
				Recorder:         record.NewFakeRecorder(1),
				Cache:            &fakeCache{},
				Generators:       map[string]generators.Generator{},
				ArgoDB:           &argoDBMock,
				ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
				KubeClientset:    kubeclientset,
			}

			appStatuses, err := r.updateApplicationSetApplicationStatus(context.TODO(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.apps, cc.appStepMap)

			// opt out of testing the LastTransitionTime is accurate
			for i := range appStatuses {
				appStatuses[i].LastTransitionTime = nil
			}

			assert.Equal(t, err, nil, "expected no errors, but errors occured")
			assert.Equal(t, cc.expectedAppStatus, appStatuses, "expected appStatuses did not match actual")
		})
	}
}

func TestUpdateApplicationSetApplicationStatusProgress(t *testing.T) {

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

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
							Application: "app1",
							Message:     "Application is out of date with the current AppSet generation, setting status to Waiting.",
							Status:      "Waiting",
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

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&cc.appSet).Build()

			r := ApplicationSetReconciler{
				Client:           client,
				Scheme:           scheme,
				Recorder:         record.NewFakeRecorder(1),
				Cache:            &fakeCache{},
				Generators:       map[string]generators.Generator{},
				ArgoDB:           &argoDBMock,
				ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
				KubeClientset:    kubeclientset,
			}

			appStatuses, err := r.updateApplicationSetApplicationStatusProgress(context.TODO(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.appSyncMap, cc.appStepMap, cc.appMap)

			// opt out of testing the LastTransitionTime is accurate
			for i := range appStatuses {
				appStatuses[i].LastTransitionTime = nil
			}

			assert.Equal(t, err, nil, "expected no errors, but errors occured")
			assert.Equal(t, cc.expectedAppStatus, appStatuses, "expected appStatuses did not match actual")
		})
	}
}

func TestUpdateResourceStatus(t *testing.T) {

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

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

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&cc.appSet).Build()

			r := ApplicationSetReconciler{
				Client:           client,
				Scheme:           scheme,
				Recorder:         record.NewFakeRecorder(1),
				Cache:            &fakeCache{},
				Generators:       map[string]generators.Generator{},
				ArgoDB:           &argoDBMock,
				ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
				KubeClientset:    kubeclientset,
			}

			err := r.updateResourcesStatus(context.TODO(), log.NewEntry(log.StandardLogger()), &cc.appSet, cc.apps)

			assert.Equal(t, err, nil, "expected no errors, but errors occured")
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
		{name: "ApplicationHealthStatusDiff", args: args{e: event.UpdateEvent{
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
		{name: "ApplicationSyncStatusDiff", args: args{e: event.UpdateEvent{
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
		{name: "ApplicationOperationStateDiff", args: args{e: event.UpdateEvent{
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
		{name: "ApplicationOperationStartedAtDiff", args: args{e: event.UpdateEvent{
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
		{name: "ApplicationDestinationSame", args: args{e: event.UpdateEvent{
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
		{name: "ApplicationDestinationDiff", args: args{e: event.UpdateEvent{
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
