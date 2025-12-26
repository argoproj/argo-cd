package path

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

func getSourceHydratorApp(annotation string, drySourcePath string, syncSourcePath string) *v1alpha1.Application {
	return &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				v1alpha1.AnnotationKeyManifestGeneratePaths: annotation,
			},
		},
		Spec: v1alpha1.ApplicationSpec{
			SourceHydrator: &v1alpha1.SourceHydrator{
				DrySource: v1alpha1.DrySource{
					RepoURL: "https://github.com/example/repo",
					Path:    drySourcePath,
				},
				SyncSource: v1alpha1.SyncSource{
					Path: syncSourcePath,
				},
			},
		},
	}
}

func Test_AppFilesHaveChanged(t *testing.T) {
	t.Parallel()

	// Create syncSource apps once to ensure source comparison works
	syncSourceApp1 := getSourceHydratorApp(".", "source/envs/dev", "ksapps")
	syncSourceApp2 := getSourceHydratorApp(".", "source/envs/dev", "helm-charts")
	syncSourceApp3 := getSourceHydratorApp("unrelated/annotation", "source/envs/dev", ".")

	tests := []struct {
		name           string
		app            *v1alpha1.Application
		files          []string
		changeExpected bool
		source         v1alpha1.ApplicationSource
	}{
		{"default no path", &v1alpha1.Application{}, []string{"README.md"}, true, v1alpha1.ApplicationSource{}},
		{"no files changed", getApp(".", "source/path"), []string{}, true, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"relative path - matching", getApp(".", "source/path"), []string{"source/path/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"relative path, multi source - matching #1", getMultiSourceApp(".", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"relative path, multi source - matching #2", getMultiSourceApp(".", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"relative path - not matching", getApp(".", "source/path"), []string{"README.md"}, false, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"relative path, multi source - not matching", getMultiSourceApp(".", "other/path", "unrelated/path"), []string{"README.md"}, false, v1alpha1.ApplicationSource{Path: "other/path"}},
		{"absolute path - matching", getApp("/source/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"absolute path, multi source - matching #1", getMultiSourceApp("/source/path", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"absolute path, multi source - matching #2", getMultiSourceApp("/source/path", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"absolute path - not matching", getApp("/source/path1", "source/path"), []string{"source/path/my-deployment.yaml"}, false, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"absolute path, multi source - not matching", getMultiSourceApp("/source/path1", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, false, v1alpha1.ApplicationSource{Path: "other/path"}},
		{"glob path * - matching", getApp("/source/**/my-deployment.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"glob path * - not matching", getApp("/source/**/my-service.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, false, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"glob path ? - matching", getApp("/source/path/my-deployment-?.yaml", "source/path"), []string{"source/path/my-deployment-0.yaml"}, true, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"glob path ? - not matching", getApp("/source/path/my-deployment-?.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, false, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"glob path char range - matching", getApp("/source/path[0-9]/my-deployment.yaml", "source/path"), []string{"source/path1/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"glob path char range - not matching", getApp("/source/path[0-9]/my-deployment.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, false, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"mixed glob path - matching", getApp("/source/path[0-9]/my-*.yaml", "source/path"), []string{"source/path1/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"mixed glob path - not matching", getApp("/source/path[0-9]/my-*.yaml", "source/path"), []string{"README.md"}, false, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"two relative paths - matching", getApp(".;../shared", "my-app"), []string{"shared/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "my-app"}},
		{"two relative paths, multi source - matching #1", getMultiSourceApp(".;../shared", "my-app", "other/path"), []string{"shared/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "my-app"}},
		{"two relative paths, multi source - matching #2", getMultiSourceApp(".;../shared", "my-app", "other/path"), []string{"shared/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "my-app"}},
		{"two relative paths - not matching", getApp(".;../shared", "my-app"), []string{"README.md"}, false, v1alpha1.ApplicationSource{Path: "my-app"}},
		{"two relative paths, multi source - not matching", getMultiSourceApp(".;../shared", "my-app", "other/path"), []string{"README.md"}, false, v1alpha1.ApplicationSource{Path: "my-app"}},
		{"file relative path - matching", getApp("./my-deployment.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"file relative path, multi source - matching #1", getMultiSourceApp("./my-deployment.yaml", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"file relative path, multi source - matching #2", getMultiSourceApp("./my-deployment.yaml", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"file relative path - not matching", getApp("./my-deployment.yaml", "source/path"), []string{"README.md"}, false, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"file relative path, multi source - not matching", getMultiSourceApp("./my-deployment.yaml", "source/path", "other/path"), []string{"README.md"}, false, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"file absolute path - matching", getApp("/source/path/my-deployment.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"file absolute path, multi source - matching #1", getMultiSourceApp("/source/path/my-deployment.yaml", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"file absolute path, multi source - matching #2", getMultiSourceApp("/source/path/my-deployment.yaml", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"file absolute path - not matching", getApp("/source/path1/README.md", "source/path"), []string{"source/path/my-deployment.yaml"}, false, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"file absolute path, multi source - not matching", getMultiSourceApp("/source/path1/README.md", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, false, v1alpha1.ApplicationSource{Path: "source/path"}},
		{"file two relative paths - matching", getApp("./README.md;../shared/my-deployment.yaml", "my-app"), []string{"shared/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "my-app"}},
		{"file two relative paths, multi source - matching", getMultiSourceApp("./README.md;../shared/my-deployment.yaml", "my-app", "other-path"), []string{"shared/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "my-app"}},
		{"file two relative paths - not matching", getApp(".README.md;../shared/my-deployment.yaml", "my-app"), []string{"kustomization.yaml"}, false, v1alpha1.ApplicationSource{Path: "my-app"}},
		{"file two relative paths, multi source - not matching", getMultiSourceApp(".README.md;../shared/my-deployment.yaml", "my-app", "other-path"), []string{"kustomization.yaml"}, false, v1alpha1.ApplicationSource{Path: "my-app"}},
		{"changed file absolute path - matching", getApp(".", "source/path"), []string{"/source/path/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "source/path"}},
		// SourceHydrator tests - paths should be resolved relative to DRY source
		{"sourceHydrator relative path - matching", getSourceHydratorApp(".", "source/envs/dev", "sync/path"), []string{"source/envs/dev/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{Path: "source/envs/dev"}},
		{"sourceHydrator relative path - not matching sync path", getSourceHydratorApp(".", "source/envs/dev", "sync/path"), []string{"sync/path/my-deployment.yaml"}, false, v1alpha1.ApplicationSource{Path: "source/envs/dev"}},
		{"sourceHydrator ../base - matching dry base", getSourceHydratorApp(".;../base", "source/envs/dev", "sync/path"), []string{"source/envs/base/kustomization.yaml"}, true, v1alpha1.ApplicationSource{Path: "source/envs/dev"}},
		{"sourceHydrator ../base - not matching sync base", getSourceHydratorApp(".;../base", "source/envs/dev", "sync/path"), []string{"sync/base/kustomization.yaml"}, false, v1alpha1.ApplicationSource{Path: "source/envs/dev"}},
		// SyncSource tests - should use syncSource path only, ignoring annotation
		{"syncSource matching files", syncSourceApp1, []string{"ksapps/test-app/app.yaml"}, true, syncSourceApp1.Spec.SourceHydrator.GetSyncSource()},
		{"syncSource not matching files", syncSourceApp2, []string{"ksapps/test-app/app.yaml"}, false, syncSourceApp2.Spec.SourceHydrator.GetSyncSource()},
		{"syncSource root path matches all", syncSourceApp3, []string{"ksapps/test-app/app.yaml"}, true, syncSourceApp3.Spec.SourceHydrator.GetSyncSource()},
		// DrySource without annotation - should use source path like syncSource does
		{"drySource without annotation matching files", getSourceHydratorApp("", "source/envs/dev", "ksapps"), []string{"source/envs/dev/my-deployment.yaml"}, true, v1alpha1.ApplicationSource{RepoURL: "https://github.com/example/repo", Path: "source/envs/dev"}},
		{"drySource without annotation not matching files", getSourceHydratorApp("", "source/envs/dev", "ksapps"), []string{"other/path/my-deployment.yaml"}, false, v1alpha1.ApplicationSource{RepoURL: "https://github.com/example/repo", Path: "source/envs/dev"}},
	}
	for _, tt := range tests {
		ttc := tt
		t.Run(ttc.name, func(t *testing.T) {
			t.Parallel()
			refreshPaths := GetAppRefreshPaths(ttc.app, ttc.source)
			assert.Equal(t, ttc.changeExpected, AppFilesHaveChanged(refreshPaths, ttc.files), "AppFilesHaveChanged()")
		})
	}
}

func Test_GetAppRefreshPaths(t *testing.T) {
	t.Parallel()

	// Create syncSource apps once to ensure source comparison works
	syncSourceApp1 := getSourceHydratorApp(".", "source/envs/dev", "ksapps")
	syncSourceApp2 := getSourceHydratorApp("some/annotation", "source/envs/dev", "helm-charts")

	tests := []struct {
		name          string
		app           *v1alpha1.Application
		source        v1alpha1.ApplicationSource
		expectedPaths []string
	}{
		{"default no path", &v1alpha1.Application{}, v1alpha1.ApplicationSource{}, []string{}},
		{"relative path", getApp(".", "source/path"), v1alpha1.ApplicationSource{Path: "source/path"}, []string{"source/path"}},
		{"absolute path - multi source", getMultiSourceApp("/source/path", "source/path", "other/path"), v1alpha1.ApplicationSource{Path: "source/path"}, []string{"source/path"}},
		{"relative path multi-source iterates all sources", getMultiSourceApp("subdir", "app1", "app2"), v1alpha1.ApplicationSource{}, []string{"app1/subdir", "app2/subdir"}},
		{"two relative paths ", getApp(".;../shared", "my-app"), v1alpha1.ApplicationSource{Path: "my-app"}, []string{"my-app", "shared"}},
		{"file relative path", getApp("./my-deployment.yaml", "source/path"), v1alpha1.ApplicationSource{Path: "source/path"}, []string{"source/path/my-deployment.yaml"}},
		{"file absolute path", getApp("/source/path/my-deployment.yaml", "source/path"), v1alpha1.ApplicationSource{Path: "source/path"}, []string{"source/path/my-deployment.yaml"}},
		{"file two relative paths", getApp("./README.md;../shared/my-deployment.yaml", "my-app"), v1alpha1.ApplicationSource{Path: "my-app"}, []string{"my-app/README.md", "shared/my-deployment.yaml"}},
		{"glob path", getApp("/source/*/my-deployment.yaml", "source/path"), v1alpha1.ApplicationSource{Path: "source/path"}, []string{"source/*/my-deployment.yaml"}},
		{"empty path", getApp(".;", "source/path"), v1alpha1.ApplicationSource{Path: "source/path"}, []string{"source/path"}},
		// SourceHydrator tests - paths should be resolved relative to DRY source
		{"sourceHydrator relative path", getSourceHydratorApp(".", "argocd-kustomize/envs/dev/usw2", "argocd/dev"), v1alpha1.ApplicationSource{Path: "argocd-kustomize/envs/dev/usw2"}, []string{"argocd-kustomize/envs/dev/usw2"}},
		{"sourceHydrator ../base", getSourceHydratorApp(".;../base", "argocd-kustomize/envs/dev/usw2", "argocd/dev"), v1alpha1.ApplicationSource{Path: "argocd-kustomize/envs/dev/usw2"}, []string{"argocd-kustomize/envs/dev/usw2", "argocd-kustomize/envs/dev/base"}},
		{"sourceHydrator absolute path", getSourceHydratorApp("/argocd-kustomize/base", "argocd-kustomize/envs/dev/usw2", "argocd/dev"), v1alpha1.ApplicationSource{Path: "argocd-kustomize/envs/dev/usw2"}, []string{"argocd-kustomize/base"}},
		// SyncSource tests - should return syncSource path only, ignoring annotation
		{"syncSource path ignores annotation", syncSourceApp1, syncSourceApp1.Spec.SourceHydrator.GetSyncSource(), []string{"ksapps"}},
		{"syncSource path with annotation should ignore annotation", syncSourceApp2, syncSourceApp2.Spec.SourceHydrator.GetSyncSource(), []string{"helm-charts"}},
		// DrySource without annotation - should fallback to source path like syncSource does
		{"drySource without annotation returns source path", getSourceHydratorApp("", "source/envs/dev", "ksapps"), v1alpha1.ApplicationSource{RepoURL: "https://github.com/example/repo", Path: "source/envs/dev"}, []string{"source/envs/dev"}},
	}
	for _, tt := range tests {
		ttc := tt
		t.Run(ttc.name, func(t *testing.T) {
			t.Parallel()
			assert.ElementsMatch(t, ttc.expectedPaths, GetAppRefreshPaths(ttc.app, ttc.source), "GetAppRefreshPath()")
		})
	}
}
