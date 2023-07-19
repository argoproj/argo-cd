package testdata

import _ "embed"

var (
	//go:embed live-deployment.yaml
	LiveDeploymentYaml string

	//go:embed target-deployment.yaml
	TargetDeploymentYaml string

	//go:embed live-mutating-webhook-config.yaml
	LiveMutatingWebhookConfigYaml string

	//go:embed target-mutating-webhook-config.yaml
	TargetMutatingWebhookConfigYaml string

	//go:embed target-deployment-new-entries.yaml
	TargetDeploymentNewEntries string
)
