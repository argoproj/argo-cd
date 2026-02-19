package settings

import (
	_ "embed"

	"gopkg.in/yaml.v3"
)

//go:embed default_ignore_resource_updates.yaml
var defaultIgnoreResourceUpdatesYaml []byte

// defaultIgnoreResourceUpdates holds the default map of resource-specific ignoreResourceUpdates configurations.
var defaultIgnoreResourceUpdates map[string]string

func init() {
	err := yaml.Unmarshal(defaultIgnoreResourceUpdatesYaml, &defaultIgnoreResourceUpdates)
	if err != nil {
		panic(err)
	}
}
