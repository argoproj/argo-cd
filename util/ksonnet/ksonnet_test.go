package ksonnet

import (
	"encoding/json"
	"path"
	"reflect"
	"runtime"
	"testing"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
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
	paramPointers, err := ksApp.ListEnvParams(testEnvName)
	assert.Nil(t, err)
	params := make([]v1alpha1.ComponentParameter, len(paramPointers))
	for i, paramPointer := range paramPointers {
		param := *paramPointer
		params[i] = param
	}

	expected := []v1alpha1.ComponentParameter{{
		Component: "demo",
		Name:      "containerPort",
		Value:     "80",
	}, {
		Component: "demo",
		Name:      "image",
		Value:     "gcr.io/kuar-demo/kuard-amd64:1",
	}, {
		Component: "demo",
		Name:      "name",
		Value:     "demo",
	}, {
		Component: "demo",
		Name:      "replicas",
		Value:     "2",
	}, {
		Component: "demo",
		Name:      "servicePort",
		Value:     "80",
	}, {
		Component: "demo",
		Name:      "type",
		Value:     "ClusterIP",
	}}

	if !reflect.DeepEqual(expected, params) {
		t.Errorf("Env params were not equal!  Expected (%s), got (%s).", expected, params)
	}
}
