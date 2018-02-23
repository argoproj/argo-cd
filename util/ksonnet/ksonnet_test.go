package ksonnet

import (
	"path"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	testDataDir string
)

const (
	testAppName = "test-app"
)

func init() {
	_, filename, _, _ := runtime.Caller(0)
	testDataDir = path.Join(path.Dir(filename), "testdata")
}

func TestKsonnet(t *testing.T) {
	ksApp, err := NewKsonnetApp(path.Join(testDataDir, testAppName))
	assert.Nil(t, err)
	defaultEnv, ok := ksApp.Spec.Environments["default"]
	assert.True(t, ok)
	assert.Equal(t, 1, len(defaultEnv.Destinations))
}
