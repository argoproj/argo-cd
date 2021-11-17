package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/mattn/go-zglob"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/cmpserver/apiclient"
	executil "github.com/argoproj/argo-cd/v2/util/exec"
)

// Service implements ConfigManagementPluginService interface
type Service struct {
	initConstants CMPServerInitConstants
}

type CMPServerInitConstants struct {
	PluginConfig PluginConfig
}

// NewService returns a new instance of the ConfigManagementPluginService
func NewService(initConstants CMPServerInitConstants) *Service {
	return &Service{
		initConstants: initConstants,
	}
}

func runCommand(command Command, path string, env []string) (string, error) {
	if len(command.Command) == 0 {
		return "", fmt.Errorf("Command is empty")
	}
	cmd := exec.Command(command.Command[0], append(command.Command[1:], command.Args...)...)
	cmd.Env = env
	cmd.Dir = path
	return executil.Run(cmd)
}

// Environ returns a list of environment variables in name=value format from a list of variables
func environ(envVars []*apiclient.EnvEntry) []string {
	var environ []string
	for _, item := range envVars {
		if item != nil && item.Name != "" && item.Value != "" {
			environ = append(environ, fmt.Sprintf("%s=%s", item.Name, item.Value))
		}
	}
	return environ
}

// GenerateManifest runs generate command from plugin config file and returns generated manifest files
func (s *Service) GenerateManifest(ctx context.Context, q *apiclient.ManifestRequest) (*apiclient.ManifestResponse, error) {
	config := s.initConstants.PluginConfig

	env := append(os.Environ(), environ(q.Env)...)
	if len(config.Spec.Init.Command) > 0 {
		_, err := runCommand(config.Spec.Init, q.AppPath, env)
		if err != nil {
			return &apiclient.ManifestResponse{}, err
		}
	}

	out, err := runCommand(config.Spec.Generate, q.AppPath, env)
	if err != nil {
		return &apiclient.ManifestResponse{}, err
	}

	manifests, err := kube.SplitYAMLToString([]byte(out))
	if err != nil {
		return &apiclient.ManifestResponse{}, err
	}

	return &apiclient.ManifestResponse{
		Manifests: manifests,
	}, err
}

// MatchRepository checks whether the application repository type is supported by config management plugin server
func (s *Service) MatchRepository(ctx context.Context, q *apiclient.RepositoryRequest) (*apiclient.RepositoryResponse, error) {
	var repoResponse apiclient.RepositoryResponse
	config := s.initConstants.PluginConfig
	if config.Spec.Discover.FileName != "" {
		log.Debugf("config.Spec.Discover.FileName is provided")
		pattern := strings.TrimSuffix(q.Path, "/") + "/" + strings.TrimPrefix(config.Spec.Discover.FileName, "/")
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) == 0 {
			log.Debugf("Could not find match for pattern %s. Error is %v.", pattern, err)
			return &repoResponse, err
		} else if len(matches) > 0 {
			repoResponse.IsSupported = true
			return &repoResponse, nil
		}
	}

	if config.Spec.Discover.Find.Glob != "" {
		log.Debugf("config.Spec.Discover.Find.Glob is provided")
		pattern := strings.TrimSuffix(q.Path, "/") + "/" + strings.TrimPrefix(config.Spec.Discover.Find.Glob, "/")
		// filepath.Glob doesn't have '**' support hence selecting third-party lib
		// https://github.com/golang/go/issues/11862
		matches, err := zglob.Glob(pattern)
		if err != nil || len(matches) == 0 {
			log.Debugf("Could not find match for pattern %s. Error is %v.", pattern, err)
			return &repoResponse, err
		} else if len(matches) > 0 {
			repoResponse.IsSupported = true
			return &repoResponse, nil
		}
	}

	log.Debugf("Going to try runCommand.")
	find, err := runCommand(config.Spec.Discover.Find.Command, q.Path, os.Environ())
	if err != nil {
		return &repoResponse, err
	}

	var isSupported bool
	if find != "" {
		isSupported = true
	}
	return &apiclient.RepositoryResponse{
		IsSupported: isSupported,
	}, nil
}

// GetPluginConfig returns plugin config
func (s *Service) GetPluginConfig(ctx context.Context, q *apiclient.ConfigRequest) (*apiclient.ConfigResponse, error) {
	config := s.initConstants.PluginConfig
	return &apiclient.ConfigResponse{
		AllowConcurrency: config.Spec.AllowConcurrency,
		LockRepo:         config.Spec.LockRepo,
	}, nil
}
