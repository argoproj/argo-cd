package git

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/argoproj/argo-cd/util/repos/api"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/util/kustomize"
)

type repoCfg struct {
	client client
}

func (c repoCfg) LockKey() string {
	return c.client.getRoot()
}

func (c repoCfg) makeReady(revision string) error {
	err := c.client.init()
	if err != nil {
		return err
	}

	err = c.client.fetch()
	if err != nil {
		return err
	}

	return c.client.checkout(revision)
}

func (c repoCfg) ListAppCfgs(revision api.RepoRevision) (map[api.AppPath]api.AppType, error) {
	appCfgs := make(map[api.AppPath]api.AppType)

	err := c.makeReady(revision)
	if err != nil {
		return nil, err
	}

	err = filepath.Walk(c.client.getRoot(), func(path string, info os.FileInfo, err error) error {

		_, file := filepath.Split(path)
		if file == ".git" {
			return filepath.SkipDir
		}

		if info.IsDir() {

			log.WithFields(log.Fields{"path": path}).Debug()

			appType, err := getAppType(path)
			if err != nil {
				return err
			}
			if appType != "" {
				appPath := strings.Trim(strings.TrimPrefix(path, c.client.getRoot()), "/")
				if _, ok := appCfgs[appPath]; !ok {
					appCfgs[appPath] = appType
				}
			}
		}
		return err
	})

	return appCfgs, err
}

func getAppType(dir string) (api.AppType, error) {

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

func (c repoCfg) GetAppCfg(path, revision string) (string, api.AppType, error) {
	if revision == "" {
		return "", "", errors.New("must resolved revision")
	}

	err := c.makeReady(revision)
	if err != nil {
		return "", "", err
	}

	dir := filepath.Join(c.client.getRoot(), path)

	appType, err := getAppType(dir)

	return dir, appType, err
}

func (c repoCfg) ResolveRevision(path api.AppPath, revision api.AppRevision) (string, error) {
	return c.client.lsRemote(revision)
}
