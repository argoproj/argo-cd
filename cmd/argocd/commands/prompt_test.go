package commands

import (
	"context"
	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/admin"
	utils "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"testing"
)

func tempFile(content string) (string, io.Closer, error) {
	f, err := os.CreateTemp("", "*.yaml")
	if err != nil {
		return "", nil, err
	}
	_, err = f.Write([]byte(content))
	if err != nil {
		_ = os.Remove(f.Name())
		return "", nil, err
	}
	defer func() {
		if err = f.Close(); err != nil {
			panic(err)
		}
	}()
	return f.Name(), utils.NewCloser(func() error {
		return os.Remove(f.Name())
	}), nil
}

func TestNewPrompt_PromptsEnabled_True(t *testing.T) {
	ctx := context.Background()

	f, closer, err := tempFile(`apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
data:
  prompts.enabled: "true"`)
	require.NoError(t, err)
	defer utils.Close(closer)

	opts := admin.SettingsOpts{ArgocdCMPath: f}

	settingsManager, err := opts.CreateSettingsManager(ctx)
	require.NoError(t, err)

	prompt, err := NewPrompt(settingsManager)
	require.NoError(t, err)

	assert.Equal(t, true, prompt.enabled)
}

func TestNewPrompt_PromptsEnabled_False(t *testing.T) {
	ctx := context.Background()

	f, closer, err := tempFile(`apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
data:
  prompts.enabled: "false"`)
	require.NoError(t, err)
	defer utils.Close(closer)

	opts := admin.SettingsOpts{ArgocdCMPath: f}

	settingsManager, err := opts.CreateSettingsManager(ctx)
	require.NoError(t, err)

	prompt, err := NewPrompt(settingsManager)
	require.NoError(t, err)

	assert.Equal(t, false, prompt.enabled)
}

func TestNewPrompt_PromptsEnabled_Unspecified(t *testing.T) {
	ctx := context.Background()

	f, closer, err := tempFile(`apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm`)
	require.NoError(t, err)
	defer utils.Close(closer)

	opts := admin.SettingsOpts{ArgocdCMPath: f}

	settingsManager, err := opts.CreateSettingsManager(ctx)
	require.NoError(t, err)

	prompt, err := NewPrompt(settingsManager)
	require.NoError(t, err)

	assert.Equal(t, false, prompt.enabled)
}
