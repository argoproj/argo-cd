package testdata

import _ "embed"

var (
	//go:embed live-deployment.yaml
	LiveDeploymentYaml string

	//go:embed target-deployment.yaml
	TargetDeploymentYaml string

	//go:embed target-deployment-new-entries.yaml
	TargetDeploymentNewEntries string

	//go:embed diff-cache.yaml
	DiffCacheYaml string

	//go:embed live-httpproxy.yaml
	LiveHTTPProxy string

	//go:embed target-httpproxy.yaml
	TargetHTTPProxy string

	//go:embed live-deployment-env-vars.yaml
	LiveDeploymentEnvVarsYaml string

	//go:embed target-deployment-env-vars.yaml
	TargetDeploymentEnvVarsYaml string

	//go:embed minimal-image-replicas-deployment.yaml
	MinimalImageReplicaDeploymentYaml string

	//go:embed additional-image-replicas-deployment.yaml
	AdditionalImageReplicaDeploymentYaml string

	//go:embed simple-app-live.yaml
	SimpleAppLiveYaml string

	//go:embed simple-app-target.yaml
	SimpleAppTargetYaml string

	//go:embed target-rollout.yaml
	TargetRolloutYaml string

	//go:embed live-rollout.yaml
	LiveRolloutYaml string

	//go:embed live-child-application.yaml
	LiveChildApplicationYaml string

	//go:embed target-child-application.yaml
	TargetChildApplicationYaml string
)
