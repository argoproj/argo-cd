package git

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runCmd(workingDir string, name string, args ...string) error {
	return runCmdEx(workingDir, os.Stdout, name, args...)
}

func runCmdEx(workingDir string, stdoutWriter io.Writer, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = workingDir
	cmd.Stdout = stdoutWriter
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Test_nativeGitClient_Fetch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)

	err = runCmd(tempDir, "git", "init")
	require.NoError(t, err)

	err = runCmd(tempDir, "git", "commit", "-m", "Initial commit", "--allow-empty")
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

	err = runCmd(tempDir, "git", "init")
	require.NoError(t, err)

	err = runCmd(tempDir, "git", "commit", "-m", "Initial commit", "--allow-empty")
	require.NoError(t, err)

	client, err := NewClient(fmt.Sprintf("file://%s", tempDir), NopCreds{}, true, false, "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	err = runCmd(tempDir, "git", "branch", "test/foo")
	require.NoError(t, err)

	err = client.Fetch("")
	assert.NoError(t, err)

	err = runCmd(tempDir, "git", "branch", "-d", "test/foo")
	require.NoError(t, err)
	err = runCmd(tempDir, "git", "branch", "test/foo/bar")
	require.NoError(t, err)

	err = client.Fetch("")
	assert.NoError(t, err)
}

func Test_IsAnnotatedTag(t *testing.T) {
	tempDir := t.TempDir()
	client, err := NewClient(fmt.Sprintf("file://%s", tempDir), NopCreds{}, true, false, "")
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
	assert.NoError(t, err)
	err = runCmd(client.Root(), "git", "commit", "-m", "remove README", "-a")
	assert.NoError(t, err)

	// We moved on, so tag doesn't point to HEAD anymore
	atag = client.IsAnnotatedTag("HEAD")
	assert.False(t, atag)
}

func Test_nativeGitClient_Submodule(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)

	foo := filepath.Join(tempDir, "foo")
	err = os.Mkdir(foo, 0755)
	require.NoError(t, err)

	err = runCmd(foo, "git", "init")
	require.NoError(t, err)

	bar := filepath.Join(tempDir, "bar")
	err = os.Mkdir(bar, 0755)
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
	err = runCmd(client.Root(), "git", "config", "submodule.bar.url")
	assert.Error(t, err)

	// Call Submodule() via Checkout() with submoduleEnabled=true.
	err = client.Checkout(commitSHA, true)
	assert.NoError(t, err)

	// Check if the .gitmodule URL is reflected in .git/config
	cmd := exec.Command("git", "config", "submodule.bar.url")
	cmd.Dir = client.Root()
	result, err := cmd.Output()
	assert.NoError(t, err)
	assert.Equal(t, bar+"\n", string(result))

	// Change URL of submodule bar
	err = runCmd(client.Root(), "git", "config", "--file=.gitmodules", "submodule.bar.url", bar+"baz")
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

func Test_nativeGitClient_PreserveDependenciesChartsArchives(t *testing.T) {
	// Preperations phase:
	// 1. Creating a git repository with two charts: basechart and parentchart
	// 2. basechart is a dependency of parentchart
	// 3. Commiting the charts to the git repository
	// 4. Creating the git client and checkout the repository
	// 5. Enabling the preserve dependencies charts archives feature

	tempDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)

	err = runCmd(tempDir, "git", "init")
	require.NoError(t, err)

	err = runCmd(tempDir, "git", "commit", "-m", "Initial commit", "--allow-empty")
	require.NoError(t, err)

	// Creating basechart
	err = runCmd(tempDir, "helm", "create", "basechart")
	require.NoError(t, err)

	// Creating parentchart
	err = runCmd(tempDir, "helm", "create", "parentchart")
	require.NoError(t, err)

	// Adding basechart as a dependency to parentchart
	err = runCmd(tempDir, "bash", "-c", "echo \"dependencies:\n- name: basechart\n  version: 0.1.0\n  repository: \\\"file://../basechart\\\"\" >> parentchart/Chart.yaml")
	require.NoError(t, err)

	err = runCmd(tempDir, "git", "add", ".")
	require.NoError(t, err)

	err = runCmd(tempDir, "git", "commit", "-m", "Added charts", "--allow-empty")
	require.NoError(t, err)

	client, err := NewClient(fmt.Sprintf("file://%s", tempDir), NopCreds{}, true, false, "")
	require.NoError(t, err)

	// Enable the preserve dependencies charts archives feature
	WithPreserveDependenciesChartsArchives()(client.(*nativeGitClient))

	err = client.Init()
	require.NoError(t, err)

	err = client.Fetch("")
	assert.NoError(t, err)

	commitSHA, err := client.LsRemote("HEAD")
	assert.NoError(t, err)

	err = client.Checkout(commitSHA, false)
	require.NoError(t, err)

	// First phase - pre checkout:
	// 1. Verify, using `helm dependency list`, that the basechart dependency is missing
	// 2. Run `helm dependency build` to build the basechart dependency
	// 3. Verify that the basechart tgz file exists in the parentchart charts directory
	// 4. Verify, using `helm dependency list`, that the basechart dependency is ok now
	// 5. Run checkout that will in turn, run the git clean command

	// Verify that `helm dependency list` specifies the basechart dependency is missing
	stdout := &bytes.Buffer{}
	err = runCmdEx(client.Root(), stdout, "bash", "-c", "cd parentchart && helm dependency list")
	require.NoError(t, err)
	require.Contains(t, stdout.String(), "basechart\t0.1.0  \tfile://../basechart\tmissing")

	// Run `helm dependency build` to build the basechart dependency
	err = runCmd(client.Root(), "bash", "-c", "cd parentchart && helm dependency build . --skip-refresh")
	require.NoError(t, err)

	// Check that the basechart tgz file exists in the parentchart charts directory
	_, err = os.Stat(path.Join(client.Root(), "parentchart/charts/basechart-0.1.0.tgz"))
	require.NoError(t, err)

	// Verify that `helm dependency list` specifies the basechart dependency is ok
	stdout.Reset()
	err = runCmdEx(client.Root(), stdout, "bash", "-c", "cd parentchart && helm dependency list")
	require.NoError(t, err)
	require.Contains(t, stdout.String(), "basechart\t0.1.0  \tfile://../basechart\tok")

	// Run Checkout - it runs the git clean command
	err = client.Checkout(commitSHA, false)
	require.NoError(t, err)

	// Second phase - post checkout:
	// 1. Verify that the basechart tgz file exists in the parentchart charts directory - it wasn't deleted by the git clean command
	// 2. Update the basechart version and the basechart dependency version in the parentchart
	// 3. Verify that `helm dependency list` specifies the basechart dependency is wrong version
	// 4. Run `helm dependency build` to build the new version of basechart dependency and delete the old one
	// 5. Verify that the previous version basechart tgz file does not exist in the parentchart charts directory and the new one does
	// 6. Verify that `helm dependency list` specifies the new version of basechart dependency is ok
	// 7. Verify that helm template is running successfully

	// Check that the basechart tgz file still exists in the parentchart charts directory
	_, err = os.Stat(path.Join(client.Root(), "parentchart/charts/basechart-0.1.0.tgz"))
	require.NoError(t, err)

	// Bump the basechart version using sed
	err = runCmd(client.Root(), "bash", "-c", "sed -i 's/version: 0.1.0/version: 0.2.0/g' basechart/Chart.yaml")
	require.NoError(t, err)

	// Bump the basechart dependency version in the parentchart
	err = runCmd(client.Root(), "bash", "-c", "sed -i 's/  version: 0.1.0/  version: 0.2.0/g' parentchart/Chart.yaml")
	require.NoError(t, err)

	// Verify that `helm dependency list` specifies the new version of basechart dependency is wrong version
	stdout.Reset()
	err = runCmdEx(client.Root(), stdout, "bash", "-c", "cd parentchart && helm dependency list")
	require.NoError(t, err)
	require.Contains(t, stdout.String(), "basechart\t0.2.0  \tfile://../basechart\twrong version")

	// Runn `helm dependency build` to build the new version of basechart dependency and delete the old one
	err = runCmd(client.Root(), "bash", "-c", "cd parentchart && helm dependency build . --skip-refresh")
	require.NoError(t, err)

	// check that the previous version basechart tgz file does not exist in the parentchart charts directory
	_, err = os.Stat(path.Join(client.Root(), "parentchart/charts/basechart-0.1.0.tgz"))
	require.Error(t, err)

	// check that the new version basechart tgz file exists in the parentchart charts directory
	_, err = os.Stat(path.Join(client.Root(), "parentchart/charts/basechart-0.2.0.tgz"))
	require.NoError(t, err)

	// Verify that `helm dependency list` specifies the new version of basechart dependency is ok
	stdout.Reset()
	err = runCmdEx(client.Root(), stdout, "bash", "-c", "cd parentchart && helm dependency list")
	require.NoError(t, err)
	require.Contains(t, stdout.String(), "basechart\t0.2.0  \tfile://../basechart\tok")

	// Verify that helm template is running successfully
	err = runCmd(client.Root(), "bash", "-c", "cd parentchart && helm template .")
	require.NoError(t, err)
}
