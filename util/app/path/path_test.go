package path

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	fileutil "github.com/argoproj/argo-cd/v2/test/fixture/path"
)

func TestPathRoot(t *testing.T) {
	_, err := Path("./testdata", "/")
	require.EqualError(t, err, "/: app path is absolute")
}

func TestPathAbsolute(t *testing.T) {
	_, err := Path("./testdata", "/etc/passwd")
	require.EqualError(t, err, "/etc/passwd: app path is absolute")
}

func TestPathDotDot(t *testing.T) {
	_, err := Path("./testdata", "..")
	require.EqualError(t, err, "..: app path outside root")
}

func TestPathDotDotSlash(t *testing.T) {
	_, err := Path("./testdata", "../")
	require.EqualError(t, err, "../: app path outside root")
}

func TestPathDot(t *testing.T) {
	_, err := Path("./testdata", ".")
	require.NoError(t, err)
}

func TestPathDotSlash(t *testing.T) {
	_, err := Path("./testdata", "./")
	require.NoError(t, err)
}

func TestNonExistentPath(t *testing.T) {
	_, err := Path("./testdata", "does-not-exist")
	require.EqualError(t, err, "does-not-exist: app path does not exist")
}

func TestPathNotDir(t *testing.T) {
	_, err := Path("./testdata", "file.txt")
	require.EqualError(t, err, "file.txt: app path is not a directory")
}

func TestGoodSymlinks(t *testing.T) {
	err := CheckOutOfBoundsSymlinks("./testdata/goodlink")
	require.NoError(t, err)
}

// Simple check of leaving the repo
func TestBadSymlinks(t *testing.T) {
	err := CheckOutOfBoundsSymlinks("./testdata/badlink")
	var oobError *OutOfBoundsSymlinkError
	require.ErrorAs(t, err, &oobError)
	assert.Equal(t, "badlink", oobError.File)
}

// Crazy formatting check
func TestBadSymlinks2(t *testing.T) {
	err := CheckOutOfBoundsSymlinks("./testdata/badlink2")
	var oobError *OutOfBoundsSymlinkError
	require.ErrorAs(t, err, &oobError)
	assert.Equal(t, "badlink", oobError.File)
}

// Make sure no part of the symlink can leave the repo, even if it ultimately targets inside the repo
func TestBadSymlinks3(t *testing.T) {
	err := CheckOutOfBoundsSymlinks("./testdata/badlink3")
	var oobError *OutOfBoundsSymlinkError
	require.ErrorAs(t, err, &oobError)
	assert.Equal(t, "badlink", oobError.File)
}

// No absolute symlinks allowed
func TestAbsSymlink(t *testing.T) {
	const testDir = "./testdata/abslink"
	require.NoError(t, fileutil.CreateSymlink(t, testDir, "/somethingbad", "abslink"))
	defer os.Remove(path.Join(testDir, "abslink"))
	err := CheckOutOfBoundsSymlinks(testDir)
	var oobError *OutOfBoundsSymlinkError
	require.ErrorAs(t, err, &oobError)
	assert.Equal(t, "abslink", oobError.File)
}

func getApp(annotation string, sourcePath string) *v1alpha1.Application {
	return &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				v1alpha1.AnnotationKeyManifestGeneratePaths: annotation,
			},
		},
		Spec: v1alpha1.ApplicationSpec{
			Source: &v1alpha1.ApplicationSource{
				Path: sourcePath,
			},
		},
	}
}

func getMultiSourceApp(annotation string, paths ...string) *v1alpha1.Application {
	var sources v1alpha1.ApplicationSources
	for _, path := range paths {
		sources = append(sources, v1alpha1.ApplicationSource{Path: path})
	}
	return &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				v1alpha1.AnnotationKeyManifestGeneratePaths: annotation,
			},
		},
		Spec: v1alpha1.ApplicationSpec{
			Sources: sources,
		},
	}
}

func Test_AppFilesHaveChanged(t *testing.T) {
	tests := []struct {
		name           string
		app            *v1alpha1.Application
		files          []string
		changeExpected bool
	}{
		{"default no path", &v1alpha1.Application{}, []string{"README.md"}, true},
		{"no files changed", getApp(".", "source/path"), []string{}, true},
		{"relative path - matching", getApp(".", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"relative path, multi source - matching #1", getMultiSourceApp(".", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"relative path, multi source - matching #2", getMultiSourceApp(".", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"relative path - not matching", getApp(".", "source/path"), []string{"README.md"}, false},
		{"relative path, multi source - not matching", getMultiSourceApp(".", "other/path", "unrelated/path"), []string{"README.md"}, false},
		{"absolute path - matching", getApp("/source/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"absolute path, multi source - matching #1", getMultiSourceApp("/source/path", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"absolute path, multi source - matching #2", getMultiSourceApp("/source/path", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"absolute path - not matching", getApp("/source/path1", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"absolute path, multi source - not matching", getMultiSourceApp("/source/path1", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"glob path * - matching", getApp("/source/**/my-deployment.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"glob path * - not matching", getApp("/source/**/my-service.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"glob path ? - matching", getApp("/source/path/my-deployment-?.yaml", "source/path"), []string{"source/path/my-deployment-0.yaml"}, true},
		{"glob path ? - not matching", getApp("/source/path/my-deployment-?.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"glob path char range - matching", getApp("/source/path[0-9]/my-deployment.yaml", "source/path"), []string{"source/path1/my-deployment.yaml"}, true},
		{"glob path char range - not matching", getApp("/source/path[0-9]/my-deployment.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"mixed glob path - matching", getApp("/source/path[0-9]/my-*.yaml", "source/path"), []string{"source/path1/my-deployment.yaml"}, true},
		{"mixed glob path - not matching", getApp("/source/path[0-9]/my-*.yaml", "source/path"), []string{"README.md"}, false},
		{"two relative paths - matching", getApp(".;../shared", "my-app"), []string{"shared/my-deployment.yaml"}, true},
		{"two relative paths, multi source - matching #1", getMultiSourceApp(".;../shared", "my-app", "other/path"), []string{"shared/my-deployment.yaml"}, true},
		{"two relative paths, multi source - matching #2", getMultiSourceApp(".;../shared", "my-app", "other/path"), []string{"shared/my-deployment.yaml"}, true},
		{"two relative paths - not matching", getApp(".;../shared", "my-app"), []string{"README.md"}, false},
		{"two relative paths, multi source - not matching", getMultiSourceApp(".;../shared", "my-app", "other/path"), []string{"README.md"}, false},
		{"file relative path - matching", getApp("./my-deployment.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file relative path, multi source - matching #1", getMultiSourceApp("./my-deployment.yaml", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file relative path, multi source - matching #2", getMultiSourceApp("./my-deployment.yaml", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file relative path - not matching", getApp("./my-deployment.yaml", "source/path"), []string{"README.md"}, false},
		{"file relative path, multi source - not matching", getMultiSourceApp("./my-deployment.yaml", "source/path", "other/path"), []string{"README.md"}, false},
		{"file absolute path - matching", getApp("/source/path/my-deployment.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file absolute path, multi source - matching #1", getMultiSourceApp("/source/path/my-deployment.yaml", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file absolute path, multi source - matching #2", getMultiSourceApp("/source/path/my-deployment.yaml", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file absolute path - not matching", getApp("/source/path1/README.md", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"file absolute path, multi source - not matching", getMultiSourceApp("/source/path1/README.md", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"file two relative paths - matching", getApp("./README.md;../shared/my-deployment.yaml", "my-app"), []string{"shared/my-deployment.yaml"}, true},
		{"file two relative paths, multi source - matching", getMultiSourceApp("./README.md;../shared/my-deployment.yaml", "my-app", "other-path"), []string{"shared/my-deployment.yaml"}, true},
		{"file two relative paths - not matching", getApp(".README.md;../shared/my-deployment.yaml", "my-app"), []string{"kustomization.yaml"}, false},
		{"file two relative paths, multi source - not matching", getMultiSourceApp(".README.md;../shared/my-deployment.yaml", "my-app", "other-path"), []string{"kustomization.yaml"}, false},
		{"changed file absolute path - matching", getApp(".", "source/path"), []string{"/source/path/my-deployment.yaml"}, true},
	}
	for _, tt := range tests {
		ttc := tt
		t.Run(ttc.name, func(t *testing.T) {
			t.Parallel()
			refreshPaths := GetAppRefreshPaths(ttc.app)
			if got := AppFilesHaveChanged(refreshPaths, ttc.files); got != ttc.changeExpected {
				t.Errorf("AppFilesHaveChanged() = %v, want %v", got, ttc.changeExpected)
			}
		})
	}
}

func Test_GetAppRefreshPaths(t *testing.T) {
	tests := []struct {
		name          string
		app           *v1alpha1.Application
		expectedPaths []string
	}{
		{"default no path", &v1alpha1.Application{}, []string{}},
		{"relative path", getApp(".", "source/path"), []string{"source/path"}},
		{"absolute path - multi source", getMultiSourceApp("/source/path", "source/path", "other/path"), []string{"source/path"}},
		{"two relative paths ", getApp(".;../shared", "my-app"), []string{"my-app", "shared"}},
		{"file relative path", getApp("./my-deployment.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}},
		{"file absolute path", getApp("/source/path/my-deployment.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}},
		{"file two relative paths", getApp("./README.md;../shared/my-deployment.yaml", "my-app"), []string{"my-app/README.md", "shared/my-deployment.yaml"}},
		{"glob path", getApp("/source/*/my-deployment.yaml", "source/path"), []string{"source/*/my-deployment.yaml"}},
		{"empty path", getApp(".;", "source/path"), []string{"source/path"}},
	}
	for _, tt := range tests {
		ttc := tt
		t.Run(ttc.name, func(t *testing.T) {
			t.Parallel()
			if got := GetAppRefreshPaths(ttc.app); !assert.ElementsMatch(t, ttc.expectedPaths, got) {
				t.Errorf("GetAppRefreshPath() = %v, want %v", got, ttc.expectedPaths)
			}
		})
	}
}
