package exec

import (
	"os"
	"os/exec"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_timeout(t *testing.T) {
	defer func() { _ = os.Unsetenv("ARGOCD_EXEC_TIMEOUT") }()
	t.Run("Default", func(t *testing.T) {
		initTimeout()
		assert.Equal(t, 90*time.Second, timeout)
	})
	t.Run("Default", func(t *testing.T) {
		_ = os.Setenv("ARGOCD_EXEC_TIMEOUT", "1s")
		initTimeout()
		assert.Equal(t, 1*time.Second, timeout)
	})
}

func TestRun(t *testing.T) {
	out, err := Run(exec.Command("ls"))
	assert.NoError(t, err)
	assert.NotEmpty(t, out)
}

func TestHideUsernamePassword(t *testing.T) {
	_, err := RunWithRedactor(exec.Command("helm registry login https://charts.bitnami.com/bitnami", "--username", "foo", "--password", "bar"), nil)
	assert.NotEmpty(t, err)

	var redactor = func(text string) string {
		return regexp.MustCompile("(--username|--password) [^ ]*").ReplaceAllString(text, "$1 ******")
	}
	_, err = RunWithRedactor(exec.Command("helm registry login https://charts.bitnami.com/bitnami", "--username", "foo", "--password", "bar"), redactor)
	assert.NotEmpty(t, err)
}
