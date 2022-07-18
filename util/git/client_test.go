package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_nativeGitClient_Fetch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)

	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	require.NoError(t, err)

	cmd = exec.Command("git", "commit", "-m", "Initial commit", "--allow-empty")
	cmd.Dir = tempDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	require.NoError(t, err)

	client, err := NewClient(fmt.Sprintf("file://%s", tempDir), NopCreds{}, true, false, "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	err = client.Fetch("")
	assert.NoError(t, err)
}

func Test_nativeGitClient_Fetch_Prune(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)

	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	require.NoError(t, err)

	cmd = exec.Command("git", "commit", "-m", "Initial commit", "--allow-empty")
	cmd.Dir = tempDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	require.NoError(t, err)

	client, err := NewClient(fmt.Sprintf("file://%s", tempDir), NopCreds{}, true, false, "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	// Create branch
	cmd = exec.Command("git", "branch", "test/foo")
	cmd.Dir = tempDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	require.NoError(t, err)

	err = client.Fetch("")
	assert.NoError(t, err)

	// Delete branch
	cmd = exec.Command("git", "branch", "-d", "test/foo")
	cmd.Dir = tempDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	require.NoError(t, err)

	// Create branch that conflicts with previous branch name
	cmd = exec.Command("git", "branch", "test/foo/bar")
	cmd.Dir = tempDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	require.NoError(t, err)

	err = client.Fetch("")
	assert.NoError(t, err)
}

func Test_nativeGitClient_Submodule(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)

	foo := filepath.Join(tempDir, "foo")
	err = os.Mkdir(foo, 0755)
	require.NoError(t, err)

	cmd := exec.Command("git", "init")
	cmd.Dir = foo
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	require.NoError(t, err)

	bar := filepath.Join(tempDir, "bar")
	err = os.Mkdir(bar, 0755)
	require.NoError(t, err)

	cmd = exec.Command("git", "init")
	cmd.Dir = bar
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	require.NoError(t, err)

	cmd = exec.Command("git", "commit", "-m", "Initial commit", "--allow-empty")
	cmd.Dir = bar
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	require.NoError(t, err)

	// Embed repository bar into repository foo
	cmd = exec.Command("git", "submodule", "add", bar)
	cmd.Dir = foo
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	require.NoError(t, err)

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = foo
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	require.NoError(t, err)

	tempDir, err = os.MkdirTemp("", "")
	require.NoError(t, err)

	// Clone foo
	cmd = exec.Command("git", "clone", foo)
	cmd.Dir = tempDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	require.NoError(t, err)

	client, err := NewClient(fmt.Sprintf("file://%s", foo), NopCreds{}, true, false, "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	err = client.Fetch("")
	assert.NoError(t, err)

	commitSHA, err := client.LsRemote("HEAD")
	assert.NoError(t, err)

	// Call Checkout() with submoduleEnabled=false.
	err = client.Checkout(commitSHA, false)
	assert.NoError(t, err)

	// Check if submodule url does not exist in .git/config
	cmd = exec.Command("git", "config", "submodule.bar.url")
	cmd.Dir = client.Root()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	assert.Error(t, err)

	// Call Submodule() via Checkout() with submoduleEnabled=true.
	err = client.Checkout(commitSHA, true)
	assert.NoError(t, err)

	// Check if the .gitmodule URL is reflected in .git/config
	cmd = exec.Command("git", "config", "submodule.bar.url")
	cmd.Dir = client.Root()
	result, err := cmd.Output()
	assert.NoError(t, err)
	assert.Equal(t, bar+"\n", string(result))

	// Change URL of submodule bar
	cmd = exec.Command("git", "config", "--file=.gitmodules", "submodule.bar.url", bar+"baz")
	cmd.Dir = client.Root()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	require.NoError(t, err)

	// Call Submodule()
	err = client.Submodule()
	assert.NoError(t, err)

	// Check if the URL change in .gitmodule is reflected in .git/config
	cmd = exec.Command("git", "config", "submodule.bar.url")
	cmd.Dir = client.Root()
	result, err = cmd.Output()
	assert.NoError(t, err)
	assert.Equal(t, bar+"baz\n", string(result))
}

func TestNewClient_invalidSSHURL(t *testing.T) {
	client, err := NewClient("ssh://bitbucket.org:org/repo", NopCreds{}, false, false, "")
	assert.Nil(t, client)
	assert.ErrorIs(t, err, ErrInvalidRepoURL)
}
