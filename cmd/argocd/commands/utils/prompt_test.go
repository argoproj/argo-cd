package utils

import (
	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

const testConfigFilePath = "../testdata/local.config"

func TestNewPrompt_PromptsEnabled_True(t *testing.T) {
	clientOpts := apiclient.ClientOptions{
		PromptsEnabled: true,
	}

	prompt, err := NewPrompt(&clientOpts)
	require.NoError(t, err)

	assert.True(t, prompt.enabled)
}

func TestNewPrompt_PromptsEnabled_False(t *testing.T) {
	clientOpts := apiclient.ClientOptions{
		PromptsEnabled: false,
	}

	prompt, err := NewPrompt(&clientOpts)
	require.NoError(t, err)

	assert.False(t, prompt.enabled)
}

func TestNewPrompt_PromptsEnabled_Unspecified(t *testing.T) {
	clientOpts := apiclient.ClientOptions{}

	prompt, err := NewPrompt(&clientOpts)
	require.NoError(t, err)

	assert.False(t, prompt.enabled)
}

func TestConfirm_PromptsEnabled_False(t *testing.T) {
	clientOpts := apiclient.ClientOptions{
		PromptsEnabled: false,
	}

	prompt, err := NewPrompt(&clientOpts)
	require.NoError(t, err)

	assert.True(t, prompt.Confirm("Are you sure you want to run this command? (y/n) "))
}
