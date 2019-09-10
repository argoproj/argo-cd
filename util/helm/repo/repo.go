package repo

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/argoproj/argo-cd/util/helm"
	"github.com/argoproj/argo-cd/util/repo"
)

var indexCache = cache.New(5*time.Minute, 5*time.Minute)

type helmRepo struct {
	cmd                           *helm.Cmd
	url, name, username, password string
	caData, certData, keyData     []byte
}

func (c helmRepo) Init() error {
	_, err := c.getIndex()
	if err != nil {
		return err
	}
	_, err = c.repoAdd()
	if err != nil {
		return err
	}
	_, err = c.cmd.RepoUpdate()
	return err
}

func (c helmRepo) LockKey() string {
	return c.cmd.WorkDir
}

func (c helmRepo) ResolveRevision(app, revision string) (string, error) {
	if revision != "" {
		return revision, nil
	}

	index, err := c.getIndex()
	if err != nil {
		return "", err
	}

	for chartName := range index.Entries {
		if chartName == app {
			return index.Entries[chartName][0].Version, nil
		}
	}

	return "", errors.New("failed to find chart " + app)
}

func (c helmRepo) RevisionMetadata(app, resolvedRevision string) (*repo.RevisionMetadata, error) {

	index, err := c.getIndex()
	if err != nil {
		return nil, err
	}

	for _, entry := range index.Entries[app] {
		if entry.Version == resolvedRevision {
			return &repo.RevisionMetadata{Date: entry.Created}, nil
		}
	}

	return nil, fmt.Errorf("unknown chart \"%s/%s\"", app, resolvedRevision)
}

type entry struct {
	Version string
	Created time.Time
}

type index struct {
	Entries map[string][]entry
}

func (c helmRepo) getIndex() (*index, error) {

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

func (c helmRepo) ListApps(revision string) (map[string]string, string, error) {
	index, err := c.getIndex()
	if err != nil {
		return nil, "", err
	}
	apps := make(map[string]string, len(index.Entries))
	for chartName := range index.Entries {
		apps[chartName] = "Helm"
	}
	return apps, revision, nil
}

func (c helmRepo) repoAdd() (string, error) {
	return c.cmd.RepoAdd(c.name, c.url, helm.RepoAddOpts{
		Username: c.username, Password: c.password,
		CertData: c.certData, KeyData: c.keyData, CAData: c.caData,
	})
}

func (c helmRepo) GetApp(app string, resolvedRevision string) (string, error) {
	if resolvedRevision == "" {
		return "", fmt.Errorf("invalid resolved revision \"%s\", must be resolved", resolvedRevision)
	}

	err := c.checkKnownChart(app)
	if err != nil {
		return "", err
	}

	_, err = c.cmd.Fetch(c.name, app, helm.FetchOpts{Version: resolvedRevision, Destination: "."})

	return filepath.Join(c.cmd.WorkDir, app), err
}

func (c helmRepo) checkKnownChart(chartName string) error {
	knownChart, err := c.isKnownChart(chartName)
	if err != nil {
		return err
	}
	if !knownChart {
		return fmt.Errorf("unknown chart \"%s\"", chartName)
	}
	return nil
}

func (c helmRepo) isKnownChart(chartName string) (bool, error) {

	index, err := c.getIndex()
	if err != nil {
		return false, err
	}

	_, ok := index.Entries[chartName]

	return ok, nil
}
