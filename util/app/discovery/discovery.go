package discovery

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/argoproj/argo-cd/v2/util/kustomize"
)

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
		if kustomize.IsKustomization(base) {
			apps[dir] = "Kustomize"
		}
		return nil
	})
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
