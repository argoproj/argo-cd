package plugin

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	configUtil "github.com/argoproj/argo-cd/v2/util/config"
)

const (
	ConfigManagementPluginKind string = "ConfigManagementPlugin"
)

type PluginConfig struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ObjectMeta `json:"metadata"`
	Spec            PluginConfigSpec  `json:"spec"`
}

type PluginConfigSpec struct {
	Version          string     `json:"version"`
	Init             Command    `json:"init,omitempty"`
	Generate         Command    `json:"generate"`
	Discover         Discover   `json:"discover"`
	Parameters       Parameters `yaml:"parameters"`
	PreserveFileMode bool       `json:"preserveFileMode,omitempty"`
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

	if err = ValidatePluginConfig(config); err != nil {
		return nil, err
	}

	return &config, nil
}

func ValidatePluginConfig(config PluginConfig) error {
	if config.Metadata.Name == "" {
		return fmt.Errorf("invalid plugin configuration file. metadata.name should be non-empty.")
	}
	if config.TypeMeta.Kind != ConfigManagementPluginKind {
		return fmt.Errorf("invalid plugin configuration file. kind should be %s, found %s", ConfigManagementPluginKind, config.TypeMeta.Kind)
	}
	if len(config.Spec.Generate.Command) == 0 {
		return fmt.Errorf("invalid plugin configuration file. spec.generate command should be non-empty")
	}
	// discovery field is optional as apps can now specify plugin names directly
	return nil
}

func (cfg *PluginConfig) Address() string {
	var address string
	pluginSockFilePath := common.GetPluginSockFilePath()
	if cfg.Spec.Version != "" {
		address = fmt.Sprintf("%s/%s-%s.sock", pluginSockFilePath, cfg.Metadata.Name, cfg.Spec.Version)
	} else {
		address = fmt.Sprintf("%s/%s.sock", pluginSockFilePath, cfg.Metadata.Name)
	}
	return address
}
