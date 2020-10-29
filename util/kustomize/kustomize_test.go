package kustomize

import (
	"fmt"
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
	nameSuffix := "-nameSuffix"
	kustomize := NewKustomizeApp(appPath, git.NopCreds{}, "", "")
	kustomizeSource := v1alpha1.ApplicationSourceKustomize{
		NamePrefix: namePrefix,
		NameSuffix: nameSuffix,
		Images:     v1alpha1.KustomizeImages{"nginx:1.15.5"},
		CommonLabels: map[string]string{
			"app.kubernetes.io/managed-by": "argo-cd",
			"app.kubernetes.io/part-of":    "argo-cd-tests",
		},
		CommonAnnotations: map[string]string{
			"app.kubernetes.io/managed-by": "argo-cd",
			"app.kubernetes.io/part-of":    "argo-cd-tests",
		},
	}
	objs, images, err := kustomize.Build(&kustomizeSource, nil)
	assert.Nil(t, err)
	if err != nil {
		assert.Equal(t, len(objs), 2)
		assert.Equal(t, len(images), 2)
	}
	for _, obj := range objs {
		fmt.Println(obj.GetAnnotations())
		switch obj.GetKind() {
		case "StatefulSet":
			assert.Equal(t, namePrefix+"web"+nameSuffix, obj.GetName())
			assert.Equal(t, map[string]string{
				"app.kubernetes.io/managed-by": "argo-cd",
				"app.kubernetes.io/part-of":    "argo-cd-tests",
			}, obj.GetLabels())
			assert.Equal(t, map[string]string{
				"app.kubernetes.io/managed-by": "argo-cd",
				"app.kubernetes.io/part-of":    "argo-cd-tests",
			}, obj.GetAnnotations())
		case "Deployment":
			assert.Equal(t, namePrefix+"nginx-deployment"+nameSuffix, obj.GetName())
			assert.Equal(t, map[string]string{
				"app":                          "nginx",
				"app.kubernetes.io/managed-by": "argo-cd",
				"app.kubernetes.io/part-of":    "argo-cd-tests",
			}, obj.GetLabels())
			assert.Equal(t, map[string]string{
				"app.kubernetes.io/managed-by": "argo-cd",
				"app.kubernetes.io/part-of":    "argo-cd-tests",
			}, obj.GetAnnotations())
		}
	}

	for _, image := range images {
		switch image {
		case "nginx":
			assert.Equal(t, "1.15.5", image)
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

func TestIsKustomization(t *testing.T) {
	assert.True(t, IsKustomization("kustomization.yaml"))
	assert.True(t, IsKustomization("kustomization.yml"))
	assert.True(t, IsKustomization("Kustomization"))
	assert.False(t, IsKustomization("rubbish.yml"))
}

func TestParseKustomizeBuildOptions(t *testing.T) {
	built := parseKustomizeBuildOptions("guestbook", "-v 6 --logtostderr")
	assert.Equal(t, []string{"build", "guestbook", "-v", "6", "--logtostderr"}, built)
}

func TestVersion(t *testing.T) {
	ver, err := Version(false)
	assert.NoError(t, err)
	assert.NotEmpty(t, ver)
}
