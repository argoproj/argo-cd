package util

import (
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	"k8s.io/apimachinery/pkg/util/intstr"
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
		assert.True(t, src.Helm.IgnoreMissingValueFiles)
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
		assert.True(t, src.Helm.PassCredentials)
	})
	t.Run("HelmSkipCrds", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setHelmOpt(&src, helmOpts{skipCrds: true})
		assert.True(t, src.Helm.SkipCrds)
	})
	t.Run("HelmNamespace", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setHelmOpt(&src, helmOpts{namespace: "custom-namespace"})
		assert.Equal(t, "custom-namespace", src.Helm.Namespace)
	})
	t.Run("HelmKubeVersion", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setHelmOpt(&src, helmOpts{kubeVersion: "v1.16.0"})
		assert.Equal(t, "v1.16.0", src.Helm.KubeVersion)
	})
	t.Run("HelmApiVersions", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setHelmOpt(&src, helmOpts{apiVersions: []string{"v1", "v2"}})
		assert.Equal(t, []string{"v1", "v2"}, src.Helm.APIVersions)
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
	t.Run("Replicas", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		testReplicasString := []string{"my-deployment=2", "my-statefulset=4"}
		testReplicas := v1alpha1.KustomizeReplicas{
			{
				Name:  "my-deployment",
				Count: intstr.FromInt(2),
			},
			{
				Name:  "my-statefulset",
				Count: intstr.FromInt(4),
			},
		}
		setKustomizeOpt(&src, kustomizeOpts{replicas: testReplicasString})
		assert.Equal(t, &v1alpha1.ApplicationSourceKustomize{Replicas: testReplicas}, src.Kustomize)
	})
	t.Run("Version", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setKustomizeOpt(&src, kustomizeOpts{version: "v0.1"})
		assert.Equal(t, &v1alpha1.ApplicationSourceKustomize{Version: "v0.1"}, src.Kustomize)
	})
	t.Run("Namespace", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setKustomizeOpt(&src, kustomizeOpts{namespace: "custom-namespace"})
		assert.Equal(t, &v1alpha1.ApplicationSourceKustomize{Namespace: "custom-namespace"}, src.Kustomize)
	})
	t.Run("KubeVersion", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setKustomizeOpt(&src, kustomizeOpts{kubeVersion: "999.999.999"})
		assert.Equal(t, &v1alpha1.ApplicationSourceKustomize{KubeVersion: "999.999.999"}, src.Kustomize)
	})
	t.Run("ApiVersions", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setKustomizeOpt(&src, kustomizeOpts{apiVersions: []string{"v1", "v2"}})
		assert.Equal(t, &v1alpha1.ApplicationSourceKustomize{APIVersions: []string{"v1", "v2"}}, src.Kustomize)
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
	t.Run("Label Without Selector", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setKustomizeOpt(&src, kustomizeOpts{commonLabels: map[string]string{"foo1": "bar1", "foo2": "bar2"}, labelWithoutSelector: true})
		assert.Equal(t, &v1alpha1.ApplicationSourceKustomize{CommonLabels: map[string]string{"foo1": "bar1", "foo2": "bar2"}, LabelWithoutSelector: true}, src.Kustomize)
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
	_ = SetAppSpecOptions(f.command.Flags(), f.spec, f.options, 0)
	return err
}

func (f *appOptionsFixture) SetFlagWithSourcePosition(key, value string, sourcePosition int) error {
	err := f.command.Flags().Set(key, value)
	if err != nil {
		return err
	}
	_ = SetAppSpecOptions(f.command.Flags(), f.spec, f.options, sourcePosition)
	return err
}

func newAppOptionsFixture() *appOptionsFixture {
	fixture := &appOptionsFixture{
		spec: &v1alpha1.ApplicationSpec{
			Source: &v1alpha1.ApplicationSource{},
		},
		command: &cobra.Command{},
		options: &AppOptions{},
	}
	AddAppFlags(fixture.command, fixture.options)
	return fixture
}

func Test_setAppSpecOptions(t *testing.T) {
	f := newAppOptionsFixture()
	t.Run("SyncPolicy", func(t *testing.T) {
		require.NoError(t, f.SetFlag("sync-policy", "automated"))
		assert.NotNil(t, f.spec.SyncPolicy.Automated)

		f.spec.SyncPolicy = nil
		require.NoError(t, f.SetFlag("sync-policy", "automatic"))
		assert.NotNil(t, f.spec.SyncPolicy.Automated)

		f.spec.SyncPolicy = nil
		require.NoError(t, f.SetFlag("sync-policy", "auto"))
		assert.NotNil(t, f.spec.SyncPolicy.Automated)

		require.NoError(t, f.SetFlag("sync-policy", "none"))
		assert.Nil(t, f.spec.SyncPolicy)
	})
	t.Run("SyncOptions", func(t *testing.T) {
		require.NoError(t, f.SetFlag("sync-option", "a=1"))
		assert.True(t, f.spec.SyncPolicy.SyncOptions.HasOption("a=1"))

		// remove the options using !
		require.NoError(t, f.SetFlag("sync-option", "!a=1"))
		assert.Nil(t, f.spec.SyncPolicy)
	})
	t.Run("RetryLimit", func(t *testing.T) {
		require.NoError(t, f.SetFlag("sync-retry-limit", "5"))
		assert.Equal(t, int64(5), f.spec.SyncPolicy.Retry.Limit)

		require.NoError(t, f.SetFlag("sync-retry-limit", "0"))
		assert.Nil(t, f.spec.SyncPolicy.Retry)
	})
	t.Run("Kustomize", func(t *testing.T) {
		require.NoError(t, f.SetFlag("kustomize-replica", "my-deployment=2"))
		require.NoError(t, f.SetFlag("kustomize-replica", "my-statefulset=4"))
		assert.Equal(t, v1alpha1.KustomizeReplicas{{Name: "my-deployment", Count: intstr.FromInt(2)}, {Name: "my-statefulset", Count: intstr.FromInt(4)}}, f.spec.Source.Kustomize.Replicas)
	})
	t.Run("Kustomize Namespace", func(t *testing.T) {
		require.NoError(t, f.SetFlag("kustomize-namespace", "override-namespace"))
		assert.Equal(t, "override-namespace", f.spec.Source.Kustomize.Namespace)
	})
	t.Run("Kustomize Kube Version", func(t *testing.T) {
		require.NoError(t, f.SetFlag("kustomize-kube-version", "999.999.999"))
		assert.Equal(t, "999.999.999", f.spec.Source.Kustomize.KubeVersion)
	})
	t.Run("Kustomize API Versions", func(t *testing.T) {
		require.NoError(t, f.SetFlag("kustomize-api-versions", "v1"))
		require.NoError(t, f.SetFlag("kustomize-api-versions", "v2"))
		assert.Equal(t, []string{"v1", "v2"}, f.spec.Source.Kustomize.APIVersions)
	})
	t.Run("Helm Namespace", func(t *testing.T) {
		require.NoError(t, f.SetFlag("helm-namespace", "override-namespace"))
		assert.Equal(t, "override-namespace", f.spec.Source.Helm.Namespace)
	})
	t.Run("Helm Kube Version", func(t *testing.T) {
		require.NoError(t, f.SetFlag("kustomize-kube-version", "999.999.999"))
		assert.Equal(t, "999.999.999", f.spec.Source.Kustomize.KubeVersion)
	})
	t.Run("Helm API Versions", func(t *testing.T) {
		require.NoError(t, f.SetFlag("helm-api-versions", "v1"))
		require.NoError(t, f.SetFlag("helm-api-versions", "v2"))
		assert.Equal(t, []string{"v1", "v2"}, f.spec.Source.Helm.APIVersions)
	})
}

func newMultiSourceAppOptionsFixture() *appOptionsFixture {
	fixture := &appOptionsFixture{
		spec: &v1alpha1.ApplicationSpec{
			Sources: v1alpha1.ApplicationSources{
				v1alpha1.ApplicationSource{},
				v1alpha1.ApplicationSource{},
			},
		},
		command: &cobra.Command{},
		options: &AppOptions{},
	}
	AddAppFlags(fixture.command, fixture.options)
	return fixture
}

func Test_setAppSpecOptionsMultiSourceApp(t *testing.T) {
	f := newMultiSourceAppOptionsFixture()
	sourcePosition := 0
	sourcePosition1 := 1
	sourcePosition2 := 2
	t.Run("SyncPolicy", func(t *testing.T) {
		require.NoError(t, f.SetFlagWithSourcePosition("sync-policy", "automated", sourcePosition1))
		assert.NotNil(t, f.spec.SyncPolicy.Automated)

		f.spec.SyncPolicy = nil
		require.NoError(t, f.SetFlagWithSourcePosition("sync-policy", "automatic", sourcePosition1))
		assert.NotNil(t, f.spec.SyncPolicy.Automated)
	})
	t.Run("Helm - SourcePosition 0", func(t *testing.T) {
		require.NoError(t, f.SetFlagWithSourcePosition("helm-version", "v2", sourcePosition))
		assert.Len(t, f.spec.GetSources(), 2)
		assert.Equal(t, "v2", f.spec.GetSources()[sourcePosition].Helm.Version)
	})
	t.Run("Kustomize", func(t *testing.T) {
		require.NoError(t, f.SetFlagWithSourcePosition("kustomize-replica", "my-deployment=2", sourcePosition1))
		assert.Equal(t, v1alpha1.KustomizeReplicas{{Name: "my-deployment", Count: intstr.FromInt(2)}}, f.spec.Sources[sourcePosition1-1].Kustomize.Replicas)
		require.NoError(t, f.SetFlagWithSourcePosition("kustomize-replica", "my-deployment=4", sourcePosition2))
		assert.Equal(t, v1alpha1.KustomizeReplicas{{Name: "my-deployment", Count: intstr.FromInt(4)}}, f.spec.Sources[sourcePosition2-1].Kustomize.Replicas)
	})
	t.Run("Helm", func(t *testing.T) {
		require.NoError(t, f.SetFlagWithSourcePosition("helm-version", "v2", sourcePosition1))
		require.NoError(t, f.SetFlagWithSourcePosition("helm-version", "v3", sourcePosition2))
		assert.Len(t, f.spec.GetSources(), 2)
		assert.Equal(t, "v2", f.spec.GetSources()[sourcePosition1-1].Helm.Version)
		assert.Equal(t, "v3", f.spec.GetSources()[sourcePosition2-1].Helm.Version)
	})
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
	file, err := os.CreateTemp(os.TempDir(), "")
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = os.Remove(file.Name())
	}()

	_, _ = file.WriteString(appsYaml)
	_ = file.Sync()

	apps := make([]*v1alpha1.Application, 0)
	err = readAppsFromURI(file.Name(), &apps)
	require.NoError(t, err)
	assert.Len(t, apps, 2)

	assert.Equal(t, "sth1", apps[0].Name)
	assert.Equal(t, "sth2", apps[1].Name)
}

func TestConstructAppFromStdin(t *testing.T) {
	file, err := os.CreateTemp(os.TempDir(), "")
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
	require.NoError(t, err)
	assert.Len(t, apps, 2)
	assert.Equal(t, "sth1", apps[0].Name)
	assert.Equal(t, "sth2", apps[1].Name)
}

func TestConstructBasedOnName(t *testing.T) {
	apps, err := ConstructApps("", "test", []string{}, []string{}, []string{}, AppOptions{}, nil)

	require.NoError(t, err)
	assert.Len(t, apps, 1)
	assert.Equal(t, "test", apps[0].Name)
}

func TestFilterResources(t *testing.T) {
	t.Run("Filter by ns", func(t *testing.T) {
		resources := []*v1alpha1.ResourceDiff{
			{
				LiveState: "{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"name\":\"test-helm-guestbook\",\"namespace\":\"argocd\"},\"spec\":{\"selector\":{\"app\":\"helm-guestbook\",\"release\":\"test\"},\"sessionAffinity\":\"None\",\"type\":\"ClusterIP\"},\"status\":{\"loadBalancer\":{}}}",
			},
			{
				LiveState: "{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"name\":\"test-helm-guestbook\",\"namespace\":\"ns\"},\"spec\":{\"selector\":{\"app\":\"helm-guestbook\",\"release\":\"test\"},\"sessionAffinity\":\"None\",\"type\":\"ClusterIP\"},\"status\":{\"loadBalancer\":{}}}",
			},
		}

		filteredResources, err := FilterResources(false, resources, "g", "Service", "ns", "test-helm-guestbook", true)
		require.NoError(t, err)
		assert.Len(t, filteredResources, 1)
	})

	t.Run("Filter by kind", func(t *testing.T) {
		resources := []*v1alpha1.ResourceDiff{
			{
				LiveState: "{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"name\":\"test-helm-guestbook\",\"namespace\":\"argocd\"},\"spec\":{\"selector\":{\"app\":\"helm-guestbook\",\"release\":\"test\"},\"sessionAffinity\":\"None\",\"type\":\"ClusterIP\"},\"status\":{\"loadBalancer\":{}}}",
			},
			{
				LiveState: "{\"apiVersion\":\"v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"test-helm-guestbook\",\"namespace\":\"argocd\"},\"spec\":{\"selector\":{\"app\":\"helm-guestbook\",\"release\":\"test\"},\"sessionAffinity\":\"None\",\"type\":\"ClusterIP\"},\"status\":{\"loadBalancer\":{}}}",
			},
		}

		filteredResources, err := FilterResources(false, resources, "g", "Deployment", "argocd", "test-helm-guestbook", true)
		require.NoError(t, err)
		assert.Len(t, filteredResources, 1)
	})

	t.Run("Filter by name", func(t *testing.T) {
		resources := []*v1alpha1.ResourceDiff{
			{
				LiveState: "{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"name\":\"test-helm-guestbook\",\"namespace\":\"argocd\"},\"spec\":{\"selector\":{\"app\":\"helm-guestbook\",\"release\":\"test\"},\"sessionAffinity\":\"None\",\"type\":\"ClusterIP\"},\"status\":{\"loadBalancer\":{}}}",
			},
			{
				LiveState: "{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"name\":\"test-helm\",\"namespace\":\"argocd\"},\"spec\":{\"selector\":{\"app\":\"helm-guestbook\",\"release\":\"test\"},\"sessionAffinity\":\"None\",\"type\":\"ClusterIP\"},\"status\":{\"loadBalancer\":{}}}",
			},
		}

		filteredResources, err := FilterResources(false, resources, "g", "Service", "argocd", "test-helm", true)
		require.NoError(t, err)
		assert.Len(t, filteredResources, 1)
	})

	t.Run("Filter no result", func(t *testing.T) {
		resources := []*v1alpha1.ResourceDiff{
			{
				LiveState: "{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"name\":\"test-helm-guestbook\",\"namespace\":\"argocd\"},\"spec\":{\"selector\":{\"app\":\"helm-guestbook\",\"release\":\"test\"},\"sessionAffinity\":\"None\",\"type\":\"ClusterIP\"},\"status\":{\"loadBalancer\":{}}}",
			},
			{
				LiveState: "{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"name\":\"test-helm\",\"namespace\":\"argocd\"},\"spec\":{\"selector\":{\"app\":\"helm-guestbook\",\"release\":\"test\"},\"sessionAffinity\":\"None\",\"type\":\"ClusterIP\"},\"status\":{\"loadBalancer\":{}}}",
			},
		}

		filteredResources, err := FilterResources(false, resources, "g", "Service", "argocd-unknown", "test-helm", true)
		require.ErrorContains(t, err, "No matching resource found")
		assert.Nil(t, filteredResources)
	})

	t.Run("Filter multiple results", func(t *testing.T) {
		resources := []*v1alpha1.ResourceDiff{
			{
				LiveState: "{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"name\":\"test-helm\",\"namespace\":\"argocd\"},\"spec\":{\"selector\":{\"app\":\"helm-guestbook\",\"release\":\"test\"},\"sessionAffinity\":\"None\",\"type\":\"ClusterIP\"},\"status\":{\"loadBalancer\":{}}}",
			},
			{
				LiveState: "{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"name\":\"test-helm\",\"namespace\":\"argocd\"},\"spec\":{\"selector\":{\"app\":\"helm-guestbook\",\"release\":\"test\"},\"sessionAffinity\":\"None\",\"type\":\"ClusterIP\"},\"status\":{\"loadBalancer\":{}}}",
			},
		}

		filteredResources, err := FilterResources(false, resources, "g", "Service", "argocd", "test-helm", false)
		require.ErrorContains(t, err, "Use the --all flag")
		assert.Nil(t, filteredResources)
	})
}
