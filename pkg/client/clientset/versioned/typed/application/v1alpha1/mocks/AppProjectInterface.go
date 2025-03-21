// Code generated by mockery v2.52.4. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	types "k8s.io/apimachinery/pkg/types"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"

	watch "k8s.io/apimachinery/pkg/watch"
)

// AppProjectInterface is an autogenerated mock type for the AppProjectInterface type
type AppProjectInterface struct {
	mock.Mock
}

// Create provides a mock function with given fields: ctx, appProject, opts
func (_m *AppProjectInterface) Create(ctx context.Context, appProject *v1alpha1.AppProject, opts v1.CreateOptions) (*v1alpha1.AppProject, error) {
	ret := _m.Called(ctx, appProject, opts)

	if len(ret) == 0 {
		panic("no return value specified for Create")
	}

	var r0 *v1alpha1.AppProject
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.AppProject, v1.CreateOptions) (*v1alpha1.AppProject, error)); ok {
		return rf(ctx, appProject, opts)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.AppProject, v1.CreateOptions) *v1alpha1.AppProject); ok {
		r0 = rf(ctx, appProject, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.AppProject)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1alpha1.AppProject, v1.CreateOptions) error); ok {
		r1 = rf(ctx, appProject, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Delete provides a mock function with given fields: ctx, name, opts
func (_m *AppProjectInterface) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	ret := _m.Called(ctx, name, opts)

	if len(ret) == 0 {
		panic("no return value specified for Delete")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, v1.DeleteOptions) error); ok {
		r0 = rf(ctx, name, opts)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteCollection provides a mock function with given fields: ctx, opts, listOpts
func (_m *AppProjectInterface) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	ret := _m.Called(ctx, opts, listOpts)

	if len(ret) == 0 {
		panic("no return value specified for DeleteCollection")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, v1.DeleteOptions, v1.ListOptions) error); ok {
		r0 = rf(ctx, opts, listOpts)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Get provides a mock function with given fields: ctx, name, opts
func (_m *AppProjectInterface) Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.AppProject, error) {
	ret := _m.Called(ctx, name, opts)

	if len(ret) == 0 {
		panic("no return value specified for Get")
	}

	var r0 *v1alpha1.AppProject
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, v1.GetOptions) (*v1alpha1.AppProject, error)); ok {
		return rf(ctx, name, opts)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, v1.GetOptions) *v1alpha1.AppProject); ok {
		r0 = rf(ctx, name, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.AppProject)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, v1.GetOptions) error); ok {
		r1 = rf(ctx, name, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// List provides a mock function with given fields: ctx, opts
func (_m *AppProjectInterface) List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.AppProjectList, error) {
	ret := _m.Called(ctx, opts)

	if len(ret) == 0 {
		panic("no return value specified for List")
	}

	var r0 *v1alpha1.AppProjectList
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, v1.ListOptions) (*v1alpha1.AppProjectList, error)); ok {
		return rf(ctx, opts)
	}
	if rf, ok := ret.Get(0).(func(context.Context, v1.ListOptions) *v1alpha1.AppProjectList); ok {
		r0 = rf(ctx, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.AppProjectList)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, v1.ListOptions) error); ok {
		r1 = rf(ctx, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Patch provides a mock function with given fields: ctx, name, pt, data, opts, subresources
func (_m *AppProjectInterface) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (*v1alpha1.AppProject, error) {
	_va := make([]interface{}, len(subresources))
	for _i := range subresources {
		_va[_i] = subresources[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, name, pt, data, opts)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for Patch")
	}

	var r0 *v1alpha1.AppProject
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, types.PatchType, []byte, v1.PatchOptions, ...string) (*v1alpha1.AppProject, error)); ok {
		return rf(ctx, name, pt, data, opts, subresources...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, types.PatchType, []byte, v1.PatchOptions, ...string) *v1alpha1.AppProject); ok {
		r0 = rf(ctx, name, pt, data, opts, subresources...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.AppProject)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, types.PatchType, []byte, v1.PatchOptions, ...string) error); ok {
		r1 = rf(ctx, name, pt, data, opts, subresources...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Update provides a mock function with given fields: ctx, appProject, opts
func (_m *AppProjectInterface) Update(ctx context.Context, appProject *v1alpha1.AppProject, opts v1.UpdateOptions) (*v1alpha1.AppProject, error) {
	ret := _m.Called(ctx, appProject, opts)

	if len(ret) == 0 {
		panic("no return value specified for Update")
	}

	var r0 *v1alpha1.AppProject
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.AppProject, v1.UpdateOptions) (*v1alpha1.AppProject, error)); ok {
		return rf(ctx, appProject, opts)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.AppProject, v1.UpdateOptions) *v1alpha1.AppProject); ok {
		r0 = rf(ctx, appProject, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.AppProject)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1alpha1.AppProject, v1.UpdateOptions) error); ok {
		r1 = rf(ctx, appProject, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Watch provides a mock function with given fields: ctx, opts
func (_m *AppProjectInterface) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	ret := _m.Called(ctx, opts)

	if len(ret) == 0 {
		panic("no return value specified for Watch")
	}

	var r0 watch.Interface
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, v1.ListOptions) (watch.Interface, error)); ok {
		return rf(ctx, opts)
	}
	if rf, ok := ret.Get(0).(func(context.Context, v1.ListOptions) watch.Interface); ok {
		r0 = rf(ctx, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(watch.Interface)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, v1.ListOptions) error); ok {
		r1 = rf(ctx, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewAppProjectInterface creates a new instance of AppProjectInterface. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewAppProjectInterface(t interface {
	mock.TestingT
	Cleanup(func())
}) *AppProjectInterface {
	mock := &AppProjectInterface{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
