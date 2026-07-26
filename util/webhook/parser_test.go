package webhook

import (
	"bytes"
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
