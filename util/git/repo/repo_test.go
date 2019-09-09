package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/git/mocks"
	"github.com/argoproj/argo-cd/util/repo"
)

func fixtures() (*gitRepo, git.Client, map[string]string) {
	client := &mocks.Client{}
	client.On("Checkout", mock.Anything, mock.Anything).Return(nil)
	client.On("Root").Return("./testdata")
	client.On("LsRemote", mock.Anything).Return("1.0.0", nil)
	m := &git.RevisionMetadata{}
	client.On("RevisionMetadata", mock.Anything).Return(m, nil)
	apps := make(map[string]string)
	r := &gitRepo{client, func(root string) (map[string]string, error) {
		return apps, nil
	}}
	return r, client, apps
}

func Test_gitRepo_LockKey(t *testing.T) {
	r, c, _ := fixtures()
	assert.Equal(t, c.Root(), r.LockKey())
}

func Test_gitRepo_GetApp(t *testing.T) {
	r, _, _ := fixtures()
	_, err := r.GetApp("/", "")
	assert.EqualError(t, err, "/: app path is absolute")
}

func Test_gitRepo_ListApps(t *testing.T) {
	r, _, apps := fixtures()
	listedApps, resolvedRevision, err := r.ListApps("")
	assert.NoError(t, err)
	assert.Equal(t, "1.0.0", resolvedRevision)
	assert.Equal(t, apps, listedApps)
}

func Test_gitRepo_ResolveRevision(t *testing.T) {
	r, _, _ := fixtures()
	resolvedRevision, err := r.ResolveRevision(".", "")
	assert.NoError(t, err)
	assert.Equal(t, "1.0.0", resolvedRevision)
}

func Test_gitRepo_RevisionMetadata(t *testing.T) {
	r, _, _ := fixtures()
	m, err := r.RevisionMetadata(".", "")
	assert.NoError(t, err)
	assert.Equal(t, repo.RevisionMetadata{}, *m)

}
