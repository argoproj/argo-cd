package testdata

import _ "embed"

var (
	//go:embed smd-service-config.yaml
	ServiceConfigYAML string

	//go:embed smd-service-live.yaml
	ServiceLiveYAML string

	//go:embed smd-service-live-no-type.yaml
	ServiceLiveNoTypeYAML string

	//go:embed smd-service-config-2-ports.yaml
	ServiceConfigWith2Ports string

	//go:embed smd-service-live-expected-2-ports.yaml
	ExpectedServiceLiveWith2PortsYAML string

	//go:embed smd-service-live-with-type.yaml
	LiveServiceWithTypeYAML string

	//go:embed smd-service-config-ports.yaml
	ServiceConfigWithSamePortsYAML string
)
