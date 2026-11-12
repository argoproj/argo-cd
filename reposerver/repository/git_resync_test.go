package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/reposerver/metrics"
	"github.com/argoproj/argo-cd/v3/util/git"
	gitmocks "github.com/argoproj/argo-cd/v3/util/git/mocks"
)

func TestIsRepoCorrupt(t *testing.T) {
	assert.False(t, isRepoCorrupt(nil))
	assert.False(t, isRepoCorrupt(errors.New("fatal: unable to connect to remote")))
	assert.False(t, isRepoCorrupt(context.Canceled))
	// Markers git prints when the local object database is damaged.
	assert.True(t, isRepoCorrupt(errors.New("error: Could not read 8bdae1ee09")))
	assert.True(t, isRepoCorrupt(fmt.Errorf("wrapped: %w", errors.New("fatal: internal error in revision walk"))))
	assert.True(t, isRepoCorrupt(errors.New("error: unable to read tree 120ce...")))
}

func TestGitFetchFailReason(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	// A cancelled caller context is classified as "cancelled" even when the
	// error text looks corrupt, so a caller giving up never inflates the
	// genuine-error or corrupt series.
	assert.Equal(t, "cancelled", gitFetchFailReason(cancelled, errors.New("error: Could not read abc")))
	assert.Equal(t, "corrupt", gitFetchFailReason(context.Background(), errors.New("error: Could not read abc")))
	assert.Equal(t, "error", gitFetchFailReason(context.Background(), errors.New("boom")))
}

// TestCheckoutRevisionReclonesCorruptRepo verifies that a checkout failing with
// a corrupted-object-database error triggers a single remove-and-re-clone, and
// that the retried checkout can then succeed. Driven through a mock so the
// recovery control flow is exercised deterministically, independent of git's
// own self-healing refetch.
func TestCheckoutRevisionReclonesCorruptRepo(t *testing.T) {
	revision := "0123456789012345678901234567890123456789"
	root := t.TempDir()
	corruptErr := errors.New("failed to checkout: error: Could not read 8bdae1ee09")

	gitClient := gitmocks.NewClient(t)
	gitClient.EXPECT().Root().Return(root)
	gitClient.EXPECT().Init().Return(nil)
	gitClient.EXPECT().IsRevisionPresent(mock.Anything, revision).Return(false)
	gitClient.EXPECT().Fetch(mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// First pass: both the direct checkout and the FETCH_HEAD fallback fail corrupt.
	gitClient.EXPECT().Checkout(mock.Anything, revision, mock.Anything, mock.Anything).Return("", corruptErr).Once()
	gitClient.EXPECT().Checkout(mock.Anything, "FETCH_HEAD", mock.Anything, mock.Anything).Return("", corruptErr).Once()
	// Second pass (after re-clone): checkout succeeds.
	gitClient.EXPECT().Checkout(mock.Anything, revision, mock.Anything, mock.Anything).Return("", nil).Once()

	err := checkoutRevision(t.Context(), gitClient, revision, false, 0, false)
	require.NoError(t, err)
}

// TestCheckoutRevisionDoesNotRecloneOnTransientError verifies that an ordinary
// (non-corruption) failure is returned as-is and never triggers a re-clone, so
// a slow remote or auth error does not throw away a healthy clone.
func TestCheckoutRevisionDoesNotRecloneOnTransientError(t *testing.T) {
	revision := "0123456789012345678901234567890123456789"

	gitClient := gitmocks.NewClient(t)
	gitClient.EXPECT().Init().Return(nil)
	gitClient.EXPECT().IsRevisionPresent(mock.Anything, revision).Return(false).Once()
	gitClient.EXPECT().Fetch(mock.Anything, "", mock.Anything).Return(errors.New("boom")).Once()
	// Root()/Checkout must never be reached; the mock fails the test if they are.

	err := checkoutRevision(t.Context(), gitClient, revision, false, 0, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

// TestCheckoutRevisionCancelledContextIsLabelledCancelled verifies that when a
// caller cancels, the recorded argocd_git_fetch_fail_total carries
// reason="cancelled" rather than being indistinguishable from a real git error.
func TestCheckoutRevisionCancelledContextIsLabelledCancelled(t *testing.T) {
	remote := t.TempDir()
	rev := initGitRepo(t, newGitRepoOptions{path: remote, remote: remote, addEmptyCommit: true})

	ms := metrics.NewMetricsServer()
	s := NewService(ms, nil, RepoServerInitConstants{ParallelismLimit: 1}, &git.NoopCredsStore{}, t.TempDir())
	gc, err := git.NewClientExt("file://"+remote, t.TempDir(), git.NopCreds{}, true, false, "", "",
		git.WithEventHandlers(metrics.NewGitClientEventHandlers(ms)))
	require.NoError(t, err)

	// Prime the clone so the revision is present locally.
	_, err = s.checkoutRevision(t.Context(), gc, rev, false, 0, false)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = s.checkoutRevision(ctx, gc, rev, false, 0, false)
	require.Error(t, err)

	assert.Equal(t, 1, gitFetchFailCount(t, ms.PrometheusRegistry, "cancelled"),
		"a cancelled request should be recorded with reason=cancelled")
	assert.Equal(t, 0, gitFetchFailCount(t, ms.PrometheusRegistry, "error"),
		"a cancelled request must not be recorded as a generic git error")
}

// TestServiceFetchRecordsFailReason verifies that the Service.fetch wrapper
// records a failed fetch with a classified reason label.
func TestServiceFetchRecordsFailReason(t *testing.T) {
	revision := "0123456789012345678901234567890123456789"
	ms := metrics.NewMetricsServer()
	s := NewService(ms, nil, RepoServerInitConstants{ParallelismLimit: 1}, &git.NoopCredsStore{}, t.TempDir())

	gitClient := gitmocks.NewClient(t)
	gitClient.EXPECT().Root().Return("/tmp/repo")
	gitClient.EXPECT().IsRevisionPresent(mock.Anything, revision).Return(false)
	gitClient.EXPECT().Fetch(mock.Anything, mock.Anything, mock.Anything).Return(errors.New("boom"))

	err := s.fetch(t.Context(), gitClient, []string{revision}, 0)
	require.Error(t, err)
	assert.Equal(t, 1, gitFetchFailCount(t, ms.PrometheusRegistry, "error"))
}

// TestCheckoutRevisionReturnsErrorWhenRecloneCleanupFails verifies that when the
// corrupted clone cannot be removed, the removal error is surfaced rather than
// silently proceeding.
func TestCheckoutRevisionReturnsErrorWhenRecloneCleanupFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("cannot make os.RemoveAll fail via directory permissions when running as root")
	}
	parent := t.TempDir()
	root := filepath.Join(parent, "repo")
	require.NoError(t, os.MkdirAll(root, 0o755))
	// A read-only parent makes os.RemoveAll(root) fail to unlink the directory.
	require.NoError(t, os.Chmod(parent, 0o555))
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })

	revision := "0123456789012345678901234567890123456789"
	corruptErr := errors.New("error: Could not read 8bdae1ee09")

	gitClient := gitmocks.NewClient(t)
	gitClient.EXPECT().Root().Return(root)
	gitClient.EXPECT().Init().Return(nil)
	gitClient.EXPECT().IsRevisionPresent(mock.Anything, revision).Return(false)
	gitClient.EXPECT().Fetch(mock.Anything, mock.Anything, mock.Anything).Return(nil)
	gitClient.EXPECT().Checkout(mock.Anything, revision, mock.Anything, mock.Anything).Return("", corruptErr).Once()
	gitClient.EXPECT().Checkout(mock.Anything, "FETCH_HEAD", mock.Anything, mock.Anything).Return("", corruptErr).Once()

	err := checkoutRevision(t.Context(), gitClient, revision, false, 0, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to remove corrupted git repo")
}

// gitFetchFailCount returns the summed argocd_git_fetch_fail_total value across
// all repo/revision series carrying the given reason label. The counter only
// ever holds whole numbers, so it is returned as an int for exact comparison.
func gitFetchFailCount(t *testing.T, reg *prometheus.Registry, reason string) int {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err)
	var total float64
	for _, mf := range mfs {
		if mf.GetName() != "argocd_git_fetch_fail_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, lp := range m.GetLabel() {
				if lp.GetName() == "reason" && lp.GetValue() == reason {
					total += m.GetCounter().GetValue()
				}
			}
		}
	}
	return int(total)
}
