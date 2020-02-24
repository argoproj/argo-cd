package commands

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/util/cert"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/helm"
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

func Test_mergeHelmRepositories(t *testing.T) {
	testCases := []struct {
		name     string
		local    []helm.LocalRepository
		server   []*argoappv1.Repository
		expected []*argoappv1.Repository
	}{
		{name: "LocalPrio",
			local: []helm.LocalRepository{
				helm.LocalRepository{Name: "stable", URL: "https://kubernetes-charts.mirror.io"},
			},
			server: []*argoappv1.Repository{
				&argoappv1.Repository{Type: "helm", Name: "stable", Repo: "https://kubernetes-charts.storage.googleapis.com"},
			},
			expected: []*argoappv1.Repository{
				&argoappv1.Repository{Type: "helm", Name: "stable", Repo: "https://kubernetes-charts.mirror.io"},
			},
		},
		{name: "ExtraFromServer",
			local: []helm.LocalRepository{
				helm.LocalRepository{Name: "stable", URL: "https://kubernetes-charts.storage.googleapis.com"},
			},
			server: []*argoappv1.Repository{
				&argoappv1.Repository{Type: "helm", Name: "incubator", Repo: "http://storage.googleapis.com/kubernetes-charts-incubator"},
			},
			expected: []*argoappv1.Repository{
				&argoappv1.Repository{Type: "helm", Name: "stable", Repo: "https://kubernetes-charts.storage.googleapis.com"},
				&argoappv1.Repository{Type: "helm", Name: "incubator", Repo: "http://storage.googleapis.com/kubernetes-charts-incubator"},
			},
		},
		{name: "NoLocal",
			local: []helm.LocalRepository{},
			server: []*argoappv1.Repository{
				&argoappv1.Repository{Type: "helm", Name: "stable", Repo: "https://kubernetes-charts.storage.googleapis.com"},
			},
			expected: []*argoappv1.Repository{
				&argoappv1.Repository{Type: "helm", Name: "stable", Repo: "https://kubernetes-charts.storage.googleapis.com"},
			},
		},
		{name: "UsernamePassword",
			local: []helm.LocalRepository{
				helm.LocalRepository{Name: "stable", URL: "https://kubernetes-charts.mirror.io", Username: "example", Password: "welcome01"},
			},
			server: []*argoappv1.Repository{},
			expected: []*argoappv1.Repository{
				&argoappv1.Repository{Type: "helm", Name: "stable", Repo: "https://kubernetes-charts.mirror.io", Username: "example", Password: "welcome01"},
			},
		},
		{name: "MissingCAFile",
			local: []helm.LocalRepository{
				helm.LocalRepository{Name: "stable", URL: "https://kubernetes-charts.mirror.io", CAFile: "does-not-exists.crt"},
			},
			server:   []*argoappv1.Repository{},
			expected: []*argoappv1.Repository{},
		},
		{name: "CAFile",
			local: []helm.LocalRepository{
				helm.LocalRepository{Name: "stable", URL: "https://kubernetes-charts.mirror.io", CAFile: "testdata/bundle.crt"},
			},
			server: []*argoappv1.Repository{},
			expected: []*argoappv1.Repository{
				&argoappv1.Repository{Type: "helm", Name: "stable", Repo: "https://kubernetes-charts.mirror.io"},
			},
		},
		{name: "MissingTLSClientCertFile",
			local: []helm.LocalRepository{
				helm.LocalRepository{Name: "stable", URL: "https://kubernetes-charts.mirror.io", CertFile: "testdata/missing_client.crt"},
			},
			server: []*argoappv1.Repository{
				&argoappv1.Repository{Type: "helm", Name: "stable", Repo: "https://kubernetes-charts.storage.googleapis.com"},
			},
			expected: []*argoappv1.Repository{
				&argoappv1.Repository{Type: "helm", Name: "stable", Repo: "https://kubernetes-charts.storage.googleapis.com"},
			},
		},
		{name: "MissingTLSClientKeyFile",
			local: []helm.LocalRepository{
				helm.LocalRepository{Name: "stable", URL: "https://kubernetes-charts.mirror.io", KeyFile: "testdata/missing_client.key"},
			},
			server: []*argoappv1.Repository{
				&argoappv1.Repository{Type: "helm", Name: "stable", Repo: "https://kubernetes-charts.storage.googleapis.com"},
			},
			expected: []*argoappv1.Repository{
				&argoappv1.Repository{Type: "helm", Name: "stable", Repo: "https://kubernetes-charts.storage.googleapis.com"},
			},
		},
		{name: "TLSClientAuth",
			local: []helm.LocalRepository{
				helm.LocalRepository{Name: "stable", URL: "https://kubernetes-charts.mirror.io", CertFile: "testdata/client.crt", KeyFile: "testdata/client.key"},
			},
			server: []*argoappv1.Repository{},
			expected: []*argoappv1.Repository{
				&argoappv1.Repository{Type: "helm", Name: "stable", Repo: "https://kubernetes-charts.mirror.io", TLSClientCertData: "--- CERTIFICATE --- ... --- END CERTIFICATE ---", TLSClientCertKey: "--- PRIVATE KEY --- ... --- END PRIVATE KEY ---"},
			},
		},
	}

	tmp, err := ioutil.TempDir("", "helm")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmp) }()
	os.Setenv(common.EnvVarTLSDataPath, tmp)

	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			actual := mergeHelmRepositories(tc.local, tc.server)
			sort.Slice(actual, func(i, j int) bool {
				return strings.Compare(actual[i].Name, actual[j].Name) < 0
			})
			sort.Slice(tc.expected, func(i, j int) bool {
				return strings.Compare(tc.expected[i].Name, tc.expected[j].Name) < 0
			})
			assert.EqualValues(tt, tc.expected, actual)

			for _, local := range tc.local {
				if strings.HasPrefix(local.CAFile, "testdata/") {
					parsedURL, err := url.Parse(local.URL)
					assert.NoError(t, err)
					certPath := fmt.Sprintf("%s/%s", cert.GetTLSCertificateDataPath(), cert.ServerNameWithoutPort(parsedURL.Hostname()))
					_, err = os.Stat(certPath)
					println(certPath, os.IsNotExist(err))
					assert.False(t, os.IsNotExist(err))
				}
			}
		})
	}
}
