package disco

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/util/kustomize"
)

func FindAppTemplates(root string) (map[string]string, error) {

	appTemplates := make(map[string]string)
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
				if _, ok := appTemplates[appPath]; !ok {
					appTemplates[appPath] = appType
				}
			}
		}
		return err
	})

	log.WithFields(log.Fields{"appTemplates": appTemplates}).Info("found appTemplates")

	return appTemplates, err

}

func GetAppType(dir string) (string, error) {

	infos, err := ioutil.ReadDir(dir)

	if err != nil {
		return "", err
	}

	for _, info := range infos {
		if !info.IsDir() {
			if strings.HasSuffix(info.Name(), "app.yaml") {
				return "ksonnet", nil
			} else if info.Name() == "Chart.yaml" {
				return "helm", nil
			} else if kustomize.IsKustomization(info.Name()) {
				return "kustomize", nil
			}
		}
	}

	return "", nil
}
