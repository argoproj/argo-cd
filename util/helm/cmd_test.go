package helm

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_cmd_redactor(t *testing.T) {
	assert.Equal(t, "--foo bar", redactor("--foo bar"))
	assert.Equal(t, "--username ******", redactor("--username bar"))
	assert.Equal(t, "--password ******", redactor("--password bar"))
}

func TestCmd_template_kubeVersion(t *testing.T) {
	cmd, err := NewCmdWithVersion(".", false, "", "")
	require.NoError(t, err)
	s, _, err := cmd.template("testdata/redis", &TemplateOpts{
		KubeVersion: "1.14",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, s)
}

func TestCmd_template_noApiVersionsInError(t *testing.T) {
	cmd, err := NewCmdWithVersion(".", false, "", "")
	require.NoError(t, err)
	_, _, err = cmd.template("testdata/chart-does-not-exist", &TemplateOpts{
		KubeVersion: "1.14",
		APIVersions: []string{"foo", "bar"},
	})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "--api-version")
	assert.ErrorContains(t, err, "<api versions removed> ")
}

func TestNewCmd_helmInvalidVersion(t *testing.T) {
	_, err := NewCmd(".", "abcd", "", "")
	log.Println(err)
	assert.EqualError(t, err, "helm chart version 'abcd' is not supported")
}

func TestNewCmd_withProxy(t *testing.T) {
	cmd, err := NewCmd(".", "", "https://proxy:8888", ".argoproj.io")
	require.NoError(t, err)
	assert.Equal(t, "https://proxy:8888", cmd.proxy)
	assert.Equal(t, ".argoproj.io", cmd.noProxy)
}
