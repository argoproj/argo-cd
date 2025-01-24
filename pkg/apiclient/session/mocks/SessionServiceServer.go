// Code generated by mockery v2.43.2. DO NOT EDIT.

package mocks

import (
	context "context"

	session "github.com/argoproj/argo-cd/v3/pkg/apiclient/session"
	mock "github.com/stretchr/testify/mock"
)

// SessionServiceServer is an autogenerated mock type for the SessionServiceServer type
type SessionServiceServer struct {
	mock.Mock
}

// Create provides a mock function with given fields: _a0, _a1
func (_m *SessionServiceServer) Create(_a0 context.Context, _a1 *session.SessionCreateRequest) (*session.SessionResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for Create")
	}

	var r0 *session.SessionResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *session.SessionCreateRequest) (*session.SessionResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *session.SessionCreateRequest) *session.SessionResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*session.SessionResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *session.SessionCreateRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Delete provides a mock function with given fields: _a0, _a1
func (_m *SessionServiceServer) Delete(_a0 context.Context, _a1 *session.SessionDeleteRequest) (*session.SessionResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for Delete")
	}

	var r0 *session.SessionResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *session.SessionDeleteRequest) (*session.SessionResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *session.SessionDeleteRequest) *session.SessionResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*session.SessionResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *session.SessionDeleteRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetUserInfo provides a mock function with given fields: _a0, _a1
func (_m *SessionServiceServer) GetUserInfo(_a0 context.Context, _a1 *session.GetUserInfoRequest) (*session.GetUserInfoResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for GetUserInfo")
	}

	var r0 *session.GetUserInfoResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *session.GetUserInfoRequest) (*session.GetUserInfoResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *session.GetUserInfoRequest) *session.GetUserInfoResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*session.GetUserInfoResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *session.GetUserInfoRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewSessionServiceServer creates a new instance of SessionServiceServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewSessionServiceServer(t interface {
	mock.TestingT
	Cleanup(func())
}) *SessionServiceServer {
	mock := &SessionServiceServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
