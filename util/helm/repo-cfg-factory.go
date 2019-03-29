package helm

import (
	"errors"
	"io/ioutil"
	"os"
	"strings"

	"github.com/argoproj/argo-cd/util/repos/api"
)

type RepoCfgFactory struct {
}

func NewRepoCfgFactory() api.RepoCfgFactory {
	return RepoCfgFactory{}
}

func (f RepoCfgFactory) SameURL(leftRepo, rightRepo string) bool {
	return leftRepo == rightRepo
}

func (f RepoCfgFactory) NormalizeURL(url string) string {
	return url
}

func (f RepoCfgFactory) IsResolvedRevision(revision string) bool {
	return revision != ""
}
func (f RepoCfgFactory) NewRepoCfg(
	url, name, username, password string,
	caData, certData, keyData []byte) (api.RepoCfg, error) {
	url = f.NormalizeURL(url)
	if name == "" {
		return nil, errors.New("must name repo")
	}

	workDir, err := ioutil.TempDir(os.TempDir(), strings.ReplaceAll(url, "/", "_"))
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

	cfg := repoCfg{
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

	return cfg, err
}
