package helm

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

var indexCache = cache.New(5*time.Minute, 5*time.Minute)

type Entry struct {
	Version string
	Created time.Time
}

type Index struct {
	Entries map[string][]Entry
}

func GetIndex(repo string, username string, password string) (*Index, error) {

	cachedIndex, found := indexCache.Get(repo)
	if found {
		log.WithFields(log.Fields{"url": repo}).Debug("Index cache hit")
		i := cachedIndex.(Index)
		return &i, nil
	}

	start := time.Now()
	repoURL, err := url.Parse(repo)
	if err != nil {
		return nil, err
	}
	repoURL.Path = path.Join(repoURL.Path, "index.yaml")

	req, err := http.NewRequest("GET", repoURL.String(), nil)
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
		return nil, errors.New("failed to get Index: " + resp.Status)
	}

	index := &Index{}
	err = yaml.NewDecoder(resp.Body).Decode(index)

	log.WithFields(log.Fields{"seconds": time.Since(start).Seconds()}).Info("took to get Index")

	indexCache.Set(repo, *index, cache.DefaultExpiration)

	return index, err
}
