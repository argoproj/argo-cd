// Code generated by mockery v2.52.4. DO NOT EDIT.

package mocks

import (
	context "context"

	codecommit "github.com/aws/aws-sdk-go/service/codecommit"

	mock "github.com/stretchr/testify/mock"

	request "github.com/aws/aws-sdk-go/aws/request"
)

// AWSCodeCommitClient is an autogenerated mock type for the AWSCodeCommitClient type
type AWSCodeCommitClient struct {
	mock.Mock
}

// GetFolderWithContext provides a mock function with given fields: _a0, _a1, _a2
func (_m *AWSCodeCommitClient) GetFolderWithContext(_a0 context.Context, _a1 *codecommit.GetFolderInput, _a2 ...request.Option) (*codecommit.GetFolderOutput, error) {
	_va := make([]interface{}, len(_a2))
	for _i := range _a2 {
		_va[_i] = _a2[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _a0, _a1)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for GetFolderWithContext")
	}

	var r0 *codecommit.GetFolderOutput
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *codecommit.GetFolderInput, ...request.Option) (*codecommit.GetFolderOutput, error)); ok {
		return rf(_a0, _a1, _a2...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *codecommit.GetFolderInput, ...request.Option) *codecommit.GetFolderOutput); ok {
		r0 = rf(_a0, _a1, _a2...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*codecommit.GetFolderOutput)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *codecommit.GetFolderInput, ...request.Option) error); ok {
		r1 = rf(_a0, _a1, _a2...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetRepositoryWithContext provides a mock function with given fields: _a0, _a1, _a2
func (_m *AWSCodeCommitClient) GetRepositoryWithContext(_a0 context.Context, _a1 *codecommit.GetRepositoryInput, _a2 ...request.Option) (*codecommit.GetRepositoryOutput, error) {
	_va := make([]interface{}, len(_a2))
	for _i := range _a2 {
		_va[_i] = _a2[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _a0, _a1)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for GetRepositoryWithContext")
	}

	var r0 *codecommit.GetRepositoryOutput
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *codecommit.GetRepositoryInput, ...request.Option) (*codecommit.GetRepositoryOutput, error)); ok {
		return rf(_a0, _a1, _a2...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *codecommit.GetRepositoryInput, ...request.Option) *codecommit.GetRepositoryOutput); ok {
		r0 = rf(_a0, _a1, _a2...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*codecommit.GetRepositoryOutput)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *codecommit.GetRepositoryInput, ...request.Option) error); ok {
		r1 = rf(_a0, _a1, _a2...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListBranchesWithContext provides a mock function with given fields: _a0, _a1, _a2
func (_m *AWSCodeCommitClient) ListBranchesWithContext(_a0 context.Context, _a1 *codecommit.ListBranchesInput, _a2 ...request.Option) (*codecommit.ListBranchesOutput, error) {
	_va := make([]interface{}, len(_a2))
	for _i := range _a2 {
		_va[_i] = _a2[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _a0, _a1)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for ListBranchesWithContext")
	}

	var r0 *codecommit.ListBranchesOutput
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *codecommit.ListBranchesInput, ...request.Option) (*codecommit.ListBranchesOutput, error)); ok {
		return rf(_a0, _a1, _a2...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *codecommit.ListBranchesInput, ...request.Option) *codecommit.ListBranchesOutput); ok {
		r0 = rf(_a0, _a1, _a2...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*codecommit.ListBranchesOutput)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *codecommit.ListBranchesInput, ...request.Option) error); ok {
		r1 = rf(_a0, _a1, _a2...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListRepositoriesWithContext provides a mock function with given fields: _a0, _a1, _a2
func (_m *AWSCodeCommitClient) ListRepositoriesWithContext(_a0 context.Context, _a1 *codecommit.ListRepositoriesInput, _a2 ...request.Option) (*codecommit.ListRepositoriesOutput, error) {
	_va := make([]interface{}, len(_a2))
	for _i := range _a2 {
		_va[_i] = _a2[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _a0, _a1)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for ListRepositoriesWithContext")
	}

	var r0 *codecommit.ListRepositoriesOutput
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *codecommit.ListRepositoriesInput, ...request.Option) (*codecommit.ListRepositoriesOutput, error)); ok {
		return rf(_a0, _a1, _a2...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *codecommit.ListRepositoriesInput, ...request.Option) *codecommit.ListRepositoriesOutput); ok {
		r0 = rf(_a0, _a1, _a2...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*codecommit.ListRepositoriesOutput)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *codecommit.ListRepositoriesInput, ...request.Option) error); ok {
		r1 = rf(_a0, _a1, _a2...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewAWSCodeCommitClient creates a new instance of AWSCodeCommitClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewAWSCodeCommitClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *AWSCodeCommitClient {
	mock := &AWSCodeCommitClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
