package kustomize

import (
	"io/ioutil"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/pkg/exec"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

const kustomization1 = "kustomization_yaml"
const kustomization2a = "kustomization_yml"
const kustomization2b = "Kustomization"

func testDataDir() (string, error) {
	res, err := ioutil.TempDir("", "kustomize-test")
	if err != nil {
		return "", err
	}
	_, err = exec.RunCommand("cp", "-r", "./testdata/"+kustomization1, filepath.Join(res, "testdata"))
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
	kustomizeSource := v1alpha1.ApplicationSourceKustomize{
		NamePrefix: namePrefix,
		ImageTags: []v1alpha1.KustomizeImageTag{
			{
				Name:  "k8s.gcr.io/nginx-slim",
				Value: "latest",
			},
		},
	}
	objs, params, err := kustomize.Build(&kustomizeSource)
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

func TestFindKustomization(t *testing.T) {
	testFindKustomization(t, kustomization1, "kustomization.yaml")
	testFindKustomization(t, kustomization2a, "kustomization.yml")
	testFindKustomization(t, kustomization2b, "Kustomization")
}

func testFindKustomization(t *testing.T, set string, expected string) {
	kustomization, err := (&kustomize{path: "testdata/" + set}).findKustomization()
	assert.Nil(t, err)
	assert.Equal(t, "testdata/"+set+"/"+expected, kustomization)
}

func TestGetKustomizationVersion(t *testing.T) {
	testGetKustomizationVersion(t, kustomization1, 1)
	testGetKustomizationVersion(t, kustomization2a, 2)
	testGetKustomizationVersion(t, kustomization2b, 2)
}

func testGetKustomizationVersion(t *testing.T, set string, expected int) {
	version, err := (&kustomize{path: "testdata/" + set}).getKustomizationVersion()
	assert.Nil(t, err)
	assert.Equal(t, expected, version)
}

func TestGetCommandName(t *testing.T) {
	assert.Equal(t, "kustomize1", GetCommandName(1))
	assert.Equal(t, "kustomize", GetCommandName(2))
}

func TestIsKustomization(t *testing.T) {

	assert.True(t, IsKustomization("kustomization.yaml"))
	assert.True(t, IsKustomization("kustomization.yml"))
	assert.True(t, IsKustomization("Kustomization"))
	assert.False(t, IsKustomization("rubbish.yml"))
}
