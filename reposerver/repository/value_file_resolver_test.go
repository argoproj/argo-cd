package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/git"
	gitmocks "github.com/argoproj/argo-cd/v3/util/git/mocks"
	helmmocks "github.com/argoproj/argo-cd/v3/util/helm/mocks"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	iomocks "github.com/argoproj/argo-cd/v3/util/io/mocks"
	"github.com/argoproj/argo-cd/v3/util/oci"
	ocimocks "github.com/argoproj/argo-cd/v3/util/oci/mocks"
)

// TestResolveReferencedSources_RejectsChartOnRefSource is a regression test for the guard
// that rejects a 'chart' field on ref sources. The 'chart' field is not incorporated into
// ref resolution (which keys off the repository URL only), so accepting it - for Git or OCI -
// would silently ignore it and, for the Helm-OCI repoURL+chart pattern, extract the wrong
// artifact. Both schemes must be rejected.
func TestResolveReferencedSources_RejectsChartOnRefSource(t *testing.T) {
	helmSource := &v1alpha1.ApplicationSourceHelm{ValueFiles: []string{"$ref/values.yaml"}}

	tests := []struct {
		name    string
		refRepo v1alpha1.Repository
	}{
		{name: "git ref source with chart", refRepo: v1alpha1.Repository{Repo: "https://git.example.com/org/repo.git"}},
		{name: "oci ref source with chart", refRepo: v1alpha1.Repository{Repo: "oci://registry.example.com/charts"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refSources := map[string]*v1alpha1.RefTarget{
				"$ref": {Repo: tt.refRepo, Chart: "my-chart", TargetRevision: "1.0.0"},
			}
			// The guard rejects before any client getter is invoked, so an empty resolver is safe.
			_, err := resolveReferencedSources(t.Context(), refSourceResolver{hasMultipleSources: true, helmSource: helmSource, refSources: refSources})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "'chart' field defined")
		})
	}
}

// TestResolveReferencedSources_AllowsOCIRefWithoutChart proves the rejection above is
// specific to the 'chart' field: an OCI ref source without a chart resolves normally.
func TestResolveReferencedSources_AllowsOCIRefWithoutChart(t *testing.T) {
	helmSource := &v1alpha1.ApplicationSourceHelm{ValueFiles: []string{"$ref/values.yaml"}}
	refSources := map[string]*v1alpha1.RefTarget{
		"$ref": {Repo: v1alpha1.Repository{Repo: "oci://registry.example.com/charts"}, TargetRevision: "1.0.0"},
	}
	ociGetter := func(_ context.Context, _ *v1alpha1.Repository, _ string, _ bool) (oci.Client, string, error) {
		return nil, "sha256:deadbeef", nil
	}

	repoRefs, err := resolveReferencedSources(t.Context(), refSourceResolver{hasMultipleSources: true, helmSource: helmSource, refSources: refSources, newOCIClientResolveRevision: ociGetter})
	require.NoError(t, err)
	assert.Equal(t, "sha256:deadbeef", repoRefs[v1alpha1.NormalizeOCIURL("oci://registry.example.com/charts")])
}

// TestResolveReferencedSources_RejectsConflictingRevisionsForSameRepo is a regression test: two
// $refs pointing at one repository at revisions that resolve to different commits must be rejected
// before the cache lookup. repoRefs is keyed by repository URL only, so the second ref used to be
// silently deduplicated onto the first ref's resolved revision, hiding the conflict from the cache
// key and from the later check in runManifestGenAsync. Revisions spelled differently but resolving
// to the same commit (a branch and its current SHA) are accepted.
func TestResolveReferencedSources_RejectsConflictingRevisionsForSameRepo(t *testing.T) {
	helmSource := &v1alpha1.ApplicationSourceHelm{ValueFiles: []string{"$a/values.yaml", "$b/values.yaml"}}

	tests := []struct {
		name string
		repo v1alpha1.Repository
	}{
		{name: "git", repo: v1alpha1.Repository{Repo: "https://git.example.com/org/repo.git"}},
		// Differ only in scheme/host case: NormalizeRepoURL must still treat them as one repository.
		{name: "oci", repo: v1alpha1.Repository{Repo: "oci://Registry.Example.com/values"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolveCalls := 0
			resolver := refSourceResolver{
				hasMultipleSources: true,
				helmSource:         helmSource,
				newClientResolveRevision: func(_ *v1alpha1.Repository, revision string, _ ...git.ClientOpts) (git.Client, string, error) {
					resolveCalls++
					return nil, "sha-for-" + revision, nil
				},
				newOCIClientResolveRevision: func(_ context.Context, _ *v1alpha1.Repository, revision string, _ bool) (oci.Client, string, error) {
					resolveCalls++
					return nil, "sha256:" + revision, nil
				},
			}

			t.Run("different revisions are rejected", func(t *testing.T) {
				resolveCalls = 0
				refSources := map[string]*v1alpha1.RefTarget{
					"$a": {Repo: tt.repo, TargetRevision: "v1.0.0"},
					"$b": {Repo: v1alpha1.Repository{Repo: strings.ToLower(tt.repo.Repo)}, TargetRevision: "v2.0.0"},
				}
				resolver.refSources = refSources
				_, err := resolveReferencedSources(t.Context(), resolver)
				require.ErrorContains(t, err, "cannot reference multiple revisions for the same repository")
				assert.Contains(t, err.Error(), `$b references "v2.0.0" which resolves to`)
				assert.Contains(t, err.Error(), `while $a references "v1.0.0" which resolves to`)
				assert.Equal(t, 2, resolveCalls)
			})

			t.Run("different spellings of the same commit are accepted", func(t *testing.T) {
				resolveCalls = 0
				resolver.newClientResolveRevision = func(_ *v1alpha1.Repository, _ string, _ ...git.ClientOpts) (git.Client, string, error) {
					resolveCalls++
					return nil, "sha-for-main", nil
				}
				resolver.newOCIClientResolveRevision = func(_ context.Context, _ *v1alpha1.Repository, _ string, _ bool) (oci.Client, string, error) {
					resolveCalls++
					return nil, "sha256:main", nil
				}
				refSources := map[string]*v1alpha1.RefTarget{
					"$a": {Repo: tt.repo, TargetRevision: "main"},
					"$b": {Repo: v1alpha1.Repository{Repo: strings.ToLower(tt.repo.Repo)}, TargetRevision: "sha-of-main"},
				}
				resolver.refSources = refSources
				repoRefs, err := resolveReferencedSources(t.Context(), resolver)
				require.NoError(t, err)
				assert.Len(t, repoRefs, 1)
				assert.Equal(t, 2, resolveCalls)
			})

			t.Run("same revision is resolved once", func(t *testing.T) {
				resolveCalls = 0
				refSources := map[string]*v1alpha1.RefTarget{
					"$a": {Repo: tt.repo, TargetRevision: "v1.0.0"},
					"$b": {Repo: v1alpha1.Repository{Repo: strings.ToLower(tt.repo.Repo)}, TargetRevision: "v1.0.0"},
				}
				resolver.refSources = refSources
				repoRefs, err := resolveReferencedSources(t.Context(), resolver)
				require.NoError(t, err)
				assert.Len(t, repoRefs, 1)
				assert.Equal(t, 1, resolveCalls)
			})
		})
	}
}

// TestRedactPaths_RedactsGitAndOCIPaths is a regression test ensuring value files resolved
// from a $ref OCI source (extracted under ociPaths) are redacted from the returned helm
// template command, not just Git checkout paths. Otherwise reposerver filesystem paths leak
// into ManifestResponse.Commands.
func TestRedactPaths_RedactsGitAndOCIPaths(t *testing.T) {
	gitDir := t.TempDir()
	ociDir := t.TempDir()
	gitPaths := utilio.NewRandomizedTempPaths(t.TempDir())
	gitPaths.Add("git-key", gitDir)
	ociPaths := utilio.NewRandomizedTempPaths(t.TempDir())
	ociPaths.Add("oci-key", ociDir)

	cmd := fmt.Sprintf("helm template . --values %s/values.yaml --values %s/oci-values.yaml", gitDir, ociDir)
	got := redactPaths(cmd, "", gitPaths, ociPaths)

	assert.NotContains(t, got, gitDir)
	assert.NotContains(t, got, ociDir)
	assert.Equal(t, "helm template . --values ./values.yaml --values ./oci-values.yaml", got)
}

// TestRedactPathsInError_RedactsOCIPath ensures helm errors (which embed the rendered command,
// including OCI-extracted value file paths) are redacted before being returned.
func TestRedactPathsInError_RedactsOCIPath(t *testing.T) {
	ociDir := t.TempDir()
	ociPaths := utilio.NewRandomizedTempPaths(t.TempDir())
	ociPaths.Add("oci-key", ociDir)

	require.NoError(t, redactPathsInError(nil, "", ociPaths))

	err := fmt.Errorf("failed to render: open %s/oci-values.yaml: no such file", ociDir)
	got := redactPathsInError(err, "", ociPaths)
	require.Error(t, got)
	assert.NotContains(t, got.Error(), ociDir)
	assert.Contains(t, got.Error(), "./oci-values.yaml")

	// The message is redacted, but the original error identity is preserved so callers can
	// still match sentinels/types via errors.Is/errors.As (e.g. context cancellation).
	sentinel := fmt.Errorf("open %s/oci-values.yaml: %w", ociDir, context.Canceled)
	wrapped := redactPathsInError(sentinel, "", ociPaths)
	assert.NotContains(t, wrapped.Error(), ociDir)
	assert.ErrorIs(t, wrapped, context.Canceled)
}

func Test_valueFileResolver_ResolveValueFiles(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	paths := utilio.NewRandomizedTempPaths(tempDir)

	paths.Add(git.NormalizeGitURL("https://github.com/org/repo1"), path.Join(tempDir, "repo1"))
	// Extracted OCI content is registered under the normalized URL, so a ref whose URL differs
	// only in case must still resolve to it.
	paths.Add(v1alpha1.NormalizeOCIURL("oci://Registry.Example.com/chart"), path.Join(tempDir, "oci-chart"))
	ociRef := map[string]*v1alpha1.RefTarget{
		"$oci": {Repo: v1alpha1.Repository{Repo: "oci://REGISTRY.EXAMPLE.COM/chart"}},
	}

	testCases := []struct {
		name         string
		rawPath      string
		env          *v1alpha1.Env
		refSources   map[string]*v1alpha1.RefTarget
		expectedPath string
		expectedErr  bool
	}{
		{
			name:         "simple path",
			rawPath:      "values.yaml",
			env:          &v1alpha1.Env{},
			refSources:   map[string]*v1alpha1.RefTarget{},
			expectedPath: path.Join(tempDir, "main-repo", "values.yaml"),
		},
		{
			name:    "simple ref",
			rawPath: "$ref/values.yaml",
			env:     &v1alpha1.Env{},
			refSources: map[string]*v1alpha1.RefTarget{
				"$ref": {
					Repo: v1alpha1.Repository{
						Repo: "https://github.com/org/repo1",
					},
				},
			},
			expectedPath: path.Join(tempDir, "repo1", "values.yaml"),
		},
		{
			name:    "only ref",
			rawPath: "$ref",
			env:     &v1alpha1.Env{},
			refSources: map[string]*v1alpha1.RefTarget{
				"$ref": {
					Repo: v1alpha1.Repository{
						Repo: "https://github.com/org/repo1",
					},
				},
			},
			expectedErr: true,
		},
		{
			name:    "attempted traversal",
			rawPath: "$ref/../values.yaml",
			env:     &v1alpha1.Env{},
			refSources: map[string]*v1alpha1.RefTarget{
				"$ref": {
					Repo: v1alpha1.Repository{
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
			env:          &v1alpha1.Env{},
			refSources:   map[string]*v1alpha1.RefTarget{},
			expectedPath: path.Join(tempDir, "main-repo", "values.yaml"),
		},
		{
			name:    "repo doesn't exist",
			rawPath: "$ref/values.yaml",
			env:     &v1alpha1.Env{},
			refSources: map[string]*v1alpha1.RefTarget{
				"$ref": {
					Repo: v1alpha1.Repository{
						Repo: "https://github.com/org/repo2",
					},
				},
			},
			expectedErr: true,
		},
		{
			name:    "env var is resolved",
			rawPath: "$ref/$APP_PATH/values.yaml",
			env: &v1alpha1.Env{
				&v1alpha1.EnvEntry{
					Name:  "APP_PATH",
					Value: "app-path",
				},
			},
			refSources: map[string]*v1alpha1.RefTarget{
				"$ref": {
					Repo: v1alpha1.Repository{
						Repo: "https://github.com/org/repo1",
					},
				},
			},
			expectedPath: path.Join(tempDir, "repo1", "app-path", "values.yaml"),
		},
		{
			name:    "traversal in env var is blocked",
			rawPath: "$ref/$APP_PATH/values.yaml",
			env: &v1alpha1.Env{
				&v1alpha1.EnvEntry{
					Name:  "APP_PATH",
					Value: "..",
				},
			},
			refSources: map[string]*v1alpha1.RefTarget{
				"$ref": {
					Repo: v1alpha1.Repository{
						Repo: "https://github.com/org/repo1",
					},
				},
			},
			expectedErr: true,
		},
		{
			name:    "env var prefix",
			rawPath: "$APP_PATH/values.yaml",
			env: &v1alpha1.Env{
				&v1alpha1.EnvEntry{
					Name:  "APP_PATH",
					Value: "app-path",
				},
			},
			refSources:   map[string]*v1alpha1.RefTarget{},
			expectedPath: path.Join(tempDir, "main-repo", "app-path", "values.yaml"),
		},
		{
			name:         "unresolved env var",
			rawPath:      "$APP_PATH/values.yaml",
			env:          &v1alpha1.Env{},
			refSources:   map[string]*v1alpha1.RefTarget{},
			expectedPath: path.Join(tempDir, "main-repo", "values.yaml"),
		},
		{
			name:         "oci ref",
			rawPath:      "$oci/values.yaml",
			env:          &v1alpha1.Env{},
			refSources:   ociRef,
			expectedPath: path.Join(tempDir, "oci-chart", "values.yaml"),
		},
		{
			// A URL-looking path stays inside the artifact instead of becoming a remote download,
			// matching how Git $ref paths behave.
			name:         "oci ref url path stays inside artifact",
			rawPath:      "$oci/https://example.com/values.yaml",
			env:          &v1alpha1.Env{},
			refSources:   ociRef,
			expectedPath: path.Join(tempDir, "oci-chart", "https:/example.com/values.yaml"),
		},
		{
			// Regression: a ref with no file path after its name used to resolve to the OCI
			// extraction root.
			name:        "oci only ref",
			rawPath:     "$oci",
			env:         &v1alpha1.Env{},
			refSources:  ociRef,
			expectedErr: true,
		},
		{
			name:        "oci only ref with trailing slash",
			rawPath:     "$oci/",
			env:         &v1alpha1.Env{},
			refSources:  ociRef,
			expectedErr: true,
		},
		{
			name:        "oci attempted traversal",
			rawPath:     "$oci/../values.yaml",
			env:         &v1alpha1.Env{},
			refSources:  ociRef,
			expectedErr: true,
		},
		{
			name:    "oci repo doesn't exist",
			rawPath: "$oci/values.yaml",
			env:     &v1alpha1.Env{},
			refSources: map[string]*v1alpha1.RefTarget{
				"$oci": {Repo: v1alpha1.Repository{Repo: "oci://registry.example.com/other"}},
			},
			expectedErr: true,
		},
	}

	for _, tc := range testCases {
		tcc := tc
		t.Run(tcc.name, func(t *testing.T) {
			t.Parallel()
			resolvedPaths, err := (&valueFileResolver{appPath: path.Join(tempDir, "main-repo"), repoRoot: path.Join(tempDir, "main-repo"), env: tcc.env, allowedValueFilesSchemas: []string{}, refSources: tcc.refSources, gitRepoPaths: paths, ociPaths: paths, ignoreMissingValueFiles: false}).ResolveValueFiles([]string{tcc.rawPath})
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

func Test_valueFileResolver_ResolveValueFiles_glob(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	paths := utilio.NewRandomizedTempPaths(tempDir)
	paths.Add(git.NormalizeGitURL("https://github.com/org/repo1"), path.Join(tempDir, "repo1"))

	// main-repo files
	require.NoError(t, os.MkdirAll(path.Join(tempDir, "main-repo", "prod", "nested"), 0o755))
	require.NoError(t, os.MkdirAll(path.Join(tempDir, "main-repo", "staging"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(tempDir, "main-repo", "prod", "a.yaml"), []byte{}, 0o644))
	require.NoError(t, os.WriteFile(path.Join(tempDir, "main-repo", "prod", "b.yaml"), []byte{}, 0o644))
	require.NoError(t, os.WriteFile(path.Join(tempDir, "main-repo", "prod", "nested", "c.yaml"), []byte{}, 0o644))
	require.NoError(t, os.WriteFile(path.Join(tempDir, "main-repo", "prod", "nested", "d.yaml"), []byte{}, 0o644))
	require.NoError(t, os.WriteFile(path.Join(tempDir, "main-repo", "staging", "e.yaml"), []byte{}, 0o644))

	// main-repo envs: used to verify depth-order with ** (z.yaml sorts after nested/ alphabetically
	// but is still returned before nested/c.yaml because doublestar matches depth-0 files first).
	require.NoError(t, os.MkdirAll(path.Join(tempDir, "main-repo", "envs", "nested"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(tempDir, "main-repo", "envs", "a.yaml"), []byte{}, 0o644))
	require.NoError(t, os.WriteFile(path.Join(tempDir, "main-repo", "envs", "z.yaml"), []byte{}, 0o644))
	require.NoError(t, os.WriteFile(path.Join(tempDir, "main-repo", "envs", "nested", "c.yaml"), []byte{}, 0o644))

	// repo1 files
	require.NoError(t, os.MkdirAll(path.Join(tempDir, "repo1", "prod", "nested"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(tempDir, "repo1", "prod", "x.yaml"), []byte{}, 0o644))
	require.NoError(t, os.WriteFile(path.Join(tempDir, "repo1", "prod", "y.yaml"), []byte{}, 0o644))
	require.NoError(t, os.WriteFile(path.Join(tempDir, "repo1", "prod", "nested", "z.yaml"), []byte{}, 0o644))

	// oci-chart files
	paths.Add(v1alpha1.NormalizeOCIURL("oci://registry.example.com/chart"), path.Join(tempDir, "oci-chart"))
	require.NoError(t, os.MkdirAll(path.Join(tempDir, "oci-chart"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(tempDir, "oci-chart", "a.yaml"), []byte{}, 0o644))
	require.NoError(t, os.WriteFile(path.Join(tempDir, "oci-chart", "b.yaml"), []byte{}, 0o644))

	tests := []struct {
		name                    string
		rawPath                 string
		env                     *v1alpha1.Env
		refSources              map[string]*v1alpha1.RefTarget
		expectedPaths           []string
		ignoreMissingValueFiles bool
		expectedErr             bool
	}{
		{
			name:       "local glob matches multiple files",
			rawPath:    "prod/*.yaml",
			env:        &v1alpha1.Env{},
			refSources: map[string]*v1alpha1.RefTarget{},
			expectedPaths: []string{
				// the order is a.yaml before b.yaml
				// since doublestar.FilepathGlob returns lexical order
				path.Join(tempDir, "main-repo", "prod", "a.yaml"),
				path.Join(tempDir, "main-repo", "prod", "b.yaml"),
			},
		},
		{
			name:          "local glob matches no files returns error",
			rawPath:       "dev/*.yaml",
			env:           &v1alpha1.Env{},
			refSources:    map[string]*v1alpha1.RefTarget{},
			expectedPaths: nil,
			expectedErr:   true,
		},
		{
			name:                    "local glob matches no files with ignoreMissingValueFiles set to true",
			rawPath:                 "dev/*.yaml",
			env:                     &v1alpha1.Env{},
			refSources:              map[string]*v1alpha1.RefTarget{},
			ignoreMissingValueFiles: true,
			expectedPaths:           nil,
		},
		{
			name:    "referenced glob matches multiple files in external repo",
			rawPath: "$ref/prod/*.yaml",
			env:     &v1alpha1.Env{},
			refSources: map[string]*v1alpha1.RefTarget{
				"$ref": {
					Repo: v1alpha1.Repository{
						Repo: "https://github.com/org/repo1",
					},
				},
			},
			expectedPaths: []string{
				path.Join(tempDir, "repo1", "prod", "x.yaml"),
				path.Join(tempDir, "repo1", "prod", "y.yaml"),
			},
		},
		{
			name:    "ref glob with env var in path",
			rawPath: "$ref/$ENV/*.yaml",
			env: &v1alpha1.Env{
				&v1alpha1.EnvEntry{
					Name:  "ENV",
					Value: "prod",
				},
			},
			refSources: map[string]*v1alpha1.RefTarget{
				"$ref": {
					Repo: v1alpha1.Repository{
						Repo: "https://github.com/org/repo1",
					},
				},
			},
			expectedPaths: []string{
				path.Join(tempDir, "repo1", "prod", "x.yaml"),
				path.Join(tempDir, "repo1", "prod", "y.yaml"),
			},
		},
		{
			name:          "local glob single match",
			rawPath:       "prod/a*.yaml",
			env:           &v1alpha1.Env{},
			refSources:    map[string]*v1alpha1.RefTarget{},
			expectedPaths: []string{path.Join(tempDir, "main-repo", "prod", "a.yaml")},
		},
		{
			name: "recursive glob matches files at all depths under a subdirectory",
			// ** matches zero or more path segments, so prod/**/*.yaml covers both
			// prod/*.yaml (zero intermediate segments) and prod/nested/*.yaml (one segment), etc.
			rawPath:    "prod/**/*.yaml",
			env:        &v1alpha1.Env{},
			refSources: map[string]*v1alpha1.RefTarget{},
			// lexical order: prod/a.yaml, prod/b.yaml, prod/nested/c.yaml, prod/nested/d.yaml
			expectedPaths: []string{
				path.Join(tempDir, "main-repo", "prod", "a.yaml"),
				path.Join(tempDir, "main-repo", "prod", "b.yaml"),
				path.Join(tempDir, "main-repo", "prod", "nested", "c.yaml"),
				path.Join(tempDir, "main-repo", "prod", "nested", "d.yaml"),
			},
		},
		{
			name:       "recursive glob from repo root matches yaml files across all directories",
			rawPath:    "**/*.yaml",
			env:        &v1alpha1.Env{},
			refSources: map[string]*v1alpha1.RefTarget{},
			// doublestar traverses directories in lexical order, processing each directory's
			// own files before its subdirectories. So the order is:
			// envs/ flat files → envs/nested/ files → prod/ flat files → prod/nested/ files → staging/ files
			expectedPaths: []string{
				path.Join(tempDir, "main-repo", "envs", "a.yaml"),
				path.Join(tempDir, "main-repo", "envs", "z.yaml"),
				path.Join(tempDir, "main-repo", "envs", "nested", "c.yaml"),
				path.Join(tempDir, "main-repo", "prod", "a.yaml"),
				path.Join(tempDir, "main-repo", "prod", "b.yaml"),
				path.Join(tempDir, "main-repo", "prod", "nested", "c.yaml"),
				path.Join(tempDir, "main-repo", "prod", "nested", "d.yaml"),
				path.Join(tempDir, "main-repo", "staging", "e.yaml"),
			},
		},
		{
			name:       "recursive glob anchored to a named subdirectory matches at any depth",
			rawPath:    "**/nested/*.yaml",
			env:        &v1alpha1.Env{},
			refSources: map[string]*v1alpha1.RefTarget{},
			expectedPaths: []string{
				path.Join(tempDir, "main-repo", "envs", "nested", "c.yaml"),
				path.Join(tempDir, "main-repo", "prod", "nested", "c.yaml"),
				path.Join(tempDir, "main-repo", "prod", "nested", "d.yaml"),
			},
		},
		{
			name:                    "recursive glob with no matches and ignoreMissingValueFiles skips silently",
			rawPath:                 "**/nonexistent/*.yaml",
			env:                     &v1alpha1.Env{},
			refSources:              map[string]*v1alpha1.RefTarget{},
			ignoreMissingValueFiles: true,
			expectedPaths:           nil,
		},
		{
			name:          "recursive glob with no matches returns error",
			rawPath:       "**/nonexistent/*.yaml",
			env:           &v1alpha1.Env{},
			refSources:    map[string]*v1alpha1.RefTarget{},
			expectedPaths: nil,
			expectedErr:   true,
		},
		{
			// z.yaml sorts after "nested/" alphabetically by full path, but doublestar processes
			// each directory's own files before descending into subdirectories. So for envs/**/*.yaml:
			// envs/ flat files (a, z) come before envs/nested/ files (c), giving:
			// a.yaml, z.yaml, nested/c.yaml — not a.yaml, nested/c.yaml, z.yaml.
			name:       "** depth-order: flat files before nested even when flat file sorts after nested/ alphabetically",
			rawPath:    "envs/**/*.yaml",
			env:        &v1alpha1.Env{},
			refSources: map[string]*v1alpha1.RefTarget{},
			expectedPaths: []string{
				path.Join(tempDir, "main-repo", "envs", "a.yaml"),
				path.Join(tempDir, "main-repo", "envs", "z.yaml"),
				path.Join(tempDir, "main-repo", "envs", "nested", "c.yaml"),
			},
		},
		{
			name:    "recursive glob in external ref repo",
			rawPath: "$ref/prod/**/*.yaml",
			env:     &v1alpha1.Env{},
			refSources: map[string]*v1alpha1.RefTarget{
				"$ref": {
					Repo: v1alpha1.Repository{
						Repo: "https://github.com/org/repo1",
					},
				},
			},
			expectedPaths: []string{
				// doublestar matches zero path segments before recursing into subdirectories,
				// so flat files (x, y) come before nested ones (nested/z).
				path.Join(tempDir, "repo1", "prod", "x.yaml"),
				path.Join(tempDir, "repo1", "prod", "y.yaml"),
				path.Join(tempDir, "repo1", "prod", "nested", "z.yaml"),
			},
		},
		{
			// Regression: matches under the extracted OCI directory were rejected as outside the
			// repository root because the glob was checked against the main repo root.
			name:    "glob in oci ref source",
			rawPath: "$oci/*.yaml",
			env:     &v1alpha1.Env{},
			refSources: map[string]*v1alpha1.RefTarget{
				"$oci": {Repo: v1alpha1.Repository{Repo: "oci://registry.example.com/chart"}},
			},
			expectedPaths: []string{
				path.Join(tempDir, "oci-chart", "a.yaml"),
				path.Join(tempDir, "oci-chart", "b.yaml"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repoPath := path.Join(tempDir, "main-repo")
			resolvedPaths, err := (&valueFileResolver{appPath: repoPath, repoRoot: repoPath, env: tt.env, allowedValueFilesSchemas: []string{}, refSources: tt.refSources, gitRepoPaths: paths, ociPaths: paths, ignoreMissingValueFiles: tt.ignoreMissingValueFiles}).ResolveValueFiles([]string{tt.rawPath})
			if tt.expectedErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, resolvedPaths, len(tt.expectedPaths))

			for i, p := range tt.expectedPaths {
				assert.Equal(t, p, string(resolvedPaths[i]))
			}
		})
	}

	// Deduplication: first occurrence of a resolved path wins. Subsequent references to the
	// same file, whether explicit or via glob are silently dropped. This preserves the
	// merge-precedence position set by the first mention of each file.
	t.Run("glob then explicit: explicit entry placed at end, giving it highest Helm precedence", func(t *testing.T) {
		t.Parallel()
		repoPath := path.Join(tempDir, "main-repo")
		resolvedPaths, err := (&valueFileResolver{appPath: repoPath, repoRoot: repoPath, env: &v1alpha1.Env{}, allowedValueFilesSchemas: []string{}, refSources: map[string]*v1alpha1.RefTarget{}, gitRepoPaths: paths, ociPaths: paths, ignoreMissingValueFiles: false}).ResolveValueFiles(
			[]string{
				"envs/*.yaml", // glob - z.yaml is explicit so skipped; only a.yaml added
				"envs/z.yaml", // explicit - placed last, highest precedence
			},
		)
		require.NoError(t, err)
		require.Len(t, resolvedPaths, 2)
		assert.Equal(t, path.Join(tempDir, "main-repo", "envs", "a.yaml"), string(resolvedPaths[0]))
		assert.Equal(t, path.Join(tempDir, "main-repo", "envs", "z.yaml"), string(resolvedPaths[1]))
	})

	t.Run("explicit path before glob: explicit position is kept, glob re-match is dropped", func(t *testing.T) {
		t.Parallel()
		repoPath := path.Join(tempDir, "main-repo")
		resolvedPaths, err := (&valueFileResolver{appPath: repoPath, repoRoot: repoPath, env: &v1alpha1.Env{}, allowedValueFilesSchemas: []string{}, refSources: map[string]*v1alpha1.RefTarget{}, gitRepoPaths: paths, ociPaths: paths, ignoreMissingValueFiles: false}).ResolveValueFiles(
			[]string{
				"prod/a.yaml", // explicit locks in position 0
				"prod/*.yaml", // glob - a.yaml already seen, only b.yaml is new
			},
		)
		require.NoError(t, err)
		require.Len(t, resolvedPaths, 2)
		assert.Equal(t, path.Join(tempDir, "main-repo", "prod", "a.yaml"), string(resolvedPaths[0]))
		assert.Equal(t, path.Join(tempDir, "main-repo", "prod", "b.yaml"), string(resolvedPaths[1]))
	})

	t.Run("glob before explicit path: explicit position wins, glob skips the explicitly listed file", func(t *testing.T) {
		t.Parallel()
		repoPath := path.Join(tempDir, "main-repo")
		resolvedPaths, err := (&valueFileResolver{appPath: repoPath, repoRoot: repoPath, env: &v1alpha1.Env{}, allowedValueFilesSchemas: []string{}, refSources: map[string]*v1alpha1.RefTarget{}, gitRepoPaths: paths, ociPaths: paths, ignoreMissingValueFiles: false}).ResolveValueFiles(
			[]string{
				"prod/*.yaml", // glob - a.yaml is explicit so skipped; only b.yaml added (pos 0)
				"prod/a.yaml", // explicit - placed here at pos 1 (highest precedence)
			},
		)
		require.NoError(t, err)
		require.Len(t, resolvedPaths, 2)
		assert.Equal(t, path.Join(tempDir, "main-repo", "prod", "b.yaml"), string(resolvedPaths[0]))
		assert.Equal(t, path.Join(tempDir, "main-repo", "prod", "a.yaml"), string(resolvedPaths[1]))
	})

	t.Run("two overlapping globs: second glob only adds files not matched by first", func(t *testing.T) {
		t.Parallel()
		repoPath := path.Join(tempDir, "main-repo")
		resolvedPaths, err := (&valueFileResolver{appPath: repoPath, repoRoot: repoPath, env: &v1alpha1.Env{}, allowedValueFilesSchemas: []string{}, refSources: map[string]*v1alpha1.RefTarget{}, gitRepoPaths: paths, ociPaths: paths, ignoreMissingValueFiles: false}).ResolveValueFiles(
			[]string{
				"prod/*.yaml",    // adds a.yaml, b.yaml
				"prod/**/*.yaml", // a.yaml, b.yaml already seen; adds nested/c.yaml, nested/d.yaml
			},
		)
		require.NoError(t, err)
		require.Len(t, resolvedPaths, 4)
		assert.Equal(t, path.Join(tempDir, "main-repo", "prod", "a.yaml"), string(resolvedPaths[0]))
		assert.Equal(t, path.Join(tempDir, "main-repo", "prod", "b.yaml"), string(resolvedPaths[1]))
		assert.Equal(t, path.Join(tempDir, "main-repo", "prod", "nested", "c.yaml"), string(resolvedPaths[2]))
		assert.Equal(t, path.Join(tempDir, "main-repo", "prod", "nested", "d.yaml"), string(resolvedPaths[3]))
	})

	t.Run("explicit paths take priority: globs skip explicitly listed files, which are placed at their explicit positions", func(t *testing.T) {
		t.Parallel()
		repoPath := path.Join(tempDir, "main-repo")
		resolvedPaths, err := (&valueFileResolver{appPath: repoPath, repoRoot: repoPath, env: &v1alpha1.Env{}, allowedValueFilesSchemas: []string{}, refSources: map[string]*v1alpha1.RefTarget{}, gitRepoPaths: paths, ociPaths: paths, ignoreMissingValueFiles: false}).ResolveValueFiles(
			[]string{
				"prod/a.yaml",        // explicit - pos 0
				"prod/*.yaml",        // a.yaml and b.yaml are both explicit, skipped entirely
				"prod/b.yaml",        // explicit - pos 1
				"prod/**/*.yaml",     // a.yaml, b.yaml, nested/c.yaml all explicit and skipped; nested/d.yaml added - pos 2
				"prod/nested/c.yaml", // explicit - pos 3
			},
		)
		require.NoError(t, err)
		require.Len(t, resolvedPaths, 4)
		assert.Equal(t, path.Join(tempDir, "main-repo", "prod", "a.yaml"), string(resolvedPaths[0]))
		assert.Equal(t, path.Join(tempDir, "main-repo", "prod", "b.yaml"), string(resolvedPaths[1]))
		assert.Equal(t, path.Join(tempDir, "main-repo", "prod", "nested", "d.yaml"), string(resolvedPaths[2]))
		assert.Equal(t, path.Join(tempDir, "main-repo", "prod", "nested", "c.yaml"), string(resolvedPaths[3]))
	})
}

func Test_verifyGlobMatchesWithinRoot(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	outsideDir := filepath.Join(tempDir, "outside")

	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "values", "sub"), 0o755))
	require.NoError(t, os.MkdirAll(outsideDir, 0o755))

	// Files used as symlink targets
	inRepoFile := filepath.Join(repoDir, "values", "real.yaml")
	outsideFile := filepath.Join(outsideDir, "secret.yaml")
	require.NoError(t, os.WriteFile(inRepoFile, []byte{}, 0o644))
	require.NoError(t, os.WriteFile(outsideFile, []byte("password: hunter2"), 0o644))

	// Symlink inside repo → file inside repo (safe)
	inRepoLink := filepath.Join(repoDir, "values", "inrepo-link.yaml")
	require.NoError(t, os.Symlink(inRepoFile, inRepoLink))

	// Symlink inside repo → file outside repo (escape)
	escapeLink := filepath.Join(repoDir, "values", "escape-link.yaml")
	require.NoError(t, os.Symlink(outsideFile, escapeLink))

	// Two-hop symlink: inside repo → another symlink (still inside) → file inside repo
	hop1 := filepath.Join(repoDir, "values", "hop1.yaml")
	require.NoError(t, os.Symlink(inRepoLink, hop1)) // hop1 → inRepoLink → real.yaml

	// Two-hop symlink: inside repo → another symlink (inside repo) → file outside repo
	hop2 := filepath.Join(repoDir, "values", "hop2.yaml")
	require.NoError(t, os.Symlink(escapeLink, hop2)) // hop2 → escape-link → secret.yaml

	tests := []struct {
		name        string
		matches     []string
		expectErr   bool
		errContains string
	}{
		{
			name:    "regular file inside root passes",
			matches: []string{inRepoFile},
		},
		{
			name:    "symlink inside root pointing to file inside root passes",
			matches: []string{inRepoLink},
		},
		{
			name:    "two-hop chain that stays within root passes",
			matches: []string{hop1},
		},
		{
			name:        "symlink pointing directly outside root is rejected",
			matches:     []string{escapeLink},
			expectErr:   true,
			errContains: "resolved to outside repository root",
		},
		{
			name:        "two-hop chain that escapes root is rejected",
			matches:     []string{hop2},
			expectErr:   true,
			errContains: "resolved to outside repository root",
		},
		{
			name:    "multiple matches all inside root pass",
			matches: []string{inRepoFile, inRepoLink, hop1},
		},
		{
			name:        "one bad match in a list fails the whole call",
			matches:     []string{inRepoFile, escapeLink},
			expectErr:   true,
			errContains: "resolved to outside repository root",
		},
		{
			name:    "empty matches list is a no-op",
			matches: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := verifyGlobMatchesWithinRoot(tt.matches, repoDir)
			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// Test_valueFileResolver_ResolveValueFiles_glob_symlink_escape is an integration-level check
// that verifyGlobMatchesWithinRoot is wired into glob expansion correctly: a
// symlink inside the repo pointing outside must cause ResolveValueFiles to
// return an error rather than silently including the external file.
func Test_valueFileResolver_ResolveValueFiles_glob_symlink_escape(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	paths := utilio.NewRandomizedTempPaths(tempDir)

	repoDir := filepath.Join(tempDir, "repo")
	outsideDir := filepath.Join(tempDir, "outside")

	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "values"), 0o755))
	require.NoError(t, os.MkdirAll(outsideDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "values", "base.yaml"), []byte{}, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(outsideDir, "secret.yaml"), []byte("password: hunter2"), 0o644))
	require.NoError(t, os.Symlink(filepath.Join(outsideDir, "secret.yaml"), filepath.Join(repoDir, "values", "escape.yaml")))

	_, err := (&valueFileResolver{appPath: repoDir, repoRoot: repoDir, env: &v1alpha1.Env{}, allowedValueFilesSchemas: []string{}, refSources: map[string]*v1alpha1.RefTarget{}, gitRepoPaths: paths, ociPaths: paths, ignoreMissingValueFiles: false}).ResolveValueFiles([]string{"values/*.yaml"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolved to outside repository root")
}

func Test_isGlobPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{
			path:     "prod/*.yaml",
			expected: true,
		},
		{
			path:     "prod/?.yaml",
			expected: true,
		},
		{
			path:     "prod[ab].yaml",
			expected: true,
		},
		{
			path:     "prod/**/*.yaml",
			expected: true,
		},
		{
			path: "prod/values.yaml",
		},
		{
			path: "values.yaml",
		},
		{
			path: "",
		},
		{
			path:     "/absolute/path/to/*.yaml",
			expected: true,
		},
		{
			path: "/absolute/path/to/values.yaml",
		},
		{
			path:     "*",
			expected: true,
		},
		{
			path:     "?",
			expected: true,
		},
		{
			path:     "[",
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.expected, isGlobPath(tt.path))
		})
	}
}

func Test_getReferencedSource(t *testing.T) {
	t.Parallel()

	refTarget := &v1alpha1.RefTarget{
		Repo: v1alpha1.Repository{
			Repo: "https://github.com/org/repo1",
		},
	}
	tests := []struct {
		name         string
		rawValueFile string
		refSources   map[string]*v1alpha1.RefTarget
		expected     *v1alpha1.RefTarget
	}{
		{
			name:         "ref with file path found in map",
			rawValueFile: "$ref/values.yaml",
			refSources: map[string]*v1alpha1.RefTarget{
				"$ref": refTarget,
			},
			expected: refTarget,
		},
		{
			name:         "ref with file path not in map",
			rawValueFile: "$ref/values.yaml",
			refSources:   map[string]*v1alpha1.RefTarget{},
			expected:     nil,
		},
		{
			name:         "bare ref without file path found in map",
			rawValueFile: "$ref",
			refSources: map[string]*v1alpha1.RefTarget{
				"$ref": refTarget,
			},
			expected: refTarget,
		},
		{
			name:         "empty string returns nil",
			rawValueFile: "",
			refSources: map[string]*v1alpha1.RefTarget{
				"$ref": refTarget,
			},
			expected: nil,
		},
		{
			name:         "no $ prefix returns nil",
			rawValueFile: "values.yaml",
			refSources: map[string]*v1alpha1.RefTarget{
				"$ref": refTarget,
			},
			expected: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := getReferencedSource(tt.rawValueFile, tt.refSources)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_valueFileResolver_ResolveValueFiles_DoesNotLogResolvedPaths(t *testing.T) {
	// Regression: resolved OCI value files live under the randomized extraction directory.
	// Logging them leaked the reposerver filesystem layout, so only the count is logged.
	repoRoot := t.TempDir()
	appPath := filepath.Join(repoRoot, "app")
	require.NoError(t, os.MkdirAll(appPath, 0o755))

	ociDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(ociDir, "a.yaml"), []byte("a: 1"), 0o644))

	ociPaths := utilio.NewRandomizedTempPaths(t.TempDir())
	ociPaths.Add(v1alpha1.NormalizeOCIURL("oci://registry.example.com/chart"), ociDir)

	resolver := &valueFileResolver{
		appPath:                  appPath,
		repoRoot:                 repoRoot,
		env:                      &v1alpha1.Env{},
		allowedValueFilesSchemas: []string{"https"},
		refSources: map[string]*v1alpha1.RefTarget{
			"$oci": {Repo: v1alpha1.Repository{Repo: "oci://registry.example.com/chart"}},
		},
		gitRepoPaths:            utilio.NewRandomizedTempPaths(t.TempDir()),
		ociPaths:                ociPaths,
		ignoreMissingValueFiles: true,
	}

	hook := logtest.NewGlobal()
	t.Cleanup(hook.Reset)
	// Debug level so the "values file does not exist" message for the missing file is emitted.
	prevLevel := log.GetLevel()
	log.SetLevel(log.DebugLevel)
	t.Cleanup(func() { log.SetLevel(prevLevel) })

	result, err := resolver.ResolveValueFiles([]string{"$oci/a.yaml", "$oci/missing.yaml"})
	require.NoError(t, err)
	require.Len(t, result, 1)

	for _, entry := range hook.AllEntries() {
		msg, err := entry.String()
		require.NoError(t, err)
		assert.NotContains(t, msg, ociDir, "log entry leaked the OCI extraction directory")
	}
}

// assertNoCredentialsLogged installs a global log hook and, on cleanup, asserts that none of the given
// secrets were written to any log entry. The out-of-bounds symlink warnings log the referenced
// repository, which must be the URL only and never the full Repository (credentials included).
func assertNoCredentialsLogged(t *testing.T, secrets ...string) {
	t.Helper()
	hook := logtest.NewGlobal()
	t.Cleanup(func() {
		defer hook.Reset()
		for _, entry := range hook.AllEntries() {
			msg, err := entry.String()
			require.NoError(t, err)
			for _, secret := range secrets {
				assert.NotContains(t, msg, secret, "log entry leaked a repository credential")
			}
		}
	})
}

func ociRefRequest(t *testing.T, repo string) refSourceResolveRequest {
	t.Helper()
	refSource := &v1alpha1.RefTarget{
		Repo:           v1alpha1.Repository{Repo: repo},
		TargetRevision: "v1.0.0",
	}
	return refSourceResolveRequest{
		refSourceMapping:  refSource,
		normalizedRepoURL: refSource.Repo.NormalizeRepoURL(),
		refVar:            "$ref",
		ociRefPaths:       utilio.NewRandomizedTempPaths(t.TempDir()),
	}
}

func TestResolveOCIRefSource(t *testing.T) {
	const repo = "oci://registry.example.com/chart"

	t.Run("OCI client creation fails", func(t *testing.T) {
		service, _, _ := newServiceWithOpt(t, func(_ *gitmocks.Client, _ *helmmocks.Client, _ *ocimocks.Client, _ *iomocks.TempPaths) {}, ".")
		service.newOCIClient = func(_ string, _ oci.Creds, _ string, _ string, _ []string, _ ...oci.ClientOpts) (oci.Client, error) {
			return nil, errors.New("internal client failure")
		}

		_, closer, err := service.resolveOCIRefSource(t.Context(), ociRefRequest(t, repo))
		assert.Nil(t, closer)
		require.ErrorContains(t, err, "failed to create OCI client for repo")
		// The underlying cause must not leak into the client-facing error.
		assert.NotContains(t, err.Error(), "internal client failure")
	})

	t.Run("revision resolution fails", func(t *testing.T) {
		service, _, _ := newServiceWithOpt(t, func(_ *gitmocks.Client, _ *helmmocks.Client, ociClient *ocimocks.Client, _ *iomocks.TempPaths) {
			ociClient.EXPECT().ResolveRevision(mock.Anything, "v1.0.0", mock.Anything).Return("", errors.New("registry unreachable"))
		}, ".")

		_, closer, err := service.resolveOCIRefSource(t.Context(), ociRefRequest(t, repo))
		assert.Nil(t, closer)
		require.ErrorContains(t, err, "failed to resolve OCI revision v1.0.0")
		assert.NotContains(t, err.Error(), "registry unreachable")
	})

	t.Run("extraction fails", func(t *testing.T) {
		service, _, _ := newServiceWithOpt(t, func(_ *gitmocks.Client, _ *helmmocks.Client, ociClient *ocimocks.Client, _ *iomocks.TempPaths) {
			ociClient.EXPECT().ResolveRevision(mock.Anything, "v1.0.0", mock.Anything).Return("sha256:abc", nil)
			ociClient.EXPECT().Extract(mock.Anything, "sha256:abc").Return("", nil, errors.New("layer digest mismatch"))
		}, ".")

		_, closer, err := service.resolveOCIRefSource(t.Context(), ociRefRequest(t, repo))
		assert.Nil(t, closer)
		require.ErrorContains(t, err, "failed to extract OCI image")
		assert.NotContains(t, err.Error(), "layer digest mismatch")
	})

	t.Run("out-of-bounds symlink is rejected", func(t *testing.T) {
		ociDir := t.TempDir()
		require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(ociDir, "link.yaml")))

		service, _, _ := newServiceWithOpt(t, func(_ *gitmocks.Client, _ *helmmocks.Client, ociClient *ocimocks.Client, _ *iomocks.TempPaths) {
			ociClient.EXPECT().ResolveRevision(mock.Anything, "v1.0.0", mock.Anything).Return("sha256:abc", nil)
			ociClient.EXPECT().Extract(mock.Anything, "sha256:abc").Return(ociDir, utilio.NopCloser, nil)
		}, ".")

		req := ociRefRequest(t, repo)
		req.refSourceMapping.Repo.Password = "oci-s3cret-password"
		assertNoCredentialsLogged(t, "oci-s3cret-password")
		_, _, err := service.resolveOCIRefSource(t.Context(), req)
		require.ErrorContains(t, err, "oci image contains out-of-bounds symlinks")
		// The extracted path must not be registered when the symlink check fails.
		assert.Empty(t, req.ociRefPaths.GetPathIfExists(req.normalizedRepoURL))
	})

	t.Run("success registers the extracted path", func(t *testing.T) {
		ociDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(ociDir, "values.yaml"), []byte("foo: bar"), 0o644))

		service, _, _ := newServiceWithOpt(t, func(_ *gitmocks.Client, _ *helmmocks.Client, ociClient *ocimocks.Client, _ *iomocks.TempPaths) {
			ociClient.EXPECT().ResolveRevision(mock.Anything, "v1.0.0", mock.Anything).Return("sha256:abc", nil)
			ociClient.EXPECT().Extract(mock.Anything, "sha256:abc").Return(ociDir, utilio.NopCloser, nil)
		}, ".")

		req := ociRefRequest(t, repo)
		ref, closer, err := service.resolveOCIRefSource(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, closer)
		assert.Equal(t, "v1.0.0", ref.revision)
		assert.Equal(t, "sha256:abc", ref.commitSHA)
		assert.Equal(t, "$ref", ref.key)
		assert.Equal(t, ociDir, req.ociRefPaths.GetPathIfExists(req.normalizedRepoURL))
	})

	t.Run("recorded digest is extracted without resolving the tag again", func(t *testing.T) {
		ociDir := t.TempDir()
		service, _, _ := newServiceWithOpt(t, func(_ *gitmocks.Client, _ *helmmocks.Client, ociClient *ocimocks.Client, _ *iomocks.TempPaths) {
			// No ResolveRevision expectation: a second resolution could see a moved tag and store
			// its content under the digest already recorded in the cache key.
			ociClient.EXPECT().Extract(mock.Anything, "sha256:recorded").Return(ociDir, utilio.NopCloser, nil)
		}, ".")

		req := ociRefRequest(t, repo)
		req.resolvedRevision = "sha256:recorded"
		ref, _, err := service.resolveOCIRefSource(t.Context(), req)
		require.NoError(t, err)
		assert.Equal(t, "sha256:recorded", ref.commitSHA)
	})

	t.Run("hard refresh clears the cached archive before extracting", func(t *testing.T) {
		ociDir := t.TempDir()
		service, _, _ := newServiceWithOpt(t, func(_ *gitmocks.Client, _ *helmmocks.Client, ociClient *ocimocks.Client, _ *iomocks.TempPaths) {
			ociClient.EXPECT().ResolveRevision(mock.Anything, "v1.0.0", true).Return("sha256:abc", nil)
			ociClient.EXPECT().CleanCache("sha256:abc").Return(nil).Once()
			ociClient.EXPECT().Extract(mock.Anything, "sha256:abc").Return(ociDir, utilio.NopCloser, nil)
		}, ".")

		req := ociRefRequest(t, repo)
		req.noCache = true
		_, _, err := service.resolveOCIRefSource(t.Context(), req)
		require.NoError(t, err)
	})
}

func TestResolveGitRefSource(t *testing.T) {
	const repo = "https://github.com/test/repo.git"

	gitRefRequest := func() refSourceResolveRequest {
		refSource := &v1alpha1.RefTarget{
			Repo:           v1alpha1.Repository{Repo: repo},
			TargetRevision: "main",
		}
		return refSourceResolveRequest{
			appRepoURL:        repo,
			appRevision:       "primary-revision",
			refSourceMapping:  refSource,
			normalizedRepoURL: refSource.Repo.NormalizeRepoURL(),
			refVar:            "$ref",
			commitSHA:         "primary-sha",
		}
	}

	t.Run("git client resolution fails", func(t *testing.T) {
		service, _, _ := newServiceWithOpt(t, func(gitClient *gitmocks.Client, _ *helmmocks.Client, _ *ocimocks.Client, paths *iomocks.TempPaths) {
			paths.EXPECT().GetPath(mock.Anything).Return(t.TempDir(), nil)
			gitClient.EXPECT().Root().Return("/tmp/repo")
			gitClient.EXPECT().LsRemote(mock.Anything).Return("", errors.New("auth required"))
		}, ".")

		_, closer, err := service.resolveGitRefSource(t.Context(), gitRefRequest())
		assert.Nil(t, closer)
		require.ErrorContains(t, err, "failed to get git client for repo")
		assert.NotContains(t, err.Error(), "auth required")
	})

	t.Run("referencing a different revision of the same repository is rejected", func(t *testing.T) {
		service, _, _ := newServiceWithOpt(t, func(gitClient *gitmocks.Client, _ *helmmocks.Client, _ *ocimocks.Client, paths *iomocks.TempPaths) {
			paths.EXPECT().GetPath(mock.Anything).Return(t.TempDir(), nil)
			gitClient.EXPECT().Root().Return("/tmp/repo")
			// Resolves to a commit that differs from the primary source's commitSHA.
			gitClient.EXPECT().LsRemote(mock.Anything).Return("referenced-sha", nil)
		}, ".")

		_, closer, err := service.resolveGitRefSource(t.Context(), gitRefRequest())
		assert.Nil(t, closer)
		require.ErrorContains(t, err, "cannot reference a different revision of the same repository")
	})

	t.Run("recorded commit is checked out without resolving the branch again", func(t *testing.T) {
		root := t.TempDir()
		service, _, _ := newServiceWithOpt(t, func(gitClient *gitmocks.Client, _ *helmmocks.Client, _ *ocimocks.Client, paths *iomocks.TempPaths) {
			paths.EXPECT().GetPath(mock.Anything).Return(root, nil)
			gitClient.EXPECT().Root().Return(root)
			// No LsRemote expectation: a second resolution could see a moved branch and store its
			// content under the commit already recorded in the cache key.
			gitClient.EXPECT().Init().Return(nil)
			gitClient.EXPECT().IsRevisionPresent(mock.Anything, "recorded-sha").Return(true)
			gitClient.EXPECT().Checkout(mock.Anything, "recorded-sha", mock.Anything, mock.Anything).Return("", nil)
		}, ".")

		req := gitRefRequest()
		req.appRepoURL = "https://github.com/test/other.git" // not the application's repository, so no same-repo guard
		req.resolvedRevision = "recorded-sha"
		ref, closer, err := service.resolveGitRefSource(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, closer)
		assert.Equal(t, "recorded-sha", ref.commitSHA)
	})

	t.Run("out-of-bounds symlink is rejected after checkout", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(root, "link.yaml")))

		service, _, _ := newServiceWithOpt(t, func(gitClient *gitmocks.Client, _ *helmmocks.Client, _ *ocimocks.Client, paths *iomocks.TempPaths) {
			paths.EXPECT().GetPath(mock.Anything).Return(root, nil)
			gitClient.EXPECT().Root().Return(root)
			gitClient.EXPECT().LsRemote(mock.Anything).Return("primary-sha", nil)
			gitClient.EXPECT().Init().Return(nil)
			gitClient.EXPECT().IsRevisionPresent(mock.Anything, mock.Anything).Return(true)
			gitClient.EXPECT().Checkout(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", nil)
		}, ".")

		req := gitRefRequest()
		req.noCache = true // bypass the symlink-check cache so this test is self-contained
		req.refSourceMapping.Repo.Password = "git-s3cret-password"
		req.refSourceMapping.Repo.SSHPrivateKey = "git-s3cret-private-key"
		assertNoCredentialsLogged(t, "git-s3cret-password", "git-s3cret-private-key")
		_, _, err := service.resolveGitRefSource(t.Context(), req)
		require.ErrorContains(t, err, "repository contains out-of-bounds symlinks")
	})

	t.Run("success returns a repoRef and a closer", func(t *testing.T) {
		root := t.TempDir()

		service, _, _ := newServiceWithOpt(t, func(gitClient *gitmocks.Client, _ *helmmocks.Client, _ *ocimocks.Client, paths *iomocks.TempPaths) {
			paths.EXPECT().GetPath(mock.Anything).Return(root, nil)
			gitClient.EXPECT().Root().Return(root)
			gitClient.EXPECT().LsRemote(mock.Anything).Return("primary-sha", nil)
			gitClient.EXPECT().Init().Return(nil)
			gitClient.EXPECT().IsRevisionPresent(mock.Anything, mock.Anything).Return(true)
			gitClient.EXPECT().Checkout(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", nil)
		}, ".")

		req := gitRefRequest()
		req.noCache = true
		ref, closer, err := service.resolveGitRefSource(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, closer)
		defer utilio.Close(closer)
		assert.Equal(t, "main", ref.revision)
		assert.Equal(t, "primary-sha", ref.commitSHA)
		assert.Equal(t, "$ref", ref.key)
	})
}
