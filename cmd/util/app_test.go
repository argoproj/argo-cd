package util

import (
	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func Test_setHelmOpt(t *testing.T) {
	t.Run("Zero", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setHelmOpt(&src, helmOpts{})
		assert.Nil(t, src.Helm)
	})
	t.Run("ValueFiles", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setHelmOpt(&src, helmOpts{valueFiles: []string{"foo"}})
		assert.Equal(t, []string{"foo"}, src.Helm.ValueFiles)
	})
	t.Run("IgnoreMissingValueFiles", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setHelmOpt(&src, helmOpts{ignoreMissingValueFiles: true})
		assert.Equal(t, true, src.Helm.IgnoreMissingValueFiles)
	})
	t.Run("ReleaseName", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setHelmOpt(&src, helmOpts{releaseName: "foo"})
		assert.Equal(t, "foo", src.Helm.ReleaseName)
	})
	t.Run("HelmSets", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setHelmOpt(&src, helmOpts{helmSets: []string{"foo=bar"}})
		assert.Equal(t, []v1alpha1.HelmParameter{{Name: "foo", Value: "bar"}}, src.Helm.Parameters)
	})
	t.Run("HelmSetStrings", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setHelmOpt(&src, helmOpts{helmSetStrings: []string{"foo=bar"}})
		assert.Equal(t, []v1alpha1.HelmParameter{{Name: "foo", Value: "bar", ForceString: true}}, src.Helm.Parameters)
	})
	t.Run("HelmSetFiles", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setHelmOpt(&src, helmOpts{helmSetFiles: []string{"foo=bar"}})
		assert.Equal(t, []v1alpha1.HelmFileParameter{{Name: "foo", Path: "bar"}}, src.Helm.FileParameters)
	})
	t.Run("Version", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setHelmOpt(&src, helmOpts{version: "v3"})
		assert.Equal(t, "v3", src.Helm.Version)
	})
	t.Run("HelmPassCredentials", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setHelmOpt(&src, helmOpts{passCredentials: true})
		assert.Equal(t, true, src.Helm.PassCredentials)
	})
	t.Run("HelmSkipCrds", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setHelmOpt(&src, helmOpts{skipCrds: true})
		assert.Equal(t, true, src.Helm.SkipCrds)
	})
}

func Test_setKustomizeOpt(t *testing.T) {
	t.Run("No kustomize", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setKustomizeOpt(&src, kustomizeOpts{})
		assert.Nil(t, src.Kustomize)
	})
	t.Run("Name prefix", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setKustomizeOpt(&src, kustomizeOpts{namePrefix: "test-"})
		assert.Equal(t, &v1alpha1.ApplicationSourceKustomize{NamePrefix: "test-"}, src.Kustomize)
	})
	t.Run("Name suffix", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setKustomizeOpt(&src, kustomizeOpts{nameSuffix: "-test"})
		assert.Equal(t, &v1alpha1.ApplicationSourceKustomize{NameSuffix: "-test"}, src.Kustomize)
	})
	t.Run("Images", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setKustomizeOpt(&src, kustomizeOpts{images: []string{"org/image:v1", "org/image:v2"}})
		assert.Equal(t, &v1alpha1.ApplicationSourceKustomize{Images: v1alpha1.KustomizeImages{v1alpha1.KustomizeImage("org/image:v2")}}, src.Kustomize)
	})
	t.Run("Version", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setKustomizeOpt(&src, kustomizeOpts{version: "v0.1"})
		assert.Equal(t, &v1alpha1.ApplicationSourceKustomize{Version: "v0.1"}, src.Kustomize)
	})
	t.Run("Common labels", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setKustomizeOpt(&src, kustomizeOpts{commonLabels: map[string]string{"foo1": "bar1", "foo2": "bar2"}})
		assert.Equal(t, &v1alpha1.ApplicationSourceKustomize{CommonLabels: map[string]string{"foo1": "bar1", "foo2": "bar2"}}, src.Kustomize)
	})
	t.Run("Common annotations", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setKustomizeOpt(&src, kustomizeOpts{commonAnnotations: map[string]string{"foo1": "bar1", "foo2": "bar2"}})
		assert.Equal(t, &v1alpha1.ApplicationSourceKustomize{CommonAnnotations: map[string]string{"foo1": "bar1", "foo2": "bar2"}}, src.Kustomize)
	})
}

func Test_setJsonnetOpt(t *testing.T) {
	t.Run("TlaSets", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setJsonnetOpt(&src, []string{"foo=bar"}, false)
		assert.Equal(t, []v1alpha1.JsonnetVar{{Name: "foo", Value: "bar"}}, src.Directory.Jsonnet.TLAs)
		setJsonnetOpt(&src, []string{"bar=baz"}, false)
		assert.Equal(t, []v1alpha1.JsonnetVar{{Name: "foo", Value: "bar"}, {Name: "bar", Value: "baz"}}, src.Directory.Jsonnet.TLAs)
	})
	t.Run("ExtSets", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setJsonnetOptExtVar(&src, []string{"foo=bar"}, false)
		assert.Equal(t, []v1alpha1.JsonnetVar{{Name: "foo", Value: "bar"}}, src.Directory.Jsonnet.ExtVars)
		setJsonnetOptExtVar(&src, []string{"bar=baz"}, false)
		assert.Equal(t, []v1alpha1.JsonnetVar{{Name: "foo", Value: "bar"}, {Name: "bar", Value: "baz"}}, src.Directory.Jsonnet.ExtVars)
	})
}

func Test_setPluginOptEnvs(t *testing.T) {
	t.Run("PluginEnvs", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setPluginOptEnvs(&src, []string{"FOO=bar"})
		assert.Equal(t, v1alpha1.EnvEntry{Name: "FOO", Value: "bar"}, *src.Plugin.Env[0])
		setPluginOptEnvs(&src, []string{"BAR=baz"})
		assert.Equal(t, v1alpha1.EnvEntry{Name: "BAR", Value: "baz"}, *src.Plugin.Env[1])
		setPluginOptEnvs(&src, []string{"FOO=baz"})
		assert.Equal(t, v1alpha1.EnvEntry{Name: "FOO", Value: "baz"}, *src.Plugin.Env[0])
	})
}

type appOptionsFixture struct {
	spec    *v1alpha1.ApplicationSpec
	command *cobra.Command
	options *AppOptions
}

func (f *appOptionsFixture) SetFlag(key, value string) error {
	err := f.command.Flags().Set(key, value)
	if err != nil {
		return err
	}
	_ = SetAppSpecOptions(f.command.Flags(), f.spec, f.options)
	return err
}

func newAppOptionsFixture() *appOptionsFixture {
	fixture := &appOptionsFixture{
		spec:    &v1alpha1.ApplicationSpec{},
		command: &cobra.Command{},
		options: &AppOptions{},
	}
	AddAppFlags(fixture.command, fixture.options)
	return fixture
}

func Test_setAppSpecOptions(t *testing.T) {
	f := newAppOptionsFixture()
	t.Run("SyncPolicy", func(t *testing.T) {
		assert.NoError(t, f.SetFlag("sync-policy", "automated"))
		assert.NotNil(t, f.spec.SyncPolicy.Automated)

		f.spec.SyncPolicy = nil
		assert.NoError(t, f.SetFlag("sync-policy", "automatic"))
		assert.NotNil(t, f.spec.SyncPolicy.Automated)

		f.spec.SyncPolicy = nil
		assert.NoError(t, f.SetFlag("sync-policy", "auto"))
		assert.NotNil(t, f.spec.SyncPolicy.Automated)

		assert.NoError(t, f.SetFlag("sync-policy", "none"))
		assert.Nil(t, f.spec.SyncPolicy)
	})
	t.Run("SyncOptions", func(t *testing.T) {
		assert.NoError(t, f.SetFlag("sync-option", "a=1"))
		assert.True(t, f.spec.SyncPolicy.SyncOptions.HasOption("a=1"))

		// remove the options using !
		assert.NoError(t, f.SetFlag("sync-option", "!a=1"))
		assert.Nil(t, f.spec.SyncPolicy)
	})
	t.Run("RetryLimit", func(t *testing.T) {
		assert.NoError(t, f.SetFlag("sync-retry-limit", "5"))
		assert.True(t, f.spec.SyncPolicy.Retry.Limit == 5)

		assert.NoError(t, f.SetFlag("sync-retry-limit", "0"))
		assert.Nil(t, f.spec.SyncPolicy.Retry)
	})
}

var configMapUrl = "https://raw.githubusercontent.com/argoproj/argo-cd/db547567b9eafcba44e3df2883a40b8d03c71bf0/manifests/base/config/argocd-cm.yaml"
var congigMapYaml = `apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  labels:
    app.kubernetes.io/name: argocd-cm
    app.kubernetes.io/part-of: argocd
`

func Test_setAppSpecOptions_valuesLiteralFile(t *testing.T) {
	testCases := []struct{
		name string
		sourceUrl string
		expectedYaml string
	}{
		{
			name: "valid yaml",
			sourceUrl: writeTempFile(t, "some: yaml"),
			expectedYaml: "some: yaml",
		},
		{
			name: "invalid yaml",
			sourceUrl: writeTempFile(t, "{some invalid yaml"),
			expectedYaml: "{some invalid yaml",
		},
		{
			name:         "ConfigMap from URL",
			sourceUrl:    configMapUrl,
			expectedYaml: congigMapYaml,
		},
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			f := newAppOptionsFixture()
			assert.NoError(t, f.SetFlag("values-literal-file", testCaseCopy.sourceUrl))
			assert.Equal(t, testCaseCopy.expectedYaml, string(f.spec.Source.Helm.Values.YAML()))
		})
	}
}

func Test_setAppSpecOptions_valuesRawLiteralFile(t *testing.T) {
	// Unmarshal and re-marshal so it's sorted.
	var unmarshalledYaml interface{}
	err := yaml.Unmarshal([]byte(congigMapYaml), &unmarshalledYaml)
	require.NoError(t, err)
	marshalledYaml, err := yaml.Marshal(unmarshalledYaml)
	require.NoError(t, err)

	testCases := []struct{
		name string
		sourceUrl string
		expectedYaml string
	}{
		{
			name: "valid yaml",
			sourceUrl: writeTempFile(t, "some: yaml"),
			expectedYaml: "some: yaml\n",
		},
		{
			name:         "ConfigMap from URL",
			sourceUrl:    configMapUrl,
			expectedYaml: string(marshalledYaml),
		},
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			f := newAppOptionsFixture()
			assert.NoError(t, f.SetFlag("values-raw-literal-file", testCaseCopy.sourceUrl))
			assert.Equal(t, testCaseCopy.expectedYaml, string(f.spec.Source.Helm.Values.YAML()))
		})
	}
}

func writeTempFile(t *testing.T, contents string) string {
	tempYaml, err := ioutil.TempFile(t.TempDir(), "")
	require.NoError(t, err)
	_, err = tempYaml.WriteString(contents)
	require.NoError(t, err)
	err = tempYaml.Sync()
	require.NoError(t, err)
	err = tempYaml.Close()
	require.NoError(t, err)
	return tempYaml.Name()
}

func Test_setAnnotations(t *testing.T) {
	t.Run("Annotations", func(t *testing.T) {
		app := v1alpha1.Application{}
		setAnnotations(&app, []string{"hoge=foo", "huga=bar"})
		assert.Equal(t, map[string]string{"hoge": "foo", "huga": "bar"}, app.Annotations)
	})
	t.Run("Annotations value contains equal", func(t *testing.T) {
		app := v1alpha1.Application{}
		setAnnotations(&app, []string{"hoge=foo=bar"})
		assert.Equal(t, map[string]string{"hoge": "foo=bar"}, app.Annotations)
	})
	t.Run("Annotations empty value", func(t *testing.T) {
		app := v1alpha1.Application{}
		setAnnotations(&app, []string{"hoge"})
		assert.Equal(t, map[string]string{"hoge": ""}, app.Annotations)
	})
}

const appsYaml = `---
# Source: apps/templates/helm.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: sth1
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  destination:
    namespace: sth
    server: 'https://kubernetes.default.svc'
  project: default
  source:
    repoURL: 'https://github.com/pasha-codefresh/argocd-example-apps'
    targetRevision: HEAD
    path: apps
    helm:
      valueFiles:
        - values.yaml
---
# Source: apps/templates/helm.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: sth2
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  destination:
    namespace: sth
    server: 'https://kubernetes.default.svc'
  project: default
  source:
    repoURL: 'https://github.com/pasha-codefresh/argocd-example-apps'
    targetRevision: HEAD
    path: apps
    helm:
      valueFiles:
        - values.yaml`

func TestReadAppsFromURI(t *testing.T) {
	file, err := ioutil.TempFile(os.TempDir(), "")
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = os.Remove(file.Name())
	}()

	_, _ = file.WriteString(appsYaml)
	_ = file.Sync()

	apps := make([]*argoappv1.Application, 0)
	err = readAppsFromURI(file.Name(), &apps)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(apps))

	assert.Equal(t, "sth1", apps[0].Name)
	assert.Equal(t, "sth2", apps[1].Name)

}

func TestConstructAppFromStdin(t *testing.T) {
	file, err := ioutil.TempFile(os.TempDir(), "")
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = os.Remove(file.Name())
	}()

	_, _ = file.WriteString(appsYaml)
	_ = file.Sync()

	if _, err := file.Seek(0, 0); err != nil {
		log.Fatal(err)
	}

	os.Stdin = file

	apps, err := ConstructApps("-", "test", []string{}, []string{}, []string{}, AppOptions{}, nil)

	if err := file.Close(); err != nil {
		log.Fatal(err)
	}
	assert.NoError(t, err)
	assert.Equal(t, 2, len(apps))
	assert.Equal(t, "sth1", apps[0].Name)
	assert.Equal(t, "sth2", apps[1].Name)

}

func TestConstructBasedOnName(t *testing.T) {
	apps, err := ConstructApps("", "test", []string{}, []string{}, []string{}, AppOptions{}, nil)

	assert.NoError(t, err)
	assert.Equal(t, 1, len(apps))
	assert.Equal(t, "test", apps[0].Name)
}
