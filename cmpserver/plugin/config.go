package plugin

import (
	"errors"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	configUtil "github.com/argoproj/argo-cd/v3/util/config"
)

const (
	ConfigManagementPluginKind string = "ConfigManagementPlugin"
)

// errVersionUnsupported is returned when a plugin configuration still sets the removed spec.version
// field. Keeping it in one place lets the validator and its tests share a single source of truth.
var errVersionUnsupported = errors.New("invalid plugin configuration file. spec.version is no longer supported. Remove it and, if you need to distinguish plugin versions, append the version to metadata.name instead (e.g. metadata.name: <name>-<version>)")

type PluginConfig struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ObjectMeta `json:"metadata"`
	Spec            PluginConfigSpec  `json:"spec"`
}

type PluginConfigSpec struct {
	// Version is no longer supported. It is retained only so that the CMP server can
	// detect a config that still sets it and return an actionable error. Append the
	// version to metadata.name instead (e.g. metadata.name: <name>-<version>).
	//
	// Deprecated: unsupported in Argo CD 4.0.
	Version          string     `json:"version"`
	Init             Command    `json:"init,omitempty"`
	Generate         Command    `json:"generate"`
	Discover         Discover   `json:"discover"`
	Parameters       Parameters `yaml:"parameters"`
	PreserveFileMode bool       `json:"preserveFileMode,omitempty"`
	ProvideGitCreds  bool       `json:"provideGitCreds,omitempty"`
}

// Discover holds find and fileName
type Discover struct {
	Find     Find   `json:"find"`
	FileName string `json:"fileName"`
}

func (d Discover) IsDefined() bool {
	return d.FileName != "" || d.Find.Glob != "" || len(d.Find.Command.Command) > 0
}

// Command holds binary path and arguments list
type Command struct {
	Command []string `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

// Find holds find command or glob pattern
type Find struct {
	Command
	Glob string `json:"glob"`
}

// Parameters holds static and dynamic configurations
type Parameters struct {
	Static  []*apiclient.ParameterAnnouncement `yaml:"static"`
	Dynamic Command                            `yaml:"dynamic"`
}

// Dynamic hold the dynamic announcements for CMP's
type Dynamic struct {
	Command
}

func ReadPluginConfig(filePath string) (*PluginConfig, error) {
	path := fmt.Sprintf("%s/%s", strings.TrimRight(filePath, "/"), common.PluginConfigFileName)

	var config PluginConfig
	err := configUtil.UnmarshalLocalFile(path, &config)
	if err != nil {
		return nil, err
	}

	err = ValidatePluginConfig(config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func ValidatePluginConfig(config PluginConfig) error {
	if config.Metadata.Name == "" {
		return errors.New("invalid plugin configuration file. metadata.name should be non-empty")
	}
	if config.Kind != ConfigManagementPluginKind {
		return fmt.Errorf("invalid plugin configuration file. kind should be %s, found %s", ConfigManagementPluginKind, config.Kind)
	}
	if config.Spec.Version != "" {
		return errVersionUnsupported
	}
	if len(config.Spec.Generate.Command) == 0 {
		return errors.New("invalid plugin configuration file. spec.generate command should be non-empty")
	}
	// discovery field is optional as apps can now specify plugin names directly
	return nil
}

func (cfg *PluginConfig) Address() string {
	pluginSockFilePath := common.GetPluginSockFilePath()
	return fmt.Sprintf("%s/%s.sock", pluginSockFilePath, cfg.Metadata.Name)
}
