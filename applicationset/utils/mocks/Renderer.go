// Code generated by mockery v2.43.2. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"

	v1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// Renderer is an autogenerated mock type for the Renderer type
type Renderer struct {
	mock.Mock
}

// RenderTemplateParams provides a mock function with given fields: tmpl, syncPolicy, params, useGoTemplate, goTemplateOptions
func (_m *Renderer) RenderTemplateParams(tmpl *v1alpha1.Application, syncPolicy *v1alpha1.ApplicationSetSyncPolicy, params map[string]interface{}, useGoTemplate bool, goTemplateOptions []string) (*v1alpha1.Application, error) {
	ret := _m.Called(tmpl, syncPolicy, params, useGoTemplate, goTemplateOptions)

	if len(ret) == 0 {
		panic("no return value specified for RenderTemplateParams")
	}

	var r0 *v1alpha1.Application
	var r1 error
	if rf, ok := ret.Get(0).(func(*v1alpha1.Application, *v1alpha1.ApplicationSetSyncPolicy, map[string]interface{}, bool, []string) (*v1alpha1.Application, error)); ok {
		return rf(tmpl, syncPolicy, params, useGoTemplate, goTemplateOptions)
	}
	if rf, ok := ret.Get(0).(func(*v1alpha1.Application, *v1alpha1.ApplicationSetSyncPolicy, map[string]interface{}, bool, []string) *v1alpha1.Application); ok {
		r0 = rf(tmpl, syncPolicy, params, useGoTemplate, goTemplateOptions)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.Application)
		}
	}

	if rf, ok := ret.Get(1).(func(*v1alpha1.Application, *v1alpha1.ApplicationSetSyncPolicy, map[string]interface{}, bool, []string) error); ok {
		r1 = rf(tmpl, syncPolicy, params, useGoTemplate, goTemplateOptions)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Replace provides a mock function with given fields: tmpl, replaceMap, useGoTemplate, goTemplateOptions
func (_m *Renderer) Replace(tmpl string, replaceMap map[string]interface{}, useGoTemplate bool, goTemplateOptions []string) (string, error) {
	ret := _m.Called(tmpl, replaceMap, useGoTemplate, goTemplateOptions)

	if len(ret) == 0 {
		panic("no return value specified for Replace")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func(string, map[string]interface{}, bool, []string) (string, error)); ok {
		return rf(tmpl, replaceMap, useGoTemplate, goTemplateOptions)
	}
	if rf, ok := ret.Get(0).(func(string, map[string]interface{}, bool, []string) string); ok {
		r0 = rf(tmpl, replaceMap, useGoTemplate, goTemplateOptions)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(string, map[string]interface{}, bool, []string) error); ok {
		r1 = rf(tmpl, replaceMap, useGoTemplate, goTemplateOptions)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewRenderer creates a new instance of Renderer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewRenderer(t interface {
	mock.TestingT
	Cleanup(func())
}) *Renderer {
	mock := &Renderer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
