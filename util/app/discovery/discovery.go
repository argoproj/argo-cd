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

func Discover(ctx context.Context, repoPath string, enableGenerateManifests map[string]bool) (map[string]string, error) {
	apps := make(map[string]string)

	// Check if it is CMP
	conn, _, err := DetectConfigManagementPlugin(ctx, repoPath)
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

func AppType(ctx context.Context, path string, enableGenerateManifests map[string]bool) (string, error) {
	apps, err := Discover(ctx, path, enableGenerateManifests)
	if err != nil {
		return "", err
	}
	appType, ok := apps["."]
	if ok {
		return appType, nil
	}
	return "Directory", nil
}

// 1. list all plugins in /plugins folder
// 2. foreach plugin setup connection with respective cmp-server
// 3. check isSupported(path)?
// 4.a if no then close connection
// 4.b if yes then return conn for detected plugin
func DetectConfigManagementPlugin(ctx context.Context, repoPath string) (io.Closer, pluginclient.ConfigManagementPluginServiceClient, error) {
	var conn io.Closer
	var cmpClient pluginclient.ConfigManagementPluginServiceClient

	pluginSockFilePath := common.GetPluginSockFilePath()
	log.Debugf("pluginSockFilePath is: %s", pluginSockFilePath)

	files, err := os.ReadDir(pluginSockFilePath)
	if err != nil {
		return nil, nil, err
	}

	var connFound bool
	for _, file := range files {
		if file.Type() == os.ModeSocket {
			address := fmt.Sprintf("%s/%s", strings.TrimRight(pluginSockFilePath, "/"), file.Name())
			cmpclientset := pluginclient.NewConfigManagementPluginClientSet(address)

			conn, cmpClient, err = cmpclientset.NewConfigManagementPluginClient()
			if err != nil {
				log.Errorf("error dialing to cmp-server for plugin %s, %v", file.Name(), err)
				continue
			}

			isSupported, err := matchRepositoryCMP(ctx, repoPath, cmpClient)
			if err != nil {
				log.Errorf("repository %s is not the match because %v", repoPath, err)
				continue
			}

			if !isSupported {
				log.Debugf("Reponse from socket file %s is not supported", file.Name())
				io.Close(conn)
			} else {
				connFound = true
				break
			}
		}
	}

	if !connFound {
		return nil, nil, fmt.Errorf("Couldn't find cmp-server plugin supporting repository %s", repoPath)
	}
	return conn, cmpClient, err
}

// matchRepositoryCMP will send the repoPath to the cmp-server. The cmp-server will
// inspect the files and return true if the repo is supported for manifest generation.
// Will return false otherwise.
func matchRepositoryCMP(ctx context.Context, repoPath string, client pluginclient.ConfigManagementPluginServiceClient) (bool, error) {
	matchRepoStream, err := client.MatchRepository(ctx, grpc_retry.Disable())
	if err != nil {
		return false, fmt.Errorf("error getting stream client: %s", err)
	}

	err = cmp.SendRepoStream(ctx, repoPath, repoPath, matchRepoStream, []string{})
	if err != nil {
		return false, fmt.Errorf("error sending stream: %s", err)
	}
	resp, err := matchRepoStream.CloseAndRecv()
	if err != nil {
		return false, fmt.Errorf("error receiving stream response: %s", err)
	}
	return resp.GetIsSupported(), nil
}
