package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExecTimeout(t *testing.T) {
	defer func() { _ = os.Unsetenv("ARGOCD_EXEC_TIMEOUT") }()
	tests := []struct {
		name string
		text string
		want time.Duration
	}{
		{"Empty", "", time.Duration(0)},
		{"OneSecond", "1s", 1 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Setenv("ARGOCD_EXEC_TIMEOUT", tt.text)
			assert.Equal(t, tt.want, ExecTimeout())
		})
	}
}
