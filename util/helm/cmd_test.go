package helm

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_cmd_redactor(t *testing.T) {
	assert.Equal(t, "--foo bar", redactor("--foo bar"))
	assert.Equal(t, "--username ******", redactor("--username bar"))
	assert.Equal(t, "--password ******", redactor("--password bar"))
}

func TestCmd_template_kubeVersion(t *testing.T) {
	cmd, err := NewCmdWithVersion(".", HelmV3, false, "")
	assert.NoError(t, err)
	s, err := cmd.template("testdata/redis", &TemplateOpts{
		KubeVersion: "1.14",
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, s)
}

func TestCmd_template_noApiVersionsInError(t *testing.T) {
	cmd, err := NewCmdWithVersion(".", HelmV3, false, "")
	assert.NoError(t, err)
	_, err = cmd.template("testdata/chart-does-not-exist", &TemplateOpts{
		KubeVersion: "1.14",
		APIVersions: []string{"foo", "bar"},
	})
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), "--api-version")
	assert.ErrorContains(t, err, "<api versions removed> ")
}

func TestNewCmd_helmV3(t *testing.T) {
	cmd, err := NewCmd(".", "v3", "")
	assert.NoError(t, err)
	assert.Equal(t, "helm", cmd.HelmVer.binaryName)
}

func TestNewCmd_helmDefaultVersion(t *testing.T) {
	cmd, err := NewCmd(".", "", "")
	assert.NoError(t, err)
	assert.Equal(t, "helm", cmd.HelmVer.binaryName)
}

func TestNewCmd_helmInvalidVersion(t *testing.T) {
	_, err := NewCmd(".", "abcd", "")
	log.Println(err)
	assert.EqualError(t, err, "helm chart version 'abcd' is not supported")
}

func TestNewCmd_withProxy(t *testing.T) {
	cmd, err := NewCmd(".", "", "https://proxy:8888")
	assert.NoError(t, err)
	assert.Equal(t, "https://proxy:8888", cmd.proxy)
}
