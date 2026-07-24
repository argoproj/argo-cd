package webhook

import (
	"bytes"
	"encoding/json"
	"errors"
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

func TestGetPushEventInfoUnsupportedPayload(t *testing.T) {
	require.Nil(t, GetPushEventInfo(struct{}{}))
}

func TestGetPushEventInfoAzureDevOpsWithoutRefUpdates(t *testing.T) {
	payload := azuredevops.GitPushEvent{}
	payload.Resource.Repository.RemoteURL = "https://dev.azure.com/example/project/_git/repo"

	require.Equal(t, &PushEventInfo{RepositoryURLs: []string{payload.Resource.Repository.RemoteURL}}, GetPushEventInfo(payload))
}

func TestDispatchApplicationPushEvents(t *testing.T) {
	parsers, err := NewProviderParsers(&settings.ArgoCDSettings{}, WebhookConsumerApplication)
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
		{"GHCR", "testdata/ghcr-package-event.json", "X-GitHub-Event", "package", WebhookProviderGHCR, &RegistryEvent{}},
		{"GitLab", "testdata/gitlab-event.json", "X-Gitlab-Event", "Push Hook", WebhookProviderGitLab, gitlab.PushEventPayload{}},
		{"Azure DevOps", "testdata/azuredevops-git-push-event.json", "X-Vss-Activityid", "test", WebhookProviderAzureDevOps, azuredevops.GitPushEvent{}},
		{"Gogs", "testdata/gogs-event.json", "X-Gogs-Event", "push", WebhookProviderGogs, gogsclient.PushPayload{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(tt.file)
			require.NoError(t, err)
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook", bytes.NewReader(data))
			req.Header.Set(tt.header, tt.headerValue)

			payload, provider, err := Dispatch(parsers, req, WebhookConsumerApplication)
			require.NoError(t, err)
			require.Equal(t, tt.provider, provider)
			require.IsType(t, tt.payloadType, payload)
		})
	}
}

type fakeProviderParser struct {
	name       WebhookProvider
	match      bool
	payload    any
	err        error
	canCalls   int
	parseCalls int
}

func (p *fakeProviderParser) CanHandle(_ *http.Request) bool {
	p.canCalls++
	return p.match
}

func (p *fakeProviderParser) Parse(_ *http.Request, _ WebhookConsumer) (any, error) {
	p.parseCalls++
	return p.payload, p.err
}

func (p *fakeProviderParser) Name() WebhookProvider {
	return p.name
}

func TestDispatch(t *testing.T) {
	expectedErr := errors.New("parse failed")
	first := &fakeProviderParser{name: WebhookProviderGitLab}
	matching := &fakeProviderParser{name: WebhookProviderGitHub, match: true, payload: "payload", err: expectedErr}
	unreached := &fakeProviderParser{name: WebhookProviderGogs, match: true}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", http.NoBody)

	payload, provider, err := Dispatch([]ProviderParser{first, matching, unreached}, req, WebhookConsumerApplication)

	require.Equal(t, "payload", payload)
	require.Equal(t, WebhookProviderGitHub, provider)
	require.ErrorIs(t, err, expectedErr)
	require.Equal(t, 1, first.canCalls)
	require.Zero(t, first.parseCalls)
	require.Equal(t, 1, matching.canCalls)
	require.Equal(t, 1, matching.parseCalls)
	require.Zero(t, unreached.canCalls)
	require.Zero(t, unreached.parseCalls)
}

func TestDispatchNoMatch(t *testing.T) {
	parser := &fakeProviderParser{name: WebhookProviderGitHub}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", http.NoBody)

	payload, provider, err := Dispatch([]ProviderParser{parser}, req, WebhookConsumerApplication)

	require.NoError(t, err)
	require.Nil(t, payload)
	require.Empty(t, provider)
	require.Equal(t, 1, parser.canCalls)
	require.Zero(t, parser.parseCalls)
}

func TestNewProviderParsers(t *testing.T) {
	tests := []struct {
		name     string
		consumer WebhookConsumer
		expected []WebhookProvider
	}{
		{
			name:     "Application",
			consumer: WebhookConsumerApplication,
			expected: []WebhookProvider{WebhookProviderAzureDevOps, WebhookProviderGogs, WebhookProviderGitHub, WebhookProviderGitLab, WebhookProviderBitbucket, WebhookProviderBitbucketServer, WebhookProviderGHCR},
		},
		{
			name:     "ApplicationSet",
			consumer: WebhookConsumerApplicationSet,
			expected: []WebhookProvider{WebhookProviderAzureDevOps, WebhookProviderGitHub, WebhookProviderGitLab},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsers, err := NewProviderParsers(&settings.ArgoCDSettings{}, tt.consumer)
			require.NoError(t, err)
			names := make([]WebhookProvider, 0, len(parsers))
			for _, parser := range parsers {
				names = append(names, parser.Name())
			}
			require.Equal(t, tt.expected, names)
		})
	}
}

func TestNewProviderParsersContinuesAfterFailure(t *testing.T) {
	expectedErr := errors.New("broken provider")
	healthy := &fakeProviderParser{name: WebhookProviderGitHub}
	factories := []providerFactory{
		{
			name:      WebhookProviderGitLab,
			consumers: []WebhookConsumer{WebhookConsumerApplication},
			new: func() (ProviderParser, error) {
				return nil, expectedErr
			},
		},
		{
			name:      WebhookProviderGitHub,
			consumers: []WebhookConsumer{WebhookConsumerApplication},
			new: func() (ProviderParser, error) {
				return healthy, nil
			},
		},
	}

	parsers, err := newProviderParsers(WebhookConsumerApplication, factories)

	require.ErrorIs(t, err, expectedErr)
	require.Equal(t, []ProviderParser{healthy}, parsers)
}

func TestNewProviderParsersSkipsUnsupportedFactories(t *testing.T) {
	unsupportedCalled := false
	healthy := &fakeProviderParser{name: WebhookProviderGitHub}
	factories := []providerFactory{
		{
			name:      WebhookProviderGogs,
			consumers: []WebhookConsumer{WebhookConsumerApplication},
			new: func() (ProviderParser, error) {
				unsupportedCalled = true
				return nil, nil
			},
		},
		{
			name:      WebhookProviderGitHub,
			consumers: []WebhookConsumer{WebhookConsumerApplicationSet},
			new: func() (ProviderParser, error) {
				return healthy, nil
			},
		},
	}

	parsers, err := newProviderParsers(WebhookConsumerApplicationSet, factories)

	require.NoError(t, err)
	require.False(t, unsupportedCalled)
	require.Equal(t, []ProviderParser{healthy}, parsers)
}

func TestProviderDisambiguation(t *testing.T) {
	t.Run("Gogs wins over GitHub for Application", func(t *testing.T) {
		parsers, err := NewProviderParsers(&settings.ArgoCDSettings{}, WebhookConsumerApplication)
		require.NoError(t, err)
		data, err := os.ReadFile("testdata/gogs-event.json")
		require.NoError(t, err)
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", bytes.NewReader(data))
		req.Header.Set("X-Gogs-Event", "push")
		req.Header.Set("X-GitHub-Event", "push")

		_, provider, err := Dispatch(parsers, req, WebhookConsumerApplication)

		require.NoError(t, err)
		require.Equal(t, WebhookProviderGogs, provider)
	})

	t.Run("Gogs is unknown for ApplicationSet", func(t *testing.T) {
		parsers, err := NewProviderParsers(&settings.ArgoCDSettings{}, WebhookConsumerApplicationSet)
		require.NoError(t, err)
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", http.NoBody)
		req.Header.Set("X-Gogs-Event", "push")
		req.Header.Set("X-GitHub-Event", "push")

		payload, provider, err := Dispatch(parsers, req, WebhookConsumerApplicationSet)

		require.NoError(t, err)
		require.Nil(t, payload)
		require.Empty(t, provider)
	})

	t.Run("GHCR package is not GitHub", func(t *testing.T) {
		parsers, err := NewProviderParsers(&settings.ArgoCDSettings{}, WebhookConsumerApplication)
		require.NoError(t, err)
		data, err := os.ReadFile("testdata/ghcr-package-event.json")
		require.NoError(t, err)
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", bytes.NewReader(data))
		req.Header.Set("X-GitHub-Event", "package")

		_, provider, err := Dispatch(parsers, req, WebhookConsumerApplication)

		require.NoError(t, err)
		require.Equal(t, WebhookProviderGHCR, provider)
	})
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
