package discovery

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/argoproj/argo-cd/v2/util/io/files"

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

func Discover(ctx context.Context, repoPath string, enableGenerateManifests map[string]bool, tarExcludedGlobs []string) (map[string]string, error) {
	apps := make(map[string]string)

	// Check if it is CMP
	conn, _, err := DetectConfigManagementPlugin(ctx, repoPath, "", []string{}, tarExcludedGlobs)
	if err == nil {
		// Found CMP
		io.Close(conn)

		apps["."] = string(v1alpha1.ApplicationSourceTypePlugin)
		return apps, nil
	}

	err = filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		dir, err := filepath.Rel(repoPath, filepath.Dir(path))
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

func AppType(ctx context.Context, path string, enableGenerateManifests map[string]bool, tarExcludedGlobs []string) (string, error) {
	apps, err := Discover(ctx, path, enableGenerateManifests, tarExcludedGlobs)
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

func DetectConfigManagementPlugin(ctx context.Context, repoPath, pluginName string, env []string, tarExcludedGlobs []string) (io.Closer, pluginclient.ConfigManagementPluginServiceClient, error) {
	var conn io.Closer
	var cmpClient pluginclient.ConfigManagementPluginServiceClient
	var connFound bool

	pluginSockFilePath := common.GetPluginSockFilePath()
	log.WithFields(log.Fields{
		common.SecurityField:    common.SecurityLow,
		common.SecurityCWEField: 775,
	}).Debugf("pluginSockFilePath is: %s", pluginSockFilePath)

	if pluginName != "" {
		// check if the given plugin supports the repo
		conn, cmpClient, connFound = cmpSupports(ctx, pluginSockFilePath, repoPath, fmt.Sprintf("%v.sock", pluginName), env, tarExcludedGlobs)
		if !connFound {
			return nil, nil, fmt.Errorf("couldn't find cmp-server plugin with name %v supporting the given repository", pluginName)
		}
	} else {
		fileList, err := os.ReadDir(pluginSockFilePath)
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to list all plugins in dir, error=%w", err)
		}
		for _, file := range fileList {
			if file.Type() == os.ModeSocket {
				conn, cmpClient, connFound = cmpSupports(ctx, pluginSockFilePath, repoPath, file.Name(), env, tarExcludedGlobs)
				if connFound {
					break
				}
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
func matchRepositoryCMP(ctx context.Context, repoPath string, client pluginclient.ConfigManagementPluginServiceClient, env []string, tarExcludedGlobs []string) (bool, error) {
	matchRepoStream, err := client.MatchRepository(ctx, grpc_retry.Disable())
	if err != nil {
		return false, fmt.Errorf("error getting stream client: %s", err)
	}

	err = cmp.SendRepoStream(ctx, repoPath, repoPath, matchRepoStream, env, tarExcludedGlobs)
	if err != nil {
		return false, fmt.Errorf("error sending stream: %s", err)
	}
	resp, err := matchRepoStream.CloseAndRecv()
	if err != nil {
		return false, fmt.Errorf("error receiving stream response: %s", err)
	}
	return resp.GetIsSupported(), nil
}

func cmpSupports(ctx context.Context, pluginSockFilePath, repoPath, fileName string, env []string, tarExcludedGlobs []string) (io.Closer, pluginclient.ConfigManagementPluginServiceClient, bool) {
	address := filepath.Join(pluginSockFilePath, fileName)
	if !files.Inbound(address, pluginSockFilePath) {
		log.Errorf("invalid socket file path, %v is outside plugin socket dir %v", fileName, pluginSockFilePath)
		return nil, nil, false
	}

	cmpclientset := pluginclient.NewConfigManagementPluginClientSet(address)

	conn, cmpClient, err := cmpclientset.NewConfigManagementPluginClient()
	if err != nil {
		log.WithFields(log.Fields{
			common.SecurityField:    common.SecurityMedium,
			common.SecurityCWEField: 775,
		}).Errorf("error dialing to cmp-server for plugin %s, %v", fileName, err)
		return nil, nil, false
	}

	isSupported, err := matchRepositoryCMP(ctx, repoPath, cmpClient, env, tarExcludedGlobs)
	if err != nil {
		log.WithFields(log.Fields{
			common.SecurityField:    common.SecurityMedium,
			common.SecurityCWEField: 775,
		}).Errorf("repository %s is not the match because %v", repoPath, err)
		return nil, nil, false
	}

	if !isSupported {
		log.WithFields(log.Fields{
			common.SecurityField:    common.SecurityLow,
			common.SecurityCWEField: 775,
		}).Debugf("Reponse from socket file %s is not supported", fileName)
		io.Close(conn)
		return nil, nil, false
	}
	return conn, cmpClient, true
}
