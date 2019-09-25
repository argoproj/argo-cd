package repo

import (
	"fmt"
	"path/filepath"

	"github.com/argoproj/argo-cd/util/helm"
	"github.com/argoproj/argo-cd/util/repo"
)

type helmRepo struct {
	cmd                           *helm.Cmd
	url, name, username, password string
	caData, certData, keyData     []byte
}

func (c helmRepo) Init() error {
	_, err := c.index()
	if err != nil {
		return err
	}
	_, err = c.repoAdd()
	if err != nil {
		return err
	}
	_, err = c.cmd.RepoUpdate()
	return err
}

func (c helmRepo) LockKey() string {
	return c.cmd.WorkDir
}

func (c helmRepo) ResolveAppRevision(app, revision string) (string, error) {
	if revision != "" {
		return revision, nil
	}

	index, err := c.index()
	if err != nil {
		return "", err
	}

	return index.latest(app)
}

func (c helmRepo) RevisionMetadata(app, resolvedRevision string) (*repo.RevisionMetadata, error) {
	index, err := c.index()
	if err != nil {
		return nil, err
	}
	entry, err := index.entry(app, resolvedRevision)
	if err != nil {
		return nil, err
	}
	return &repo.RevisionMetadata{Date: entry.Created}, nil
}

func (c helmRepo) ListApps(_ string) (map[string]string, error) {
	index, err := c.index()
	if err != nil {
		return nil, err
	}
	apps := make(map[string]string, len(index.Entries))
	for chartName := range index.Entries {
		apps[chartName] = "Helm"
	}
	return apps, nil
}

func (c helmRepo) repoAdd() (string, error) {
	return c.cmd.RepoAdd(c.name, c.url, helm.RepoAddOpts{
		Username: c.username, Password: c.password,
		CertData: c.certData, KeyData: c.keyData, CAData: c.caData,
	})
}

func (c helmRepo) GetApp(app string, resolvedRevision string) (string, error) {
	if resolvedRevision == "" {
		return "", fmt.Errorf("invalid resolved revision \"%s\", must be resolved", resolvedRevision)
	}

	err := c.checkKnownChart(app)
	if err != nil {
		return "", err
	}

	_, err = c.cmd.Fetch(c.name, app, helm.FetchOpts{Version: resolvedRevision, Destination: "."})

	return filepath.Join(c.cmd.WorkDir, app), err
}

func (c helmRepo) checkKnownChart(chartName string) error {
	index, err := c.index()
	if err != nil {
		return err
	}
	if !index.contains(chartName) {
		return fmt.Errorf("unknown chart \"%s\"", chartName)
	}
	return nil
}

func (c helmRepo) index() (*index, error) {
	return Index(c.url, c.username, c.password)
}
