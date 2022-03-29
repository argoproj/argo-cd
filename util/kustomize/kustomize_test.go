package kustomize

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/argoproj/pkg/exec"
	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/git"
)

const kustomization1 = "kustomization_yaml"
const kustomization2a = "kustomization_yml"
const kustomization2b = "Kustomization"
const kustomization3 = "force_common"
const kustomization4 = "custom_version"

func testDataDir(testData string) (string, error) {
	res, err := ioutil.TempDir("", "kustomize-test")
	if err != nil {
		return "", err
	}
	_, err = exec.RunCommand("cp", exec.CmdOpts{}, "-r", "./testdata/"+testData, filepath.Join(res, "testdata"))
	if err != nil {
		return "", err
	}
	return path.Join(res, "testdata"), nil
}

func TestKustomizeBuild(t *testing.T) {
	appPath, err := testDataDir(kustomization1)
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
	objs, images, err := kustomize.Build(&kustomizeSource, nil, nil)
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

func TestGetSemver(t *testing.T) {
	ver, err := getSemver()
	assert.NoError(t, err)
	assert.NotEmpty(t, ver)
}

func TestKustomizeBuildForceCommonLabels(t *testing.T) {
	type testCase struct {
		TestData        string
		KustomizeSource v1alpha1.ApplicationSourceKustomize
		ExpectedLabels  map[string]string
		ExpectErr       bool
	}
	testCases := []testCase{
		{
			TestData: kustomization3,
			KustomizeSource: v1alpha1.ApplicationSourceKustomize{
				ForceCommonLabels: true,
				CommonLabels: map[string]string{
					"foo": "edited",
				},
			},
			ExpectedLabels: map[string]string{
				"app": "nginx",
				"foo": "edited",
			},
		},
		{
			TestData: kustomization3,
			KustomizeSource: v1alpha1.ApplicationSourceKustomize{
				ForceCommonLabels: false,
				CommonLabels: map[string]string{
					"foo": "edited",
				},
			},
			ExpectErr: true,
		},
	}
	for _, tc := range testCases {
		appPath, err := testDataDir(tc.TestData)
		assert.Nil(t, err)
		kustomize := NewKustomizeApp(appPath, git.NopCreds{}, "", "")
		objs, _, err := kustomize.Build(&tc.KustomizeSource, nil, nil)
		switch tc.ExpectErr {
		case true:
			assert.Error(t, err)
		default:
			assert.Nil(t, err)
			if assert.Equal(t, len(objs), 1) {
				assert.Equal(t, tc.ExpectedLabels, objs[0].GetLabels())
			}
		}
	}
}

func TestKustomizeBuildForceCommonAnnotations(t *testing.T) {
	type testCase struct {
		TestData            string
		KustomizeSource     v1alpha1.ApplicationSourceKustomize
		ExpectedAnnotations map[string]string
		ExpectErr           bool
	}
	testCases := []testCase{
		{
			TestData: kustomization3,
			KustomizeSource: v1alpha1.ApplicationSourceKustomize{
				ForceCommonAnnotations: true,
				CommonAnnotations: map[string]string{
					"one": "edited",
				},
			},
			ExpectedAnnotations: map[string]string{
				"baz": "quux",
				"one": "edited",
			},
		},
		{
			TestData: kustomization3,
			KustomizeSource: v1alpha1.ApplicationSourceKustomize{
				ForceCommonAnnotations: false,
				CommonAnnotations: map[string]string{
					"one": "edited",
				},
			},
			ExpectErr: true,
		},
	}
	for _, tc := range testCases {
		appPath, err := testDataDir(tc.TestData)
		assert.Nil(t, err)
		kustomize := NewKustomizeApp(appPath, git.NopCreds{}, "", "")
		objs, _, err := kustomize.Build(&tc.KustomizeSource, nil, nil)
		switch tc.ExpectErr {
		case true:
			assert.Error(t, err)
		default:
			assert.Nil(t, err)
			if assert.Equal(t, len(objs), 1) {
				assert.Equal(t, tc.ExpectedAnnotations, objs[0].GetAnnotations())
			}
		}
	}
}

func TestKustomizeCustomVersion(t *testing.T) {
	appPath, err := testDataDir(kustomization1)
	assert.Nil(t, err)
	kustomizePath, err := testDataDir(kustomization4)
	assert.Nil(t, err)
	envOutputFile := kustomizePath + "/env_output"
	kustomize := NewKustomizeApp(appPath, git.NopCreds{}, "", kustomizePath+"/kustomize.special")
	kustomizeSource := v1alpha1.ApplicationSourceKustomize{
		Version: "special",
	}
	env := &v1alpha1.Env{
		&v1alpha1.EnvEntry{Name: "ARGOCD_APP_NAME", Value: "argo-cd-tests"},
	}
	objs, images, err := kustomize.Build(&kustomizeSource, nil, env)
	assert.Nil(t, err)
	if err != nil {
		assert.Equal(t, len(objs), 2)
		assert.Equal(t, len(images), 2)
	}

	content, err := os.ReadFile(envOutputFile)
	assert.Nil(t, err)
	assert.Equal(t, "ARGOCD_APP_NAME=argo-cd-tests\n", string(content))
}
