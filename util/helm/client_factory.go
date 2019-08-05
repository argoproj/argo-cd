package helm

import (
	"errors"
	"time"

	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"

	repoclient "github.com/argoproj/argo-cd/util/repo/client"
)

var clientCache = cache.New(5*time.Minute, 5*time.Minute)

func NewClient(url, name, username, password string, caData, certData, keyData []byte) (repoclient.Client, error) {

	if name == "" {
		return nil, errors.New("must name client")
	}

	cached, found := clientCache.Get(url)
	if found {
		log.WithFields(log.Fields{"url": url}).Debug("client cfg cache hit")
		return cached.(repoclient.Client), nil
	}
	cmd, err := newCmd(repoclient.TempRepoPath(url))
	if err != nil {
		return nil, err
	}

	_, err = cmd.init()
	if err != nil {
		return nil, err
	}

	cfg := client{
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
	if err != nil {
		return nil, err
	}

	_, err = cfg.repoAdd()
	if err != nil {
		return nil, err
	}

	_, err = cfg.cmd.repoUpdate()

	clientCache.Set(url, cfg, cache.DefaultExpiration)

	return cfg, err
}
