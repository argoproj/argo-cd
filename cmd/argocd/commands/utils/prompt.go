package utils

import (
	"context"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Prompt struct {
	enabled bool
}

func NewPrompt(ctx context.Context, clientConfig clientcmd.ClientConfig) (*Prompt, error) {
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	namespace, _, err := clientConfig.Namespace()
	if err != nil {
		return nil, err
	}

	kubeClientset := kubernetes.NewForConfigOrDie(config)

	settingsManager := settings.NewSettingsManager(ctx, kubeClientset, namespace)

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
