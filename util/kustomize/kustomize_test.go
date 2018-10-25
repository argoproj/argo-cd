package kustomize

import (
	"io/ioutil"
	"path"
	"testing"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"

	"github.com/argoproj/pkg/exec"
	"github.com/stretchr/testify/assert"
)

func testDataDir() (string, error) {
	res, err := ioutil.TempDir("", "kustomize-test")
	if err != nil {
		return "", err
	}
	_, err = exec.RunCommand("cp", "-r", "./testdata", res)
	if err != nil {
		return "", err
	}
	return path.Join(res, "testdata"), nil
}

func TestKustomizeBuild(t *testing.T) {
	appPath, err := testDataDir()
	assert.Nil(t, err)

	kustomize := NewKustomizeApp(appPath)
	objs, params, err := kustomize.Build("mynamespace", []*v1alpha1.ComponentParameter{{
		Component: "imagetag",
		Name:      "k8s.gcr.io/nginx-slim",
		Value:     "latest",
	}})
	assert.Nil(t, err)
	if err != nil {
		assert.Equal(t, len(objs), 2)
		assert.Equal(t, len(params), 2)
	}
	for _, obj := range objs {
		switch obj.GetKind() {
		case "StatefulSet":
			assert.Equal(t, "web", obj.GetName())
		case "Deployment":
			assert.Equal(t, "nginx-deployment", obj.GetName())
		}
		assert.Equal(t, "mynamespace", obj.GetNamespace())
	}

	for _, param := range params {
		switch param.Value {
		case "nginx":
			assert.Equal(t, "1.15.4", param.Value)
		case "k8s.gcr.io/nginx-slim":
			assert.Equal(t, "latest", param.Value)
		}
		assert.Equal(t, "imagetag", param.Component)
	}
}
