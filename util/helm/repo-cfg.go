package helm

import (
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

	_, err := c.repoAdd()
	if err != nil {
		return "", "", err
	}

	_, err = c.cmd.repoUpdate()
	if err != nil {
		return "", "", err
	}

	err = c.checkKnownChart(path)
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

func (f RepoCfgFactory) IsResolvedRevision(revision string) bool {
	return revision != ""
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

func (f RepoCfgFactory) NewRepoCfg(
	url, name, username, password string,
	caData, certData, keyData []byte) (api.RepoCfg, error) {
	url = f.NormalizeURL(url)
	if name == "" {
		return nil, errors.New("must name repo")
	}

	workDir, err := ioutil.TempDir(os.TempDir(), strings.ReplaceAll(url, "/", "_"))
	if err != nil {
		return nil, err
	}

	cmd, err := newCmd(workDir)
	if err != nil {
		return nil, err
	}
	_, err = cmd.init()
	if err != nil {
		return nil, err
	}

	cfg := repoCfg{
		workDir:  workDir,
		cmd:      cmd,
		url:      url,
		name:     name,
		username: username,
		password: password,
		caData:   caData,
		certData: certData,
		keyData:  keyData,
	}

	_, err = cfg.getIndex()

	return cfg, err
}
