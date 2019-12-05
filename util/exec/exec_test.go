package exec

import (
	"os"
	"os/exec"
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
