package path

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_resolveSymlinkRecursive(t *testing.T) {
	// Create temporary directory for test files
	testsDir, err := os.MkdirTemp("", "resolve_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(testsDir) }()

	// Create test files and symlinks
	fooFile := filepath.Join(testsDir, "foo")
	err = os.WriteFile(fooFile, []byte("test"), 0o644)
	require.NoError(t, err)

	barLink := filepath.Join(testsDir, "bar")
	err = os.Symlink("foo", barLink)
	require.NoError(t, err)

	bazLink := filepath.Join(testsDir, "baz")
	err = os.Symlink("bar", bazLink)
	require.NoError(t, err)

	bamLink := filepath.Join(testsDir, "bam")
	err = os.Symlink("baz", bamLink)
	require.NoError(t, err)

	root, err := os.OpenRoot(testsDir)
	require.NoError(t, err)

	t.Run("Resolve non-symlink", func(t *testing.T) {
		r, err := resolveSymbolicLinkRecursive(fooFile, root, 2)
		require.NoError(t, err)
		assert.Equal(t, testsDir+"/foo", r)
	})
	t.Run("Successfully resolve symlink", func(t *testing.T) {
		r, err := resolveSymbolicLinkRecursive(barLink, root, 2)
		require.NoError(t, err)
		assert.Equal(t, testsDir+"/foo", r)
	})
	t.Run("Do not allow symlink at all", func(t *testing.T) {
		r, err := resolveSymbolicLinkRecursive(testsDir+"/bar", root, 0)
		require.Error(t, err)
		assert.Empty(t, r)
	})
	t.Run("Do not allow symlink outside root", func(t *testing.T) {
		// Create a symlink that points outside the root to test restriction
		outsideDir, err := os.MkdirTemp("", "outside_test")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(outsideDir) }()

		outsideFile := filepath.Join(outsideDir, "outside")
		err = os.WriteFile(outsideFile, []byte("outside"), 0o644)
		require.NoError(t, err)

		badLink := filepath.Join(testsDir, "badlink")
		err = os.Symlink(outsideFile, badLink)
		require.NoError(t, err)

		r, err := resolveSymbolicLinkRecursive(badLink, root, 20)
		require.Error(t, err)
		assert.Empty(t, r)
	})
	t.Run("Error because too nested symlink", func(t *testing.T) {
		r, err := resolveSymbolicLinkRecursive(testsDir+"/bam", root, 2)
		require.Error(t, err)
		assert.Empty(t, r)
	})
	t.Run("Error because of circular symlink", func(t *testing.T) {
		// Create a circular symlink
		circularLink := filepath.Join(testsDir, "circular")
		err = os.Symlink("circular", circularLink)
		require.NoError(t, err)

		r, err := resolveSymbolicLinkRecursive(circularLink, root, 2)
		require.Error(t, err)
		assert.Empty(t, r)
	})
	t.Run("No such file or directory", func(t *testing.T) {
		nonExistentPath := filepath.Join(testsDir, "foobar")
		r, err := resolveSymbolicLinkRecursive(nonExistentPath, root, 2)
		require.NoError(t, err)
		assert.Equal(t, testsDir+"/foobar", r)
	})
}

func Test_isURLSchemeAllowed(t *testing.T) {
	type testdata struct {
		name     string
		scheme   string
		allowed  []string
		expected bool
	}
	tts := []testdata{
		{
			name:     "Allowed scheme matches",
			scheme:   "http",
			allowed:  []string{"http", "https"},
			expected: true,
		},
		{
			name:     "Allowed scheme matches only partially",
			scheme:   "http",
			allowed:  []string{"https"},
			expected: false,
		},
		{
			name:     "Scheme is not allowed",
			scheme:   "file",
			allowed:  []string{"http", "https"},
			expected: false,
		},
		{
			name:     "Empty scheme with valid allowances is forbidden",
			scheme:   "",
			allowed:  []string{"http", "https"},
			expected: false,
		},
		{
			name:     "Empty scheme with empty allowances is forbidden",
			scheme:   "",
			allowed:  []string{},
			expected: false,
		},
		{
			name:     "Some scheme with empty allowances is forbidden",
			scheme:   "file",
			allowed:  []string{},
			expected: false,
		},
	}
	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			r := isURLSchemeAllowed(tt.scheme, tt.allowed)
			assert.Equal(t, tt.expected, r)
		})
	}
}

var allowedRemoteProtocols = []string{"http", "https"}

func Test_resolveFilePath(t *testing.T) {
	// Create temporary directory for test files
	testsDir, err := os.MkdirTemp("", "resolve_path_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(testsDir) }()

	// Create subdirectory structure
	barDir := filepath.Join(testsDir, "bar")
	err = os.MkdirAll(barDir, 0o755)
	require.NoError(t, err)

	bazDir := filepath.Join(barDir, "baz")
	err = os.MkdirAll(bazDir, 0o755)
	require.NoError(t, err)

	root, err := os.OpenRoot(testsDir)
	require.NoError(t, err)

	t.Run("Resolve normal relative path into absolute path", func(t *testing.T) {
		p, remote, err := ResolveValueFilePathOrUrl(barDir, root, "baz/bim.yaml", allowedRemoteProtocols)
		require.NoError(t, err)
		assert.False(t, remote)
		assert.Equal(t, filepath.Join(barDir, "baz", "bim.yaml"), string(p))
	})
	t.Run("Resolve normal relative path with .. into absolute path", func(t *testing.T) {
		p, remote, err := ResolveValueFilePathOrUrl(barDir, root, "baz/../../bim.yaml", allowedRemoteProtocols)
		require.NoError(t, err)
		assert.False(t, remote)
		assert.Equal(t, filepath.Join(testsDir, "bim.yaml"), string(p))
	})
	t.Run("Error on path resolving outside repository root", func(t *testing.T) {
		p, remote, err := ResolveValueFilePathOrUrl(barDir, root, "baz/../../../bim.yaml", allowedRemoteProtocols)
		require.ErrorContains(t, err, "outside repository root")
		assert.False(t, remote)
		assert.Empty(t, string(p))
	})
	t.Run("Return verbatim URL", func(t *testing.T) {
		url := "https://some.where/foo,yaml"
		p, remote, err := ResolveValueFilePathOrUrl(barDir, root, url, allowedRemoteProtocols)
		require.NoError(t, err)
		assert.True(t, remote)
		assert.Equal(t, url, string(p))
	})
	t.Run("URL scheme not allowed", func(t *testing.T) {
		url := "file:///some.where/foo,yaml"
		p, remote, err := ResolveValueFilePathOrUrl(barDir, root, url, allowedRemoteProtocols)
		require.Error(t, err)
		assert.False(t, remote)
		assert.Empty(t, string(p))
	})
	t.Run("Implicit URL by absolute path", func(t *testing.T) {
		p, remote, err := ResolveValueFilePathOrUrl(barDir, root, "/baz.yaml", allowedRemoteProtocols)
		require.NoError(t, err)
		assert.False(t, remote)
		assert.Equal(t, filepath.Join(testsDir, "baz.yaml"), string(p))
	})
	t.Run("Relative app path", func(t *testing.T) {
		p, remote, err := ResolveValueFilePathOrUrl(".", root, "/baz.yaml", allowedRemoteProtocols)
		require.NoError(t, err)
		assert.False(t, remote)
		assert.Equal(t, filepath.Join(testsDir, "baz.yaml"), string(p))
	})
	t.Run("Relative repo path", func(t *testing.T) {
		// Create a test directory for this specific test
		testDir, err := os.MkdirTemp("", "relative_repo_test")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(testDir) }()

		root, err := os.OpenRoot(testDir)
		require.NoError(t, err)
		p, remote, err := ResolveValueFilePathOrUrl(testDir, root, "baz.yaml", allowedRemoteProtocols)
		require.NoError(t, err)
		assert.False(t, remote)
		assert.Equal(t, filepath.Join(testDir, "baz.yaml"), string(p))
	})
	t.Run("Overlapping root prefix without trailing slash", func(t *testing.T) {
		// Create temporary directory with "foo" name and another with "foo2" name
		parentDir, err := os.MkdirTemp("", "overlap_test")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(parentDir) }()

		fooDir := filepath.Join(parentDir, "foo")
		err = os.MkdirAll(fooDir, 0o755)
		require.NoError(t, err)

		foo2Dir := filepath.Join(parentDir, "foo2")
		err = os.MkdirAll(foo2Dir, 0o755)
		require.NoError(t, err)

		root, err := os.OpenRoot(fooDir)
		require.NoError(t, err)

		p, remote, err := ResolveValueFilePathOrUrl(".", root, "../foo2/baz.yaml", allowedRemoteProtocols)
		require.ErrorContains(t, err, "outside repository root")
		assert.False(t, remote)
		assert.Empty(t, string(p))
	})
	t.Run("Overlapping root prefix with trailing slash", func(t *testing.T) {
		// Create temporary directory with "foo" name and another with "foo2" name
		parentDir, err := os.MkdirTemp("", "overlap_test2")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(parentDir) }()

		fooDir := filepath.Join(parentDir, "foo")
		err = os.MkdirAll(fooDir, 0o755)
		require.NoError(t, err)

		foo2Dir := filepath.Join(parentDir, "foo2")
		err = os.MkdirAll(foo2Dir, 0o755)
		require.NoError(t, err)

		root, err := os.OpenRoot(fooDir)
		require.NoError(t, err)

		p, remote, err := ResolveValueFilePathOrUrl(".", root, "../foo2/baz.yaml", allowedRemoteProtocols)
		require.ErrorContains(t, err, "outside repository root")
		assert.False(t, remote)
		assert.Empty(t, string(p))
	})
	t.Run("Garbage input as values file", func(t *testing.T) {
		p, remote, err := ResolveValueFilePathOrUrl(".", root, "kfdj\\ks&&&321209.,---e32908923%$§!\"", allowedRemoteProtocols)
		require.ErrorContains(t, err, "outside repository root")
		assert.False(t, remote)
		assert.Empty(t, string(p))
	})
	t.Run("NUL-byte path input as values file", func(t *testing.T) {
		p, remote, err := ResolveValueFilePathOrUrl(".", root, "\000", allowedRemoteProtocols)
		require.ErrorContains(t, err, "outside repository root")
		assert.False(t, remote)
		assert.Empty(t, string(p))
	})
	t.Run("Resolve root path into absolute path - jsonnet library path", func(t *testing.T) {
		p, err := ResolveFileOrDirectoryPath(testsDir, root, "./")
		require.NoError(t, err)
		assert.Equal(t, testsDir, string(p))
	})
}
