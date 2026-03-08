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

func TestClient_ListOrgRepos(t *testing.T) {
	tests := []struct {
		name           string
		orgSlug        string
		options        ListOrgReposOptions
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		checkResponse  func(t *testing.T, resp *ListOrgReposResponse)
		checkRequest   func(t *testing.T, r *http.Request)
	}{
		{
			name:    "SuccessfulListRepos",
			orgSlug: "test-org",
			options: ListOrgReposOptions{},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				now := time.Now()
				resp := ListOrgReposResponse{
					Repositories: []*Repository{
						{
							Id:            "repo1",
							Name:          "test-repo-1",
							Slug:          "test-repo-1",
							DefaultBranch: "main",
							Description:   "First test repository",
							Visibility:    "public",
							IsEmpty:       false,
							LastUpdated:   &now,
							Organization: &OrganizationEmbedded{
								Id:   "org1",
								Slug: "test-org",
							},
						},
						{
							Id:            "repo2",
							Name:          "test-repo-2",
							Slug:          "test-repo-2",
							DefaultBranch: "develop",
							Description:   "Second test repository",
							Visibility:    "private",
							IsEmpty:       true,
							LastUpdated:   &now,
							Organization: &OrganizationEmbedded{
								Id:   "org1",
								Slug: "test-org",
							},
						},
					},
					NextPageToken: "next-repos-token",
				}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkResponse: func(t *testing.T, resp *ListOrgReposResponse) {
				t.Helper()
				assert.Len(t, resp.Repositories, 2)
				assert.Equal(t, "test-repo-1", resp.Repositories[0].Name)
				assert.Equal(t, "main", resp.Repositories[0].DefaultBranch)
				assert.Equal(t, "public", resp.Repositories[0].Visibility)
				assert.False(t, resp.Repositories[0].IsEmpty)
				assert.Equal(t, "test-repo-2", resp.Repositories[1].Name)
				assert.Equal(t, "develop", resp.Repositories[1].DefaultBranch)
				assert.Equal(t, "private", resp.Repositories[1].Visibility)
				assert.True(t, resp.Repositories[1].IsEmpty)
				assert.Equal(t, "next-repos-token", resp.NextPageToken)
			},
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				assert.Equal(t, "/orgs/test-org/repos", r.URL.Path)
				assert.Equal(t, "GET", r.Method)
			},
		},
		{
			name:    "WithPagination",
			orgSlug: "test-org",
			options: ListOrgReposOptions{
				ListOptions: ListOptions{
					PageToken: "page-token-123",
					PageSize:  25,
				},
			},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				resp := ListOrgReposResponse{
					Repositories:  []*Repository{},
					NextPageToken: "",
				}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				query := r.URL.Query()
				assert.Equal(t, "page-token-123", query.Get("page_token"))
				assert.Equal(t, "25", query.Get("page_size"))
			},
		},
		{
			name:    "WithFilter",
			orgSlug: "test-org",
			options: ListOrgReposOptions{
				ListOptions: ListOptions{
					Filter: "test",
				},
			},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				resp := ListOrgReposResponse{
					Repositories: []*Repository{
						{
							Id:   "repo1",
							Name: "test-repo",
							Slug: "test-repo",
						},
					},
				}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				assert.Equal(t, "test", r.URL.Query().Get("filter"))
			},
			checkResponse: func(t *testing.T, resp *ListOrgReposResponse) {
				t.Helper()
				assert.Len(t, resp.Repositories, 1)
				assert.Equal(t, "test-repo", resp.Repositories[0].Name)
			},
		},
		{
			name:    "WithSortBy",
			orgSlug: "test-org",
			options: ListOrgReposOptions{
				ListOptions: ListOptions{
					SortBy: "name",
				},
			},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				resp := ListOrgReposResponse{Repositories: []*Repository{}}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				assert.Equal(t, "name", r.URL.Query().Get("sort_by"))
			},
		},
		{
			name:    "WithAllOptions",
			orgSlug: "test-org",
			options: ListOrgReposOptions{
				ListOptions: ListOptions{
					PageToken: "token",
					PageSize:  10,
					Filter:    "active",
					SortBy:    "updated_at",
				},
			},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				resp := ListOrgReposResponse{Repositories: []*Repository{}}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				query := r.URL.Query()
				assert.Equal(t, "token", query.Get("page_token"))
				assert.Equal(t, "10", query.Get("page_size"))
				assert.Equal(t, "active", query.Get("filter"))
				assert.Equal(t, "updated_at", query.Get("sort_by"))
			},
		},
		{
			name:    "EmptyOrgSlug",
			orgSlug: "",
			options: ListOrgReposOptions{},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr: true,
		},
		{
			name:    "SpecialCharactersInOrgSlug",
			orgSlug: "test org",
			options: ListOrgReposOptions{},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				resp := ListOrgReposResponse{Repositories: []*Repository{}}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				// URL path encoding uses %20 for spaces
				escapedPath := r.URL.EscapedPath()
				assert.Contains(t, escapedPath, "test%20org")
			},
		},
		{
			name:    "ServerError404",
			orgSlug: "nonexistent-org",
			options: ListOrgReposOptions{},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, err := w.Write([]byte(`{"message":"organization not found"}`))
				require.NoError(t, err)
			},
			wantErr: true,
		},
		{
			name:    "ServerError403",
			orgSlug: "private-org",
			options: ListOrgReposOptions{},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_, err := w.Write([]byte(`{"message":"access denied"}`))
				require.NoError(t, err)
			},
			wantErr: true,
		},
		{
			name:    "ServerError500",
			orgSlug: "test-org",
			options: ListOrgReposOptions{},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, err := w.Write([]byte(`{"message":"internal server error"}`))
				require.NoError(t, err)
			},
			wantErr: true,
		},
		{
			name:    "EmptyRepositoriesList",
			orgSlug: "empty-org",
			options: ListOrgReposOptions{},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				resp := ListOrgReposResponse{
					Repositories:  []*Repository{},
					NextPageToken: "",
				}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkResponse: func(t *testing.T, resp *ListOrgReposResponse) {
				t.Helper()
				assert.Empty(t, resp.Repositories)
				assert.Empty(t, resp.NextPageToken)
			},
		},
		{
			name:    "RepositoryWithCompleteMetadata",
			orgSlug: "test-org",
			options: ListOrgReposOptions{},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				now := time.Now()
				resp := ListOrgReposResponse{
					Repositories: []*Repository{
						{
							Id:            "repo1",
							Name:          "complete-repo",
							Slug:          "complete-repo",
							DefaultBranch: "main",
							Description:   "A complete repository with all metadata",
							Visibility:    "public",
							IsEmpty:       false,
							TemplateType:  "standard",
							WebURL:        "https://example.com/test-org/complete-repo",
							LastUpdated:   &now,
							Organization: &OrganizationEmbedded{
								Id:   "org1",
								Slug: "test-org",
							},
							Logo: &Image{
								URL: "https://example.com/logo.png",
							},
							CloneURL: &CloneURL{
								HTTPS: "https://example.com/test-org/complete-repo.git",
								SSH:   "git@example.com:test-org/complete-repo.git",
							},
							Language: &Language{
								Name:  "Go",
								Color: "#00ADD8",
							},
							Counters: &RepositoryCounters{
								Forks:        "5",
								PullRequests: "10",
								Issues:       "3",
								Tags:         "2",
								Branches:     "4",
							},
							Links: []*Link{
								{Link: "https://example.com/docs", Type: "documentation"},
								{Link: "https://example.com/issues", Type: "issues"},
							},
						},
					},
					NextPageToken: "",
				}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkResponse: func(t *testing.T, resp *ListOrgReposResponse) {
				t.Helper()
				assert.Len(t, resp.Repositories, 1)
				repo := resp.Repositories[0]
				assert.Equal(t, "complete-repo", repo.Name)
				assert.NotNil(t, repo.Logo)
				assert.Equal(t, "https://example.com/logo.png", repo.Logo.URL)
				assert.NotNil(t, repo.CloneURL)
				assert.Equal(t, "https://example.com/test-org/complete-repo.git", repo.CloneURL.HTTPS)
				assert.NotNil(t, repo.Language)
				assert.Equal(t, "Go", repo.Language.Name)
				assert.NotNil(t, repo.Counters)
				assert.Equal(t, "5", repo.Counters.Forks)
				assert.Len(t, repo.Links, 2)
			},
		},
		{
			name:    "RepositoryWithParent",
			orgSlug: "test-org",
			options: ListOrgReposOptions{},
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				resp := ListOrgReposResponse{
					Repositories: []*Repository{
						{
							Id:   "repo1",
							Name: "forked-repo",
							Slug: "forked-repo",
							Parent: &RepositoryEmbedded{
								Id:   "parent-repo-id",
								Slug: "parent-repo",
							},
						},
					},
				}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			},
			wantErr: false,
			checkResponse: func(t *testing.T, resp *ListOrgReposResponse) {
				t.Helper()
				assert.Len(t, resp.Repositories, 1)
				assert.NotNil(t, resp.Repositories[0].Parent)
				assert.Equal(t, "parent-repo", resp.Repositories[0].Parent.Slug)
			},
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

			resp, httpResp, err := client.ListOrgRepos(context.Background(), tt.orgSlug, tt.options)
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

func TestClient_ListOrgRepos_WithAuthentication(t *testing.T) {
	var capturedAuthHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuthHeader = r.Header.Get("Authorization")
		resp := ListOrgReposResponse{Repositories: []*Repository{}}
		w.WriteHeader(http.StatusOK)
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer server.Close()

	token := "test-secret-token"
	client, err := NewClient(server.URL, SetToken(token))
	require.NoError(t, err)

	_, _, err = client.ListOrgRepos(context.Background(), "test-org", ListOrgReposOptions{})
	require.NoError(t, err)
	assert.Equal(t, "Bearer "+token, capturedAuthHeader)
}

func TestClient_ListOrgRepos_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, _, err = client.ListOrgRepos(ctx, "test-org", ListOrgReposOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestClient_ListOrgRepos_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{invalid json response}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	_, _, err = client.ListOrgRepos(context.Background(), "test-org", ListOrgReposOptions{})
	assert.Error(t, err)
}

func TestClient_ListOrgRepos_Pagination(t *testing.T) {
	pageCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		pageCount++
		var resp ListOrgReposResponse

		if pageCount == 1 {
			resp = ListOrgReposResponse{
				Repositories: []*Repository{
					{Id: "repo1", Name: "repo-1", Slug: "repo-1"},
					{Id: "repo2", Name: "repo-2", Slug: "repo-2"},
				},
				NextPageToken: "page2",
			}
		} else {
			resp = ListOrgReposResponse{
				Repositories: []*Repository{
					{Id: "repo3", Name: "repo-3", Slug: "repo-3"},
				},
				NextPageToken: "",
			}
		}

		w.WriteHeader(http.StatusOK)
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	// First page
	resp1, _, err := client.ListOrgRepos(context.Background(), "test-org", ListOrgReposOptions{})
	require.NoError(t, err)
	assert.Len(t, resp1.Repositories, 2)
	assert.Equal(t, "page2", resp1.NextPageToken)

	// Second page
	resp2, _, err := client.ListOrgRepos(context.Background(), "test-org", ListOrgReposOptions{
		ListOptions: ListOptions{PageToken: resp1.NextPageToken},
	})
	require.NoError(t, err)
	assert.Len(t, resp2.Repositories, 1)
	assert.Empty(t, resp2.NextPageToken)
}

func TestClient_ListOrgRepos_ConcurrentRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := ListOrgReposResponse{
			Repositories: []*Repository{
				{Id: "repo1", Name: "test-repo", Slug: "test-repo"},
			},
		}
		w.WriteHeader(http.StatusOK)
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	// Make concurrent requests
	done := make(chan bool)
	for range 10 {
		go func() {
			_, _, err := client.ListOrgRepos(context.Background(), "test-org", ListOrgReposOptions{})
			assert.NoError(t, err)
			done <- true
		}()
	}

	for range 10 {
		<-done
	}
}
