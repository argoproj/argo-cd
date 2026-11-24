package webhook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-playground/webhooks/v6/azuredevops"
	"github.com/go-playground/webhooks/v6/bitbucket"
	bitbucketserver "github.com/go-playground/webhooks/v6/bitbucket-server"
	"github.com/go-playground/webhooks/v6/github"
	"github.com/go-playground/webhooks/v6/gitlab"
	gogsclient "github.com/gogits/go-gogs-client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadPayload[T any](t *testing.T, file string) T {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", file))
	require.NoError(t, err)
	var payload T
	require.NoError(t, json.Unmarshal(data, &payload))
	return payload
}

func TestParsePushEvent(t *testing.T) {
	// Bitbucket Cloud has no fixture, and its payload is built from anonymous
	// structs, so decode a minimal payload instead.
	var bitbucketPush bitbucket.RepoPushPayload
	require.NoError(t, json.Unmarshal([]byte(`{
		"repository": {"links": {"html": {"href": "https://bitbucket.org/org/repo"}}},
		"push": {"changes": [
			{"new": {"name": "main", "target": {"hash": "new-sha"}}, "old": {"target": {"hash": "old-sha"}}},
			{"new": {"name": "other", "target": {"hash": "ignored"}}, "old": {"target": {"hash": "ignored"}}}
		]}
	}`), &bitbucketPush))

	tests := []struct {
		name    string
		payload any
		want    *PushEventInfo
	}{
		{
			name:    "GitHub push",
			payload: loadPayload[github.PushPayload](t, "github-commit-event.json"),
			want: &PushEventInfo{
				WebURLs:     []string{"https://github.com/jessesuen/test-repo"},
				Revision:    "master",
				SHABefore:   "d5c1ffa8e294bc18c639bfb4e0df499251034414",
				SHAAfter:    "63738bb582c8b540af7bcfc18f87c575c3ed66e0",
				TouchedHead: true,
				ChangedFiles: []string{
					"ksapps/test-app/environments/staging-argocd-demo/main.jsonnet",
					"ksapps/test-app/environments/staging-argocd-demo/params.libsonnet",
					"ksapps/test-app/app.yaml",
				},
			},
		},
		{
			name:    "GitLab push",
			payload: loadPayload[gitlab.PushEventPayload](t, "gitlab-event.json"),
			want: &PushEventInfo{
				WebURLs:      []string{"https://gitlab.com/group/name"},
				Revision:     "master",
				SHABefore:    "e5ba5f6c13b64670048daa88e4c053d60b0e115a",
				SHAAfter:     "bb0748feaa336d841c251017e4e374c22d0c8a98",
				TouchedHead:  true,
				ChangedFiles: []string{"file.yaml"},
			},
		},
		{
			name: "GitLab tag named like the default branch does not touch HEAD",
			payload: gitlab.TagEventPayload{
				Ref:     "refs/tags/main",
				Project: gitlab.Project{WebURL: "https://gitlab.com/group/name", DefaultBranch: "main"},
			},
			want: &PushEventInfo{
				WebURLs:     []string{"https://gitlab.com/group/name"},
				Revision:    "main",
				TouchedHead: false,
			},
		},
		{
			name:    "Azure DevOps push",
			payload: loadPayload[azuredevops.GitPushEvent](t, "azuredevops-git-push-event.json"),
			want: &PushEventInfo{
				WebURLs:     []string{"https://dev.azure.com/alexander0053/alex-test/_git/alex-test"},
				Revision:    "master",
				SHABefore:   "fa51eeb1e50b98293ce281e6d5492b9decae613b",
				SHAAfter:    "298a79aa1552799a70718a0ee914d153d5a1a76b",
				TouchedHead: true,
			},
		},
		{
			name: "Azure DevOps push without ref updates",
			payload: azuredevops.GitPushEvent{Resource: azuredevops.Resource{
				Repository: azuredevops.Repository{RemoteURL: "https://dev.azure.com/org/project/_git/repo"},
			}},
			want: &PushEventInfo{
				WebURLs: []string{"https://dev.azure.com/org/project/_git/repo"},
			},
		},
		{
			name:    "Bitbucket Cloud push uses the first change",
			payload: bitbucketPush,
			want: &PushEventInfo{
				WebURLs:     []string{"https://bitbucket.org/org/repo"},
				Revision:    "main",
				SHABefore:   "old-sha",
				SHAAfter:    "new-sha",
				TouchedHead: true,
			},
		},
		{
			name:    "Bitbucket Server push",
			payload: loadPayload[bitbucketserver.RepositoryReferenceChangedPayload](t, "bitbucket-server-event.json"),
			want: &PushEventInfo{
				WebURLs: []string{
					"ssh://git@bitbucketserver:7999/myproject/test-repo.git",
					"https://bitbucketserver/scm/myproject/test-repo.git",
				},
				Revision:    "master",
				SHABefore:   "f09c8889a2d234985734958795a31589cd91ffda",
				SHAAfter:    "22671f0349857934857983457983475ec39f196b",
				TouchedHead: true,
			},
		},
		{
			name:    "Gogs push",
			payload: loadPayload[gogsclient.PushPayload](t, "gogs-event.json"),
			want: &PushEventInfo{
				WebURLs:      []string{"http://gogs-server/john/repo-test"},
				Revision:     "master",
				SHABefore:    "0000000000000000000000000000000000000000",
				SHAAfter:     "0a05129851238652bf806a400af89fa974ade739",
				TouchedHead:  true,
				ChangedFiles: []string{"cm.yaml"},
			},
		},
		{
			name:    "not a push event",
			payload: github.PullRequestPayload{},
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ParsePushEvent(tt.payload))
		})
	}
}

func TestBitbucketServerCloneURLs(t *testing.T) {
	payload := loadPayload[bitbucketserver.RepositoryReferenceChangedPayload](t, "bitbucket-server-event.json")
	httpURL, urls := bitbucketServerCloneURLs(payload.Repository)
	assert.Equal(t, "https://bitbucketserver/scm/myproject/test-repo.git", httpURL)
	assert.Equal(t, []string{
		"ssh://git@bitbucketserver:7999/myproject/test-repo.git",
		"https://bitbucketserver/scm/myproject/test-repo.git",
	}, urls)

	httpURL, urls = bitbucketServerCloneURLs(bitbucketserver.Repository{})
	assert.Empty(t, httpURL)
	assert.Empty(t, urls)
}
