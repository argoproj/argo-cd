package git

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runCmd(workingDir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = workingDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func outputCmd(workingDir string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = workingDir
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

func _createEmptyGitRepo() (string, error) {
	tempDir, err := os.MkdirTemp("", "")
	if err != nil {
		return tempDir, err
	}

	err = runCmd(tempDir, "git", "init")
	if err != nil {
		return tempDir, err
	}

	err = runCmd(tempDir, "git", "commit", "-m", "Initial commit", "--allow-empty")
	return tempDir, err
}

func Test_nativeGitClient_Fetch(t *testing.T) {
	tempDir, err := _createEmptyGitRepo()
	require.NoError(t, err)

	client, err := NewClient(fmt.Sprintf("file://%s", tempDir), NopCreds{}, true, false, "", "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	err = client.Fetch("")
	require.NoError(t, err)
}

func Test_nativeGitClient_Fetch_Prune(t *testing.T) {
	tempDir, err := _createEmptyGitRepo()
	require.NoError(t, err)

	client, err := NewClient(fmt.Sprintf("file://%s", tempDir), NopCreds{}, true, false, "", "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	err = runCmd(tempDir, "git", "branch", "test/foo")
	require.NoError(t, err)

	err = client.Fetch("")
	require.NoError(t, err)

	err = runCmd(tempDir, "git", "branch", "-d", "test/foo")
	require.NoError(t, err)
	err = runCmd(tempDir, "git", "branch", "test/foo/bar")
	require.NoError(t, err)

	err = client.Fetch("")
	require.NoError(t, err)
}

func Test_IsAnnotatedTag(t *testing.T) {
	tempDir := t.TempDir()
	client, err := NewClient(fmt.Sprintf("file://%s", tempDir), NopCreds{}, true, false, "", "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	p := path.Join(client.Root(), "README")
	f, err := os.Create(p)
	require.NoError(t, err)
	_, err = f.WriteString("Hello.")
	require.NoError(t, err)
	err = f.Close()
	require.NoError(t, err)

	err = runCmd(client.Root(), "git", "add", "README")
	require.NoError(t, err)

	err = runCmd(client.Root(), "git", "commit", "-m", "Initial commit", "-a")
	require.NoError(t, err)

	atag := client.IsAnnotatedTag("master")
	assert.False(t, atag)

	err = runCmd(client.Root(), "git", "tag", "some-tag", "-a", "-m", "Create annotated tag")
	require.NoError(t, err)
	atag = client.IsAnnotatedTag("some-tag")
	assert.True(t, atag)

	// Tag effectually points to HEAD, so it's considered the same
	atag = client.IsAnnotatedTag("HEAD")
	assert.True(t, atag)

	err = runCmd(client.Root(), "git", "rm", "README")
	require.NoError(t, err)
	err = runCmd(client.Root(), "git", "commit", "-m", "remove README", "-a")
	require.NoError(t, err)

	// We moved on, so tag doesn't point to HEAD anymore
	atag = client.IsAnnotatedTag("HEAD")
	assert.False(t, atag)
}

func Test_ChangedFiles(t *testing.T) {
	tempDir := t.TempDir()

	client, err := NewClientExt(fmt.Sprintf("file://%s", tempDir), tempDir, NopCreds{}, true, false, "", "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	err = runCmd(client.Root(), "git", "commit", "-m", "Initial commit", "--allow-empty")
	require.NoError(t, err)

	// Create a tag to have a second ref
	err = runCmd(client.Root(), "git", "tag", "some-tag")
	require.NoError(t, err)

	p := path.Join(client.Root(), "README")
	f, err := os.Create(p)
	require.NoError(t, err)
	_, err = f.WriteString("Hello.")
	require.NoError(t, err)
	err = f.Close()
	require.NoError(t, err)

	err = runCmd(client.Root(), "git", "add", "README")
	require.NoError(t, err)

	err = runCmd(client.Root(), "git", "commit", "-m", "Changes", "-a")
	require.NoError(t, err)

	previousSHA, err := client.LsRemote("some-tag")
	require.NoError(t, err)

	commitSHA, err := client.LsRemote("HEAD")
	require.NoError(t, err)

	// Invalid commits, error
	_, err = client.ChangedFiles("0000000000000000000000000000000000000000", "1111111111111111111111111111111111111111")
	require.Error(t, err)

	// Not SHAs, error
	_, err = client.ChangedFiles(previousSHA, "HEAD")
	require.Error(t, err)

	// Same commit, no changes
	changedFiles, err := client.ChangedFiles(commitSHA, commitSHA)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{}, changedFiles)

	// Different ref, with changes
	changedFiles, err = client.ChangedFiles(previousSHA, commitSHA)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"README"}, changedFiles)
}

func Test_SemverTags(t *testing.T) {
	tempDir := t.TempDir()

	client, err := NewClientExt(fmt.Sprintf("file://%s", tempDir), tempDir, NopCreds{}, true, false, "", "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	mapTagRefs := map[string]string{}
	for _, tag := range []string{
		"v1.0.0-rc1",
		"v1.0.0-rc2",
		"v1.0.0",
		"v1.0",
		"v1.0.1",
		"v1.1.0",
		"2024-apple",
		"2024-banana",
	} {
		err = runCmd(client.Root(), "git", "commit", "-m", tag+" commit", "--allow-empty")
		require.NoError(t, err)

		// Create an rc semver tag
		err = runCmd(client.Root(), "git", "tag", tag)
		require.NoError(t, err)

		sha, err := client.LsRemote("HEAD")
		require.NoError(t, err)

		mapTagRefs[tag] = sha
	}

	for _, tc := range []struct {
		name     string
		ref      string
		expected string
		error    bool
	}{{
		name:     "pinned rc version",
		ref:      "v1.0.0-rc1",
		expected: mapTagRefs["v1.0.0-rc1"],
	}, {
		name:     "lt rc constraint",
		ref:      "< v1.0.0-rc3",
		expected: mapTagRefs["v1.0.0-rc2"],
	}, {
		name:     "pinned major version",
		ref:      "v1.0.0",
		expected: mapTagRefs["v1.0.0"],
	}, {
		name:     "pinned patch version",
		ref:      "v1.0.1",
		expected: mapTagRefs["v1.0.1"],
	}, {
		name:     "pinned minor version",
		ref:      "v1.1.0",
		expected: mapTagRefs["v1.1.0"],
	}, {
		name:     "patch wildcard constraint",
		ref:      "v1.0.*",
		expected: mapTagRefs["v1.0.1"],
	}, {
		name:     "patch tilde constraint",
		ref:      "~v1.0.0",
		expected: mapTagRefs["v1.0.1"],
	}, {
		name:     "minor wildcard constraint",
		ref:      "v1.*",
		expected: mapTagRefs["v1.1.0"],
	}, {
		// The semver library allows for using both * and x as the wildcard modifier.
		name:     "alternative minor wildcard constraint",
		ref:      "v1.x",
		expected: mapTagRefs["v1.1.0"],
	}, {
		name:     "minor gte constraint",
		ref:      ">= v1.0.0",
		expected: mapTagRefs["v1.1.0"],
	}, {
		name:     "multiple constraints",
		ref:      "> v1.0.0 < v1.1.0",
		expected: mapTagRefs["v1.0.1"],
	}, {
		// We treat non-specific semver versions as regular tags, rather than constraints.
		name:     "non-specific version",
		ref:      "v1.0",
		expected: mapTagRefs["v1.0"],
	}, {
		// Which means a missing tag will raise an error.
		name:  "missing non-specific version",
		ref:   "v1.1",
		error: true,
	}, {
		// This is NOT a semver constraint, so it should always resolve to itself - because specifying a tag should
		// return the commit for that tag.
		// semver/v3 has the unfortunate semver-ish behaviour where any tag starting with a number is considered to be
		// "semver-ish", where that number is the semver major version, and the rest then gets coerced into a beta
		// version string. This can cause unexpected behaviour with constraints logic.
		// In this case, if the tag is being incorrectly coerced into semver (for being semver-ish), it will incorrectly
		// return the commit for the 2024-banana tag; which we want to avoid.
		name:     "apple non-semver tag",
		ref:      "2024-apple",
		expected: mapTagRefs["2024-apple"],
	}, {
		name:     "banana non-semver tag",
		ref:      "2024-banana",
		expected: mapTagRefs["2024-banana"],
	}, {
		// A semver version (without constraints) should ONLY match itself.
		// We do not want "2024-apple" to get "semver-ish'ed" into matching "2024.0.0-apple"; they're different tags.
		name:  "no semver tag coercion",
		ref:   "2024.0.0-apple",
		error: true,
	}, {
		// No minor versions are specified, so we would expect a major version of 2025 or more.
		// This is because if we specify > 11 in semver, we would not expect 11.1.0 to pass; it should be 12.0.0 or more.
		// Similarly, if we were to specify > 11.0, we would expect 11.1.0 or more.
		name:  "semver constraints on non-semver tags",
		ref:   "> 2024-apple",
		error: true,
	}, {
		// However, if one specifies the minor/patch versions, semver constraints can be used to match non-semver tags.
		// 2024-banana is considered as "2024.0.0-banana" in semver-ish, and banana > apple, so it's a match.
		// Note: this is more for documentation and future reference than real testing, as it seems like quite odd behaviour.
		name:     "semver constraints on non-semver tags",
		ref:      "> 2024.0.0-apple",
		expected: mapTagRefs["2024-banana"],
	}} {
		t.Run(tc.name, func(t *testing.T) {
			commitSHA, err := client.LsRemote(tc.ref)
			if tc.error {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.True(t, IsCommitSHA(commitSHA))
			assert.Equal(t, tc.expected, commitSHA)
		})
	}
}

func Test_nativeGitClient_Submodule(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)

	foo := filepath.Join(tempDir, "foo")
	err = os.Mkdir(foo, 0o755)
	require.NoError(t, err)

	err = runCmd(foo, "git", "init")
	require.NoError(t, err)

	bar := filepath.Join(tempDir, "bar")
	err = os.Mkdir(bar, 0o755)
	require.NoError(t, err)

	err = runCmd(bar, "git", "init")
	require.NoError(t, err)

	err = runCmd(bar, "git", "commit", "-m", "Initial commit", "--allow-empty")
	require.NoError(t, err)

	// Embed repository bar into repository foo
	t.Setenv("GIT_ALLOW_PROTOCOL", "file")
	err = runCmd(foo, "git", "submodule", "add", bar)
	require.NoError(t, err)

	err = runCmd(foo, "git", "commit", "-m", "Initial commit")
	require.NoError(t, err)

	tempDir, err = os.MkdirTemp("", "")
	require.NoError(t, err)

	// Clone foo
	err = runCmd(tempDir, "git", "clone", foo)
	require.NoError(t, err)

	client, err := NewClient(fmt.Sprintf("file://%s", foo), NopCreds{}, true, false, "", "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	err = client.Fetch("")
	require.NoError(t, err)

	commitSHA, err := client.LsRemote("HEAD")
	require.NoError(t, err)

	// Call Checkout() with submoduleEnabled=false.
	_, err = client.Checkout(commitSHA, false)
	require.NoError(t, err)

	// Check if submodule url does not exist in .git/config
	err = runCmd(client.Root(), "git", "config", "submodule.bar.url")
	require.Error(t, err)

	// Call Submodule() via Checkout() with submoduleEnabled=true.
	_, err = client.Checkout(commitSHA, true)
	require.NoError(t, err)

	// Check if the .gitmodule URL is reflected in .git/config
	cmd := exec.Command("git", "config", "submodule.bar.url")
	cmd.Dir = client.Root()
	result, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, bar+"\n", string(result))

	// Change URL of submodule bar
	err = runCmd(client.Root(), "git", "config", "--file=.gitmodules", "submodule.bar.url", bar+"baz")
	require.NoError(t, err)

	// Call Submodule()
	err = client.Submodule()
	require.NoError(t, err)

	// Check if the URL change in .gitmodule is reflected in .git/config
	cmd = exec.Command("git", "config", "submodule.bar.url")
	cmd.Dir = client.Root()
	result, err = cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, bar+"baz\n", string(result))
}

func TestNewClient_invalidSSHURL(t *testing.T) {
	client, err := NewClient("ssh://bitbucket.org:org/repo", NopCreds{}, false, false, "", "")
	assert.Nil(t, client)
	assert.ErrorIs(t, err, ErrInvalidRepoURL)
}

func Test_IsRevisionPresent(t *testing.T) {
	tempDir := t.TempDir()

	client, err := NewClientExt(fmt.Sprintf("file://%s", tempDir), tempDir, NopCreds{}, true, false, "", "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	p := path.Join(client.Root(), "README")
	f, err := os.Create(p)
	require.NoError(t, err)
	_, err = f.WriteString("Hello.")
	require.NoError(t, err)
	err = f.Close()
	require.NoError(t, err)

	err = runCmd(client.Root(), "git", "add", "README")
	require.NoError(t, err)

	err = runCmd(client.Root(), "git", "commit", "-m", "Initial Commit", "-a")
	require.NoError(t, err)

	commitSHA, err := client.LsRemote("HEAD")
	require.NoError(t, err)

	// Ensure revision for HEAD is present locally.
	revisionPresent := client.IsRevisionPresent(commitSHA)
	assert.True(t, revisionPresent)

	// Ensure invalid revision is not returned.
	revisionPresent = client.IsRevisionPresent("invalid-revision")
	assert.False(t, revisionPresent)
}

func Test_nativeGitClient_RevisionMetadata(t *testing.T) {
	tempDir := t.TempDir()
	client, err := NewClient(fmt.Sprintf("file://%s", tempDir), NopCreds{}, true, false, "", "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	p := path.Join(client.Root(), "README")
	f, err := os.Create(p)
	require.NoError(t, err)
	_, err = f.WriteString("Hello.")
	require.NoError(t, err)
	err = f.Close()
	require.NoError(t, err)

	err = runCmd(client.Root(), "git", "config", "user.name", "FooBar ||| something\nelse")
	require.NoError(t, err)
	err = runCmd(client.Root(), "git", "config", "user.email", "foo@foo.com")
	require.NoError(t, err)

	err = runCmd(client.Root(), "git", "add", "README")
	require.NoError(t, err)
	err = runCmd(client.Root(), "git", "commit", "--date=\"Sat Jun 5 20:00:00 2021 +0000 UTC\"", "-m", `| Initial commit |


(╯°□°)╯︵ ┻━┻
		`, "-a")
	require.NoError(t, err)

	metadata, err := client.RevisionMetadata("HEAD")
	require.NoError(t, err)
	require.Equal(t, &RevisionMetadata{
		Author:  `FooBar ||| somethingelse <foo@foo.com>`,
		Date:    time.Date(2021, time.June, 5, 20, 0, 0, 0, time.UTC).Local(),
		Tags:    []string{},
		Message: "| Initial commit |\n\n(╯°□°)╯︵ ┻━┻",
	}, metadata)
}

func Test_nativeGitClient_SetAuthor(t *testing.T) {
	expectedName := "Tester"
	expectedEmail := "test@example.com"

	tempDir, err := _createEmptyGitRepo()
	require.NoError(t, err)

	client, err := NewClient(fmt.Sprintf("file://%s", tempDir), NopCreds{}, true, false, "", "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	out, err := client.SetAuthor(expectedName, expectedEmail)
	require.NoError(t, err, "error output: ", out)

	// Check git user.name
	gitUserName, err := outputCmd(client.Root(), "git", "config", "--local", "user.name")
	require.NoError(t, err)
	actualName := strings.TrimSpace(string(gitUserName))
	require.Equal(t, expectedName, actualName)

	// Check git user.email
	gitUserEmail, err := outputCmd(client.Root(), "git", "config", "--local", "user.email")
	require.NoError(t, err)
	actualEmail := strings.TrimSpace(string(gitUserEmail))
	require.Equal(t, expectedEmail, actualEmail)
}

func Test_nativeGitClient_CheckoutOrOrphan(t *testing.T) {
	t.Run("checkout to an existing branch", func(t *testing.T) {
		// not main or master
		expectedBranch := "feature"

		tempDir, err := _createEmptyGitRepo()
		require.NoError(t, err)

		client, err := NewClientExt(fmt.Sprintf("file://%s", tempDir), tempDir, NopCreds{}, true, false, "", "")
		require.NoError(t, err)

		err = client.Init()
		require.NoError(t, err)

		// set the author for the initial commit of the orphan branch
		out, err := client.SetAuthor("test", "test@example.com")
		require.NoError(t, err, "error output: %s", out)

		// get base branch
		gitCurrentBranch, err := outputCmd(tempDir, "git", "rev-parse", "--abbrev-ref", "HEAD")
		require.NoError(t, err)
		baseBranch := strings.TrimSpace(string(gitCurrentBranch))

		// get base commit
		gitCurrentCommitHash, err := outputCmd(tempDir, "git", "rev-parse", "HEAD")
		require.NoError(t, err)
		expectedCommitHash := strings.TrimSpace(string(gitCurrentCommitHash))

		// make expected branch
		err = runCmd(tempDir, "git", "checkout", "-b", expectedBranch)
		require.NoError(t, err)

		// checkout to base branch, ready to test
		err = runCmd(tempDir, "git", "checkout", baseBranch)
		require.NoError(t, err)

		out, err = client.CheckoutOrOrphan(expectedBranch, false)
		require.NoError(t, err, "error output: ", out)

		// get current branch, verify current branch
		gitCurrentBranch, err = outputCmd(tempDir, "git", "rev-parse", "--abbrev-ref", "HEAD")
		require.NoError(t, err)
		actualBranch := strings.TrimSpace(string(gitCurrentBranch))
		require.Equal(t, expectedBranch, actualBranch)

		// get current commit hash, verify current commit hash
		// equal -> not orphan
		gitCurrentCommitHash, err = outputCmd(tempDir, "git", "rev-parse", "HEAD")
		require.NoError(t, err)
		actualCommitHash := strings.TrimSpace(string(gitCurrentCommitHash))
		require.Equal(t, expectedCommitHash, actualCommitHash)
	})

	t.Run("orphan", func(t *testing.T) {
		// not main or master
		expectedBranch := "feature"

		// make origin git repository
		tempDir, err := _createEmptyGitRepo()
		require.NoError(t, err)
		originGitRepoUrl := fmt.Sprintf("file://%s", tempDir)
		err = runCmd(tempDir, "git", "commit", "-m", "Second commit", "--allow-empty")
		require.NoError(t, err)

		// get base branch
		gitCurrentBranch, err := outputCmd(tempDir, "git", "rev-parse", "--abbrev-ref", "HEAD")
		require.NoError(t, err)
		baseBranch := strings.TrimSpace(string(gitCurrentBranch))

		// make test dir
		tempDir, err = os.MkdirTemp("", "")
		require.NoError(t, err)

		client, err := NewClientExt(originGitRepoUrl, tempDir, NopCreds{}, true, false, "", "")
		require.NoError(t, err)

		err = client.Init()
		require.NoError(t, err)

		// set the author for the initial commit of the orphan branch
		out, err := client.SetAuthor("test", "test@example.com")
		require.NoError(t, err, "error output: %s", out)

		err = client.Fetch("")
		require.NoError(t, err)

		// checkout to origin base branch
		err = runCmd(tempDir, "git", "checkout", baseBranch)
		require.NoError(t, err)

		// get base commit
		gitCurrentCommitHash, err := outputCmd(tempDir, "git", "rev-parse", "HEAD")
		require.NoError(t, err)
		baseCommitHash := strings.TrimSpace(string(gitCurrentCommitHash))

		out, err = client.CheckoutOrOrphan(expectedBranch, false)
		require.NoError(t, err, "error output: ", out)

		// get current branch, verify current branch
		gitCurrentBranch, err = outputCmd(tempDir, "git", "rev-parse", "--abbrev-ref", "HEAD")
		require.NoError(t, err)
		actualBranch := strings.TrimSpace(string(gitCurrentBranch))
		require.Equal(t, expectedBranch, actualBranch)

		// check orphan branch

		// get current commit hash, verify current commit hash
		// not equal -> orphan
		gitCurrentCommitHash, err = outputCmd(tempDir, "git", "rev-parse", "HEAD")
		require.NoError(t, err)
		currentCommitHash := strings.TrimSpace(string(gitCurrentCommitHash))
		require.NotEqual(t, baseCommitHash, currentCommitHash)

		// get commit count on current branch, verify 1 -> orphan
		gitCommitCount, err := outputCmd(tempDir, "git", "rev-list", "--count", actualBranch)
		require.NoError(t, err)
		require.Equal(t, "1", strings.TrimSpace(string(gitCommitCount)))
	})
}

func Test_nativeGitClient_CheckoutOrNew(t *testing.T) {
	t.Run("checkout to an existing branch", func(t *testing.T) {
		// Example status
		// * 57aef63 (feature) Second commit
		// * a4fad22 (main) Initial commit

		// Test scenario
		// given : main branch (w/ Initial commit)
		// when  : try to check out [main -> feature]
		// then  : feature branch (w/ Second commit)

		// not main or master
		expectedBranch := "feature"

		tempDir, err := _createEmptyGitRepo()
		require.NoError(t, err)

		client, err := NewClientExt(fmt.Sprintf("file://%s", tempDir), tempDir, NopCreds{}, true, false, "", "")
		require.NoError(t, err)

		err = client.Init()
		require.NoError(t, err)

		out, err := client.SetAuthor("test", "test@example.com")
		require.NoError(t, err, "error output: %s", out)

		// get base branch
		gitCurrentBranch, err := outputCmd(tempDir, "git", "rev-parse", "--abbrev-ref", "HEAD")
		require.NoError(t, err)
		baseBranch := strings.TrimSpace(string(gitCurrentBranch))

		// make expected branch
		err = runCmd(tempDir, "git", "checkout", "-b", expectedBranch)
		require.NoError(t, err)

		// make expected commit
		err = runCmd(tempDir, "git", "commit", "-m", "Second commit", "--allow-empty")
		require.NoError(t, err)

		// get expected commit
		expectedCommitHash, err := client.CommitSHA()
		require.NoError(t, err)

		// checkout to base branch, ready to test
		err = runCmd(tempDir, "git", "checkout", baseBranch)
		require.NoError(t, err)

		out, err = client.CheckoutOrNew(expectedBranch, baseBranch, false)
		require.NoError(t, err, "error output: ", out)

		// get current branch, verify current branch
		gitCurrentBranch, err = outputCmd(tempDir, "git", "rev-parse", "--abbrev-ref", "HEAD")
		require.NoError(t, err)
		actualBranch := strings.TrimSpace(string(gitCurrentBranch))
		require.Equal(t, expectedBranch, actualBranch)

		// get current commit hash, verify current commit hash
		actualCommitHash, err := client.CommitSHA()
		require.NoError(t, err)
		require.Equal(t, expectedCommitHash, actualCommitHash)
	})

	t.Run("new", func(t *testing.T) {
		// Test scenario
		// given : main branch (w/ Initial commit)
		// 	 * a4fad22 (main) Initial commit
		// when  : try to check out [main -> feature]
		// then  : feature branch (w/ Initial commit)
		// 	 * a4fad22 (feature, main) Initial commit

		// not main or master
		expectedBranch := "feature"

		tempDir, err := _createEmptyGitRepo()
		require.NoError(t, err)

		client, err := NewClientExt(fmt.Sprintf("file://%s", tempDir), tempDir, NopCreds{}, true, false, "", "")
		require.NoError(t, err)

		err = client.Init()
		require.NoError(t, err)

		out, err := client.SetAuthor("test", "test@example.com")
		require.NoError(t, err, "error output: %s", out)

		// get base branch
		gitCurrentBranch, err := outputCmd(tempDir, "git", "rev-parse", "--abbrev-ref", "HEAD")
		require.NoError(t, err)
		baseBranch := strings.TrimSpace(string(gitCurrentBranch))

		// get expected commit
		expectedCommitHash, err := client.CommitSHA()
		require.NoError(t, err)

		out, err = client.CheckoutOrNew(expectedBranch, baseBranch, false)
		require.NoError(t, err, "error output: ", out)

		// get current branch, verify current branch
		gitCurrentBranch, err = outputCmd(tempDir, "git", "rev-parse", "--abbrev-ref", "HEAD")
		require.NoError(t, err)
		actualBranch := strings.TrimSpace(string(gitCurrentBranch))
		require.Equal(t, expectedBranch, actualBranch)

		// get current commit hash, verify current commit hash
		actualCommitHash, err := client.CommitSHA()
		require.NoError(t, err)
		require.Equal(t, expectedCommitHash, actualCommitHash)
	})
}

func Test_nativeGitClient_RemoveContents(t *testing.T) {
	// Example status
	// 2 files :
	//   * <RepoRoot>/README.md
	//   * <RepoRoot>/scripts/startup.sh

	// given
	tempDir, err := _createEmptyGitRepo()
	require.NoError(t, err)

	client, err := NewClient(fmt.Sprintf("file://%s", tempDir), NopCreds{}, true, false, "", "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	out, err := client.SetAuthor("test", "test@example.com")
	require.NoError(t, err, "error output: ", out)

	err = runCmd(client.Root(), "touch", "README.md")
	require.NoError(t, err)

	err = runCmd(client.Root(), "mkdir", "scripts")
	require.NoError(t, err)

	err = runCmd(client.Root(), "touch", "scripts/startup.sh")
	require.NoError(t, err)

	err = runCmd(client.Root(), "git", "add", "--all")
	require.NoError(t, err)

	err = runCmd(client.Root(), "git", "commit", "-m", "Make files")
	require.NoError(t, err)

	// when
	out, err = client.RemoveContents()
	require.NoError(t, err, "error output: ", out)

	// then
	ls, err := outputCmd(client.Root(), "ls", "-l")
	require.NoError(t, err)
	require.Equal(t, "total 0", strings.TrimSpace(string(ls)))
}

func Test_nativeGitClient_CommitAndPush(t *testing.T) {
	tempDir, err := _createEmptyGitRepo()
	require.NoError(t, err)

	// config receive.denyCurrentBranch updateInstead
	// because local git init make a non-bare repository which cannot be pushed normally
	err = runCmd(tempDir, "git", "config", "--local", "receive.denyCurrentBranch", "updateInstead")
	require.NoError(t, err)

	// get branch
	gitCurrentBranch, err := outputCmd(tempDir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	require.NoError(t, err)
	branch := strings.TrimSpace(string(gitCurrentBranch))

	client, err := NewClient(fmt.Sprintf("file://%s", tempDir), NopCreds{}, true, false, "", "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	out, err := client.SetAuthor("test", "test@example.com")
	require.NoError(t, err, "error output: ", out)

	err = client.Fetch(branch)
	require.NoError(t, err)

	out, err = client.Checkout(branch, false)
	require.NoError(t, err, "error output: ", out)

	// make a file then commit and push
	err = runCmd(client.Root(), "touch", "README.md")
	require.NoError(t, err)

	out, err = client.CommitAndPush(branch, "docs: README")
	require.NoError(t, err, "error output: %s", out)

	// get current commit hash of the cloned repository
	expectedCommitHash, err := client.CommitSHA()
	require.NoError(t, err)

	// get origin repository's current commit hash
	gitCurrentCommitHash, err := outputCmd(tempDir, "git", "rev-parse", "HEAD")
	require.NoError(t, err)
	actualCommitHash := strings.TrimSpace(string(gitCurrentCommitHash))
	require.Equal(t, expectedCommitHash, actualCommitHash)
}
