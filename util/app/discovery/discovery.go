package discovery

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/golang/protobuf/ptypes/empty"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	log "github.com/sirupsen/logrus"

	pluginclient "github.com/argoproj/argo-cd/v2/cmpserver/apiclient"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/cmp"
	"github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/kustomize"
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
	_, closer, err := DetectConfigManagementPlugin(ctx, appPath, repoPath, "", env, tarExcludedGlobs)
	if err == nil {
		// Found CMP
		io.Close(closer)

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

// DetectConfigManagementPlugin will return a client for the CMP plugin if a plugin is found supporting the given
// repository. It will also return a closer for the connection. If a compatible plugin is not found, it wil return an
// error.
func DetectConfigManagementPlugin(ctx context.Context, appPath, repoPath, pluginName string, env, tarExcludedGlobs []string) (pluginclient.ConfigManagementPluginServiceClient, io.Closer, error) {
	var cmpClient pluginclient.ConfigManagementPluginServiceClient
	var closer io.Closer

	pluginSockFilePath := common.GetPluginSockFilePath()
	log.WithFields(log.Fields{
		common.SecurityField:    common.SecurityLow,
		common.SecurityCWEField: common.SecurityCWEMissingReleaseOfFileDescriptor,
	}).Debugf("pluginSockFilePath is: %s", pluginSockFilePath)

	if pluginName != "" {
		c := newSocketCMPClientConstructorForPluginName(pluginSockFilePath, pluginName)
		cmpClient, closer = namedCMPSupports(ctx, c, appPath, repoPath, env, tarExcludedGlobs)
		if cmpClient == nil {
			return nil, nil, fmt.Errorf("couldn't find cmp-server plugin with name %q supporting the given repository", pluginName)
		}
	} else {
		fileList, err := os.ReadDir(pluginSockFilePath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list all plugins in dir: %w", err)
		}
		for _, file := range fileList {
			if file.Type() == os.ModeSocket {
				c := newSocketCMPClientConstructorForPath(pluginSockFilePath, file.Name())
				cmpClient, closer = unnamedCMPSupports(ctx, c, appPath, repoPath, env, tarExcludedGlobs)
				if cmpClient != nil {
					break
				}
			}
		}
		if cmpClient == nil {
			return nil, nil, fmt.Errorf("could not find plugin supporting the given repository")
		}
	}
	return cmpClient, closer, nil
}

// namedCMPSupports will return a client for the named plugin if it supports the given repository. It will also return
// a closer for the connection.
func namedCMPSupports(ctx context.Context, c CMPClientConstructor, appPath, repoPath string, env, tarExcludedGlobs []string) (pluginclient.ConfigManagementPluginServiceClient, io.Closer) {
	cmpClient, closer, isDiscoveryConfigured, err := getClientAndConfig(ctx, c)
	if err != nil {
		log.Errorf("error getting client for plugin: %v", err)
		return nil, nil
	}
	if !isDiscoveryConfigured {
		// If discovery isn't configured, assume the CMP supports the plugin since it was explicitly named.
		return cmpClient, closer
	}
	usePlugin := cmpSupportsForClient(ctx, cmpClient, appPath, repoPath, env, tarExcludedGlobs)
	if !usePlugin {
		io.Close(closer)
		return nil, nil
	}
	return cmpClient, closer
}

// unnamedCMPSupports will return a client if the plugin supports the given repository. It will also return a closer for
// the connection.
func unnamedCMPSupports(ctx context.Context, c CMPClientConstructor, appPath, repoPath string, env, tarExcludedGlobs []string) (pluginclient.ConfigManagementPluginServiceClient, io.Closer) {
	cmpClient, closer, isDiscoveryConfigured, err := getClientAndConfig(ctx, c)
	if err != nil {
		log.Errorf("error getting client for plugin: %v", err)
		return nil, nil
	}
	if !isDiscoveryConfigured {
		// If discovery isn't configured, we can't assume the CMP supports the repo.
		return nil, nil
	}
	usePlugin := cmpSupportsForClient(ctx, cmpClient, appPath, repoPath, env, tarExcludedGlobs)
	if !usePlugin {
		io.Close(closer)
		return nil, nil
	}
	return cmpClient, closer
}

// CMPClientConstructor is an interface for creating a client for a CMP.
type CMPClientConstructor interface {
	// NewConfigManagementPluginClient returns a client for the CMP. It also returns a closer for the connection.
	NewConfigManagementPluginClient() (pluginclient.ConfigManagementPluginServiceClient, io.Closer, error)
}

// socketCMPClientConstructor is a CMPClientConstructor for a CMP that uses a socket file.
type socketCMPClientConstructor struct {
	pluginSockFilePath string
	filename           string
}

// newSocketCMPClientConstructorForPath returns a new socketCMPClientConstructor.
func newSocketCMPClientConstructorForPath(pluginSockFilePath, filename string) socketCMPClientConstructor {
	return socketCMPClientConstructor{
		pluginSockFilePath: pluginSockFilePath,
		filename:           filename,
	}
}

// newSocketCMPClientConstructorForPluginName returns a new socketCMPClientConstructor for the given plugin name.
func newSocketCMPClientConstructorForPluginName(pluginSockFilePath, pluginName string) socketCMPClientConstructor {
	return newSocketCMPClientConstructorForPath(pluginSockFilePath, pluginName+".sock")
}

// NewConfigManagementPluginClient returns a client for the CMP. It also returns a closer for the connection.
func (c socketCMPClientConstructor) NewConfigManagementPluginClient() (pluginclient.ConfigManagementPluginServiceClient, io.Closer, error) {
	absPluginSockFilePath, err := filepath.Abs(c.pluginSockFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting absolute path for plugin socket dir %q: %w", c.pluginSockFilePath, err)
	}
	address, err := securejoin.SecureJoin(absPluginSockFilePath, c.filename)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid socket file path, %v is outside plugin socket dir %v: %w", c.filename, c.pluginSockFilePath, err)
	}
	cmpclientset := pluginclient.NewConfigManagementPluginClientSet(address)
	conn, cmpClient, err := cmpclientset.NewConfigManagementPluginClient()
	if err != nil {
		return nil, nil, fmt.Errorf("error dialing to cmp-server for plugin: %w", err)
	}
	return cmpClient, conn, nil
}

// getClientAndConfig returns a client for the given filepath. It also returns a closer for the connection and
// a boolean indicating if the plugin has discovery configured.
func getClientAndConfig(ctx context.Context, c CMPClientConstructor) (pluginclient.ConfigManagementPluginServiceClient, io.Closer, bool, error) {
	cmpClient, closer, err := c.NewConfigManagementPluginClient()
	if err != nil {
		return nil, nil, false, fmt.Errorf("error getting client for plugin: %w", err)
	}
	cfg, err := cmpClient.CheckPluginConfiguration(ctx, &empty.Empty{})
	if err != nil {
		return nil, nil, false, fmt.Errorf("error checking plugin configuration: %w", err)
	}
	return cmpClient, closer, cfg.IsDiscoveryConfigured, nil
}

// cmpSupportsForClient will send the repoPath to the cmp-server. The cmp-server will
// inspect the files and return true if the repo is supported for manifest generation.
// Will return false otherwise.
func cmpSupportsForClient(ctx context.Context, cmpClient pluginclient.ConfigManagementPluginServiceClient, appPath string, repoPath string, env []string, tarExcludedGlobs []string) bool {
	isSupported, err := matchRepositoryCMP(ctx, appPath, repoPath, cmpClient, env, tarExcludedGlobs)
	if err != nil {
		log.WithFields(log.Fields{
			common.SecurityField:    common.SecurityMedium,
			common.SecurityCWEField: common.SecurityCWEMissingReleaseOfFileDescriptor,
		}).Errorf("repository %s is not the match because %v", repoPath, err)
		return false
	}

	if !isSupported {
		log.WithFields(log.Fields{
			common.SecurityField:    common.SecurityLow,
			common.SecurityCWEField: common.SecurityCWEMissingReleaseOfFileDescriptor,
		}).Debugf("Plugin does not support %v", repoPath)
		return false
	}
	return true
}

// matchRepositoryCMP will send the repoPath to the cmp-server. The cmp-server will inspect the files and return true if
// the repo is supported for manifest generation. Will return false otherwise.
func matchRepositoryCMP(ctx context.Context, appPath, repoPath string, client pluginclient.ConfigManagementPluginServiceClient, env, tarExcludedGlobs []string) (bool, error) {
	matchRepoStream, err := client.MatchRepository(ctx, grpc_retry.Disable())
	if err != nil {
		return false, fmt.Errorf("error getting stream client: %w", err)
	}

	err = cmp.SendRepoStream(ctx, appPath, repoPath, matchRepoStream, env, tarExcludedGlobs)
	if err != nil {
		return false, fmt.Errorf("error sending stream: %w", err)
	}
	resp, err := matchRepoStream.CloseAndRecv()
	if err != nil {
		return false, fmt.Errorf("error receiving stream response: %w", err)
	}
	return resp.GetIsSupported(), nil
}
