package repo

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

var indexCache = cache.New(5*time.Minute, 5*time.Minute)

type entry struct {
	Version string
	Created time.Time
}

type index struct {
	Entries map[string][]entry
}

func Index(url, username, password string) (*index, error) {

	cachedIndex, found := indexCache.Get(url)
	if found {
		log.WithFields(log.Fields{"url": url}).Debug("index cache hit")
		i := cachedIndex.(index)
		return &i, nil
	}

	start := time.Now()

	req, err := http.NewRequest("GET", url+"/index.yaml", nil)
	if err != nil {
		return nil, err
	}
	if username != "" {
		// only basic supported
		token := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
		req.Header.Add("Authorization", fmt.Sprintf("Basic %s", token))
	}
	resp, err := http.DefaultClient.Do(req)
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

	indexCache.Set(url, *index, cache.DefaultExpiration)

	return index, err
}

func (i *index) contains(chart string) bool {
	_, ok := i.Entries[chart]
	return ok
}

func (i *index) entry(chart, version string) (*entry, error) {
	for _, entry := range i.Entries[chart] {
		if entry.Version == version {
			return &entry, nil
		}
	}
	return nil, fmt.Errorf("unknown chart \"%s/%s\"", chart, version)
}

func (i *index) latest(chart string) (string, error) {
	for chartName := range i.Entries {
		if chartName == chart {
			return i.Entries[chartName][0].Version, nil
		}
	}
	return "", fmt.Errorf("failed to find chart %s", chart)
}
