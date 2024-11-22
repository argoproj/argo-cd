package utils

import (
	"github.com/argoproj/argo-cd/v2/util/cli"
)

type Prompt struct {
	enabled bool
}

func NewPrompt(promptsEnabled bool) *Prompt {
	return &Prompt{
		enabled: promptsEnabled,
	}
}

func (p *Prompt) Confirm(message string) bool {
	if !p.enabled {
		return true
	}

	return cli.AskToProceed(message)
}

// ConfirmAll asks the user to confirm an action. If prompts are disabled, it will return true.
// support y/n and A, which means all
// return if confirm and if all
func (p *Prompt) ConfirmAll(message string) (bool, bool) {
	if !p.enabled {
		return true, true
	}

	result := cli.AskToProceedS(message)

	if result == "a" {
		return true, true
	}

	if result == "y" {
		return true, false
	}

	return false, false
}

func (p *Prompt) ConfirmBaseOnCount(messageForSingle string, messageForArray string, count int) (bool, bool) {
	if !p.enabled {
		return true, true
	}

	if count == 0 {
		return true, true
	}

	if count == 1 {
		return p.Confirm(messageForSingle), true
	}

	return p.ConfirmAll(messageForArray)
}
