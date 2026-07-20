package webhook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-playground/webhooks/v6/azuredevops"
	"github.com/go-playground/webhooks/v6/github"
	"github.com/go-playground/webhooks/v6/gitlab"
	gogsclient "github.com/gogits/go-gogs-client"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/settings"
)

func TestGetPushEventInfo(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		payload  any
		expected PushEventInfo
	}{
		{
			name:    "GitHub push",
			file:    "testdata/github-commit-event.json",
			payload: &github.PushPayload{},
			expected: PushEventInfo{
				RepositoryURLs: []string{"https://github.com/jessesuen/test-repo"},
				Revision:       "master",
				TouchedHead:    true,
				BeforeSHA:      "d5c1ffa8e294bc18c639bfb4e0df499251034414",
				AfterSHA:       "63738bb582c8b540af7bcfc18f87c575c3ed66e0",
				ChangedFiles:   []string{"ksapps/test-app/environments/staging-argocd-demo/main.jsonnet", "ksapps/test-app/environments/staging-argocd-demo/params.libsonnet", "ksapps/test-app/app.yaml"},
			},
		},
		{
			name:    "GitLab push",
			file:    "testdata/gitlab-event.json",
			payload: &gitlab.PushEventPayload{},
			expected: PushEventInfo{
				RepositoryURLs: []string{"https://gitlab.com/group/name"},
				Revision:       "master",
				TouchedHead:    true,
				BeforeSHA:      "e5ba5f6c13b64670048daa88e4c053d60b0e115a",
				AfterSHA:       "bb0748feaa336d841c251017e4e374c22d0c8a98",
				ChangedFiles:   []string{"file.yaml"},
			},
		},
		{
			name:    "Azure DevOps push",
			file:    "testdata/azuredevops-git-push-event.json",
			payload: &azuredevops.GitPushEvent{},
			expected: PushEventInfo{
				RepositoryURLs: []string{"https://dev.azure.com/alexander0053/alex-test/_git/alex-test"},
				Revision:       "master",
				TouchedHead:    true,
				BeforeSHA:      "fa51eeb1e50b98293ce281e6d5492b9decae613b",
				AfterSHA:       "298a79aa1552799a70718a0ee914d153d5a1a76b",
			},
		},
		{
			name:    "Gogs push",
			file:    "testdata/gogs-event.json",
			payload: &gogsclient.PushPayload{},
			expected: PushEventInfo{
				RepositoryURLs: []string{"http://gogs-server/john/repo-test"},
				Revision:       "master",
				TouchedHead:    true,
				BeforeSHA:      "0000000000000000000000000000000000000000",
				AfterSHA:       "0a05129851238652bf806a400af89fa974ade739",
				ChangedFiles:   []string{"cm.yaml"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(tt.file)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(data, tt.payload))

			require.Equal(t, &tt.expected, GetPushEventInfo(derefPushPayload(tt.payload)))
		})
	}
}

func TestPayloadParserParseApplicationPushEvents(t *testing.T) {
	parser, err := NewPayloadParser(&settings.ArgoCDSettings{})
	require.NoError(t, err)

	tests := []struct {
		name        string
		file        string
		header      string
		headerValue string
		provider    WebhookProvider
		payloadType any
	}{
		{"GitHub", "testdata/github-commit-event.json", "X-GitHub-Event", "push", WebhookProviderGitHub, github.PushPayload{}},
		{"GitLab", "testdata/gitlab-event.json", "X-Gitlab-Event", "Push Hook", WebhookProviderGitLab, gitlab.PushEventPayload{}},
		{"Azure DevOps", "testdata/azuredevops-git-push-event.json", "X-Vss-Activityid", "test", WebhookProviderAzureDevOps, azuredevops.GitPushEvent{}},
		{"Gogs", "testdata/gogs-event.json", "X-Gogs-Event", "push", WebhookProviderGogs, gogsclient.PushPayload{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(tt.file)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/api/webhook", bytes.NewReader(data))
			req.Header.Set(tt.header, tt.headerValue)

			payload, provider, err := parser.Parse(req, WebhookConsumerApplication)
			require.NoError(t, err)
			require.Equal(t, tt.provider, provider)
			require.IsType(t, tt.payloadType, payload)
		})
	}
}

func derefPushPayload(payload any) any {
	switch p := payload.(type) {
	case *github.PushPayload:
		return *p
	case *gitlab.PushEventPayload:
		return *p
	case *azuredevops.GitPushEvent:
		return *p
	case *gogsclient.PushPayload:
		return *p
	default:
		return payload
	}
}
