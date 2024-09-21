package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	goio "io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/reposerver/cache"
	repositorymocks "github.com/argoproj/argo-cd/v2/reposerver/cache/mocks"
	"github.com/argoproj/argo-cd/v2/reposerver/metrics"
	fileutil "github.com/argoproj/argo-cd/v2/test/fixture/path"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/git"
	gitmocks "github.com/argoproj/argo-cd/v2/util/git/mocks"
	"github.com/argoproj/argo-cd/v2/util/helm"
	helmmocks "github.com/argoproj/argo-cd/v2/util/helm/mocks"
	"github.com/argoproj/argo-cd/v2/util/io"
	iomocks "github.com/argoproj/argo-cd/v2/util/io/mocks"
)

const testSignature = `gpg: Signature made Wed Feb 26 23:22:34 2020 CET
gpg:                using RSA key 4AEE18F83AFDEB23
gpg: Good signature from "GitHub (web-flow commit signing) <noreply@github.com>" [ultimate]
`

type clientFunc func(*gitmocks.Client, *helmmocks.Client, *iomocks.TempPaths)

type repoCacheMocks struct {
	mock.Mock
	cacheutilCache *cacheutil.Cache
	cache          *cache.Cache
	mockCache      *repositorymocks.MockRepoCache
}

type newGitRepoHelmChartOptions struct {
	chartName    string
	chartVersion string
	// valuesFiles is a map of the values file name to the key/value pairs to be written to the file
	valuesFiles map[string]map[string]string
}

type newGitRepoOptions struct {
	path             string
	createPath       bool
	remote           string
	addEmptyCommit   bool
	helmChartOptions newGitRepoHelmChartOptions
}

func newCacheMocks() *repoCacheMocks {
	return newCacheMocksWithOpts(1*time.Minute, 1*time.Minute, 10*time.Second)
}

func newCacheMocksWithOpts(repoCacheExpiration, revisionCacheExpiration, revisionCacheLockTimeout time.Duration) *repoCacheMocks {
	mockRepoCache := repositorymocks.NewMockRepoCache(&repositorymocks.MockCacheOptions{
		RepoCacheExpiration:     1 * time.Minute,
		RevisionCacheExpiration: 1 * time.Minute,
		ReadDelay:               0,
		WriteDelay:              0,
	})
	cacheutilCache := cacheutil.NewCache(mockRepoCache.RedisClient)
	return &repoCacheMocks{
		cacheutilCache: cacheutilCache,
		cache:          cache.NewCache(cacheutilCache, repoCacheExpiration, revisionCacheExpiration, revisionCacheLockTimeout),
		mockCache:      mockRepoCache,
	}
}

func newServiceWithMocks(t *testing.T, root string, signed bool) (*Service, *gitmocks.Client, *repoCacheMocks) {
	root, err := filepath.Abs(root)
	if err != nil {
		panic(err)
	}
	return newServiceWithOpt(t, func(gitClient *gitmocks.Client, helmClient *helmmocks.Client, paths *iomocks.TempPaths) {
		gitClient.On("Init").Return(nil)
		gitClient.On("IsRevisionPresent", mock.Anything).Return(false)
		gitClient.On("Fetch", mock.Anything).Return(nil)
		gitClient.On("Checkout", mock.Anything, mock.Anything).Return(nil)
		gitClient.On("LsRemote", mock.Anything).Return(mock.Anything, nil)
		gitClient.On("CommitSHA").Return(mock.Anything, nil)
		gitClient.On("Root").Return(root)
		gitClient.On("IsAnnotatedTag").Return(false)
		if signed {
			gitClient.On("VerifyCommitSignature", mock.Anything).Return(testSignature, nil)
		} else {
			gitClient.On("VerifyCommitSignature", mock.Anything).Return("", nil)
		}

		chart := "my-chart"
		oobChart := "out-of-bounds-chart"
		version := "1.1.0"
		helmClient.On("GetIndex", mock.AnythingOfType("bool"), mock.Anything).Return(&helm.Index{Entries: map[string]helm.Entries{
			chart:    {{Version: "1.0.0"}, {Version: version}},
			oobChart: {{Version: "1.0.0"}, {Version: version}},
		}}, nil)
		helmClient.On("ExtractChart", chart, version, "", false, int64(0), false).Return("./testdata/my-chart", io.NopCloser, nil)
		helmClient.On("ExtractChart", oobChart, version, "", false, int64(0), false).Return("./testdata2/out-of-bounds-chart", io.NopCloser, nil)
		helmClient.On("CleanChartCache", chart, version, "").Return(nil)
		helmClient.On("CleanChartCache", oobChart, version, "").Return(nil)
		helmClient.On("DependencyBuild").Return(nil)

		paths.On("Add", mock.Anything, mock.Anything).Return(root, nil)
		paths.On("GetPath", mock.Anything).Return(root, nil)
		paths.On("GetPathIfExists", mock.Anything).Return(root, nil)
		paths.On("GetPaths").Return(map[string]string{"fake-nonce": root})
	}, root)
}

func newServiceWithOpt(t *testing.T, cf clientFunc, root string) (*Service, *gitmocks.Client, *repoCacheMocks) {
	helmClient := &helmmocks.Client{}
	gitClient := &gitmocks.Client{}
	paths := &iomocks.TempPaths{}
	cf(gitClient, helmClient, paths)
	cacheMocks := newCacheMocks()
	t.Cleanup(cacheMocks.mockCache.StopRedisCallback)
	service := NewService(metrics.NewMetricsServer(), cacheMocks.cache, RepoServerInitConstants{ParallelismLimit: 1}, argo.NewResourceTracking(), &git.NoopCredsStore{}, root)

	service.newGitClient = func(rawRepoURL string, root string, creds git.Creds, insecure bool, enableLfs bool, proxy string, noProxy string, opts ...git.ClientOpts) (client git.Client, e error) {
		return gitClient, nil
	}
	service.newHelmClient = func(repoURL string, creds helm.Creds, enableOci bool, proxy string, noProxy string, opts ...helm.ClientOpts) helm.Client {
		return helmClient
	}
	service.gitRepoInitializer = func(rootPath string) goio.Closer {
		return io.NopCloser
	}
	service.gitRepoPaths = paths
	return service, gitClient, cacheMocks
}

func newService(t *testing.T, root string) *Service {
	service, _, _ := newServiceWithMocks(t, root, false)
	return service
}

func newServiceWithSignature(t *testing.T, root string) *Service {
	service, _, _ := newServiceWithMocks(t, root, true)
	return service
}

func newServiceWithCommitSHA(t *testing.T, root, revision string) *Service {
	var revisionErr error

	commitSHARegex := regexp.MustCompile("^[0-9A-Fa-f]{40}$")
	if !commitSHARegex.MatchString(revision) {
		revisionErr = errors.New("not a commit SHA")
	}

	service, gitClient, _ := newServiceWithOpt(t, func(gitClient *gitmocks.Client, helmClient *helmmocks.Client, paths *iomocks.TempPaths) {
		gitClient.On("Init").Return(nil)
		gitClient.On("IsRevisionPresent", mock.Anything).Return(false)
		gitClient.On("Fetch", mock.Anything).Return(nil)
		gitClient.On("Checkout", mock.Anything, mock.Anything).Return(nil)
		gitClient.On("LsRemote", revision).Return(revision, revisionErr)
		gitClient.On("CommitSHA").Return("632039659e542ed7de0c170a4fcc1c571b288fc0", nil)
		gitClient.On("Root").Return(root)
		paths.On("GetPath", mock.Anything).Return(root, nil)
		paths.On("GetPathIfExists", mock.Anything).Return(root, nil)
	}, root)

	service.newGitClient = func(rawRepoURL string, root string, creds git.Creds, insecure bool, enableLfs bool, proxy string, noProxy string, opts ...git.ClientOpts) (client git.Client, e error) {
		return gitClient, nil
	}

	return service
}

func TestGenerateYamlManifestInDir(t *testing.T) {
	service := newService(t, "../../manifests/base")

	src := argoappv1.ApplicationSource{Path: "."}
	q := apiclient.ManifestRequest{
		Repo:               &argoappv1.Repository{},
		ApplicationSource:  &src,
		ProjectName:        "something",
		ProjectSourceRepos: []string{"*"},
	}

	// update this value if we add/remove manifests
	const countOfManifests = 50

	res1, err := service.GenerateManifest(context.Background(), &q)

	require.NoError(t, err)
	assert.Len(t, res1.Manifests, countOfManifests)

	// this will test concatenated manifests to verify we split YAMLs correctly
	res2, err := GenerateManifests(context.Background(), "./testdata/concatenated", "/", "", &q, false, &git.NoopCredsStore{}, resource.MustParse("0"), nil)
	require.NoError(t, err)
	assert.Len(t, res2.Manifests, 3)
}

func Test_GenerateManifests_NoOutOfBoundsAccess(t *testing.T) {
	testCases := []struct {
		name                    string
		outOfBoundsFilename     string
		outOfBoundsFileContents string
		mustNotContain          string // Optional string that must not appear in error or manifest output. If empty, use outOfBoundsFileContents.
	}{
		{
			name:                    "out of bounds JSON file should not appear in error output",
			outOfBoundsFilename:     "test.json",
			outOfBoundsFileContents: `{"some": "json"}`,
		},
		{
			name:                    "malformed JSON file contents should not appear in error output",
			outOfBoundsFilename:     "test.json",
			outOfBoundsFileContents: "$",
		},
		{
			name:                "out of bounds JSON manifest should not appear in manifest output",
			outOfBoundsFilename: "test.json",
			// JSON marshalling is deterministic. So if there's a leak, exactly this should appear in the manifests.
			outOfBoundsFileContents: `{"apiVersion":"v1","kind":"Secret","metadata":{"name":"test","namespace":"default"},"type":"Opaque"}`,
		},
		{
			name:                    "out of bounds YAML manifest should not appear in manifest output",
			outOfBoundsFilename:     "test.yaml",
			outOfBoundsFileContents: "apiVersion: v1\nkind: Secret\nmetadata:\n  name: test\n  namespace: default\ntype: Opaque",
			mustNotContain:          `{"apiVersion":"v1","kind":"Secret","metadata":{"name":"test","namespace":"default"},"type":"Opaque"}`,
		},
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase
		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			outOfBoundsDir := t.TempDir()
			outOfBoundsFile := path.Join(outOfBoundsDir, testCaseCopy.outOfBoundsFilename)
			err := os.WriteFile(outOfBoundsFile, []byte(testCaseCopy.outOfBoundsFileContents), os.FileMode(0o444))
			require.NoError(t, err)

			repoDir := t.TempDir()
			err = os.Symlink(outOfBoundsFile, path.Join(repoDir, testCaseCopy.outOfBoundsFilename))
			require.NoError(t, err)

			mustNotContain := testCaseCopy.outOfBoundsFileContents
			if testCaseCopy.mustNotContain != "" {
				mustNotContain = testCaseCopy.mustNotContain
			}

			q := apiclient.ManifestRequest{
				Repo: &argoappv1.Repository{}, ApplicationSource: &argoappv1.ApplicationSource{}, ProjectName: "something",
				ProjectSourceRepos: []string{"*"},
			}
			res, err := GenerateManifests(context.Background(), repoDir, "", "", &q, false, &git.NoopCredsStore{}, resource.MustParse("0"), nil)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), mustNotContain)
			assert.Contains(t, err.Error(), "illegal filepath")
			assert.Nil(t, res)
		})
	}
}

func TestGenerateManifests_MissingSymlinkDestination(t *testing.T) {
	repoDir := t.TempDir()
	err := os.Symlink("/obviously/does/not/exist", path.Join(repoDir, "test.yaml"))
	require.NoError(t, err)

	q := apiclient.ManifestRequest{
		Repo: &argoappv1.Repository{}, ApplicationSource: &argoappv1.ApplicationSource{}, ProjectName: "something",
		ProjectSourceRepos: []string{"*"},
	}
	_, err = GenerateManifests(context.Background(), repoDir, "", "", &q, false, &git.NoopCredsStore{}, resource.MustParse("0"), nil)
	require.NoError(t, err)
}

func TestGenerateManifests_K8SAPIResetCache(t *testing.T) {
	service := newService(t, "../../manifests/base")

	src := argoappv1.ApplicationSource{Path: "."}
	q := apiclient.ManifestRequest{
		KubeVersion:        "v1.16.0",
		Repo:               &argoappv1.Repository{},
		ApplicationSource:  &src,
		ProjectName:        "something",
		ProjectSourceRepos: []string{"*"},
	}

	cachedFakeResponse := &apiclient.ManifestResponse{Manifests: []string{"Fake"}, Revision: mock.Anything}

	err := service.cache.SetManifests(mock.Anything, &src, q.RefSources, &q, "", "", "", "", &cache.CachedManifestResponse{ManifestResponse: cachedFakeResponse}, nil)
	require.NoError(t, err)

	res, err := service.GenerateManifest(context.Background(), &q)
	require.NoError(t, err)
	assert.Equal(t, cachedFakeResponse, res)

	q.KubeVersion = "v1.17.0"
	res, err = service.GenerateManifest(context.Background(), &q)
	require.NoError(t, err)
	assert.NotEqual(t, cachedFakeResponse, res)
	assert.Greater(t, len(res.Manifests), 1)
}

func TestGenerateManifests_EmptyCache(t *testing.T) {
	service, gitMocks, mockCache := newServiceWithMocks(t, "../../manifests/base", false)

	src := argoappv1.ApplicationSource{Path: "."}
	q := apiclient.ManifestRequest{
		Repo:               &argoappv1.Repository{},
		ApplicationSource:  &src,
		ProjectName:        "something",
		ProjectSourceRepos: []string{"*"},
	}

	err := service.cache.SetManifests(mock.Anything, &src, q.RefSources, &q, "", "", "", "", &cache.CachedManifestResponse{ManifestResponse: nil}, nil)
	require.NoError(t, err)

	res, err := service.GenerateManifest(context.Background(), &q)
	require.NoError(t, err)
	assert.NotEmpty(t, res.Manifests)
	mockCache.mockCache.AssertCacheCalledTimes(t, &repositorymocks.CacheCallCounts{
		ExternalSets:    2,
		ExternalGets:    2,
		ExternalDeletes: 1,
	})
	gitMocks.AssertCalled(t, "LsRemote", mock.Anything)
	gitMocks.AssertCalled(t, "Fetch", mock.Anything)
}

// Test that when Generate manifest is called with a source that is ref only it does not try to generate manifests or hit the manifest cache
// but it does resolve and cache the revision
func TestGenerateManifest_RefOnlyShortCircuit(t *testing.T) {
	lsremoteCalled := false
	dir := t.TempDir()
	repopath := fmt.Sprintf("%s/tmprepo", dir)
	repoRemote := fmt.Sprintf("file://%s", repopath)
	cacheMocks := newCacheMocks()
	t.Cleanup(cacheMocks.mockCache.StopRedisCallback)
	service := NewService(metrics.NewMetricsServer(), cacheMocks.cache, RepoServerInitConstants{ParallelismLimit: 1}, argo.NewResourceTracking(), &git.NoopCredsStore{}, repopath)
	service.newGitClient = func(rawRepoURL string, root string, creds git.Creds, insecure bool, enableLfs bool, proxy string, noProxy string, opts ...git.ClientOpts) (client git.Client, e error) {
		opts = append(opts, git.WithEventHandlers(git.EventHandlers{
			// Primary check, we want to make sure ls-remote is not called when the item is in cache
			OnLsRemote: func(repo string) func() {
				return func() {
					lsremoteCalled = true
				}
			},
			OnFetch: func(repo string) func() {
				return func() {
					assert.Fail(t, "Fetch should not be called from GenerateManifest when the source is ref only")
				}
			},
		}))
		gitClient, err := git.NewClientExt(rawRepoURL, root, creds, insecure, enableLfs, proxy, noProxy, opts...)
		return gitClient, err
	}
	revision := initGitRepo(t, newGitRepoOptions{
		path:           repopath,
		createPath:     true,
		remote:         repoRemote,
		addEmptyCommit: true,
	})
	src := argoappv1.ApplicationSource{RepoURL: repoRemote, TargetRevision: "HEAD", Ref: "test-ref"}
	repo := &argoappv1.Repository{
		Repo: repoRemote,
	}
	q := apiclient.ManifestRequest{
		Repo:               repo,
		Revision:           "HEAD",
		HasMultipleSources: true,
		ApplicationSource:  &src,
		ProjectName:        "default",
		ProjectSourceRepos: []string{"*"},
	}
	_, err := service.GenerateManifest(context.Background(), &q)
	require.NoError(t, err)
	cacheMocks.mockCache.AssertCacheCalledTimes(t, &repositorymocks.CacheCallCounts{
		ExternalSets: 2,
		ExternalGets: 2,
	})
	assert.True(t, lsremoteCalled, "ls-remote should be called when the source is ref only")
	var revisions [][2]string
	require.NoError(t, cacheMocks.cacheutilCache.GetItem(fmt.Sprintf("git-refs|%s", repoRemote), &revisions))
	assert.ElementsMatch(t, [][2]string{{"refs/heads/main", revision}, {"HEAD", "ref: refs/heads/main"}}, revisions)
}

// Test that calling manifest generation on source helm reference helm files that when the revision is cached it does not call ls-remote
func TestGenerateManifestsHelmWithRefs_CachedNoLsRemote(t *testing.T) {
	dir := t.TempDir()
	repopath := fmt.Sprintf("%s/tmprepo", dir)
	cacheMocks := newCacheMocks()
	t.Cleanup(func() {
		cacheMocks.mockCache.StopRedisCallback()
		err := filepath.WalkDir(dir,
			func(path string, di fs.DirEntry, err error) error {
				if err == nil {
					return os.Chmod(path, 0o777)
				}
				return err
			})
		if err != nil {
			t.Fatal(err)
		}
	})
	service := NewService(metrics.NewMetricsServer(), cacheMocks.cache, RepoServerInitConstants{ParallelismLimit: 1}, argo.NewResourceTracking(), &git.NoopCredsStore{}, repopath)
	var gitClient git.Client
	var err error
	service.newGitClient = func(rawRepoURL string, root string, creds git.Creds, insecure bool, enableLfs bool, proxy string, noProxy string, opts ...git.ClientOpts) (client git.Client, e error) {
		opts = append(opts, git.WithEventHandlers(git.EventHandlers{
			// Primary check, we want to make sure ls-remote is not called when the item is in cache
			OnLsRemote: func(repo string) func() {
				return func() {
					assert.Fail(t, "LsRemote should not be called when the item is in cache")
				}
			},
		}))
		gitClient, err = git.NewClientExt(rawRepoURL, root, creds, insecure, enableLfs, proxy, noProxy, opts...)
		return gitClient, err
	}
	repoRemote := fmt.Sprintf("file://%s", repopath)
	revision := initGitRepo(t, newGitRepoOptions{
		path:       repopath,
		createPath: true,
		remote:     repoRemote,
		helmChartOptions: newGitRepoHelmChartOptions{
			chartName:    "my-chart",
			chartVersion: "v1.0.0",
			valuesFiles:  map[string]map[string]string{"test.yaml": {"testval": "test"}},
		},
	})
	src := argoappv1.ApplicationSource{RepoURL: repoRemote, Path: ".", TargetRevision: "HEAD", Helm: &argoappv1.ApplicationSourceHelm{
		ValueFiles: []string{"$ref/test.yaml"},
	}}
	repo := &argoappv1.Repository{
		Repo: repoRemote,
	}
	q := apiclient.ManifestRequest{
		Repo:               repo,
		Revision:           "HEAD",
		HasMultipleSources: true,
		ApplicationSource:  &src,
		ProjectName:        "default",
		ProjectSourceRepos: []string{"*"},
		RefSources:         map[string]*argoappv1.RefTarget{"$ref": {TargetRevision: "HEAD", Repo: *repo}},
	}
	err = cacheMocks.cacheutilCache.SetItem(fmt.Sprintf("git-refs|%s", repoRemote), [][2]string{{"HEAD", revision}}, nil)
	require.NoError(t, err)
	_, err = service.GenerateManifest(context.Background(), &q)
	require.NoError(t, err)
	cacheMocks.mockCache.AssertCacheCalledTimes(t, &repositorymocks.CacheCallCounts{
		ExternalSets: 2,
		ExternalGets: 5,
	})
}

// ensure we can use a semver constraint range (>= 1.0.0) and get back the correct chart (1.0.0)
func TestHelmManifestFromChartRepo(t *testing.T) {
	root := t.TempDir()
	service, gitMocks, mockCache := newServiceWithMocks(t, root, false)
	source := &argoappv1.ApplicationSource{Chart: "my-chart", TargetRevision: ">= 1.0.0"}
	request := &apiclient.ManifestRequest{
		Repo: &argoappv1.Repository{}, ApplicationSource: source, NoCache: true, ProjectName: "something",
		ProjectSourceRepos: []string{"*"},
	}
	response, err := service.GenerateManifest(context.Background(), request)
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, &apiclient.ManifestResponse{
		Manifests:  []string{"{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"my-map\"}}"},
		Namespace:  "",
		Server:     "",
		Revision:   "1.1.0",
		SourceType: "Helm",
		Commands:   []string{`helm template . --name-template "" --include-crds`},
	}, response)
	mockCache.mockCache.AssertCacheCalledTimes(t, &repositorymocks.CacheCallCounts{
		ExternalSets: 1,
		ExternalGets: 0,
	})
	gitMocks.AssertNotCalled(t, "LsRemote", mock.Anything)
}

func TestHelmChartReferencingExternalValues(t *testing.T) {
	service := newService(t, ".")
	spec := argoappv1.ApplicationSpec{
		Sources: []argoappv1.ApplicationSource{
			{RepoURL: "https://helm.example.com", Chart: "my-chart", TargetRevision: ">= 1.0.0", Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles: []string{"$ref/testdata/my-chart/my-chart-values.yaml"},
			}},
			{Ref: "ref", RepoURL: "https://git.example.com/test/repo"},
		},
	}
	refSources, err := argo.GetRefSources(context.Background(), spec.Sources, spec.Project, func(ctx context.Context, url string, project string) (*argoappv1.Repository, error) {
		return &argoappv1.Repository{
			Repo: "https://git.example.com/test/repo",
		}, nil
	}, []string{}, false)
	require.NoError(t, err)
	request := &apiclient.ManifestRequest{
		Repo: &argoappv1.Repository{}, ApplicationSource: &spec.Sources[0], NoCache: true, RefSources: refSources, HasMultipleSources: true, ProjectName: "something",
		ProjectSourceRepos: []string{"*"},
	}
	response, err := service.GenerateManifest(context.Background(), request)
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, &apiclient.ManifestResponse{
		Manifests:  []string{"{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"my-map\"}}"},
		Namespace:  "",
		Server:     "",
		Revision:   "1.1.0",
		SourceType: "Helm",
		Commands:   []string{`helm template . --name-template "" --values ./testdata/my-chart/my-chart-values.yaml --include-crds`},
	}, response)
}

func TestHelmChartReferencingExternalValues_InvalidRefs(t *testing.T) {
	spec := argoappv1.ApplicationSpec{
		Sources: []argoappv1.ApplicationSource{
			{RepoURL: "https://helm.example.com", Chart: "my-chart", TargetRevision: ">= 1.0.0", Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles: []string{"$ref/testdata/my-chart/my-chart-values.yaml"},
			}},
			{RepoURL: "https://git.example.com/test/repo"},
		},
	}

	// Empty refsource
	service := newService(t, ".")

	getRepository := func(ctx context.Context, url string, project string) (*argoappv1.Repository, error) {
		return &argoappv1.Repository{
			Repo: "https://git.example.com/test/repo",
		}, nil
	}

	refSources, err := argo.GetRefSources(context.Background(), spec.Sources, spec.Project, getRepository, []string{}, false)
	require.NoError(t, err)

	request := &apiclient.ManifestRequest{
		Repo: &argoappv1.Repository{}, ApplicationSource: &spec.Sources[0], NoCache: true, RefSources: refSources, HasMultipleSources: true, ProjectName: "something",
		ProjectSourceRepos: []string{"*"},
	}
	response, err := service.GenerateManifest(context.Background(), request)
	require.Error(t, err)
	assert.Nil(t, response)

	// Invalid ref
	service = newService(t, ".")

	spec.Sources[1].Ref = "Invalid"
	refSources, err = argo.GetRefSources(context.Background(), spec.Sources, spec.Project, getRepository, []string{}, false)
	require.NoError(t, err)

	request = &apiclient.ManifestRequest{
		Repo: &argoappv1.Repository{}, ApplicationSource: &spec.Sources[0], NoCache: true, RefSources: refSources, HasMultipleSources: true, ProjectName: "something",
		ProjectSourceRepos: []string{"*"},
	}
	response, err = service.GenerateManifest(context.Background(), request)
	require.Error(t, err)
	assert.Nil(t, response)

	// Helm chart as ref (unsupported)
	service = newService(t, ".")

	spec.Sources[1].Ref = "ref"
	spec.Sources[1].Chart = "helm-chart"
	refSources, err = argo.GetRefSources(context.Background(), spec.Sources, spec.Project, getRepository, []string{}, false)
	require.NoError(t, err)

	request = &apiclient.ManifestRequest{
		Repo: &argoappv1.Repository{}, ApplicationSource: &spec.Sources[0], NoCache: true, RefSources: refSources, HasMultipleSources: true, ProjectName: "something",
		ProjectSourceRepos: []string{"*"},
	}
	response, err = service.GenerateManifest(context.Background(), request)
	require.Error(t, err)
	assert.Nil(t, response)
}

func TestHelmChartReferencingExternalValues_OutOfBounds_Symlink(t *testing.T) {
	service := newService(t, ".")
	err := os.Mkdir("testdata/oob-symlink", 0o755)
	require.NoError(t, err)
	t.Cleanup(func() {
		err = os.RemoveAll("testdata/oob-symlink")
		require.NoError(t, err)
	})
	// Create a symlink to a file outside the repo
	err = os.Symlink("../../../values.yaml", "./testdata/oob-symlink/oob-symlink.yaml")
	// Create a regular file to reference from another source
	err = os.WriteFile("./testdata/oob-symlink/values.yaml", []byte("foo: bar"), 0o644)
	require.NoError(t, err)
	spec := argoappv1.ApplicationSpec{
		Project: "default",
		Sources: []argoappv1.ApplicationSource{
			{RepoURL: "https://helm.example.com", Chart: "my-chart", TargetRevision: ">= 1.0.0", Helm: &argoappv1.ApplicationSourceHelm{
				// Reference `ref` but do not use the oob symlink. The mere existence of the link should be enough to
				// cause an error.
				ValueFiles: []string{"$ref/testdata/oob-symlink/values.yaml"},
			}},
			{Ref: "ref", RepoURL: "https://git.example.com/test/repo"},
		},
	}
	refSources, err := argo.GetRefSources(context.Background(), spec.Sources, spec.Project, func(ctx context.Context, url string, project string) (*argoappv1.Repository, error) {
		return &argoappv1.Repository{
			Repo: "https://git.example.com/test/repo",
		}, nil
	}, []string{}, false)
	require.NoError(t, err)
	request := &apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: &spec.Sources[0], NoCache: true, RefSources: refSources, HasMultipleSources: true}
	_, err = service.GenerateManifest(context.Background(), request)
	require.Error(t, err)
}

func TestGenerateManifestsUseExactRevision(t *testing.T) {
	service, gitClient, _ := newServiceWithMocks(t, ".", false)

	src := argoappv1.ApplicationSource{Path: "./testdata/recurse", Directory: &argoappv1.ApplicationSourceDirectory{Recurse: true}}

	q := apiclient.ManifestRequest{
		Repo: &argoappv1.Repository{}, ApplicationSource: &src, Revision: "abc", ProjectName: "something",
		ProjectSourceRepos: []string{"*"},
	}

	res1, err := service.GenerateManifest(context.Background(), &q)
	require.NoError(t, err)
	assert.Len(t, res1.Manifests, 2)
	assert.Equal(t, "abc", gitClient.Calls[0].Arguments[0])
}

func TestRecurseManifestsInDir(t *testing.T) {
	service := newService(t, ".")

	src := argoappv1.ApplicationSource{Path: "./testdata/recurse", Directory: &argoappv1.ApplicationSourceDirectory{Recurse: true}}

	q := apiclient.ManifestRequest{
		Repo: &argoappv1.Repository{}, ApplicationSource: &src, ProjectName: "something",
		ProjectSourceRepos: []string{"*"},
	}

	res1, err := service.GenerateManifest(context.Background(), &q)
	require.NoError(t, err)
	assert.Len(t, res1.Manifests, 2)
}

func TestInvalidManifestsInDir(t *testing.T) {
	service := newService(t, ".")

	src := argoappv1.ApplicationSource{Path: "./testdata/invalid-manifests", Directory: &argoappv1.ApplicationSourceDirectory{Recurse: true}}

	q := apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: &src}

	_, err := service.GenerateManifest(context.Background(), &q)
	require.Error(t, err)
}

func TestInvalidMetadata(t *testing.T) {
	service := newService(t, ".")

	src := argoappv1.ApplicationSource{Path: "./testdata/invalid-metadata", Directory: &argoappv1.ApplicationSourceDirectory{Recurse: true}}
	q := apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: &src, AppLabelKey: "test", AppName: "invalid-metadata", TrackingMethod: "annotation+label"}
	_, err := service.GenerateManifest(context.Background(), &q)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contains non-string value in the map under key \"invalid\"")
}

func TestNilMetadataAccessors(t *testing.T) {
	service := newService(t, ".")
	expected := "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"annotations\":{\"argocd.argoproj.io/tracking-id\":\"nil-metadata-accessors:/ConfigMap:/my-map\"},\"labels\":{\"test\":\"nil-metadata-accessors\"},\"name\":\"my-map\"},\"stringData\":{\"foo\":\"bar\"}}"

	src := argoappv1.ApplicationSource{Path: "./testdata/nil-metadata-accessors", Directory: &argoappv1.ApplicationSourceDirectory{Recurse: true}}
	q := apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: &src, AppLabelKey: "test", AppName: "nil-metadata-accessors", TrackingMethod: "annotation+label"}
	res, err := service.GenerateManifest(context.Background(), &q)
	require.NoError(t, err)
	assert.Len(t, res.Manifests, 1)
	assert.Equal(t, expected, res.Manifests[0])
}

func TestGenerateJsonnetManifestInDir(t *testing.T) {
	service := newService(t, ".")

	q := apiclient.ManifestRequest{
		Repo: &argoappv1.Repository{},
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./testdata/jsonnet",
			Directory: &argoappv1.ApplicationSourceDirectory{
				Jsonnet: argoappv1.ApplicationSourceJsonnet{
					ExtVars: []argoappv1.JsonnetVar{{Name: "extVarString", Value: "extVarString"}, {Name: "extVarCode", Value: "\"extVarCode\"", Code: true}},
					TLAs:    []argoappv1.JsonnetVar{{Name: "tlaString", Value: "tlaString"}, {Name: "tlaCode", Value: "\"tlaCode\"", Code: true}},
					Libs:    []string{"testdata/jsonnet/vendor"},
				},
			},
		},
		ProjectName:        "something",
		ProjectSourceRepos: []string{"*"},
	}
	res1, err := service.GenerateManifest(context.Background(), &q)
	require.NoError(t, err)
	assert.Len(t, res1.Manifests, 2)
}

func TestGenerateJsonnetManifestInRootDir(t *testing.T) {
	service := newService(t, "testdata/jsonnet-1")

	q := apiclient.ManifestRequest{
		Repo: &argoappv1.Repository{},
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: ".",
			Directory: &argoappv1.ApplicationSourceDirectory{
				Jsonnet: argoappv1.ApplicationSourceJsonnet{
					ExtVars: []argoappv1.JsonnetVar{{Name: "extVarString", Value: "extVarString"}, {Name: "extVarCode", Value: "\"extVarCode\"", Code: true}},
					TLAs:    []argoappv1.JsonnetVar{{Name: "tlaString", Value: "tlaString"}, {Name: "tlaCode", Value: "\"tlaCode\"", Code: true}},
					Libs:    []string{"."},
				},
			},
		},
		ProjectName:        "something",
		ProjectSourceRepos: []string{"*"},
	}
	res1, err := service.GenerateManifest(context.Background(), &q)
	require.NoError(t, err)
	assert.Len(t, res1.Manifests, 2)
}

func TestGenerateJsonnetLibOutside(t *testing.T) {
	service := newService(t, ".")

	q := apiclient.ManifestRequest{
		Repo: &argoappv1.Repository{},
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./testdata/jsonnet",
			Directory: &argoappv1.ApplicationSourceDirectory{
				Jsonnet: argoappv1.ApplicationSourceJsonnet{
					Libs: []string{"../../../testdata/jsonnet/vendor"},
				},
			},
		},
		ProjectName:        "something",
		ProjectSourceRepos: []string{"*"},
	}
	_, err := service.GenerateManifest(context.Background(), &q)
	require.Error(t, err)
	require.Contains(t, err.Error(), "file '../../../testdata/jsonnet/vendor' resolved to outside repository root")
}

func TestManifestGenErrorCacheByNumRequests(t *testing.T) {
	// Returns the state of the manifest generation cache, by querying the cache for the previously set result
	getRecentCachedEntry := func(service *Service, manifestRequest *apiclient.ManifestRequest) *cache.CachedManifestResponse {
		assert.NotNil(t, service)
		assert.NotNil(t, manifestRequest)

		cachedManifestResponse := &cache.CachedManifestResponse{}
		err := service.cache.GetManifests(mock.Anything, manifestRequest.ApplicationSource, manifestRequest.RefSources, manifestRequest, manifestRequest.Namespace, "", manifestRequest.AppLabelKey, manifestRequest.AppName, cachedManifestResponse, nil)
		require.NoError(t, err)
		return cachedManifestResponse
	}

	// Example:
	// With repo server (test) parameters:
	// - PauseGenerationAfterFailedGenerationAttempts: 2
	// - PauseGenerationOnFailureForRequests: 4
	// - TotalCacheInvocations: 10
	//
	// After 2 manifest generation failures in a row, the next 4 manifest generation requests should be cached,
	// with the next 2 after that being uncached. Here's how it looks...
	//
	//  request count) result
	// --------------------------
	// 1) Attempt to generate manifest, fails.
	// 2) Second attempt to generate manifest, fails.
	// 3) Return cached error attempt from #2
	// 4) Return cached error attempt from #2
	// 5) Return cached error attempt from #2
	// 6) Return cached error attempt from #2. Max response limit hit, so reset cache entry.
	// 7) Attempt to generate manifest, fails.
	// 8) Attempt to generate manifest, fails.
	// 9) Return cached error attempt from #8
	// 10) Return cached error attempt from #8

	// The same pattern PauseGenerationAfterFailedGenerationAttempts generation attempts, followed by
	// PauseGenerationOnFailureForRequests cached responses, should apply for various combinations of
	// both parameters.

	tests := []struct {
		PauseGenerationAfterFailedGenerationAttempts int
		PauseGenerationOnFailureForRequests          int
		TotalCacheInvocations                        int
	}{
		{2, 4, 10},
		{3, 5, 10},
		{1, 2, 5},
	}
	for _, tt := range tests {
		testName := fmt.Sprintf("gen-attempts-%d-pause-%d-total-%d", tt.PauseGenerationAfterFailedGenerationAttempts, tt.PauseGenerationOnFailureForRequests, tt.TotalCacheInvocations)
		t.Run(testName, func(t *testing.T) {
			service := newService(t, ".")

			service.initConstants = RepoServerInitConstants{
				ParallelismLimit: 1,
				PauseGenerationAfterFailedGenerationAttempts: tt.PauseGenerationAfterFailedGenerationAttempts,
				PauseGenerationOnFailureForMinutes:           0,
				PauseGenerationOnFailureForRequests:          tt.PauseGenerationOnFailureForRequests,
			}

			totalAttempts := service.initConstants.PauseGenerationAfterFailedGenerationAttempts + service.initConstants.PauseGenerationOnFailureForRequests

			for invocationCount := 0; invocationCount < tt.TotalCacheInvocations; invocationCount++ {
				adjustedInvocation := invocationCount % totalAttempts

				fmt.Printf("%d )-------------------------------------------\n", invocationCount)

				manifestRequest := &apiclient.ManifestRequest{
					Repo:    &argoappv1.Repository{},
					AppName: "test",
					ApplicationSource: &argoappv1.ApplicationSource{
						Path: "./testdata/invalid-helm",
					},
				}

				res, err := service.GenerateManifest(context.Background(), manifestRequest)

				// Verify invariant: res != nil xor err != nil
				if err != nil {
					assert.Nil(t, res, "both err and res are non-nil res: %v   err: %v", res, err)
				} else {
					assert.NotNil(t, res, "both err and res are nil")
				}

				cachedManifestResponse := getRecentCachedEntry(service, manifestRequest)

				isCachedError := err != nil && strings.HasPrefix(err.Error(), cachedManifestGenerationPrefix)

				if adjustedInvocation < service.initConstants.PauseGenerationAfterFailedGenerationAttempts {
					// GenerateManifest should not return cached errors for the first X responses, where X is the FailGenAttempts constants
					require.False(t, isCachedError)

					require.NotNil(t, cachedManifestResponse)
					// nolint:staticcheck
					assert.Nil(t, cachedManifestResponse.ManifestResponse)
					// nolint:staticcheck
					assert.NotEqual(t, 0, cachedManifestResponse.FirstFailureTimestamp)

					// Internal cache consec failures value should increase with invocations, cached response should stay the same,
					// nolint:staticcheck
					assert.Equal(t, cachedManifestResponse.NumberOfConsecutiveFailures, adjustedInvocation+1)
					// nolint:staticcheck
					assert.Equal(t, 0, cachedManifestResponse.NumberOfCachedResponsesReturned)
				} else {
					// GenerateManifest SHOULD return cached errors for the next X responses, where X is the
					// PauseGenerationOnFailureForRequests constant
					assert.True(t, isCachedError)
					require.NotNil(t, cachedManifestResponse)
					// nolint:staticcheck
					assert.Nil(t, cachedManifestResponse.ManifestResponse)
					// nolint:staticcheck
					assert.NotEqual(t, 0, cachedManifestResponse.FirstFailureTimestamp)

					// Internal cache values should update correctly based on number of return cache entries, consecutive failures should stay the same
					// nolint:staticcheck
					assert.Equal(t, cachedManifestResponse.NumberOfConsecutiveFailures, service.initConstants.PauseGenerationAfterFailedGenerationAttempts)
					// nolint:staticcheck
					assert.Equal(t, cachedManifestResponse.NumberOfCachedResponsesReturned, (adjustedInvocation - service.initConstants.PauseGenerationAfterFailedGenerationAttempts + 1))
				}
			}
		})
	}
}

func TestManifestGenErrorCacheFileContentsChange(t *testing.T) {
	tmpDir := t.TempDir()

	service := newService(t, tmpDir)

	service.initConstants = RepoServerInitConstants{
		ParallelismLimit: 1,
		PauseGenerationAfterFailedGenerationAttempts: 2,
		PauseGenerationOnFailureForMinutes:           0,
		PauseGenerationOnFailureForRequests:          4,
	}

	for step := 0; step < 3; step++ {
		// step 1) Attempt to generate manifests against invalid helm chart (should return uncached error)
		// step 2) Attempt to generate manifest against valid helm chart (should succeed and return valid response)
		// step 3) Attempt to generate manifest against invalid helm chart (should return cached value from step 2)

		errorExpected := step%2 == 0

		// Ensure that the target directory will succeed or fail, so we can verify the cache correctly handles it
		err := os.RemoveAll(tmpDir)
		require.NoError(t, err)
		err = os.MkdirAll(tmpDir, 0o777)
		require.NoError(t, err)
		if errorExpected {
			// Copy invalid helm chart into temporary directory, ensuring manifest generation will fail
			err = fileutil.CopyDir("./testdata/invalid-helm", tmpDir)
			require.NoError(t, err)
		} else {
			// Copy valid helm chart into temporary directory, ensuring generation will succeed
			err = fileutil.CopyDir("./testdata/my-chart", tmpDir)
			require.NoError(t, err)
		}

		res, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
			Repo:    &argoappv1.Repository{},
			AppName: "test",
			ApplicationSource: &argoappv1.ApplicationSource{
				Path: ".",
			},
			ProjectName:        "something",
			ProjectSourceRepos: []string{"*"},
		})

		fmt.Println("-", step, "-", res != nil, err != nil, errorExpected)
		fmt.Println("    err: ", err)
		fmt.Println("    res: ", res)

		if step < 2 {
			if errorExpected {
				require.Error(t, err, "error return value and error expected did not match")
				assert.Nil(t, res, "GenerateManifest return value and expected value did not match")
			} else {
				require.NoError(t, err, "error return value and error expected did not match")
				assert.NotNil(t, res, "GenerateManifest return value and expected value did not match")
			}
		}

		if step == 2 {
			require.NoError(t, err, "error ret val was non-nil on step 3")
			assert.NotNil(t, res, "GenerateManifest ret val was nil on step 3")
		}
	}
}

func TestManifestGenErrorCacheByMinutesElapsed(t *testing.T) {
	tests := []struct {
		// Test with a range of pause expiration thresholds
		PauseGenerationOnFailureForMinutes int
	}{
		{1}, {2}, {10}, {24 * 60},
	}
	for _, tt := range tests {
		testName := fmt.Sprintf("pause-time-%d", tt.PauseGenerationOnFailureForMinutes)
		t.Run(testName, func(t *testing.T) {
			service := newService(t, ".")

			// Here we simulate the passage of time by overriding the now() function of Service
			currentTime := time.Now()
			service.now = func() time.Time {
				return currentTime
			}

			service.initConstants = RepoServerInitConstants{
				ParallelismLimit: 1,
				PauseGenerationAfterFailedGenerationAttempts: 1,
				PauseGenerationOnFailureForMinutes:           tt.PauseGenerationOnFailureForMinutes,
				PauseGenerationOnFailureForRequests:          0,
			}

			// 1) Put the cache into the failure state
			for x := 0; x < 2; x++ {
				res, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
					Repo:    &argoappv1.Repository{},
					AppName: "test",
					ApplicationSource: &argoappv1.ApplicationSource{
						Path: "./testdata/invalid-helm",
					},
				})

				assert.True(t, err != nil && res == nil)

				// Ensure that the second invocation triggers the cached error state
				if x == 1 {
					assert.True(t, strings.HasPrefix(err.Error(), cachedManifestGenerationPrefix))
				}
			}

			// 2) Jump forward X-1 minutes in time, where X is the expiration boundary
			currentTime = currentTime.Add(time.Duration(tt.PauseGenerationOnFailureForMinutes-1) * time.Minute)
			res, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
				Repo:    &argoappv1.Repository{},
				AppName: "test",
				ApplicationSource: &argoappv1.ApplicationSource{
					Path: "./testdata/invalid-helm",
				},
			})

			// 3) Ensure that the cache still returns a cached copy of the last error
			assert.True(t, err != nil && res == nil)
			assert.True(t, strings.HasPrefix(err.Error(), cachedManifestGenerationPrefix))

			// 4) Jump forward 2 minutes in time, such that the pause generation time has elapsed and we should return to normal state
			currentTime = currentTime.Add(2 * time.Minute)

			res, err = service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
				Repo:    &argoappv1.Repository{},
				AppName: "test",
				ApplicationSource: &argoappv1.ApplicationSource{
					Path: "./testdata/invalid-helm",
				},
			})

			// 5) Ensure that the service no longer returns a cached copy of the last error
			assert.True(t, err != nil && res == nil)
			assert.False(t, strings.HasPrefix(err.Error(), cachedManifestGenerationPrefix))
		})
	}
}

func TestManifestGenErrorCacheRespectsNoCache(t *testing.T) {
	service := newService(t, ".")

	service.initConstants = RepoServerInitConstants{
		ParallelismLimit: 1,
		PauseGenerationAfterFailedGenerationAttempts: 1,
		PauseGenerationOnFailureForMinutes:           0,
		PauseGenerationOnFailureForRequests:          4,
	}

	// 1) Put the cache into the failure state
	for x := 0; x < 2; x++ {
		res, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
			Repo:    &argoappv1.Repository{},
			AppName: "test",
			ApplicationSource: &argoappv1.ApplicationSource{
				Path: "./testdata/invalid-helm",
			},
		})

		assert.True(t, err != nil && res == nil)

		// Ensure that the second invocation is cached
		if x == 1 {
			assert.True(t, strings.HasPrefix(err.Error(), cachedManifestGenerationPrefix))
		}
	}

	// 2) Call generateManifest with NoCache enabled
	res, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:    &argoappv1.Repository{},
		AppName: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./testdata/invalid-helm",
		},
		NoCache: true,
	})

	// 3) Ensure that the cache returns a new generation attempt, rather than a previous cached error
	assert.True(t, err != nil && res == nil)
	assert.False(t, strings.HasPrefix(err.Error(), cachedManifestGenerationPrefix))

	// 4) Call generateManifest
	res, err = service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:    &argoappv1.Repository{},
		AppName: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./testdata/invalid-helm",
		},
	})

	// 5) Ensure that the subsequent invocation, after nocache, is cached
	assert.True(t, err != nil && res == nil)
	assert.True(t, strings.HasPrefix(err.Error(), cachedManifestGenerationPrefix))
}

func TestGenerateHelmWithValues(t *testing.T) {
	service := newService(t, "../../util/helm/testdata/redis")

	res, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:    &argoappv1.Repository{},
		AppName: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: ".",
			Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles:   []string{"values-production.yaml"},
				ValuesObject: &runtime.RawExtension{Raw: []byte(`cluster: {slaveCount: 2}`)},
			},
		},
		ProjectName:        "something",
		ProjectSourceRepos: []string{"*"},
	})

	require.NoError(t, err)

	replicasVerified := false
	for _, src := range res.Manifests {
		obj := unstructured.Unstructured{}
		err = json.Unmarshal([]byte(src), &obj)
		require.NoError(t, err)

		if obj.GetKind() == "Deployment" && obj.GetName() == "test-redis-slave" {
			var dep v1.Deployment
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &dep)
			require.NoError(t, err)
			assert.Equal(t, int32(2), *dep.Spec.Replicas)
			replicasVerified = true
		}
	}
	assert.True(t, replicasVerified)
}

func TestHelmWithMissingValueFiles(t *testing.T) {
	service := newService(t, "../../util/helm/testdata/redis")
	missingValuesFile := "values-prod-overrides.yaml"

	req := &apiclient.ManifestRequest{
		Repo:    &argoappv1.Repository{},
		AppName: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: ".",
			Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles: []string{"values-production.yaml", missingValuesFile},
			},
		},
		ProjectName:        "something",
		ProjectSourceRepos: []string{"*"},
	}

	// Should fail since we're passing a non-existent values file, and error should indicate that
	_, err := service.GenerateManifest(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("%s: no such file or directory", missingValuesFile))

	// Should template without error even if defining a non-existent values file
	req.ApplicationSource.Helm.IgnoreMissingValueFiles = true
	_, err = service.GenerateManifest(context.Background(), req)
	require.NoError(t, err)
}

func TestGenerateHelmWithEnvVars(t *testing.T) {
	service := newService(t, "../../util/helm/testdata/redis")

	res, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:    &argoappv1.Repository{},
		AppName: "production",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: ".",
			Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles: []string{"values-$ARGOCD_APP_NAME.yaml"},
			},
		},
		ProjectName:        "something",
		ProjectSourceRepos: []string{"*"},
	})

	require.NoError(t, err)

	replicasVerified := false
	for _, src := range res.Manifests {
		obj := unstructured.Unstructured{}
		err = json.Unmarshal([]byte(src), &obj)
		require.NoError(t, err)

		if obj.GetKind() == "Deployment" && obj.GetName() == "production-redis-slave" {
			var dep v1.Deployment
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &dep)
			require.NoError(t, err)
			assert.Equal(t, int32(3), *dep.Spec.Replicas)
			replicasVerified = true
		}
	}
	assert.True(t, replicasVerified)
}

// The requested value file (`../minio/values.yaml`) is outside the app path (`./util/helm/testdata/redis`), however
// since the requested value is still under the repo directory (`~/go/src/github.com/argoproj/argo-cd`), it is allowed
func TestGenerateHelmWithValuesDirectoryTraversal(t *testing.T) {
	service := newService(t, "../../util/helm/testdata")
	_, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:    &argoappv1.Repository{},
		AppName: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./redis",
			Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles:   []string{"../minio/values.yaml"},
				ValuesObject: &runtime.RawExtension{Raw: []byte(`cluster: {slaveCount: 2}`)},
			},
		},
		ProjectName:        "something",
		ProjectSourceRepos: []string{"*"},
	})
	require.NoError(t, err)

	// Test the case where the path is "."
	service = newService(t, "./testdata")
	_, err = service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:    &argoappv1.Repository{},
		AppName: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./my-chart",
		},
		ProjectName:        "something",
		ProjectSourceRepos: []string{"*"},
	})
	require.NoError(t, err)
}

func TestChartRepoWithOutOfBoundsSymlink(t *testing.T) {
	service := newService(t, ".")
	source := &argoappv1.ApplicationSource{Chart: "out-of-bounds-chart", TargetRevision: ">= 1.0.0"}
	request := &apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: source, NoCache: true}
	_, err := service.GenerateManifest(context.Background(), request)
	assert.ErrorContains(t, err, "chart contains out-of-bounds symlinks")
}

// This is a Helm first-class app with a values file inside the repo directory
// (`~/go/src/github.com/argoproj/argo-cd/reposerver/repository`), so it is allowed
func TestHelmManifestFromChartRepoWithValueFile(t *testing.T) {
	service := newService(t, ".")
	source := &argoappv1.ApplicationSource{
		Chart:          "my-chart",
		TargetRevision: ">= 1.0.0",
		Helm: &argoappv1.ApplicationSourceHelm{
			ValueFiles: []string{"./my-chart-values.yaml"},
		},
	}
	request := &apiclient.ManifestRequest{
		Repo:               &argoappv1.Repository{},
		ApplicationSource:  source,
		NoCache:            true,
		ProjectName:        "something",
		ProjectSourceRepos: []string{"*"},
	}
	response, err := service.GenerateManifest(context.Background(), request)
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, &apiclient.ManifestResponse{
		Manifests:  []string{"{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"my-map\"}}"},
		Namespace:  "",
		Server:     "",
		Revision:   "1.1.0",
		SourceType: "Helm",
		Commands:   []string{`helm template . --name-template "" --values ./testdata/my-chart/my-chart-values.yaml --include-crds`},
	}, response)
}

// This is a Helm first-class app with a values file outside the repo directory
// (`~/go/src/github.com/argoproj/argo-cd/reposerver/repository`), so it is not allowed
func TestHelmManifestFromChartRepoWithValueFileOutsideRepo(t *testing.T) {
	service := newService(t, ".")
	source := &argoappv1.ApplicationSource{
		Chart:          "my-chart",
		TargetRevision: ">= 1.0.0",
		Helm: &argoappv1.ApplicationSourceHelm{
			ValueFiles: []string{"../my-chart-2/my-chart-2-values.yaml"},
		},
	}
	request := &apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: source, NoCache: true}
	_, err := service.GenerateManifest(context.Background(), request)
	require.Error(t, err)
}

func TestHelmManifestFromChartRepoWithValueFileLinks(t *testing.T) {
	t.Run("Valid symlink", func(t *testing.T) {
		service := newService(t, ".")
		source := &argoappv1.ApplicationSource{
			Chart:          "my-chart",
			TargetRevision: ">= 1.0.0",
			Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles: []string{"my-chart-link.yaml"},
			},
		}
		request := &apiclient.ManifestRequest{
			Repo: &argoappv1.Repository{}, ApplicationSource: source, NoCache: true, ProjectName: "something",
			ProjectSourceRepos: []string{"*"},
		}
		_, err := service.GenerateManifest(context.Background(), request)
		require.NoError(t, err)
	})
}

func TestGenerateHelmWithURL(t *testing.T) {
	service := newService(t, "../../util/helm/testdata/redis")

	_, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:    &argoappv1.Repository{},
		AppName: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: ".",
			Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles:   []string{"https://raw.githubusercontent.com/argoproj/argocd-example-apps/master/helm-guestbook/values.yaml"},
				ValuesObject: &runtime.RawExtension{Raw: []byte(`cluster: {slaveCount: 2}`)},
			},
		},
		ProjectName:        "something",
		ProjectSourceRepos: []string{"*"},
		HelmOptions:        &argoappv1.HelmOptions{ValuesFileSchemes: []string{"https"}},
	})
	require.NoError(t, err)
}

// The requested value file (`../minio/values.yaml`) is outside the repo directory
// (`~/go/src/github.com/argoproj/argo-cd/util/helm/testdata/redis`), so it is blocked
func TestGenerateHelmWithValuesDirectoryTraversalOutsideRepo(t *testing.T) {
	t.Run("Values file with relative path pointing outside repo root", func(t *testing.T) {
		service := newService(t, "../../util/helm/testdata/redis")
		_, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
			Repo:    &argoappv1.Repository{},
			AppName: "test",
			ApplicationSource: &argoappv1.ApplicationSource{
				Path: ".",
				Helm: &argoappv1.ApplicationSourceHelm{
					ValueFiles:   []string{"../minio/values.yaml"},
					ValuesObject: &runtime.RawExtension{Raw: []byte(`cluster: {slaveCount: 2}`)},
				},
			},
			ProjectName:        "something",
			ProjectSourceRepos: []string{"*"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outside repository root")
	})

	t.Run("Values file with relative path pointing inside repo root", func(t *testing.T) {
		service := newService(t, "./testdata")
		_, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
			Repo:    &argoappv1.Repository{},
			AppName: "test",
			ApplicationSource: &argoappv1.ApplicationSource{
				Path: "./my-chart",
				Helm: &argoappv1.ApplicationSourceHelm{
					ValueFiles:   []string{"../my-chart/my-chart-values.yaml"},
					ValuesObject: &runtime.RawExtension{Raw: []byte(`cluster: {slaveCount: 2}`)},
				},
			},
			ProjectName:        "something",
			ProjectSourceRepos: []string{"*"},
		})
		require.NoError(t, err)
	})

	t.Run("Values file with absolute path stays within repo root", func(t *testing.T) {
		service := newService(t, "./testdata")
		_, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
			Repo:    &argoappv1.Repository{},
			AppName: "test",
			ApplicationSource: &argoappv1.ApplicationSource{
				Path: "./my-chart",
				Helm: &argoappv1.ApplicationSourceHelm{
					ValueFiles:   []string{"/my-chart/my-chart-values.yaml"},
					ValuesObject: &runtime.RawExtension{Raw: []byte(`cluster: {slaveCount: 2}`)},
				},
			},
			ProjectName:        "something",
			ProjectSourceRepos: []string{"*"},
		})
		require.NoError(t, err)
	})

	t.Run("Values file with absolute path using back-references outside repo root", func(t *testing.T) {
		service := newService(t, "./testdata")
		_, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
			Repo:    &argoappv1.Repository{},
			AppName: "test",
			ApplicationSource: &argoappv1.ApplicationSource{
				Path: "./my-chart",
				Helm: &argoappv1.ApplicationSourceHelm{
					ValueFiles:   []string{"/../../../my-chart-values.yaml"},
					ValuesObject: &runtime.RawExtension{Raw: []byte(`cluster: {slaveCount: 2}`)},
				},
			},
			ProjectName:        "something",
			ProjectSourceRepos: []string{"*"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outside repository root")
	})

	t.Run("Remote values file from forbidden protocol", func(t *testing.T) {
		service := newService(t, "./testdata")
		_, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
			Repo:    &argoappv1.Repository{},
			AppName: "test",
			ApplicationSource: &argoappv1.ApplicationSource{
				Path: "./my-chart",
				Helm: &argoappv1.ApplicationSourceHelm{
					ValueFiles:   []string{"file://../../../../my-chart-values.yaml"},
					ValuesObject: &runtime.RawExtension{Raw: []byte(`cluster: {slaveCount: 2}`)},
				},
			},
			ProjectName:        "something",
			ProjectSourceRepos: []string{"*"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not allowed")
	})

	t.Run("Remote values file from custom allowed protocol", func(t *testing.T) {
		service := newService(t, "./testdata")
		_, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
			Repo:    &argoappv1.Repository{},
			AppName: "test",
			ApplicationSource: &argoappv1.ApplicationSource{
				Path: "./my-chart",
				Helm: &argoappv1.ApplicationSourceHelm{
					ValueFiles: []string{"s3://my-bucket/my-chart-values.yaml"},
				},
			},
			HelmOptions:        &argoappv1.HelmOptions{ValuesFileSchemes: []string{"s3"}},
			ProjectName:        "something",
			ProjectSourceRepos: []string{"*"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "s3://my-bucket/my-chart-values.yaml: no such file or directory")
	})
}

// File parameter should not allow traversal outside of the repository root
func TestGenerateHelmWithAbsoluteFileParameter(t *testing.T) {
	service := newService(t, "../..")

	file, err := os.CreateTemp("", "external-secret.txt")
	require.NoError(t, err)
	externalSecretPath := file.Name()
	defer func() { _ = os.RemoveAll(externalSecretPath) }()
	expectedFileContent, err := os.ReadFile("../../util/helm/testdata/external/external-secret.txt")
	require.NoError(t, err)
	err = os.WriteFile(externalSecretPath, expectedFileContent, 0o644)
	require.NoError(t, err)
	defer func() {
		if err = file.Close(); err != nil {
			panic(err)
		}
	}()

	_, err = service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:    &argoappv1.Repository{},
		AppName: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./util/helm/testdata/redis",
			Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles:   []string{"values-production.yaml"},
				ValuesObject: &runtime.RawExtension{Raw: []byte(`cluster: {slaveCount: 2}`)},
				FileParameters: []argoappv1.HelmFileParameter{{
					Name: "passwordContent",
					Path: externalSecretPath,
				}},
			},
		},
		ProjectName:        "something",
		ProjectSourceRepos: []string{"*"},
	})
	require.Error(t, err)
}

// The requested file parameter (`../external/external-secret.txt`) is outside the app path
// (`./util/helm/testdata/redis`), however since the requested value is still under the repo
// directory (`~/go/src/github.com/argoproj/argo-cd`), it is allowed. It is used as a means of
// providing direct content to a helm chart via a specific key.
func TestGenerateHelmWithFileParameter(t *testing.T) {
	service := newService(t, "../../util/helm/testdata")

	res, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:    &argoappv1.Repository{},
		AppName: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./redis",
			Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles:   []string{"values-production.yaml"},
				Values:       `cluster: {slaveCount: 10}`,
				ValuesObject: &runtime.RawExtension{Raw: []byte(`cluster: {slaveCount: 2}`)},
				FileParameters: []argoappv1.HelmFileParameter{{
					Name: "passwordContent",
					Path: "../external/external-secret.txt",
				}},
			},
		},
		ProjectName:        "something",
		ProjectSourceRepos: []string{"*"},
	})
	require.NoError(t, err)
	assert.Contains(t, res.Manifests[6], `"replicas":2`, "ValuesObject should override Values")
}

func TestGenerateNullList(t *testing.T) {
	service := newService(t, ".")

	t.Run("null list", func(t *testing.T) {
		res1, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
			Repo:               &argoappv1.Repository{},
			ApplicationSource:  &argoappv1.ApplicationSource{Path: "./testdata/null-list"},
			NoCache:            true,
			ProjectName:        "something",
			ProjectSourceRepos: []string{"*"},
		})
		require.NoError(t, err)
		assert.Len(t, res1.Manifests, 1)
		assert.Contains(t, res1.Manifests[0], "prometheus-operator-operator")
	})

	t.Run("empty list", func(t *testing.T) {
		res1, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
			Repo:               &argoappv1.Repository{},
			ApplicationSource:  &argoappv1.ApplicationSource{Path: "./testdata/empty-list"},
			NoCache:            true,
			ProjectName:        "something",
			ProjectSourceRepos: []string{"*"},
		})
		require.NoError(t, err)
		assert.Len(t, res1.Manifests, 1)
		assert.Contains(t, res1.Manifests[0], "prometheus-operator-operator")
	})

	t.Run("weird list", func(t *testing.T) {
		res1, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
			Repo:               &argoappv1.Repository{},
			ApplicationSource:  &argoappv1.ApplicationSource{Path: "./testdata/weird-list"},
			NoCache:            true,
			ProjectName:        "something",
			ProjectSourceRepos: []string{"*"},
		})
		require.NoError(t, err)
		assert.Len(t, res1.Manifests, 2)
	})
}

func TestIdentifyAppSourceTypeByAppDirWithKustomizations(t *testing.T) {
	sourceType, err := GetAppSourceType(context.Background(), &argoappv1.ApplicationSource{}, "./testdata/kustomization_yaml", "./testdata", "testapp", map[string]bool{}, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, argoappv1.ApplicationSourceTypeKustomize, sourceType)

	sourceType, err = GetAppSourceType(context.Background(), &argoappv1.ApplicationSource{}, "./testdata/kustomization_yml", "./testdata", "testapp", map[string]bool{}, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, argoappv1.ApplicationSourceTypeKustomize, sourceType)

	sourceType, err = GetAppSourceType(context.Background(), &argoappv1.ApplicationSource{}, "./testdata/Kustomization", "./testdata", "testapp", map[string]bool{}, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, argoappv1.ApplicationSourceTypeKustomize, sourceType)
}

func TestGenerateFromUTF16(t *testing.T) {
	q := apiclient.ManifestRequest{
		Repo:               &argoappv1.Repository{},
		ApplicationSource:  &argoappv1.ApplicationSource{},
		ProjectName:        "something",
		ProjectSourceRepos: []string{"*"},
	}
	res1, err := GenerateManifests(context.Background(), "./testdata/utf-16", "/", "", &q, false, &git.NoopCredsStore{}, resource.MustParse("0"), nil)
	require.NoError(t, err)
	assert.Len(t, res1.Manifests, 2)
}

func TestListApps(t *testing.T) {
	service := newService(t, "./testdata")

	res, err := service.ListApps(context.Background(), &apiclient.ListAppsRequest{Repo: &argoappv1.Repository{}})
	require.NoError(t, err)

	expectedApps := map[string]string{
		"Kustomization":                     "Kustomize",
		"app-parameters/multi":              "Kustomize",
		"app-parameters/single-app-only":    "Kustomize",
		"app-parameters/single-global":      "Kustomize",
		"app-parameters/single-global-helm": "Helm",
		"in-bounds-values-file-link":        "Helm",
		"invalid-helm":                      "Helm",
		"invalid-kustomize":                 "Kustomize",
		"kustomization_yaml":                "Kustomize",
		"kustomization_yml":                 "Kustomize",
		"my-chart":                          "Helm",
		"my-chart-2":                        "Helm",
		"oci-dependencies":                  "Helm",
		"out-of-bounds-values-file-link":    "Helm",
		"values-files":                      "Helm",
		"helm-with-dependencies":            "Helm",
		"helm-with-dependencies-alias":      "Helm",
		"helm-with-local-dependency":        "Helm",
		"simple-chart":                      "Helm",
	}
	assert.Equal(t, expectedApps, res.Apps)
}

func TestGetAppDetailsHelm(t *testing.T) {
	service := newService(t, "../../util/helm/testdata/dependency")

	res, err := service.GetAppDetails(context.Background(), &apiclient.RepoServerAppDetailsQuery{
		Repo: &argoappv1.Repository{},
		Source: &argoappv1.ApplicationSource{
			Path: ".",
		},
	})

	require.NoError(t, err)
	assert.NotNil(t, res.Helm)

	assert.Equal(t, "Helm", res.Type)
	assert.EqualValues(t, []string{"values-production.yaml", "values.yaml"}, res.Helm.ValueFiles)
}

func TestGetAppDetailsHelmUsesCache(t *testing.T) {
	service := newService(t, "../../util/helm/testdata/dependency")

	res, err := service.GetAppDetails(context.Background(), &apiclient.RepoServerAppDetailsQuery{
		Repo: &argoappv1.Repository{},
		Source: &argoappv1.ApplicationSource{
			Path: ".",
		},
	})

	require.NoError(t, err)
	assert.NotNil(t, res.Helm)

	assert.Equal(t, "Helm", res.Type)
	assert.EqualValues(t, []string{"values-production.yaml", "values.yaml"}, res.Helm.ValueFiles)
}

func TestGetAppDetailsHelm_WithNoValuesFile(t *testing.T) {
	service := newService(t, "../../util/helm/testdata/api-versions")

	res, err := service.GetAppDetails(context.Background(), &apiclient.RepoServerAppDetailsQuery{
		Repo: &argoappv1.Repository{},
		Source: &argoappv1.ApplicationSource{
			Path: ".",
		},
	})

	require.NoError(t, err)
	assert.NotNil(t, res.Helm)

	assert.Equal(t, "Helm", res.Type)
	assert.Empty(t, res.Helm.ValueFiles)
	assert.Equal(t, "", res.Helm.Values)
}

func TestGetAppDetailsKustomize(t *testing.T) {
	service := newService(t, "../../util/kustomize/testdata/kustomization_yaml")

	res, err := service.GetAppDetails(context.Background(), &apiclient.RepoServerAppDetailsQuery{
		Repo: &argoappv1.Repository{},
		Source: &argoappv1.ApplicationSource{
			Path: ".",
		},
	})

	require.NoError(t, err)

	assert.Equal(t, "Kustomize", res.Type)
	assert.NotNil(t, res.Kustomize)
	assert.EqualValues(t, []string{"nginx:1.15.4", "registry.k8s.io/nginx-slim:0.8"}, res.Kustomize.Images)
}

func TestGetHelmCharts(t *testing.T) {
	service := newService(t, "../..")
	res, err := service.GetHelmCharts(context.Background(), &apiclient.HelmChartsRequest{Repo: &argoappv1.Repository{}})

	// fix flakiness
	sort.Slice(res.Items, func(i, j int) bool {
		return res.Items[i].Name < res.Items[j].Name
	})

	require.NoError(t, err)
	assert.Len(t, res.Items, 2)

	item := res.Items[0]
	assert.Equal(t, "my-chart", item.Name)
	assert.EqualValues(t, []string{"1.0.0", "1.1.0"}, item.Versions)

	item2 := res.Items[1]
	assert.Equal(t, "out-of-bounds-chart", item2.Name)
	assert.EqualValues(t, []string{"1.0.0", "1.1.0"}, item2.Versions)
}

func TestGetRevisionMetadata(t *testing.T) {
	service, gitClient, _ := newServiceWithMocks(t, "../..", false)
	now := time.Now()

	gitClient.On("RevisionMetadata", mock.Anything).Return(&git.RevisionMetadata{
		Message: "test",
		Author:  "author",
		Date:    now,
		Tags:    []string{"tag1", "tag2"},
	}, nil)

	res, err := service.GetRevisionMetadata(context.Background(), &apiclient.RepoServerRevisionMetadataRequest{
		Repo:           &argoappv1.Repository{},
		Revision:       "c0b400fc458875d925171398f9ba9eabd5529923",
		CheckSignature: true,
	})

	require.NoError(t, err)
	assert.Equal(t, "test", res.Message)
	assert.Equal(t, now, res.Date.Time)
	assert.Equal(t, "author", res.Author)
	assert.EqualValues(t, []string{"tag1", "tag2"}, res.Tags)
	assert.NotEmpty(t, res.SignatureInfo)

	// Check for truncated revision value
	res, err = service.GetRevisionMetadata(context.Background(), &apiclient.RepoServerRevisionMetadataRequest{
		Repo:           &argoappv1.Repository{},
		Revision:       "c0b400f",
		CheckSignature: true,
	})

	require.NoError(t, err)
	assert.Equal(t, "test", res.Message)
	assert.Equal(t, now, res.Date.Time)
	assert.Equal(t, "author", res.Author)
	assert.EqualValues(t, []string{"tag1", "tag2"}, res.Tags)
	assert.NotEmpty(t, res.SignatureInfo)

	// Cache hit - signature info should not be in result
	res, err = service.GetRevisionMetadata(context.Background(), &apiclient.RepoServerRevisionMetadataRequest{
		Repo:           &argoappv1.Repository{},
		Revision:       "c0b400fc458875d925171398f9ba9eabd5529923",
		CheckSignature: false,
	})
	require.NoError(t, err)
	assert.Empty(t, res.SignatureInfo)

	// Enforce cache miss - signature info should not be in result
	res, err = service.GetRevisionMetadata(context.Background(), &apiclient.RepoServerRevisionMetadataRequest{
		Repo:           &argoappv1.Repository{},
		Revision:       "da52afd3b2df1ec49470603d8bbb46954dab1091",
		CheckSignature: false,
	})
	require.NoError(t, err)
	assert.Empty(t, res.SignatureInfo)

	// Cache hit on previous entry that did not have signature info
	res, err = service.GetRevisionMetadata(context.Background(), &apiclient.RepoServerRevisionMetadataRequest{
		Repo:           &argoappv1.Repository{},
		Revision:       "da52afd3b2df1ec49470603d8bbb46954dab1091",
		CheckSignature: true,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, res.SignatureInfo)
}

func TestGetSignatureVerificationResult(t *testing.T) {
	// Commit with signature and verification requested
	{
		service := newServiceWithSignature(t, "../../manifests/base")

		src := argoappv1.ApplicationSource{Path: "."}
		q := apiclient.ManifestRequest{
			Repo:               &argoappv1.Repository{},
			ApplicationSource:  &src,
			VerifySignature:    true,
			ProjectName:        "something",
			ProjectSourceRepos: []string{"*"},
		}

		res, err := service.GenerateManifest(context.Background(), &q)
		require.NoError(t, err)
		assert.Equal(t, testSignature, res.VerifyResult)
	}
	// Commit with signature and verification not requested
	{
		service := newServiceWithSignature(t, "../../manifests/base")

		src := argoappv1.ApplicationSource{Path: "."}
		q := apiclient.ManifestRequest{
			Repo: &argoappv1.Repository{}, ApplicationSource: &src, ProjectName: "something",
			ProjectSourceRepos: []string{"*"},
		}

		res, err := service.GenerateManifest(context.Background(), &q)
		require.NoError(t, err)
		assert.Empty(t, res.VerifyResult)
	}
	// Commit without signature and verification requested
	{
		service := newService(t, "../../manifests/base")

		src := argoappv1.ApplicationSource{Path: "."}
		q := apiclient.ManifestRequest{
			Repo: &argoappv1.Repository{}, ApplicationSource: &src, VerifySignature: true, ProjectName: "something",
			ProjectSourceRepos: []string{"*"},
		}

		res, err := service.GenerateManifest(context.Background(), &q)
		require.NoError(t, err)
		assert.Empty(t, res.VerifyResult)
	}
	// Commit without signature and verification not requested
	{
		service := newService(t, "../../manifests/base")

		src := argoappv1.ApplicationSource{Path: "."}
		q := apiclient.ManifestRequest{
			Repo: &argoappv1.Repository{}, ApplicationSource: &src, VerifySignature: true, ProjectName: "something",
			ProjectSourceRepos: []string{"*"},
		}

		res, err := service.GenerateManifest(context.Background(), &q)
		require.NoError(t, err)
		assert.Empty(t, res.VerifyResult)
	}
}

func Test_newEnv(t *testing.T) {
	assert.Equal(t, &argoappv1.Env{
		&argoappv1.EnvEntry{Name: "ARGOCD_APP_NAME", Value: "my-app-name"},
		&argoappv1.EnvEntry{Name: "ARGOCD_APP_NAMESPACE", Value: "my-namespace"},
		&argoappv1.EnvEntry{Name: "ARGOCD_APP_REVISION", Value: "my-revision"},
		&argoappv1.EnvEntry{Name: "ARGOCD_APP_REVISION_SHORT", Value: "my-revi"},
		&argoappv1.EnvEntry{Name: "ARGOCD_APP_REVISION_SHORT_8", Value: "my-revis"},
		&argoappv1.EnvEntry{Name: "ARGOCD_APP_SOURCE_REPO_URL", Value: "https://github.com/my-org/my-repo"},
		&argoappv1.EnvEntry{Name: "ARGOCD_APP_SOURCE_PATH", Value: "my-path"},
		&argoappv1.EnvEntry{Name: "ARGOCD_APP_SOURCE_TARGET_REVISION", Value: "my-target-revision"},
	}, newEnv(&apiclient.ManifestRequest{
		AppName:   "my-app-name",
		Namespace: "my-namespace",
		Repo:      &argoappv1.Repository{Repo: "https://github.com/my-org/my-repo"},
		ApplicationSource: &argoappv1.ApplicationSource{
			Path:           "my-path",
			TargetRevision: "my-target-revision",
		},
	}, "my-revision"))
}

func TestService_newHelmClientResolveRevision(t *testing.T) {
	service := newService(t, ".")

	t.Run("EmptyRevision", func(t *testing.T) {
		_, _, err := service.newHelmClientResolveRevision(&argoappv1.Repository{}, "", "", true)
		assert.EqualError(t, err, "invalid revision '': improper constraint: ")
	})
	t.Run("InvalidRevision", func(t *testing.T) {
		_, _, err := service.newHelmClientResolveRevision(&argoappv1.Repository{}, "???", "", true)
		assert.EqualError(t, err, "invalid revision '???': improper constraint: ???", true)
	})
}

func TestGetAppDetailsWithAppParameterFile(t *testing.T) {
	t.Run("No app name set and app specific file exists", func(t *testing.T) {
		service := newService(t, ".")
		runWithTempTestdata(t, "multi", func(t *testing.T, path string) {
			details, err := service.GetAppDetails(context.Background(), &apiclient.RepoServerAppDetailsQuery{
				Repo: &argoappv1.Repository{},
				Source: &argoappv1.ApplicationSource{
					Path: path,
				},
			})
			require.NoError(t, err)
			assert.EqualValues(t, []string{"gcr.io/heptio-images/ks-guestbook-demo:0.2"}, details.Kustomize.Images)
		})
	})
	t.Run("No app specific override", func(t *testing.T) {
		service := newService(t, ".")
		runWithTempTestdata(t, "single-global", func(t *testing.T, path string) {
			details, err := service.GetAppDetails(context.Background(), &apiclient.RepoServerAppDetailsQuery{
				Repo: &argoappv1.Repository{},
				Source: &argoappv1.ApplicationSource{
					Path: path,
				},
				AppName: "testapp",
			})
			require.NoError(t, err)
			assert.EqualValues(t, []string{"gcr.io/heptio-images/ks-guestbook-demo:0.2"}, details.Kustomize.Images)
		})
	})
	t.Run("Only app specific override", func(t *testing.T) {
		service := newService(t, ".")
		runWithTempTestdata(t, "single-app-only", func(t *testing.T, path string) {
			details, err := service.GetAppDetails(context.Background(), &apiclient.RepoServerAppDetailsQuery{
				Repo: &argoappv1.Repository{},
				Source: &argoappv1.ApplicationSource{
					Path: path,
				},
				AppName: "testapp",
			})
			require.NoError(t, err)
			assert.EqualValues(t, []string{"gcr.io/heptio-images/ks-guestbook-demo:0.3"}, details.Kustomize.Images)
		})
	})
	t.Run("App specific override", func(t *testing.T) {
		service := newService(t, ".")
		runWithTempTestdata(t, "multi", func(t *testing.T, path string) {
			details, err := service.GetAppDetails(context.Background(), &apiclient.RepoServerAppDetailsQuery{
				Repo: &argoappv1.Repository{},
				Source: &argoappv1.ApplicationSource{
					Path: path,
				},
				AppName: "testapp",
			})
			require.NoError(t, err)
			assert.EqualValues(t, []string{"gcr.io/heptio-images/ks-guestbook-demo:0.3"}, details.Kustomize.Images)
		})
	})
	t.Run("App specific overrides containing non-mergeable field", func(t *testing.T) {
		service := newService(t, ".")
		runWithTempTestdata(t, "multi", func(t *testing.T, path string) {
			details, err := service.GetAppDetails(context.Background(), &apiclient.RepoServerAppDetailsQuery{
				Repo: &argoappv1.Repository{},
				Source: &argoappv1.ApplicationSource{
					Path: path,
				},
				AppName: "unmergeable",
			})
			require.NoError(t, err)
			assert.EqualValues(t, []string{"gcr.io/heptio-images/ks-guestbook-demo:0.3"}, details.Kustomize.Images)
		})
	})
	t.Run("Broken app-specific overrides", func(t *testing.T) {
		service := newService(t, ".")
		runWithTempTestdata(t, "multi", func(t *testing.T, path string) {
			_, err := service.GetAppDetails(context.Background(), &apiclient.RepoServerAppDetailsQuery{
				Repo: &argoappv1.Repository{},
				Source: &argoappv1.ApplicationSource{
					Path: path,
				},
				AppName: "broken",
			})
			require.Error(t, err)
		})
	})
}

// There are unit test that will use kustomize set and by that modify the
// kustomization.yaml. For proper testing, we need to copy the testdata to a
// temporary path, run the tests, and then throw the copy away again.
func mkTempParameters(source string) string {
	tempDir, err := os.MkdirTemp("./testdata", "app-parameters")
	if err != nil {
		panic(err)
	}
	cmd := exec.Command("cp", "-R", source, tempDir)
	err = cmd.Run()
	if err != nil {
		os.RemoveAll(tempDir)
		panic(err)
	}
	return tempDir
}

// Simple wrapper run a test with a temporary copy of the testdata, because
// the test would modify the data when run.
func runWithTempTestdata(t *testing.T, path string, runner func(t *testing.T, path string)) {
	tempDir := mkTempParameters("./testdata/app-parameters")
	runner(t, filepath.Join(tempDir, "app-parameters", path))
	os.RemoveAll(tempDir)
}

func TestGenerateManifestsWithAppParameterFile(t *testing.T) {
	t.Run("Single global override", func(t *testing.T) {
		runWithTempTestdata(t, "single-global", func(t *testing.T, path string) {
			service := newService(t, ".")
			manifests, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
				Repo: &argoappv1.Repository{},
				ApplicationSource: &argoappv1.ApplicationSource{
					Path: path,
				},
				ProjectName:        "something",
				ProjectSourceRepos: []string{"*"},
			})
			require.NoError(t, err)
			resourceByKindName := make(map[string]*unstructured.Unstructured)
			for _, manifest := range manifests.Manifests {
				var un unstructured.Unstructured
				err := yaml.Unmarshal([]byte(manifest), &un)
				require.NoError(t, err)
				resourceByKindName[fmt.Sprintf("%s/%s", un.GetKind(), un.GetName())] = &un
			}
			deployment, ok := resourceByKindName["Deployment/guestbook-ui"]
			require.True(t, ok)
			containers, ok, _ := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
			require.True(t, ok)
			image, ok, _ := unstructured.NestedString(containers[0].(map[string]interface{}), "image")
			require.True(t, ok)
			assert.Equal(t, "gcr.io/heptio-images/ks-guestbook-demo:0.2", image)
		})
	})

	t.Run("Single global override Helm", func(t *testing.T) {
		runWithTempTestdata(t, "single-global-helm", func(t *testing.T, path string) {
			service := newService(t, ".")
			manifests, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
				Repo: &argoappv1.Repository{},
				ApplicationSource: &argoappv1.ApplicationSource{
					Path: path,
				},
				ProjectName:        "something",
				ProjectSourceRepos: []string{"*"},
			})
			require.NoError(t, err)
			resourceByKindName := make(map[string]*unstructured.Unstructured)
			for _, manifest := range manifests.Manifests {
				var un unstructured.Unstructured
				err := yaml.Unmarshal([]byte(manifest), &un)
				require.NoError(t, err)
				resourceByKindName[fmt.Sprintf("%s/%s", un.GetKind(), un.GetName())] = &un
			}
			deployment, ok := resourceByKindName["Deployment/guestbook-ui"]
			require.True(t, ok)
			containers, ok, _ := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
			require.True(t, ok)
			image, ok, _ := unstructured.NestedString(containers[0].(map[string]interface{}), "image")
			require.True(t, ok)
			assert.Equal(t, "gcr.io/heptio-images/ks-guestbook-demo:0.2", image)
		})
	})

	t.Run("Application specific override", func(t *testing.T) {
		service := newService(t, ".")
		runWithTempTestdata(t, "single-app-only", func(t *testing.T, path string) {
			manifests, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
				Repo: &argoappv1.Repository{},
				ApplicationSource: &argoappv1.ApplicationSource{
					Path: path,
				},
				AppName:            "testapp",
				ProjectName:        "something",
				ProjectSourceRepos: []string{"*"},
			})
			require.NoError(t, err)
			resourceByKindName := make(map[string]*unstructured.Unstructured)
			for _, manifest := range manifests.Manifests {
				var un unstructured.Unstructured
				err := yaml.Unmarshal([]byte(manifest), &un)
				require.NoError(t, err)
				resourceByKindName[fmt.Sprintf("%s/%s", un.GetKind(), un.GetName())] = &un
			}
			deployment, ok := resourceByKindName["Deployment/guestbook-ui"]
			require.True(t, ok)
			containers, ok, _ := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
			require.True(t, ok)
			image, ok, _ := unstructured.NestedString(containers[0].(map[string]interface{}), "image")
			require.True(t, ok)
			assert.Equal(t, "gcr.io/heptio-images/ks-guestbook-demo:0.3", image)
		})
	})

	t.Run("Multi-source with source as ref only does not generate manifests", func(t *testing.T) {
		service := newService(t, ".")
		runWithTempTestdata(t, "single-app-only", func(t *testing.T, path string) {
			manifests, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
				Repo: &argoappv1.Repository{},
				ApplicationSource: &argoappv1.ApplicationSource{
					Path:  "",
					Chart: "",
					Ref:   "test",
				},
				AppName:            "testapp-multi-ref-only",
				ProjectName:        "something",
				ProjectSourceRepos: []string{"*"},
				HasMultipleSources: true,
			})
			require.NoError(t, err)
			assert.Empty(t, manifests.Manifests)
			assert.NotEmpty(t, manifests.Revision)
		})
	})

	t.Run("Application specific override for other app", func(t *testing.T) {
		service := newService(t, ".")
		runWithTempTestdata(t, "single-app-only", func(t *testing.T, path string) {
			manifests, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
				Repo: &argoappv1.Repository{},
				ApplicationSource: &argoappv1.ApplicationSource{
					Path: path,
				},
				AppName:            "testapp2",
				ProjectName:        "something",
				ProjectSourceRepos: []string{"*"},
			})
			require.NoError(t, err)
			resourceByKindName := make(map[string]*unstructured.Unstructured)
			for _, manifest := range manifests.Manifests {
				var un unstructured.Unstructured
				err := yaml.Unmarshal([]byte(manifest), &un)
				require.NoError(t, err)
				resourceByKindName[fmt.Sprintf("%s/%s", un.GetKind(), un.GetName())] = &un
			}
			deployment, ok := resourceByKindName["Deployment/guestbook-ui"]
			require.True(t, ok)
			containers, ok, _ := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
			require.True(t, ok)
			image, ok, _ := unstructured.NestedString(containers[0].(map[string]interface{}), "image")
			require.True(t, ok)
			assert.Equal(t, "gcr.io/heptio-images/ks-guestbook-demo:0.1", image)
		})
	})

	t.Run("Override info does not appear in cache key", func(t *testing.T) {
		service := newService(t, ".")
		runWithTempTestdata(t, "single-global", func(t *testing.T, path string) {
			source := &argoappv1.ApplicationSource{
				Path: path,
			}
			sourceCopy := source.DeepCopy() // make a copy in case GenerateManifest mutates it.
			_, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
				Repo:               &argoappv1.Repository{},
				ApplicationSource:  sourceCopy,
				AppName:            "test",
				ProjectName:        "something",
				ProjectSourceRepos: []string{"*"},
			})
			require.NoError(t, err)
			res := &cache.CachedManifestResponse{}
			// Try to pull from the cache with a `source` that does not include any overrides. Overrides should not be
			// part of the cache key, because you can't get the overrides without a repo operation. And avoiding repo
			// operations is the point of the cache.
			err = service.cache.GetManifests(mock.Anything, source, argoappv1.RefTargetRevisionMapping{}, &argoappv1.ClusterInfo{}, "", "", "", "test", res, nil)
			require.NoError(t, err)
		})
	})
}

func TestGenerateManifestWithAnnotatedAndRegularGitTagHashes(t *testing.T) {
	regularGitTagHash := "632039659e542ed7de0c170a4fcc1c571b288fc0"
	annotatedGitTaghash := "95249be61b028d566c29d47b19e65c5603388a41"
	invalidGitTaghash := "invalid-tag"
	actualCommitSHA := "632039659e542ed7de0c170a4fcc1c571b288fc0"

	tests := []struct {
		name            string
		ctx             context.Context
		manifestRequest *apiclient.ManifestRequest
		wantError       bool
		service         *Service
	}{
		{
			name: "Case: Git tag hash matches latest commit SHA (regular tag)",
			ctx:  context.Background(),
			manifestRequest: &apiclient.ManifestRequest{
				Repo: &argoappv1.Repository{},
				ApplicationSource: &argoappv1.ApplicationSource{
					TargetRevision: regularGitTagHash,
				},
				NoCache:            true,
				ProjectName:        "something",
				ProjectSourceRepos: []string{"*"},
			},
			wantError: false,
			service:   newServiceWithCommitSHA(t, ".", regularGitTagHash),
		},

		{
			name: "Case: Git tag hash does not match latest commit SHA (annotated tag)",
			ctx:  context.Background(),
			manifestRequest: &apiclient.ManifestRequest{
				Repo: &argoappv1.Repository{},
				ApplicationSource: &argoappv1.ApplicationSource{
					TargetRevision: annotatedGitTaghash,
				},
				NoCache:            true,
				ProjectName:        "something",
				ProjectSourceRepos: []string{"*"},
			},
			wantError: false,
			service:   newServiceWithCommitSHA(t, ".", annotatedGitTaghash),
		},

		{
			name: "Case: Git tag hash is invalid",
			ctx:  context.Background(),
			manifestRequest: &apiclient.ManifestRequest{
				Repo: &argoappv1.Repository{},
				ApplicationSource: &argoappv1.ApplicationSource{
					TargetRevision: invalidGitTaghash,
				},
				NoCache:            true,
				ProjectName:        "something",
				ProjectSourceRepos: []string{"*"},
			},
			wantError: true,
			service:   newServiceWithCommitSHA(t, ".", invalidGitTaghash),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifestResponse, err := tt.service.GenerateManifest(tt.ctx, tt.manifestRequest)
			if !tt.wantError {
				if err == nil {
					assert.Equal(t, manifestResponse.Revision, actualCommitSHA)
				} else {
					t.Errorf("unexpected error")
				}
			} else {
				if err == nil {
					t.Errorf("expected an error but did not throw one")
				}
			}
		})
	}
}

func TestGenerateManifestWithAnnotatedTagsAndMultiSourceApp(t *testing.T) {
	annotatedGitTaghash := "95249be61b028d566c29d47b19e65c5603388a41"

	service := newServiceWithCommitSHA(t, ".", annotatedGitTaghash)

	refSources := map[string]*argoappv1.RefTarget{}

	refSources["$global"] = &argoappv1.RefTarget{
		TargetRevision: annotatedGitTaghash,
	}

	refSources["$default"] = &argoappv1.RefTarget{
		TargetRevision: annotatedGitTaghash,
	}

	manifestRequest := &apiclient.ManifestRequest{
		Repo: &argoappv1.Repository{},
		ApplicationSource: &argoappv1.ApplicationSource{
			TargetRevision: annotatedGitTaghash,
			Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles: []string{"$global/values.yaml", "$default/secrets.yaml"},
			},
		},
		HasMultipleSources: true,
		NoCache:            true,
		RefSources:         refSources,
	}

	response, err := service.GenerateManifest(context.Background(), manifestRequest)
	if err != nil {
		t.Errorf("unexpected %s", err)
	}

	if response.Revision != annotatedGitTaghash {
		t.Errorf("returned SHA %s is different from expected annotated tag %s", response.Revision, annotatedGitTaghash)
	}
}

func TestGenerateMultiSourceHelmWithFileParameter(t *testing.T) {
	expectedFileContent, err := os.ReadFile("../../util/helm/testdata/external/external-secret.txt")
	require.NoError(t, err)

	service := newService(t, "../../util/helm/testdata")

	testCases := []struct {
		name            string
		refSources      map[string]*argoappv1.RefTarget
		expectedContent string
		expectedErr     bool
	}{{
		name: "Successfully resolve multi-source ref for helm set-file",
		refSources: map[string]*argoappv1.RefTarget{
			"$global": {
				TargetRevision: "HEAD",
			},
		},
		expectedContent: string(expectedFileContent),
		expectedErr:     false,
	}, {
		name:            "Failed to resolve multi-source ref for helm set-file",
		refSources:      map[string]*argoappv1.RefTarget{},
		expectedContent: "DOES-NOT-EXIST",
		expectedErr:     true,
	}}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			manifestRequest := &apiclient.ManifestRequest{
				Repo: &argoappv1.Repository{},
				ApplicationSource: &argoappv1.ApplicationSource{
					Ref:            "$global",
					Path:           "./redis",
					TargetRevision: "HEAD",
					Helm: &argoappv1.ApplicationSourceHelm{
						ValueFiles: []string{"$global/redis/values-production.yaml"},
						FileParameters: []argoappv1.HelmFileParameter{{
							Name: "passwordContent",
							Path: "$global/external/external-secret.txt",
						}},
					},
				},
				HasMultipleSources: true,
				NoCache:            true,
				RefSources:         tc.refSources,
			}

			res, err := service.GenerateManifest(context.Background(), manifestRequest)

			if !tc.expectedErr {
				require.NoError(t, err)

				// Check that any of the manifests contains the secret
				idx := slices.IndexFunc(res.Manifests, func(content string) bool {
					return strings.Contains(content, tc.expectedContent)
				})
				assert.GreaterOrEqual(t, idx, 0, "No manifest contains the value set with the helm fileParameters")
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestFindResources(t *testing.T) {
	testCases := []struct {
		name          string
		include       string
		exclude       string
		expectedNames []string
	}{{
		name:          "Include One Match",
		include:       "subdir/deploymentSub.yaml",
		expectedNames: []string{"nginx-deployment-sub"},
	}, {
		name:          "Include Everything",
		include:       "*.yaml",
		expectedNames: []string{"nginx-deployment", "nginx-deployment-sub"},
	}, {
		name:          "Include Subdirectory",
		include:       "**/*.yaml",
		expectedNames: []string{"nginx-deployment-sub"},
	}, {
		name:          "Include No Matches",
		include:       "nothing.yaml",
		expectedNames: []string{},
	}, {
		name:          "Exclude - One Match",
		exclude:       "subdir/deploymentSub.yaml",
		expectedNames: []string{"nginx-deployment"},
	}, {
		name:          "Exclude - Everything",
		exclude:       "*.yaml",
		expectedNames: []string{},
	}}
	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			objs, err := findManifests(&log.Entry{}, "testdata/app-include-exclude", ".", nil, argoappv1.ApplicationSourceDirectory{
				Recurse: true,
				Include: tc.include,
				Exclude: tc.exclude,
			}, map[string]bool{}, resource.MustParse("0"))
			require.NoError(t, err)
			var names []string
			for i := range objs {
				names = append(names, objs[i].GetName())
			}
			assert.ElementsMatch(t, tc.expectedNames, names)
		})
	}
}

func TestFindManifests_Exclude(t *testing.T) {
	objs, err := findManifests(&log.Entry{}, "testdata/app-include-exclude", ".", nil, argoappv1.ApplicationSourceDirectory{
		Recurse: true,
		Exclude: "subdir/deploymentSub.yaml",
	}, map[string]bool{}, resource.MustParse("0"))

	require.NoError(t, err)
	require.Len(t, objs, 1)

	assert.Equal(t, "nginx-deployment", objs[0].GetName())
}

func TestFindManifests_Exclude_NothingMatches(t *testing.T) {
	objs, err := findManifests(&log.Entry{}, "testdata/app-include-exclude", ".", nil, argoappv1.ApplicationSourceDirectory{
		Recurse: true,
		Exclude: "nothing.yaml",
	}, map[string]bool{}, resource.MustParse("0"))

	require.NoError(t, err)
	require.Len(t, objs, 2)

	assert.ElementsMatch(t,
		[]string{"nginx-deployment", "nginx-deployment-sub"}, []string{objs[0].GetName(), objs[1].GetName()})
}

func tempDir(t *testing.T) string {
	dir, err := os.MkdirTemp(".", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		err = os.RemoveAll(dir)
		if err != nil {
			panic(err)
		}
	})
	absDir, err := filepath.Abs(dir)
	require.NoError(t, err)
	return absDir
}

func walkFor(t *testing.T, root string, testPath string, run func(info fs.FileInfo)) {
	hitExpectedPath := false
	err := filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if path == testPath {
			require.NoError(t, err)
			hitExpectedPath = true
			run(info)
		}
		return nil
	})
	require.NoError(t, err)
	assert.True(t, hitExpectedPath, "did not hit expected path when walking directory")
}

func Test_getPotentiallyValidManifestFile(t *testing.T) {
	// These tests use filepath.Walk instead of os.Stat to get file info, because FileInfo from os.Stat does not return
	// true for IsSymlink like os.Walk does.

	// These tests do not use t.TempDir() because those directories can contain symlinks which cause test to fail
	// InBound checks.

	t.Run("non-JSON/YAML is skipped with an empty ignore message", func(t *testing.T) {
		appDir := tempDir(t)
		filePath := filepath.Join(appDir, "not-json-or-yaml")
		file, err := os.OpenFile(filePath, os.O_RDONLY|os.O_CREATE, 0o644)
		require.NoError(t, err)
		err = file.Close()
		require.NoError(t, err)

		walkFor(t, appDir, filePath, func(info fs.FileInfo) {
			realFileInfo, ignoreMessage, err := getPotentiallyValidManifestFile(filePath, info, appDir, appDir, "", "")
			assert.Nil(t, realFileInfo)
			assert.Empty(t, ignoreMessage)
			require.NoError(t, err)
		})
	})

	t.Run("circular link should throw an error", func(t *testing.T) {
		appDir := tempDir(t)

		aPath := filepath.Join(appDir, "a.json")
		bPath := filepath.Join(appDir, "b.json")
		err := os.Symlink(bPath, aPath)
		require.NoError(t, err)
		err = os.Symlink(aPath, bPath)
		require.NoError(t, err)

		walkFor(t, appDir, aPath, func(info fs.FileInfo) {
			realFileInfo, ignoreMessage, err := getPotentiallyValidManifestFile(aPath, info, appDir, appDir, "", "")
			assert.Nil(t, realFileInfo)
			assert.Empty(t, ignoreMessage)
			assert.ErrorContains(t, err, "too many links")
		})
	})

	t.Run("symlink with missing destination should throw an error", func(t *testing.T) {
		appDir := tempDir(t)

		aPath := filepath.Join(appDir, "a.json")
		bPath := filepath.Join(appDir, "b.json")
		err := os.Symlink(bPath, aPath)
		require.NoError(t, err)

		walkFor(t, appDir, aPath, func(info fs.FileInfo) {
			realFileInfo, ignoreMessage, err := getPotentiallyValidManifestFile(aPath, info, appDir, appDir, "", "")
			assert.Nil(t, realFileInfo)
			assert.NotEmpty(t, ignoreMessage)
			require.NoError(t, err)
		})
	})

	t.Run("out-of-bounds symlink should throw an error", func(t *testing.T) {
		appDir := tempDir(t)

		linkPath := filepath.Join(appDir, "a.json")
		err := os.Symlink("..", linkPath)
		require.NoError(t, err)

		walkFor(t, appDir, linkPath, func(info fs.FileInfo) {
			realFileInfo, ignoreMessage, err := getPotentiallyValidManifestFile(linkPath, info, appDir, appDir, "", "")
			assert.Nil(t, realFileInfo)
			assert.Empty(t, ignoreMessage)
			assert.ErrorContains(t, err, "illegal filepath in symlink")
		})
	})

	t.Run("symlink to a non-regular file should be skipped with warning", func(t *testing.T) {
		appDir := tempDir(t)

		dirPath := filepath.Join(appDir, "test.dir")
		err := os.MkdirAll(dirPath, 0o644)
		require.NoError(t, err)
		linkPath := filepath.Join(appDir, "test.json")
		err = os.Symlink(dirPath, linkPath)
		require.NoError(t, err)

		walkFor(t, appDir, linkPath, func(info fs.FileInfo) {
			realFileInfo, ignoreMessage, err := getPotentiallyValidManifestFile(linkPath, info, appDir, appDir, "", "")
			assert.Nil(t, realFileInfo)
			assert.Contains(t, ignoreMessage, "non-regular file")
			require.NoError(t, err)
		})
	})

	t.Run("non-included file should be skipped with no message", func(t *testing.T) {
		appDir := tempDir(t)

		filePath := filepath.Join(appDir, "not-included.yaml")
		file, err := os.OpenFile(filePath, os.O_RDONLY|os.O_CREATE, 0o644)
		require.NoError(t, err)
		err = file.Close()
		require.NoError(t, err)

		walkFor(t, appDir, filePath, func(info fs.FileInfo) {
			realFileInfo, ignoreMessage, err := getPotentiallyValidManifestFile(filePath, info, appDir, appDir, "*.json", "")
			assert.Nil(t, realFileInfo)
			assert.Empty(t, ignoreMessage)
			require.NoError(t, err)
		})
	})

	t.Run("excluded file should be skipped with no message", func(t *testing.T) {
		appDir := tempDir(t)

		filePath := filepath.Join(appDir, "excluded.json")
		file, err := os.OpenFile(filePath, os.O_RDONLY|os.O_CREATE, 0o644)
		require.NoError(t, err)
		err = file.Close()
		require.NoError(t, err)

		walkFor(t, appDir, filePath, func(info fs.FileInfo) {
			realFileInfo, ignoreMessage, err := getPotentiallyValidManifestFile(filePath, info, appDir, appDir, "", "excluded.*")
			assert.Nil(t, realFileInfo)
			assert.Empty(t, ignoreMessage)
			require.NoError(t, err)
		})
	})

	t.Run("symlink to a regular file is potentially valid", func(t *testing.T) {
		appDir := tempDir(t)

		filePath := filepath.Join(appDir, "regular-file")
		file, err := os.OpenFile(filePath, os.O_RDONLY|os.O_CREATE, 0o644)
		require.NoError(t, err)
		err = file.Close()
		require.NoError(t, err)

		linkPath := filepath.Join(appDir, "link.json")
		err = os.Symlink(filePath, linkPath)
		require.NoError(t, err)

		walkFor(t, appDir, linkPath, func(info fs.FileInfo) {
			realFileInfo, ignoreMessage, err := getPotentiallyValidManifestFile(linkPath, info, appDir, appDir, "", "")
			assert.NotNil(t, realFileInfo)
			assert.Empty(t, ignoreMessage)
			require.NoError(t, err)
		})
	})

	t.Run("a regular file is potentially valid", func(t *testing.T) {
		appDir := tempDir(t)

		filePath := filepath.Join(appDir, "regular-file.json")
		file, err := os.OpenFile(filePath, os.O_RDONLY|os.O_CREATE, 0o644)
		require.NoError(t, err)
		err = file.Close()
		require.NoError(t, err)

		walkFor(t, appDir, filePath, func(info fs.FileInfo) {
			realFileInfo, ignoreMessage, err := getPotentiallyValidManifestFile(filePath, info, appDir, appDir, "", "")
			assert.NotNil(t, realFileInfo)
			assert.Empty(t, ignoreMessage)
			require.NoError(t, err)
		})
	})

	t.Run("realFileInfo is for the destination rather than the symlink", func(t *testing.T) {
		appDir := tempDir(t)

		filePath := filepath.Join(appDir, "regular-file")
		file, err := os.OpenFile(filePath, os.O_RDONLY|os.O_CREATE, 0o644)
		require.NoError(t, err)
		err = file.Close()
		require.NoError(t, err)

		linkPath := filepath.Join(appDir, "link.json")
		err = os.Symlink(filePath, linkPath)
		require.NoError(t, err)

		walkFor(t, appDir, linkPath, func(info fs.FileInfo) {
			realFileInfo, ignoreMessage, err := getPotentiallyValidManifestFile(linkPath, info, appDir, appDir, "", "")
			assert.NotNil(t, realFileInfo)
			assert.Equal(t, filepath.Base(filePath), realFileInfo.Name())
			assert.Empty(t, ignoreMessage)
			require.NoError(t, err)
		})
	})
}

func Test_getPotentiallyValidManifests(t *testing.T) {
	// Tests which return no manifests and an error check to make sure the directory exists before running. A missing
	// directory would produce those same results.

	logCtx := log.WithField("test", "test")

	t.Run("unreadable file throws error", func(t *testing.T) {
		appDir := t.TempDir()
		unreadablePath := filepath.Join(appDir, "unreadable.json")
		err := os.WriteFile(unreadablePath, []byte{}, 0o666)
		require.NoError(t, err)
		err = os.Chmod(appDir, 0o000)
		require.NoError(t, err)

		manifests, err := getPotentiallyValidManifests(logCtx, appDir, appDir, false, "", "", resource.MustParse("0"))
		assert.Empty(t, manifests)
		require.Error(t, err)

		// allow cleanup
		err = os.Chmod(appDir, 0o777)
		if err != nil {
			panic(err)
		}
	})

	t.Run("no recursion when recursion is disabled", func(t *testing.T) {
		manifests, err := getPotentiallyValidManifests(logCtx, "./testdata/recurse", "./testdata/recurse", false, "", "", resource.MustParse("0"))
		assert.Len(t, manifests, 1)
		require.NoError(t, err)
	})

	t.Run("recursion when recursion is enabled", func(t *testing.T) {
		manifests, err := getPotentiallyValidManifests(logCtx, "./testdata/recurse", "./testdata/recurse", true, "", "", resource.MustParse("0"))
		assert.Len(t, manifests, 2)
		require.NoError(t, err)
	})

	t.Run("non-JSON/YAML is skipped", func(t *testing.T) {
		manifests, err := getPotentiallyValidManifests(logCtx, "./testdata/non-manifest-file", "./testdata/non-manifest-file", false, "", "", resource.MustParse("0"))
		assert.Empty(t, manifests)
		require.NoError(t, err)
	})

	t.Run("circular link should throw an error", func(t *testing.T) {
		const testDir = "./testdata/circular-link"
		require.DirExists(t, testDir)
		require.NoError(t, fileutil.CreateSymlink(t, testDir, "a.json", "b.json"))
		defer os.Remove(path.Join(testDir, "a.json"))
		require.NoError(t, fileutil.CreateSymlink(t, testDir, "b.json", "a.json"))
		defer os.Remove(path.Join(testDir, "b.json"))
		manifests, err := getPotentiallyValidManifests(logCtx, "./testdata/circular-link", "./testdata/circular-link", false, "", "", resource.MustParse("0"))
		assert.Empty(t, manifests)
		require.Error(t, err)
	})

	t.Run("out-of-bounds symlink should throw an error", func(t *testing.T) {
		require.DirExists(t, "./testdata/out-of-bounds-link")
		manifests, err := getPotentiallyValidManifests(logCtx, "./testdata/out-of-bounds-link", "./testdata/out-of-bounds-link", false, "", "", resource.MustParse("0"))
		assert.Empty(t, manifests)
		require.Error(t, err)
	})

	t.Run("symlink to a regular file works", func(t *testing.T) {
		repoRoot, err := filepath.Abs("./testdata/in-bounds-link")
		require.NoError(t, err)
		appPath, err := filepath.Abs("./testdata/in-bounds-link/app")
		require.NoError(t, err)
		manifests, err := getPotentiallyValidManifests(logCtx, appPath, repoRoot, false, "", "", resource.MustParse("0"))
		assert.Len(t, manifests, 1)
		require.NoError(t, err)
	})

	t.Run("symlink to nowhere should be ignored", func(t *testing.T) {
		manifests, err := getPotentiallyValidManifests(logCtx, "./testdata/link-to-nowhere", "./testdata/link-to-nowhere", false, "", "", resource.MustParse("0"))
		assert.Empty(t, manifests)
		require.NoError(t, err)
	})

	t.Run("link to over-sized manifest fails", func(t *testing.T) {
		repoRoot, err := filepath.Abs("./testdata/in-bounds-link")
		require.NoError(t, err)
		appPath, err := filepath.Abs("./testdata/in-bounds-link/app")
		require.NoError(t, err)
		// The file is 35 bytes.
		manifests, err := getPotentiallyValidManifests(logCtx, appPath, repoRoot, false, "", "", resource.MustParse("34"))
		assert.Empty(t, manifests)
		assert.ErrorIs(t, err, ErrExceededMaxCombinedManifestFileSize)
	})

	t.Run("group of files should be limited at precisely the sum of their size", func(t *testing.T) {
		// There is a total of 10 files, ech file being 10 bytes.
		manifests, err := getPotentiallyValidManifests(logCtx, "./testdata/several-files", "./testdata/several-files", false, "", "", resource.MustParse("365"))
		assert.Len(t, manifests, 10)
		require.NoError(t, err)

		manifests, err = getPotentiallyValidManifests(logCtx, "./testdata/several-files", "./testdata/several-files", false, "", "", resource.MustParse("100"))
		assert.Empty(t, manifests)
		assert.ErrorIs(t, err, ErrExceededMaxCombinedManifestFileSize)
	})
}

func Test_findManifests(t *testing.T) {
	logCtx := log.WithField("test", "test")
	noRecurse := argoappv1.ApplicationSourceDirectory{Recurse: false}

	t.Run("unreadable file throws error", func(t *testing.T) {
		appDir := t.TempDir()
		unreadablePath := filepath.Join(appDir, "unreadable.json")
		err := os.WriteFile(unreadablePath, []byte{}, 0o666)
		require.NoError(t, err)
		err = os.Chmod(appDir, 0o000)
		require.NoError(t, err)

		manifests, err := findManifests(logCtx, appDir, appDir, nil, noRecurse, nil, resource.MustParse("0"))
		assert.Empty(t, manifests)
		require.Error(t, err)

		// allow cleanup
		err = os.Chmod(appDir, 0o777)
		if err != nil {
			panic(err)
		}
	})

	t.Run("no recursion when recursion is disabled", func(t *testing.T) {
		manifests, err := findManifests(logCtx, "./testdata/recurse", "./testdata/recurse", nil, noRecurse, nil, resource.MustParse("0"))
		assert.Len(t, manifests, 2)
		require.NoError(t, err)
	})

	t.Run("recursion when recursion is enabled", func(t *testing.T) {
		recurse := argoappv1.ApplicationSourceDirectory{Recurse: true}
		manifests, err := findManifests(logCtx, "./testdata/recurse", "./testdata/recurse", nil, recurse, nil, resource.MustParse("0"))
		assert.Len(t, manifests, 4)
		require.NoError(t, err)
	})

	t.Run("non-JSON/YAML is skipped", func(t *testing.T) {
		manifests, err := findManifests(logCtx, "./testdata/non-manifest-file", "./testdata/non-manifest-file", nil, noRecurse, nil, resource.MustParse("0"))
		assert.Empty(t, manifests)
		require.NoError(t, err)
	})

	t.Run("circular link should throw an error", func(t *testing.T) {
		const testDir = "./testdata/circular-link"
		require.DirExists(t, testDir)
		require.NoError(t, fileutil.CreateSymlink(t, testDir, "a.json", "b.json"))
		defer os.Remove(path.Join(testDir, "a.json"))
		require.NoError(t, fileutil.CreateSymlink(t, testDir, "b.json", "a.json"))
		defer os.Remove(path.Join(testDir, "b.json"))
		manifests, err := findManifests(logCtx, "./testdata/circular-link", "./testdata/circular-link", nil, noRecurse, nil, resource.MustParse("0"))
		assert.Empty(t, manifests)
		require.Error(t, err)
	})

	t.Run("out-of-bounds symlink should throw an error", func(t *testing.T) {
		require.DirExists(t, "./testdata/out-of-bounds-link")
		manifests, err := findManifests(logCtx, "./testdata/out-of-bounds-link", "./testdata/out-of-bounds-link", nil, noRecurse, nil, resource.MustParse("0"))
		assert.Empty(t, manifests)
		require.Error(t, err)
	})

	t.Run("symlink to a regular file works", func(t *testing.T) {
		repoRoot, err := filepath.Abs("./testdata/in-bounds-link")
		require.NoError(t, err)
		appPath, err := filepath.Abs("./testdata/in-bounds-link/app")
		require.NoError(t, err)
		manifests, err := findManifests(logCtx, appPath, repoRoot, nil, noRecurse, nil, resource.MustParse("0"))
		assert.Len(t, manifests, 1)
		require.NoError(t, err)
	})

	t.Run("symlink to nowhere should be ignored", func(t *testing.T) {
		manifests, err := findManifests(logCtx, "./testdata/link-to-nowhere", "./testdata/link-to-nowhere", nil, noRecurse, nil, resource.MustParse("0"))
		assert.Empty(t, manifests)
		require.NoError(t, err)
	})

	t.Run("link to over-sized manifest fails", func(t *testing.T) {
		repoRoot, err := filepath.Abs("./testdata/in-bounds-link")
		require.NoError(t, err)
		appPath, err := filepath.Abs("./testdata/in-bounds-link/app")
		require.NoError(t, err)
		// The file is 35 bytes.
		manifests, err := findManifests(logCtx, appPath, repoRoot, nil, noRecurse, nil, resource.MustParse("34"))
		assert.Empty(t, manifests)
		assert.ErrorIs(t, err, ErrExceededMaxCombinedManifestFileSize)
	})

	t.Run("group of files should be limited at precisely the sum of their size", func(t *testing.T) {
		// There is a total of 10 files, each file being 10 bytes.
		manifests, err := findManifests(logCtx, "./testdata/several-files", "./testdata/several-files", nil, noRecurse, nil, resource.MustParse("365"))
		assert.Len(t, manifests, 10)
		require.NoError(t, err)

		manifests, err = findManifests(logCtx, "./testdata/several-files", "./testdata/several-files", nil, noRecurse, nil, resource.MustParse("364"))
		assert.Empty(t, manifests)
		assert.ErrorIs(t, err, ErrExceededMaxCombinedManifestFileSize)
	})

	t.Run("jsonnet isn't counted against size limit", func(t *testing.T) {
		// Each file is 36 bytes. Only the 36-byte json file should be counted against the limit.
		manifests, err := findManifests(logCtx, "./testdata/jsonnet-and-json", "./testdata/jsonnet-and-json", nil, noRecurse, nil, resource.MustParse("36"))
		assert.Len(t, manifests, 2)
		require.NoError(t, err)

		manifests, err = findManifests(logCtx, "./testdata/jsonnet-and-json", "./testdata/jsonnet-and-json", nil, noRecurse, nil, resource.MustParse("35"))
		assert.Empty(t, manifests)
		assert.ErrorIs(t, err, ErrExceededMaxCombinedManifestFileSize)
	})

	t.Run("partially valid YAML file throws an error", func(t *testing.T) {
		require.DirExists(t, "./testdata/partially-valid-yaml")
		manifests, err := findManifests(logCtx, "./testdata/partially-valid-yaml", "./testdata/partially-valid-yaml", nil, noRecurse, nil, resource.MustParse("0"))
		assert.Empty(t, manifests)
		require.Error(t, err)
	})

	t.Run("invalid manifest throws an error", func(t *testing.T) {
		require.DirExists(t, "./testdata/invalid-manifests")
		manifests, err := findManifests(logCtx, "./testdata/invalid-manifests", "./testdata/invalid-manifests", nil, noRecurse, nil, resource.MustParse("0"))
		assert.Empty(t, manifests)
		require.Error(t, err)
	})

	t.Run("irrelevant YAML gets skipped, relevant YAML gets parsed", func(t *testing.T) {
		manifests, err := findManifests(logCtx, "./testdata/irrelevant-yaml", "./testdata/irrelevant-yaml", nil, noRecurse, nil, resource.MustParse("0"))
		assert.Len(t, manifests, 1)
		require.NoError(t, err)
	})

	t.Run("multiple JSON objects in one file throws an error", func(t *testing.T) {
		require.DirExists(t, "./testdata/json-list")
		manifests, err := findManifests(logCtx, "./testdata/json-list", "./testdata/json-list", nil, noRecurse, nil, resource.MustParse("0"))
		assert.Empty(t, manifests)
		require.Error(t, err)
	})

	t.Run("invalid JSON throws an error", func(t *testing.T) {
		require.DirExists(t, "./testdata/invalid-json")
		manifests, err := findManifests(logCtx, "./testdata/invalid-json", "./testdata/invalid-json", nil, noRecurse, nil, resource.MustParse("0"))
		assert.Empty(t, manifests)
		require.Error(t, err)
	})

	t.Run("valid JSON returns manifest and no error", func(t *testing.T) {
		manifests, err := findManifests(logCtx, "./testdata/valid-json", "./testdata/valid-json", nil, noRecurse, nil, resource.MustParse("0"))
		assert.Len(t, manifests, 1)
		require.NoError(t, err)
	})

	t.Run("YAML with an empty document doesn't throw an error", func(t *testing.T) {
		manifests, err := findManifests(logCtx, "./testdata/yaml-with-empty-document", "./testdata/yaml-with-empty-document", nil, noRecurse, nil, resource.MustParse("0"))
		assert.Len(t, manifests, 1)
		require.NoError(t, err)
	})
}

func TestTestRepoOCI(t *testing.T) {
	service := newService(t, ".")
	_, err := service.TestRepository(context.Background(), &apiclient.TestRepositoryRequest{
		Repo: &argoappv1.Repository{
			Repo:      "https://demo.goharbor.io",
			Type:      "helm",
			EnableOCI: true,
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OCI Helm repository URL should include hostname and port only")
}

func Test_getHelmDependencyRepos(t *testing.T) {
	repo1 := "https://charts.bitnami.com/bitnami"
	repo2 := "https://eventstore.github.io/EventStore.Charts"

	repos, err := getHelmDependencyRepos("../../util/helm/testdata/dependency")
	require.NoError(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, repos[0].Repo, repo1)
	assert.Equal(t, repos[1].Repo, repo2)
}

func TestResolveRevision(t *testing.T) {
	service := newService(t, ".")
	repo := &argoappv1.Repository{Repo: "https://github.com/argoproj/argo-cd"}
	app := &argoappv1.Application{Spec: argoappv1.ApplicationSpec{Source: &argoappv1.ApplicationSource{}}}
	resolveRevisionResponse, err := service.ResolveRevision(context.Background(), &apiclient.ResolveRevisionRequest{
		Repo:              repo,
		App:               app,
		AmbiguousRevision: "v2.2.2",
	})

	expectedResolveRevisionResponse := &apiclient.ResolveRevisionResponse{
		Revision:          "03b17e0233e64787ffb5fcf65c740cc2a20822ba",
		AmbiguousRevision: "v2.2.2 (03b17e0233e64787ffb5fcf65c740cc2a20822ba)",
	}

	assert.NotNil(t, resolveRevisionResponse.Revision)
	require.NoError(t, err)
	assert.Equal(t, expectedResolveRevisionResponse, resolveRevisionResponse)
}

func TestResolveRevisionNegativeScenarios(t *testing.T) {
	service := newService(t, ".")
	repo := &argoappv1.Repository{Repo: "https://github.com/argoproj/argo-cd"}
	app := &argoappv1.Application{Spec: argoappv1.ApplicationSpec{Source: &argoappv1.ApplicationSource{}}}
	resolveRevisionResponse, err := service.ResolveRevision(context.Background(), &apiclient.ResolveRevisionRequest{
		Repo:              repo,
		App:               app,
		AmbiguousRevision: "v2.a.2",
	})

	expectedResolveRevisionResponse := &apiclient.ResolveRevisionResponse{
		Revision:          "",
		AmbiguousRevision: "",
	}

	assert.NotNil(t, resolveRevisionResponse.Revision)
	require.Error(t, err)
	assert.Equal(t, expectedResolveRevisionResponse, resolveRevisionResponse)
}

func TestDirectoryPermissionInitializer(t *testing.T) {
	dir := t.TempDir()

	file, err := os.CreateTemp(dir, "")
	require.NoError(t, err)
	io.Close(file)

	// remove read permissions
	require.NoError(t, os.Chmod(dir, 0o000))

	// Remember to restore permissions when the test finishes so dir can
	// be removed properly.
	t.Cleanup(func() {
		require.NoError(t, os.Chmod(dir, 0o777))
	})

	// make sure permission are restored
	closer := directoryPermissionInitializer(dir)
	_, err = os.ReadFile(file.Name())
	require.NoError(t, err)

	// make sure permission are removed by closer
	io.Close(closer)
	_, err = os.ReadFile(file.Name())
	require.Error(t, err)
}

func addHelmToGitRepo(t *testing.T, options newGitRepoOptions) {
	err := os.WriteFile(filepath.Join(options.path, "Chart.yaml"), []byte("name: test\nversion: v1.0.0"), 0o777)
	require.NoError(t, err)
	for valuesFileName, values := range options.helmChartOptions.valuesFiles {
		valuesFileContents, err := yaml.Marshal(values)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(options.path, valuesFileName), valuesFileContents, 0o777)
		require.NoError(t, err)
	}
	require.NoError(t, err)
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = options.path
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = options.path
	require.NoError(t, cmd.Run())
}

func initGitRepo(t *testing.T, options newGitRepoOptions) (revision string) {
	if options.createPath {
		require.NoError(t, os.Mkdir(options.path, 0o755))
	}

	cmd := exec.Command("git", "init", "-b", "main", options.path)
	cmd.Dir = options.path
	require.NoError(t, cmd.Run())

	if options.remote != "" {
		cmd = exec.Command("git", "remote", "add", "origin", options.path)
		cmd.Dir = options.path
		require.NoError(t, cmd.Run())
	}

	commitAdded := options.addEmptyCommit || options.helmChartOptions.chartName != ""
	if options.addEmptyCommit {
		cmd = exec.Command("git", "commit", "-m", "Initial commit", "--allow-empty")
		cmd.Dir = options.path
		require.NoError(t, cmd.Run())
	} else if options.helmChartOptions.chartName != "" {
		addHelmToGitRepo(t, options)
	}

	if commitAdded {
		var revB bytes.Buffer
		cmd = exec.Command("git", "rev-parse", "HEAD", options.path)
		cmd.Dir = options.path
		cmd.Stdout = &revB
		require.NoError(t, cmd.Run())
		revision = strings.Split(revB.String(), "\n")[0]
	}
	return revision
}

func TestInit(t *testing.T) {
	dir := t.TempDir()

	// service.Init sets permission to 0300. Restore permissions when the test
	// finishes so dir can be removed properly.
	t.Cleanup(func() {
		require.NoError(t, os.Chmod(dir, 0o777))
	})

	repoPath := path.Join(dir, "repo1")
	initGitRepo(t, newGitRepoOptions{path: repoPath, remote: "https://github.com/argo-cd/test-repo1", createPath: true, addEmptyCommit: false})

	service := newService(t, ".")
	service.rootDir = dir

	require.NoError(t, service.Init())

	_, err := os.ReadDir(dir)
	require.Error(t, err)
	initGitRepo(t, newGitRepoOptions{path: path.Join(dir, "repo2"), remote: "https://github.com/argo-cd/test-repo2", createPath: true, addEmptyCommit: false})
}

// TestCheckoutRevisionCanGetNonstandardRefs shows that we can fetch a revision that points to a non-standard ref. In
// other words, we haven't regressed and caused this issue again: https://github.com/argoproj/argo-cd/issues/4935
func TestCheckoutRevisionCanGetNonstandardRefs(t *testing.T) {
	rootPath := t.TempDir()

	sourceRepoPath, err := os.MkdirTemp(rootPath, "")
	require.NoError(t, err)

	// Create a repo such that one commit is on a non-standard ref _and nowhere else_. This is meant to simulate, for
	// example, a GitHub ref for a pull into one repo from a fork of that repo.
	runGit(t, sourceRepoPath, "init")
	runGit(t, sourceRepoPath, "checkout", "-b", "main") // make sure there's a main branch to switch back to
	runGit(t, sourceRepoPath, "commit", "-m", "empty", "--allow-empty")
	runGit(t, sourceRepoPath, "checkout", "-b", "branch")
	runGit(t, sourceRepoPath, "commit", "-m", "empty", "--allow-empty")
	sha := runGit(t, sourceRepoPath, "rev-parse", "HEAD")
	runGit(t, sourceRepoPath, "update-ref", "refs/pull/123/head", strings.TrimSuffix(sha, "\n"))
	runGit(t, sourceRepoPath, "checkout", "main")
	runGit(t, sourceRepoPath, "branch", "-D", "branch")

	destRepoPath, err := os.MkdirTemp(rootPath, "")
	require.NoError(t, err)

	gitClient, err := git.NewClientExt("file://"+sourceRepoPath, destRepoPath, &git.NopCreds{}, true, false, "", "")
	require.NoError(t, err)

	pullSha, err := gitClient.LsRemote("refs/pull/123/head")
	require.NoError(t, err)

	err = checkoutRevision(gitClient, "does-not-exist", false)
	require.Error(t, err)

	err = checkoutRevision(gitClient, pullSha, false)
	require.NoError(t, err)
}

func TestCheckoutRevisionPresentSkipFetch(t *testing.T) {
	revision := "0123456789012345678901234567890123456789"

	gitClient := &gitmocks.Client{}
	gitClient.On("Init").Return(nil)
	gitClient.On("IsRevisionPresent", revision).Return(true)
	gitClient.On("Checkout", revision, mock.Anything).Return(nil)

	err := checkoutRevision(gitClient, revision, false)
	require.NoError(t, err)
}

func TestCheckoutRevisionNotPresentCallFetch(t *testing.T) {
	revision := "0123456789012345678901234567890123456789"

	gitClient := &gitmocks.Client{}
	gitClient.On("Init").Return(nil)
	gitClient.On("IsRevisionPresent", revision).Return(false)
	gitClient.On("Fetch", "").Return(nil)
	gitClient.On("Checkout", revision, mock.Anything).Return(nil)

	err := checkoutRevision(gitClient, revision, false)
	require.NoError(t, err)
}

// runGit runs a git command in the given working directory. If the command succeeds, it returns the combined standard
// and error output. If it fails, it stops the test with a failure message.
func runGit(t *testing.T, workDir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	stringOut := string(out)
	require.NoError(t, err, stringOut)
	return stringOut
}

func Test_walkHelmValueFilesInPath(t *testing.T) {
	t.Run("does not exist", func(t *testing.T) {
		var files []string
		root := "/obviously/does/not/exist"
		err := filepath.Walk(root, walkHelmValueFilesInPath(root, &files))
		require.Error(t, err)
		assert.Empty(t, files)
	})
	t.Run("values files", func(t *testing.T) {
		var files []string
		root := "./testdata/values-files"
		err := filepath.Walk(root, walkHelmValueFilesInPath(root, &files))
		require.NoError(t, err)
		assert.Len(t, files, 5)
	})
	t.Run("unrelated root", func(t *testing.T) {
		var files []string
		root := "./testdata/values-files"
		unrelated_root := "/different/root/path"
		err := filepath.Walk(root, walkHelmValueFilesInPath(unrelated_root, &files))
		require.Error(t, err)
	})
}

func Test_populateHelmAppDetails(t *testing.T) {
	emptyTempPaths := io.NewRandomizedTempPaths(t.TempDir())
	res := apiclient.RepoAppDetailsResponse{}
	q := apiclient.RepoServerAppDetailsQuery{
		Repo: &argoappv1.Repository{},
		Source: &argoappv1.ApplicationSource{
			Helm: &argoappv1.ApplicationSourceHelm{ValueFiles: []string{"exclude.yaml", "has-the-word-values.yaml"}},
		},
	}
	appPath, err := filepath.Abs("./testdata/values-files/")
	require.NoError(t, err)
	err = populateHelmAppDetails(&res, appPath, appPath, &q, emptyTempPaths)
	require.NoError(t, err)
	assert.Len(t, res.Helm.Parameters, 3)
	assert.Len(t, res.Helm.ValueFiles, 5)
}

func Test_populateHelmAppDetails_values_symlinks(t *testing.T) {
	emptyTempPaths := io.NewRandomizedTempPaths(t.TempDir())
	t.Run("inbound", func(t *testing.T) {
		res := apiclient.RepoAppDetailsResponse{}
		q := apiclient.RepoServerAppDetailsQuery{Repo: &argoappv1.Repository{}, Source: &argoappv1.ApplicationSource{}}
		err := populateHelmAppDetails(&res, "./testdata/in-bounds-values-file-link/", "./testdata/in-bounds-values-file-link/", &q, emptyTempPaths)
		require.NoError(t, err)
		assert.NotEmpty(t, res.Helm.Values)
		assert.NotEmpty(t, res.Helm.Parameters)
	})

	t.Run("out of bounds", func(t *testing.T) {
		res := apiclient.RepoAppDetailsResponse{}
		q := apiclient.RepoServerAppDetailsQuery{Repo: &argoappv1.Repository{}, Source: &argoappv1.ApplicationSource{}}
		err := populateHelmAppDetails(&res, "./testdata/out-of-bounds-values-file-link/", "./testdata/out-of-bounds-values-file-link/", &q, emptyTempPaths)
		require.NoError(t, err)
		assert.Empty(t, res.Helm.Values)
		assert.Empty(t, res.Helm.Parameters)
	})
}

func TestGetHelmRepos_OCIDependenciesWithHelmRepo(t *testing.T) {
	src := argoappv1.ApplicationSource{Path: "."}
	q := apiclient.ManifestRequest{Repos: []*argoappv1.Repository{}, ApplicationSource: &src, HelmRepoCreds: []*argoappv1.RepoCreds{
		{URL: "example.com", Username: "test", Password: "test", EnableOCI: true},
	}}

	helmRepos, err := getHelmRepos("./testdata/oci-dependencies", q.Repos, q.HelmRepoCreds)
	require.NoError(t, err)

	assert.Len(t, helmRepos, 1)
	assert.Equal(t, "test", helmRepos[0].Username)
	assert.True(t, helmRepos[0].EnableOci)
	assert.Equal(t, "example.com/myrepo", helmRepos[0].Repo)
}

func TestGetHelmRepos_OCIDependenciesWithRepo(t *testing.T) {
	src := argoappv1.ApplicationSource{Path: "."}
	q := apiclient.ManifestRequest{Repos: []*argoappv1.Repository{{Repo: "example.com", Username: "test", Password: "test", EnableOCI: true}}, ApplicationSource: &src, HelmRepoCreds: []*argoappv1.RepoCreds{}}

	helmRepos, err := getHelmRepos("./testdata/oci-dependencies", q.Repos, q.HelmRepoCreds)
	require.NoError(t, err)

	assert.Len(t, helmRepos, 1)
	assert.Equal(t, "test", helmRepos[0].Username)
	assert.True(t, helmRepos[0].EnableOci)
	assert.Equal(t, "example.com/myrepo", helmRepos[0].Repo)
}

func TestGetHelmRepo_NamedRepos(t *testing.T) {
	src := argoappv1.ApplicationSource{Path: "."}
	q := apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: &src, Repos: []*argoappv1.Repository{{
		Name:     "custom-repo",
		Repo:     "https://example.com",
		Username: "test",
	}}}

	helmRepos, err := getHelmRepos("./testdata/helm-with-dependencies", q.Repos, q.HelmRepoCreds)
	require.NoError(t, err)

	assert.Len(t, helmRepos, 1)
	assert.Equal(t, "test", helmRepos[0].Username)
	assert.Equal(t, "https://example.com", helmRepos[0].Repo)
}

func TestGetHelmRepo_NamedReposAlias(t *testing.T) {
	src := argoappv1.ApplicationSource{Path: "."}
	q := apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: &src, Repos: []*argoappv1.Repository{{
		Name:     "custom-repo-alias",
		Repo:     "https://example.com",
		Username: "test-alias",
	}}}

	helmRepos, err := getHelmRepos("./testdata/helm-with-dependencies-alias", q.Repos, q.HelmRepoCreds)
	require.NoError(t, err)

	assert.Len(t, helmRepos, 1)
	assert.Equal(t, "test-alias", helmRepos[0].Username)
	assert.Equal(t, "https://example.com", helmRepos[0].Repo)
}

func Test_getResolvedValueFiles(t *testing.T) {
	tempDir := t.TempDir()
	paths := io.NewRandomizedTempPaths(tempDir)

	key, _ := json.Marshal(map[string]string{"url": git.NormalizeGitURL("https://github.com/org/repo1"), "project": ""})
	paths.Add(string(key), path.Join(tempDir, "repo1"))

	testCases := []struct {
		name         string
		rawPath      string
		env          *argoappv1.Env
		refSources   map[string]*argoappv1.RefTarget
		expectedPath string
		expectedErr  bool
	}{
		{
			name:         "simple path",
			rawPath:      "values.yaml",
			env:          &argoappv1.Env{},
			refSources:   map[string]*argoappv1.RefTarget{},
			expectedPath: path.Join(tempDir, "main-repo", "values.yaml"),
		},
		{
			name:    "simple ref",
			rawPath: "$ref/values.yaml",
			env:     &argoappv1.Env{},
			refSources: map[string]*argoappv1.RefTarget{
				"$ref": {
					Repo: argoappv1.Repository{
						Repo: "https://github.com/org/repo1",
					},
				},
			},
			expectedPath: path.Join(tempDir, "repo1", "values.yaml"),
		},
		{
			name:    "only ref",
			rawPath: "$ref",
			env:     &argoappv1.Env{},
			refSources: map[string]*argoappv1.RefTarget{
				"$ref": {
					Repo: argoappv1.Repository{
						Repo: "https://github.com/org/repo1",
					},
				},
			},
			expectedErr: true,
		},
		{
			name:    "attempted traversal",
			rawPath: "$ref/../values.yaml",
			env:     &argoappv1.Env{},
			refSources: map[string]*argoappv1.RefTarget{
				"$ref": {
					Repo: argoappv1.Repository{
						Repo: "https://github.com/org/repo1",
					},
				},
			},
			expectedErr: true,
		},
		{
			// Since $ref doesn't resolve to a ref target, we assume it's an env var. Since the env var isn't specified,
			// it's replaced with an empty string. This is necessary for backwards compatibility with behavior before
			// ref targets were introduced.
			name:         "ref doesn't exist",
			rawPath:      "$ref/values.yaml",
			env:          &argoappv1.Env{},
			refSources:   map[string]*argoappv1.RefTarget{},
			expectedPath: path.Join(tempDir, "main-repo", "values.yaml"),
		},
		{
			name:    "repo doesn't exist",
			rawPath: "$ref/values.yaml",
			env:     &argoappv1.Env{},
			refSources: map[string]*argoappv1.RefTarget{
				"$ref": {
					Repo: argoappv1.Repository{
						Repo: "https://github.com/org/repo2",
					},
				},
			},
			expectedErr: true,
		},
		{
			name:    "env var is resolved",
			rawPath: "$ref/$APP_PATH/values.yaml",
			env: &argoappv1.Env{
				&argoappv1.EnvEntry{
					Name:  "APP_PATH",
					Value: "app-path",
				},
			},
			refSources: map[string]*argoappv1.RefTarget{
				"$ref": {
					Repo: argoappv1.Repository{
						Repo: "https://github.com/org/repo1",
					},
				},
			},
			expectedPath: path.Join(tempDir, "repo1", "app-path", "values.yaml"),
		},
		{
			name:    "traversal in env var is blocked",
			rawPath: "$ref/$APP_PATH/values.yaml",
			env: &argoappv1.Env{
				&argoappv1.EnvEntry{
					Name:  "APP_PATH",
					Value: "..",
				},
			},
			refSources: map[string]*argoappv1.RefTarget{
				"$ref": {
					Repo: argoappv1.Repository{
						Repo: "https://github.com/org/repo1",
					},
				},
			},
			expectedErr: true,
		},
		{
			name:    "env var prefix",
			rawPath: "$APP_PATH/values.yaml",
			env: &argoappv1.Env{
				&argoappv1.EnvEntry{
					Name:  "APP_PATH",
					Value: "app-path",
				},
			},
			refSources:   map[string]*argoappv1.RefTarget{},
			expectedPath: path.Join(tempDir, "main-repo", "app-path", "values.yaml"),
		},
		{
			name:         "unresolved env var",
			rawPath:      "$APP_PATH/values.yaml",
			env:          &argoappv1.Env{},
			refSources:   map[string]*argoappv1.RefTarget{},
			expectedPath: path.Join(tempDir, "main-repo", "values.yaml"),
		},
	}

	for _, tc := range testCases {
		tcc := tc
		t.Run(tcc.name, func(t *testing.T) {
			t.Parallel()
			resolvedPaths, err := getResolvedValueFiles(path.Join(tempDir, "main-repo"), path.Join(tempDir, "main-repo"), tcc.env, []string{}, []string{tcc.rawPath}, tcc.refSources, paths, false)
			if !tcc.expectedErr {
				require.NoError(t, err)
				require.Len(t, resolvedPaths, 1)
				assert.Equal(t, tcc.expectedPath, string(resolvedPaths[0]))
			} else {
				require.Error(t, err)
				assert.Empty(t, resolvedPaths)
			}
		})
	}
}

func TestErrorGetGitDirectories(t *testing.T) {
	// test not using the cache
	root := "./testdata/git-files-dirs"

	type fields struct {
		service *Service
	}
	type args struct {
		ctx     context.Context
		request *apiclient.GitDirectoriesRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *apiclient.GitDirectoriesResponse
		wantErr assert.ErrorAssertionFunc
	}{
		{name: "InvalidRepo", fields: fields{service: newService(t, ".")}, args: args{
			ctx: context.TODO(),
			request: &apiclient.GitDirectoriesRequest{
				Repo:             nil,
				SubmoduleEnabled: false,
				Revision:         "HEAD",
			},
		}, want: nil, wantErr: assert.Error},
		{name: "InvalidResolveRevision", fields: fields{service: func() *Service {
			s, _, _ := newServiceWithOpt(t, func(gitClient *gitmocks.Client, helmClient *helmmocks.Client, paths *iomocks.TempPaths) {
				gitClient.On("Checkout", mock.Anything, mock.Anything).Return(nil)
				gitClient.On("LsRemote", mock.Anything).Return("", fmt.Errorf("ah error"))
				gitClient.On("Root").Return(root)
				paths.On("GetPath", mock.Anything).Return(".", nil)
				paths.On("GetPathIfExists", mock.Anything).Return(".", nil)
			}, ".")
			return s
		}()}, args: args{
			ctx: context.TODO(),
			request: &apiclient.GitDirectoriesRequest{
				Repo:             &argoappv1.Repository{Repo: "not-a-valid-url"},
				SubmoduleEnabled: false,
				Revision:         "sadfsadf",
			},
		}, want: nil, wantErr: assert.Error},
		{name: "ErrorVerifyCommit", fields: fields{service: func() *Service {
			s, _, _ := newServiceWithOpt(t, func(gitClient *gitmocks.Client, helmClient *helmmocks.Client, paths *iomocks.TempPaths) {
				gitClient.On("Checkout", mock.Anything, mock.Anything).Return(nil)
				gitClient.On("LsRemote", mock.Anything).Return("", fmt.Errorf("ah error"))
				gitClient.On("VerifyCommitSignature", mock.Anything).Return("", fmt.Errorf("revision %s is not signed", "sadfsadf"))
				gitClient.On("Root").Return(root)
				paths.On("GetPath", mock.Anything).Return(".", nil)
				paths.On("GetPathIfExists", mock.Anything).Return(".", nil)
			}, ".")
			return s
		}()}, args: args{
			ctx: context.TODO(),
			request: &apiclient.GitDirectoriesRequest{
				Repo:             &argoappv1.Repository{Repo: "not-a-valid-url"},
				SubmoduleEnabled: false,
				Revision:         "sadfsadf",
				VerifyCommit:     true,
			},
		}, want: nil, wantErr: assert.Error},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.fields.service
			got, err := s.GetGitDirectories(tt.args.ctx, tt.args.request)
			if !tt.wantErr(t, err, fmt.Sprintf("GetGitDirectories(%v, %v)", tt.args.ctx, tt.args.request)) {
				return
			}
			assert.Equalf(t, tt.want, got, "GetGitDirectories(%v, %v)", tt.args.ctx, tt.args.request)
		})
	}
}

func TestGetGitDirectories(t *testing.T) {
	// test not using the cache
	root := "./testdata/git-files-dirs"
	s, _, cacheMocks := newServiceWithOpt(t, func(gitClient *gitmocks.Client, helmClient *helmmocks.Client, paths *iomocks.TempPaths) {
		gitClient.On("Init").Return(nil)
		gitClient.On("IsRevisionPresent", mock.Anything).Return(false)
		gitClient.On("Fetch", mock.Anything).Return(nil)
		gitClient.On("Checkout", mock.Anything, mock.Anything).Once().Return(nil)
		gitClient.On("LsRemote", "HEAD").Return("632039659e542ed7de0c170a4fcc1c571b288fc0", nil)
		gitClient.On("Root").Return(root)
		paths.On("GetPath", mock.Anything).Return(root, nil)
		paths.On("GetPathIfExists", mock.Anything).Return(root, nil)
	}, root)
	dirRequest := &apiclient.GitDirectoriesRequest{
		Repo:             &argoappv1.Repository{Repo: "a-url.com"},
		SubmoduleEnabled: false,
		Revision:         "HEAD",
	}
	directories, err := s.GetGitDirectories(context.TODO(), dirRequest)
	require.NoError(t, err)
	assert.ElementsMatch(t, directories.GetPaths(), []string{"app", "app/bar", "app/foo/bar", "somedir", "app/foo"})

	// do the same request again to use the cache
	// we only allow CheckOut to be called once in the mock
	directories, err = s.GetGitDirectories(context.TODO(), dirRequest)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"app", "app/bar", "app/foo/bar", "somedir", "app/foo"}, directories.GetPaths())
	cacheMocks.mockCache.AssertCacheCalledTimes(t, &repositorymocks.CacheCallCounts{
		ExternalSets: 1,
		ExternalGets: 2,
	})
}

func TestGetGitDirectoriesWithHiddenDirSupported(t *testing.T) {
	// test not using the cache
	root := "./testdata/git-files-dirs"
	s, _, cacheMocks := newServiceWithOpt(t, func(gitClient *gitmocks.Client, helmClient *helmmocks.Client, paths *iomocks.TempPaths) {
		gitClient.On("Init").Return(nil)
		gitClient.On("IsRevisionPresent", mock.Anything).Return(false)
		gitClient.On("Fetch", mock.Anything).Return(nil)
		gitClient.On("Checkout", mock.Anything, mock.Anything).Once().Return(nil)
		gitClient.On("LsRemote", "HEAD").Return("632039659e542ed7de0c170a4fcc1c571b288fc0", nil)
		gitClient.On("Root").Return(root)
		paths.On("GetPath", mock.Anything).Return(root, nil)
		paths.On("GetPathIfExists", mock.Anything).Return(root, nil)
	}, root)
	s.initConstants.IncludeHiddenDirectories = true
	dirRequest := &apiclient.GitDirectoriesRequest{
		Repo:             &argoappv1.Repository{Repo: "a-url.com"},
		SubmoduleEnabled: false,
		Revision:         "HEAD",
	}
	directories, err := s.GetGitDirectories(context.TODO(), dirRequest)
	require.NoError(t, err)
	assert.ElementsMatch(t, directories.GetPaths(), []string{"app", "app/bar", "app/foo/bar", "somedir", "app/foo", "app/bar/.hidden"})

	// do the same request again to use the cache
	// we only allow CheckOut to be called once in the mock
	directories, err = s.GetGitDirectories(context.TODO(), dirRequest)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"app", "app/bar", "app/foo/bar", "somedir", "app/foo", "app/bar/.hidden"}, directories.GetPaths())
	cacheMocks.mockCache.AssertCacheCalledTimes(t, &repositorymocks.CacheCallCounts{
		ExternalSets: 1,
		ExternalGets: 2,
	})
}

func TestErrorGetGitFiles(t *testing.T) {
	// test not using the cache
	root := ""

	type fields struct {
		service *Service
	}
	type args struct {
		ctx     context.Context
		request *apiclient.GitFilesRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *apiclient.GitFilesResponse
		wantErr assert.ErrorAssertionFunc
	}{
		{name: "InvalidRepo", fields: fields{service: newService(t, ".")}, args: args{
			ctx: context.TODO(),
			request: &apiclient.GitFilesRequest{
				Repo:             nil,
				SubmoduleEnabled: false,
				Revision:         "HEAD",
			},
		}, want: nil, wantErr: assert.Error},
		{name: "InvalidResolveRevision", fields: fields{service: func() *Service {
			s, _, _ := newServiceWithOpt(t, func(gitClient *gitmocks.Client, helmClient *helmmocks.Client, paths *iomocks.TempPaths) {
				gitClient.On("Checkout", mock.Anything, mock.Anything).Return(nil)
				gitClient.On("LsRemote", mock.Anything).Return("", fmt.Errorf("ah error"))
				gitClient.On("Root").Return(root)
				paths.On("GetPath", mock.Anything).Return(".", nil)
				paths.On("GetPathIfExists", mock.Anything).Return(".", nil)
			}, ".")
			return s
		}()}, args: args{
			ctx: context.TODO(),
			request: &apiclient.GitFilesRequest{
				Repo:             &argoappv1.Repository{Repo: "not-a-valid-url"},
				SubmoduleEnabled: false,
				Revision:         "sadfsadf",
			},
		}, want: nil, wantErr: assert.Error},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.fields.service
			got, err := s.GetGitFiles(tt.args.ctx, tt.args.request)
			if !tt.wantErr(t, err, fmt.Sprintf("GetGitFiles(%v, %v)", tt.args.ctx, tt.args.request)) {
				return
			}
			assert.Equalf(t, tt.want, got, "GetGitFiles(%v, %v)", tt.args.ctx, tt.args.request)
		})
	}
}

func TestGetGitFiles(t *testing.T) {
	// test not using the cache
	files := []string{
		"./testdata/git-files-dirs/somedir/config.yaml",
		"./testdata/git-files-dirs/config.yaml", "./testdata/git-files-dirs/config.yaml", "./testdata/git-files-dirs/app/foo/bar/config.yaml",
	}
	root := ""
	s, _, cacheMocks := newServiceWithOpt(t, func(gitClient *gitmocks.Client, helmClient *helmmocks.Client, paths *iomocks.TempPaths) {
		gitClient.On("Init").Return(nil)
		gitClient.On("IsRevisionPresent", mock.Anything).Return(false)
		gitClient.On("Fetch", mock.Anything).Return(nil)
		gitClient.On("Checkout", mock.Anything, mock.Anything).Once().Return(nil)
		gitClient.On("LsRemote", "HEAD").Return("632039659e542ed7de0c170a4fcc1c571b288fc0", nil)
		gitClient.On("Root").Return(root)
		gitClient.On("LsFiles", mock.Anything, mock.Anything).Once().Return(files, nil)
		paths.On("GetPath", mock.Anything).Return(root, nil)
		paths.On("GetPathIfExists", mock.Anything).Return(root, nil)
	}, root)
	filesRequest := &apiclient.GitFilesRequest{
		Repo:             &argoappv1.Repository{Repo: "a-url.com"},
		SubmoduleEnabled: false,
		Revision:         "HEAD",
	}

	// expected map
	expected := make(map[string][]byte)
	for _, filePath := range files {
		fileContents, err := os.ReadFile(filePath)
		require.NoError(t, err)
		expected[filePath] = fileContents
	}

	fileResponse, err := s.GetGitFiles(context.TODO(), filesRequest)
	require.NoError(t, err)
	assert.Equal(t, expected, fileResponse.GetMap())

	// do the same request again to use the cache
	// we only allow LsFiles to be called once in the mock
	fileResponse, err = s.GetGitFiles(context.TODO(), filesRequest)
	require.NoError(t, err)
	assert.Equal(t, expected, fileResponse.GetMap())
	cacheMocks.mockCache.AssertCacheCalledTimes(t, &repositorymocks.CacheCallCounts{
		ExternalSets: 1,
		ExternalGets: 2,
	})
}

func TestErrorUpdateRevisionForPaths(t *testing.T) {
	// test not using the cache
	root := ""

	type fields struct {
		service *Service
	}
	type args struct {
		ctx     context.Context
		request *apiclient.UpdateRevisionForPathsRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *apiclient.UpdateRevisionForPathsResponse
		wantErr assert.ErrorAssertionFunc
	}{
		{name: "InvalidRepo", fields: fields{service: newService(t, ".")}, args: args{
			ctx: context.TODO(),
			request: &apiclient.UpdateRevisionForPathsRequest{
				Repo:           nil,
				Revision:       "HEAD",
				SyncedRevision: "sadfsadf",
			},
		}, want: nil, wantErr: assert.Error},
		{name: "InvalidResolveRevision", fields: fields{service: func() *Service {
			s, _, _ := newServiceWithOpt(t, func(gitClient *gitmocks.Client, helmClient *helmmocks.Client, paths *iomocks.TempPaths) {
				gitClient.On("Checkout", mock.Anything, mock.Anything).Return(nil)
				gitClient.On("LsRemote", mock.Anything).Return("", fmt.Errorf("ah error"))
				gitClient.On("Root").Return(root)
				paths.On("GetPath", mock.Anything).Return(".", nil)
				paths.On("GetPathIfExists", mock.Anything).Return(".", nil)
			}, ".")
			return s
		}()}, args: args{
			ctx: context.TODO(),
			request: &apiclient.UpdateRevisionForPathsRequest{
				Repo:           &argoappv1.Repository{Repo: "not-a-valid-url"},
				Revision:       "sadfsadf",
				SyncedRevision: "HEAD",
				Paths:          []string{"."},
			},
		}, want: nil, wantErr: assert.Error},
		{name: "InvalidResolveSyncedRevision", fields: fields{service: func() *Service {
			s, _, _ := newServiceWithOpt(t, func(gitClient *gitmocks.Client, helmClient *helmmocks.Client, paths *iomocks.TempPaths) {
				gitClient.On("Checkout", mock.Anything, mock.Anything).Return(nil)
				gitClient.On("LsRemote", "HEAD").Once().Return("632039659e542ed7de0c170a4fcc1c571b288fc0", nil)
				gitClient.On("LsRemote", mock.Anything).Return("", fmt.Errorf("ah error"))
				gitClient.On("Root").Return(root)
				paths.On("GetPath", mock.Anything).Return(".", nil)
				paths.On("GetPathIfExists", mock.Anything).Return(".", nil)
			}, ".")
			return s
		}()}, args: args{
			ctx: context.TODO(),
			request: &apiclient.UpdateRevisionForPathsRequest{
				Repo:           &argoappv1.Repository{Repo: "not-a-valid-url"},
				Revision:       "HEAD",
				SyncedRevision: "sadfsadf",
				Paths:          []string{"."},
			},
		}, want: nil, wantErr: assert.Error},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.fields.service
			got, err := s.UpdateRevisionForPaths(tt.args.ctx, tt.args.request)
			if !tt.wantErr(t, err, fmt.Sprintf("UpdateRevisionForPaths(%v, %v)", tt.args.ctx, tt.args.request)) {
				return
			}
			assert.Equalf(t, tt.want, got, "UpdateRevisionForPaths(%v, %v)", tt.args.ctx, tt.args.request)
		})
	}
}

func TestUpdateRevisionForPaths(t *testing.T) {
	type fields struct {
		service *Service
		cache   *repoCacheMocks
	}
	type args struct {
		ctx     context.Context
		request *apiclient.UpdateRevisionForPathsRequest
	}
	type cacheHit struct {
		revision         string
		previousRevision string
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		want     *apiclient.UpdateRevisionForPathsResponse
		wantErr  assert.ErrorAssertionFunc
		cacheHit *cacheHit
	}{
		{name: "NoPathAbort", fields: func() fields {
			s, _, c := newServiceWithOpt(t, func(gitClient *gitmocks.Client, helmClient *helmmocks.Client, paths *iomocks.TempPaths) {
				gitClient.On("Checkout", mock.Anything, mock.Anything).Return(nil)
			}, ".")
			return fields{
				service: s,
				cache:   c,
			}
		}(), args: args{
			ctx: context.TODO(),
			request: &apiclient.UpdateRevisionForPathsRequest{
				Repo:  &argoappv1.Repository{Repo: "a-url.com"},
				Paths: []string{},
			},
		}, want: &apiclient.UpdateRevisionForPathsResponse{}, wantErr: assert.NoError},
		{name: "SameResolvedRevisionAbort", fields: func() fields {
			s, _, c := newServiceWithOpt(t, func(gitClient *gitmocks.Client, helmClient *helmmocks.Client, paths *iomocks.TempPaths) {
				gitClient.On("Checkout", mock.Anything, mock.Anything).Return(nil)
				gitClient.On("LsRemote", "HEAD").Once().Return("632039659e542ed7de0c170a4fcc1c571b288fc0", nil)
				gitClient.On("LsRemote", "SYNCEDHEAD").Once().Return("632039659e542ed7de0c170a4fcc1c571b288fc0", nil)
				paths.On("GetPath", mock.Anything).Return(".", nil)
				paths.On("GetPathIfExists", mock.Anything).Return(".", nil)
			}, ".")
			return fields{
				service: s,
				cache:   c,
			}
		}(), args: args{
			ctx: context.TODO(),
			request: &apiclient.UpdateRevisionForPathsRequest{
				Repo:           &argoappv1.Repository{Repo: "a-url.com"},
				Revision:       "HEAD",
				SyncedRevision: "SYNCEDHEAD",
				Paths:          []string{"."},
			},
		}, want: &apiclient.UpdateRevisionForPathsResponse{
			Revision: "632039659e542ed7de0c170a4fcc1c571b288fc0",
		}, wantErr: assert.NoError},
		{name: "ChangedFilesDoNothing", fields: func() fields {
			s, _, c := newServiceWithOpt(t, func(gitClient *gitmocks.Client, helmClient *helmmocks.Client, paths *iomocks.TempPaths) {
				gitClient.On("Init").Return(nil)
				gitClient.On("IsRevisionPresent", mock.Anything).Return(false)
				gitClient.On("Fetch", mock.Anything).Return(nil)
				gitClient.On("Checkout", mock.Anything, mock.Anything).Return(nil)
				gitClient.On("LsRemote", "HEAD").Once().Return("632039659e542ed7de0c170a4fcc1c571b288fc0", nil)
				gitClient.On("LsRemote", "SYNCEDHEAD").Once().Return("1e67a504d03def3a6a1125d934cb511680f72555", nil)
				paths.On("GetPath", mock.Anything).Return(".", nil)
				paths.On("GetPathIfExists", mock.Anything).Return(".", nil)
				gitClient.On("Root").Return("")
				gitClient.On("ChangedFiles", mock.Anything, mock.Anything).Return([]string{"app.yaml"}, nil)
			}, ".")
			return fields{
				service: s,
				cache:   c,
			}
		}(), args: args{
			ctx: context.TODO(),
			request: &apiclient.UpdateRevisionForPathsRequest{
				Repo:           &argoappv1.Repository{Repo: "a-url.com"},
				Revision:       "HEAD",
				SyncedRevision: "SYNCEDHEAD",
				Paths:          []string{"."},
			},
		}, want: &apiclient.UpdateRevisionForPathsResponse{
			Revision: "632039659e542ed7de0c170a4fcc1c571b288fc0",
			Changes:  true,
		}, wantErr: assert.NoError},
		{name: "NoChangesUpdateCache", fields: func() fields {
			s, _, c := newServiceWithOpt(t, func(gitClient *gitmocks.Client, helmClient *helmmocks.Client, paths *iomocks.TempPaths) {
				gitClient.On("Init").Return(nil)
				gitClient.On("IsRevisionPresent", mock.Anything).Return(false)
				gitClient.On("Fetch", mock.Anything).Return(nil)
				gitClient.On("Checkout", mock.Anything, mock.Anything).Return(nil)
				gitClient.On("LsRemote", "HEAD").Once().Return("632039659e542ed7de0c170a4fcc1c571b288fc0", nil)
				gitClient.On("LsRemote", "SYNCEDHEAD").Once().Return("1e67a504d03def3a6a1125d934cb511680f72555", nil)
				paths.On("GetPath", mock.Anything).Return(".", nil)
				paths.On("GetPathIfExists", mock.Anything).Return(".", nil)
				gitClient.On("Root").Return("")
				gitClient.On("ChangedFiles", mock.Anything, mock.Anything).Return([]string{}, nil)
			}, ".")
			return fields{
				service: s,
				cache:   c,
			}
		}(), args: args{
			ctx: context.TODO(),
			request: &apiclient.UpdateRevisionForPathsRequest{
				Repo:           &argoappv1.Repository{Repo: "a-url.com"},
				Revision:       "HEAD",
				SyncedRevision: "SYNCEDHEAD",
				Paths:          []string{"."},

				AppLabelKey:       "app.kubernetes.io/name",
				AppName:           "no-change-update-cache",
				Namespace:         "default",
				TrackingMethod:    "annotation+label",
				ApplicationSource: &argoappv1.ApplicationSource{Path: "."},
				KubeVersion:       "v1.16.0",
			},
		}, want: &apiclient.UpdateRevisionForPathsResponse{
			Revision: "632039659e542ed7de0c170a4fcc1c571b288fc0",
		}, wantErr: assert.NoError, cacheHit: &cacheHit{
			previousRevision: "1e67a504d03def3a6a1125d934cb511680f72555",
			revision:         "632039659e542ed7de0c170a4fcc1c571b288fc0",
		}},
		{name: "NoChangesHelmMultiSourceUpdateCache", fields: func() fields {
			s, _, c := newServiceWithOpt(t, func(gitClient *gitmocks.Client, helmClient *helmmocks.Client, paths *iomocks.TempPaths) {
				gitClient.On("Init").Return(nil)
				gitClient.On("IsRevisionPresent", mock.Anything).Return(false)
				gitClient.On("Fetch", mock.Anything).Return(nil)
				gitClient.On("Checkout", mock.Anything, mock.Anything).Return(nil)
				gitClient.On("LsRemote", "HEAD").Once().Return("632039659e542ed7de0c170a4fcc1c571b288fc0", nil)
				gitClient.On("LsRemote", "SYNCEDHEAD").Once().Return("1e67a504d03def3a6a1125d934cb511680f72555", nil)
				paths.On("GetPath", mock.Anything).Return(".", nil)
				paths.On("GetPathIfExists", mock.Anything).Return(".", nil)
				gitClient.On("Root").Return("")
				gitClient.On("ChangedFiles", mock.Anything, mock.Anything).Return([]string{}, nil)
			}, ".")
			return fields{
				service: s,
				cache:   c,
			}
		}(), args: args{
			ctx: context.TODO(),
			request: &apiclient.UpdateRevisionForPathsRequest{
				Repo:           &argoappv1.Repository{Repo: "a-url.com"},
				Revision:       "HEAD",
				SyncedRevision: "SYNCEDHEAD",
				Paths:          []string{"."},

				AppLabelKey:       "app.kubernetes.io/name",
				AppName:           "no-change-update-cache",
				Namespace:         "default",
				TrackingMethod:    "annotation+label",
				ApplicationSource: &argoappv1.ApplicationSource{Path: ".", Helm: &argoappv1.ApplicationSourceHelm{ReleaseName: "test"}},
				KubeVersion:       "v1.16.0",

				HasMultipleSources: true,
			},
		}, want: &apiclient.UpdateRevisionForPathsResponse{
			Revision: "632039659e542ed7de0c170a4fcc1c571b288fc0",
		}, wantErr: assert.NoError, cacheHit: &cacheHit{
			previousRevision: "1e67a504d03def3a6a1125d934cb511680f72555",
			revision:         "632039659e542ed7de0c170a4fcc1c571b288fc0",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.fields.service
			cache := tt.fields.cache

			if tt.cacheHit != nil {
				cache.mockCache.On("Rename", tt.cacheHit.previousRevision, tt.cacheHit.revision, mock.Anything).Return(nil)
			}

			got, err := s.UpdateRevisionForPaths(tt.args.ctx, tt.args.request)
			if !tt.wantErr(t, err, fmt.Sprintf("UpdateRevisionForPaths(%v, %v)", tt.args.ctx, tt.args.request)) {
				return
			}
			assert.Equalf(t, tt.want, got, "UpdateRevisionForPaths(%v, %v)", tt.args.ctx, tt.args.request)

			if tt.cacheHit != nil {
				cache.mockCache.AssertCacheCalledTimes(t, &repositorymocks.CacheCallCounts{
					ExternalRenames: 1,
				})
			} else {
				cache.mockCache.AssertCacheCalledTimes(t, &repositorymocks.CacheCallCounts{
					ExternalRenames: 0,
				})
			}
		})
	}
}

func Test_getRepoSanitizerRegex(t *testing.T) {
	r := getRepoSanitizerRegex("/tmp/_argocd-repo")
	msg := r.ReplaceAllString("error message containing /tmp/_argocd-repo/SENSITIVE and other stuff", "<path to cached source>")
	assert.Equal(t, "error message containing <path to cached source> and other stuff", msg)
	msg = r.ReplaceAllString("error message containing /tmp/_argocd-repo/SENSITIVE/with/trailing/path and other stuff", "<path to cached source>")
	assert.Equal(t, "error message containing <path to cached source>/with/trailing/path and other stuff", msg)
}

func TestGetRefs_CacheWithLockDisabled(t *testing.T) {
	// Test that when the lock is disabled the default behavior still works correctly
	// Also shows the current issue with the git requests due to cache misses
	dir := t.TempDir()
	initGitRepo(t, newGitRepoOptions{
		path:           dir,
		createPath:     false,
		remote:         "",
		addEmptyCommit: true,
	})
	// Test in-memory and redis
	cacheMocks := newCacheMocksWithOpts(1*time.Minute, 1*time.Minute, 0)
	t.Cleanup(cacheMocks.mockCache.StopRedisCallback)
	var wg sync.WaitGroup
	numberOfCallers := 10
	for i := 0; i < numberOfCallers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client, err := git.NewClient(fmt.Sprintf("file://%s", dir), git.NopCreds{}, true, false, "", "", git.WithCache(cacheMocks.cache, true))
			require.NoError(t, err)
			refs, err := client.LsRefs()
			require.NoError(t, err)
			assert.NotNil(t, refs)
			assert.NotEmpty(t, refs.Branches, "Expected branches to be populated")
			assert.NotEmpty(t, refs.Branches[0])
		}()
	}
	wg.Wait()
	// Unlock should not have been called
	cacheMocks.mockCache.AssertNumberOfCalls(t, "UnlockGitReferences", 0)
	// Lock should not have been called
	cacheMocks.mockCache.AssertNumberOfCalls(t, "TryLockGitRefCache", 0)
}

func TestGetRefs_CacheDisabled(t *testing.T) {
	// Test that default get refs with cache disabled does not call GetOrLockGitReferences
	dir := t.TempDir()
	initGitRepo(t, newGitRepoOptions{
		path:           dir,
		createPath:     false,
		remote:         "",
		addEmptyCommit: true,
	})
	cacheMocks := newCacheMocks()
	t.Cleanup(cacheMocks.mockCache.StopRedisCallback)
	client, err := git.NewClient(fmt.Sprintf("file://%s", dir), git.NopCreds{}, true, false, "", "", git.WithCache(cacheMocks.cache, false))
	require.NoError(t, err)
	refs, err := client.LsRefs()
	require.NoError(t, err)
	assert.NotNil(t, refs)
	assert.NotEmpty(t, refs.Branches, "Expected branches to be populated")
	assert.NotEmpty(t, refs.Branches[0])
	// Unlock should not have been called
	cacheMocks.mockCache.AssertNumberOfCalls(t, "UnlockGitReferences", 0)
	cacheMocks.mockCache.AssertNumberOfCalls(t, "GetOrLockGitReferences", 0)
}

func TestGetRefs_CacheWithLock(t *testing.T) {
	// Test that there is only one call to SetGitReferences for the same repo which is done after the ls-remote
	dir := t.TempDir()
	initGitRepo(t, newGitRepoOptions{
		path:           dir,
		createPath:     false,
		remote:         "",
		addEmptyCommit: true,
	})
	cacheMocks := newCacheMocks()
	t.Cleanup(cacheMocks.mockCache.StopRedisCallback)
	var wg sync.WaitGroup
	numberOfCallers := 10
	for i := 0; i < numberOfCallers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client, err := git.NewClient(fmt.Sprintf("file://%s", dir), git.NopCreds{}, true, false, "", "", git.WithCache(cacheMocks.cache, true))
			require.NoError(t, err)
			refs, err := client.LsRefs()
			require.NoError(t, err)
			assert.NotNil(t, refs)
			assert.NotEmpty(t, refs.Branches, "Expected branches to be populated")
			assert.NotEmpty(t, refs.Branches[0])
		}()
	}
	wg.Wait()
	// Unlock should not have been called
	cacheMocks.mockCache.AssertNumberOfCalls(t, "UnlockGitReferences", 0)
	cacheMocks.mockCache.AssertNumberOfCalls(t, "GetOrLockGitReferences", 0)
}

func TestGetRefs_CacheUnlockedOnUpdateFailed(t *testing.T) {
	// Worst case the ttl on the lock expires and the lock is removed
	// however if the holder of the lock fails to update the cache the caller should remove the lock
	// to allow other callers to attempt to update the cache as quickly as possible
	dir := t.TempDir()
	initGitRepo(t, newGitRepoOptions{
		path:           dir,
		createPath:     false,
		remote:         "",
		addEmptyCommit: true,
	})
	cacheMocks := newCacheMocks()
	t.Cleanup(cacheMocks.mockCache.StopRedisCallback)
	repoUrl := fmt.Sprintf("file://%s", dir)
	client, err := git.NewClient(repoUrl, git.NopCreds{}, true, false, "", "", git.WithCache(cacheMocks.cache, true))
	require.NoError(t, err)
	refs, err := client.LsRefs()
	require.NoError(t, err)
	assert.NotNil(t, refs)
	assert.NotEmpty(t, refs.Branches, "Expected branches to be populated")
	assert.NotEmpty(t, refs.Branches[0])
	var output [][2]string
	err = cacheMocks.cacheutilCache.GetItem(fmt.Sprintf("git-refs|%s|%s", repoUrl, common.CacheVersion), &output)
	require.Error(t, err, "Should be a cache miss")
	assert.Empty(t, output, "Expected cache to be empty for key")
	cacheMocks.mockCache.AssertNumberOfCalls(t, "UnlockGitReferences", 0)
	cacheMocks.mockCache.AssertNumberOfCalls(t, "GetOrLockGitReferences", 0)
}

func TestGetRefs_CacheLockTryLockGitRefCacheError(t *testing.T) {
	// Worst case the ttl on the lock expires and the lock is removed
	// however if the holder of the lock fails to update the cache the caller should remove the lock
	// to allow other callers to attempt to update the cache as quickly as possible
	dir := t.TempDir()
	initGitRepo(t, newGitRepoOptions{
		path:           dir,
		createPath:     false,
		remote:         "",
		addEmptyCommit: true,
	})
	cacheMocks := newCacheMocks()
	t.Cleanup(cacheMocks.mockCache.StopRedisCallback)
	repoUrl := fmt.Sprintf("file://%s", dir)
	// buf := bytes.Buffer{}
	// log.SetOutput(&buf)
	client, err := git.NewClient(repoUrl, git.NopCreds{}, true, false, "", "", git.WithCache(cacheMocks.cache, true))
	require.NoError(t, err)
	refs, err := client.LsRefs()
	require.NoError(t, err)
	assert.NotNil(t, refs)
}

func TestGetRevisionChartDetails(t *testing.T) {
	t.Run("Test revision semvar", func(t *testing.T) {
		root := t.TempDir()
		service := newService(t, root)
		_, err := service.GetRevisionChartDetails(context.Background(), &apiclient.RepoServerRevisionChartDetailsRequest{
			Repo: &v1alpha1.Repository{
				Repo: fmt.Sprintf("file://%s", root),
				Name: "test-repo-name",
				Type: "helm",
			},
			Name:     "test-name",
			Revision: "test-revision",
		})
		assert.ErrorContains(t, err, "invalid revision")
	})

	t.Run("Test GetRevisionChartDetails", func(t *testing.T) {
		root := t.TempDir()
		service := newService(t, root)
		repoUrl := fmt.Sprintf("file://%s", root)
		err := service.cache.SetRevisionChartDetails(repoUrl, "my-chart", "1.1.0", &argoappv1.ChartDetails{
			Description: "test-description",
			Home:        "test-home",
			Maintainers: []string{"test-maintainer"},
		})
		require.NoError(t, err)
		chartDetails, err := service.GetRevisionChartDetails(context.Background(), &apiclient.RepoServerRevisionChartDetailsRequest{
			Repo: &v1alpha1.Repository{
				Repo: fmt.Sprintf("file://%s", root),
				Name: "test-repo-name",
				Type: "helm",
			},
			Name:     "my-chart",
			Revision: "1.1.0",
		})
		require.NoError(t, err)
		assert.Equal(t, "test-description", chartDetails.Description)
		assert.Equal(t, "test-home", chartDetails.Home)
		assert.Equal(t, []string{"test-maintainer"}, chartDetails.Maintainers)
	})
}

func TestVerifyCommitSignature(t *testing.T) {
	repo := &v1alpha1.Repository{
		Repo: "https://github.com/example/repo.git",
	}

	t.Run("VerifyCommitSignature with valid signature", func(t *testing.T) {
		t.Setenv("ARGOCD_GPG_ENABLED", "true")
		mockGitClient := &gitmocks.Client{}
		mockGitClient.On("VerifyCommitSignature", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(testSignature, nil)

		err := verifyCommitSignature(true, mockGitClient, "abcd1234", repo)
		require.NoError(t, err)
	})

	t.Run("VerifyCommitSignature with invalid signature", func(t *testing.T) {
		t.Setenv("ARGOCD_GPG_ENABLED", "true")
		mockGitClient := &gitmocks.Client{}
		mockGitClient.On("VerifyCommitSignature", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return("", nil)

		err := verifyCommitSignature(true, mockGitClient, "abcd1234", repo)
		require.Error(t, err)
		assert.Equal(t, "revision abcd1234 is not signed", err.Error())
	})

	t.Run("VerifyCommitSignature with unknown signature", func(t *testing.T) {
		t.Setenv("ARGOCD_GPG_ENABLED", "true")
		mockGitClient := &gitmocks.Client{}
		mockGitClient.On("VerifyCommitSignature", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return("", fmt.Errorf("UNKNOWN signature: gpg: Unknown signature from ABCDEFGH"))

		err := verifyCommitSignature(true, mockGitClient, "abcd1234", repo)
		require.Error(t, err)
		assert.Equal(t, "UNKNOWN signature: gpg: Unknown signature from ABCDEFGH", err.Error())
	})

	t.Run("VerifyCommitSignature with error verifying signature", func(t *testing.T) {
		t.Setenv("ARGOCD_GPG_ENABLED", "true")
		mockGitClient := &gitmocks.Client{}
		mockGitClient.On("VerifyCommitSignature", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return("", fmt.Errorf("error verifying signature of commit 'abcd1234' in repo 'https://github.com/example/repo.git': failed to verify signature"))

		err := verifyCommitSignature(true, mockGitClient, "abcd1234", repo)
		require.Error(t, err)
		assert.Equal(t, "error verifying signature of commit 'abcd1234' in repo 'https://github.com/example/repo.git': failed to verify signature", err.Error())
	})

	t.Run("VerifyCommitSignature with signature verification disabled", func(t *testing.T) {
		t.Setenv("ARGOCD_GPG_ENABLED", "false")
		mockGitClient := &gitmocks.Client{}
		err := verifyCommitSignature(false, mockGitClient, "abcd1234", repo)
		require.NoError(t, err)
	})
}

func Test_GenerateManifests_Commands(t *testing.T) {
	t.Run("helm", func(t *testing.T) {
		service := newService(t, "testdata/my-chart")

		// Fill the manifest request with as many parameters affecting Helm commands as possible.
		q := apiclient.ManifestRequest{
			AppName:     "test-app",
			Namespace:   "test-namespace",
			KubeVersion: "1.2.3",
			ApiVersions: []string{"v1/Test", "v2/Test"},
			Repo:        &argoappv1.Repository{},
			ApplicationSource: &argoappv1.ApplicationSource{
				Path: ".",
				Helm: &argoappv1.ApplicationSourceHelm{
					FileParameters: []argoappv1.HelmFileParameter{
						{
							Name: "test-file-param-name",
							Path: "test-file-param.yaml",
						},
					},
					Parameters: []argoappv1.HelmParameter{
						{
							Name: "test-param-name",
							// Use build env var to test substitution.
							Value:       "test-value-$ARGOCD_APP_NAME",
							ForceString: true,
						},
						{
							Name: "test-param-bool-name",
							// Use build env var to test substitution.
							Value: "false",
						},
					},
					PassCredentials: true,
					SkipCrds:        true,
					ValueFiles: []string{
						"my-chart-values.yaml",
					},
					Values: "test: values",
				},
			},
			ProjectName:        "something",
			ProjectSourceRepos: []string{"*"},
		}

		res, err := service.GenerateManifest(context.Background(), &q)

		require.NoError(t, err)
		assert.Equal(t, []string{"helm template . --name-template test-app --namespace test-namespace --kube-version 1.2.3 --set test-param-bool-name=false --set-string test-param-name=test-value-test-app --set-file test-file-param-name=./test-file-param.yaml --values ./my-chart-values.yaml --values <temp file with values from source.helm.values/valuesObject> --api-versions v1/Test --api-versions v2/Test"}, res.Commands)

		t.Run("with overrides", func(t *testing.T) {
			// These can be set explicitly instead of using inferred values. Make sure the overrides apply.
			q.ApplicationSource.Helm.APIVersions = []string{"v3", "v4"}
			q.ApplicationSource.Helm.KubeVersion = "5.6.7"
			q.ApplicationSource.Helm.Namespace = "different-namespace"
			q.ApplicationSource.Helm.ReleaseName = "different-release-name"

			res, err = service.GenerateManifest(context.Background(), &q)

			require.NoError(t, err)
			assert.Equal(t, []string{"helm template . --name-template different-release-name --namespace different-namespace --kube-version 5.6.7 --set test-param-bool-name=false --set-string test-param-name=test-value-test-app --set-file test-file-param-name=./test-file-param.yaml --values ./my-chart-values.yaml --values <temp file with values from source.helm.values/valuesObject> --api-versions v3 --api-versions v4"}, res.Commands)
		})
	})

	t.Run("helm with dependencies", func(t *testing.T) {
		// This test makes sure we still get commands, even if we hit the code path that has to run "helm dependency build."
		// We don't actually return the "helm dependency build" command, because we expect that the user is able to read
		// the "helm template" and figure out how to fix it.
		t.Cleanup(func() {
			err := os.Remove("testdata/helm-with-local-dependency/Chart.lock")
			require.NoError(t, err)
			err = os.RemoveAll("testdata/helm-with-local-dependency/charts")
			require.NoError(t, err)
		})

		service := newService(t, "testdata/helm-with-local-dependency")

		q := apiclient.ManifestRequest{
			AppName:   "test-app",
			Namespace: "test-namespace",
			Repo:      &argoappv1.Repository{},
			ApplicationSource: &argoappv1.ApplicationSource{
				Path: ".",
			},
			ProjectName:        "something",
			ProjectSourceRepos: []string{"*"},
		}

		res, err := service.GenerateManifest(context.Background(), &q)

		require.NoError(t, err)
		assert.Equal(t, []string{"helm template . --name-template test-app --namespace test-namespace --include-crds"}, res.Commands)
	})

	t.Run("kustomize", func(t *testing.T) {
		// Write test files to a temp dir, because the test mutates kustomization.yaml in place.
		tempDir := t.TempDir()
		err := os.WriteFile(path.Join(tempDir, "kustomization.yaml"), []byte(`
resources:
- guestbook.yaml
`), os.FileMode(0o600))
		require.NoError(t, err)
		err = os.WriteFile(path.Join(tempDir, "guestbook.yaml"), []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: guestbook-ui
`), os.FileMode(0o400))
		require.NoError(t, err)
		err = os.Mkdir(path.Join(tempDir, "component"), os.FileMode(0o700))
		require.NoError(t, err)
		err = os.WriteFile(path.Join(tempDir, "component", "kustomization.yaml"), []byte(`
apiVersion: kustomize.config.k8s.io/v1alpha1
kind: Component
images:
- name: old
  newName: new
`), os.FileMode(0o400))
		require.NoError(t, err)

		service := newService(t, tempDir)

		// Fill the manifest request with as many parameters affecting Helm commands as possible.
		q := apiclient.ManifestRequest{
			AppName:     "test-app",
			Namespace:   "test-namespace",
			KubeVersion: "1.2.3",
			ApiVersions: []string{"v1/Test", "v2/Test"},
			Repo:        &argoappv1.Repository{},
			ApplicationSource: &argoappv1.ApplicationSource{
				Path: ".",
				Kustomize: &argoappv1.ApplicationSourceKustomize{
					APIVersions: []string{"v1", "v2"},
					CommonAnnotations: map[string]string{
						// Use build env var to test substitution.
						"test": "annotation-$ARGOCD_APP_NAME",
					},
					CommonAnnotationsEnvsubst: true,
					CommonLabels: map[string]string{
						"test": "label",
					},
					Components:             []string{"component"},
					ForceCommonAnnotations: true,
					ForceCommonLabels:      true,
					Images: argoappv1.KustomizeImages{
						"image=override",
					},
					KubeVersion:          "5.6.7",
					LabelWithoutSelector: true,
					NamePrefix:           "test-prefix",
					NameSuffix:           "test-suffix",
					Namespace:            "override-namespace",
					Replicas: argoappv1.KustomizeReplicas{
						{
							Name:  "guestbook-ui",
							Count: intstr.Parse("1337"),
						},
					},
				},
			},
			ProjectName:        "something",
			ProjectSourceRepos: []string{"*"},
		}

		res, err := service.GenerateManifest(context.Background(), &q)

		require.NoError(t, err)
		assert.Equal(t, []string{
			"kustomize edit set nameprefix -- test-prefix",
			"kustomize edit set namesuffix -- test-suffix",
			"kustomize edit set image image=override",
			"kustomize edit set replicas guestbook-ui=1337",
			"kustomize edit add label --force --without-selector test:label",
			"kustomize edit add annotation --force test:annotation-test-app",
			"kustomize edit set namespace -- override-namespace",
			"kustomize edit add component component",
			"kustomize build .",
		}, res.Commands)
	})
}
