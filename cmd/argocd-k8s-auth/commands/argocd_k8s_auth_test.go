package commands

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactARN(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		arn  string
		want string
	}{
		{name: "empty", arn: "", want: ""},
		{name: "role arn", arn: "arn:aws:iam::123456789012:role/MyRole", want: "role/MyRole"},
		{name: "role path arn", arn: "arn:aws:iam::123456789012:role/path/to/MyRole", want: "role/path/to/MyRole"},
		{name: "non arn name", arn: "MyRole", want: "MyRole"},
		{name: "arn without role segment", arn: "arn:aws:sts::123456789012:assumed-role/MyRole/session", want: "session"},
		{name: "arn without slash", arn: "arn:aws:iam::123456789012:root", want: "<redacted>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, redactARN(tt.arn))
		})
	}
}

func TestVerboseLog(t *testing.T) {
	t.Parallel()

	t.Run("writes nothing when verbose is disabled", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		ctx := contextWithVerbose(t.Context(), false, &buf)
		verboseLog(ctx, "should not appear %s", "here")
		assert.Empty(t, buf.String())
	})

	t.Run("writes to provided writer when verbose is enabled", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		ctx := contextWithVerbose(t.Context(), true, &buf)
		verboseLog(ctx, "hello %s", "world")
		assert.Equal(t, "hello world\n", buf.String())
	})

	t.Run("does not write when context has no verbose state", func(t *testing.T) {
		t.Parallel()
		// Ensure a missing context value is a no-op (defaults must not panic).
		verboseLog(context.Background(), "should not panic")
	})
}

func TestContextWithVerboseFromCmd(t *testing.T) {
	t.Parallel()

	t.Run("stores verbose flag and command err writer", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().Bool("verbose", false, "")
		require.NoError(t, cmd.Flags().Set("verbose", "true"))

		var errBuf bytes.Buffer
		cmd.SetErr(&errBuf)

		ctx, err := contextWithVerboseFromCmd(cmd)
		require.NoError(t, err)
		assert.True(t, verboseFromContext(ctx))

		verboseLog(ctx, "via err writer")
		assert.Equal(t, "via err writer\n", errBuf.String())
	})

	t.Run("returns error when verbose flag is missing", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{Use: "test"}
		_, err := contextWithVerboseFromCmd(cmd)
		require.Error(t, err)
	})
}

func TestVerboseFlagHelpDocumentsSecurity(t *testing.T) {
	t.Parallel()
	cmd := NewCommand()
	flag := cmd.PersistentFlags().Lookup("verbose")
	require.NotNil(t, flag)
	assert.Contains(t, strings.ToLower(flag.Usage), "stderr")
	assert.Contains(t, strings.ToLower(flag.Usage), "stdout")
	assert.Contains(t, flag.Usage, "redacted")
}

func TestAWSVerboseLogRedactsARNAndUsesStderrWriter(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	cmd := &cobra.Command{Use: "aws"}
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.Flags().Bool("verbose", true, "")

	ctx, err := contextWithVerboseFromCmd(cmd)
	require.NoError(t, err)

	roleARN := "arn:aws:iam::123456789012:role/MyRole"
	verboseLog(ctx, "argocd-k8s-auth aws: cluster-name=%q role-arn=%q profile=%q", "my-cluster", redactARN(roleARN), "default")
	verboseLog(ctx, "argocd-k8s-auth aws: assuming role %q", redactARN(roleARN))
	_, _ = fmt.Fprint(cmd.OutOrStdout(), `{"kind":"ExecCredential"}`)

	assert.Equal(t, `{"kind":"ExecCredential"}`, stdout.String())
	assert.Contains(t, stderr.String(), "argocd-k8s-auth aws:")
	assert.Contains(t, stderr.String(), "role/MyRole")
	assert.NotContains(t, stderr.String(), "arn:aws:iam::123456789012:role/MyRole")
	assert.Contains(t, stderr.String(), "my-cluster")
	assert.NotContains(t, stdout.String(), "argocd-k8s-auth aws:")
}
