package utils

import (
	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/util/cli"
)

type Prompt struct {
	enabled bool
}

func NewPrompt(clientOpts *apiclient.ClientOptions) (*Prompt, error) {
	return &Prompt{
		enabled: clientOpts.PromptsEnabled,
	}, nil
}

func (p *Prompt) Confirm(message string) bool {
	if !p.enabled {
		return true
	}

	return cli.AskToProceed(message)
}
