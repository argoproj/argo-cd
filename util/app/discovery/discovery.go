package discovery

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	log "github.com/sirupsen/logrus"

	pluginclient "github.com/argoproj/argo-cd/v2/cmpserver/apiclient"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/kustomize"
)

func Discover(ctx context.Context, root string) (map[string]string, error) {
	apps := make(map[string]string)

	// Check if it is CMP
	conn, _, err := DetectConfigManagementPlugin(ctx, root)
	if err == nil {
		// Found CMP
		io.Close(conn)

		apps["."] = string(v1alpha1.ApplicationSourceTypePlugin)
		return apps, nil
	}

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		dir, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		base := filepath.Base(path)
		if base == "params.libsonnet" && strings.HasSuffix(dir, "components") {
			apps[filepath.Dir(dir)] = string(v1alpha1.ApplicationSourceTypeKsonnet)
		}
		if strings.HasSuffix(base, "Chart.yaml") {
			apps[dir] = string(v1alpha1.ApplicationSourceTypeHelm)
		}
		if kustomize.IsKustomization(base) {
			apps[dir] = string(v1alpha1.ApplicationSourceTypeKustomize)
		}
		return nil
	})
	return apps, err
}

func AppType(ctx context.Context, path string) (string, error) {
	apps, err := Discover(ctx, path)
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
func DetectConfigManagementPlugin(ctx context.Context, appPath string) (io.Closer, pluginclient.ConfigManagementPluginServiceClient, error) {
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

			resp, err := cmpClient.MatchRepository(ctx, &pluginclient.RepositoryRequest{Path: appPath})
			if err != nil {
				log.Errorf("repository %s is not the match because %v", appPath, err)
				continue
			}

			if !resp.IsSupported {
				log.Debugf("Reponse from socket file %s is not supported", file.Name())
				io.Close(conn)
			} else {
				connFound = true
				break
			}
		}
	}

	if !connFound {
		return nil, nil, fmt.Errorf("Couldn't find cmp-server plugin supporting repository %s", appPath)
	}
	return conn, cmpClient, err
}
