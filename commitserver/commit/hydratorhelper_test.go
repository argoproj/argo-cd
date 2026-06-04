package commit

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	appsv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/hydrator"
)

// tempRoot creates a temporary directory and returns an os.Root object for it.
// We use this instead of t.TempDir() because OSX does weird things with temp directories, and it triggers
// the os.Root protections.
func tempRoot(t *testing.T) *os.Root {
	t.Helper()

	dir, err := os.MkdirTemp(".", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		err := os.RemoveAll(dir)
		require.NoError(t, err)
	})
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := root.Close()
		require.NoError(t, err)
	})
	return root
}

// setupGitRepos creates two git repositories: a bare "origin" repo and a "clone" repo.
// The clone is configured to push/pull from the origin using file:// URLs.
// Returns: originPath, clonePath
func setupGitRepos(t *testing.T) (string, string) {
	t.Helper()
	ctx := t.Context()

	// Create origin bare repository
	originPath := t.TempDir()

	cmd := exec.CommandContext(ctx, "git", "init", "--bare", "--initial-branch=main", originPath)
	require.NoError(t, cmd.Run())

	// Create clone repository
	clonePath := t.TempDir()

	cmd = exec.CommandContext(ctx, "git", "init", "--initial-branch=main")
	cmd.Dir = clonePath
	require.NoError(t, cmd.Run())

	// Configure git user for the clone
	cmd = exec.CommandContext(ctx, "git", "config", "user.name", "Test User")
	cmd.Dir = clonePath
	require.NoError(t, cmd.Run())

	cmd = exec.CommandContext(ctx, "git", "config", "user.email", "test@example.com")
	cmd.Dir = clonePath
	require.NoError(t, cmd.Run())

	// Add remote pointing to origin using file:// URL
	originURL := "file://" + originPath
	cmd = exec.CommandContext(ctx, "git", "remote", "add", "origin", originURL)
	cmd.Dir = clonePath
	require.NoError(t, cmd.Run())

	// Create initial commit in clone
	cmd = exec.CommandContext(ctx, "git", "commit", "--allow-empty", "-m", "Initial commit")
	cmd.Dir = clonePath
	require.NoError(t, cmd.Run())

	// Push to origin to establish main branch
	cmd = exec.CommandContext(ctx, "git", "push", "-u", "origin", "main")
	cmd.Dir = clonePath
	require.NoError(t, cmd.Run())

	// Configure git notes push
	cmd = exec.CommandContext(ctx, "git", "config", "--add", "remote.origin.push", "refs/notes/*:refs/notes/*")
	cmd.Dir = clonePath
	require.NoError(t, cmd.Run())

	return originPath, clonePath
}

// createGitClient creates a git.Client pointing to the specified repository.
// The client will operate on the clonePath directory.
func createGitClient(t *testing.T, repoURL, clonePath string) git.Client {
	t.Helper()

	client, err := git.NewClientExt(repoURL, clonePath, git.NopCreds{}, true, false, "", "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	err = client.Fetch("", 0)
	require.NoError(t, err)

	return client
}

// commitFile writes a file to the repository, stages it, and commits it.
// Returns the commit SHA.
func commitFile(t *testing.T, repoPath, filename, content string) string {
	t.Helper()
	ctx := t.Context()

	// Write the file
	fullPath := filepath.Join(repoPath, filename)
	err := os.MkdirAll(filepath.Dir(fullPath), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(fullPath, []byte(content), 0o644)
	require.NoError(t, err)

	// Stage the file
	cmd := exec.CommandContext(ctx, "git", "add", filename)
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	// Commit the file
	cmd = exec.CommandContext(ctx, "git", "commit", "-m", "Add "+filename)
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	// Get commit SHA
	cmd = exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	require.NoError(t, err)

	return strings.TrimSpace(string(output))
}

// verifyGitNote verifies that a git note exists with the expected content.
func verifyGitNote(t *testing.T, repoPath, commitSHA, namespace, expectedContent string) {
	t.Helper()
	ctx := t.Context()

	cmd := exec.CommandContext(ctx, "git", "notes", "--ref="+namespace, "show", commitSHA)
	cmd.Dir = repoPath
	output, err := cmd.Output()
	require.NoError(t, err)

	actualContent := strings.TrimSpace(string(output))
	assert.Equal(t, expectedContent, actualContent)
}

// TestWriteForPaths is an integration test that uses git repositories
// to validate manifest writing and change detection.
func TestWriteForPaths(t *testing.T) {
	originPath, clonePath := setupGitRepos(t)

	repoURL := "file://" + originPath
	gitClient := createGitClient(t, repoURL, clonePath)

	// Open the clone directory as a root for WriteForPaths
	root, err := os.OpenRoot(clonePath)
	require.NoError(t, err)
	defer root.Close()

	drySha := "abc123"
	paths := []*apiclient.PathDetails{
		{
			Path: "path1",
			Manifests: []*apiclient.HydratedManifestDetails{
				{ManifestJSON: `{"kind":"Pod","apiVersion":"v1"}`},
			},
			Commands: []string{"command1", "command2"},
		},
		{
			Path: "path2",
			Manifests: []*apiclient.HydratedManifestDetails{
				{ManifestJSON: `{"kind":"Service","apiVersion":"v1"}`},
			},
			Commands: []string{"command3"},
		},
		{
			Path: "path3/nested",
			Manifests: []*apiclient.HydratedManifestDetails{
				{ManifestJSON: `{"kind":"Deployment","apiVersion":"apps/v1"}`},
			},
			Commands: []string{"command4"},
		},
	}

	now := metav1.NewTime(time.Now())
	metadata := &appsv1.RevisionMetadata{
		Author: "test-author",
		Date:   &now,
		Message: `test-message

Signed-off-by: Test User <test@example.com>
Argocd-reference-commit-sha: abc123
`,
		References: []appsv1.RevisionReference{
			{
				Commit: &appsv1.CommitMetadata{
					Author:  "test-code-author <test-email-author@example.com>",
					Date:    now.Format(time.RFC3339),
					Subject: "test-code-subject",
					SHA:     "test-code-sha",
					RepoURL: "https://example.com/test/repo.git",
				},
			},
		},
	}

	// Call WriteForPaths with real git client - manifests don't exist yet so all should be new
	shouldCommit, err := WriteForPaths(root, repoURL, drySha, metadata, paths, gitClient)
	require.NoError(t, err)
	require.True(t, shouldCommit, "shouldCommit should be true since all manifests are new")

	// Check if the top-level hydrator.metadata exists and contains the repo URL and dry SHA
	topMetadataPath := filepath.Join(clonePath, "hydrator.metadata")
	topMetadataBytes, err := os.ReadFile(topMetadataPath)
	require.NoError(t, err)

	var topMetadata hydratorMetadataFile
	err = json.Unmarshal(topMetadataBytes, &topMetadata)
	require.NoError(t, err)
	assert.Equal(t, repoURL, topMetadata.RepoURL)
	assert.Equal(t, drySha, topMetadata.DrySHA)
	assert.Equal(t, metadata.Author, topMetadata.Author)
	assert.Equal(t, "test-message", topMetadata.Subject)
	// The body should exclude the Argocd- trailers.
	assert.Equal(t, "Signed-off-by: Test User <test@example.com>\n", topMetadata.Body)
	assert.Equal(t, metadata.Date.Format(time.RFC3339), topMetadata.Date)
	assert.Equal(t, metadata.References, topMetadata.References)

	for _, p := range paths {
		fullHydratePath := filepath.Join(clonePath, p.Path)

		// Check if each path directory exists
		assert.DirExists(t, fullHydratePath)

		// Check if each path contains a hydrator.metadata file and contains the repo URL
		metadataPath := path.Join(fullHydratePath, "hydrator.metadata")
		metadataBytes, err := os.ReadFile(metadataPath)
		require.NoError(t, err)

		var readMetadata hydratorMetadataFile
		err = json.Unmarshal(metadataBytes, &readMetadata)
		require.NoError(t, err)
		assert.Equal(t, repoURL, readMetadata.RepoURL)

		// Check if each path contains a README.md file and contains the repo URL
		readmePath := path.Join(fullHydratePath, "README.md")
		readmeBytes, err := os.ReadFile(readmePath)
		require.NoError(t, err)
		assert.Contains(t, string(readmeBytes), repoURL)

		// Check if each path contains a manifest.yaml file and contains the word kind
		manifestPath := path.Join(fullHydratePath, "manifest.yaml")
		manifestBytes, err := os.ReadFile(manifestPath)
		require.NoError(t, err)
		assert.Contains(t, string(manifestBytes), "kind")

		// Verify git detects the file as changed
		changed, err := gitClient.HasFileChanged(filepath.Join(p.Path, "manifest.yaml"))
		require.NoError(t, err)
		assert.True(t, changed, "manifest.yaml should be detected as changed by git")
	}
}

// TestWriteForPaths_WithOneManifestMatchesExisting is an integration test that validates
// WriteForPaths correctly detects which manifests have changed using git operations.
func TestWriteForPaths_WithOneManifestMatchesExisting(t *testing.T) {
	originPath, clonePath := setupGitRepos(t)

	repoURL := "file://" + originPath

	// Pre-populate the repo with existing manifests
	// For path1 and path2, we'll commit DIFFERENT content from what WriteForPaths will write
	// For path3/nested, we'll commit the SAME content that WriteForPaths will write
	commitFile(t, clonePath, "path1/manifest.yaml", "apiVersion: v1\nkind: ConfigMap\n")
	commitFile(t, clonePath, "path2/manifest.yaml", "apiVersion: v1\nkind: Secret\n")
	// This matches what WriteForPaths will write for path3/nested
	commitFile(t, clonePath, "path3/nested/manifest.yaml", "apiVersion: apps/v1\nkind: Deployment\n")

	// Push the commits to origin
	ctx := t.Context()
	cmd := exec.CommandContext(ctx, "git", "push", "origin", "main")
	cmd.Dir = clonePath
	require.NoError(t, cmd.Run())

	gitClient := createGitClient(t, repoURL, clonePath)

	root, err := os.OpenRoot(clonePath)
	require.NoError(t, err)
	defer root.Close()

	drySha := "abc123"
	paths := []*apiclient.PathDetails{
		{
			Path: "path1",
			Manifests: []*apiclient.HydratedManifestDetails{
				{ManifestJSON: `{"kind":"Pod","apiVersion":"v1"}`},
			},
			Commands: []string{"command1", "command2"},
		},
		{
			Path: "path2",
			Manifests: []*apiclient.HydratedManifestDetails{
				{ManifestJSON: `{"kind":"Service","apiVersion":"v1"}`},
			},
			Commands: []string{"command3"},
		},
		{
			Path: "path3/nested",
			Manifests: []*apiclient.HydratedManifestDetails{
				{ManifestJSON: `{"kind":"Deployment","apiVersion":"apps/v1"}`},
			},
			Commands: []string{"command4"},
		},
	}

	now := metav1.NewTime(time.Now())
	metadata := &appsv1.RevisionMetadata{
		Author: "test-author",
		Date:   &now,
		Message: `test-message

Signed-off-by: Test User <test@example.com>
Argocd-reference-commit-sha: abc123
`,
		References: []appsv1.RevisionReference{
			{
				Commit: &appsv1.CommitMetadata{
					Author:  "test-code-author <test-email-author@example.com>",
					Date:    now.Format(time.RFC3339),
					Subject: "test-code-subject",
					SHA:     "test-code-sha",
					RepoURL: "https://example.com/test/repo.git",
				},
			},
		},
	}

	// Call WriteForPaths - should detect path1 and path2 changed, path3/nested unchanged
	shouldCommit, err := WriteForPaths(root, repoURL, drySha, metadata, paths, gitClient)
	require.NoError(t, err)
	require.True(t, shouldCommit, "shouldCommit should be true since path1 and path2 changed")

	// Verify that path1 and path2 changed, path3/nested did not
	changed1, err := gitClient.HasFileChanged("path1/manifest.yaml")
	require.NoError(t, err)
	assert.True(t, changed1, "path1/manifest.yaml should be detected as changed")

	changed2, err := gitClient.HasFileChanged("path2/manifest.yaml")
	require.NoError(t, err)
	assert.True(t, changed2, "path2/manifest.yaml should be detected as changed")

	changed3, err := gitClient.HasFileChanged("path3/nested/manifest.yaml")
	require.NoError(t, err)
	assert.False(t, changed3, "path3/nested/manifest.yaml should NOT be detected as changed")

	// Check if the top-level hydrator.metadata exists and contains the repo URL and dry SHA
	topMetadataPath := filepath.Join(clonePath, "hydrator.metadata")
	topMetadataBytes, err := os.ReadFile(topMetadataPath)
	require.NoError(t, err)

	var topMetadata hydratorMetadataFile
	err = json.Unmarshal(topMetadataBytes, &topMetadata)
	require.NoError(t, err)
	assert.Equal(t, repoURL, topMetadata.RepoURL)
	assert.Equal(t, drySha, topMetadata.DrySHA)
	assert.Equal(t, metadata.Author, topMetadata.Author)
	assert.Equal(t, "test-message", topMetadata.Subject)
	// The body should exclude the Argocd- trailers.
	assert.Equal(t, "Signed-off-by: Test User <test@example.com>\n", topMetadata.Body)
	assert.Equal(t, metadata.Date.Format(time.RFC3339), topMetadata.Date)
	assert.Equal(t, metadata.References, topMetadata.References)

	for _, p := range paths {
		fullHydratePath := filepath.Join(clonePath, p.Path)
		if p.Path == "path3/nested" {
			assert.DirExists(t, fullHydratePath)
			manifestPath := path.Join(fullHydratePath, "manifest.yaml")
			_, err := os.ReadFile(manifestPath)
			require.NoError(t, err)
			// For path3/nested, metadata and README should NOT have been written since manifest didn't change
			metadataPath := path.Join(fullHydratePath, "hydrator.metadata")
			_, err = os.ReadFile(metadataPath)
			require.Error(t, err, "hydrator.metadata should not exist for unchanged path3/nested")
			continue
		}
		// Check if each path directory exists
		assert.DirExists(t, fullHydratePath)

		// Check if each path contains a hydrator.metadata file and contains the repo URL
		metadataPath := path.Join(fullHydratePath, "hydrator.metadata")
		metadataBytes, err := os.ReadFile(metadataPath)
		require.NoError(t, err)

		var readMetadata hydratorMetadataFile
		err = json.Unmarshal(metadataBytes, &readMetadata)
		require.NoError(t, err)
		assert.Equal(t, repoURL, readMetadata.RepoURL)

		// Check if each path contains a README.md file and contains the repo URL
		readmePath := path.Join(fullHydratePath, "README.md")
		readmeBytes, err := os.ReadFile(readmePath)
		require.NoError(t, err)
		assert.Contains(t, string(readmeBytes), repoURL)

		// Check if each path contains a manifest.yaml file and contains the word kind
		manifestPath := path.Join(fullHydratePath, "manifest.yaml")
		manifestBytes, err := os.ReadFile(manifestPath)
		require.NoError(t, err)
		assert.Contains(t, string(manifestBytes), "kind")
	}
}

func TestWriteMetadata(t *testing.T) {
	root := tempRoot(t)

	metadata := hydrator.HydratorCommitMetadata{
		RepoURL: "https://github.com/example/repo",
		DrySHA:  "abc123",
	}

	err := writeMetadata(root, "", metadata)
	require.NoError(t, err)

	metadataPath := filepath.Join(root.Name(), "hydrator.metadata")
	metadataBytes, err := os.ReadFile(metadataPath)
	require.NoError(t, err)

	var readMetadata hydrator.HydratorCommitMetadata
	err = json.Unmarshal(metadataBytes, &readMetadata)
	require.NoError(t, err)
	assert.Equal(t, metadata, readMetadata)
}

func TestWriteReadme(t *testing.T) {
	root := tempRoot(t)

	randomData := make([]byte, 32)
	_, err := rand.Read(randomData)
	require.NoError(t, err)
	hash := sha256.Sum256(randomData)
	sha := hex.EncodeToString(hash[:])

	metadata := hydrator.HydratorCommitMetadata{
		RepoURL: "https://github.com/example/repo",
		DrySHA:  "abc123",
		References: []appsv1.RevisionReference{
			{
				Commit: &appsv1.CommitMetadata{
					Author:  "test-code-author <test@example.com>",
					Date:    time.Now().Format(time.RFC3339),
					Subject: "test-code-subject",
					SHA:     sha,
					RepoURL: "https://example.com/test/repo.git",
				},
			},
		},
	}

	err = writeReadme(root, "", metadata)
	require.NoError(t, err)

	readmePath := filepath.Join(root.Name(), "README.md")
	readmeBytes, err := os.ReadFile(readmePath)
	require.NoError(t, err)
	assert.Equal(t, `# Manifest Hydration

To hydrate the manifests in this repository, run the following commands:

`+"```shell"+`
git clone https://github.com/example/repo
# cd into the cloned directory
git checkout abc123
`+"```"+fmt.Sprintf(`
## References

* [%s](https://example.com/test/repo.git): test-code-subject (test-code-author <test@example.com>)
`, sha[:7]), string(readmeBytes))
}

func TestWriteManifests(t *testing.T) {
	root := tempRoot(t)

	manifests := []*apiclient.HydratedManifestDetails{
		{ManifestJSON: `{"kind":"Pod","apiVersion":"v1"}`},
	}

	err := writeManifests(root, "", manifests)
	require.NoError(t, err)

	manifestPath := path.Join(root.Name(), "manifest.yaml")
	manifestBytes, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	assert.Contains(t, string(manifestBytes), "kind")
}

func TestWriteGitAttributes(t *testing.T) {
	root := tempRoot(t)

	err := writeGitAttributes(root)
	require.NoError(t, err)

	gitAttributesPath := filepath.Join(root.Name(), ".gitattributes")
	gitAttributesBytes, err := os.ReadFile(gitAttributesPath)
	require.NoError(t, err)
	assert.Contains(t, string(gitAttributesBytes), "README.md linguist-generated=true")
	assert.Contains(t, string(gitAttributesBytes), "hydrator.metadata linguist-generated=true")
}

func TestWriteGitAttributes_MatchesAllDepths(t *testing.T) {
	root := tempRoot(t)

	err := writeGitAttributes(root)
	require.NoError(t, err)

	// The gitattributes pattern needs to match files at all depths:
	// - hydrator.metadata (root level)
	// - path1/hydrator.metadata (one level deep)
	// - path1/nested/deep/hydrator.metadata (multiple levels deep)
	// Same for README.md files
	//
	// The pattern "**/hydrator.metadata" matches at any depth including root
	// The pattern "*/hydrator.metadata" only matches exactly one directory level deep

	// Test actual Git behavior using git check-attr
	// Initialize a git repo
	ctx := t.Context()
	repoPath := root.Name()
	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to init git repo: %s", string(output))

	// Test files at different depths
	testCases := []struct {
		path        string
		shouldMatch bool
		description string
	}{
		{"hydrator.metadata", true, "root level hydrator.metadata"},
		{"README.md", true, "root level README.md"},
		{"path1/hydrator.metadata", true, "one level deep hydrator.metadata"},
		{"path1/README.md", true, "one level deep README.md"},
		{"path1/nested/hydrator.metadata", true, "two levels deep hydrator.metadata"},
		{"path1/nested/README.md", true, "two levels deep README.md"},
		{"path1/nested/deep/hydrator.metadata", true, "three levels deep hydrator.metadata"},
		{"path1/nested/deep/README.md", true, "three levels deep README.md"},
		{"manifest.yaml", false, "manifest.yaml should not match"},
		{"path1/manifest.yaml", false, "nested manifest.yaml should not match"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// Use git check-attr to verify if linguist-generated attribute is set
			cmd := exec.CommandContext(ctx, "git", "check-attr", "linguist-generated", tc.path)
			cmd.Dir = repoPath
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "Failed to run git check-attr: %s", string(output))

			// Output format: <path>: <attribute>: <value>
			// Example: "hydrator.metadata: linguist-generated: true"
			outputStr := strings.TrimSpace(string(output))

			if tc.shouldMatch {
				expectedOutput := tc.path + ": linguist-generated: true"
				assert.Equal(t, expectedOutput, outputStr,
					"File %s should have linguist-generated=true attribute", tc.path)
			} else {
				// Attribute should be unspecified
				expectedOutput := tc.path + ": linguist-generated: unspecified"
				assert.Equal(t, expectedOutput, outputStr,
					"File %s should not have linguist-generated=true attribute", tc.path)
			}
		})
	}
}

// TestIsHydrated is an integration test that validates git note operations
// for tracking hydration state using git repositories.
func TestIsHydrated(t *testing.T) {
	originPath, clonePath := setupGitRepos(t)

	repoURL := "file://" + originPath
	gitClient := createGitClient(t, repoURL, clonePath)
	ctx := t.Context()

	drySha := "abc123"

	t.Run("existing note with matching drySha", func(t *testing.T) {
		// Create a commit with a note containing the matching drySha
		commitSha := commitFile(t, clonePath, "test1.txt", "test content 1")

		// Add a git note with matching drySha
		noteContent := `{"drySha":"abc123"}`
		cmd := exec.CommandContext(ctx, "git", "notes", "--ref="+NoteNamespace, "add", "-m", noteContent, commitSha)
		cmd.Dir = clonePath
		require.NoError(t, cmd.Run())

		// Test IsHydrated - should return true
		isHydrated, err := IsHydrated(gitClient, drySha, commitSha)
		require.NoError(t, err)
		assert.True(t, isHydrated, "should be hydrated when note exists with matching drySha")
	})

	t.Run("no note found", func(t *testing.T) {
		// Create a commit without any note
		commitShaNoNote := commitFile(t, clonePath, "test2.txt", "test content 2")

		// Test IsHydrated - should return false with no error
		isHydrated, err := IsHydrated(gitClient, drySha, commitShaNoNote)
		require.NoError(t, err)
		assert.False(t, isHydrated, "should not be hydrated when no note exists")
	})

	t.Run("existing note with different drySha", func(t *testing.T) {
		// Create a commit with a note containing a different drySha
		commitShaDifferent := commitFile(t, clonePath, "test3.txt", "test content 3")

		// Add a git note with different drySha
		noteContent := `{"drySha":"different-sha"}`
		cmd := exec.CommandContext(ctx, "git", "notes", "--ref="+NoteNamespace, "add", "-m", noteContent, commitShaDifferent)
		cmd.Dir = clonePath
		require.NoError(t, cmd.Run())

		// Test IsHydrated - should return false (drySha doesn't match)
		isHydrated, err := IsHydrated(gitClient, drySha, commitShaDifferent)
		require.NoError(t, err)
		assert.False(t, isHydrated, "should not be hydrated when note exists with different drySha")
	})

	t.Run("malformed JSON in note", func(t *testing.T) {
		// Create a commit with malformed JSON in the note
		commitShaMalformed := commitFile(t, clonePath, "test4.txt", "test content 4")

		// Add a git note with invalid JSON
		cmd := exec.CommandContext(ctx, "git", "notes", "--ref="+NoteNamespace, "add", "-m", "invalid json content", commitShaMalformed)
		cmd.Dir = clonePath
		require.NoError(t, cmd.Run())

		// Test IsHydrated - should return error due to malformed JSON
		isHydrated, err := IsHydrated(gitClient, drySha, commitShaMalformed)
		require.Error(t, err, "should return error when note contains invalid JSON")
		assert.False(t, isHydrated)
		assert.Contains(t, err.Error(), "json unmarshal failed")
	})
}

// TestAddNote is an integration test that validates git note creation and
// push operations using git repositories.
func TestAddNote(t *testing.T) {
	originPath, clonePath := setupGitRepos(t)

	repoURL := "file://" + originPath
	gitClient := createGitClient(t, repoURL, clonePath)

	drySha := "abc123"

	t.Run("successfully add and push note", func(t *testing.T) {
		// Create a commit
		commitSha := commitFile(t, clonePath, "test-note.txt", "test content")

		// Call AddNote to add a note with the drySha
		err := AddNote(gitClient, drySha, commitSha)
		require.NoError(t, err)

		// Verify note was added locally using git command
		expectedNoteJSON := fmt.Sprintf(`{"drySha":%q}`, drySha)
		verifyGitNote(t, clonePath, commitSha, NoteNamespace, expectedNoteJSON)

		// Verify note was pushed to origin
		verifyGitNote(t, originPath, commitSha, NoteNamespace, expectedNoteJSON)

		// Parse and verify the note content
		var note CommitNote
		err = json.Unmarshal([]byte(expectedNoteJSON), &note)
		require.NoError(t, err)
		assert.Equal(t, drySha, note.DrySHA)
	})

	t.Run("error when push fails", func(t *testing.T) {
		// Create a new set of repos for this test to avoid interference
		originPath2, clonePath2 := setupGitRepos(t)

		repoURL2 := "file://" + originPath2
		gitClient2 := createGitClient(t, repoURL2, clonePath2)

		// Create a commit in the new clone
		commitSha := commitFile(t, clonePath2, "test-fail.txt", "fail test content")

		// Make origin directory read-only to cause push to fail
		err := os.Chmod(originPath2, 0o444)
		require.NoError(t, err)
		// Restore permissions in cleanup
		defer func() {
			_ = os.Chmod(originPath2, 0o755)
		}()

		// Call AddNote - should fail because push will fail due to read-only origin
		err = AddNote(gitClient2, drySha, commitSha)
		require.Error(t, err, "AddNote should fail when push fails")
		assert.Contains(t, err.Error(), "failed to add commit note")
	})
}

// TestWriteForPaths_NoOpScenario tests that when manifests don't change between two hydrations,
// shouldCommit returns false. This reproduces the bug where a new DRY commit that doesn't affect
// manifests should not create a new hydrated commit.
func TestWriteForPaths_NoOpScenario(t *testing.T) {
	root := tempRoot(t)

	repoURL := "https://github.com/example/repo"
	drySha1 := "abc123"
	drySha2 := "def456" // Different dry SHA
	paths := []*apiclient.PathDetails{
		{
			Path: "guestbook",
			Manifests: []*apiclient.HydratedManifestDetails{
				{ManifestJSON: `{"apiVersion":"v1","kind":"Service","metadata":{"name":"guestbook-ui"}}`},
				{ManifestJSON: `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"guestbook-ui"}}`},
			},
			Commands: []string{"kustomize build ."},
		},
	}

	now1 := metav1.NewTime(time.Now())
	metadata1 := &appsv1.RevisionMetadata{
		Author:  "test-author",
		Date:    &now1,
		Message: "Initial commit",
	}

	// First hydration - manifests are new, so HasFileChanged should return true
	mockGitClient1 := gitmocks.NewClient(t)
	mockGitClient1.On("HasFileChanged", "guestbook/manifest.yaml").Return(true, nil).Once()

	shouldCommit1, err := WriteForPaths(root, repoURL, drySha1, metadata1, paths, mockGitClient1)
	require.NoError(t, err)
	require.True(t, shouldCommit1, "First hydration should commit because manifests are new")

	// Second hydration - same manifest content but different dry SHA and metadata
	// Simulate adding a README.md to the dry source (which doesn't affect manifests)
	now2 := metav1.NewTime(time.Now().Add(1 * time.Hour)) // Different timestamp
	metadata2 := &appsv1.RevisionMetadata{
		Author:  "test-author",
		Date:    &now2,
		Message: "Add README.md", // Different commit message
	}

	// The manifests are identical, so HasFileChanged should return false
	mockGitClient2 := gitmocks.NewClient(t)
	mockGitClient2.On("HasFileChanged", "guestbook/manifest.yaml").Return(false, nil).Once()

	shouldCommit2, err := WriteForPaths(root, repoURL, drySha2, metadata2, paths, mockGitClient2)
	require.NoError(t, err)
	require.False(t, shouldCommit2, "Second hydration should NOT commit because manifests didn't change")

	// Verify that the root-level metadata WAS updated (even though we're not committing)
	// The files get written to the working directory, but since shouldCommit is false, they won't be committed
	topMetadataPath := filepath.Join(root.Name(), "hydrator.metadata")
	topMetadataBytes, err := os.ReadFile(topMetadataPath)
	require.NoError(t, err)

	var topMetadata hydratorMetadataFile
	err = json.Unmarshal(topMetadataBytes, &topMetadata)
	require.NoError(t, err)
	// The top-level metadata should have the NEW dry SHA (files are written, just not committed)
	assert.Equal(t, drySha2, topMetadata.DrySHA)
	assert.Equal(t, metadata2.Date.Format(time.RFC3339), topMetadata.Date)
}
