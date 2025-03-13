package testdata

import _ "embed"

var (
	//go:embed smd-service-config.yaml
	ServiceConfigYAML string

	//go:embed smd-service-live.yaml
	ServiceLiveYAML string

	//go:embed smd-service-config-2-ports.yaml
	ServiceConfigWith2Ports string

	//go:embed smd-service-live-with-type.yaml
	LiveServiceWithTypeYAML string

	//go:embed smd-service-config-ports.yaml
	ServiceConfigWithSamePortsYAML string

	//go:embed smd-deploy-live.yaml
	DeploymentLiveYAML string

	//go:embed smd-deploy-config.yaml
	DeploymentConfigYAML string

	//go:embed smd-deploy2-live.yaml
	Deployment2LiveYAML string

	//go:embed smd-deploy2-config.yaml
	Deployment2ConfigYAML string

	//go:embed smd-deploy2-predicted-live.json
	Deployment2PredictedLiveJSONSSD string

	// OpenAPIV2Doc is a binary representation of the openapi
	// document available in a given k8s instance. To update
	// this file the following commands can be executed:
	//    kubectl proxy --port=7777 &
	//    curl -s -H Accept:application/com.github.proto-openapi.spec.v2@v1.0+protobuf http://localhost:7777/openapi/v2 > openapiv2.bin
	//
	//go:embed openapiv2.bin
	OpenAPIV2Doc []byte

	//go:embed ssd-service-config.yaml
	ServiceConfigYAMLSSD string

	//go:embed ssd-service-live.yaml
	ServiceLiveYAMLSSD string

	//go:embed ssd-service-predicted-live.json
	ServicePredictedLiveJSONSSD string

	//go:embed ssd-deploy-nested-config.yaml
	DeploymentNestedConfigYAMLSSD string

	//go:embed ssd-deploy-nested-live.yaml
	DeploymentNestedLiveYAMLSSD string

	//go:embed ssd-deploy-nested-predicted-live.json
	DeploymentNestedPredictedLiveJSONSSD string

	//go:embed ssd-deploy-with-manual-apply-config.yaml
	DeploymentApplyConfigYAMLSSD string

	//go:embed ssd-deploy-with-manual-apply-live.yaml
	DeploymentApplyLiveYAMLSSD string

	//go:embed ssd-deploy-with-manual-apply-predicted-live.json
	DeploymentApplyPredictedLiveJSONSSD string
)
