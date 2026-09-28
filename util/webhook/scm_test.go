package webhook

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	bitbucketserver "github.com/go-playground/webhooks/v6/bitbucket-server"
	"github.com/go-playground/webhooks/v6/github"
	"github.com/go-playground/webhooks/v6/gitlab"
	"github.com/go-playground/webhooks/v6/gogs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/settings"
)

func TestNewParsers(t *testing.T) {
	t.Run("registers only enabled providers in dispatch order", func(t *testing.T) {
		parsers := NewParsers(&settings.ArgoCDSettings{}, ParserOptions{
			GitHubEvents: []github.Event{github.PushEvent},
			GogsEvents:   []gogs.Event{gogs.PushEvent},
			GitLabEvents: []gitlab.Event{gitlab.PushEvents},
			GHCR:         true,
		})
		var got []string
		for _, p := range parsers {
			got = append(got, fmt.Sprintf("%T", p))
		}
		assert.Equal(t, []string{
			"*webhook.gogsParser",
			"*webhook.githubParser",
			"*webhook.gitlabParser",
			"*webhook.GHCRParser",
		}, got)
	})
	t.Run("parser rejects events it was not configured for", func(t *testing.T) {
		parsers := NewParsers(&settings.ArgoCDSettings{}, ParserOptions{
			GitHubEvents: []github.Event{github.PushEvent},
		})
		require.Len(t, parsers, 1)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook", strings.NewReader(`{}`))
		req.Header.Set("X-GitHub-Event", "pull_request")
		require.True(t, parsers[0].CanHandle(req))

		_, err := parsers[0].Parse(req)
		require.ErrorIs(t, err, github.ErrEventNotFound)
	})
	t.Run("parser verifies requests with its own provider's secret", func(t *testing.T) {
		parsers := NewParsers(&settings.ArgoCDSettings{WebhookBitbucketServerSecret: "bb-secret"}, ParserOptions{
			BitbucketServerEvents: []bitbucketserver.Event{bitbucketserver.RepositoryReferenceChangedEvent},
		})
		require.Len(t, parsers, 1)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook", strings.NewReader(`{}`))
		req.Header.Set("X-Event-Key", "repo:refs_changed")
		require.True(t, parsers[0].CanHandle(req))

		// The request is unsigned, so it must be rejected once the Bitbucket Server secret is set.
		_, err := parsers[0].Parse(req)
		require.ErrorIs(t, err, bitbucketserver.ErrMissingHubSignatureHeader)
	})
}

func TestDispatch(t *testing.T) {
	parsers := NewParsers(&settings.ArgoCDSettings{}, ParserOptions{
		GitHubEvents: []github.Event{github.PushEvent},
	})

	t.Run("unclaimed request is not handled", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook", strings.NewReader(`{}`))
		req.Header.Set("X-Gitlab-Event", "Push Hook")

		payload, handled, err := Dispatch(parsers, req)
		require.NoError(t, err)
		assert.False(t, handled)
		assert.Nil(t, payload)
	})

	t.Run("claimed request is handled even when parsing fails", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook", strings.NewReader(`{}`))
		req.Header.Set("X-GitHub-Event", "pull_request")

		_, handled, err := Dispatch(parsers, req)
		assert.True(t, handled)
		require.ErrorIs(t, err, github.ErrEventNotFound)
	})
}
