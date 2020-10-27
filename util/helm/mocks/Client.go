// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import (
	helm "github.com/argoproj/argo-cd/util/helm"

	io "github.com/argoproj/argo-cd/util/io"

	mock "github.com/stretchr/testify/mock"

	semver "github.com/Masterminds/semver"
)

// Client is an autogenerated mock type for the Client type
type Client struct {
	mock.Mock
}

// CleanChartCache provides a mock function with given fields: chart, version
func (_m *Client) CleanChartCache(chart string, version *semver.Version) error {
	ret := _m.Called(chart, version)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, *semver.Version) error); ok {
		r0 = rf(chart, version)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ExtractChart provides a mock function with given fields: chart, version
func (_m *Client) ExtractChart(chart string, version *semver.Version) (string, io.Closer, error) {
	ret := _m.Called(chart, version)

	var r0 string
	if rf, ok := ret.Get(0).(func(string, *semver.Version) string); ok {
		r0 = rf(chart, version)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 io.Closer
	if rf, ok := ret.Get(1).(func(string, *semver.Version) io.Closer); ok {
		r1 = rf(chart, version)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(io.Closer)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string, *semver.Version) error); ok {
		r2 = rf(chart, version)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// GetIndex provides a mock function with given fields:
func (_m *Client) GetIndex() (*helm.Index, error) {
	ret := _m.Called()

	var r0 *helm.Index
	if rf, ok := ret.Get(0).(func() *helm.Index); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*helm.Index)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
