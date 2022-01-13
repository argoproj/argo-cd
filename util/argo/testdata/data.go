package testdata

import _ "embed"

var (
	//go:embed live_deployment_with_managed_replica.yaml
	LiveDeploymentWithManagedReplicaYaml string

	//go:embed desired_deployment.yaml
	DesiredDeploymentYaml string
)
