package repo

import (
	"github.com/argoproj/argo-cd/util/app/path"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/repo"
)

type gitRepo struct {
	client git.Client
	disco  func(root string) (map[string]string, error)
}

func (g gitRepo) Init() error {
	err := g.client.Init()
	if err != nil {
		return err
	}
	return g.client.Fetch()
}

func (g gitRepo) LockKey() string {
	return g.client.Root()
}

func (g gitRepo) GetApp(app, resolvedRevision string) (string, error) {
	err := g.client.Checkout(resolvedRevision)
	if err != nil {
		return "", err
	}
	appPath, err := path.Path(g.client.Root(), app)
	if err != nil {
		return "", err
	}
	return appPath, nil
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

func NewRepo(url string, creds git.Creds, insecure, enableLfs bool, disco func(root string) (map[string]string, error)) (repo.Repo, error) {
	client, err := git.NewFactory().NewClient(url, repo.TempRepoPath(url), creds, insecure, enableLfs)
	if err != nil {
		return nil, err
	}
	r := &gitRepo{client, disco}
	return r, r.Init()
}
