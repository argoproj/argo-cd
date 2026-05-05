package sandbox

import (
	configUtil "github.com/argoproj/argo-cd/v3/util/config"
	"github.com/argoproj/argo-cd/v3/util/env"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const BEST_EFFORT_MODE = "best_effort"
const STRICT_MODE = "strict"

type ArgocdSandboxConfig struct {
	Landlock *LandlockConfig `yaml:"landlock"`
}

type ToolOpts struct {
	toolName       string
	isEnabled      bool
	configFilePath string
	configStr      string
	modulesList    []string
	compatMode     string
}

func ReadSandboxConfig(filePath string) (*ArgocdSandboxConfig, error) {
	var config ArgocdSandboxConfig
	err := configUtil.UnmarshalLocalFile(filePath, &config)
	if err != nil {
		return nil, err
	}
	log.Debugf("read sandbox configuration from file: %v", config)
	return &config, nil
}

func ReadSandboxConfigStr(configStr string) (*ArgocdSandboxConfig, error) {
	var config ArgocdSandboxConfig
	err := configUtil.Unmarshal([]byte(configStr), &config)
	if err != nil {
		return nil, err
	}
	log.Debugf("read sandbox configuration: %v", config)
	return &config, nil
}

var CompatMode = BEST_EFFORT_MODE

var HelmToolOps = ToolOpts{
	toolName:       "helm",
	isEnabled:      false,
	modulesList:    []string{},
	configFilePath: "",
}
var KustomizeToolOps = ToolOpts{
	toolName:       "kustomize",
	isEnabled:      false,
	modulesList:    []string{},
	configFilePath: "",
}

func AddSandboxFlagsToRepoServerCmd(command *cobra.Command) {
	command.Flags().StringVar(&CompatMode, "sandbox-compat-mode",
		env.StringFromEnv("ARGOCD_REPO_SERVER_SANDBOX_COMPAT_MODE", BEST_EFFORT_MODE),
		"Sandbox compatibility mode")

	command.Flags().BoolVar(&HelmToolOps.isEnabled, "helm-sandbox-enabled",
		env.ParseBoolFromEnv("ARGOCD_REPO_SERVER_HELM_SANDBOX_ENABLED", true),
		"Run Helm in security sandbox")
	command.Flags().StringVar(&HelmToolOps.configFilePath, "helm-sandbox-config",
		env.StringFromEnv("ARGOCD_REPO_SERVER_HELM_SANDBOX_CONFIG", ""),
		"Path to Helm sandbox configuration file")
	command.Flags().StringSliceVar(&HelmToolOps.modulesList, "helm-sandbox-modules",
		env.StringsFromEnv("ARGOCD_REPO_SERVER_HELM_SANDBOX_MODULES", []string{"landlock"}, ","),
		"Security modules enabled for Helm sandbox")

	command.Flags().BoolVar(&KustomizeToolOps.isEnabled, "kustomize-sandbox-enabled",
		env.ParseBoolFromEnv("ARGOCD_REPO_SERVER_KUSTOMIZE_SANDBOX_ENABLED", true),
		"Run Kustomize in security sandbox")
	command.Flags().StringVar(&KustomizeToolOps.configFilePath, "kustomize-sandbox-config",
		env.StringFromEnv("ARGOCD_REPO_SERVER_KUSTOMIZE_SANDBOX_CONFIG", ""),
		"Path to Kustomize sandbox configuration file")
	command.Flags().StringSliceVar(&KustomizeToolOps.modulesList, "kustomize-sandbox-modules",
		env.StringsFromEnv("ARGOCD_REPO_SERVER_KUSTOMIZE_SANDBOX_MODULES", []string{"landlock"}, ","),
		"Security modules enabled for Kustomize sandbox")
}

// func ValidateSandboxConfig(config ArgocdSandboxConfig) error {
// 	if config.Landlock != nil {
// 		err := ValidateLandlockConfig(*config.Landlock)
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }
