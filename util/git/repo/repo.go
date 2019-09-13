package repo

import (
	"github.com/argoproj/argo-cd/util/app/path"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/repo"
	"github.com/argoproj/argo-cd/util/repo/metrics"
)

type GitRepo struct {
	client git.Client
	disco  func(root string) (map[string]string, error)
}

func (g GitRepo) Init() error {
	err := g.client.Init()
	if err != nil {
		return err
	}
	return g.client.Fetch()
}

func (g GitRepo) LockKey() string {
	return g.client.Root()
}

func (g GitRepo) GetApp(app, resolvedRevision string) (string, error) {
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

// convert an ambiguous revision (e.g. "", "master" or "HEAD") into a specific revision (e.g. "231345034boc" or "5.8.0")
func (g GitRepo) ResolveRevision(revision string) (resolvedRevision string, err error) {
	return g.client.LsRemote(revision)
}

func (g GitRepo) ListApps(resolvedRevision string) (map[string]string, error) {
	err := g.client.Checkout(resolvedRevision)
	if err != nil {
		return nil, err
	}
	apps, err := g.disco(g.client.Root())
	return apps, err
}

func (g GitRepo) ResolveAppRevision(path, revision string) (string, error) {
	return g.client.LsRemote(revision)
}

func (g GitRepo) RevisionMetadata(_, resolvedRevision string) (*repo.RevisionMetadata, error) {
	metadata, err := g.client.RevisionMetadata(resolvedRevision)
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

func NewRepo(url string, creds git.Creds, insecure, enableLfs bool, disco func(root string) (map[string]string, error), reporter metrics.Reporter) (repo.Repo, error) {
	workDir, err := repo.WorkDir(url)
	if err != nil {
		return nil, err
	}
	client, err := git.NewClient(url, workDir, creds, insecure, enableLfs, reporter)
	if err != nil {
		return nil, err
	}
	return &GitRepo{client, disco}, nil
}
