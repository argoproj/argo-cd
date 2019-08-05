package helm

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gobwas/glob"
	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	client2 "github.com/argoproj/argo-cd/util/depot/client"
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
	info, err := os.Stat(c.cmd.workDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		_, err = c.cmd.init()
	}
	return err
}

func (c client) ResolveRevision(path, revision string) (string, error) {
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

func (c client) Revision(path string) (string, error) {

	chartName := strings.Split(path, "/")[0]
	yamlFile, err := ioutil.ReadFile(filepath.Join(c.cmd.workDir, chartName, "Chart.yaml"))
	if err != nil {
		return "", err
	}

	entry := &entry{}
	err = yaml.Unmarshal(yamlFile, entry)

	return entry.Version, err
}

func (c client) RevisionMetadata(path, revision string) (*client2.RevisionMetadata, error) {

	index, err := c.getIndex()
	if err != nil {
		return nil, err
	}

	for _, entry := range index.Entries[path] {
		if entry.Version == revision {
			return &client2.RevisionMetadata{Date: entry.Created}, nil
		}
	}

	return nil, fmt.Errorf("unknown chart \"%s/%s\"", path, revision)
}

type entry struct {
	Version string
	Created time.Time
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

	matcher, err := glob.Compile(path)
	if err != nil {
		return nil, err
	}
	index, err := c.getIndex()
	if err != nil {
		return nil, err
	}

	var files []string
	for chartName := range index.Entries {
		file := filepath.Join(chartName, "Chart.yaml")
		if matcher.Match(file) {
			files = append(files, file)
		}
	}

	return files, nil
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

	chartName := strings.Split(path, "/")[0]
	err := c.checkKnownChart(chartName)
	if err != nil {
		return err
	}

	_, err = c.cmd.fetch(c.name, chartName, fetchOpts{version: resolvedRevision, destination: "."})

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
