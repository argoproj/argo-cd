package discovery

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	log "github.com/sirupsen/logrus"

	pluginclient "github.com/argoproj/argo-cd/v2/cmpserver/apiclient"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/cmp"
	"github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/kustomize"
)

type Discoverer struct {
	plugins *plugins
}

// NewWithServices provides a fully functional discoverer, which is
// expected to be long lived as it runs an informer watching for cmp
// plugins as services
func NewWithServices() *Discoverer {
	return &Discoverer{
		plugins: newPluginService(),
	}
}

// NewNoServices provides a discoverer which will not watch services
// but otherwise works fully. For use in the CLI and testing.
func NewNoServices() *Discoverer {
	return &Discoverer{
		plugins: noServicesPluginService(),
	}
}

func GetPluginNames(discoverer *Discoverer) ([]string, error) {
	plugins, err := discoverer.plugins.getAllPlugins()
	pluginNames := make([]string, len(plugins))
	if err != nil {
		return pluginNames, err
	}
	for i := range plugins {
		pluginNames[i] = plugins[i].name
	}
	return pluginNames, nil
}

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

func Discover(ctx context.Context, appPath, repoPath string, enableGenerateManifests map[string]bool, tarExcludedGlobs []string) (map[string]string, error) {
	apps := make(map[string]string)

	// Check if it is CMP. When running from the CLI this won't work, so we don't try.
	conn, _, err := DetectConfigManagementPlugin(ctx, discoverer, appPath, repoPath, "", []string{}, tarExcludedGlobs)
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
		return nil
	})
	return apps, err
}

func AppType(ctx context.Context, discoverer *Discoverer, appPath, repoPath string, enableGenerateManifests map[string]bool, tarExcludedGlobs []string, env []string) (string, error) {
	apps, err := Discover(ctx, discoverer, appPath, repoPath, enableGenerateManifests, tarExcludedGlobs, env)
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

func DetectConfigManagementPlugin(ctx context.Context, discoverer *Discoverer, appPath, repoPath, pluginName string, env []string, tarExcludedGlobs []string) (io.Closer, pluginclient.ConfigManagementPluginServiceClient, error) {
	var conn io.Closer
	var cmpClient pluginclient.ConfigManagementPluginServiceClient
	var connFound bool

	if pluginName != "" {
		// check if the given plugin supports the repo
		plugin, err := discoverer.plugins.getPluginByName(pluginName)
		if plugin == nil {
			return nil, nil, fmt.Errorf("couldn't find cmp-server plugin with name %q supporting the given repository", pluginName)
		}
		if err != nil {
			return nil, nil, fmt.Errorf("failed to find cmp-server plugin with name %q, error=%w", pluginName, err)
		}
		conn, cmpClient, connFound = cmpSupports(ctx, appPath, repoPath, plugin, env, tarExcludedGlobs, true)
		if !connFound {
			return nil, nil, fmt.Errorf("named cmp service %q not reachable", pluginName)
		}
	} else {
		plugins, err := discoverer.plugins.getAllPlugins()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list all plugins, error=%w", err)
		}
		for _, plugin := range plugins {
			conn, cmpClient, connFound = cmpSupports(ctx, appPath, repoPath, plugin, env, tarExcludedGlobs, false)
			if connFound {
				break
			}
		}
		if !connFound {
			return nil, nil, fmt.Errorf("could not find plugin supporting the given repository")
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

func cmpSupports(ctx context.Context, appPath, repoPath string, plugin *plugin, env []string, tarExcludedGlobs []string, namedPlugin bool) (io.Closer, pluginclient.ConfigManagementPluginServiceClient, bool) {
	cmpclientset := pluginclient.NewConfigManagementPluginClientSet(plugin.address, plugin.pluginType.clientSetType())

	conn, cmpClient, err := cmpclientset.NewConfigManagementPluginClient()
	if err != nil {
		log.WithFields(log.Fields{
			common.SecurityField:    common.SecurityMedium,
			common.SecurityCWEField: common.SecurityCWEMissingReleaseOfFileDescriptor,
		}).Errorf("error dialing to cmp-server for plugin %s, %v", plugin.address, err)
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
		}).Debugf("Response from socket file %s does not support %v", plugin.address, repoPath)
		io.Close(conn)
		return nil, nil, false
	}
	return conn, cmpClient, true
}
