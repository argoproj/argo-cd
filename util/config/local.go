package config

import (
	"os/user"
	"path"

	"github.com/argoproj/argo-cd/util/cli"
)

// LocalConfig holds all local session config.
type LocalConfig struct {
	Sessions map[string]string `json:"sessions"`
}

// LocalConfigDir returns the local configuration path for settings such as cached authentication tokens.
func localConfigDir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return path.Join(usr.HomeDir, ".argocd"), nil
}

// LocalConfigPath returns the local configuration path for settings such as cached authentication tokens.
func localConfigPath() (string, error) {
	dir, err := localConfigDir()
	if err != nil {
		return "", err
	}
	return path.Join(dir, "config"), nil
}

// ReadLocalConfig loads up the local configuration file.
func ReadLocalConfig() (LocalConfig, error) {
	config := LocalConfig{
		Sessions: make(map[string]string),
	}

	path, err := localConfigPath()
	if err == nil {
		err = cli.UnmarshalLocalFile(path, &config)
	}

	return config, err
}

// WriteLocalConfig writes a new local configuration file.
func WriteLocalConfig(config LocalConfig) error {
	path, err := localConfigPath()
	if err == nil {
		err = cli.MarshalLocalYAMLFile(path, config)
	}

	return err
}
