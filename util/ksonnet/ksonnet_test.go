package ksonnet

import (
	"encoding/json"
	"path"
	"reflect"
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
	testDataDir = path.Join(path.Dir(filename), "testdata")
}

func TestKsonnet(t *testing.T) {
	ksApp, err := NewKsonnetApp(path.Join(testDataDir, testAppName))
	assert.Nil(t, err)
	app := ksApp.App()
	defaultEnv, err := app.Environment(testEnvName)
	assert.True(t, err == nil)
	assert.Equal(t, "https://1.2.3.4", defaultEnv.Destination.Server)
}

func TestShow(t *testing.T) {
	ksApp, err := NewKsonnetApp(path.Join(testDataDir, testAppName))
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

func TestListEnvParams(t *testing.T) {
	ksApp, err := NewKsonnetApp(path.Join(testDataDir, testAppName))
	assert.Nil(t, err)
	params, err := ksApp.ListEnvParams(testEnvName)
	assert.Nil(t, err)

	expected := map[string]string{
		"containerPort": "80",
		"image":         "gcr.io/kuar-demo/kuard-amd64:1",
		"name":          "demo",
		"replicas":      "2",
		"servicePort":   "80",
		"type":          "ClusterIP",
	}

	if !reflect.DeepEqual(expected, params) {
		t.Errorf("Env param maps were not equal!  Expected (%s), got (%s).", expected, params)
	}
}
