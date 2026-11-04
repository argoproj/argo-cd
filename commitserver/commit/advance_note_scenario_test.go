package commit

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/git"
)

type localRepoClientFactory struct{ remotePath string }

func (f localRepoClientFactory) NewClient(_ *v1alpha1.Repository, rootPath string) (git.Client, error) {
	return git.NewClientExt(f.remotePath, rootPath, &git.NopCreds{}, false, false, "", "")
}

func noteDrySHA(t *testing.T, remotePath, sha string) string {
	t.Helper()
	raw, err := getClientForClone(t, remotePath).GetCommitNote(t.Context(), sha, NoteNamespace)
	require.NoError(t, err)
	var note CommitNote
	require.NoError(t, json.Unmarshal([]byte(raw), &note))
	return note.DrySHA
}

func remoteHead(t *testing.T, remotePath, branch string) string {
	t.Helper()
	sha, err := runGitCmd(t.Context(), remotePath, "rev-parse", branch)
	require.NoError(t, err)
	return sha
}

// Real git remote: int skipped dry commit db205bd because of
// manifest-generate-paths and kept its note at 1265141, while stage's retry advanced to db205bd.
func TestAdvanceHydratorNote_RealignsEnvSkippedByPathFilter(t *testing.T) {
	remotePath, localPath := setupRepoWithRemote(t)
	commitAndPushBranch(t, localPath, "int")
	intNext := commitAndPushBranch(t, localPath, "int-next")
	commitAndPushBranch(t, localPath, "stage")
	stageNext := commitAndPushBranch(t, localPath, "stage-next")

	require.NoError(t, AddNote(t.Context(), getClientForClone(t, remotePath), "1265141", intNext))
	require.NoError(t, AddNote(t.Context(), getClientForClone(t, remotePath), "db205bd", stageNext))
	require.NotEqual(t, noteDrySHA(t, remotePath, intNext), noteDrySHA(t, remotePath, stageNext), "precondition: stalled state")

	svc := NewService(nil, nil, nil)
	svc.repoClientFactory = localRepoClientFactory{remotePath: remotePath}
	req := &apiclient.AdvanceHydratorNoteRequest{
		Repo:         &v1alpha1.Repository{Repo: remotePath},
		SyncBranch:   "int",
		TargetBranch: "int-next",
		DrySha:       "db205bd",
	}
	resp, err := svc.AdvanceHydratorNote(t.Context(), req)
	require.NoError(t, err)
	assert.True(t, resp.NoteAdvanced)
	assert.Equal(t, intNext, resp.HydratedSha)

	assert.Equal(t, intNext, remoteHead(t, remotePath, "int-next"), "no new hydrated commit")
	assert.Equal(t, "db205bd", noteDrySHA(t, remotePath, intNext), "int realigned")
	assert.Equal(t, "db205bd", noteDrySHA(t, remotePath, stageNext), "stage note preserved")

	again, err := svc.AdvanceHydratorNote(t.Context(), req)
	require.NoError(t, err)
	assert.False(t, again.NoteAdvanced, "idempotent")
}

func TestAdvanceHydratorNote_RejectsEmptyDrySha(t *testing.T) {
	svc := NewService(nil, nil, nil)
	_, err := svc.AdvanceHydratorNote(t.Context(), &apiclient.AdvanceHydratorNoteRequest{
		Repo:         &v1alpha1.Repository{Repo: "https://example.com/repo.git"},
		SyncBranch:   "env",
		TargetBranch: "env-next",
	})
	require.ErrorContains(t, err, "dry sha is required")
}
