package git

import (
	"fmt"
	"path/filepath"

	"github.com/argoproj/argo-cd/util/repos/disco"
)

type repo struct {
	client client
}

func (c repo) LockKey() string {
	return c.client.getRoot()
}

func (c repo) makeReady(resolvedRevision string) error {
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

func (c repo) FindApps(revision string) (map[string]string, error) {

	resolvedRevision, err := c.client.lsRemote(revision)
	if err != nil {
		return nil, err
	}

	err = c.makeReady(resolvedRevision)
	if err != nil {
		return nil, err
	}

	return disco.FindAppTemplates(c.client.getRoot())
}

func (c repo) GetTemplate(path, resolvedRevision string) (string, string, error) {
	if !isCommitSHA(resolvedRevision) {
		return "", "", fmt.Errorf("invalid resolved revision \"%s\", must be resolved", resolvedRevision)
	}

	err := c.makeReady(resolvedRevision)
	if err != nil {
		return "", "", err
	}

	dir := filepath.Join(c.client.getRoot(), path)

	appType, err := disco.GetAppType(dir)

	return dir, appType, err
}

func (c repo) ResolveRevision(path string, revision string) (string, error) {
	return c.client.lsRemote(revision)
}
