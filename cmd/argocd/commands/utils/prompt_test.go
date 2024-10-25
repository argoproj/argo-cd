package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPrompt_PromptsEnabled_True(t *testing.T) {
	promptsEnabled := true

	prompt, err := NewPrompt(promptsEnabled)
	require.NoError(t, err)

	assert.True(t, prompt.enabled)
}

func TestNewPrompt_PromptsEnabled_False(t *testing.T) {
	promptsEnabled := false

	prompt, err := NewPrompt(promptsEnabled)
	require.NoError(t, err)

	assert.False(t, prompt.enabled)
}

func TestConfirm_PromptsEnabled_False(t *testing.T) {
	promptsEnabled := false

	prompt, err := NewPrompt(promptsEnabled)
	require.NoError(t, err)

	assert.True(t, prompt.Confirm("Are you sure you want to run this command? (y/n) "))
}
