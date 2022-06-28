package exec

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/common"
)

func TestRun(t *testing.T) {
	out, err := Run(exec.Command("ls"), common.DefaultExecTimeout)
	assert.NoError(t, err)
	assert.NotEmpty(t, out)
}

func TestHideUsernamePassword(t *testing.T) {
	_, err := RunWithRedactor(exec.Command("helm registry login https://charts.bitnami.com/bitnami", "--username", "foo", "--password", "bar"), nil, common.DefaultExecTimeout)
	assert.NotEmpty(t, err)

	var redactor = func(text string) string {
		return regexp.MustCompile("(--username|--password) [^ ]*").ReplaceAllString(text, "$1 ******")
	}
	_, err = RunWithRedactor(exec.Command("helm registry login https://charts.bitnami.com/bitnami", "--username", "foo", "--password", "bar"), redactor, common.DefaultExecTimeout)
	assert.NotEmpty(t, err)
}
