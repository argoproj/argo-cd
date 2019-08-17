package git

import (
	"github.com/argoproj/argo-cd/util/repo"
)

type gitRepo struct {
	client Client
}

func (g gitRepo) Test() error {
	return g.Init()
}

func (g gitRepo) Root() string {
	return g.client.Root()
}

func (g gitRepo) Init() error {
	err := g.client.Init()
	if err != nil {
		return err
	}
	return g.client.Fetch()
}

func (g gitRepo) Checkout(_, revision string) error {
	return g.client.Checkout(revision)
}

func (g gitRepo) ResolveRevision(path, revision string) (string, error) {
	return g.client.LsRemote(revision)
}

func (g gitRepo) LsFiles(path string) ([]string, error) {
	return g.client.LsFiles(path)
}

func (g gitRepo) Revision(path string) (string, error) {
	return g.client.CommitSHA()
}

func (g gitRepo) RevisionMetadata(_, revision string) (*repo.RevisionMetadata, error) {
	metadata, err := g.client.RevisionMetadata(revision)
	if err != nil {
		return nil, err
	}
	out := repo.RevisionMetadata(*metadata)
	return &out, err
}

func NewRepo(url string, creds Creds, insecure, enableLfs bool) (repo.Repo, error) {
	client, err := NewFactory().NewClient(url, repo.TempRepoPath(url), creds, insecure, enableLfs)
	if err != nil {
		return nil, err
	}
	return &gitRepo{client}, nil
}
