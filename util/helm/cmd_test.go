package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_cmd_redactor(t *testing.T) {
	assert.Equal(t, "--foo bar", redactor("--foo bar"))
	assert.Equal(t, "--username ******", redactor("--username bar"))
	assert.Equal(t, "--password ******", redactor("--password bar"))
}

func TestCmd_template_kubeVersion(t *testing.T) {
	cmd, err := NewCmd(".")
	assert.NoError(t, err)
	s, err := cmd.template("testdata/redis", &TemplateOpts{
		KubeVersion: "1.14",
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, s)
}
