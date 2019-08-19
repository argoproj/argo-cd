package git

import (
	"path/filepath"

	"github.com/argoproj/argo-cd/util/repo"
)

type gitRepo struct {
	client Client
	disco  func(root string) (map[string]string, error)
}

func (g gitRepo) LockKey() string {
	return g.client.Root()
}

func (g gitRepo) GetApp(app, resolvedRevision string) (string, error) {
	err := g.client.Checkout(resolvedRevision)
	if err != nil {
		return "", err
	}
	return filepath.Join(g.client.Root(), app), nil
}

func (g gitRepo) ListApps(revision string) (map[string]string, string, error) {
	resolvedRevision, err := g.client.LsRemote(revision)
	if err != nil {
		return nil, "", err
	}
	apps, err := g.disco(g.client.Root())
	return apps, resolvedRevision, err
}

func (g gitRepo) ResolveRevision(path, revision string) (string, error) {
	return g.client.LsRemote(revision)
}

func (g gitRepo) RevisionMetadata(_, revision string) (*repo.RevisionMetadata, error) {
	metadata, err := g.client.RevisionMetadata(revision)
	if err != nil {
		return nil, err
	}
	out := &repo.RevisionMetadata{
		Author:  metadata.Author,
		Date:    metadata.Date,
		Tags:    metadata.Tags,
		Message: metadata.Message,
	}
	return out, err
}

func NewRepo(url string, creds Creds, insecure, enableLfs bool, disco func(root string) (map[string]string, error)) (repo.Repo, error) {
	client, err := NewFactory().NewClient(url, repo.TempRepoPath(url), creds, insecure, enableLfs)
	if err != nil {
		return nil, err
	}
	err = client.Init()
	if err != nil {
		return nil, err
	}
	err = client.Fetch()
	if err != nil {
		return nil, err
	}
	return &gitRepo{client, disco}, nil
}
