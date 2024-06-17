package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

type RendererMock struct {
	mock.Mock
}

func (r *RendererMock) RenderTemplateParams(tmpl *v1alpha1.Application, syncPolicy *v1alpha1.ApplicationSetSyncPolicy, params map[string]interface{}, useGoTemplate bool, goTemplateOptions []string) (*v1alpha1.Application, error) {
	args := r.Called(tmpl, params, useGoTemplate, goTemplateOptions)

	if args.Error(1) != nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*v1alpha1.Application), args.Error(1)
}

func (r *RendererMock) Replace(tmpl string, replaceMap map[string]interface{}, useGoTemplate bool, goTemplateOptions []string) (string, error) {
	args := r.Called(tmpl, replaceMap, useGoTemplate, goTemplateOptions)

	return args.Get(0).(string), args.Error(1)
}
