package ksonnet

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var (
	testDataDir string
)

const (
	testAppName = "test-app"
	testEnvName = "test-env"
)

func init() {
	_, filename, _, _ := runtime.Caller(0)
	testDataDir = filepath.Join(filepath.Dir(filename), "testdata")
}

func TestKsonnet(t *testing.T) {
	ksApp, err := NewKsonnetApp(filepath.Join(testDataDir, testAppName))
	assert.Nil(t, err)
	defaultDest, err := ksApp.Destination(testEnvName)
	assert.True(t, err == nil)
	assert.Equal(t, "https://1.2.3.4", defaultDest.Server)
}

func TestShow(t *testing.T) {
	ksApp, err := NewKsonnetApp(filepath.Join(testDataDir, testAppName))
	assert.Nil(t, err)
	objs, err := ksApp.Show(testEnvName)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(objs))
	for _, obj := range objs {
		jsonBytes, err := json.Marshal(obj)
		assert.Nil(t, err)
		log.Infof("%v", string(jsonBytes))
	}
}
