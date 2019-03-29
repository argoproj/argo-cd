package helm

import (
	"errors"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/util/repos/api"
)

var repoCache = cache.New(5*time.Minute, 5*time.Minute)

type RepoFactory struct {
}

func GetRepoFactory() api.RepoFactory {
	return RepoFactory{}
}

func (f RepoFactory) SameURL(leftRepo, rightRepo string) bool {
	return leftRepo == rightRepo
}

func (f RepoFactory) NormalizeURL(url string) string {
	return url
}

func (f RepoFactory) GetRepo(url, name, username, password string, caData, certData, keyData []byte) (api.Repo, error) {

	if name == "" {
		return nil, errors.New("must name repo")
	}
	url = f.NormalizeURL(url)

	cachedRepo, found := repoCache.Get(url)
	if found {
		log.WithFields(log.Fields{"url": url}).Debug("repo cfg cache hit")
		return cachedRepo.(api.Repo), nil
	}

	workDir, err := ioutil.TempDir(os.TempDir(), strings.Replace(url, "/", "_", -1))
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

	cfg := repo{
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
	if err != nil {
		return nil, err
	}

	_, err = cfg.repoAdd()
	if err != nil {
		return nil, err
	}

	_, err = cfg.cmd.repoUpdate()

	repoCache.Set(url, cfg, cache.DefaultExpiration)

	return cfg, err
}
