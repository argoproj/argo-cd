package plugin

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/common"
	configUtil "github.com/argoproj/argo-cd/v2/util/config"
)

const (
	ConfigManagementPluginKind string = "ConfigManagementPlugin"
)

type PluginConfig struct {
	metav1.TypeMeta `yaml:",inline"`
	Metadata        metav1.ObjectMeta `yaml:"metadata"`
	Spec            PluginConfigSpec  `yaml:"spec"`
}

type PluginConfigSpec struct {
	Version          string   `yaml:"version"`
	Init             Command  `yaml:"init,omitempty"`
	Generate         Command  `yaml:"generate"`
	Discover         Discover `yaml:"discover"`
	AllowConcurrency bool     `yaml:"allowConcurrency"`
	LockRepo         bool     `yaml:"lockRepo"`
}

//Discover holds find and fileName
type Discover struct {
	Find     Command `yaml:"find"`
	FileName string  `yaml:"fileName"`
}

// Command holds binary path and arguments list
type Command struct {
	Command []string `yaml:"command,omitempty"`
	Args    []string `yaml:"args,omitempty"`
	Glob    string   `yaml:"glob"`
}

func ReadPluginConfig(filePath string) (*PluginConfig, error) {
	cnfFiles, err := os.ReadDir(filePath)
	if err != nil {
		return nil, err
	}

	var config PluginConfig
	for _, file := range cnfFiles {
		if !file.IsDir() {
			path := fmt.Sprintf("%s/%s", strings.TrimRight(filePath, "/"), file.Name())
			err = configUtil.UnmarshalLocalFile(path, &config)
			if err == nil {
				break
			} else {
				log.Errorf("failed to unmarshal plugin config file, %v", err)
			}
		}
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
	if config.Spec.Discover.Find.Glob == "" && len(config.Spec.Discover.Find.Command) == 0 && config.Spec.Discover.FileName == "" {
		return fmt.Errorf("invalid plugin configuration file. atleast one of discover.find.command or discover.find.glob or discover.fineName should be non-empty")
	}
	return nil
}

func (cfg *PluginConfig) Address() string {
	var address string
	pluginSockFilePath := common.GetPluginSockFilePath()
	address = fmt.Sprintf("%s/%s.sock", pluginSockFilePath, cfg.Metadata.Name)
	if cfg.Spec.Version != "" {
		address = fmt.Sprintf("%s/%s-%s.sock", pluginSockFilePath, cfg.Metadata.Name, cfg.Spec.Version)
	}
	return address
}
