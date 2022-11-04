package registry

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/argoproj/argo-cd/v2/image-updater/log"

	"gopkg.in/yaml.v2"
)

// RegistryConfiguration represents a single repository configuration for being
// unmarshaled from YAML.
type RegistryConfiguration struct {
	Name        string        `yaml:"name"`
	ApiURL      string        `yaml:"api_url"`
	Ping        bool          `yaml:"ping,omitempty"`
	Credentials string        `yaml:"credentials,omitempty"`
	CredsExpire time.Duration `yaml:"credsexpire,omitempty"`
	TagSortMode string        `yaml:"tagsortmode,omitempty"`
	Prefix      string        `yaml:"prefix,omitempty"`
	Insecure    bool          `yaml:"insecure,omitempty"`
	DefaultNS   string        `yaml:"defaultns,omitempty"`
	Limit       int           `yaml:"limit,omitempty"`
	IsDefault   bool          `yaml:"default,omitempty"`
}

// RegistryList contains multiple RegistryConfiguration items
type RegistryList struct {
	Items []RegistryConfiguration `yaml:"registries"`
}

func clearRegistries() {
	registryLock.Lock()
	registries = make(map[string]*RegistryEndpoint)
	registryLock.Unlock()
}

// LoadRegistryConfiguration loads a YAML-formatted registry configuration from
// a given file at path.
func LoadRegistryConfiguration(path string, clear bool) error {
	registryBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	registryList, err := ParseRegistryConfiguration(string(registryBytes))
	if err != nil {
		return err
	}

	if clear {
		clearRegistries()
	}

	haveDefault := false

	for _, reg := range registryList.Items {
		tagSortMode := TagListSortFromString(reg.TagSortMode)
		if tagSortMode != TagListSortUnsorted {
			log.Warnf("Registry %s has tag sort mode set to %s, meta data retrieval will be disabled for this registry.", reg.ApiURL, tagSortMode)
		}
		ep := NewRegistryEndpoint(reg.Prefix, reg.Name, reg.ApiURL, reg.Credentials, reg.DefaultNS, reg.Insecure, tagSortMode, reg.Limit, reg.CredsExpire)
		if reg.IsDefault {
			if haveDefault {
				dep := GetDefaultRegistry()
				if dep == nil {
					panic("unexpected: default registry should be set, but is not")
				}
				return fmt.Errorf("cannot set registry %s as default - only one default registry allowed, currently set to %s", ep.RegistryPrefix, dep.RegistryPrefix)
			}
		}

		if err := AddRegistryEndpoint(ep); err != nil {
			return err
		}

		if reg.IsDefault {
			SetDefaultRegistry(ep)
			haveDefault = true
		}
	}

	log.Infof("Loaded %d registry configurations from %s", len(registryList.Items), path)
	return nil
}

// Parses a registry configuration from a YAML input string and returns a list
// of registries.
func ParseRegistryConfiguration(yamlSource string) (RegistryList, error) {
	var regList RegistryList
	var defaultPrefixFound = ""
	err := yaml.UnmarshalStrict([]byte(yamlSource), &regList)
	if err != nil {
		return RegistryList{}, err
	}

	// validate the parsed list
	for _, registry := range regList.Items {
		if registry.Name == "" {
			err = fmt.Errorf("registry name is missing for entry %v", registry)
		} else if registry.ApiURL == "" {
			err = fmt.Errorf("API URL must be specified for registry %s", registry.Name)
		} else if registry.Prefix == "" {
			if defaultPrefixFound != "" {
				err = fmt.Errorf("there must be only one default registry (already is %s), %s needs a prefix", defaultPrefixFound, registry.Name)
			} else {
				defaultPrefixFound = registry.Name
			}
		}

		if err == nil {
			if tls := TagListSortFromString(registry.TagSortMode); tls == TagListSortUnknown {
				err = fmt.Errorf("unknown tag sort mode for registry %s: %s", registry.Name, registry.TagSortMode)
			}
		}
	}

	if err != nil {
		return RegistryList{}, err
	}

	return regList, nil
}

// RestRestoreDefaultRegistryConfiguration restores the registry configuration
// to the default values.
func RestoreDefaultRegistryConfiguration() {
	registryLock.Lock()
	defer registryLock.Unlock()
	defaultRegistry = nil
	registries = make(map[string]*RegistryEndpoint)
	for k, v := range registryTweaks {
		registries[k] = v.DeepCopy()
		if v.IsDefault {
			SetDefaultRegistry(registries[k])
		}
	}
}
