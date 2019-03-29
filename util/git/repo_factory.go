package git

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/argoproj/argo-cd/util/repos/api"
)

type RepoFactory struct {
}

func GetRepoFactory() RepoFactory {
	return RepoFactory{}
}

func (f RepoFactory) SameURL(leftRepo, rightRepo string) bool {
	return sameURL(leftRepo, rightRepo)
}

func (f RepoFactory) NormalizeURL(url string) string {
	return normalizeGitURL(url)
}

func (f RepoFactory) GetRepo(url, username, password, sshPrivateKey string, insecureIgnoreHostKey bool) (api.Repo, error) {
	url = f.NormalizeURL(url)

	workDir, err := ioutil.TempDir(os.TempDir(), strings.Replace(url, "/", "_", -1))
	if err != nil {
		return nil, err
	}

	client, err := newFactory().newClient(url, workDir, username, password, sshPrivateKey, insecureIgnoreHostKey)
	if err != nil {
		return nil, err
	}

	_, err = client.lsRemote("HEAD")
	if err != nil {
		return nil, err
	}

	return repo{client}, nil
}
