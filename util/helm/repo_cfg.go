package helm

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

var indexCache = cache.New(5*time.Minute, 5*time.Minute)

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

	cachedIndex, found := indexCache.Get(c.url)
	if found {
		log.WithFields(log.Fields{"url": c.url}).Debug("index cache hit")
		i := cachedIndex.(index)
		return &i, nil
	}

	start := time.Now()

	resp, err := http.Get(strings.TrimSuffix(c.url, "/") + "/index.yaml")
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

	indexCache.Set(c.url, *index, cache.DefaultExpiration)

	return index, err
}

func (c repoCfg) FindApps(_ string) (map[string]string, error) {

	index, err := c.getIndex()
	if err != nil {
		return nil, err
	}

	cfgs := make(map[string]string)

	for chartName := range index.Entries {
		cfgs[chartName] = "helm"
	}

	return cfgs, nil
}

func (c repoCfg) repoAdd() (string, error) {
	return c.cmd.repoAdd(c.name, c.url, repoAddOpts{
		Username: c.username, Password: c.password,
		CAData: c.caData, CertData: c.certData, KeyData: c.keyData,
	})
}

func (c repoCfg) GetTemplate(path string, resolvedRevision string) (string, string, error) {

	if resolvedRevision == "" {
		return "", "", errors.New("resolvedRevision must be resolved")
	}

	err := c.checkKnownChart(path)
	if err != nil {
		return "", "", err
	}

	log.WithFields(log.Fields{"chartName": path}).Debug("chart name")

	_, err = c.cmd.fetch(c.name, path, fetchOpts{
		Version: resolvedRevision, Destination: ".",
	})

	return filepath.Join(c.workDir, path), "helm", err
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

func (c repoCfg) ResolveRevision(path string, revision string) (string, error) {

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
