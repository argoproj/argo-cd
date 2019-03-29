package helm

import (
	"errors"
	"net/http"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/argoproj/argo-cd/util/repos/api"
)

type repoCfg struct {
	workDir                       string
	cmd                           *cmd
	url, name, username, password string
	caData, certData, keyData     []byte
}

func (c repoCfg) LockKey() string {
	return c.workDir
}

type entry struct {
	Version string
}

type index struct {
	Entries map[string][]entry
}

func (c repoCfg) getIndex() (*index, error) {
	start := time.Now()

	resp, err := http.Get(c.url + "/index.yaml")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return nil, errors.New("failed to get index: " + resp.Status)
	}

	index := &index{}
	err = yaml.NewDecoder(resp.Body).Decode(index)

	log.WithFields(log.Fields{"seconds": time.Since(start).Seconds()}).Info("took to get index")
	return index, err
}

func (c repoCfg) FindAppCfgs(_ api.RepoRevision) (map[api.AppPath]api.AppType, error) {

	index, err := c.getIndex()
	if err != nil {
		return nil, err
	}

	cfgs := make(map[api.AppPath]api.AppType)

	for chartName := range index.Entries {
		cfgs[chartName] = api.HelmAppType
	}

	return cfgs, nil
}

func (c repoCfg) repoAdd() (string, error) {
	return c.cmd.repoAdd(c.name, c.url, repoAddOpts{
		Username: c.username, Password: c.password,
		CAData: c.caData, CertData: c.certData, KeyData: c.keyData,
	})
}

func (c repoCfg) GetAppCfg(path api.AppPath, revision api.AppRevision) (string, api.AppType, error) {

	if revision == "" {
		return "", "", errors.New("revision must be resolved")
	}

	err := c.checkKnownChart(path)
	if err != nil {
		return "", "", err
	}

	log.WithFields(log.Fields{"chartName": path}).Debug("chart name")

	_, err = c.cmd.fetch(c.name, path, fetchOpts{
		Version: revision, Destination: ".",
	})

	return filepath.Join(c.workDir, path), api.HelmAppType, err
}

func (c repoCfg) checkKnownChart(chartName string) error {
	knownChart, err := c.isKnownChart(chartName)
	if err != nil {
		return err
	}
	if !knownChart {
		return errors.New("unknown chart " + chartName)
	}
	return nil
}

func (c repoCfg) isKnownChart(chartName string) (bool, error) {

	index, err := c.getIndex()
	if err != nil {
		return false, err
	}

	_, ok := index.Entries[chartName]

	return ok, nil
}

func (c repoCfg) ResolveRevision(path api.AppPath, revision api.AppRevision) (string, error) {

	if revision != "" {
		return revision, nil
	}

	index, err := c.getIndex()
	if err != nil {
		return "", err
	}

	for chartName := range index.Entries {
		if chartName == path {
			return index.Entries[chartName][0].Version, nil
		}
	}
	return "", errors.New("failed to find chart " + path)
}

func (c repoCfg) GetUrl() string {
	return c.url
}
