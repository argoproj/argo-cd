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

// Returns true, true when prompt is disabled
func TestConfirmAllPromptDisabled(t *testing.T) {
	p := &Prompt{enabled: false}
	result1, result2 := p.ConfirmAll("Proceed?")
	if result1 != true || result2 != true {
		t.Errorf("Expected (true, true), got (%v, %v)", result1, result2)
	}
}

func TestConfirmBaseOnCountPromptDisabled(t *testing.T) {
	p := &Prompt{enabled: false}
	result1, result2 := p.ConfirmBaseOnCount("Proceed?", "Process all?", 2)

	if result1 != true || result2 != true {
		t.Errorf("Expected (true, true), got (%v, %v)", result1, result2)
	}
}

func TestConfirmBaseOnCountZeroApps(t *testing.T) {
	p := &Prompt{enabled: true}
	result1, result2 := p.ConfirmBaseOnCount("Proceed?", "Process all?", 0)

	if result1 != true || result2 != true {
		t.Errorf("Expected (true, true), got (%v, %v)", result1, result2)
	}
}
