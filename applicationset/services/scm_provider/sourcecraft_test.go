package scm_provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func sourcecraftMockHandler(t *testing.T) func(http.ResponseWriter, *http.Request) {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/orgs/test-org/repos?":
			f, err := os.Open("testdata/sourcecraft_org_repos.json")
			require.NoError(t, err)
			defer f.Close()
			_, err = io.Copy(w, f)
			require.NoError(t, err)
		case "/repos/test-org/test-repo/branches?filter=main":
			f, err := os.Open("testdata/sourcecraft_branch_main.json")
			require.NoError(t, err)
			defer f.Close()
			_, err = io.Copy(w, f)
			require.NoError(t, err)
		case "/repos/test-org/test-repo/branches?":
			f, err := os.Open("testdata/sourcecraft_branches_all.json")
			require.NoError(t, err)
			defer f.Close()
			_, err = io.Copy(w, f)
			require.NoError(t, err)
		case "/repos/test-org/another-repo/branches?filter=master":
			f, err := os.Open("testdata/sourcecraft_branch_master.json")
			require.NoError(t, err)
			defer f.Close()
			_, err = io.Copy(w, f)
			require.NoError(t, err)
		case "/repos/test-org/another-repo/branches?":
			// Return single branch for another-repo when all branches requested
			f, err := os.Open("testdata/sourcecraft_branch_master.json")
			require.NoError(t, err)
			defer f.Close()
			_, err = io.Copy(w, f)
			require.NoError(t, err)
		case "/repos/test-org/test-repo/labels?":
			f, err := os.Open("testdata/sourcecraft_labels.json")
			require.NoError(t, err)
			defer f.Close()
			_, err = io.Copy(w, f)
			require.NoError(t, err)
		case "/repos/test-org/another-repo/labels?":
			// Return empty labels for another-repo
			_, err := io.WriteString(w, `{"items": [], "next_page_token": ""}`)
			require.NoError(t, err)
		case "/repos/test-org/test-repo/trees?path=README.md&revision=main":
			f, err := os.Open("testdata/sourcecraft_file_tree_exists.json")
			require.NoError(t, err)
			defer f.Close()
			_, err = io.Copy(w, f)
			require.NoError(t, err)
		case "/repos/test-org/test-repo/trees?path=src&revision=main":
			f, err := os.Open("testdata/sourcecraft_file_tree_directory.json")
			require.NoError(t, err)
			defer f.Close()
			_, err = io.Copy(w, f)
			require.NoError(t, err)
		case "/repos/test-org/test-repo/trees?path=notathing&revision=main":
			w.WriteHeader(http.StatusNotFound)
			_, err := io.WriteString(w, `{"message": "Path not found"}`)
			require.NoError(t, err)
		case "/repos/test-org/test-repo/trees?path=emptydir&revision=main":
			f, err := os.Open("testdata/sourcecraft_file_tree_empty.json")
			require.NoError(t, err)
			defer f.Close()
			_, err = io.Copy(w, f)
			require.NoError(t, err)
		case "/repos/test-org/nonexistent-repo/branches?filter=main":
			w.WriteHeader(http.StatusNotFound)
			_, err := io.WriteString(w, `{"message": "Repository not found"}`)
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, err := io.WriteString(w, `{"message": "Not found"}`)
			require.NoError(t, err)
		}
	}
}

func TestSourceCraftListRepos(t *testing.T) {
	cases := []struct {
		name, proto, url                        string
		hasError, allBranches, includeSubgroups bool
		branches                                []string
		filters                                 []v1alpha1.SCMProviderGeneratorFilter
	}{
		{
			name:        "blank protocol",
			allBranches: false,
			url:         "git@git.sourcecraft.com:test-org/test-repo.git",
			branches:    []string{"main"},
		},
		{
			name:        "ssh protocol",
			allBranches: false,
			proto:       "ssh",
			url:         "git@git.sourcecraft.com:test-org/test-repo.git",
			branches:    []string{"main"},
		},
		{
			name:        "https protocol",
			allBranches: false,
			proto:       "https",
			url:         "https://git.sourcecraft.com/test-org/test-repo.git",
			branches:    []string{"main"},
		},
		{
			name:        "other protocol",
			allBranches: false,
			proto:       "other",
			hasError:    true,
		},
		{
			name:        "all branches",
			allBranches: true,
			url:         "git@git.sourcecraft.com:test-org/test-repo.git",
			branches:    []string{"main", "develop", "feature-branch"},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourcecraftMockHandler(t)(w, r)
	}))
	defer ts.Close()

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			provider, err := NewSourceCraftProvider("test-org", "test-token", ts.URL, c.allBranches, false)
			require.NoError(t, err)

			rawRepos, err := ListRepos(context.Background(), provider, c.filters, c.proto)
			if c.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				// Check that the test-repo shows up
				repos := []*Repository{}
				branches := []string{}
				for _, r := range rawRepos {
					if r.Repository == "test-repo" {
						repos = append(repos, r)
						branches = append(branches, r.Branch)
					}
				}
				assert.NotEmpty(t, repos)
				assert.Equal(t, c.url, repos[0].URL)
				for _, b := range c.branches {
					assert.Contains(t, branches, b)
				}
			}
		})
	}
}

func TestSourceCraftListReposWithLabels(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourcecraftMockHandler(t)(w, r)
	}))
	defer ts.Close()

	provider, err := NewSourceCraftProvider("test-org", "test-token", ts.URL, false, false)
	require.NoError(t, err)

	repos, err := ListRepos(context.Background(), provider, []v1alpha1.SCMProviderGeneratorFilter{}, "ssh")
	require.NoError(t, err)

	// Find test-repo and check its labels
	var testRepo *Repository
	for _, r := range repos {
		if r.Repository == "test-repo" {
			testRepo = r
			break
		}
	}

	require.NotNil(t, testRepo, "test-repo should be in the list")
	assert.Contains(t, testRepo.Labels, "backend")
	assert.Contains(t, testRepo.Labels, "frontend")
	assert.Len(t, testRepo.Labels, 2)
}

func TestSourceCraftGetBranches(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourcecraftMockHandler(t)(w, r)
	}))
	defer ts.Close()

	t.Run("default branch only", func(t *testing.T) {
		provider, err := NewSourceCraftProvider("test-org", "test-token", ts.URL, false, false)
		require.NoError(t, err)

		repo := &Repository{
			Organization: "test-org",
			Repository:   "test-repo",
			Branch:       "main",
			URL:          "git@git.sourcecraft.com:test-org/test-repo.git",
		}

		branches, err := provider.GetBranches(context.Background(), repo)
		require.NoError(t, err)
		assert.Len(t, branches, 1)
		assert.Equal(t, "main", branches[0].Branch)
		assert.Equal(t, "72687815ccba81ef014a96201cc2e846a68789d8", branches[0].SHA)
	})

	t.Run("all branches", func(t *testing.T) {
		provider, err := NewSourceCraftProvider("test-org", "test-token", ts.URL, true, false)
		require.NoError(t, err)

		repo := &Repository{
			Organization: "test-org",
			Repository:   "test-repo",
			Branch:       "main",
			URL:          "git@git.sourcecraft.com:test-org/test-repo.git",
		}

		branches, err := provider.GetBranches(context.Background(), repo)
		require.NoError(t, err)
		assert.Len(t, branches, 3)

		branchNames := []string{}
		for _, b := range branches {
			branchNames = append(branchNames, b.Branch)
		}
		assert.Contains(t, branchNames, "main")
		assert.Contains(t, branchNames, "develop")
		assert.Contains(t, branchNames, "feature-branch")
	})

	t.Run("branch not found", func(t *testing.T) {
		provider, err := NewSourceCraftProvider("test-org", "test-token", ts.URL, false, false)
		require.NoError(t, err)

		repo := &Repository{
			Organization: "test-org",
			Repository:   "nonexistent-repo",
			Branch:       "main",
			URL:          "git@git.sourcecraft.com:test-org/nonexistent-repo.git",
		}

		_, err = provider.GetBranches(context.Background(), repo)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "got 404 while getting default branch")
	})
}

func TestSourceCraftRepoHasPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourcecraftMockHandler(t)(w, r)
	}))
	defer ts.Close()

	provider, err := NewSourceCraftProvider("test-org", "test-token", ts.URL, false, false)
	require.NoError(t, err)

	repo := &Repository{
		Organization: "test-org",
		Repository:   "test-repo",
		Branch:       "main",
	}

	t.Run("file exists", func(t *testing.T) {
		ok, err := provider.RepoHasPath(context.Background(), repo, "README.md")
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("directory exists", func(t *testing.T) {
		ok, err := provider.RepoHasPath(context.Background(), repo, "src")
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("path does not exist", func(t *testing.T) {
		ok, err := provider.RepoHasPath(context.Background(), repo, "notathing")
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("empty directory", func(t *testing.T) {
		ok, err := provider.RepoHasPath(context.Background(), repo, "emptydir")
		require.NoError(t, err)
		assert.False(t, ok)
	})
}

func TestSourceCraftProviderWithToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the token is sent correctly
		assert.Equal(t, "Bearer test-token-123", r.Header.Get("Authorization"))
		sourcecraftMockHandler(t)(w, r)
	}))
	defer ts.Close()

	provider, err := NewSourceCraftProvider("test-org", "test-token-123", ts.URL, false, false)
	require.NoError(t, err)

	repos, err := ListRepos(context.Background(), provider, []v1alpha1.SCMProviderGeneratorFilter{}, "ssh")
	require.NoError(t, err)
	assert.NotEmpty(t, repos)
}

func TestSourceCraftProviderWithEnvToken(t *testing.T) {
	// Set environment variable
	envToken := "env-test-token"
	t.Setenv("SOURCECRAFT_TOKEN", envToken)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the env token is sent correctly
		assert.Equal(t, "Bearer "+envToken, r.Header.Get("Authorization"))
		sourcecraftMockHandler(t)(w, r)
	}))
	defer ts.Close()

	// Pass empty token to trigger env variable usage
	provider, err := NewSourceCraftProvider("test-org", "", ts.URL, false, false)
	require.NoError(t, err)

	repos, err := ListRepos(context.Background(), provider, []v1alpha1.SCMProviderGeneratorFilter{}, "ssh")
	require.NoError(t, err)
	assert.NotEmpty(t, repos)
}

func TestSourceCraftProviderInsecure(t *testing.T) {
	// Create a test server with TLS
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourcecraftMockHandler(t)(w, r)
	}))
	defer ts.Close()

	// Test with insecure=true (should work with self-signed cert)
	provider, err := NewSourceCraftProvider("test-org", "test-token", ts.URL, false, true)
	require.NoError(t, err)

	repos, err := ListRepos(context.Background(), provider, []v1alpha1.SCMProviderGeneratorFilter{}, "ssh")
	require.NoError(t, err)
	assert.NotEmpty(t, repos)
}

func TestSourceCraftProviderSecure(t *testing.T) {
	// Create a test server with TLS
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourcecraftMockHandler(t)(w, r)
	}))
	defer ts.Close()

	// Test with insecure=false (should fail with self-signed cert)
	provider, err := NewSourceCraftProvider("test-org", "test-token", ts.URL, false, false)
	require.NoError(t, err)

	_, err = ListRepos(context.Background(), provider, []v1alpha1.SCMProviderGeneratorFilter{}, "ssh")
	// Should fail due to certificate verification
	require.Error(t, err)
}

func TestSourceCraftGetBranchesPreservesRepoData(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourcecraftMockHandler(t)(w, r)
	}))
	defer ts.Close()

	provider, err := NewSourceCraftProvider("test-org", "test-token", ts.URL, false, false)
	require.NoError(t, err)

	repo := &Repository{
		Organization: "test-org",
		Repository:   "test-repo",
		Branch:       "main",
		URL:          "git@git.sourcecraft.com:test-org/test-repo.git",
		Labels:       []string{"label1", "label2"},
		RepositoryId: "repo-id-123",
	}

	branches, err := provider.GetBranches(context.Background(), repo)
	require.NoError(t, err)
	assert.Len(t, branches, 1)

	// Verify that all original repo data is preserved
	assert.Equal(t, repo.Organization, branches[0].Organization)
	assert.Equal(t, repo.Repository, branches[0].Repository)
	assert.Equal(t, repo.URL, branches[0].URL)
	assert.Equal(t, repo.Labels, branches[0].Labels)
	assert.Equal(t, repo.RepositoryId, branches[0].RepositoryId)
	assert.Equal(t, "72687815ccba81ef014a96201cc2e846a68789d8", branches[0].SHA)
}

func TestSourceCraftListReposReturnsAllRepos(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourcecraftMockHandler(t)(w, r)
	}))
	defer ts.Close()

	provider, err := NewSourceCraftProvider("test-org", "test-token", ts.URL, false, false)
	require.NoError(t, err)

	repos, err := ListRepos(context.Background(), provider, []v1alpha1.SCMProviderGeneratorFilter{}, "ssh")
	require.NoError(t, err)

	// Should have 2 repos from the fixture
	assert.Len(t, repos, 2)

	// Check first repo
	var testRepo *Repository
	var anotherRepo *Repository
	for _, r := range repos {
		switch r.Repository {
		case "test-repo":
			testRepo = r
		case "another-repo":
			anotherRepo = r
		}
	}

	require.NotNil(t, testRepo)
	assert.Equal(t, "test-org", testRepo.Organization)
	assert.Equal(t, "main", testRepo.Branch)
	assert.Equal(t, "git@git.sourcecraft.com:test-org/test-repo.git", testRepo.URL)
	assert.Equal(t, "01989dab-806f-7b1b-9186-f437bb2cf241", testRepo.RepositoryId)

	require.NotNil(t, anotherRepo)
	assert.Equal(t, "test-org", anotherRepo.Organization)
	assert.Equal(t, "master", anotherRepo.Branch)
	assert.Equal(t, "git@git.sourcecraft.com:test-org/another-repo.git", anotherRepo.URL)
	assert.Equal(t, "01989dab-806f-7b1b-9186-f437bb2cf242", anotherRepo.RepositoryId)
}

func TestSourceCraftGetBranchesNilBranch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return response without branches
		if r.RequestURI == "/repos/test-org/test-repo/branches?filter=main" {
			_, err := io.WriteString(w, `{"branches": [], "next_page_token": ""}`)
			require.NoError(t, err)
		} else {
			sourcecraftMockHandler(t)(w, r)
		}
	}))
	defer ts.Close()

	provider, err := NewSourceCraftProvider("test-org", "test-token", ts.URL, false, false)
	require.NoError(t, err)

	repo := &Repository{
		Organization: "test-org",
		Repository:   "test-repo",
		Branch:       "main",
		URL:          "git@git.sourcecraft.com:test-org/test-repo.git",
	}

	_, err = provider.GetBranches(context.Background(), repo)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "got nil branch while getting default branch")
}
