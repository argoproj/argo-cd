package testdata

import _ "embed"

var (
	//go:embed live_deployment_with_managed_replica.yaml
	LiveDeploymentWithManagedReplicaYaml string

	//go:embed desired_deployment.yaml
	DesiredDeploymentYaml string

	//go:embed live_deployment_with_managed_replica_env_duplicate.yaml
	LiveDeploymentWithManagedReplicaEnvDuplicateYaml string

	//go:embed desired_deployment_env_duplicate.yaml
	DesiredDeploymentEnvDuplicateYaml string

	//go:embed live_validating_webhook.yaml
	LiveValidatingWebhookYaml string

	//go:embed desired_validating_webhook.yaml
	DesiredValidatingWebhookYaml string
)
