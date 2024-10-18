package kustomize

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/argoproj/pkg/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/git"
)

const kustomization1 = "kustomization_yaml"
const kustomization2a = "kustomization_yml"
const kustomization2b = "Kustomization"
const kustomization3 = "force_common"
const kustomization4 = "custom_version"
const kustomization5 = "kustomization_yaml_patches"
const kustomization6 = "kustomization_yaml_components"

func testDataDir(tb testing.TB, testData string) (string, error) {
	res := tb.TempDir()
	_, err := exec.RunCommand("cp", exec.CmdOpts{}, "-r", "./testdata/"+testData, filepath.Join(res, "testdata"))
	if err != nil {
		return "", err
	}
	return path.Join(res, "testdata"), nil
}

func TestKustomizeBuild(t *testing.T) {
	appPath, err := testDataDir(t, kustomization1)
	assert.Nil(t, err)
	namePrefix := "namePrefix-"
	nameSuffix := "-nameSuffix"
	namespace := "custom-namespace"
	kustomize := NewKustomizeApp(appPath, appPath, git.NopCreds{}, "", "")
	env := &v1alpha1.Env{
		&v1alpha1.EnvEntry{Name: "ARGOCD_APP_NAME", Value: "argo-cd-tests"},
	}
	kustomizeSource := v1alpha1.ApplicationSourceKustomize{
		NamePrefix: namePrefix,
		NameSuffix: nameSuffix,
		Images:     v1alpha1.KustomizeImages{"nginx:1.15.5"},
		CommonLabels: map[string]string{
			"app.kubernetes.io/managed-by": "argo-cd",
			"app.kubernetes.io/part-of":    "${ARGOCD_APP_NAME}",
		},
		CommonAnnotations: map[string]string{
			"app.kubernetes.io/managed-by": "argo-cd",
			"app.kubernetes.io/part-of":    "${ARGOCD_APP_NAME}",
		},
		Namespace:                 namespace,
		CommonAnnotationsEnvsubst: true,
		Replicas: []v1alpha1.KustomizeReplica{
			{
				Name:  "nginx-deployment",
				Count: intstr.FromInt(2),
			},
			{
				Name:  "web",
				Count: intstr.FromString("4"),
			},
		},
	}
	objs, images, err := kustomize.Build(&kustomizeSource, nil, env)
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
			replicas, ok, err := unstructured.NestedInt64(obj.Object, "spec", "replicas")
			require.NoError(t, err)
			require.True(t, ok)
			assert.Equal(t, int64(4), replicas)
			assert.Equal(t, namespace, obj.GetNamespace())
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
			replicas, ok, err := unstructured.NestedInt64(obj.Object, "spec", "replicas")
			require.NoError(t, err)
			require.True(t, ok)
			assert.Equal(t, int64(2), replicas)
			assert.Equal(t, namespace, obj.GetNamespace())
		}
	}

	for _, image := range images {
		switch image {
		case "nginx":
			assert.Equal(t, "1.15.5", image)
		}
	}
}

func TestFailKustomizeBuild(t *testing.T) {
	appPath, err := testDataDir(t, kustomization1)
	assert.Nil(t, err)
	kustomize := NewKustomizeApp(appPath, appPath, git.NopCreds{}, "", "")
	kustomizeSource := v1alpha1.ApplicationSourceKustomize{
		Replicas: []v1alpha1.KustomizeReplica{
			{
				Name:  "nginx-deployment",
				Count: intstr.Parse("garbage"),
			},
		},
	}
	_, _, err = kustomize.Build(&kustomizeSource, nil, nil)
	assert.EqualError(t, err, "expected integer value for count. Received: garbage")
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
		Env             *v1alpha1.Env
	}
	testCases := []testCase{
		{
			TestData: kustomization3,
			KustomizeSource: v1alpha1.ApplicationSourceKustomize{
				ForceCommonLabels: true,
				CommonLabels: map[string]string{
					"foo":  "edited",
					"test": "${ARGOCD_APP_NAME}",
				},
			},
			ExpectedLabels: map[string]string{
				"app":  "nginx",
				"foo":  "edited",
				"test": "argo-cd-tests",
			},
			Env: &v1alpha1.Env{
				&v1alpha1.EnvEntry{
					Name:  "ARGOCD_APP_NAME",
					Value: "argo-cd-tests",
				},
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
			Env: &v1alpha1.Env{
				&v1alpha1.EnvEntry{
					Name:  "ARGOCD_APP_NAME",
					Value: "argo-cd-tests",
				},
			},
		},
	}
	for _, tc := range testCases {
		appPath, err := testDataDir(t, tc.TestData)
		assert.Nil(t, err)
		kustomize := NewKustomizeApp(appPath, appPath, git.NopCreds{}, "", "")
		objs, _, err := kustomize.Build(&tc.KustomizeSource, nil, tc.Env)
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
		Env                 *v1alpha1.Env
	}
	testCases := []testCase{
		{
			TestData: kustomization3,
			KustomizeSource: v1alpha1.ApplicationSourceKustomize{
				ForceCommonAnnotations: true,
				CommonAnnotations: map[string]string{
					"one":   "edited",
					"two":   "${test}",
					"three": "$ARGOCD_APP_NAME",
				},
				CommonAnnotationsEnvsubst: false,
			},
			ExpectedAnnotations: map[string]string{
				"baz":   "quux",
				"one":   "edited",
				"two":   "${test}",
				"three": "$ARGOCD_APP_NAME",
			},
			Env: &v1alpha1.Env{
				&v1alpha1.EnvEntry{
					Name:  "ARGOCD_APP_NAME",
					Value: "argo-cd-tests",
				},
			},
		},
		{
			TestData: kustomization3,
			KustomizeSource: v1alpha1.ApplicationSourceKustomize{
				ForceCommonAnnotations: true,
				CommonAnnotations: map[string]string{
					"one":   "edited",
					"two":   "${test}",
					"three": "$ARGOCD_APP_NAME",
				},
				CommonAnnotationsEnvsubst: true,
			},
			ExpectedAnnotations: map[string]string{
				"baz":   "quux",
				"one":   "edited",
				"two":   "",
				"three": "argo-cd-tests",
			},
			Env: &v1alpha1.Env{
				&v1alpha1.EnvEntry{
					Name:  "ARGOCD_APP_NAME",
					Value: "argo-cd-tests",
				},
			},
		},
		{
			TestData: kustomization3,
			KustomizeSource: v1alpha1.ApplicationSourceKustomize{
				ForceCommonAnnotations: false,
				CommonAnnotations: map[string]string{
					"one": "edited",
				},
				CommonAnnotationsEnvsubst: true,
			},
			ExpectErr: true,
			Env: &v1alpha1.Env{
				&v1alpha1.EnvEntry{
					Name:  "ARGOCD_APP_NAME",
					Value: "argo-cd-tests",
				},
			},
		},
	}
	for _, tc := range testCases {
		appPath, err := testDataDir(t, tc.TestData)
		assert.Nil(t, err)
		kustomize := NewKustomizeApp(appPath, appPath, git.NopCreds{}, "", "")
		objs, _, err := kustomize.Build(&tc.KustomizeSource, nil, tc.Env)
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
	appPath, err := testDataDir(t, kustomization1)
	assert.Nil(t, err)
	kustomizePath, err := testDataDir(t, kustomization4)
	assert.Nil(t, err)
	envOutputFile := kustomizePath + "/env_output"
	kustomize := NewKustomizeApp(appPath, appPath, git.NopCreds{}, "", kustomizePath+"/kustomize.special")
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

func TestKustomizeBuildComponents(t *testing.T) {
	appPath, err := testDataDir(t, kustomization6)
	assert.Nil(t, err)
	kustomize := NewKustomizeApp(appPath, appPath, git.NopCreds{}, "", "")

	kustomizeSource := v1alpha1.ApplicationSourceKustomize{
		Components: []string{"./components"},
	}
	objs, _, err := kustomize.Build(&kustomizeSource, nil, nil)
	assert.Nil(t, err)
	obj := objs[0]
	assert.Equal(t, "nginx-deployment", obj.GetName())
	assert.Equal(t, map[string]string{
		"app": "nginx",
	}, obj.GetLabels())
	replicas, ok, err := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, int64(3), replicas)
}

func TestKustomizeBuildPatches(t *testing.T) {
	appPath, err := testDataDir(t, kustomization5)
	assert.Nil(t, err)
	kustomize := NewKustomizeApp(appPath, appPath, git.NopCreds{}, "", "")

	kustomizeSource := v1alpha1.ApplicationSourceKustomize{
		Patches: []v1alpha1.KustomizePatch{
			{
				Patch: `[ { "op": "replace", "path": "/spec/template/spec/containers/0/ports/0/containerPort", "value": 443 },  { "op": "replace", "path": "/spec/template/spec/containers/0/name", "value": "test" }]`,
				Target: &v1alpha1.KustomizeSelector{
					KustomizeResId: v1alpha1.KustomizeResId{
						KustomizeGvk: v1alpha1.KustomizeGvk{
							Kind: "Deployment",
						},
						Name: "nginx-deployment",
					},
				},
			},
		},
	}
	objs, _, err := kustomize.Build(&kustomizeSource, nil, nil)
	assert.Nil(t, err)
	obj := objs[0]
	containers, found, err := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
	assert.Nil(t, err)
	assert.Equal(t, found, true)

	ports, found, err := unstructured.NestedSlice(
		containers[0].(map[string]interface{}),
		"ports",
	)
	assert.Equal(t, found, true)
	assert.Nil(t, err)

	port, found, err := unstructured.NestedInt64(
		ports[0].(map[string]interface{}),
		"containerPort",
	)

	assert.Equal(t, found, true)
	assert.Nil(t, err)
	assert.Equal(t, port, int64(443))

	name, found, err := unstructured.NestedString(
		containers[0].(map[string]interface{}),
		"name",
	)
	assert.Equal(t, found, true)
	assert.Nil(t, err)
	assert.Equal(t, name, "test")
}
