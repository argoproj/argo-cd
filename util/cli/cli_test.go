package cli

import (
	"errors"
	"flag"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v3/common"

	"github.com/stretchr/testify/require"
)

const pwd = "test-password"

func TestPromptPassword_Fallback(t *testing.T) {
	oldStdin := os.Stdin
	defer func() {
		os.Stdin = oldStdin
	}()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	_, err = w.WriteString(pwd + "\n")
	if err != nil {
		t.Fatalf("Failed to write to pipe: %v", err)
	}
	w.Close()

	os.Stdin = r
	password := PromptPassword("")
	require.Equal(t, pwd, password)
}

func TestSetLogFormat(t *testing.T) {
	tests := []struct {
		name          string
		logFormat     string
		expected      string
		expectedFatal bool
	}{
		{
			name:      "log format is set to json",
			logFormat: "json",
			expected:  "json",
		},
		{
			name:      "log format is set to text",
			logFormat: "text",
			expected:  "text",
		},
		{
			name:      "log format is not set",
			logFormat: "text",
			expected:  "text",
		},
		{
			name:          "invalid log format",
			logFormat:     "invalid",
			expected:      "",
			expectedFatal: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectedFatal {
				if os.Getenv("TEST_FATAL") == "1" {
					SetLogFormat(tt.logFormat)
					return
				}
				cmd := exec.Command(os.Args[0], "-test.run="+t.Name())
				cmd.Env = append(os.Environ(), "TEST_FATAL=1")
				err := cmd.Run()
				e := &exec.ExitError{}
				if errors.As(err, &e) {
					return
				}
				t.Fatalf("expected fatal exit for invalid log format")
			} else {
				SetLogFormat(tt.logFormat)
				assert.Equal(t, tt.expected, os.Getenv(common.EnvLogFormat))
			}
		})
	}
}

func TestSetLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected string
	}{
		{
			name:     "log level is set to debug",
			level:    "debug",
			expected: "debug",
		},
		{
			name:     "log level is set to info",
			level:    "info",
			expected: "info",
		},
		{
			name:     "log level is set to warn",
			level:    "warn",
			expected: "warning",
		},
		{
			name:     "log level is set to error",
			level:    "error",
			expected: "error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLogLevel(tt.level)
			assert.Equal(t, tt.expected, os.Getenv(common.EnvLogLevel))
		})
	}
}

func TestSetGLogLevel(t *testing.T) {
	SetGLogLevel(3)

	vFlag := flag.Lookup("v")
	assert.Equal(t, "3", vFlag.Value.String())

	logToStderrFlag := flag.Lookup("logtostderr")
	assert.Equal(t, "true", logToStderrFlag.Value.String())
}
