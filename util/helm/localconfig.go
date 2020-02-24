package helm

import (
	"os"
	"os/user"
	"path"

	configUtil "github.com/argoproj/argo-cd/util/config"
)

// Helm configuration data
type LocalRepositoryConfig struct {
	APIVersion   string            `yaml:"apiVersion"`
	Repositories []LocalRepository `yaml:"repositories"`
}

// Helm repository configuration data
type LocalRepository struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	CAFile   string `yaml:"caFile"`
	CertFile string `yaml:"certFile"`
	KeyFile  string `yaml:"keyFile"`
}

// ReadLocalConfig loads up the local configuration file. Returns nil if config does not exist
func ReadLocalRepositoryConfig(path string) ([]LocalRepository, error) {
	var err error
	var config LocalRepositoryConfig
	err = configUtil.UnmarshalLocalFile(path, &config)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return config.Repositories, nil
}

// DefaultConfigDir returns the default Helm configuration path.
func DefaultConfigDir() (string, error) {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		usr, err := user.Current()
		if err != nil {
			return "", err
		}
		homeDir = usr.HomeDir
	}
	return path.Join(homeDir, ".helm"), nil
}

// DefaultLocalRepositoryConfigPath returns the local Helm repositories configuration path.
func DefaultLocalRepositoryConfigPath() (string, error) {
	dir, err := DefaultConfigDir()
	if err != nil {
		return "", err
	}
	return path.Join(dir, "repository", "repositories.yaml"), nil
}
