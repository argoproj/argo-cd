package webhook

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

func TestHandleRequest(t *testing.T) {
	const maxPayloadSizeB = 1024 * 1024

	tests := []struct {
		name         string
		githubSecret string
		method       string
		event        string
		body         string
		queueFull    bool
		wantStatus   int
		wantBody     string
		wantQueued   bool
	}{
		{
			name:       "unknown webhook event is rejected",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   "Unknown webhook event",
		},
		{
			name:       "payload over the size limit is rejected",
			event:      "push",
			body:       strings.Repeat("a", maxPayloadSizeB+1),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Webhook processing failed: payload must be valid JSON under 1 MB",
		},
		{
			name:         "failed registry signature check is unauthorized",
			githubSecret: "secret",
			event:        "package",
			body:         `{}`,
			wantStatus:   http.StatusUnauthorized,
			wantBody:     "Unauthorized",
		},
		{
			name:       "parse error is not echoed to the caller",
			event:      "pull_request",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   "Webhook processing failed",
		},
		{
			name:       "non-POST request is not allowed",
			method:     http.MethodGet,
			event:      "push",
			body:       `{}`,
			wantStatus: http.StatusMethodNotAllowed,
			wantBody:   "Webhook processing failed",
		},
		{
			name:       "skipped event is acknowledged without queueing",
			event:      "package",
			body:       `{"action":"created"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "parsed event is queued",
			event:      "push",
			body:       `{}`,
			wantStatus: http.StatusOK,
			wantQueued: true,
		},
		{
			name:       "payload is discarded when the queue is full",
			event:      "push",
			body:       `{}`,
			queueFull:  true,
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   "Queue is full, discarding webhook payload",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsers := NewParsers(&settings.ArgoCDSettings{WebhookGitHubSecret: tt.githubSecret}, ParserOptions{
				GitHubEvents: []github.Event{github.PushEvent},
				GHCR:         true,
			})
			queue := make(chan any, 1)
			if tt.queueFull {
				queue = make(chan any)
			}
			method := http.MethodPost
			if tt.method != "" {
				method = tt.method
			}
			req := httptest.NewRequestWithContext(t.Context(), method, "/api/webhook", strings.NewReader(tt.body))
			if tt.event != "" {
				req.Header.Set("X-GitHub-Event", tt.event)
			}
			w := httptest.NewRecorder()

			HandleRequest(w, req, parsers, maxPayloadSizeB, queue)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, tt.wantBody, strings.TrimSpace(w.Body.String()))
			if tt.wantQueued {
				assert.Len(t, queue, 1)
			} else {
				assert.Empty(t, queue)
			}
		})
	}
}

func TestStartWorkers(t *testing.T) {
	var wg sync.WaitGroup
	queue := make(chan any, 3)
	var handled []any

	// A single worker, so the payload after the panic is only handled if the
	// worker recovered and kept reading from the queue.
	StartWorkers(&wg, 1, queue, func(payload any) {
		if payload == "panic" {
			panic("boom")
		}
		handled = append(handled, payload)
	}, "test-webhook", "panic in test worker")

	queue <- "first"
	queue <- "panic"
	queue <- "second"
	close(queue)
	wg.Wait()

	assert.Equal(t, []any{"first", "second"}, handled)
}
