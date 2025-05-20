package template

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/v2/applicationset/generators"
	genmock "github.com/argoproj/argo-cd/v2/applicationset/generators/mocks"
	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	rendmock "github.com/argoproj/argo-cd/v2/applicationset/utils/mocks"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/collections"
)

func TestGenerateApplications(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	err = v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

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
				Name:      "test",
				Namespace: "namespace",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       application.ApplicationKind,
				APIVersion: "argoproj.io/v1alpha1",
			},
		}

		t.Run(cc.name, func(t *testing.T) {
			generatorMock := genmock.Generator{}
			generator := v1alpha1.ApplicationSetGenerator{
				List: &v1alpha1.ListGenerator{},
			}

			generatorMock.On("GenerateParams", &generator, mock.AnythingOfType("*v1alpha1.ApplicationSet"), mock.Anything).
				Return(cc.params, cc.generateParamsError)

			generatorMock.On("GetTemplate", &generator).
				Return(&v1alpha1.ApplicationSetTemplate{})

			rendererMock := rendmock.Renderer{}

			var expectedApps []v1alpha1.Application

			if cc.generateParamsError == nil {
				for _, p := range cc.params {
					if cc.rendererError != nil {
						rendererMock.On("RenderTemplateParams", GetTempApplication(cc.template), mock.AnythingOfType("*v1alpha1.ApplicationSetSyncPolicy"), p, false, []string(nil)).
							Return(nil, cc.rendererError)
					} else {
						rendererMock.On("RenderTemplateParams", GetTempApplication(cc.template), mock.AnythingOfType("*v1alpha1.ApplicationSetSyncPolicy"), p, false, []string(nil)).
							Return(&app, nil)
						expectedApps = append(expectedApps, app)
					}
				}
			}

			generators := map[string]generators.Generator{
				"List": &generatorMock,
			}
			renderer := &rendererMock

			got, reason, err := GenerateApplications(log.NewEntry(log.StandardLogger()), v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Generators: []v1alpha1.ApplicationSetGenerator{generator},
					Template:   cc.template,
				},
			},
				generators,
				renderer,
				nil,
			)

			if cc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
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
			generatorMock := genmock.Generator{}
			generator := v1alpha1.ApplicationSetGenerator{
				List: &v1alpha1.ListGenerator{},
			}

			generatorMock.On("GenerateParams", &generator, mock.AnythingOfType("*v1alpha1.ApplicationSet"), mock.Anything).
				Return(cc.params, nil)

			generatorMock.On("GetTemplate", &generator).
				Return(&cc.overrideTemplate)

			rendererMock := rendmock.Renderer{}

			rendererMock.On("RenderTemplateParams", GetTempApplication(cc.expectedMerged), mock.AnythingOfType("*v1alpha1.ApplicationSetSyncPolicy"), cc.params[0], false, []string(nil)).
				Return(&cc.expectedApps[0], nil)

			generators := map[string]generators.Generator{
				"List": &generatorMock,
			}
			renderer := &rendererMock

			got, _, _ := GenerateApplications(log.NewEntry(log.StandardLogger()), v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Generators: []v1alpha1.ApplicationSetGenerator{generator},
					Template:   cc.template,
				},
			},
				generators,
				renderer,
				nil,
			)

			assert.Equal(t, cc.expectedApps, got)
		})
	}
}

// Test app generation from a go template application set using a pull request generator
func TestGenerateAppsUsingPullRequestGenerator(t *testing.T) {
	for _, cases := range []struct {
		name        string
		params      []map[string]interface{}
		template    v1alpha1.ApplicationSetTemplate
		expectedApp []v1alpha1.Application
	}{
		{
			name: "Generate an application from a go template application set manifest using a pull request generator",
			params: []map[string]interface{}{
				{
					"number":                                "1",
					"title":                                 "title1",
					"branch":                                "branch1",
					"branch_slug":                           "branchSlug1",
					"head_sha":                              "089d92cbf9ff857a39e6feccd32798ca700fb958",
					"head_short_sha":                        "089d92cb",
					"branch_slugify_default":                "feat/a_really+long_pull_request_name_to_test_argo_slugification_and_branch_name_shortening_feature",
					"branch_slugify_smarttruncate_disabled": "feat/areallylongpullrequestnametotestargoslugificationandbranchnameshorteningfeature",
					"branch_slugify_smarttruncate_enabled":  "feat/testwithsmarttruncateenabledramdomlonglistofcharacters",
					"labels":                                []string{"label1"},
				},
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
			generatorMock := genmock.Generator{}
			generator := v1alpha1.ApplicationSetGenerator{
				PullRequest: &v1alpha1.PullRequestGenerator{},
			}

			generatorMock.On("GenerateParams", &generator, mock.AnythingOfType("*v1alpha1.ApplicationSet"), mock.Anything).
				Return(cases.params, nil)

			generatorMock.On("GetTemplate", &generator).
				Return(&cases.template, nil)

			generators := map[string]generators.Generator{
				"PullRequest": &generatorMock,
			}
			renderer := &utils.Render{}

			gotApp, _, _ := GenerateApplications(log.NewEntry(log.StandardLogger()), v1alpha1.ApplicationSet{
				Spec: v1alpha1.ApplicationSetSpec{
					GoTemplate: true,
					Generators: []v1alpha1.ApplicationSetGenerator{{
						PullRequest: &v1alpha1.PullRequestGenerator{},
					}},
					Template: cases.template,
				},
			},
				generators,
				renderer,
				nil,
			)
			assert.EqualValues(t, cases.expectedApp[0].ObjectMeta.Name, gotApp[0].ObjectMeta.Name)
			assert.EqualValues(t, cases.expectedApp[0].Spec.Source.TargetRevision, gotApp[0].Spec.Source.TargetRevision)
			assert.EqualValues(t, cases.expectedApp[0].Spec.Destination.Namespace, gotApp[0].Spec.Destination.Namespace)
			assert.True(t, collections.StringMapsEqual(cases.expectedApp[0].ObjectMeta.Labels, gotApp[0].ObjectMeta.Labels))
		})
	}
}
