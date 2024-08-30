package utils

import (
	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/localconfig"
)

type Prompt struct {
	enabled bool
}

func NewPrompt(clientOpts *apiclient.ClientOptions) (*Prompt, error) {
	localCfg, err := localconfig.ReadLocalConfig(clientOpts.ConfigPath)
	if err != nil {
		return nil, err
	}

	return &Prompt{
		enabled: localCfg.PromptsEnabled,
	}, nil
}

func (p *Prompt) Confirm(message string) bool {
	if !p.enabled {
		return true
	}

	return cli.AskToProceed(message)
}
