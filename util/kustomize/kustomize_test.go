package kustomize

import (
	"io/ioutil"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/pkg/exec"
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
	namePrefix := "namePrefix-"
	kustomize := NewKustomizeApp(appPath)
	opts := KustomizeBuildOpts{
		NamePrefix: namePrefix,
	}
	objs, params, err := kustomize.Build(opts, []*v1alpha1.ComponentParameter{{
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
			assert.Equal(t, namePrefix+"web", obj.GetName())
		case "Deployment":
			assert.Equal(t, namePrefix+"nginx-deployment", obj.GetName())
		}
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
