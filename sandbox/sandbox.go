package sandbox

import (
	"fmt"
	"os"
	"syscall"

	log "github.com/sirupsen/logrus"
)

type SandboxImpl interface {
	Name() string
	Init(sandboxConfig *ArgocdSandboxConfig, allowRules []string) error
	Apply() error
	GetConfig() string
}

func ExecuteCommand(cfg *ArgocdSandboxConfig, allowRules []string, args []string) error {
	modules, err := getModules(cfg)
	if err != nil {
		return err
	}
	for _, module := range modules {
		name := module.Name()
		log.Infof("Initializing sandbox module: %s", name)
		err := module.Init(cfg, allowRules)
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

func getModules(cfg *ArgocdSandboxConfig) ([]SandboxImpl, error) {
	var result []SandboxImpl
	if nil != cfg.Landlock {
		result = append(result, &Landlock{})
	}
	return result, nil
}
