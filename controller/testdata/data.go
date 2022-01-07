package testdata

import _ "embed"

var (
	//go:embed live-deployment.yaml
	LiveDeploymentYaml string

	//go:embed target-deployment.yaml
	TargetDeploymentYaml string
)
