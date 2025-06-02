// Four subcommands:
//   • setup-fix       (requires --release, --fix-suffix)
//   • promote-fix     (requires --fix-branch, --proposal-branch)
//   • sync-fork       (syncs both forks with upstream)
//   • work-on         (requires --dev-branch, --suffix)
//
// In CI mode (GITHUB_ACTIONS=true), any conflict aborts and prints a short
// "run locally" instruction. Locally, conflicts leave you inside a cherry-pick for
// manual resolution.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// CommandRunner abstracts command execution for better testability
type CommandRunner interface {
	// Run executes a command with output to stdout/stderr
	Run(cmdName string, args ...string) error
	// RunOrExit executes a command and exits on failure
	RunOrExit(cmdName string, args ...string)
	// RunWithOutput executes a command and returns its stdout output
	RunWithOutput(cmdName string, args ...string) (string, error)
	// RunAndCaptureOrExit executes a command, captures its output, exits on error
	RunAndCaptureOrExit(cmdName string, args ...string) string
	// BranchExists checks if a git branch exists
	BranchExists(ref string) bool
	// TagExists checks if a git tag exists
	TagExists(tag string) bool
	// IsCI checks if running in CI environment
	IsCI() bool
	// ExitWithError prints error message and exits
	ExitWithError(format string, args ...any)
	// HasUncommittedChanges checks if working directory is dirty
	HasUncommittedChanges() bool
	// IsBranchUpToDate checks if local branch is up to date with remote
	IsBranchUpToDate(localBranch, remoteBranch string) bool
}

// DefaultRunner is the standard command runner used in production
type DefaultRunner struct {
	stdout io.Writer
	stderr io.Writer
	exitFn func(int)
	env    map[string]string
}

// NewDefaultRunner creates a runner with standard configuration
func NewDefaultRunner() *DefaultRunner {
	return &DefaultRunner{
		stdout: os.Stdout,
		stderr: os.Stderr,
		exitFn: os.Exit,
		env:    envToMap(),
	}
}

func envToMap() map[string]string {
	result := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

func (r *DefaultRunner) IsCI() bool {
	return r.env["GITHUB_ACTIONS"] == "true"
}

func (r *DefaultRunner) Run(cmdName string, args ...string) error {
	cmd := exec.Command(cmdName, args...)
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr
	return cmd.Run()
}

func (r *DefaultRunner) RunOrExit(cmdName string, args ...string) {
	if err := r.Run(cmdName, args...); err != nil {
		fmt.Fprintf(r.stderr, "ERROR: command failed: %s %s\n", cmdName, strings.Join(args, " "))
		r.exitFn(1)
	}
}

func (r *DefaultRunner) RunWithOutput(cmdName string, args ...string) (string, error) {
	var stdout bytes.Buffer
	cmd := exec.Command(cmdName, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = r.stderr
	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), err
}

func (r *DefaultRunner) RunAndCaptureOrExit(cmdName string, args ...string) string {
	output, err := r.RunWithOutput(cmdName, args...)
	if err != nil {
		fmt.Fprintf(r.stderr, "ERROR: '%s %s' failed\n", cmdName, strings.Join(args, " "))
		r.exitFn(1)
	}
	return output
}

func (r *DefaultRunner) BranchExists(ref string) bool {
	err := exec.Command("git", "rev-parse", "--verify", "--quiet", ref).Run()
	return err == nil
}

func (r *DefaultRunner) TagExists(tag string) bool {
	err := exec.Command("git", "rev-parse", "--verify", "--quiet", "tags/"+tag).Run()
	return err == nil
}

func (r *DefaultRunner) HasUncommittedChanges() bool {
	// Check if working directory is clean
	output, err := r.RunWithOutput("git", "status", "--porcelain")
	if err != nil {
		return true // assume dirty on error
	}
	return strings.TrimSpace(output) != ""
}

func (r *DefaultRunner) IsBranchUpToDate(localBranch, remoteBranch string) bool {
	// Get the commit hash of local branch
	localHash, err := r.RunWithOutput("git", "rev-parse", localBranch)
	if err != nil {
		return false
	}

	// Get the commit hash of remote branch
	remoteHash, err := r.RunWithOutput("git", "rev-parse", remoteBranch)
	if err != nil {
		return false
	}

	return localHash == remoteHash
}

func (r *DefaultRunner) ExitWithError(format string, args ...any) {
	fmt.Fprintf(r.stderr, format+"\n", args...)
	r.exitFn(1)
}

// Core CLI application
func main() {
	runner := NewDefaultRunner()
	exitCode := run(os.Args, runner)
	os.Exit(exitCode)
}

func run(args []string, runner CommandRunner) int {
	if len(args) < 2 {
		usage(os.Stderr)
		return 1
	}

	switch args[1] {
	case "setup-fix":
		return setupFixCmd(args[2:], runner)
	case "promote-fix":
		return promoteFixCmd(args[2:], runner)
	case "sync-fork":
		return syncForkCmd(args[2:], runner)
	case "work-on":
		return workOnCmd(args[2:], runner)
	default:
		usage(os.Stderr)
		return 1
	}
}

func usage(w io.Writer) {
	fmt.Fprintf(w, `
Usage:
  cli setup-fix --release=<tag> --fix-suffix=<suffix>
  cli promote-fix --fix-branch=<branch> --proposal-branch=<name>
  cli sync-fork
  cli work-on --dev-branch=<branch> --suffix=<suffix>

Environment:
  • Set GITHUB_ACTIONS=true for CI mode. On conflict in CI, the CLI prints a
    one-liner telling you to run it locally, then exits 1.
  • Locally (GITHUB_ACTIONS unset), conflicts leave you inside a cherry-pick for
    manual resolution.

Commands:

  setup-fix
    • --release        exact tag to base off (e.g. v2.9.14)
    • --fix-suffix     short name (e.g. fix-issue-123)

  promote-fix
    • --fix-branch       must be "skyscanner-internal/develop/<release>/<suffix>"
    • --proposal-branch  short name under skyscanner-contrib/proposal/ (e.g. fix-issue-123)
    
  sync-fork
    • Syncs both forks with upstream GitHub repository (argoproj/argo-cd)
    • Rebases skyscanner-contrib/master on top of upstream/master
    • Rebases skyscanner-internal/master on top of skyscanner-contrib/master

  work-on
    • --dev-branch     release-pinned dev branch (e.g. skyscanner-internal/develop/v2.14.9/fix-issue-123)
    • --suffix         short name for feature branch (e.g. add-logging)
`)
}

// setupFixCmd: rebase internal/master → create release branch → import CI → push.
func setupFixCmd(args []string, runner CommandRunner) int {
	fs := flag.NewFlagSet("setup-fix", flag.ExitOnError)
	release := fs.String("release", "", "exact tag to base off (e.g. v2.9.14)")
	fixSuffix := fs.String("fix-suffix", "", "short name for fix branch (e.g. fix-issue-123)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		return 1
	}

	if *release == "" || *fixSuffix == "" {
		fs.Usage()
		return 1
	}

	// Check for uncommitted changes
	if runner.HasUncommittedChanges() {
		fmt.Fprintln(os.Stderr, "❌ You have uncommitted changes. Please commit or stash them before running setup-fix.")
		fmt.Fprintln(os.Stderr, "   git stash push -m \"WIP before setup-fix\"")
		fmt.Fprintln(os.Stderr, "   # or commit your changes")
		return 1
	}

	// Check if the tag exists
	if !runner.TagExists(*release) {
		fmt.Fprintf(os.Stderr, "❌ Tag '%s' does not exist. Please fetch latest tags:\n", *release)
		fmt.Fprintln(os.Stderr, "   git fetch --tags")
		fmt.Fprintln(os.Stderr, "   # or run sync-fork to update everything")
		return 1
	}

	// 1) Fetch remotes
	runner.RunOrExit("git", "fetch", "origin", "skyscanner-contrib/master:skyscanner-contrib/master")

	// Check if skyscanner-internal/master is up to date
	if !runner.IsBranchUpToDate("skyscanner-internal/master", "origin/skyscanner-internal/master") {
		fmt.Fprintln(os.Stderr, "❌ Your local skyscanner-internal/master is not up to date with origin.")
		fmt.Fprintln(os.Stderr, "   Please run sync-fork first to update all branches:")
		fmt.Fprintln(os.Stderr, "   go run tools/fork-cli/main.go sync-fork")
		return 1
	}

	// 2) Rebase internal/master onto contrib/master
	runner.RunOrExit("git", "checkout", "skyscanner-internal/master")
	if err := runner.Run("git", "rebase", "skyscanner-contrib/master"); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: rebase conflict. Resolve manually, then `git rebase --continue`.")
		return 1
	}
	runner.RunOrExit("git", "push", "--force", "origin", "skyscanner-internal/master")

	// 3) Create release‐based branch
	newBranch := fmt.Sprintf("skyscanner-internal/develop/%s/%s", *release, *fixSuffix)
	runner.RunOrExit("git", "checkout", "tags/"+*release, "-b", newBranch)

	// 4) Import .github/ and tools/fork-cli from internal/master
	runner.RunOrExit("git", "checkout", "skyscanner-internal/master", "--", ".github")
	runner.RunOrExit("git", "checkout", "skyscanner-internal/master", "--", "tools/fork-cli")
	runner.RunOrExit("git", "commit", "-m", "chore: import CI and fork-cli tools into "+newBranch)

	// 5) Push new branch
	runner.RunOrExit("git", "push", "-u", "origin", newBranch)

	fmt.Printf("✅ Created branch %s\n", newBranch)
	return 0
}

// promoteFixCmd: create proposal branch only, don't touch internal/master.
func promoteFixCmd(args []string, runner CommandRunner) int {
	fs := flag.NewFlagSet("promote-fix", flag.ExitOnError)
	fixBranch := fs.String("fix-branch", "", "e.g. skyscanner-internal/develop/v2.9.14/fix-issue-123")
	proposal := fs.String("proposal-branch", "", "e.g. fix-issue-123)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		return 1
	}

	if *fixBranch == "" || *proposal == "" {
		fs.Usage()
		return 1
	}

	ciMode := runner.IsCI()

	// 1) Extract release tag from fixBranch
	parts := strings.Split(*fixBranch, "/")
	if len(parts) < 4 || parts[0] != "skyscanner-internal" || parts[1] != "develop" {
		fmt.Fprintf(os.Stderr, "ERROR: fix-branch must be 'skyscanner-internal/develop/<release>/<suffix>'. Got '%s'\n", *fixBranch)
		return 1
	}

	// 2) Fetch necessary refs
	runner.RunOrExit("git", "fetch", "origin", "skyscanner-contrib/master:skyscanner-contrib/master")

	// Check if skyscanner-contrib/master is up to date
	if !runner.IsBranchUpToDate("skyscanner-contrib/master", "origin/skyscanner-contrib/master") {
		fmt.Fprintln(os.Stderr, "❌ Your local skyscanner-contrib/master is not up to date with origin.")
		fmt.Fprintln(os.Stderr, "   Please commit/stash local changes and run sync-fork first:")
		fmt.Fprintln(os.Stderr, "   git stash push -m \"WIP before sync-fork\"")
		fmt.Fprintln(os.Stderr, "   go run tools/fork-cli/main.go sync-fork")
		return 1
	}

	baseRelease := parts[2] // e.g. v2.9.14

	// 3) Compute merge-base & commit-range
	gb := strings.TrimSpace(runner.RunAndCaptureOrExit("git", "merge-base", *fixBranch, baseRelease))
	if gb == "" {
		fmt.Fprintf(os.Stderr, "ERROR: cannot find merge-base between %s and %s\n", *fixBranch, baseRelease)
		return 1
	}
	commitRange := fmt.Sprintf("%s..%s", gb, *fixBranch)
	fmt.Printf("Cherry-pick range: %s\n\n", commitRange)

	// Cherry-pick into skyscanner-contrib/proposal/<proposal>
	proposalFull := "skyscanner-contrib/proposal/" + *proposal
	fmt.Printf("→ Creating/updating %s off skyscanner-contrib/master …\n", proposalFull)
	runner.RunOrExit("git", "checkout", "skyscanner-contrib/master")
	if runner.BranchExists(proposalFull) {
		runner.RunOrExit("git", "branch", "-D", proposalFull)
	}
	runner.RunOrExit("git", "checkout", "-b", proposalFull)

	fmt.Printf("→ Cherry-picking into %s …\n", proposalFull)
	err := runner.Run("git", "cherry-pick", "--keep-redundant-commits", commitRange)
	if err != nil {
		if ciMode {
			printConflictMessage(os.Stderr, baseRelease, *fixBranch, *proposal)
			return 1
		}
		fmt.Fprintln(os.Stderr, "⚠️  Conflict detected in proposal branch. Resolve manually and then:")
		fmt.Fprintln(os.Stderr, "    git cherry-pick --continue")
		return 1
	}
	fmt.Printf("✅ %s is ready for upstream contribution\n\n", proposalFull)
	fmt.Println("Next steps:")
	fmt.Printf("  git push origin %s\n", proposalFull)
	fmt.Println("  # Then create a PR to argoproj/argo-cd:master from this branch")
	return 0
}

// syncForkCmd: Sync forks by rebasing branches on top of their upstream counterparts.
func syncForkCmd(args []string, runner CommandRunner) int {
	fs := flag.NewFlagSet("sync-fork", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		return 1
	}

	// Check for uncommitted changes first
	if runner.HasUncommittedChanges() {
		fmt.Fprintln(os.Stderr, "❌ You have uncommitted changes. Please commit or stash them before syncing:")
		fmt.Fprintln(os.Stderr, "   git stash push -m \"WIP before sync-fork\"")
		fmt.Fprintln(os.Stderr, "   # or commit your changes")
		fmt.Fprintln(os.Stderr, "   # then re-run: go run tools/fork-cli/main.go sync-fork")
		return 1
	}

	ciMode := runner.IsCI()

	// 1) Ensure upstream remote exists
	fmt.Println("→ Ensuring upstream remote exists...")
	remoteOutput, _ := runner.RunWithOutput("git", "remote")
	remotes := strings.Fields(remoteOutput)

	hasUpstream := false
	for _, remote := range remotes {
		if remote == "upstream" {
			hasUpstream = true
			break
		}
	}

	if !hasUpstream {
		fmt.Println("→ Adding upstream remote: https://github.com/argoproj/argo-cd.git")
		runner.RunOrExit("git", "remote", "add", "upstream", "https://github.com/argoproj/argo-cd.git")
	}

	// 2) Fetch from all remotes to ensure we have latest code
	fmt.Println("→ Fetching latest changes from all remotes...")
	runner.RunOrExit("git", "fetch", "--all")

	// 3) Handle contrib fork: rebase on top of upstream
	contribBranch := "skyscanner-contrib/master"
	upstreamBranch := "upstream/master"

	fmt.Printf("→ Rebasing %s on top of %s...\n", contribBranch, upstreamBranch)

	// Checkout contrib branch
	runner.RunOrExit("git", "checkout", contribBranch)

	// Attempt to rebase on upstream
	err := runner.Run("git", "rebase", upstreamBranch)
	if err != nil {
		if ciMode {
			fmt.Fprintf(os.Stderr, "❌ Rebase conflict between %s and %s!\n", contribBranch, upstreamBranch)
			fmt.Fprintln(os.Stderr, "Please run this command locally to resolve conflicts:")
			fmt.Fprintf(os.Stderr, "  go run tools/fork-cli/main.go sync-fork\n")
			return 1
		}
		fmt.Fprintf(os.Stderr, "⚠️ Conflict detected rebasing %s on %s.\n", contribBranch, upstreamBranch)
		fmt.Fprintln(os.Stderr, "Resolve conflicts, then run:")
		fmt.Fprintln(os.Stderr, "  git rebase --continue")
		fmt.Fprintln(os.Stderr, "Once finished, re-run this command to proceed with the internal fork.")
		return 1
	}

	// Push the rebased contrib branch
	fmt.Printf("→ Pushing rebased %s...\n", contribBranch)
	runner.RunOrExit("git", "push", "--force", "origin", contribBranch)

	// 4) Handle the internal fork: rebase on top of contrib
	internalBranch := "skyscanner-internal/master"
	fmt.Printf("→ Rebasing %s on top of %s...\n", internalBranch, contribBranch)

	// Checkout internal branch
	runner.RunOrExit("git", "checkout", internalBranch)

	// Attempt to rebase on contrib
	err = runner.Run("git", "rebase", contribBranch)
	if err != nil {
		if ciMode {
			fmt.Fprintf(os.Stderr, "❌ Rebase conflict between %s and %s!\n", internalBranch, contribBranch)
			fmt.Fprintln(os.Stderr, "Please run this command locally to resolve conflicts:")
			fmt.Fprintf(os.Stderr, "  go run tools/fork-cli/main.go sync-fork\n")
			return 1
		}
		fmt.Fprintf(os.Stderr, "⚠️ Conflict detected rebasing %s on %s.\n", internalBranch, contribBranch)
		fmt.Fprintln(os.Stderr, "Resolve conflicts, then run:")
		fmt.Fprintln(os.Stderr, "  git rebase --continue")
		return 1
	}

	// Push the rebased internal branch
	fmt.Printf("→ Pushing rebased %s...\n", internalBranch)
	runner.RunOrExit("git", "push", "--force", "origin", internalBranch)

	fmt.Println("\n✅ Fork synchronization complete!")
	fmt.Printf("• %s is rebased on %s\n", contribBranch, upstreamBranch)
	fmt.Printf("• %s is rebased on %s\n", internalBranch, contribBranch)
	return 0
}

// workOnCmd: Create a feature branch and open PR against internal fork
func workOnCmd(args []string, runner CommandRunner) int {
	fs := flag.NewFlagSet("work-on", flag.ExitOnError)
	devBranch := fs.String("dev-branch", "", "release-pinned dev branch (e.g. skyscanner-internal/develop/v2.14.9/fix-issue-123)")
	suffix := fs.String("suffix", "", "short name for feature branch (e.g. add-logging)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		return 1
	}

	if *devBranch == "" || *suffix == "" {
		fs.Usage()
		return 1
	}

	// Validate dev branch format
	parts := strings.Split(*devBranch, "/")
	if len(parts) < 4 || parts[0] != "skyscanner-internal" || parts[1] != "develop" {
		fmt.Fprintf(os.Stderr, "ERROR: dev-branch must be 'skyscanner-internal/develop/<release>/<suffix>'. Got '%s'\n", *devBranch)
		return 1
	}

	// Create feature branch name
	featureBranch := fmt.Sprintf("%s-%s", *devBranch, *suffix)

	// Create and checkout feature branch
	runner.RunOrExit("git", "checkout", *devBranch)
	runner.RunOrExit("git", "checkout", "-b", featureBranch)

	fmt.Printf("✅ Created feature branch: %s\n", featureBranch)
	fmt.Printf("→ Base branch: %s\n", *devBranch)

	// Read current VERSION file
	currentVersion, err := runner.RunWithOutput("cat", "VERSION")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Could not read VERSION file: %v\n", err)
		return 1
	}
	currentVersion = strings.TrimSpace(currentVersion)

	// Update VERSION file with suffix
	newVersion := fmt.Sprintf("%s-%s", currentVersion, *suffix)
	runner.RunOrExit("sh", "-c", fmt.Sprintf("echo '%s' > VERSION", newVersion))

	// Commit the VERSION change
	runner.RunOrExit("git", "add", "VERSION")
	commitMsg := fmt.Sprintf("feat: %s\n\nUpdate VERSION to %s", *suffix, newVersion)
	runner.RunOrExit("git", "commit", "-m", commitMsg)

	fmt.Printf("✅ Updated VERSION from %s to %s\n", currentVersion, newVersion)

	fmt.Println("Setting up default repo and creating PR...")

	// Set default repo to fork (not upstream)
	runner.RunOrExit("gh", "repo", "set-default", "Skyscanner/argo-cd")

	// Push the branch
	runner.RunOrExit("git", "push", "-u", "origin", featureBranch)

	// Create PR automatically
	title := fmt.Sprintf("feat: %s", *suffix)
	body := fmt.Sprintf("Automated PR created via fork-cli\n\nUpdates VERSION to %s", newVersion)
	runner.RunOrExit("gh", "pr", "create", "--base", *devBranch, "--title", title, "--body", body)

	fmt.Println("✅ PR created successfully!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("1. Make additional changes if needed")
	fmt.Println("2. Commit and push any further changes")

	return 0
}

func printConflictMessage(w io.Writer, releaseTag, fixBranch, proposal string) {
	cmd := fmt.Sprintf("go run tools/fork-cli/main.go promote-fix --fix-branch=\"%s\" --proposal-branch=\"%s\"",
		fixBranch, proposal)
	fmt.Fprintf(w, "\n❌ Conflict detected during promotion of release %s.\n", releaseTag)
	fmt.Fprintln(w, "Please re-run this command locally to resolve interactively:")
	fmt.Fprintf(w, "  %s\n\n", cmd)
}
