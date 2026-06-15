package repository

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/util/git"
)

type worktreeRef struct {
	path  string
	count int
}

// worktreeCleanup releases one reference to a shared worktree; the caller should defer it.
type worktreeCleanup struct {
	s      *Service
	client git.Client
	key    string
}

func (w *worktreeCleanup) cleanup() {
	w.s.releaseWorktree(w.client, w.key)
}

// createAndRegisterWorktree returns a worktree for the given commit, creating one if needed.
// Worktrees are ref-counted by "<normalizedRepoURL>@<commitSHA>", so concurrent requests and
// repeated refs at the same commit share one checkout. The caller must defer the returned cleanup.
func (s *Service) createAndRegisterWorktree(
	gitClient git.Client,
	normalizedRepoURL string,
	commitSHA string,
	depth int64,
) (string, *worktreeCleanup, error) {
	key := normalizedRepoURL + "@" + commitSHA
	root := gitClient.Root()

	// Serialize git ops per root (they race on shared .git lock files) and make the reuse check atomic.
	s.gitWorktreeLock.Lock(root)
	defer s.gitWorktreeLock.Unlock(root)

	s.worktreesMu.Lock()
	if ref, ok := s.worktrees[key]; ok {
		ref.count++
		path := ref.path
		s.worktreesMu.Unlock()
		return path, &worktreeCleanup{s: s, client: gitClient, key: key}, nil
	}
	s.worktreesMu.Unlock()

	// Init creates worktreeRootDir, but may not have run (e.g. in tests).
	if err := os.MkdirAll(s.worktreeRootDir, 0o700); err != nil {
		return "", nil, fmt.Errorf("failed to create worktree root dir %s: %w", s.worktreeRootDir, err)
	}
	worktreePath := filepath.Join(s.worktreeRootDir, "argocd-worktree-"+uuid.New().String())

	// ls-remote resolved the SHA but didn't fetch objects.
	if !gitClient.IsRevisionPresent(commitSHA) {
		if err := gitClient.Fetch(commitSHA, depth); err != nil {
			return "", nil, fmt.Errorf("failed to fetch revision %s: %w", commitSHA, err)
		}
	}

	if err := gitClient.CreateWorktree(commitSHA, worktreePath); err != nil {
		return "", nil, fmt.Errorf("failed to create worktree at revision %s: %w", commitSHA, err)
	}

	// Register so getResolvedRefValueFile can resolve value files from this worktree.
	s.gitRepoPaths.Add(key, worktreePath)
	s.worktreesMu.Lock()
	s.worktrees[key] = &worktreeRef{path: worktreePath, count: 1}
	s.worktreesMu.Unlock()

	return worktreePath, &worktreeCleanup{s: s, client: gitClient, key: key}, nil
}

// releaseWorktree drops one reference to a shared worktree, removing it once no users remain.
func (s *Service) releaseWorktree(gitClient git.Client, key string) {
	root := gitClient.Root()
	s.gitWorktreeLock.Lock(root)
	defer s.gitWorktreeLock.Unlock(root)

	s.worktreesMu.Lock()
	ref, ok := s.worktrees[key]
	if !ok {
		s.worktreesMu.Unlock()
		return
	}
	ref.count--
	if ref.count > 0 {
		s.worktreesMu.Unlock()
		return
	}
	delete(s.worktrees, key)
	worktreePath := ref.path
	s.worktreesMu.Unlock()

	// `git worktree remove` deletes both the working dir and its .git/worktrees admin entry. If it
	// fails, fall back to deleting just the working dir so it doesn't leak; the stale admin entry is
	// pruned at the next startup.
	if err := gitClient.RemoveWorktree(worktreePath); err != nil {
		log.Warnf("Failed to remove worktree at %s: %v; removing directory directly", worktreePath, err)
		if rmErr := os.RemoveAll(worktreePath); rmErr != nil {
			log.Warnf("Failed to remove worktree directory %s: %v", worktreePath, rmErr)
		}
	}
	s.gitRepoPaths.Remove(key)
}

// cleanupOrphanedWorktrees clears worktreeRootDir at startup. Worktrees are ref-counted in memory,
// so none are live then: anything present was orphaned by a crash and would otherwise accumulate.
func (s *Service) cleanupOrphanedWorktrees() error {
	if err := os.RemoveAll(s.worktreeRootDir); err != nil {
		return fmt.Errorf("failed to remove orphaned worktrees at %s: %w", s.worktreeRootDir, err)
	}
	return os.MkdirAll(s.worktreeRootDir, 0o700)
}

// pruneWorktreeAdminDir removes a repo clone's stale .git/worktrees entries (left by crashed
// worktrees). Safe at startup; git recreates the directory on the next worktree add.
func pruneWorktreeAdminDir(repoPath string) {
	adminDir := filepath.Join(repoPath, ".git", "worktrees")
	if err := os.RemoveAll(adminDir); err != nil {
		log.Warnf("Failed to prune worktree admin dir %s: %v", adminDir, err)
	}
}
