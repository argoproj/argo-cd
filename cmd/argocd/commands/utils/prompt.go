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
