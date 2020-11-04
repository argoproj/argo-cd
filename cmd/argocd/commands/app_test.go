package commands

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
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

type appOptionsFixture struct {
	spec    *v1alpha1.ApplicationSpec
	command *cobra.Command
	options *appOptions
}

func (f *appOptionsFixture) SetFlag(key, value string) error {
	err := f.command.Flags().Set(key, value)
	if err != nil {
		return err
	}
	_ = setAppSpecOptions(f.command.Flags(), f.spec, f.options)
	return err
}

func newAppOptionsFixture() *appOptionsFixture {
	fixture := &appOptionsFixture{
		spec:    &v1alpha1.ApplicationSpec{},
		command: &cobra.Command{},
		options: &appOptions{},
	}
	addAppFlags(fixture.command, fixture.options)
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
}
