package disco

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/argoproj/argo-cd/util/kustomize"
)

func Discover(path string) (apps map[string]string, err error) {
	err = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		dir := filepath.Dir(info.Name())
		base := filepath.Base(info.Name())
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
	return
}

func AppType(path string) string {
	if pathExists(path, "app.yaml") {
		return "Ksonnet"
	}
	if pathExists(path, "Chart.yaml") {
		return "Helm"
	}
	for _, f := range kustomize.KustomizationNames {
		if pathExists(path, f) {
			return "Kustomize"
		}
	}
	return "Directory"
}

// pathExists reports whether the file or directory at the named concatenation of paths exists.
func pathExists(ss ...string) bool {
	name := filepath.Join(ss...)
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
