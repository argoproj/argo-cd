package sourcecraft

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_ListRepoBranches(t *testing.T) {
	tests := []struct {
		name           string
		orgSlug        string
		repoSlug       string
		options        ListRepoBranchesOptions
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		checkResponse  func(t *testing.T, resp *ListRepoBranchesResponse)
		checkRequest   func(t *testing.T, r *http.Request)
	}{
		{
			name:     "SuccessfulListBranches",
			orgSlug:  "test-org",
			repoSlug: "test-repo",
			options:  ListRepoBranchesOptions{},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				resp := ListRepoBranchesResponse{
					Branches: []*Branch{
						{Name: "main", Commit: &Commit{Hash: "abc123"}},
						{Name: "develop", Commit: &Commit{Hash: "def456"}},
					},
					NextPageToken: "token123",
				}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkResponse: func(t *testing.T, resp *ListRepoBranchesResponse) {
				t.Helper()
				assert.Len(t, resp.Branches, 2)
				assert.Equal(t, "main", resp.Branches[0].Name)
				assert.Equal(t, "develop", resp.Branches[1].Name)
				assert.Equal(t, "token123", resp.NextPageToken)
			},
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				assert.Equal(t, "/repos/test-org/test-repo/branches", r.URL.Path)
				assert.Equal(t, "GET", r.Method)
			},
		},
		{
			name:     "WithPagination",
			orgSlug:  "test-org",
			repoSlug: "test-repo",
			options: ListRepoBranchesOptions{
				ListOptions: ListOptions{
					PageToken: "next-page",
					PageSize:  10,
				},
			},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				resp := ListRepoBranchesResponse{
					Branches:      []*Branch{{Name: "feature-1"}},
					NextPageToken: "",
				}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				query := r.URL.Query()
				assert.Equal(t, "next-page", query.Get("page_token"))
				assert.Equal(t, "10", query.Get("page_size"))
			},
		},
		{
			name:     "WithFilter",
			orgSlug:  "test-org",
			repoSlug: "test-repo",
			options: ListRepoBranchesOptions{
				ListOptions: ListOptions{
					Filter: "main",
				},
			},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				resp := ListRepoBranchesResponse{
					Branches: []*Branch{{Name: "main"}},
				}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				assert.Equal(t, "main", r.URL.Query().Get("filter"))
			},
		},
		{
			name:     "EmptyOrgSlug",
			orgSlug:  "",
			repoSlug: "test-repo",
			options:  ListRepoBranchesOptions{},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr: true,
		},
		{
			name:     "EmptyRepoSlug",
			orgSlug:  "test-org",
			repoSlug: "",
			options:  ListRepoBranchesOptions{},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr: true,
		},
		{
			name:     "SpecialCharactersInSlugs",
			orgSlug:  "test org",
			repoSlug: "test/repo",
			options:  ListRepoBranchesOptions{},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				resp := ListRepoBranchesResponse{Branches: []*Branch{}}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				// URL path encoding: %20 for spaces, %2F for forward slash
				escapedPath := r.URL.EscapedPath()
				assert.Contains(t, escapedPath, "test%20org")
				assert.Contains(t, escapedPath, "test%2Frepo")
			},
		},
		{
			name:     "ServerError",
			orgSlug:  "test-org",
			repoSlug: "test-repo",
			options:  ListRepoBranchesOptions{},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, err := w.Write([]byte(`{"message":"internal error"}`))
				require.NoError(t, err)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedRequest *http.Request
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedRequest = r
				tt.serverResponse(w, r)
			}))
			defer server.Close()

			client, err := NewClient(server.URL)
			require.NoError(t, err)

			resp, httpResp, err := client.ListRepoBranches(context.Background(), tt.orgSlug, tt.repoSlug, tt.options)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, httpResp)
				if tt.checkResponse != nil {
					tt.checkResponse(t, resp)
				}
				if tt.checkRequest != nil {
					tt.checkRequest(t, capturedRequest)
				}
			}
		})
	}
}

func TestClient_GetRepoBranch(t *testing.T) {
	tests := []struct {
		name           string
		orgSlug        string
		repoSlug       string
		branch         string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		checkResponse  func(t *testing.T, branch *Branch)
	}{
		{
			name:     "SuccessfulGetBranch",
			orgSlug:  "test-org",
			repoSlug: "test-repo",
			branch:   "main",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				resp := ListRepoBranchesResponse{
					Branches: []*Branch{
						{Name: "main", Commit: &Commit{Hash: "abc123"}},
					},
				}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkResponse: func(t *testing.T, branch *Branch) {
				t.Helper()
				assert.NotNil(t, branch)
				assert.Equal(t, "main", branch.Name)
				assert.Equal(t, "abc123", branch.Commit.Hash)
			},
		},
		{
			name:     "BranchNotFound",
			orgSlug:  "test-org",
			repoSlug: "test-repo",
			branch:   "nonexistent",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				resp := ListRepoBranchesResponse{
					Branches: []*Branch{},
				}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkResponse: func(t *testing.T, branch *Branch) {
				t.Helper()
				assert.Nil(t, branch)
			},
		},
		{
			name:     "ServerError",
			orgSlug:  "test-org",
			repoSlug: "test-repo",
			branch:   "main",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, err := w.Write([]byte(`{"message":"repository not found"}`))
				require.NoError(t, err)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			client, err := NewClient(server.URL)
			require.NoError(t, err)

			branch, _, err := client.GetRepoBranch(context.Background(), tt.orgSlug, tt.repoSlug, tt.branch)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkResponse != nil {
					tt.checkResponse(t, branch)
				}
			}
		})
	}
}

func TestClient_ListRepoLabels(t *testing.T) {
	tests := []struct {
		name           string
		orgSlug        string
		repoSlug       string
		options        ListLabelsOptions
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		checkResponse  func(t *testing.T, resp *ListRepoLabelsResponse)
		checkRequest   func(t *testing.T, r *http.Request)
	}{
		{
			name:     "SuccessfulListLabels",
			orgSlug:  "test-org",
			repoSlug: "test-repo",
			options:  ListLabelsOptions{},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				now := time.Now()
				resp := ListRepoLabelsResponse{
					Labels: []*Label{
						{
							Id:        "label1",
							Name:      "bug",
							Slug:      "bug",
							Color:     "#ff0000",
							CreatedAt: &now,
						},
						{
							Id:        "label2",
							Name:      "feature",
							Slug:      "feature",
							Color:     "#00ff00",
							CreatedAt: &now,
						},
					},
					NextPageToken: "next-token",
				}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkResponse: func(t *testing.T, resp *ListRepoLabelsResponse) {
				t.Helper()
				assert.Len(t, resp.Labels, 2)
				assert.Equal(t, "bug", resp.Labels[0].Name)
				assert.Equal(t, "feature", resp.Labels[1].Name)
				assert.Equal(t, "next-token", resp.NextPageToken)
			},
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				assert.Equal(t, "/repos/test-org/test-repo/labels", r.URL.Path)
			},
		},
		{
			name:     "WithPaginationAndFilter",
			orgSlug:  "test-org",
			repoSlug: "test-repo",
			options: ListLabelsOptions{
				ListOptions: ListOptions{
					PageSize: 5,
					Filter:   "bug",
					SortBy:   "name",
				},
			},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				resp := ListRepoLabelsResponse{Labels: []*Label{}}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				query := r.URL.Query()
				assert.Equal(t, "bug", query.Get("filter"))
				assert.Equal(t, "name", query.Get("sort_by"))
			},
		},
		{
			name:     "EmptyOrgSlug",
			orgSlug:  "",
			repoSlug: "test-repo",
			options:  ListLabelsOptions{},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedRequest *http.Request
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedRequest = r
				tt.serverResponse(w, r)
			}))
			defer server.Close()

			client, err := NewClient(server.URL)
			require.NoError(t, err)

			resp, _, err := client.ListRepoLabels(context.Background(), tt.orgSlug, tt.repoSlug, tt.options)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkResponse != nil {
					tt.checkResponse(t, resp)
				}
				if tt.checkRequest != nil {
					tt.checkRequest(t, capturedRequest)
				}
			}
		})
	}
}

func TestClient_ListRepoFileTree(t *testing.T) {
	recursiveTrue := true
	recursiveFalse := false

	tests := []struct {
		name           string
		orgSlug        string
		repoSlug       string
		revision       string
		path           string
		options        ListRepoFileTreeOptions
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		checkResponse  func(t *testing.T, resp *ListRepoFileTreeResponse)
		checkRequest   func(t *testing.T, r *http.Request)
	}{
		{
			name:     "SuccessfulListFileTree",
			orgSlug:  "test-org",
			repoSlug: "test-repo",
			revision: "main",
			path:     "/src",
			options:  ListRepoFileTreeOptions{},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				resp := ListRepoFileTreeResponse{
					Trees: []*TreeEntry{
						{Name: "file1.go", Path: "/src/file1.go", Type: "file"},
						{Name: "file2.go", Path: "/src/file2.go", Type: "file"},
						{Name: "subdir", Path: "/src/subdir", Type: "directory"},
					},
					NextPageToken: "",
				}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkResponse: func(t *testing.T, resp *ListRepoFileTreeResponse) {
				t.Helper()
				assert.Len(t, resp.Trees, 3)
				assert.Equal(t, "file1.go", resp.Trees[0].Name)
				assert.Equal(t, "file", resp.Trees[0].Type)
				assert.Equal(t, "directory", resp.Trees[2].Type)
			},
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				assert.Equal(t, "/repos/test-org/test-repo/trees", r.URL.Path)
				query := r.URL.Query()
				assert.Equal(t, "main", query.Get("revision"))
				assert.Equal(t, "/src", query.Get("path"))
			},
		},
		{
			name:     "WithRecursiveTrue",
			orgSlug:  "test-org",
			repoSlug: "test-repo",
			revision: "develop",
			path:     "/",
			options: ListRepoFileTreeOptions{
				Recursive: &recursiveTrue,
			},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				resp := ListRepoFileTreeResponse{Trees: []*TreeEntry{}}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				assert.Equal(t, "true", r.URL.Query().Get("recursive"))
			},
		},
		{
			name:     "WithRecursiveFalse",
			orgSlug:  "test-org",
			repoSlug: "test-repo",
			revision: "main",
			path:     "/",
			options: ListRepoFileTreeOptions{
				Recursive: &recursiveFalse,
			},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				resp := ListRepoFileTreeResponse{Trees: []*TreeEntry{}}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				assert.Equal(t, "false", r.URL.Query().Get("recursive"))
			},
		},
		{
			name:     "WithPagination",
			orgSlug:  "test-org",
			repoSlug: "test-repo",
			revision: "main",
			path:     "/",
			options: ListRepoFileTreeOptions{
				ListOptions: ListOptions{
					PageToken: "token123",
					PageSize:  50,
				},
			},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				resp := ListRepoFileTreeResponse{Trees: []*TreeEntry{}}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				query := r.URL.Query()
				assert.Equal(t, "token123", query.Get("page_token"))
				assert.Equal(t, "50", query.Get("page_size"))
			},
		},
		{
			name:     "EmptyOrgSlug",
			orgSlug:  "",
			repoSlug: "test-repo",
			revision: "main",
			path:     "/",
			options:  ListRepoFileTreeOptions{},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedRequest *http.Request
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedRequest = r
				tt.serverResponse(w, r)
			}))
			defer server.Close()

			client, err := NewClient(server.URL)
			require.NoError(t, err)

			resp, _, err := client.ListRepoFileTree(context.Background(), tt.orgSlug, tt.repoSlug, tt.revision, tt.path, tt.options)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkResponse != nil {
					tt.checkResponse(t, resp)
				}
				if tt.checkRequest != nil {
					tt.checkRequest(t, capturedRequest)
				}
			}
		})
	}
}

func TestClient_ListRepoPullRequests(t *testing.T) {
	tests := []struct {
		name           string
		orgSlug        string
		repoSlug       string
		options        ListRepoPullRequestsOptions
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		checkResponse  func(t *testing.T, resp *ListRepoPullRequestsResponse)
		checkRequest   func(t *testing.T, r *http.Request)
	}{
		{
			name:     "SuccessfulListPullRequests",
			orgSlug:  "test-org",
			repoSlug: "test-repo",
			options:  ListRepoPullRequestsOptions{},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				now := time.Now()
				resp := ListRepoPullRequestsResponse{
					PullRequests: []*PullRequest{
						{
							Id:           "pr1",
							Slug:         "1",
							Title:        "Add feature",
							Status:       "open",
							SourceBranch: "feature",
							TargetBranch: "main",
							CreatedAt:    &now,
						},
						{
							Id:           "pr2",
							Slug:         "2",
							Title:        "Fix bug",
							Status:       "merged",
							SourceBranch: "bugfix",
							TargetBranch: "main",
							CreatedAt:    &now,
						},
					},
					NextPageToken: "next-pr-token",
				}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkResponse: func(t *testing.T, resp *ListRepoPullRequestsResponse) {
				t.Helper()
				assert.Len(t, resp.PullRequests, 2)
				assert.Equal(t, "Add feature", resp.PullRequests[0].Title)
				assert.Equal(t, "open", resp.PullRequests[0].Status)
				assert.Equal(t, "Fix bug", resp.PullRequests[1].Title)
				assert.Equal(t, "merged", resp.PullRequests[1].Status)
				assert.Equal(t, "next-pr-token", resp.NextPageToken)
			},
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				assert.Equal(t, "/repos/test-org/test-repo/pulls", r.URL.Path)
				assert.Equal(t, "GET", r.Method)
			},
		},
		{
			name:     "WithFilterAndSort",
			orgSlug:  "test-org",
			repoSlug: "test-repo",
			options: ListRepoPullRequestsOptions{
				ListOptions: ListOptions{
					Filter: "open",
					SortBy: "created_at",
				},
			},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				resp := ListRepoPullRequestsResponse{PullRequests: []*PullRequest{}}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				query := r.URL.Query()
				assert.Equal(t, "open", query.Get("filter"))
				assert.Equal(t, "created_at", query.Get("sort_by"))
			},
		},
		{
			name:     "WithPagination",
			orgSlug:  "test-org",
			repoSlug: "test-repo",
			options: ListRepoPullRequestsOptions{
				ListOptions: ListOptions{
					PageToken: "pr-page-2",
					PageSize:  20,
				},
			},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				resp := ListRepoPullRequestsResponse{PullRequests: []*PullRequest{}}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				query := r.URL.Query()
				assert.Equal(t, "pr-page-2", query.Get("page_token"))
				assert.Equal(t, "20", query.Get("page_size"))
			},
		},
		{
			name:     "EmptyRepoSlug",
			orgSlug:  "test-org",
			repoSlug: "",
			options:  ListRepoPullRequestsOptions{},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr: true,
		},
		{
			name:     "ServerError",
			orgSlug:  "test-org",
			repoSlug: "test-repo",
			options:  ListRepoPullRequestsOptions{},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_, err := w.Write([]byte(`{"message":"access denied"}`))
				require.NoError(t, err)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedRequest *http.Request
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedRequest = r
				tt.serverResponse(w, r)
			}))
			defer server.Close()

			client, err := NewClient(server.URL)
			require.NoError(t, err)

			resp, _, err := client.ListRepoPullRequests(context.Background(), tt.orgSlug, tt.repoSlug, tt.options)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkResponse != nil {
					tt.checkResponse(t, resp)
				}
				if tt.checkRequest != nil {
					tt.checkRequest(t, capturedRequest)
				}
			}
		})
	}
}

func TestListRepoFileTreeOptions_getURLQuery(t *testing.T) {
	recursiveTrue := true
	recursiveFalse := false

	tests := []struct {
		name     string
		options  ListRepoFileTreeOptions
		expected map[string]string
	}{
		{
			name:    "EmptyOptions",
			options: ListRepoFileTreeOptions{},
			expected: map[string]string{
				"recursive": "",
			},
		},
		{
			name: "WithRecursiveTrue",
			options: ListRepoFileTreeOptions{
				Recursive: &recursiveTrue,
			},
			expected: map[string]string{
				"recursive": "true",
			},
		},
		{
			name: "WithRecursiveFalse",
			options: ListRepoFileTreeOptions{
				Recursive: &recursiveFalse,
			},
			expected: map[string]string{
				"recursive": "false",
			},
		},
		{
			name: "WithAllOptions",
			options: ListRepoFileTreeOptions{
				ListOptions: ListOptions{
					PageToken: "token",
					PageSize:  10,
					Filter:    "*.go",
					SortBy:    "name",
				},
				Recursive: &recursiveTrue,
			},
			expected: map[string]string{
				"page_token": "token",
				"page_size":  "10",
				"filter":     "*.go",
				"sort_by":    "name",
				"recursive":  "true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := tt.options.getURLQuery()
			for key, expectedValue := range tt.expected {
				if expectedValue == "" {
					assert.Empty(t, query.Get(key), "Expected %s to be empty", key)
				} else {
					assert.Equal(t, expectedValue, query.Get(key), "Mismatch for key %s", key)
				}
			}
		})
	}
}
