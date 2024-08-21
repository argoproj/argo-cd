package commands

import (
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

type Prompt struct {
	enabled bool
}

func NewPrompt(settingsManager *settings.SettingsManager) (*Prompt, error) {
	argoCDSettings, err := settingsManager.GetSettings()
	if err != nil {
		return nil, err
	}

	return &Prompt{
		enabled: argoCDSettings.PromptsEnabled,
	}, nil
}

func (p *Prompt) Confirm(message string) bool {
	if p.enabled {
		return cli.AskToProceed(message)
	}

	return true
}
