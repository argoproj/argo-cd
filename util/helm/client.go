package helm

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/argoproj/argo-cd/util/depot"
)

var indexCache = cache.New(5*time.Minute, 5*time.Minute)

type client struct {
	cmd                           *cmd
	url, name, username, password string
	caData, certData, keyData     []byte
}

func (c client) Test() error {
	_, err := c.cmd.init()
	return err
}

func (c client) Root() string {
	return c.cmd.workDir
}

func (c client) Init() error {
	_, err := c.cmd.init()
	return err
}

func (c client) Fetch() error {
	return nil
}

func (c client) LsRemote(path, revision string) (string, error) {
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

func (c client) CommitSHA() (string, error) {
	return "", nil
}

func (c client) RevisionMetadata(revision string) (*depot.RevisionMetadata, error) {
	return &depot.RevisionMetadata{}, nil
}

type entry struct {
	Version string
}

type index struct {
	Entries map[string][]entry
}

func (c client) getIndex() (*index, error) {

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

func (c client) LsFiles(path string) ([]string, error) {

	index, err := c.getIndex()
	if err != nil {
		return nil, err
	}

	var charts []string

	for chartName := range index.Entries {
		if strings.HasPrefix(chartName, path) {
			charts = append(charts, chartName)
		}
	}

	return charts, nil
}

func (c client) repoAdd() (string, error) {
	return c.cmd.repoAdd(c.name, c.url, repoAddOpts{
		username: c.username, password: c.password,
		caData: c.caData, certData: c.certData, keyData: c.keyData,
	})
}

func (c client) Checkout(path string, resolvedRevision string) error {

	if resolvedRevision == "" {
		return fmt.Errorf("invalid resolved revision \"%s\", must be resolved", resolvedRevision)
	}

	err := c.checkKnownChart(path)
	if err != nil {
		return err
	}

	_, err = c.cmd.fetch(c.name, path, fetchOpts{version: resolvedRevision, destination: "."})

	return err
}

func (c client) checkKnownChart(chartName string) error {
	knownChart, err := c.isKnownChart(chartName)
	if err != nil {
		return err
	}
	if !knownChart {
		return fmt.Errorf("unknown chart \"%s\"", chartName)
	}
	return nil
}

func (c client) isKnownChart(chartName string) (bool, error) {

	index, err := c.getIndex()
	if err != nil {
		return false, err
	}

	_, ok := index.Entries[chartName]

	return ok, nil
}
