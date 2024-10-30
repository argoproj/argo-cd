package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPrompt_PromptsEnabled_True(t *testing.T) {
	prompt := NewPrompt(true)
	assert.True(t, prompt.enabled)
}

func TestNewPrompt_PromptsEnabled_False(t *testing.T) {
	prompt := NewPrompt(false)
	assert.False(t, prompt.enabled)
}

func TestConfirm_PromptsEnabled_False(t *testing.T) {
	prompt := NewPrompt(false)
	assert.True(t, prompt.Confirm("Are you sure you want to run this command? (y/n) "))
}
