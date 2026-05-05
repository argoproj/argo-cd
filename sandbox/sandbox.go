package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/argoproj/argo-cd/v3/common"
	log "github.com/sirupsen/logrus"
)

type SandboxRunOpts struct {
	RODirs []string
	RWDirs []string
}

type SandboxImpl interface {
	Name() string
	Init(sandboxConfig *ArgocdSandboxConfig, allowRules []string) error
	Apply() error
	GetConfig() string
	MakeArgs(runOpts *SandboxRunOpts) []string
}

func ExecuteCommand(cfg *ArgocdSandboxConfig, allowRules map[string][]string, args []string) error {
	modules, err := getModules(cfg)
	if err != nil {
		return err
	}
	for _, module := range modules {
		name := module.Name()
		log.Infof("Initializing sandbox module: %s", name)
		err := module.Init(cfg, allowRules[name])
		if err != nil {
			return fmt.Errorf("Failed to initialize module %q: %v", name, err)
		}
		log.Infof("module config is: %s", module.GetConfig())
	}
	for _, module := range modules {
		name := module.Name()
		log.Infof("Applying sandbox module: %s", name)
		log.Infof("module config is: %s", module.GetConfig())
		err := module.Apply()
		if err != nil {
			return fmt.Errorf("Failed to apply module %q: %v", name, err)
		}
	}
	binary := args[0]
	env := os.Environ()
	log.Infof("Executing %q %v", binary, args)
	err = syscall.Exec(binary, args, env)
	// normally won't get here
	return err
}

func RunStartupTest(ops *ToolOpts) error {
	if !ops.isEnabled {
		log.Infof("%s execution sandbox is disabled", ops.toolName)
		return nil
	}
	log.Infof("Performing %s execution sandbox self test", ops.toolName)
	_, err := initToolSandboxConfig(ops)
	if err != nil {
		return err
	}

	return nil
}

func GenerateDefaultSandboxConfig(ops *ToolOpts) (*ArgocdSandboxConfig, error) {
	cfg := &ArgocdSandboxConfig{}
	var err error
	for _, moduleName := range ops.modulesList {
		log.Infof("Generating default %s configuration for %s", moduleName, ops.toolName)
		if moduleName == LANDLOCK {
			cfg.Landlock, err = GenerateDefaultLandlockConfig(ops)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("Unknown sandbox module %q", moduleName)
		}

	}
	return cfg, nil
}

func CommandContext(ctx context.Context, sandboxRunOpts *SandboxRunOpts, cmdName string, args ...string) (*exec.Cmd, error) {
	var toolOpts *ToolOpts
	if cmdName == "helm" && HelmToolOps.isEnabled {
		log.Infof("executing command %s in sandbox", cmdName)
		toolOpts = &HelmToolOps
	} else if cmdName == "kustomize" && KustomizeToolOps.isEnabled {
		log.Infof("executing command %s in sandbox", cmdName)
		toolOpts = &KustomizeToolOps
	} else {
		log.Infof("executing command %q without sandbox", cmdName)
		return exec.CommandContext(ctx, cmdName, args...), nil
	}
	binPath, err := exec.LookPath(cmdName)
	if err != nil {
		return nil, fmt.Errorf("Failed to create command context for helm: %w", err)
	}
	args = makeSandboxCmdline(toolOpts, sandboxRunOpts, binPath, args...)
	return exec.CommandContext(ctx, common.CommandSandbox, args...), nil
}

func makeSandboxCmdline(toolOpts *ToolOpts, runOpts *SandboxRunOpts, binPath string, args ...string) []string {
	result := []string{}
	if toolOpts.configFilePath != "" {
		result = append(result, "--config", toolOpts.configFilePath)
	} else if toolOpts.configStr != "" {
		result = append(result, "--config-str", toolOpts.configStr)
	}
	modulesImpls := getModulesForNames(toolOpts.modulesList)
	for _, module := range modulesImpls {
		result = append(result, module.MakeArgs(runOpts)...)
	}
	result = append(result, "--")
	result = append(result, binPath)
	result = append(result, args...)
	return result
}

func sandboxConfigToString(cfg *ArgocdSandboxConfig) (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("Failed to marshal generated sandbox configuration: %v", err)
	}
	return string(b), nil
}

func initToolSandboxConfig(ops *ToolOpts) (*ArgocdSandboxConfig, error) {
	var sandboxCfg *ArgocdSandboxConfig
	var err error
	if ops.configFilePath != "" {
		log.Infof("Loading sandbox config for %s from file %q", ops.toolName, ops.configFilePath)
		sandboxCfg, err = ReadSandboxConfig(ops.configFilePath)
		if err != nil {
			log.Errorf("Failed to load sandbox config for %s: %v", ops.toolName, err)
			return nil, err
		}
	} else {
		log.Infof("Generating default sandbox config for %s", ops.toolName)
		sandboxCfg, err = GenerateDefaultSandboxConfig(ops)
		if err != nil {
			log.Errorf("Failed to generate sandbox config for %s: %v", ops.toolName, err)
			return nil, err
		}
		ops.configStr, err = sandboxConfigToString(sandboxCfg)
		if err != nil {
			log.Errorf("Failed to generate sandbox config for %s: %v", ops.toolName, err)
		}
		log.Infof("Generated sandbox config for %s is  %v", ops.toolName, ops.configStr)
	}
	return sandboxCfg, err
}

func RunStartupTests() error {
	log.Info("Performing tools execution sandbox self tests")
	err := RunStartupTest(&HelmToolOps)
	if err != nil {
		return err
	}
	err = RunStartupTest(&KustomizeToolOps)
	if err != nil {
		return err
	}
	return nil
}

func getModules(cfg *ArgocdSandboxConfig) ([]SandboxImpl, error) {
	var result []SandboxImpl
	if nil != cfg.Landlock {
		result = append(result, &Landlock{})
	}
	return result, nil
}

func getModulesForNames(moduleNames []string) []SandboxImpl {
	var result []SandboxImpl
	for _, moduleName := range moduleNames {
		if moduleName == LANDLOCK {
			result = append(result, &Landlock{})
		}
	}
	return result
}
