package path

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	fileutil "github.com/argoproj/argo-cd/v3/test/fixture/path"
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
	wd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		os.Remove(path.Join(testDir, "abslink"))
	})
	t.Chdir(testDir)
	require.NoError(t, fileutil.CreateSymlink(t, "/somethingbad", "abslink"))
	t.Chdir(wd)
	err = CheckOutOfBoundsSymlinks(testDir)
	var oobError *OutOfBoundsSymlinkError
	require.ErrorAs(t, err, &oobError)
	assert.Equal(t, "abslink", oobError.File)
}

func getApp(annotation *string, sourcePath *string) *v1alpha1.Application {
	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-app",
		},
	}
	if annotation != nil {
		app.Annotations = make(map[string]string)
		app.Annotations[v1alpha1.AnnotationKeyManifestGeneratePaths] = *annotation
	}

	if sourcePath != nil {
		app.Spec.Source = &v1alpha1.ApplicationSource{
			Path: *sourcePath,
		}
	}

	return app
}

func getSourceHydratorApp(annotation *string, drySourcePath string, syncSourcePath string) *v1alpha1.Application {
	app := getApp(annotation, nil)
	app.Spec.SourceHydrator = &v1alpha1.SourceHydrator{
		DrySource: v1alpha1.DrySource{
			Path: drySourcePath,
		},
		SyncSource: v1alpha1.SyncSource{
			Path: syncSourcePath,
		},
	}

	return app
}

func Test_GetAppRefreshPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		app           *v1alpha1.Application
		source        v1alpha1.ApplicationSource
		expectedPaths []string
	}{
		{
			name:          "single source without annotation",
			app:           getApp(nil, ptr.To("source/path")),
			source:        v1alpha1.ApplicationSource{Path: "source/path"},
			expectedPaths: []string{},
		},
		{
			name:          "single source with annotation",
			app:           getApp(ptr.To(".;dev/deploy;other/path"), ptr.To("source/path")),
			source:        v1alpha1.ApplicationSource{Path: "source/path"},
			expectedPaths: []string{"source/path", "source/path/dev/deploy", "source/path/other/path"},
		},
		{
			name:          "single source with empty annotation",
			app:           getApp(ptr.To(".;;"), ptr.To("source/path")),
			source:        v1alpha1.ApplicationSource{Path: "source/path"},
			expectedPaths: []string{"source/path"},
		},
		{
			name:          "single source with absolute path annotation",
			app:           getApp(ptr.To("/fullpath/deploy;other/path"), ptr.To("source/path")),
			source:        v1alpha1.ApplicationSource{Path: "source/path"},
			expectedPaths: []string{"fullpath/deploy", "source/path/other/path"},
		},
		{
			name:          "source hydrator sync source without annotation",
			app:           getSourceHydratorApp(nil, "dry/path", "sync/path"),
			source:        v1alpha1.ApplicationSource{Path: "sync/path"},
			expectedPaths: []string{"sync/path"},
		},
		{
			name:          "source hydrator dry source without annotation",
			app:           getSourceHydratorApp(nil, "dry/path", "sync/path"),
			source:        v1alpha1.ApplicationSource{Path: "dry/path"},
			expectedPaths: []string{},
		},
		{
			name:          "source hydrator sync source with annotation",
			app:           getSourceHydratorApp(ptr.To("deploy"), "dry/path", "sync/path"),
			source:        v1alpha1.ApplicationSource{Path: "sync/path"},
			expectedPaths: []string{"sync/path"},
		},
		{
			name:          "source hydrator dry source with annotation",
			app:           getSourceHydratorApp(ptr.To("deploy"), "dry/path", "sync/path"),
			source:        v1alpha1.ApplicationSource{Path: "dry/path"},
			expectedPaths: []string{"dry/path/deploy"},
		},
		{
			name:   "annotation paths with spaces after semicolon",
			app:    getApp(ptr.To(".; dev/deploy; other/path"), ptr.To("source/path")),
			source: v1alpha1.ApplicationSource{Path: "source/path"},
			expectedPaths: []string{
				"source/path",
				"source/path/dev/deploy",
				"source/path/other/path",
			},
		},
		{
			name:   "annotation paths with spaces before semicolon",
			app:    getApp(ptr.To(". ;dev/deploy ;other/path"), ptr.To("source/path")),
			source: v1alpha1.ApplicationSource{Path: "source/path"},
			expectedPaths: []string{
				"source/path",
				"source/path/dev/deploy",
				"source/path/other/path",
			},
		},
		{
			name:   "annotation paths with spaces around absolute path",
			app:    getApp(ptr.To(" /fullpath/deploy ; other/path "), ptr.To("source/path")),
			source: v1alpha1.ApplicationSource{Path: "source/path"},
			expectedPaths: []string{
				"fullpath/deploy",
				"source/path/other/path",
			},
		},
		{
			name:   "annotation paths only spaces and separators",
			app:    getApp(ptr.To(" ; ; . ; "), ptr.To("source/path")),
			source: v1alpha1.ApplicationSource{Path: "source/path"},
			expectedPaths: []string{
				"source/path",
			},
		},
	}

	for _, tt := range tests {
		ttc := tt
		t.Run(ttc.name, func(t *testing.T) {
			t.Parallel()
			assert.ElementsMatch(t, ttc.expectedPaths, GetSourceRefreshPaths(ttc.app, ttc.source), "GetAppRefreshPath()")
		})
	}
}
