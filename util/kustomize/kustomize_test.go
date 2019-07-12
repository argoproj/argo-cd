package kustomize

import (
	"io/ioutil"
	"path"
	"path/filepath"
	"testing"

	"github.com/argoproj/pkg/exec"
	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/git"
)

const kustomization1 = "kustomization_yaml"
const kustomization2a = "kustomization_yml"
const kustomization2b = "Kustomization"

func testDataDir() (string, error) {
	res, err := ioutil.TempDir("", "kustomize-test")
	if err != nil {
		return "", err
	}
	_, err = exec.RunCommand("cp", exec.CmdOpts{}, "-r", "./testdata/"+kustomization1, filepath.Join(res, "testdata"))
	if err != nil {
		return "", err
	}
	return path.Join(res, "testdata"), nil
}

func TestKustomizeBuild(t *testing.T) {
	appPath, err := testDataDir()
	assert.Nil(t, err)
	namePrefix := "namePrefix-"
	kustomize := NewKustomizeApp(appPath, git.NopCreds{})
	kustomizeSource := v1alpha1.ApplicationSourceKustomize{
		NamePrefix: namePrefix,
		ImageTags: []v1alpha1.KustomizeImageTag{
			{
				Name:  "k8s.gcr.io/nginx-slim",
				Value: "latest",
			},
		},
		Images: []string{"nginx:1.15.5"},
		CommonLabels: map[string]string{
			"app.kubernetes.io/managed-by": "argo-cd",
			"app.kubernetes.io/part-of":    "argo-cd-tests",
		},
	}
	objs, imageTags, images, err := kustomize.Build(&kustomizeSource)
	assert.Nil(t, err)
	if err != nil {
		assert.Equal(t, len(objs), 2)
		assert.Equal(t, len(imageTags), 0)
		assert.Equal(t, len(images), 2)
	}
	for _, obj := range objs {
		switch obj.GetKind() {
		case "StatefulSet":
			assert.Equal(t, namePrefix+"web", obj.GetName())
			assert.Equal(t, map[string]string{
				"app.kubernetes.io/managed-by": "argo-cd",
				"app.kubernetes.io/part-of":    "argo-cd-tests",
			}, obj.GetLabels())
		case "Deployment":
			assert.Equal(t, namePrefix+"nginx-deployment", obj.GetName())
			assert.Equal(t, map[string]string{
				"app":                          "nginx",
				"app.kubernetes.io/managed-by": "argo-cd",
				"app.kubernetes.io/part-of":    "argo-cd-tests",
			}, obj.GetLabels())
		}
	}

	for _, image := range images {
		switch image {
		case "nginx":
			assert.Equal(t, "1.15.5", image)
		case "k8s.gcr.io/nginx-slim":
			assert.Equal(t, "latest", image)
		}
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

func TestNewImageTag(t *testing.T) {
	tag := newImageTag(Image("busybox"))
	assert.Equal(t, tag.Name, "busybox")
	assert.Equal(t, tag.Value, "latest")

	tag = newImageTag(Image("k8s.gcr.io/nginx-slim:0.8"))
	assert.Equal(t, tag.Name, "k8s.gcr.io/nginx-slim")
	assert.Equal(t, tag.Value, "0.8")
}
