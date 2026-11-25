package cli

import (
	"errors"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
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
				cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run="+t.Name())
				cmd.Env = append(os.Environ(), "TEST_FATAL=1")
				err := cmd.Run()
				e := &exec.ExitError{}
				if errors.As(err, &e) {
					return
				}
				t.Fatal("expected fatal exit for invalid log format")
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

func TestBoundedFloat64Var(t *testing.T) {
	newFlagSet := func() (*pflag.FlagSet, *float64) {
		var v float64
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		fs.SetOutput(io.Discard)
		BoundedFloat64Var(fs, &v, "ratio", 1.0, 0.0, 1.0, "usage")
		return fs, &v
	}

	t.Run("default applied when flag absent", func(t *testing.T) {
		fs, v := newFlagSet()
		require.NoError(t, fs.Parse(nil))
		assert.InDelta(t, 1.0, *v, 0)
	})

	t.Run("type and default string for help/docs", func(t *testing.T) {
		fs, _ := newFlagSet()
		f := fs.Lookup("ratio")
		assert.Equal(t, "float", f.Value.Type())
		assert.Equal(t, "1", f.DefValue)
	})

	valid := map[string]float64{"0": 0.0, "0.3": 0.3, "1": 1.0}
	for in, want := range valid {
		t.Run("accepts "+in, func(t *testing.T) {
			fs, v := newFlagSet()
			require.NoError(t, fs.Parse([]string{"--ratio=" + in}))
			assert.InDelta(t, want, *v, 0)
		})
	}

	for _, in := range []string{"-0.5", "2", "NaN", "abc"} {
		t.Run("rejects "+in, func(t *testing.T) {
			fs, _ := newFlagSet()
			assert.Error(t, fs.Parse([]string{"--ratio=" + in}))
		})
	}
}

func TestInteractiveEdit_CleansUpTempFile(t *testing.T) {
	editorScript := filepath.Join(t.TempDir(), "editor.sh")
	err := os.WriteFile(editorScript, []byte("#!/bin/sh\necho 'edited: true' > \"$1\"\n"), 0o755)
	require.NoError(t, err)
	t.Setenv("EDITOR", editorScript)

	pattern := "test-interactive-edit-*.yaml"
	globPattern := filepath.Join(os.TempDir(), pattern)

	beforeMatches, err := filepath.Glob(globPattern)
	require.NoError(t, err)

	saved := false
	InteractiveEdit(pattern, []byte("initial: content"), func(input []byte) error {
		saved = true
		assert.Equal(t, []byte("edited: true\n"), input)
		return nil
	})

	assert.True(t, saved)

	afterMatches, err := filepath.Glob(globPattern)
	require.NoError(t, err)

	assert.Len(t, afterMatches, len(beforeMatches), "InteractiveEdit should clean up temporary file on success")
}

func TestInteractiveEdit_CleansUpTempFileOnRetry(t *testing.T) {
	editorScript := filepath.Join(t.TempDir(), "editor.sh")
	err := os.WriteFile(editorScript, []byte("#!/bin/sh\necho 'attempt: edit' > \"$1\"\n"), 0o755)
	require.NoError(t, err)
	t.Setenv("EDITOR", editorScript)

	pattern := "test-interactive-retry-*.yaml"
	globPattern := filepath.Join(os.TempDir(), pattern)

	beforeMatches, err := filepath.Glob(globPattern)
	require.NoError(t, err)

	attempts := 0
	InteractiveEdit(pattern, []byte("initial: content"), func(_ []byte) error {
		attempts++
		if attempts == 1 {
			return errors.New("validation error, retry required")
		}
		return nil
	})

	assert.Equal(t, 2, attempts)

	afterMatches, err := filepath.Glob(globPattern)
	require.NoError(t, err)

	assert.Len(t, afterMatches, len(beforeMatches), "InteractiveEdit should clean up all temporary files across retries")
}

func TestInteractiveEdit_CleansUpTempFileOnCancel(t *testing.T) {
	if os.Getenv("TEST_INTERACTIVE_EDIT_CANCEL") == "1" {
		t.Setenv("EDITOR", "true") // leaves file unchanged, triggering cancellation
		InteractiveEdit("test-interactive-cancel-*.yaml", []byte("initial: content"), func(_ []byte) error {
			return nil
		})
		return
	}

	pattern := "test-interactive-cancel-*.yaml"
	globPattern := filepath.Join(os.TempDir(), pattern)

	beforeMatches, err := filepath.Glob(globPattern)
	require.NoError(t, err)

	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run="+t.Name())
	cmd.Env = append(os.Environ(), "TEST_INTERACTIVE_EDIT_CANCEL=1")
	err = cmd.Run()
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 20, exitErr.ExitCode(), "expected generic error exit code from errors.CheckError")

	afterMatches, err := filepath.Glob(globPattern)
	require.NoError(t, err)

	assert.Len(t, afterMatches, len(beforeMatches), "InteractiveEdit should clean up temporary file on cancellation")
}
