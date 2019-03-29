package git

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/argoproj/argo-cd/util/repos/api"
)

type RepoCfgFactory struct {
}

func NewRepoCfgFactory() RepoCfgFactory {
	return RepoCfgFactory{}
}

func (f RepoCfgFactory) SameURL(leftRepo, rightRepo string) bool {
	return sameURL(leftRepo, rightRepo)
}

func (f RepoCfgFactory) NormalizeURL(url string) string {
	return normalizeGitURL(url)
}

func (f RepoCfgFactory) IsResolvedRevision(revision string) bool {
	return isCommitSHA(revision)
}

func (f RepoCfgFactory) NewRepoCfg(url, username, password, sshPrivateKey string, insecureIgnoreHostKey bool) (api.RepoCfg, error) {
	url = f.NormalizeURL(url)

	workDir, err := ioutil.TempDir(os.TempDir(), strings.ReplaceAll(url, "/", "_"))
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

	return repoCfg{client}, nil
}
