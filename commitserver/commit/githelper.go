package commit

import (
	"fmt"
	"os/exec"
	"strings"
)

// GitHelper is an interface for interacting with git for managing manifest commits.
type GitHelper interface {
	// Clone clones a git repository.
	Clone(repoURL string) ([]byte, error)
	// Config runs a git config command.
	Config(args ...string) ([]byte, error)
	// SetAuthor sets the author name and email in the git configuration.
	SetAuthor(name, email string) ([]byte, error)
	// CheckoutSyncBranch checks out the sync branch. If the branch does not exist, it creates an orphan branch.
	CheckoutSyncBranch() ([]byte, error)
	// CheckoutTargetBranch checks out the target branch. If the branch does not exist, it creates an empty branch based
	// on the sync branch.
	CheckoutTargetBranch() ([]byte, error)
	// RemoveContents removes all files from the git repository.
	RemoveContents() ([]byte, error)
	// CommitAndPush commits and pushes changes to the target branch.
	CommitAndPush(message string) ([]byte, error)
}

func newGitHelper(repoDir, syncBranch, targetBranch string) GitHelper {
	return &gitHelper{repoDir: repoDir, syncBranch: syncBranch, targetBranch: targetBranch}
}

type gitHelper struct {
	repoDir      string
	syncBranch   string
	targetBranch string
}

// Clone clones a git repository.
func (g *gitHelper) Clone(repoURL string) ([]byte, error) {
	cmd := exec.Command("git", "clone", repoURL, g.repoDir)
	return cmd.CombinedOutput()
}

// Config runs a git config command.
func (g *gitHelper) Config(args ...string) ([]byte, error) {
	args = append([]string{"config"}, args...)
	cmd := exec.Command("git", args...)
	cmd.Dir = g.repoDir
	return cmd.CombinedOutput()
}

// SetAuthor sets the author name and email in the git configuration.
func (g *gitHelper) SetAuthor(name, email string) ([]byte, error) {
	if name != "" {
		out, err := g.Config("--local", "user.name", name)
		if err != nil {
			return out, err
		}
	}
	if email != "" {
		out, err := g.Config("--local", "user.email", email)
		if err != nil {
			return out, err
		}
	}
	return nil, nil
}

// CheckoutSyncBranch checks out the sync branch. If the branch does not exist, it creates an orphan branch.
func (g *gitHelper) CheckoutSyncBranch() ([]byte, error) {
	checkoutCmd := exec.Command("git", "checkout", g.syncBranch)
	checkoutCmd.Dir = g.repoDir
	out, err := checkoutCmd.CombinedOutput()
	if err != nil {
		// If the sync branch doesn't exist, create it as an orphan branch.
		if strings.Contains(string(out), "did not match any file(s) known to git") {
			checkoutCmd = exec.Command("git", "switch", "--orphan", g.syncBranch)
			checkoutCmd.Dir = g.repoDir
			out, err = checkoutCmd.CombinedOutput()
			if err != nil {
				return out, fmt.Errorf("failed to create orphan branch: %w", err)
			}
		} else {
			return out, fmt.Errorf("failed to checkout sync branch: %w", err)
		}

		// Make an empty initial commit.
		commitCmd := exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
		commitCmd.Dir = g.repoDir
		out, err = commitCmd.CombinedOutput()
		if err != nil {
			return out, fmt.Errorf("failed to commit initial commit: %w", err)
		}

		// Push the commit.
		pushCmd := exec.Command("git", "push", "origin", g.syncBranch)
		pushCmd.Dir = g.repoDir
		out, err = pushCmd.CombinedOutput()
		if err != nil {
			return out, fmt.Errorf("failed to push sync branch: %w", err)
		}
	}
	return nil, nil
}

// CheckoutTargetBranch checks out the target branch. If the branch does not exist, it creates an empty branch based on
// the sync branch.
func (g *gitHelper) CheckoutTargetBranch() ([]byte, error) {
	checkoutCmd := exec.Command("git", "checkout", g.targetBranch)
	checkoutCmd.Dir = g.repoDir
	out, err := checkoutCmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "did not match any file(s) known to git") {
			// If the branch does not exist, create any empty branch based on the sync branch
			// First, checkout the sync branch.
			checkoutCmd = exec.Command("git", "checkout", g.syncBranch)
			checkoutCmd.Dir = g.repoDir
			out, err = checkoutCmd.CombinedOutput()
			if err != nil {
				return out, fmt.Errorf("failed to checkout sync branch: %w", err)
			}

			checkoutCmd = exec.Command("git", "checkout", "-b", g.targetBranch)
			checkoutCmd.Dir = g.repoDir
			out, err = checkoutCmd.CombinedOutput()
			if err != nil {
				return out, fmt.Errorf("failed to create branch: %w", err)
			}
		} else {
			return out, fmt.Errorf("failed to checkout branch: %w", err)
		}
	}
	return nil, nil
}

// RemoveContents removes all files from the git repository.
func (g *gitHelper) RemoveContents() ([]byte, error) {
	rmCmd := exec.Command("git", "rm", "-r", "--ignore-unmatch", ".")
	rmCmd.Dir = g.repoDir
	out, err := rmCmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("failed to clear repo contents: %w", err)
	}
	return nil, nil
}

// CommitAndPush commits and pushes changes to the target branch.
func (g *gitHelper) CommitAndPush(message string) ([]byte, error) {
	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = g.repoDir
	out, err := addCmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("failed to add files: %w", err)
	}

	commitCmd := exec.Command("git", "commit", "-m", message)
	commitCmd.Dir = g.repoDir
	out, err = commitCmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "nothing to commit, working tree clean") {
			return out, nil
		}
		return out, fmt.Errorf("failed to commit: %w", err)
	}

	pushCmd := exec.Command("git", "push", "origin", g.targetBranch)
	pushCmd.Dir = g.repoDir
	out, err = pushCmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("failed to push: %w", err)
	}

	return out, nil
}
