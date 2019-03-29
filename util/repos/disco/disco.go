package disco

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/util/kustomize"
	"github.com/argoproj/argo-cd/util/repos/api"
)

func FindAppCfgs(root string) (map[api.AppPath]api.AppType, error) {

	appCfgs := make(map[api.AppPath]api.AppType)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {

		_, file := filepath.Split(path)
		if file == ".git" {
			return filepath.SkipDir
		}

		if info.IsDir() {

			log.WithFields(log.Fields{"path": path}).Debug()

			appType, err := GetAppType(path)
			if err != nil {
				return err
			}
			if appType != "" {
				appPath := strings.Trim(strings.TrimPrefix(path, root), "/")
				if _, ok := appCfgs[appPath]; !ok {
					appCfgs[appPath] = appType
				}
			}
		}
		return err
	})

	log.WithFields(log.Fields{"appCfgs": appCfgs}).Info("found appCfgs")

	return appCfgs, err

}

func GetAppType(dir string) (api.AppType, error) {

	infos, err := ioutil.ReadDir(dir)

	if err != nil {
		return "", err
	}

	for _, info := range infos {
		if !info.IsDir() {
			if strings.HasSuffix(info.Name(), "app.yaml") {
				return api.KsonnetAppType, nil
			} else if info.Name() == "Chart.yaml" {
				return api.HelmAppType, nil
			} else if kustomize.IsKustomization(info.Name()) {
				return api.KustomizeAppType, nil
			}
		}
	}

	return "", nil
}
