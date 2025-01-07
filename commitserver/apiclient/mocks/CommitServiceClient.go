// Code generated by mockery v2.43.2. DO NOT EDIT.

package mocks

import (
	context "context"

	apiclient "github.com/argoproj/argo-cd/v2/commitserver/apiclient"

	grpc "google.golang.org/grpc"

	mock "github.com/stretchr/testify/mock"
)

// CommitServiceClient is an autogenerated mock type for the CommitServiceClient type
type CommitServiceClient struct {
	mock.Mock
}

// CommitHydratedManifests provides a mock function with given fields: ctx, in, opts
func (_m *CommitServiceClient) CommitHydratedManifests(ctx context.Context, in *apiclient.CommitHydratedManifestsRequest, opts ...grpc.CallOption) (*apiclient.CommitHydratedManifestsResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for CommitHydratedManifests")
	}

	var r0 *apiclient.CommitHydratedManifestsResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *apiclient.CommitHydratedManifestsRequest, ...grpc.CallOption) (*apiclient.CommitHydratedManifestsResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *apiclient.CommitHydratedManifestsRequest, ...grpc.CallOption) *apiclient.CommitHydratedManifestsResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*apiclient.CommitHydratedManifestsResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *apiclient.CommitHydratedManifestsRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewCommitServiceClient creates a new instance of CommitServiceClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewCommitServiceClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *CommitServiceClient {
	mock := &CommitServiceClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
