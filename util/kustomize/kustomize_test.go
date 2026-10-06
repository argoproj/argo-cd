package kustomize

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/exec"
	"github.com/argoproj/argo-cd/v3/util/git"
)

const (
	kustomization1 = "kustomization_yaml"
	kustomization3 = "force_common"
	kustomization4 = "custom_version"
	kustomization5 = "kustomization_yaml_patches"
	kustomization6 = "kustomization_yaml_components"
	kustomization7 = "label_without_selector"
	kustomization8 = "kustomization_yaml_patches_empty"
	kustomization9 = "kustomization_yaml_components_monorepo"
)

func testDataDir(tb testing.TB, testData string) (string, error) {
	tb.Helper()
	res := tb.TempDir()
	_, err := exec.RunCommand("cp", exec.CmdOpts{}, "-r", "./testdata/"+testData, filepath.Join(res, "testdata"))
	if err != nil {
		return "", err
	}
	return path.Join(res, "testdata"), nil
}

func TestKustomizeBuild(t *testing.T) {
	appPath, err := testDataDir(t, kustomization1)
	require.NoError(t, err)
	namePrefix := "namePrefix-"
	nameSuffix := "-nameSuffix"
	namespace := "custom-namespace"
	kustomize := NewKustomizeApp(appPath, appPath, git.NopCreds{}, "", "", "", "")
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
				Count: intstr.FromInt32(2),
			},
			{
				Name:  "web",
				Count: intstr.FromString("4"),
			},
		},
	}
	objs, images, _, err := kustomize.Build(&kustomizeSource, nil, env, &BuildOpts{
		KubeVersion: "1.27", APIVersions: []string{"foo", "bar"},
	})
	require.NoError(t, err)
	if err != nil {
		assert.Len(t, objs, 2)
		assert.Len(t, images, 2)
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
		if image == "nginx" {
			assert.Equal(t, "1.15.5", image)
		}
	}
}

func TestFailKustomizeBuild(t *testing.T) {
	appPath, err := testDataDir(t, kustomization1)
	require.NoError(t, err)
	kustomize := NewKustomizeApp(appPath, appPath, git.NopCreds{}, "", "", "", "")
	kustomizeSource := v1alpha1.ApplicationSourceKustomize{
		Replicas: []v1alpha1.KustomizeReplica{
			{
				Name:  "nginx-deployment",
				Count: intstr.Parse("garbage"),
			},
		},
	}
	_, _, _, err = kustomize.Build(&kustomizeSource, nil, nil, nil)
	assert.EqualError(t, err, "expected integer value for count. Received: garbage")
}

func TestIsKustomization(t *testing.T) {
	assert.True(t, IsKustomization("kustomization.yaml"))
	assert.True(t, IsKustomization("kustomization.yml"))
	assert.True(t, IsKustomization("Kustomization"))
	assert.False(t, IsKustomization("rubbish.yml"))
}

func TestParseKustomizeBuildOptions(t *testing.T) {
	built := parseKustomizeBuildOptions(t.Context(), &kustomize{path: "guestbook"}, "-v 6 --logtostderr", &BuildOpts{
		KubeVersion: "1.27", APIVersions: []string{"foo", "bar"},
	})
	// Helm is not enabled so helm options are not in the params
	assert.Equal(t, []string{"build", "guestbook", "-v", "6", "--logtostderr"}, built)
}

func TestParseKustomizeBuildHelmOptions(t *testing.T) {
	built := parseKustomizeBuildOptions(t.Context(), &kustomize{path: "guestbook"}, "-v 6 --logtostderr --enable-helm", &BuildOpts{
		KubeVersion: "1.27",
		APIVersions: []string{"foo", "bar"},
	})
	assert.Equal(t, []string{
		"build", "guestbook",
		"-v", "6", "--logtostderr", "--enable-helm",
		"--helm-kube-version", "1.27",
		"--helm-api-versions", "foo", "--helm-api-versions", "bar",
	}, built)
}

func TestVersion(t *testing.T) {
	ver, err := Version()
	require.NoError(t, err)
	assert.NotEmpty(t, ver)
}

func TestVersionWithBinaryPath(t *testing.T) {
	ver, err := versionWithBinaryPath(t.Context(), &kustomize{binaryPath: "kustomize"})
	require.NoError(t, err)
	assert.NotEmpty(t, ver)
}

func TestGetSemver(t *testing.T) {
	ver, err := getSemver(t.Context(), &kustomize{})
	require.NoError(t, err)
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
		require.NoError(t, err)
		kustomize := NewKustomizeApp(appPath, appPath, git.NopCreds{}, "", "", "", "")
		objs, _, _, err := kustomize.Build(&tc.KustomizeSource, nil, tc.Env, nil)
		switch tc.ExpectErr {
		case true:
			require.Error(t, err)
		default:
			require.NoError(t, err)
			if assert.Len(t, objs, 1) {
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
		require.NoError(t, err)
		kustomize := NewKustomizeApp(appPath, appPath, git.NopCreds{}, "", "", "", "")
		objs, _, _, err := kustomize.Build(&tc.KustomizeSource, nil, tc.Env, nil)
		switch tc.ExpectErr {
		case true:
			require.Error(t, err)
		default:
			require.NoError(t, err)
			if assert.Len(t, objs, 1) {
				assert.Equal(t, tc.ExpectedAnnotations, objs[0].GetAnnotations())
			}
		}
	}
}

func TestKustomizeLabelWithoutSelector(t *testing.T) {
	type testCase struct {
		TestData               string
		KustomizeSource        v1alpha1.ApplicationSourceKustomize
		ExpectedMetadataLabels map[string]string
		ExpectedSelectorLabels map[string]string
		ExpectedTemplateLabels map[string]string
		ExpectErr              bool
		Env                    *v1alpha1.Env
	}
	testCases := []testCase{
		{
			TestData: kustomization7,
			KustomizeSource: v1alpha1.ApplicationSourceKustomize{
				CommonLabels: map[string]string{
					"foo": "bar",
				},
				LabelWithoutSelector: true,
			},
			ExpectedMetadataLabels: map[string]string{"app": "nginx", "managed-by": "helm", "foo": "bar"},
			ExpectedSelectorLabels: map[string]string{"app": "nginx"},
			ExpectedTemplateLabels: map[string]string{"app": "nginx"},
			Env: &v1alpha1.Env{
				&v1alpha1.EnvEntry{
					Name:  "ARGOCD_APP_NAME",
					Value: "argo-cd-tests",
				},
			},
		},
		{
			TestData: kustomization7,
			KustomizeSource: v1alpha1.ApplicationSourceKustomize{
				CommonLabels: map[string]string{
					"managed-by": "argocd",
				},
				LabelWithoutSelector: true,
				ForceCommonLabels:    true,
			},
			ExpectedMetadataLabels: map[string]string{"app": "nginx", "managed-by": "argocd"},
			ExpectedSelectorLabels: map[string]string{"app": "nginx"},
			ExpectedTemplateLabels: map[string]string{"app": "nginx"},
			Env: &v1alpha1.Env{
				&v1alpha1.EnvEntry{
					Name:  "ARGOCD_APP_NAME",
					Value: "argo-cd-tests",
				},
			},
		},
		{
			TestData: kustomization7,
			KustomizeSource: v1alpha1.ApplicationSourceKustomize{
				CommonLabels: map[string]string{
					"foo": "bar",
				},
				LabelWithoutSelector:  true,
				LabelIncludeTemplates: true,
			},
			ExpectedMetadataLabels: map[string]string{"app": "nginx", "managed-by": "helm", "foo": "bar"},
			ExpectedSelectorLabels: map[string]string{"app": "nginx"},
			ExpectedTemplateLabels: map[string]string{"app": "nginx", "foo": "bar"},
			Env: &v1alpha1.Env{
				&v1alpha1.EnvEntry{
					Name:  "ARGOCD_APP_NAME",
					Value: "argo-cd-tests",
				},
			},
		},
		{
			TestData: kustomization7,
			KustomizeSource: v1alpha1.ApplicationSourceKustomize{
				CommonLabels: map[string]string{
					"managed-by": "argocd",
				},
				LabelWithoutSelector:  true,
				LabelIncludeTemplates: true,
				ForceCommonLabels:     true,
			},
			ExpectedMetadataLabels: map[string]string{"app": "nginx", "managed-by": "argocd"},
			ExpectedSelectorLabels: map[string]string{"app": "nginx"},
			ExpectedTemplateLabels: map[string]string{"app": "nginx", "managed-by": "argocd"},
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
		require.NoError(t, err)
		kustomize := NewKustomizeApp(appPath, appPath, git.NopCreds{}, "", "", "", "")
		objs, _, _, err := kustomize.Build(&tc.KustomizeSource, nil, tc.Env, nil)

		switch tc.ExpectErr {
		case true:
			require.Error(t, err)
		default:
			require.NoError(t, err)
			if assert.Len(t, objs, 1) {
				obj := objs[0]
				sl, found, err := unstructured.NestedStringMap(obj.Object, "spec", "selector", "matchLabels")
				require.NoError(t, err)
				assert.True(t, found)
				tl, found, err := unstructured.NestedStringMap(obj.Object, "spec", "template", "metadata", "labels")
				require.NoError(t, err)
				assert.True(t, found)
				assert.Equal(t, tc.ExpectedMetadataLabels, obj.GetLabels())
				assert.Equal(t, tc.ExpectedSelectorLabels, sl)
				assert.Equal(t, tc.ExpectedTemplateLabels, tl)
			}
		}
	}
}

func TestKustomizeCustomVersion(t *testing.T) {
	appPath, err := testDataDir(t, kustomization1)
	require.NoError(t, err)
	kustomizePath, err := testDataDir(t, kustomization4)
	require.NoError(t, err)
	envOutputFile := kustomizePath + "/env_output"
	kustomize := NewKustomizeApp(appPath, appPath, git.NopCreds{}, "", kustomizePath+"/kustomize.special", "", "")
	kustomizeSource := v1alpha1.ApplicationSourceKustomize{
		Version: "special",
	}
	env := &v1alpha1.Env{
		&v1alpha1.EnvEntry{Name: "ARGOCD_APP_NAME", Value: "argo-cd-tests"},
	}
	objs, images, _, err := kustomize.Build(&kustomizeSource, nil, env, nil)
	require.NoError(t, err)
	if err != nil {
		assert.Len(t, objs, 2)
		assert.Len(t, images, 2)
	}

	content, err := os.ReadFile(envOutputFile)
	require.NoError(t, err)
	assert.Equal(t, "ARGOCD_APP_NAME=argo-cd-tests\n", string(content))
}

func TestKustomizeBuildComponents(t *testing.T) {
	appPath, err := testDataDir(t, kustomization6)
	require.NoError(t, err)
	kustomize := NewKustomizeApp(appPath, appPath, git.NopCreds{}, "", "", "", "")

	kustomizeSource := v1alpha1.ApplicationSourceKustomize{
		Components:              []string{"./components", "./missing-components"},
		IgnoreMissingComponents: false,
	}
	_, _, _, err = kustomize.Build(&kustomizeSource, nil, nil, nil)
	require.Error(t, err)

	kustomizeSource = v1alpha1.ApplicationSourceKustomize{
		Components:              []string{"./components", "./missing-components"},
		IgnoreMissingComponents: true,
	}
	objs, _, _, err := kustomize.Build(&kustomizeSource, nil, nil, nil)
	require.NoError(t, err)
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

func TestKustomizeBuildComponentsMonoRepo(t *testing.T) {
	rootPath, err := testDataDir(t, kustomization9)
	require.NoError(t, err)
	appPath := path.Join(rootPath, "envs/inseng-pdx-egert-sandbox/namespaces/inst-system/apps/hello-world")
	kustomize := NewKustomizeApp(rootPath, appPath, git.NopCreds{}, "", "", "", "")
	kustomizeSource := v1alpha1.ApplicationSourceKustomize{
		Components:              []string{"../../../../../../kustomize/components/all"},
		IgnoreMissingComponents: true,
	}
	objs, _, _, err := kustomize.Build(&kustomizeSource, nil, nil, nil)
	require.NoError(t, err)
	obj := objs[2]
	require.Equal(t, "hello-world-kustomize", obj.GetName())
	require.Equal(t, map[string]string{
		"app.kubernetes.io/name":  "hello-world-kustomize",
		"app.kubernetes.io/owner": "fire-team",
	}, obj.GetLabels())
	replicas, ok, err := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "tolerations")
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, replicas, 1)
	require.Equal(t, "my-special-toleration", replicas[0].(map[string]any)["key"])
	require.Equal(t, "Exists", replicas[0].(map[string]any)["operator"])
}

func TestKustomizeBuildPatches(t *testing.T) {
	appPath, err := testDataDir(t, kustomization5)
	require.NoError(t, err)
	kustomize := NewKustomizeApp(appPath, appPath, git.NopCreds{}, "", "", "", "")

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
	objs, _, _, err := kustomize.Build(&kustomizeSource, nil, nil, nil)
	require.NoError(t, err)
	obj := objs[0]
	containers, found, err := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
	require.NoError(t, err)
	assert.True(t, found)

	ports, found, err := unstructured.NestedSlice(
		containers[0].(map[string]any),
		"ports",
	)
	assert.True(t, found)
	require.NoError(t, err)

	port, found, err := unstructured.NestedInt64(
		ports[0].(map[string]any),
		"containerPort",
	)

	assert.True(t, found)
	require.NoError(t, err)
	assert.Equal(t, int64(443), port)

	name, found, err := unstructured.NestedString(
		containers[0].(map[string]any),
		"name",
	)
	assert.True(t, found)
	require.NoError(t, err)
	assert.Equal(t, "test", name)
}

func TestFailKustomizeBuildPatches(t *testing.T) {
	appPath, err := testDataDir(t, kustomization8)
	require.NoError(t, err)
	kustomize := NewKustomizeApp(appPath, appPath, git.NopCreds{}, "", "", "", "")

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

	_, _, _, err = kustomize.Build(&kustomizeSource, nil, nil, nil)
	require.EqualError(t, err, "kustomization file not found in the path")
}

func TestKustomizeBuildComponentsNoFoundComponents(t *testing.T) {
	appPath, err := testDataDir(t, kustomization6)
	require.NoError(t, err)
	kustomize := NewKustomizeApp(appPath, appPath, git.NopCreds{}, "", "", "", "")

	// Test with non-existent components and IgnoreMissingComponents = true
	// This should result in foundComponents being empty, so no "edit add component" command should be executed
	kustomizeSource := v1alpha1.ApplicationSourceKustomize{
		Components:              []string{"./non-existent-component1", "./non-existent-component2"},
		IgnoreMissingComponents: true,
	}
	_, _, commands, err := kustomize.Build(&kustomizeSource, nil, nil, nil)
	require.NoError(t, err)

	// Verify that no "edit add component" command was executed
	for _, cmd := range commands {
		assert.NotContains(t, cmd, "edit add component", "kustomize edit add component should not be invoked when foundComponents is empty")
	}
}

// TestKustomizeBuildGitRefArgumentInjection is a regression test for GHSA-9v9p-x54c-58gc.
//
// kustomize passes a remote resource's `ref=` query value unsanitized to `git fetch` as a
// positional argument. A value beginning with `-`, such as `--upload-pack=<command>`, is instead
// interpreted by git as a flag, and `--upload-pack` makes git execute an arbitrary local command
// while resolving the "remote" side of a file:// transfer - achieving command execution wherever
// `kustomize build` runs.
//
// This is closed by a git wrapper installed ahead of the real git on the build subprocess's PATH.
// It inserts `--end-of-options` before a `fetch`'s positional arguments, so a
// `--upload-pack=<command>` ref is treated as an ordinary (non-existent) refspec rather than a git
// option, and no command is executed - while remote bases remain a fully supported feature. See
// gitwrapper.go.
func TestKustomizeBuildGitRefArgumentInjection(t *testing.T) {
	appPath := t.TempDir()
	marker := filepath.Join(t.TempDir(), "pwned")

	// Base64-wrap the payload so its shell metacharacters and spaces survive being embedded in a
	// URL query parameter and re-parsed by git, mirroring the technique used in the public PoC.
	script := "touch " + marker
	encoded := base64.StdEncoding.EncodeToString([]byte(script))
	runner := url.QueryEscape(fmt.Sprintf(`sh -c "echo %s|base64 -d|sh"`, encoded))
	kustomization := fmt.Sprintf(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- 'file:///a/b/c?ref=--upload-pack=%s'
`, runner)
	require.NoError(t, os.WriteFile(filepath.Join(appPath, "kustomization.yaml"), []byte(kustomization), 0o644))

	k := NewKustomizeApp(appPath, appPath, git.NopCreds{}, "", "", "", "")
	// The build always fails (the "remote" cannot be resolved); we only care whether the payload
	// ran as a side effect.
	_, _, _, _ = k.Build(nil, nil, nil, &BuildOpts{})

	assert.NoFileExists(t, marker, "the git-ref-injected command must not execute")
}

// TestKustomizeBuildHelmChartsIgnoresConfigHomePlugin is a regression test for
// GHSA-fw5c-w8rc-j7fx: a kustomization can point helmGlobals.configHome at a directory of its own
// choosing, and Helm auto-loads any downloader plugin found there with no explicit `helm plugin
// install` step - reaching arbitrary command execution when a helmCharts entry's repo uses that
// plugin's registered protocol scheme. The helm wrapper installed by withHelmWrapper must make
// Helm ignore the kustomization's chosen configHome, so the planted plugin's downloader command
// is never invoked, regardless of whether the content is trusted or not.
func TestKustomizeBuildHelmChartsIgnoresConfigHomePlugin(t *testing.T) {
	appPath := t.TempDir()

	// A marker file the plugin's downloader would create if it ever ran.
	marker := filepath.Join(appPath, "pwned")
	pluginDir := filepath.Join(appPath, "helm-home", ".data", "plugins", "probe")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "run.sh"), []byte(fmt.Sprintf("#!/bin/sh\ntouch %q\n", marker)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(fmt.Sprintf(`name: probe
version: 1.0.0
downloaders:
  - command: /bin/sh %s
    protocols:
      - probe
`, filepath.Join(pluginDir, "run.sh"))), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(appPath, "kustomization.yaml"), []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
helmGlobals:
  configHome: helm-home
helmCharts:
  - name: probe
    version: 1.0.0
    repo: probe://chart
`), 0o644))

	k := NewKustomizeApp(appPath, appPath, git.NopCreds{}, "", "", "", "")
	_, _, _, err := k.Build(nil, &v1alpha1.KustomizeOptions{BuildOptions: "--enable-helm"}, nil, &BuildOpts{})
	require.Error(t, err, "the \"probe\" protocol is only known via the planted plugin, which the wrapper must have hidden from helm")
	_, statErr := os.Stat(marker)
	assert.True(t, os.IsNotExist(statErr), "the plugin's downloader command must never have been executed")
}

// TestKustomizeBuildHelmChartsFromRealRepoStillWorksWithWrapper proves the helm wrapper installed
// by withHelmWrapper doesn't regress ordinary --enable-helm chart inflation, which needs no
// plugin at all.
func TestKustomizeBuildHelmChartsFromRealRepoStillWorksWithWrapper(t *testing.T) {
	appPath := t.TempDir()
	chartDir := filepath.Join(appPath, "helm-chart")
	require.NoError(t, os.MkdirAll(filepath.Join(chartDir, "templates"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(appPath, "kustomization.yaml"), []byte(`helmGlobals:
  chartHome: .

helmCharts:
- releaseName: test
  name: helm-chart
  version: v1.0.0
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("version: 1.0.0\nname: helm-chart\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "values.yaml"), nil, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "templates", "config-map.yaml"), []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: my-map
data:
  foo: bar
`), 0o644))

	k := NewKustomizeApp(appPath, appPath, git.NopCreds{}, "", "", "", "")
	objs, _, _, err := k.Build(nil, &v1alpha1.KustomizeOptions{BuildOptions: "--enable-helm"}, nil, &BuildOpts{})
	require.NoError(t, err)
	assert.Len(t, objs, 1)
}

func Test_getImageParameters_sorted(t *testing.T) {
	apps := []*unstructured.Unstructured{
		{
			Object: map[string]any{
				"kind": "Deployment",
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name":  "nginx",
									"image": "nginx:1.15.6",
								},
							},
						},
					},
				},
			},
		},
		{
			Object: map[string]any{
				"kind": "Deployment",
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name":  "nginx",
									"image": "nginx:1.15.5",
								},
							},
						},
					},
				},
			},
		},
	}
	params := getImageParameters(apps)
	assert.Equal(t, []string{"nginx:1.15.5", "nginx:1.15.6"}, params)
}
