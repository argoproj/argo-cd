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
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	crtclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/argoproj/argo-cd/v2/applicationset/generators"
	"github.com/argoproj/argo-cd/v2/applicationset/utils"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	dbmocks "github.com/argoproj/argo-cd/v2/util/db/mocks"
)

type generatorMock struct {
	mock.Mock
}

func (g *generatorMock) GetTemplate(appSetGenerator *argov1alpha1.ApplicationSetGenerator) *argov1alpha1.ApplicationSetTemplate {
	args := g.Called(appSetGenerator)

	return args.Get(0).(*argov1alpha1.ApplicationSetTemplate)
}

func (g *generatorMock) GenerateParams(appSetGenerator *argov1alpha1.ApplicationSetGenerator, _ *argov1alpha1.ApplicationSet) ([]map[string]interface{}, error) {
	args := g.Called(appSetGenerator)

	return args.Get(0).([]map[string]interface{}), args.Error(1)
}

type rendererMock struct {
	mock.Mock
}

func (g *generatorMock) GetRequeueAfter(appSetGenerator *argov1alpha1.ApplicationSetGenerator) time.Duration {
	args := g.Called(appSetGenerator)

	return args.Get(0).(time.Duration)
}

func (r *rendererMock) RenderTemplateParams(tmpl *argov1alpha1.Application, syncPolicy *argov1alpha1.ApplicationSetSyncPolicy, params map[string]interface{}, useGoTemplate bool) (*argov1alpha1.Application, error) {
	args := r.Called(tmpl, params, useGoTemplate)

	if args.Error(1) != nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*argov1alpha1.Application), args.Error(1)

}

func TestExtractApplications(t *testing.T) {
	scheme := runtime.NewScheme()
	err := argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	for _, c := range []struct {
		name                string
		params              []map[string]interface{}
		template            argov1alpha1.ApplicationSetTemplate
		generateParamsError error
		rendererError       error
		expectErr           bool
		expectedReason      v1alpha1.ApplicationSetReasonType
	}{
		{
			name:   "Generate two applications",
			params: []map[string]interface{}{{"name": "app1"}, {"name": "app2"}},
			template: argov1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: argov1alpha1.ApplicationSetTemplateMeta{
					Name:      "name",
					Namespace: "namespace",
					Labels:    map[string]string{"label_name": "label_value"},
				},
				Spec: argov1alpha1.ApplicationSpec{},
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
			template: argov1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: argov1alpha1.ApplicationSetTemplateMeta{
					Name:      "name",
					Namespace: "namespace",
					Labels:    map[string]string{"label_name": "label_value"},
				},
				Spec: argov1alpha1.ApplicationSpec{},
			},
			rendererError:  fmt.Errorf("error"),
			expectErr:      true,
			expectedReason: v1alpha1.ApplicationSetReasonRenderTemplateParamsError,
		},
	} {
		cc := c
		app := argov1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}

		t.Run(cc.name, func(t *testing.T) {

			appSet := &argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(appSet).Build()

			generatorMock := generatorMock{}
			generator := argov1alpha1.ApplicationSetGenerator{
				List: &argov1alpha1.ListGenerator{},
			}

			generatorMock.On("GenerateParams", &generator).
				Return(cc.params, cc.generateParamsError)

			generatorMock.On("GetTemplate", &generator).
				Return(&argov1alpha1.ApplicationSetTemplate{})

			rendererMock := rendererMock{}

			var expectedApps []argov1alpha1.Application

			if cc.generateParamsError == nil {
				for _, p := range cc.params {

					if cc.rendererError != nil {
						rendererMock.On("RenderTemplateParams", getTempApplication(cc.template), p, false).
							Return(nil, cc.rendererError)
					} else {
						rendererMock.On("RenderTemplateParams", getTempApplication(cc.template), p, false).
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
			}

			got, reason, err := r.generateApplications(argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Generators: []argov1alpha1.ApplicationSetGenerator{generator},
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
	_ = argov1alpha1.AddToScheme(scheme)
	_ = argov1alpha1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	for _, c := range []struct {
		name             string
		params           []map[string]interface{}
		template         argov1alpha1.ApplicationSetTemplate
		overrideTemplate argov1alpha1.ApplicationSetTemplate
		expectedMerged   argov1alpha1.ApplicationSetTemplate
		expectedApps     []argov1alpha1.Application
	}{
		{
			name:   "Generate app",
			params: []map[string]interface{}{{"name": "app1"}},
			template: argov1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: argov1alpha1.ApplicationSetTemplateMeta{
					Name:      "name",
					Namespace: "namespace",
					Labels:    map[string]string{"label_name": "label_value"},
				},
				Spec: argov1alpha1.ApplicationSpec{},
			},
			overrideTemplate: argov1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: argov1alpha1.ApplicationSetTemplateMeta{
					Name:   "test",
					Labels: map[string]string{"foo": "bar"},
				},
				Spec: argov1alpha1.ApplicationSpec{},
			},
			expectedMerged: argov1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: argov1alpha1.ApplicationSetTemplateMeta{
					Name:      "test",
					Namespace: "namespace",
					Labels:    map[string]string{"label_name": "label_value", "foo": "bar"},
				},
				Spec: argov1alpha1.ApplicationSpec{},
			},
			expectedApps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
						Labels:    map[string]string{"foo": "bar"},
					},
					Spec: argov1alpha1.ApplicationSpec{},
				},
			},
		},
	} {
		cc := c

		t.Run(cc.name, func(t *testing.T) {

			generatorMock := generatorMock{}
			generator := argov1alpha1.ApplicationSetGenerator{
				List: &argov1alpha1.ListGenerator{},
			}

			generatorMock.On("GenerateParams", &generator).
				Return(cc.params, nil)

			generatorMock.On("GetTemplate", &generator).
				Return(&cc.overrideTemplate)

			rendererMock := rendererMock{}

			rendererMock.On("RenderTemplateParams", getTempApplication(cc.expectedMerged), cc.params[0], false).
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

			got, _, _ := r.generateApplications(argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Generators: []argov1alpha1.ApplicationSetGenerator{generator},
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
	err := argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	for _, c := range []struct {
		// name is human-readable test name
		name string
		// appSet is the ApplicationSet we are generating resources for
		appSet argov1alpha1.ApplicationSet
		// existingApps are the apps that already exist on the cluster
		existingApps []argov1alpha1.Application
		// desiredApps are the generated apps to create/update
		desiredApps []argov1alpha1.Application
		// expected is what we expect the cluster Applications to look like, after createOrUpdateInCluster
		expected []argov1alpha1.Application
	}{
		{
			name: "Create an app that doesn't exist",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
			},
			existingApps: nil,
			desiredApps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
				},
			},
			expected: []argov1alpha1.Application{
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
				},
			},
		},
		{
			name: "Update an existing app with a different project name",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Template: argov1alpha1.ApplicationSetTemplate{
						Spec: argov1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			},
			existingApps: []argov1alpha1.Application{
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
					Spec: argov1alpha1.ApplicationSpec{
						Project: "test",
					},
				},
			},
			desiredApps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			expected: []argov1alpha1.Application{
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
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
		},
		{
			name: "Create a new app and check it doesn't replace the existing app",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Template: argov1alpha1.ApplicationSetTemplate{
						Spec: argov1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			},
			existingApps: []argov1alpha1.Application{
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
					Spec: argov1alpha1.ApplicationSpec{
						Project: "test",
					},
				},
			},
			desiredApps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app2",
					},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			expected: []argov1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Application",
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app2",
						Namespace:       "namespace",
						ResourceVersion: "1",
					},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
		},
		{
			name: "Ensure that labels and annotations are added (via update) into an exiting application",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Template: argov1alpha1.ApplicationSetTemplate{
						Spec: argov1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			},
			existingApps: []argov1alpha1.Application{
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
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			desiredApps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "app1",
						Labels:      map[string]string{"label-key": "label-value"},
						Annotations: map[string]string{"annot-key": "annot-value"},
					},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			expected: []argov1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Application",
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						Labels:          map[string]string{"label-key": "label-value"},
						Annotations:     map[string]string{"annot-key": "annot-value"},
						ResourceVersion: "3",
					},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
		},
		{
			name: "Ensure that labels and annotations are removed from an existing app",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Template: argov1alpha1.ApplicationSetTemplate{
						Spec: argov1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			},
			existingApps: []argov1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Application",
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "2",
						Labels:          map[string]string{"label-key": "label-value"},
						Annotations:     map[string]string{"annot-key": "annot-value"},
					},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			desiredApps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			expected: []argov1alpha1.Application{
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
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
		},
		{
			name: "Ensure that status and operation fields are not overridden by an update, when removing labels/annotations",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Template: argov1alpha1.ApplicationSetTemplate{
						Spec: argov1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			},
			existingApps: []argov1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Application",
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "2",
						Labels:          map[string]string{"label-key": "label-value"},
						Annotations:     map[string]string{"annot-key": "annot-value"},
					},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
					Status: argov1alpha1.ApplicationStatus{
						Resources: []argov1alpha1.ResourceStatus{{Name: "sample-name"}},
					},
					Operation: &argov1alpha1.Operation{
						Sync: &argov1alpha1.SyncOperation{Revision: "sample-revision"},
					},
				},
			},
			desiredApps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			expected: []argov1alpha1.Application{
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
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
					Status: argov1alpha1.ApplicationStatus{
						Resources: []argov1alpha1.ResourceStatus{{Name: "sample-name"}},
					},
					Operation: &argov1alpha1.Operation{
						Sync: &argov1alpha1.SyncOperation{Revision: "sample-revision"},
					},
				},
			},
		},
		{
			name: "Ensure that status and operation fields are not overridden by an update, when removing labels/annotations and adding other fields",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Template: argov1alpha1.ApplicationSetTemplate{
						Spec: argov1alpha1.ApplicationSpec{
							Project:     "project",
							Source:      argov1alpha1.ApplicationSource{Path: "path", TargetRevision: "revision", RepoURL: "repoURL"},
							Destination: argov1alpha1.ApplicationDestination{Server: "server", Namespace: "namespace"},
						},
					},
				},
			},
			existingApps: []argov1alpha1.Application{
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
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
					Status: argov1alpha1.ApplicationStatus{
						Resources: []argov1alpha1.ResourceStatus{{Name: "sample-name"}},
					},
					Operation: &argov1alpha1.Operation{
						Sync: &argov1alpha1.SyncOperation{Revision: "sample-revision"},
					},
				},
			},
			desiredApps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "app1",
						Labels:      map[string]string{"label-key": "label-value"},
						Annotations: map[string]string{"annot-key": "annot-value"},
					},
					Spec: argov1alpha1.ApplicationSpec{
						Project:     "project",
						Source:      argov1alpha1.ApplicationSource{Path: "path", TargetRevision: "revision", RepoURL: "repoURL"},
						Destination: argov1alpha1.ApplicationDestination{Server: "server", Namespace: "namespace"},
					},
				},
			},
			expected: []argov1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Application",
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						Labels:          map[string]string{"label-key": "label-value"},
						Annotations:     map[string]string{"annot-key": "annot-value"},
						ResourceVersion: "3",
					},
					Spec: argov1alpha1.ApplicationSpec{
						Project:     "project",
						Source:      argov1alpha1.ApplicationSource{Path: "path", TargetRevision: "revision", RepoURL: "repoURL"},
						Destination: argov1alpha1.ApplicationDestination{Server: "server", Namespace: "namespace"},
					},
					Status: argov1alpha1.ApplicationStatus{
						Resources: []argov1alpha1.ResourceStatus{{Name: "sample-name"}},
					},
					Operation: &argov1alpha1.Operation{
						Sync: &argov1alpha1.SyncOperation{Revision: "sample-revision"},
					},
				},
			},
		},
		{
			name: "Ensure that argocd notifications state and refresh annotation is preserved from an existing app",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Template: argov1alpha1.ApplicationSetTemplate{
						Spec: argov1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			},
			existingApps: []argov1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Application",
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app1",
						Namespace:       "namespace",
						ResourceVersion: "2",
						Labels:          map[string]string{"label-key": "label-value"},
						Annotations: map[string]string{
							"annot-key":                       "annot-value",
							NotifiedAnnotationKey:             `{"b620d4600c771a6f4cxxxxxxx:on-deployed:[0].y7b5sbwa2Q329JYHxxxxxx-fBs:slack:slack-test":1617144614}`,
							argov1alpha1.AnnotationKeyRefresh: string(argov1alpha1.RefreshTypeNormal),
						},
					},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			desiredApps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			expected: []argov1alpha1.Application{
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
							NotifiedAnnotationKey:             `{"b620d4600c771a6f4cxxxxxxx:on-deployed:[0].y7b5sbwa2Q329JYHxxxxxx-fBs:slack:slack-test":1617144614}`,
							argov1alpha1.AnnotationKeyRefresh: string(argov1alpha1.RefreshTypeNormal),
						},
					},
					Spec: argov1alpha1.ApplicationSpec{
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

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).Build()

			r := ApplicationSetReconciler{
				Client:   client,
				Scheme:   scheme,
				Recorder: record.NewFakeRecorder(len(initObjs) + len(c.expected)),
			}

			err = r.createOrUpdateInCluster(context.TODO(), c.appSet, c.desiredApps)
			assert.Nil(t, err)

			for _, obj := range c.expected {
				got := &argov1alpha1.Application{}
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

func TestRemoveFinalizerOnInvalidDestination_FinalizerTypes(t *testing.T) {

	scheme := runtime.NewScheme()
	err := argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = argov1alpha1.AddToScheme(scheme)
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
			existingFinalizers: []string{argov1alpha1.ResourcesFinalizerName},
			expectedFinalizers: nil,
		},
		{
			name:               "contains only non-argo finalizer",
			existingFinalizers: []string{"non-argo-finalizer"},
			expectedFinalizers: []string{"non-argo-finalizer"},
		},
		{
			name:               "contains both argo and non-argo finalizer",
			existingFinalizers: []string{"non-argo-finalizer", argov1alpha1.ResourcesFinalizerName},
			expectedFinalizers: []string{"non-argo-finalizer"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {

			appSet := argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Template: argov1alpha1.ApplicationSetTemplate{
						Spec: argov1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			}

			app := argov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "app1",
					Finalizers: c.existingFinalizers,
				},
				Spec: argov1alpha1.ApplicationSpec{
					Project: "project",
					Source:  argov1alpha1.ApplicationSource{Path: "path", TargetRevision: "revision", RepoURL: "repoURL"},
					// Destination is always invalid, for this test:
					Destination: argov1alpha1.ApplicationDestination{Name: "my-cluster", Namespace: "namespace"},
				},
			}

			initObjs := []crtclient.Object{&app, &appSet}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).Build()
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

			retrievedApp := argov1alpha1.Application{}
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
	err := argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	for _, c := range []struct {
		// name is human-readable test name
		name                   string
		destinationField       argov1alpha1.ApplicationDestination
		expectFinalizerRemoved bool
	}{
		{
			name: "invalid cluster: empty destination",
			destinationField: argov1alpha1.ApplicationDestination{
				Namespace: "namespace",
			},
			expectFinalizerRemoved: true,
		},
		{
			name: "invalid cluster: invalid server url",
			destinationField: argov1alpha1.ApplicationDestination{
				Namespace: "namespace",
				Server:    "https://1.2.3.4",
			},
			expectFinalizerRemoved: true,
		},
		{
			name: "invalid cluster: invalid cluster name",
			destinationField: argov1alpha1.ApplicationDestination{
				Namespace: "namespace",
				Name:      "invalid-cluster",
			},
			expectFinalizerRemoved: true,
		},
		{
			name: "invalid cluster by both valid",
			destinationField: argov1alpha1.ApplicationDestination{
				Namespace: "namespace",
				Name:      "mycluster2",
				Server:    "https://kubernetes.default.svc",
			},
			expectFinalizerRemoved: true,
		},
		{
			name: "invalid cluster by both invalid",
			destinationField: argov1alpha1.ApplicationDestination{
				Namespace: "namespace",
				Name:      "mycluster3",
				Server:    "https://4.5.6.7",
			},
			expectFinalizerRemoved: true,
		},
		{
			name: "valid cluster by name",
			destinationField: argov1alpha1.ApplicationDestination{
				Namespace: "namespace",
				Name:      "mycluster2",
			},
			expectFinalizerRemoved: false,
		},
		{
			name: "valid cluster by server",
			destinationField: argov1alpha1.ApplicationDestination{
				Namespace: "namespace",
				Server:    "https://kubernetes.default.svc",
			},
			expectFinalizerRemoved: false,
		},
	} {

		t.Run(c.name, func(t *testing.T) {

			appSet := argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Template: argov1alpha1.ApplicationSetTemplate{
						Spec: argov1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			}

			app := argov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "app1",
					Finalizers: []string{argov1alpha1.ResourcesFinalizerName},
				},
				Spec: argov1alpha1.ApplicationSpec{
					Project:     "project",
					Source:      argov1alpha1.ApplicationSource{Path: "path", TargetRevision: "revision", RepoURL: "repoURL"},
					Destination: c.destinationField,
				},
			}

			initObjs := []crtclient.Object{&app, &appSet}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).Build()
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

			retrievedApp := argov1alpha1.Application{}
			err = client.Get(context.Background(), crtclient.ObjectKeyFromObject(&app), &retrievedApp)
			assert.NoError(t, err, "Unexpected error")

			finalizerRemoved := len(retrievedApp.Finalizers) == 0

			assert.True(t, c.expectFinalizerRemoved == finalizerRemoved)

			bytes, _ := json.MarshalIndent(retrievedApp, "", "  ")
			t.Log("Contents of app after call:", string(bytes))

		})
	}
}

func TestCreateApplications(t *testing.T) {

	scheme := runtime.NewScheme()
	err := argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	for _, c := range []struct {
		appSet     argov1alpha1.ApplicationSet
		existsApps []argov1alpha1.Application
		apps       []argov1alpha1.Application
		expected   []argov1alpha1.Application
	}{
		{
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
			},
			existsApps: nil,
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
				},
			},
			expected: []argov1alpha1.Application{
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
				},
			},
		},
		{
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Template: argov1alpha1.ApplicationSetTemplate{
						Spec: argov1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			},
			existsApps: []argov1alpha1.Application{
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
					Spec: argov1alpha1.ApplicationSpec{
						Project: "test",
					},
				},
			},
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			expected: []argov1alpha1.Application{
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
					Spec: argov1alpha1.ApplicationSpec{
						Project: "test",
					},
				},
			},
		},
		{
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Template: argov1alpha1.ApplicationSetTemplate{
						Spec: argov1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			},
			existsApps: []argov1alpha1.Application{
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
					Spec: argov1alpha1.ApplicationSpec{
						Project: "test",
					},
				},
			},
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app2",
					},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			expected: []argov1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Application",
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "app2",
						Namespace:       "namespace",
						ResourceVersion: "1",
					},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
		},
	} {
		initObjs := []crtclient.Object{&c.appSet}
		for _, a := range c.existsApps {
			err = controllerutil.SetControllerReference(&c.appSet, &a, scheme)
			assert.Nil(t, err)
			initObjs = append(initObjs, &a)
		}

		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).Build()

		r := ApplicationSetReconciler{
			Client:   client,
			Scheme:   scheme,
			Recorder: record.NewFakeRecorder(len(initObjs) + len(c.expected)),
		}

		err = r.createInCluster(context.TODO(), c.appSet, c.apps)
		assert.Nil(t, err)

		for _, obj := range c.expected {
			got := &argov1alpha1.Application{}
			_ = client.Get(context.Background(), crtclient.ObjectKey{
				Namespace: obj.Namespace,
				Name:      obj.Name,
			}, got)

			err = controllerutil.SetControllerReference(&c.appSet, &obj, r.Scheme)
			assert.Nil(t, err)

			assert.Equal(t, obj, *got)
		}
	}

}

func TestDeleteInCluster(t *testing.T) {

	scheme := runtime.NewScheme()
	err := argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)
	err = argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	for _, c := range []struct {
		// appSet is the application set on which the delete function is called
		appSet argov1alpha1.ApplicationSet
		// existingApps is the current state of Applications on the cluster
		existingApps []argov1alpha1.Application
		// desireApps is the apps generated by the generator that we wish to keep alive
		desiredApps []argov1alpha1.Application
		// expected is the list of applications that we expect to exist after calling delete
		expected []argov1alpha1.Application
		// notExpected is the list of applications that we expect not to exist after calling delete
		notExpected []argov1alpha1.Application
	}{
		{
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Template: argov1alpha1.ApplicationSetTemplate{
						Spec: argov1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			},
			existingApps: []argov1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Application",
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "delete",
						Namespace:       "namespace",
						ResourceVersion: "2",
					},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Application",
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "keep",
						Namespace:       "namespace",
						ResourceVersion: "2",
					},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			desiredApps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "keep",
					},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			expected: []argov1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Application",
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "keep",
						Namespace:       "namespace",
						ResourceVersion: "2",
					},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "project",
					},
				},
			},
			notExpected: []argov1alpha1.Application{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Application",
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "delete",
						Namespace:       "namespace",
						ResourceVersion: "1",
					},
					Spec: argov1alpha1.ApplicationSpec{
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

		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).Build()

		r := ApplicationSetReconciler{
			Client:        client,
			Scheme:        scheme,
			Recorder:      record.NewFakeRecorder(len(initObjs) + len(c.expected)),
			KubeClientset: kubefake.NewSimpleClientset(),
		}

		err = r.deleteInCluster(context.TODO(), c.appSet, c.desiredApps)
		assert.Nil(t, err)

		// For each of the expected objects, verify they exist on the cluster
		for _, obj := range c.expected {
			got := &argov1alpha1.Application{}
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
			got := &argov1alpha1.Application{}
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
	err := argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)
	err = argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	generator := argov1alpha1.ApplicationSetGenerator{
		List:     &argov1alpha1.ListGenerator{},
		Git:      &argov1alpha1.GitGenerator{},
		Clusters: &argov1alpha1.ClusterGenerator{},
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
		Generators: map[string]generators.Generator{
			"List":     &generatorMock10,
			"Git":      &generatorMock1,
			"Clusters": &generatorMock1,
		},
	}

	got := r.getMinRequeueAfter(&argov1alpha1.ApplicationSet{
		Spec: argov1alpha1.ApplicationSetSpec{
			Generators: []argov1alpha1.ApplicationSetGenerator{generator},
		},
	})

	assert.Equal(t, time.Duration(1)*time.Second, got)
}

func TestValidateGeneratedApplications(t *testing.T) {

	scheme := runtime.NewScheme()
	err := argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Valid cluster
	myCluster := argov1alpha1.Cluster{
		Server: "https://kubernetes.default.svc",
		Name:   "my-cluster",
	}

	// Valid project
	myProject := &argov1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "namespace"},
		Spec: argov1alpha1.AppProjectSpec{
			SourceRepos: []string{"*"},
			Destinations: []argov1alpha1.ApplicationDestination{
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
		apps             []argov1alpha1.Application
		expectedErrors   []string
		validationErrors map[int]error
	}{
		{
			name: "valid app should return true",
			apps: []argov1alpha1.Application{
				{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: argov1alpha1.ApplicationSource{
							RepoURL:        "https://url",
							Path:           "/",
							TargetRevision: "HEAD",
						},
						Destination: argov1alpha1.ApplicationDestination{
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
			apps: []argov1alpha1.Application{
				{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: argov1alpha1.ApplicationSource{
							RepoURL:        "https://url",
							Path:           "/",
							TargetRevision: "HEAD",
						},
						Destination: argov1alpha1.ApplicationDestination{
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
			apps: []argov1alpha1.Application{
				{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "DOES-NOT-EXIST",
						Source: argov1alpha1.ApplicationSource{
							RepoURL:        "https://url",
							Path:           "/",
							TargetRevision: "HEAD",
						},
						Destination: argov1alpha1.ApplicationDestination{
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
			apps: []argov1alpha1.Application{
				{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: argov1alpha1.ApplicationSource{
							RepoURL:        "https://url",
							Path:           "/",
							TargetRevision: "HEAD",
						},
						Destination: argov1alpha1.ApplicationDestination{
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
			apps: []argov1alpha1.Application{
				{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: argov1alpha1.ApplicationSource{
							RepoURL:        "https://url",
							Path:           "/",
							TargetRevision: "HEAD",
						},
						Destination: argov1alpha1.ApplicationDestination{
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
			argoDBMock.On("ListClusters", mock.Anything).Return(&argov1alpha1.ClusterList{Items: []argov1alpha1.Cluster{
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
				ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
				KubeClientset:    kubeclientset,
			}

			appSetInfo := argov1alpha1.ApplicationSet{}

			validationErrors, _ := r.validateGeneratedApplications(context.TODO(), cc.apps, appSetInfo, "namespace")
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

func TestReconcilerValidationErrorBehaviour(t *testing.T) {

	scheme := runtime.NewScheme()
	err := argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)
	err = argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	defaultProject := argov1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "argocd"},
		Spec:       argov1alpha1.AppProjectSpec{SourceRepos: []string{"*"}, Destinations: []argov1alpha1.ApplicationDestination{{Namespace: "*", Server: "https://good-cluster"}}},
	}
	appSet := argov1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "argocd",
		},
		Spec: argov1alpha1.ApplicationSetSpec{
			GoTemplate: true,
			Generators: []argov1alpha1.ApplicationSetGenerator{
				{
					List: &argov1alpha1.ListGenerator{
						Elements: []apiextensionsv1.JSON{{
							Raw: []byte(`{"cluster": "good-cluster","url": "https://good-cluster"}`),
						}, {
							Raw: []byte(`{"cluster": "bad-cluster","url": "https://bad-cluster"}`),
						}},
					},
				},
			},
			Template: argov1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: argov1alpha1.ApplicationSetTemplateMeta{
					Name:      "{{.cluster}}",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSpec{
					Source:      argov1alpha1.ApplicationSource{RepoURL: "https://github.com/argoproj/argocd-example-apps", Path: "guestbook"},
					Project:     "default",
					Destination: argov1alpha1.ApplicationDestination{Server: "{{.url}}"},
				},
			},
		},
	}

	kubeclientset := kubefake.NewSimpleClientset()
	argoDBMock := dbmocks.ArgoDB{}
	argoObjs := []runtime.Object{&defaultProject}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet).Build()
	goodCluster := argov1alpha1.Cluster{Server: "https://good-cluster", Name: "good-cluster"}
	badCluster := argov1alpha1.Cluster{Server: "https://bad-cluster", Name: "bad-cluster"}
	argoDBMock.On("GetCluster", mock.Anything, "https://good-cluster").Return(&goodCluster, nil)
	argoDBMock.On("GetCluster", mock.Anything, "https://bad-cluster").Return(&badCluster, nil)
	argoDBMock.On("ListClusters", mock.Anything).Return(&argov1alpha1.ClusterList{Items: []argov1alpha1.Cluster{
		goodCluster,
	}}, nil)

	r := ApplicationSetReconciler{
		Log:      ctrl.Log.WithName("controllers").WithName("ApplicationSet"),
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
		Policy:           &utils.SyncPolicy{},
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
	assert.True(t, res.RequeueAfter == 0)

	var app argov1alpha1.Application

	// make sure good app got created
	err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "good-cluster"}, &app)
	assert.NoError(t, err)
	assert.Equal(t, app.Name, "good-cluster")

	// make sure bad app was not created
	err = r.Client.Get(context.TODO(), crtclient.ObjectKey{Namespace: "argocd", Name: "bad-cluster"}, &app)
	assert.Error(t, err)
}

func TestSetApplicationSetStatusCondition(t *testing.T) {
	scheme := runtime.NewScheme()
	err := argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)
	err = argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	appSet := argov1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "argocd",
		},
		Spec: argov1alpha1.ApplicationSetSpec{
			Generators: []argov1alpha1.ApplicationSetGenerator{
				{List: &argov1alpha1.ListGenerator{
					Elements: []apiextensionsv1.JSON{{
						Raw: []byte(`{"cluster": "my-cluster","url": "https://kubernetes.default.svc"}`),
					}},
				}},
			},
			Template: argov1alpha1.ApplicationSetTemplate{},
		},
	}

	appCondition := argov1alpha1.ApplicationSetCondition{
		Type:    argov1alpha1.ApplicationSetConditionResourcesUpToDate,
		Message: "All applications have been generated successfully",
		Reason:  argov1alpha1.ApplicationSetReasonApplicationSetUpToDate,
		Status:  argov1alpha1.ApplicationSetConditionStatusTrue,
	}

	kubeclientset := kubefake.NewSimpleClientset([]runtime.Object{}...)
	argoDBMock := dbmocks.ArgoDB{}
	argoObjs := []runtime.Object{}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet).Build()

	r := ApplicationSetReconciler{
		Log:      ctrl.Log.WithName("controllers").WithName("ApplicationSet"),
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
	}

	err = r.setApplicationSetStatusCondition(context.TODO(), &appSet, appCondition, true)
	assert.Nil(t, err)

	assert.Len(t, appSet.Status.Conditions, 3)
}
