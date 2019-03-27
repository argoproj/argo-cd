package repos

import "github.com/argoproj/argo-cd/util/git"

type gitClient struct {
	client git.Client
}

func (g gitClient) WorkDir() string {
	return g.client.Root()
}

func (g gitClient) Test() error {
	_, err := g.client.LsRemote("HEAD")
	return err
}

func (g gitClient) Checkout(glob, revision string) (string, error) {
	err := g.client.Checkout(revision)
	if err != nil {
		return "", err
	}
	return g.client.CommitSHA()
}

func (g gitClient) ResolveRevision(glob, revision string) (string, error) {
	return g.client.LsRemote(revision)
}

func (g gitClient) LsFiles(glob string) ([]string, error) {
	return g.client.LsFiles(glob)
}

func newGitClient(url, workDir, username, password, sshPrivateKey string, insecureIgnoreHostKey bool) (Client, error) {
	client, err := git.NewFactory().NewClient(url, workDir, username, password, sshPrivateKey, insecureIgnoreHostKey)

	return gitClient{client}, err
}
