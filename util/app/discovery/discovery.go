package discovery

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/argoproj/argo-cd/util/plugins"
)

type App struct {
	Type string
	Path string
}

func Discover(root string) (map[string]string, error) {
	apps := make(map[string]string)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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
			apps[filepath.Dir(dir)] = "Ksonnet"
		}
		if strings.HasSuffix(base, "Chart.yaml") {
			apps[dir] = "Helm"
		}
		return nil
	})
	for _, plugin := range plugins.Plugins() {
		pluginApps, err := plugin.Discover(root)
		if err != nil {
			return nil, err
		}
		for _, path := range pluginApps {
			apps[path] = "Plugin"
		}
	}
	return apps, err
}

func AppType(path string) (string, error) {
	apps, err := Discover(path)
	if err != nil {
		return "", err
	}
	appType, ok := apps["."]
	if ok {
		return appType, nil
	}
	return "Directory", nil
}
