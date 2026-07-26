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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
			assert.Equal(t, tt.provider, provider)
			assert.IsType(t, tt.payloadType, payload)
		})
	}
}

type providerParserMock struct {
	mock.Mock
}

func (p *providerParserMock) CanHandle(r *http.Request) bool {
	args := p.Called(r)
	return args.Bool(0)
}

func (p *providerParserMock) Parse(r *http.Request, consumer WebhookConsumer) (any, error) {
	args := p.Called(r, consumer)
	return args.Get(0), args.Error(1)
}

func (p *providerParserMock) Name() WebhookProvider {
	args := p.Called()
	return args.Get(0).(WebhookProvider)
}

func TestDispatch(t *testing.T) {
	expectedErr := errors.New("parse failed")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", http.NoBody)
	first := &providerParserMock{}
	first.On("CanHandle", req).Return(false).Once()
	matching := &providerParserMock{}
	matching.On("CanHandle", req).Return(true).Once()
	matching.On("Parse", req, WebhookConsumerApplication).Return("payload", expectedErr).Once()
	matching.On("Name").Return(WebhookProviderGitHub).Once()
	unreached := &providerParserMock{}

	payload, provider, err := Dispatch([]ProviderParser{first, matching, unreached}, req, WebhookConsumerApplication)

	assert.Equal(t, "payload", payload)
	assert.Equal(t, WebhookProviderGitHub, provider)
	assert.ErrorIs(t, err, expectedErr)
	first.AssertExpectations(t)
	first.AssertNotCalled(t, "Parse", mock.Anything, mock.Anything)
	matching.AssertExpectations(t)
	unreached.AssertNotCalled(t, "CanHandle", mock.Anything)
	unreached.AssertNotCalled(t, "Parse", mock.Anything, mock.Anything)
}

func TestDispatchNoMatch(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", http.NoBody)
	parser := &providerParserMock{}
	parser.On("CanHandle", req).Return(false).Once()

	payload, provider, err := Dispatch([]ProviderParser{parser}, req, WebhookConsumerApplication)

	require.NoError(t, err)
	assert.Nil(t, payload)
	assert.Empty(t, provider)
	parser.AssertExpectations(t)
	parser.AssertNotCalled(t, "Parse", mock.Anything, mock.Anything)
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
			assert.Equal(t, tt.expected, names)
		})
	}
}

func TestNewProviderParsersContinuesAfterFailure(t *testing.T) {
	expectedErr := errors.New("broken provider")
	healthy := &providerParserMock{}
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

	assert.ErrorIs(t, err, expectedErr)
	assert.Equal(t, []ProviderParser{healthy}, parsers)
}

func TestNewProviderParsersSkipsUnsupportedFactories(t *testing.T) {
	unsupportedCalled := false
	healthy := &providerParserMock{}
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
	assert.False(t, unsupportedCalled)
	assert.Equal(t, []ProviderParser{healthy}, parsers)
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
		assert.Equal(t, WebhookProviderGogs, provider)
	})

	t.Run("Gogs is unknown for ApplicationSet", func(t *testing.T) {
		parsers, err := NewProviderParsers(&settings.ArgoCDSettings{}, WebhookConsumerApplicationSet)
		require.NoError(t, err)
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", http.NoBody)
		req.Header.Set("X-Gogs-Event", "push")
		req.Header.Set("X-GitHub-Event", "push")

		payload, provider, err := Dispatch(parsers, req, WebhookConsumerApplicationSet)

		require.NoError(t, err)
		assert.Nil(t, payload)
		assert.Empty(t, provider)
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
		assert.Equal(t, WebhookProviderGHCR, provider)
	})
}
