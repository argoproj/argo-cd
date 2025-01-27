package discovery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"
	"gopkg.in/yaml.v2"

	"github.com/argoproj/argo-cd/v3/util/io/files"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	log "github.com/sirupsen/logrus"

	pluginclient "github.com/argoproj/argo-cd/v3/cmpserver/apiclient"
	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/cmp"
	"github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/kustomize"
)

func IsManifestGenerationEnabled(sourceType v1alpha1.ApplicationSourceType, enableGenerateManifests map[string]bool) bool {
	if enableGenerateManifests == nil {
		return true
	}
	enabled, ok := enableGenerateManifests[string(sourceType)]
	if !ok {
		return true
	}
	return enabled
}

func Discover(ctx context.Context, appPath, repoPath string, enableGenerateManifests map[string]bool, tarExcludedGlobs []string, env []string) (map[string]string, error) {
	apps := make(map[string]string)

	// Check if it is CMP
	conn, _, err := DetectConfigManagementPlugin(ctx, appPath, repoPath, "", env, tarExcludedGlobs)
	if err == nil {
		// Found CMP
		io.Close(conn)

		apps["."] = string(v1alpha1.ApplicationSourceTypePlugin)
		return apps, nil
	}

	err = filepath.Walk(appPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		dir, err := filepath.Rel(appPath, filepath.Dir(path))
		if err != nil {
			return err
		}
		base := filepath.Base(path)
		if strings.HasSuffix(base, "Chart.yaml") && IsManifestGenerationEnabled(v1alpha1.ApplicationSourceTypeHelm, enableGenerateManifests) {
			apps[dir] = string(v1alpha1.ApplicationSourceTypeHelm)
		}
		if kustomize.IsKustomization(base) && IsManifestGenerationEnabled(v1alpha1.ApplicationSourceTypeKustomize, enableGenerateManifests) {
			apps[dir] = string(v1alpha1.ApplicationSourceTypeKustomize)
		}

		if IsManifestGenerationEnabled(v1alpha1.ApplicationSourceTypeDirectory, enableGenerateManifests) {
			isK8sManifestDir, err := IsK8sManifestDir(path)
			if err != nil || !isK8sManifestDir {
				// can happen on decode errors, EOF, etc.
				return nil
			}

			// handling for helm app templates/ directory, if yaml
			parentDir := filepath.Dir(dir) // apppath = ./testdata dir= baz3/templates
			if parentDir != "" && apps[parentDir] == string(v1alpha1.ApplicationSourceTypeHelm) {
				return nil
			}

			if (apps[dir] != string(v1alpha1.ApplicationSourceTypeKustomize)) && (apps[dir] != string(v1alpha1.ApplicationSourceTypeHelm)) {
				apps[dir] = string(v1alpha1.ApplicationSourceTypeDirectory)
			}
		}
		return nil
	})
	return apps, err
}

func AppType(ctx context.Context, appPath, repoPath string, enableGenerateManifests map[string]bool, tarExcludedGlobs []string, env []string) (string, error) {
	apps, err := Discover(ctx, appPath, repoPath, enableGenerateManifests, tarExcludedGlobs, env)
	if err != nil {
		return "", err
	}
	appType, ok := apps["."]
	if ok {
		return appType, nil
	}
	return "Directory", nil
}

// if pluginName is provided setup connection to that cmp-server
// else
// list all plugins in /plugins folder and foreach plugin
// check cmpSupports()
// if supported return conn for the cmp-server

func DetectConfigManagementPlugin(ctx context.Context, appPath, repoPath, pluginName string, env []string, tarExcludedGlobs []string) (io.Closer, pluginclient.ConfigManagementPluginServiceClient, error) {
	var conn io.Closer
	var cmpClient pluginclient.ConfigManagementPluginServiceClient
	var connFound bool

	pluginSockFilePath := common.GetPluginSockFilePath()
	log.WithFields(log.Fields{
		common.SecurityField:    common.SecurityLow,
		common.SecurityCWEField: common.SecurityCWEMissingReleaseOfFileDescriptor,
	}).Debugf("pluginSockFilePath is: %s", pluginSockFilePath)

	if pluginName != "" {
		// check if the given plugin supports the repo
		conn, cmpClient, connFound = cmpSupports(ctx, pluginSockFilePath, appPath, repoPath, fmt.Sprintf("%v.sock", pluginName), env, tarExcludedGlobs, true)
		if !connFound {
			return nil, nil, fmt.Errorf("couldn't find cmp-server plugin with name %q supporting the given repository", pluginName)
		}
	} else {
		fileList, err := os.ReadDir(pluginSockFilePath)
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to list all plugins in dir, error=%w", err)
		}
		for _, file := range fileList {
			if file.Type() == os.ModeSocket {
				conn, cmpClient, connFound = cmpSupports(ctx, pluginSockFilePath, appPath, repoPath, file.Name(), env, tarExcludedGlobs, false)
				if connFound {
					break
				}
			}
		}
		if !connFound {
			return nil, nil, errors.New("could not find plugin supporting the given repository")
		}
	}
	return conn, cmpClient, nil
}

// matchRepositoryCMP will send the repoPath to the cmp-server. The cmp-server will
// inspect the files and return true if the repo is supported for manifest generation.
// Will return false otherwise.
func matchRepositoryCMP(ctx context.Context, appPath, repoPath string, client pluginclient.ConfigManagementPluginServiceClient, env []string, tarExcludedGlobs []string) (bool, bool, error) {
	matchRepoStream, err := client.MatchRepository(ctx, grpc_retry.Disable())
	if err != nil {
		return false, false, fmt.Errorf("error getting stream client: %w", err)
	}

	err = cmp.SendRepoStream(ctx, appPath, repoPath, matchRepoStream, env, tarExcludedGlobs)
	if err != nil {
		return false, false, fmt.Errorf("error sending stream: %w", err)
	}
	resp, err := matchRepoStream.CloseAndRecv()
	if err != nil {
		return false, false, fmt.Errorf("error receiving stream response: %w", err)
	}
	return resp.GetIsSupported(), resp.GetIsDiscoveryEnabled(), nil
}

func cmpSupports(ctx context.Context, pluginSockFilePath, appPath, repoPath, fileName string, env []string, tarExcludedGlobs []string, namedPlugin bool) (io.Closer, pluginclient.ConfigManagementPluginServiceClient, bool) {
	absPluginSockFilePath, err := filepath.Abs(pluginSockFilePath)
	if err != nil {
		log.Errorf("error getting absolute path for plugin socket dir %v, %v", pluginSockFilePath, err)
		return nil, nil, false
	}
	address := filepath.Join(absPluginSockFilePath, fileName)
	if !files.Inbound(address, absPluginSockFilePath) {
		log.Errorf("invalid socket file path, %v is outside plugin socket dir %v", fileName, pluginSockFilePath)
		return nil, nil, false
	}

	cmpclientset := pluginclient.NewConfigManagementPluginClientSet(address)

	conn, cmpClient, err := cmpclientset.NewConfigManagementPluginClient()
	if err != nil {
		log.WithFields(log.Fields{
			common.SecurityField:    common.SecurityMedium,
			common.SecurityCWEField: common.SecurityCWEMissingReleaseOfFileDescriptor,
		}).Errorf("error dialing to cmp-server for plugin %s, %v", fileName, err)
		return nil, nil, false
	}

	cfg, err := cmpClient.CheckPluginConfiguration(ctx, &empty.Empty{})
	if err != nil {
		log.Errorf("error checking plugin configuration %s, %v", fileName, err)
		io.Close(conn)
		return nil, nil, false
	}

	if !cfg.IsDiscoveryConfigured {
		// If discovery isn't configured but the plugin is named, then the plugin supports the repo.
		if namedPlugin {
			return conn, cmpClient, true
		}
		io.Close(conn)
		return nil, nil, false
	}

	isSupported, isDiscoveryEnabled, err := matchRepositoryCMP(ctx, appPath, repoPath, cmpClient, env, tarExcludedGlobs)
	if err != nil {
		log.WithFields(log.Fields{
			common.SecurityField:    common.SecurityMedium,
			common.SecurityCWEField: common.SecurityCWEMissingReleaseOfFileDescriptor,
		}).Errorf("repository %s is not the match because %v", repoPath, err)
		io.Close(conn)
		return nil, nil, false
	}

	if !isSupported {
		// if discovery is not set and the plugin name is specified, let app use the plugin
		if !isDiscoveryEnabled && namedPlugin {
			return conn, cmpClient, true
		}
		log.WithFields(log.Fields{
			common.SecurityField:    common.SecurityLow,
			common.SecurityCWEField: common.SecurityCWEMissingReleaseOfFileDescriptor,
		}).Debugf("Response from socket file %s does not support %v", fileName, repoPath)
		io.Close(conn)
		return nil, nil, false
	}
	return conn, cmpClient, true
}

// IsK8sManifestDir will determine if given file is a plain k8s manifest file
func IsK8sManifestDir(filePath string) (bool, error) {
	if !strings.HasSuffix(filePath, ".yaml") && !strings.HasSuffix(filePath, ".yml") {
		return false, nil
	}
	f, err := os.Open(filePath)
	if err != nil {
		log.Errorf("failed to open %s: %v", filePath, err)
		return false, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer func(f *os.File) {
		err = f.Close()
		if err != nil {
			return
		}
	}(f)
	decoder := yaml.NewDecoder(f)
	obj := make(map[string]interface{})
	err = decoder.Decode(&obj)
	if err != nil {
		// this can occur on broken yaml files too
		return false, fmt.Errorf("failed to decode file %s: %w", filePath, err)
	}

	if obj["kind"] != nil && obj["apiVersion"] != nil && obj["metadata"] != nil {
		return true, nil
	}
	return false, nil
}
