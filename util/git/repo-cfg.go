package git

import (
	"errors"
	"path/filepath"

	"github.com/argoproj/argo-cd/util/repos/disco"

	"github.com/argoproj/argo-cd/util/repos/api"
)

type repoCfg struct {
	client client
}

func (c repoCfg) LockKey() string {
	return c.client.getRoot()
}

func (c repoCfg) makeReady(resolvedRevision string) error {
	err := c.client.init()
	if err != nil {
		return err
	}

	err = c.client.fetch()
	if err != nil {
		return err
	}

	return c.client.checkout(resolvedRevision)
}

func (c repoCfg) FindAppCfgs(revision api.RepoRevision) (map[api.AppPath]api.AppType, error) {

	resolvedRevision, err := c.client.lsRemote(revision)
	if err != nil {
		return nil, err
	}

	err = c.makeReady(resolvedRevision)
	if err != nil {
		return nil, err
	}

	return disco.FindAppCfgs(c.client.getRoot())
}

func (c repoCfg) GetAppCfg(path, resolvedRevision string) (string, api.AppType, error) {
	if !isCommitSHA(resolvedRevision) {
		return "", "", errors.New("must be resolved resolvedRevision")
	}

	err := c.makeReady(resolvedRevision)
	if err != nil {
		return "", "", err
	}

	dir := filepath.Join(c.client.getRoot(), path)

	appType, err := disco.GetAppType(dir)

	return dir, appType, err
}

func (c repoCfg) ResolveRevision(path api.AppPath, revision api.AppRevision) (string, error) {
	return c.client.lsRemote(revision)
}
