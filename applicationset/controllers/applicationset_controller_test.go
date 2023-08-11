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
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	crtclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/argoproj/argo-cd/v2/applicationset/generators"
	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync/common"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/v2/util/collections"
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
							Source:      &argov1alpha1.ApplicationSource{Path: "path", TargetRevision: "revision", RepoURL: "repoURL"},
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
						Source:      &argov1alpha1.ApplicationSource{Path: "path", TargetRevision: "revision", RepoURL: "repoURL"},
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
						Source:      &argov1alpha1.ApplicationSource{Path: "path", TargetRevision: "revision", RepoURL: "repoURL"},
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
					Source:  &argov1alpha1.ApplicationSource{Path: "path", TargetRevision: "revision", RepoURL: "repoURL"},
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
					Source:      &argov1alpha1.ApplicationSource{Path: "path", TargetRevision: "revision", RepoURL: "repoURL"},
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
						Source: &argov1alpha1.ApplicationSource{
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
						Source: &argov1alpha1.ApplicationSource{
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
						Source: &argov1alpha1.ApplicationSource{
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
						Source: &argov1alpha1.ApplicationSource{
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
						Source: &argov1alpha1.ApplicationSource{
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
					Source:      &argov1alpha1.ApplicationSource{RepoURL: "https://github.com/argoproj/argocd-example-apps", Path: "guestbook"},
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

// Test app generation from a go template application set using a pull request generator
func TestGenerateAppsUsingPullRequestGenerator(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	for _, cases := range []struct {
		name        string
		params      []map[string]interface{}
		template    argov1alpha1.ApplicationSetTemplate
		expectedApp []argov1alpha1.Application
	}{
		{
			name: "Generate an application from a go template application set manifest using a pull request generator",
			params: []map[string]interface{}{{
				"number":         "1",
				"branch":         "branch1",
				"branch_slug":    "branchSlug1",
				"head_sha":       "089d92cbf9ff857a39e6feccd32798ca700fb958",
				"head_short_sha": "089d92cb",
				"labels":         []string{"label1"}}},
			template: argov1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: argov1alpha1.ApplicationSetTemplateMeta{
					Name: "AppSet-{{.branch}}-{{.number}}",
					Labels: map[string]string{
						"app1": "{{index .labels 0}}",
					},
				},
				Spec: argov1alpha1.ApplicationSpec{
					Source: &argov1alpha1.ApplicationSource{
						RepoURL:        "https://testurl/testRepo",
						TargetRevision: "{{.head_short_sha}}",
					},
					Destination: argov1alpha1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "AppSet-{{.branch_slug}}-{{.head_sha}}",
					},
				},
			},
			expectedApp: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "AppSet-branch1-1",
						Labels: map[string]string{
							"app1": "label1",
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
			generator := argov1alpha1.ApplicationSetGenerator{
				PullRequest: &argov1alpha1.PullRequestGenerator{},
			}

			generatorMock.On("GenerateParams", &generator).
				Return(cases.params, nil)

			generatorMock.On("GetTemplate", &generator).
				Return(&cases.template, nil)

			appSetReconciler := ApplicationSetReconciler{
				Client:   client,
				Scheme:   scheme,
				Recorder: record.NewFakeRecorder(1),
				Generators: map[string]generators.Generator{
					"PullRequest": &generatorMock,
				},
				Renderer:      &utils.Render{},
				KubeClientset: kubefake.NewSimpleClientset(),
			}

			gotApp, _, _ := appSetReconciler.generateApplications(argov1alpha1.ApplicationSet{
				Spec: argov1alpha1.ApplicationSetSpec{
					GoTemplate: true,
					Generators: []argov1alpha1.ApplicationSetGenerator{{
						PullRequest: &argov1alpha1.PullRequestGenerator{},
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
	err := argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	defaultProject := argov1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "argocd"},
		Spec:       argov1alpha1.AppProjectSpec{SourceRepos: []string{"*"}, Destinations: []argov1alpha1.ApplicationDestination{{Namespace: "*", Server: "https://kubernetes.default.svc"}}},
	}
	myCluster := argov1alpha1.Cluster{
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
								Elements: []apiextensionsv1.JSON{
									{
										Raw: []byte(`{"name": "my-app"}`),
									},
								},
							},
						},
					},
					Template: argov1alpha1.ApplicationSetTemplate{
						ApplicationSetTemplateMeta: argov1alpha1.ApplicationSetTemplateMeta{
							Name:      "{{.name}}",
							Namespace: "argocd",
							Annotations: map[string]string{
								"key": "value",
							},
						},
						Spec: argov1alpha1.ApplicationSpec{
							Source:      &argov1alpha1.ApplicationSource{RepoURL: "https://github.com/argoproj/argocd-example-apps", Path: "guestbook"},
							Project:     "default",
							Destination: argov1alpha1.ApplicationDestination{Server: "https://kubernetes.default.svc"},
						},
					},
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet).Build()

			r := ApplicationSetReconciler{
				Client:   client,
				Scheme:   scheme,
				Renderer: &utils.Render{},
				Recorder: record.NewFakeRecorder(10),
				Generators: map[string]generators.Generator{
					"List": generators.NewListGenerator(),
				},
				ArgoDB:           &argoDBMock,
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

			var app argov1alpha1.Application
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
			appSet.Spec.Generators[0] = argov1alpha1.ApplicationSetGenerator{
				List: &argov1alpha1.ListGenerator{
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

	appStatuses := []argov1alpha1.ApplicationSetApplicationStatus{
		{
			Application:        "my-application",
			LastTransitionTime: &metav1.Time{},
			Message:            "testing SetApplicationSetApplicationStatus to Healthy",
			Status:             "Healthy",
		},
	}

	kubeclientset := kubefake.NewSimpleClientset([]runtime.Object{}...)
	argoDBMock := dbmocks.ArgoDB{}
	argoObjs := []runtime.Object{}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet).Build()

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
	}

	err = r.setAppSetApplicationStatus(context.TODO(), &appSet, appStatuses)
	assert.Nil(t, err)

	assert.Len(t, appSet.Status.ApplicationStatus, 1)
}

func TestBuildAppDependencyList(t *testing.T) {

	scheme := runtime.NewScheme()
	err := argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	for _, cc := range []struct {
		name            string
		appSet          argov1alpha1.ApplicationSet
		apps            []argov1alpha1.Application
		expectedList    [][]string
		expectedStepMap map[string]int
	}{
		{
			name: "handles an empty set of applications and no strategy",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{},
			},
			apps:            []argov1alpha1.Application{},
			expectedList:    [][]string{},
			expectedStepMap: map[string]int{},
		},
		{
			name: "handles an empty set of applications and ignores AllAtOnce strategy",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type: "AllAtOnce",
					},
				},
			},
			apps:            []argov1alpha1.Application{},
			expectedList:    [][]string{},
			expectedStepMap: map[string]int{},
		},
		{
			name: "handles an empty set of applications with good 'In' selectors",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
							Steps: []argov1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{
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
			apps: []argov1alpha1.Application{},
			expectedList: [][]string{
				{},
			},
			expectedStepMap: map[string]int{},
		},
		{
			name: "handles selecting 1 application with 1 'In' selector",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
							Steps: []argov1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{
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
			apps: []argov1alpha1.Application{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
							Steps: []argov1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{
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
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{
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
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{
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
			apps: []argov1alpha1.Application{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
							Steps: []argov1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{
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
			apps: []argov1alpha1.Application{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
							Steps: []argov1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{
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
			apps: []argov1alpha1.Application{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
							Steps: []argov1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{
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
			apps: []argov1alpha1.Application{},
			expectedList: [][]string{
				{},
			},
			expectedStepMap: map[string]int{},
		},
		{
			name: "selects 1 application with 1 'NotIn' selector",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
							Steps: []argov1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{
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
			apps: []argov1alpha1.Application{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
							Steps: []argov1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{
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
			apps: []argov1alpha1.Application{
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
			name: "multiple 'NotIn' selectors only match Applications with all labels",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
							Steps: []argov1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{
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
			apps: []argov1alpha1.Application{
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
				{"app-qa1"},
			},
			expectedStepMap: map[string]int{
				"app-qa1": 0,
			},
		},
		{
			name: "multiple values in the same 'NotIn' matchExpression exclude a match from any value",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
							Steps: []argov1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{
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
			apps: []argov1alpha1.Application{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
							Steps: []argov1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{
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
			apps: []argov1alpha1.Application{
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
			}

			appDependencyList, appStepMap, err := r.buildAppDependencyList(context.TODO(), cc.appSet, cc.apps)
			assert.Equal(t, err, nil, "expected no errors, but errors occured")
			assert.Equal(t, cc.expectedList, appDependencyList, "expected appDependencyList did not match actual")
			assert.Equal(t, cc.expectedStepMap, appStepMap, "expected appStepMap did not match actual")
		})
	}
}

func TestBuildAppSyncMap(t *testing.T) {

	scheme := runtime.NewScheme()
	err := argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	for _, cc := range []struct {
		name              string
		appSet            argov1alpha1.ApplicationSet
		appMap            map[string]argov1alpha1.Application
		appDependencyList [][]string
		expectedMap       map[string]bool
	}{
		{
			name: "handles an empty app dependency list",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{},
					},
				},
			},
			appDependencyList: [][]string{},
			expectedMap:       map[string]bool{},
		},
		{
			name: "handles two applications with no statuses",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{},
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{},
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appMap: map[string]argov1alpha1.Application{
				"app1": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				"app2": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app2",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appMap: map[string]argov1alpha1.Application{
				"app1": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				"app2": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app2",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appMap: map[string]argov1alpha1.Application{
				"app1": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationRunning,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				"app2": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app2",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationRunning,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appMap: map[string]argov1alpha1.Application{
				"app1": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusDegraded,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationRunning,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				"app2": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app2",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusDegraded,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationRunning,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appMap: map[string]argov1alpha1.Application{
				"app1": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				"app2": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app2",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appMap: map[string]argov1alpha1.Application{
				"app1": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				"app2": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app2",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				"app3": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app3",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				"app5": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app5",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				"app6": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app6",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusDegraded,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
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
			}

			appSyncMap, err := r.buildAppSyncMap(context.TODO(), cc.appSet, cc.appDependencyList, cc.appMap)
			assert.Equal(t, err, nil, "expected no errors, but errors occured")
			assert.Equal(t, cc.expectedMap, appSyncMap, "expected appSyncMap did not match actual")
		})
	}
}

func TestUpdateApplicationSetApplicationStatus(t *testing.T) {

	scheme := runtime.NewScheme()
	err := argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	for _, cc := range []struct {
		name              string
		appSet            argov1alpha1.ApplicationSet
		apps              []argov1alpha1.Application
		appStepMap        map[string]int
		expectedAppStatus []argov1alpha1.ApplicationSetApplicationStatus
	}{
		{
			name: "handles a nil list of statuses and no applications",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{},
					},
				},
			},
			apps:              []argov1alpha1.Application{},
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{},
		},
		{
			name: "handles a nil list of statuses with a healthy application",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{},
					},
				},
			},
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{},
			},
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Message:     "",
							Status:      "Healthy",
							Step:        "1",
						},
					},
				},
			},
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Message:     "",
							Status:      "Pending",
							Step:        "1",
						},
					},
				},
			},
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusProgressing,
						},
					},
				},
			},
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Message:     "",
							Status:      "Pending",
							Step:        "1",
						},
					},
				},
			},
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationRunning,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Message:     "",
							Status:      "Progressing",
							Step:        "1",
						},
					},
				},
			},
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Message:     "",
							Status:      "Waiting",
							Step:        "1",
						},
					},
				},
			},
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{},
					},
				},
			},
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationSucceeded,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
			appStepMap: map[string]int{
				"app1": 1,
				"app2": 0,
			},
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusDegraded,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationSucceeded,
							StartedAt: metav1.Time{
								Time: time.Now(),
							},
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type:        "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusDegraded,
						},
						OperationState: &argov1alpha1.OperationState{
							Phase: common.OperationSucceeded,
							StartedAt: metav1.Time{
								Time: time.Now().Add(time.Duration(-11) * time.Second),
							},
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{
				{
					Application: "app1",
					Message:     "Application moved to Pending status, watching for the Application resource to start Progressing.",
					Status:      "Pending",
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
				Generators:       map[string]generators.Generator{},
				ArgoDB:           &argoDBMock,
				ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
				KubeClientset:    kubeclientset,
			}

			appStatuses, err := r.updateApplicationSetApplicationStatus(context.TODO(), &cc.appSet, cc.apps, cc.appStepMap)

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
	err := argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	for _, cc := range []struct {
		name              string
		appSet            argov1alpha1.ApplicationSet
		appSyncMap        map[string]bool
		appStepMap        map[string]int
		appMap            map[string]argov1alpha1.Application
		expectedAppStatus []argov1alpha1.ApplicationSetApplicationStatus
	}{
		{
			name: "handles an empty appSync and appStepMap",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
							Steps: []argov1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{},
				},
			},
			appSyncMap:        map[string]bool{},
			appStepMap:        map[string]int{},
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{},
		},
		{
			name: "handles an empty strategy",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{},
				},
			},
			appSyncMap:        map[string]bool{},
			appStepMap:        map[string]int{},
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{},
		},
		{
			name: "handles an empty applicationset strategy",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{},
				},
			},
			appSyncMap:        map[string]bool{},
			appStepMap:        map[string]int{},
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{},
		},
		{
			name: "handles an appSyncMap with no existing statuses",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{},
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
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{},
		},
		{
			name: "handles updating a RollingSync status from Waiting to Pending",
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
							Steps: []argov1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
							Steps: []argov1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
							Steps: []argov1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
							Steps: []argov1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{},
									MaxUpdate: &intstr.IntOrString{
										Type:   intstr.Int,
										IntVal: 3,
									},
								},
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appMap: map[string]argov1alpha1.Application{
				"app1": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app1",
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				"app2": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app2",
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				"app3": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app3",
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				"app4": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "app4",
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
							Steps: []argov1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{},
									MaxUpdate: &intstr.IntOrString{
										Type:   intstr.String,
										StrVal: "50%",
									},
								},
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
							Steps: []argov1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{},
									MaxUpdate: &intstr.IntOrString{
										Type:   intstr.Int,
										IntVal: 0,
									},
								},
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
							Steps: []argov1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{},
									MaxUpdate: &intstr.IntOrString{
										Type:   intstr.String,
										StrVal: "100%",
									},
								},
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			appSet: argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					Strategy: &argov1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
							Steps: []argov1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{},
									MaxUpdate: &intstr.IntOrString{
										Type:   intstr.String,
										StrVal: "1%",
									},
								},
								{
									MatchExpressions: []argov1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: argov1alpha1.ApplicationSetStatus{
					ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
			expectedAppStatus: []argov1alpha1.ApplicationSetApplicationStatus{
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
				Generators:       map[string]generators.Generator{},
				ArgoDB:           &argoDBMock,
				ArgoAppClientset: appclientset.NewSimpleClientset(argoObjs...),
				KubeClientset:    kubeclientset,
			}

			appStatuses, err := r.updateApplicationSetApplicationStatusProgress(context.TODO(), &cc.appSet, cc.appSyncMap, cc.appStepMap, cc.appMap)

			// opt out of testing the LastTransitionTime is accurate
			for i := range appStatuses {
				appStatuses[i].LastTransitionTime = nil
			}

			assert.Equal(t, err, nil, "expected no errors, but errors occured")
			assert.Equal(t, cc.expectedAppStatus, appStatuses, "expected appStatuses did not match actual")
		})
	}
}
