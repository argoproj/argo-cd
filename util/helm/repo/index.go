package repo

import (
	"errors"
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

func Index(url string) (*index, error) {

	cachedIndex, found := indexCache.Get(url)
	if found {
		log.WithFields(log.Fields{"url": url}).Debug("index cache hit")
		i := cachedIndex.(index)
		return &i, nil
	}

	start := time.Now()

	resp, err := http.Get(url + "/index.yaml")
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

func (i *index) contains(chartName string) bool {
	_, ok := i.Entries[chartName]
	return ok
}
