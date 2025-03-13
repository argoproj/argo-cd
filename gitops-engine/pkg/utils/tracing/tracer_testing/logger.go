// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/go-logr/logr (interfaces: LogSink)
//
// Generated by this command:
//
//	mockgen -destination logger.go -package tracer_testing github.com/go-logr/logr LogSink
//

// Package tracer_testing is a generated GoMock package.
package tracer_testing

import (
	reflect "reflect"

	logr "github.com/go-logr/logr"
	gomock "go.uber.org/mock/gomock"
)

// MockLogSink is a mock of LogSink interface.
type MockLogSink struct {
	ctrl     *gomock.Controller
	recorder *MockLogSinkMockRecorder
	isgomock struct{}
}

// MockLogSinkMockRecorder is the mock recorder for MockLogSink.
type MockLogSinkMockRecorder struct {
	mock *MockLogSink
}

// NewMockLogSink creates a new mock instance.
func NewMockLogSink(ctrl *gomock.Controller) *MockLogSink {
	mock := &MockLogSink{ctrl: ctrl}
	mock.recorder = &MockLogSinkMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockLogSink) EXPECT() *MockLogSinkMockRecorder {
	return m.recorder
}

// Enabled mocks base method.
func (m *MockLogSink) Enabled(level int) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Enabled", level)
	ret0, _ := ret[0].(bool)
	return ret0
}

// Enabled indicates an expected call of Enabled.
func (mr *MockLogSinkMockRecorder) Enabled(level any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Enabled", reflect.TypeOf((*MockLogSink)(nil).Enabled), level)
}

// Error mocks base method.
func (m *MockLogSink) Error(err error, msg string, keysAndValues ...any) {
	m.ctrl.T.Helper()
	varargs := []any{err, msg}
	for _, a := range keysAndValues {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Error", varargs...)
}

// Error indicates an expected call of Error.
func (mr *MockLogSinkMockRecorder) Error(err, msg any, keysAndValues ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{err, msg}, keysAndValues...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Error", reflect.TypeOf((*MockLogSink)(nil).Error), varargs...)
}

// Info mocks base method.
func (m *MockLogSink) Info(level int, msg string, keysAndValues ...any) {
	m.ctrl.T.Helper()
	varargs := []any{level, msg}
	for _, a := range keysAndValues {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Info", varargs...)
}

// Info indicates an expected call of Info.
func (mr *MockLogSinkMockRecorder) Info(level, msg any, keysAndValues ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{level, msg}, keysAndValues...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Info", reflect.TypeOf((*MockLogSink)(nil).Info), varargs...)
}

// Init mocks base method.
func (m *MockLogSink) Init(info logr.RuntimeInfo) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Init", info)
}

// Init indicates an expected call of Init.
func (mr *MockLogSinkMockRecorder) Init(info any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockLogSink)(nil).Init), info)
}

// WithName mocks base method.
func (m *MockLogSink) WithName(name string) logr.LogSink {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WithName", name)
	ret0, _ := ret[0].(logr.LogSink)
	return ret0
}

// WithName indicates an expected call of WithName.
func (mr *MockLogSinkMockRecorder) WithName(name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WithName", reflect.TypeOf((*MockLogSink)(nil).WithName), name)
}

// WithValues mocks base method.
func (m *MockLogSink) WithValues(keysAndValues ...any) logr.LogSink {
	m.ctrl.T.Helper()
	varargs := []any{}
	for _, a := range keysAndValues {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "WithValues", varargs...)
	ret0, _ := ret[0].(logr.LogSink)
	return ret0
}

// WithValues indicates an expected call of WithValues.
func (mr *MockLogSinkMockRecorder) WithValues(keysAndValues ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WithValues", reflect.TypeOf((*MockLogSink)(nil).WithValues), keysAndValues...)
}
