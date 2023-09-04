package testdata

import _ "embed"

var (
	//go:embed live-deployment.yaml
	LiveDeploymentYaml string

	//go:embed target-deployment.yaml
	TargetDeploymentYaml string

	//go:embed live-deployment-env-vars.yaml
	LiveDeploymentEnvVarsYaml string

	//go:embed target-deployment-env-vars.yaml
	TargetDeploymentEnvVarsYaml string

	//go:embed minimal-image-replicas-deployment.yaml
	MinimalImageReplicaDeploymentYaml string

	//go:embed additional-image-replicas-deployment.yaml
	AdditionalImageReplicaDeploymentYaml string

	//go:embed live-mutating-webhook-config.yaml
	LiveMutatingWebhookConfigYaml string

	//go:embed target-mutating-webhook-config.yaml
	TargetMutatingWebhookConfigYaml string

	//go:embed live-rollout.yaml
	LiveRolloutYaml string

	//go:embed target-rollout.yaml
	TargetRolloutYaml string

	//go:embed target-deployment-new-entries.yaml
	TargetDeploymentNewEntries string
)
