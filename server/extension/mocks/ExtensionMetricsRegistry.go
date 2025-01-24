// Code generated by mockery v2.43.2. DO NOT EDIT.

package mocks

import (
	time "time"

	mock "github.com/stretchr/testify/mock"
)

// ExtensionMetricsRegistry is an autogenerated mock type for the ExtensionMetricsRegistry type
type ExtensionMetricsRegistry struct {
	mock.Mock
}

// IncExtensionRequestCounter provides a mock function with given fields: _a0, status
func (_m *ExtensionMetricsRegistry) IncExtensionRequestCounter(_a0 string, status int) {
	_m.Called(_a0, status)
}

// ObserveExtensionRequestDuration provides a mock function with given fields: _a0, duration
func (_m *ExtensionMetricsRegistry) ObserveExtensionRequestDuration(_a0 string, duration time.Duration) {
	_m.Called(_a0, duration)
}

// NewExtensionMetricsRegistry creates a new instance of ExtensionMetricsRegistry. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewExtensionMetricsRegistry(t interface {
	mock.TestingT
	Cleanup(func())
}) *ExtensionMetricsRegistry {
	mock := &ExtensionMetricsRegistry{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
