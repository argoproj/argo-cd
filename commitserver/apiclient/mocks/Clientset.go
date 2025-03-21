// Code generated by mockery v2.52.4. DO NOT EDIT.

package mocks

import (
	apiclient "github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	io "github.com/argoproj/argo-cd/v3/util/io"

	mock "github.com/stretchr/testify/mock"
)

// Clientset is an autogenerated mock type for the Clientset type
type Clientset struct {
	mock.Mock
}

// NewCommitServerClient provides a mock function with no fields
func (_m *Clientset) NewCommitServerClient() (io.Closer, apiclient.CommitServiceClient, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for NewCommitServerClient")
	}

	var r0 io.Closer
	var r1 apiclient.CommitServiceClient
	var r2 error
	if rf, ok := ret.Get(0).(func() (io.Closer, apiclient.CommitServiceClient, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() io.Closer); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(io.Closer)
		}
	}

	if rf, ok := ret.Get(1).(func() apiclient.CommitServiceClient); ok {
		r1 = rf()
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(apiclient.CommitServiceClient)
		}
	}

	if rf, ok := ret.Get(2).(func() error); ok {
		r2 = rf()
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// NewClientset creates a new instance of Clientset. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewClientset(t interface {
	mock.TestingT
	Cleanup(func())
}) *Clientset {
	mock := &Clientset{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
