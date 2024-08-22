package git

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"

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
	err = client.Checkout(commitSHA, false)
	require.NoError(t, err)

	// Check if submodule url does not exist in .git/config
	err = runCmd(client.Root(), "git", "config", "submodule.bar.url")
	require.Error(t, err)

	// Call Submodule() via Checkout() with submoduleEnabled=true.
	err = client.Checkout(commitSHA, true)
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
