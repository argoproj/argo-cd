package config

import (
	"os"
	"testing"
	"time"

	"github.com/argoproj/pkg/exec"
	"github.com/stretchr/testify/assert"
)

func Test_timeout(t *testing.T) {
	defer func() { _ = os.Unsetenv("ARGOCD_EXEC_TIMEOUT") }()
	tests := []struct {
		name string
		text string
		want time.Duration
	}{
		{"Default", "", 90 * time.Second},
		{"OneSecond", "1s", 1 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Setenv("ARGOCD_EXEC_TIMEOUT", tt.text)
			initTimeout()
			assert.Equal(t, tt.want, timeout)
		})
	}
}

func TestCmdOpts(t *testing.T) {
	initTimeout()
	assert.Equal(t, exec.CmdOpts{Timeout: 90 * time.Second}, CmdOpts())
}
