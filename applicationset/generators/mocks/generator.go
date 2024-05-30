package mocks

import (
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/mock"
)

type GeneratorMock struct {
	mock.Mock
}

func (g *GeneratorMock) GetTemplate(appSetGenerator *v1alpha1.ApplicationSetGenerator) *v1alpha1.ApplicationSetTemplate {
	args := g.Called(appSetGenerator)

	return args.Get(0).(*v1alpha1.ApplicationSetTemplate)
}

func (g *GeneratorMock) GenerateParams(appSetGenerator *v1alpha1.ApplicationSetGenerator, _ *v1alpha1.ApplicationSet) ([]map[string]interface{}, error) {
	args := g.Called(appSetGenerator)

	return args.Get(0).([]map[string]interface{}), args.Error(1)
}

func (g *GeneratorMock) Replace(tmpl string, replaceMap map[string]interface{}, useGoTemplate bool, goTemplateOptions []string) (string, error) {
	args := g.Called(tmpl, replaceMap, useGoTemplate, goTemplateOptions)

	return args.Get(0).(string), args.Error(1)
}

func (g *GeneratorMock) GetRequeueAfter(appSetGenerator *v1alpha1.ApplicationSetGenerator) time.Duration {
	args := g.Called(appSetGenerator)

	return args.Get(0).(time.Duration)
}
