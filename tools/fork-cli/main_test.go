package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockRunner implements CommandRunner for testing
type MockRunner struct {
	t                     *testing.T
	expectedCmds          []expectedCmd
	ciMode                bool
	exitCalls             []string
	branchesExist         map[string]bool
	tagsExist             map[string]bool
	output                *bytes.Buffer
	cmdIndex              int
	hasUncommittedChanges bool
	branchUpToDate        map[string]bool
}

type expectedCmd struct {
	command string
	args    []string
	output  string
	err     error
}

func NewMockRunner(t *testing.T) *MockRunner {
	t.Helper()
	return &MockRunner{
		t:              t,
		expectedCmds:   []expectedCmd{},
		branchesExist:  make(map[string]bool),
		tagsExist:      make(map[string]bool),
		output:         &bytes.Buffer{},
		branchUpToDate: make(map[string]bool),
	}
}

func (m *MockRunner) ExpectCommand(command string, args ...string) *MockRunner {
	m.expectedCmds = append(m.expectedCmds, expectedCmd{
		command: command,
		args:    args,
		output:  "",
		err:     nil,
	})
	return m
}

func (m *MockRunner) WithOutput(output string) *MockRunner {
	lastIdx := len(m.expectedCmds) - 1
	if lastIdx >= 0 {
		m.expectedCmds[lastIdx].output = output
	}
	return m
}

func (m *MockRunner) WithError(err error) *MockRunner {
	lastIdx := len(m.expectedCmds) - 1
	if lastIdx >= 0 {
		m.expectedCmds[lastIdx].err = err
	}
	return m
}

func (m *MockRunner) SetCIMode(enabled bool) *MockRunner {
	m.ciMode = enabled
	return m
}

func (m *MockRunner) SetBranchExists(branch string, exists bool) *MockRunner {
	m.branchesExist[branch] = exists
	return m
}

func (m *MockRunner) SetTagExists(tag string, exists bool) *MockRunner {
	m.tagsExist[tag] = exists
	return m
}

func (m *MockRunner) SetHasUncommittedChanges(dirty bool) *MockRunner {
	m.hasUncommittedChanges = dirty
	return m
}

func (m *MockRunner) SetBranchUpToDate(localBranch string, upToDate bool) *MockRunner {
	m.branchUpToDate[localBranch] = upToDate
	return m
}

func (m *MockRunner) VerifyExpectations() {
	if m.cmdIndex != len(m.expectedCmds) {
		m.t.Errorf("Not all expected commands were executed. Expected %d, got %d",
			len(m.expectedCmds), m.cmdIndex)
	}
}

func (m *MockRunner) Run(cmdName string, args ...string) error {
	return m.checkCommand(cmdName, args, false)
}

func (m *MockRunner) RunOrExit(cmdName string, args ...string) {
	if err := m.checkCommand(cmdName, args, false); err != nil {
		m.exitCalls = append(m.exitCalls, fmt.Sprintf("%s %s", cmdName, strings.Join(args, " ")))
	}
}

func (m *MockRunner) RunWithOutput(cmdName string, args ...string) (string, error) {
	err := m.checkCommand(cmdName, args, true)
	if m.cmdIndex-1 >= 0 && m.cmdIndex-1 < len(m.expectedCmds) {
		return m.expectedCmds[m.cmdIndex-1].output, err
	}
	return "", err
}

func (m *MockRunner) RunAndCaptureOrExit(cmdName string, args ...string) string {
	output, err := m.RunWithOutput(cmdName, args...)
	if err != nil {
		m.exitCalls = append(m.exitCalls, fmt.Sprintf("%s %s", cmdName, strings.Join(args, " ")))
		return ""
	}
	return output
}

func (m *MockRunner) BranchExists(ref string) bool {
	exists, ok := m.branchesExist[ref]
	if !ok {
		m.t.Logf("Branch existence check not mocked for: %s", ref)
		return false
	}
	return exists
}

func (m *MockRunner) TagExists(tag string) bool {
	exists, ok := m.tagsExist[tag]
	if !ok {
		m.t.Logf("Tag existence check not mocked for: %s", tag)
		return false
	}
	return exists
}

func (m *MockRunner) HasUncommittedChanges() bool {
	return m.hasUncommittedChanges
}

func (m *MockRunner) IsBranchUpToDate(localBranch, remoteBranch string) bool {
	upToDate, ok := m.branchUpToDate[localBranch]
	if !ok {
		m.t.Logf("Branch up-to-date check not mocked for: %s", localBranch)
		return false
	}
	return upToDate
}

func (m *MockRunner) IsCI() bool {
	return m.ciMode
}

func (m *MockRunner) ExitWithError(format string, args ...any) {
	m.exitCalls = append(m.exitCalls, fmt.Sprintf(format, args...))
}

func (m *MockRunner) checkCommand(cmdName string, args []string, _ bool) error {
	if m.cmdIndex >= len(m.expectedCmds) {
		m.t.Errorf("Unexpected command: %s %s", cmdName, strings.Join(args, " "))
		return errors.New("unexpected command")
	}

	expected := m.expectedCmds[m.cmdIndex]
	m.cmdIndex++

	assert.Equal(m.t, expected.command, cmdName, "Command name mismatch")
	assert.Equal(m.t, expected.args, args, "Command args mismatch")

	return expected.err
}

func TestSetupFixCmd(t *testing.T) {
	t.Run("successful setup", func(t *testing.T) {
		mock := NewMockRunner(t)
		mock.SetHasUncommittedChanges(false)
		mock.SetTagExists("v2.14.9", true)
		mock.SetBranchUpToDate("skyscanner-internal/master", true)

		// Expect the git commands to be called in sequence
		mock.ExpectCommand("git", "fetch", "origin", "skyscanner-contrib/master:skyscanner-contrib/master")
		mock.ExpectCommand("git", "checkout", "skyscanner-internal/master")
		mock.ExpectCommand("git", "rebase", "skyscanner-contrib/master")
		mock.ExpectCommand("git", "push", "--force", "origin", "skyscanner-internal/master")
		mock.ExpectCommand("git", "checkout", "tags/v2.14.9", "-b", "skyscanner-internal/develop/v2.14.9/fix-issue-123")
		mock.ExpectCommand("git", "checkout", "skyscanner-internal/master", "--", ".github")
		mock.ExpectCommand("git", "checkout", "skyscanner-internal/master", "--", "tools/fork-cli")
		mock.ExpectCommand("git", "commit", "-m", "chore: import CI and fork-cli tools into skyscanner-internal/develop/v2.14.9/fix-issue-123")
		mock.ExpectCommand("git", "push", "-u", "origin", "skyscanner-internal/develop/v2.14.9/fix-issue-123")

		exitCode := setupFixCmd([]string{"--release=v2.14.9", "--fix-suffix=fix-issue-123"}, mock)
		assert.Equal(t, 0, exitCode)
		mock.VerifyExpectations()
	})

	t.Run("fails with uncommitted changes", func(t *testing.T) {
		mock := NewMockRunner(t)
		mock.SetHasUncommittedChanges(true)

		exitCode := setupFixCmd([]string{"--release=v2.14.9", "--fix-suffix=fix-issue-123"}, mock)
		assert.Equal(t, 1, exitCode)
		mock.VerifyExpectations()
	})

	t.Run("fails with non-existent tag", func(t *testing.T) {
		mock := NewMockRunner(t)
		mock.SetHasUncommittedChanges(false)
		mock.SetTagExists("v2.14.9", false)

		exitCode := setupFixCmd([]string{"--release=v2.14.9", "--fix-suffix=fix-issue-123"}, mock)
		assert.Equal(t, 1, exitCode)
		mock.VerifyExpectations()
	})

	t.Run("fails with out-of-date internal master", func(t *testing.T) {
		mock := NewMockRunner(t)
		mock.SetHasUncommittedChanges(false)
		mock.SetTagExists("v2.14.9", true)
		mock.SetBranchUpToDate("skyscanner-internal/master", false)

		mock.ExpectCommand("git", "fetch", "origin", "skyscanner-contrib/master:skyscanner-contrib/master")

		exitCode := setupFixCmd([]string{"--release=v2.14.9", "--fix-suffix=fix-issue-123"}, mock)
		assert.Equal(t, 1, exitCode)
		mock.VerifyExpectations()
	})

	t.Run("missing arguments", func(t *testing.T) {
		mock := NewMockRunner(t)
		exitCode := setupFixCmd([]string{"--release=v2.14.9"}, mock) // Missing fix-suffix
		assert.Equal(t, 1, exitCode)
		mock.VerifyExpectations()
	})

	t.Run("rebase conflict", func(t *testing.T) {
		mock := NewMockRunner(t)
		mock.SetHasUncommittedChanges(false)
		mock.SetTagExists("v2.14.9", true)
		mock.SetBranchUpToDate("skyscanner-internal/master", true)

		mock.ExpectCommand("git", "fetch", "origin", "skyscanner-contrib/master:skyscanner-contrib/master")
		mock.ExpectCommand("git", "checkout", "skyscanner-internal/master")
		mock.ExpectCommand("git", "rebase", "skyscanner-contrib/master").WithError(errors.New("conflict"))

		exitCode := setupFixCmd([]string{"--release=v2.14.9", "--fix-suffix=fix-issue-123"}, mock)
		assert.Equal(t, 1, exitCode)
		mock.VerifyExpectations()
	})
}

func TestPromoteFixCmd(t *testing.T) {
	t.Run("successful promotion", func(t *testing.T) {
		mock := NewMockRunner(t)
		fixBranch := "skyscanner-internal/develop/v2.14.9/fix-issue-123"
		proposalBranch := "fix-issue-123"
		proposalFull := "skyscanner-contrib/proposal/" + proposalBranch

		mock.SetBranchUpToDate("skyscanner-contrib/master", true)
		mock.ExpectCommand("git", "fetch", "origin", "skyscanner-contrib/master:skyscanner-contrib/master")
		mock.ExpectCommand("git", "fetch", "origin", fixBranch+":"+fixBranch)
		mock.ExpectCommand("git", "merge-base", fixBranch, "skyscanner-internal/develop/v2.14.9").WithOutput("abcdef123456")
		mock.ExpectCommand("git", "checkout", "skyscanner-contrib/master")
		mock.SetBranchExists(proposalFull, false)
		mock.ExpectCommand("git", "checkout", "-b", proposalFull)
		mock.ExpectCommand("git", "cherry-pick", "--keep-redundant-commits", "abcdef123456.."+fixBranch)

		args := []string{"--fix-branch=" + fixBranch, "--proposal-branch=" + proposalBranch}
		exitCode := promoteFixCmd(args, mock)
		assert.Equal(t, 0, exitCode)
		mock.VerifyExpectations()
	})

	t.Run("fails with out-of-date contrib master", func(t *testing.T) {
		mock := NewMockRunner(t)
		fixBranch := "skyscanner-internal/develop/v2.14.9/fix-issue-123"
		proposalBranch := "fix-issue-123"

		mock.SetBranchUpToDate("skyscanner-contrib/master", false)
		mock.ExpectCommand("git", "fetch", "origin", "skyscanner-contrib/master:skyscanner-contrib/master")
		mock.ExpectCommand("git", "fetch", "origin", fixBranch+":"+fixBranch)

		args := []string{"--fix-branch=" + fixBranch, "--proposal-branch=" + proposalBranch}
		exitCode := promoteFixCmd(args, mock)
		assert.Equal(t, 1, exitCode)
		mock.VerifyExpectations()
	})

	t.Run("conflict in CI mode", func(t *testing.T) {
		mock := NewMockRunner(t).SetCIMode(true)
		fixBranch := "skyscanner-internal/develop/v2.14.9/fix-issue-123"
		proposalBranch := "fix-issue-123"
		proposalFull := "skyscanner-contrib/proposal/" + proposalBranch

		mock.SetBranchUpToDate("skyscanner-contrib/master", true)
		mock.ExpectCommand("git", "fetch", "origin", "skyscanner-contrib/master:skyscanner-contrib/master")
		mock.ExpectCommand("git", "fetch", "origin", fixBranch+":"+fixBranch)
		mock.ExpectCommand("git", "merge-base", fixBranch, "skyscanner-internal/develop/v2.14.9").WithOutput("abcdef123456")
		mock.ExpectCommand("git", "checkout", "skyscanner-contrib/master")
		mock.SetBranchExists(proposalFull, false)
		mock.ExpectCommand("git", "checkout", "-b", proposalFull)
		mock.ExpectCommand("git", "cherry-pick", "--keep-redundant-commits", "abcdef123456.."+fixBranch).WithError(errors.New("conflict"))

		args := []string{"--fix-branch=" + fixBranch, "--proposal-branch=" + proposalBranch}
		exitCode := promoteFixCmd(args, mock)
		assert.Equal(t, 1, exitCode)
		mock.VerifyExpectations()
	})
}

func TestSyncForkCmd(t *testing.T) {
	t.Run("successful sync with auth in CI mode", func(t *testing.T) {
		mock := NewMockRunner(t).SetCIMode(true)
		mock.SetHasUncommittedChanges(false)

		// Setup mock commands for successful execution
		mock.ExpectCommand("git", "remote").WithOutput("origin upstream")
		mock.ExpectCommand("git", "fetch", "--all")
		mock.ExpectCommand("git", "checkout", "skyscanner-contrib/master")
		mock.ExpectCommand("git", "rebase", "upstream/master")
		mock.ExpectCommand("git", "push", "--force", "origin", "skyscanner-contrib/master")
		mock.ExpectCommand("git", "checkout", "skyscanner-internal/master")
		mock.ExpectCommand("git", "rebase", "skyscanner-contrib/master")
		mock.ExpectCommand("git", "push", "--force", "origin", "skyscanner-internal/master")

		// Set test environment
		oldToken := os.Getenv("GITHUB_TOKEN")
		os.Setenv("GITHUB_TOKEN", "test-token")
		defer os.Setenv("GITHUB_TOKEN", oldToken)

		// Run the command
		exitCode := syncForkCmd([]string{}, mock)
		assert.Equal(t, 0, exitCode)
		mock.VerifyExpectations()
	})

	t.Run("fails with uncommitted changes", func(t *testing.T) {
		mock := NewMockRunner(t)
		mock.SetHasUncommittedChanges(true)

		exitCode := syncForkCmd([]string{}, mock)
		assert.Equal(t, 1, exitCode)
		mock.VerifyExpectations()
	})

	t.Run("successful sync with existing upstream", func(t *testing.T) {
		mock := NewMockRunner(t)
		mock.SetHasUncommittedChanges(false)

		// Setup mock commands for successful execution
		mock.ExpectCommand("git", "remote").WithOutput("origin upstream")
		mock.ExpectCommand("git", "fetch", "--all")
		mock.ExpectCommand("git", "checkout", "skyscanner-contrib/master")
		mock.ExpectCommand("git", "rebase", "upstream/master")
		mock.ExpectCommand("git", "push", "--force", "origin", "skyscanner-contrib/master")
		mock.ExpectCommand("git", "checkout", "skyscanner-internal/master")
		mock.ExpectCommand("git", "rebase", "skyscanner-contrib/master")
		mock.ExpectCommand("git", "push", "--force", "origin", "skyscanner-internal/master")

		// Run the command
		exitCode := syncForkCmd([]string{}, mock)
		assert.Equal(t, 0, exitCode)
		mock.VerifyExpectations()
	})

	t.Run("successful sync adding upstream", func(t *testing.T) {
		mock := NewMockRunner(t)
		mock.SetHasUncommittedChanges(false)

		// Setup mock commands for successful execution with adding upstream
		mock.ExpectCommand("git", "remote").WithOutput("origin")
		mock.ExpectCommand("git", "remote", "add", "upstream", "https://github.com/argoproj/argo-cd.git")
		mock.ExpectCommand("git", "fetch", "--all")
		mock.ExpectCommand("git", "checkout", "skyscanner-contrib/master")
		mock.ExpectCommand("git", "rebase", "upstream/master")
		mock.ExpectCommand("git", "push", "--force", "origin", "skyscanner-contrib/master")
		mock.ExpectCommand("git", "checkout", "skyscanner-internal/master")
		mock.ExpectCommand("git", "rebase", "skyscanner-contrib/master")
		mock.ExpectCommand("git", "push", "--force", "origin", "skyscanner-internal/master")

		// Run the command
		exitCode := syncForkCmd([]string{}, mock)
		assert.Equal(t, 0, exitCode)
		mock.VerifyExpectations()
	})

	t.Run("conflict in contrib rebase", func(t *testing.T) {
		mock := NewMockRunner(t)
		mock.SetHasUncommittedChanges(false)

		// Setup mock commands with a conflict during first rebase
		mock.ExpectCommand("git", "remote").WithOutput("origin upstream")
		mock.ExpectCommand("git", "fetch", "--all")
		mock.ExpectCommand("git", "checkout", "skyscanner-contrib/master")
		mock.ExpectCommand("git", "rebase", "upstream/master").WithError(errors.New("conflict"))

		// Run the command - should fail with exit code 1
		exitCode := syncForkCmd([]string{}, mock)
		assert.Equal(t, 1, exitCode)
		mock.VerifyExpectations()
	})

	t.Run("conflict in internal rebase", func(t *testing.T) {
		mock := NewMockRunner(t)
		mock.SetHasUncommittedChanges(false)

		// Setup mock commands with conflict in second rebase
		mock.ExpectCommand("git", "remote").WithOutput("origin upstream")
		mock.ExpectCommand("git", "fetch", "--all")
		mock.ExpectCommand("git", "checkout", "skyscanner-contrib/master")
		mock.ExpectCommand("git", "rebase", "upstream/master")
		mock.ExpectCommand("git", "push", "--force", "origin", "skyscanner-contrib/master")
		mock.ExpectCommand("git", "checkout", "skyscanner-internal/master")
		mock.ExpectCommand("git", "rebase", "skyscanner-contrib/master").WithError(errors.New("conflict"))

		// Run the command - should fail with exit code 1
		exitCode := syncForkCmd([]string{}, mock)
		assert.Equal(t, 1, exitCode)
		mock.VerifyExpectations()
	})

	t.Run("ci mode conflict handling", func(t *testing.T) {
		mock := NewMockRunner(t).SetCIMode(true)
		mock.SetHasUncommittedChanges(false)

		// Setup mock commands with conflict in CI mode
		mock.ExpectCommand("git", "remote").WithOutput("origin upstream")
		mock.ExpectCommand("git", "fetch", "--all")
		mock.ExpectCommand("git", "checkout", "skyscanner-contrib/master")
		mock.ExpectCommand("git", "rebase", "upstream/master").WithError(errors.New("conflict"))

		// Run the command - should fail with exit code 1
		exitCode := syncForkCmd([]string{}, mock)
		assert.Equal(t, 1, exitCode)
		mock.VerifyExpectations()
	})
}

func TestWorkOnCmd(t *testing.T) {
	t.Run("successful feature branch creation", func(t *testing.T) {
		mock := NewMockRunner(t)
		devBranch := "skyscanner-internal/develop/v2.14.9/fix-issue-123"
		suffix := "add-logging"

		mock.ExpectCommand("git", "fetch", "origin", devBranch+":"+devBranch)
		mock.ExpectCommand("git", "checkout", devBranch)
		mock.ExpectCommand("git", "checkout", "-b", "feature/add-logging")

		exitCode := workOnCmd([]string{"--dev-branch=" + devBranch, "--suffix=" + suffix}, mock)
		assert.Equal(t, 0, exitCode)
		mock.VerifyExpectations()
	})

	t.Run("invalid dev branch format", func(t *testing.T) {
		mock := NewMockRunner(t)
		devBranch := "invalid/branch/format"
		suffix := "add-logging"

		exitCode := workOnCmd([]string{"--dev-branch=" + devBranch, "--suffix=" + suffix}, mock)
		assert.Equal(t, 1, exitCode)
		mock.VerifyExpectations()
	})

	t.Run("missing arguments", func(t *testing.T) {
		mock := NewMockRunner(t)
		exitCode := workOnCmd([]string{"--dev-branch=skyscanner-internal/develop/v2.14.9/fix-issue-123"}, mock) // Missing suffix
		assert.Equal(t, 1, exitCode)
		mock.VerifyExpectations()
	})
}
