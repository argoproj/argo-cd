package git

import (
	"errors"
	"fmt"
	"strings"
)

func (m *nativeGitClient) ListRevisions(revision string, targetRevision string) ([]string, error) {
	// it happens when app just created and there is no revision yet
	if revision == "" {
		return []string{targetRevision}, nil
	}

	if !IsCommitSHA(revision) || !IsCommitSHA(targetRevision) {
		return nil, errors.New("invalid revision provided, must be SHA")
	}

	if revision == targetRevision {
		return []string{revision}, nil
	}

	out, err := m.runCmd("rev-list", "--ancestry-path", fmt.Sprintf("%s..%s", revision, targetRevision))
	if err != nil {
		return nil, err
	}
	ss := strings.Split(out, "\n")
	return ss, nil
}

func (m *nativeGitClient) DiffTree(targetRevision string) ([]string, error) {
	if !IsCommitSHA(targetRevision) {
		return []string{}, errors.New("invalid revision provided, must be SHA")
	}
	out, err := m.runCmd("diff-tree", "--no-commit-id", "--name-only", "-r", targetRevision)
	if err != nil {
		return nil, fmt.Errorf("failed to diff %s: %w", targetRevision, err)
	}

	if out == "" {
		return []string{}, nil
	}

	files := strings.Split(out, "\n")
	return files, nil
}
