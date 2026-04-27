package sandbox

import (
	configUtil "github.com/argoproj/argo-cd/v3/util/config"
	log "github.com/sirupsen/logrus"
)

type ArgocdSandboxConfig struct {
	Landlock *LandlockConfig `yaml:"landlock"`
}

func ReadSandboxConfig(filePath string) (*ArgocdSandboxConfig, error) {
	var config ArgocdSandboxConfig
	err := configUtil.UnmarshalLocalFile(filePath, &config)
	if err != nil {
		return nil, err
	}
	log.Debugf("read sandbox configuration: %v", config)
	// err = ValidateSandboxConfig(config)
	// if err != nil {
	// 	return nil, err
	// }

	return &config, nil
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
