package sourcecraft

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	path = "/webhooks"
)

var hook *Webhook

func TestMain(m *testing.M) {
	// setup
	var err error
	hook, err = New(Options.Secret("95bd75d617a34488bf9c334d8a590232"))
	if err != nil {
		log.Fatal(err)
	}
	os.Exit(m.Run())
	// teardown
}

func newServer(handler http.HandlerFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc(path, handler)
	return httptest.NewServer(mux)
}

func TestBadRequests(t *testing.T) {
	t.Parallel()

	assert := require.New(t)
	tests := []struct {
		name    string
		event   Event
		payload io.Reader
		headers http.Header
	}{
		{
			name:    "BadNoEventHeader",
			event:   PushEvent,
			payload: bytes.NewBuffer([]byte("{}")),
			headers: http.Header{},
		},
		{
			name:    "UnsubscribedEvent",
			event:   PullRequestCreateEvent,
			payload: bytes.NewBuffer([]byte("{}")),
			headers: http.Header{
				"X-Src-Event": []string{"noneexistant_event"},
			},
		},
		{
			name:    "BadBody",
			event:   PullRequestPublishEvent,
			payload: bytes.NewBuffer([]byte("")),
			headers: http.Header{
				"X-Src-Event":     []string{"pull_request.publish"},
				"X-Src-Signature": []string{"35063441a894a77bcfcaaedbef3ecaf4dacb504d154e0a960c5d1960d33365f3"},
			},
		},
		{
			name:    "BadSignatureLength",
			event:   PullRequestMergeEvent,
			payload: bytes.NewBuffer([]byte("{}")),
			headers: http.Header{
				"X-Src-Event":     []string{"pull_request.merge"},
				"X-Src-Signature": []string{""},
			},
		},
		{
			name:    "BadSignatureMatch",
			event:   PullRequestNewIterationEvent,
			payload: bytes.NewBuffer([]byte("{}")),
			headers: http.Header{
				"X-Src-Event":     []string{"pull_request.new_iteration"},
				"X-Src-Signature": []string{"111"},
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		client := &http.Client{}
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var parseError error
			server := newServer(func(_ http.ResponseWriter, r *http.Request) {
				_, parseError = hook.Parse(r, tc.event)
			})
			defer server.Close()
			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+path, tc.payload)
			assert.NoError(err)
			req.Header = tc.headers
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			assert.NoError(err)
			assert.Equal(http.StatusOK, resp.StatusCode)
			assert.Error(parseError)
		})
	}
}

func TestPullRequestEventAggregateStruct(t *testing.T) {
	t.Parallel()

	assert := require.New(t)

	tests := []struct {
		name          string
		eventHeader   string
		filename      string
		expectedEvent Event
	}{
		{
			name:          "PullRequestCreateEvent",
			eventHeader:   "pull_request.create",
			filename:      "testdata/pull-request-create.json",
			expectedEvent: PullRequestCreateEvent,
		},
		{
			name:          "PullRequestUpdateEvent",
			eventHeader:   "pull_request.update",
			filename:      "testdata/pull-request-update.json",
			expectedEvent: PullRequestUpdateEvent,
		},
		{
			name:          "PullRequestPublishEvent",
			eventHeader:   "pull_request.publish",
			filename:      "testdata/pull-request-publish.json",
			expectedEvent: PullRequestPublishEvent,
		},
		{
			name:          "PullRequestRefreshEvent",
			eventHeader:   "pull_request.refresh",
			filename:      "testdata/pull-request-refresh.json",
			expectedEvent: PullRequestRefreshEvent,
		},
		{
			name:          "PullRequestNewIterationEvent",
			eventHeader:   "pull_request.new_iteration",
			filename:      "testdata/pull-request-new-iteration.json",
			expectedEvent: PullRequestNewIterationEvent,
		},
		{
			name:          "PullRequestReviewAssignmentEvent",
			eventHeader:   "pull_request.review_assignment",
			filename:      "testdata/pull-request-review-assignment.json",
			expectedEvent: PullRequestReviewAssignmentEvent,
		},
		{
			name:          "PullRequestReviewDecisionEvent",
			eventHeader:   "pull_request.review_decision",
			filename:      "testdata/pull-request-review-decision.json",
			expectedEvent: PullRequestReviewDecisionEvent,
		},
		{
			name:          "PullRequestMergeEvent",
			eventHeader:   "pull_request.merge",
			filename:      "testdata/pull-request-merge.json",
			expectedEvent: PullRequestMergeEvent,
		},
		{
			name:          "PullRequestMergeFailureEvent",
			eventHeader:   "pull_request.merge_failure",
			filename:      "testdata/pull-request-merge-failure.json",
			expectedEvent: PullRequestMergeFailureEvent,
		},
	}

	for _, tt := range tests {
		tc := tt
		client := &http.Client{}
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			payload, err := os.ReadFile(tc.filename)
			assert.NoError(err)

			var parseError error
			var results any
			server := newServer(func(_ http.ResponseWriter, r *http.Request) {
				results, parseError = hook.Parse(r, PullRequestAggregate)
			})
			defer server.Close()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+path, bytes.NewReader(payload))
			assert.NoError(err)
			req.Header.Set("X-Src-Event", tc.eventHeader)

			mac := hmac.New(sha256.New, []byte(hook.secret))
			mac.Write(payload)
			req.Header.Set("X-Src-Signature", hex.EncodeToString(mac.Sum(nil)))
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			assert.NoError(err)
			assert.Equal(http.StatusOK, resp.StatusCode)
			assert.NoError(parseError)

			// All PR events parsed via PullRequestAggregate return PullRequestEventAggregate
			prPayload, ok := results.(PullRequestEventAggregate)
			assert.True(ok, "Expected PullRequestEventAggregate struct, got %T", results)

			// Verify common fields are populated
			assert.NotNil(prPayload.Header, "Header should not be nil")
			assert.NotEmpty(prPayload.Header.Id, "Header ID should not be empty")
			assert.NotEmpty(prPayload.Header.Type, "Header Type should not be empty")
			assert.NotNil(prPayload.Repository, "Repository should not be nil")
			assert.NotEmpty(prPayload.Repository.Id, "Repository ID should not be empty")
			assert.NotEmpty(prPayload.Repository.Slug, "Repository Slug should not be empty")
			assert.NotNil(prPayload.PullRequest, "PullRequest should not be nil")
			assert.NotEmpty(prPayload.PullRequest.Id, "PullRequest ID should not be empty")
			assert.NotEmpty(prPayload.PullRequest.Slug, "PullRequest Slug should not be empty")

			// Verify EventType is set correctly
			assert.Equal(tc.expectedEvent, prPayload.EventType, "EventType should match the incoming event")

			// Verify RawEvent is populated
			assert.NotNil(prPayload.RawEvent, "RawEvent should not be nil")
		})
	}
}

func TestNonPullRequestEventsDoNotReturnPRPayload(t *testing.T) {
	t.Parallel()

	assert := require.New(t)

	tests := []struct {
		name     string
		event    Event
		filename string
	}{
		{
			name:     "PingEvent",
			event:    PingEvent,
			filename: "testdata/ping.json",
		},
		{
			name:     "PushEvent",
			event:    PushEvent,
			filename: "testdata/push-event.json",
		},
	}

	for _, tt := range tests {
		tc := tt
		client := &http.Client{}
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			payload, err := os.ReadFile(tc.filename)
			assert.NoError(err)

			var parseError error
			var results any
			server := newServer(func(_ http.ResponseWriter, r *http.Request) {
				results, parseError = hook.Parse(r, tc.event)
			})
			defer server.Close()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+path, bytes.NewReader(payload))
			assert.NoError(err)
			req.Header.Set("X-Src-Event", string(tc.event))

			mac := hmac.New(sha256.New, []byte(hook.secret))
			mac.Write(payload)
			req.Header.Set("X-Src-Signature", hex.EncodeToString(mac.Sum(nil)))
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			assert.NoError(err)
			assert.Equal(http.StatusOK, resp.StatusCode)
			assert.NoError(parseError)

			// Verify that non-PR events are never returned as PullRequestEventAggregate
			_, isPRPayload := results.(PullRequestEventAggregate)
			assert.False(isPRPayload, "Non-PR event %s should not be returned as PullRequestEventAggregate", tc.event)
		})
	}
}

func TestWebhooks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		event    Event
		typ      any
		filename string
		headers  http.Header
	}{
		{
			name:     "PingEvent",
			event:    PingEvent,
			typ:      PingEventPayload{},
			filename: "testdata/ping.json",
			headers: http.Header{
				"X-Src-Event": []string{"webhook.ping"},
			},
		},
		{
			name:     "PushEvent",
			event:    PushEvent,
			typ:      PushEventPayload{},
			filename: "testdata/push-event.json",
			headers: http.Header{
				"X-Src-Event": []string{"repository.push"},
			},
		},
		{
			name:     "PullRequestCreateEvent",
			event:    PullRequestCreateEvent,
			typ:      PullRequestCreateEventPayload{},
			filename: "testdata/pull-request-create.json",
			headers: http.Header{
				"X-Src-Event": []string{"pull_request.create"},
			},
		},
		{
			name:     "PullRequestUpdateEvent",
			event:    PullRequestUpdateEvent,
			typ:      PullRequestUpdateEventPayload{},
			filename: "testdata/pull-request-update.json",
			headers: http.Header{
				"X-Src-Event": []string{"pull_request.update"},
			},
		},
		{
			name:     "PullRequestPublishEvent",
			event:    PullRequestPublishEvent,
			typ:      PullRequestPublishEventPayload{},
			filename: "testdata/pull-request-publish.json",
			headers: http.Header{
				"X-Src-Event": []string{"pull_request.publish"},
			},
		},
		{
			name:     "PullRequestRefreshEvent",
			event:    PullRequestRefreshEvent,
			typ:      PullRequestRefreshEventPayload{},
			filename: "testdata/pull-request-refresh.json",
			headers: http.Header{
				"X-Src-Event": []string{"pull_request.refresh"},
			},
		},
		{
			name:     "PullRequestNewIterationEvent",
			event:    PullRequestNewIterationEvent,
			typ:      PullRequestNewIterationEventPayload{},
			filename: "testdata/pull-request-new-iteration.json",
			headers: http.Header{
				"X-Src-Event": []string{"pull_request.new_iteration"},
			},
		},
		{
			name:     "PullRequestReviewAssignmentEvent",
			event:    PullRequestReviewAssignmentEvent,
			typ:      PullRequestReviewAssignmentEventPayload{},
			filename: "testdata/pull-request-review-assignment.json",
			headers: http.Header{
				"X-Src-Event": []string{"pull_request.review_assignment"},
			},
		},
		{
			name:     "PullRequestReviewDecisionEvent",
			event:    PullRequestReviewDecisionEvent,
			typ:      PullRequestReviewDecisionEventPaylaod{},
			filename: "testdata/pull-request-review-decision.json",
			headers: http.Header{
				"X-Src-Event": []string{"pull_request.review_decision"},
			},
		},
		{
			name:     "PullRequestMergeEvent",
			event:    PullRequestMergeEvent,
			typ:      PullRequestMergeEventPayload{},
			filename: "testdata/pull-request-merge.json",
			headers: http.Header{
				"X-Src-Event": []string{"pull_request.merge"},
			},
		},
		{
			name:     "PullRequestMergeFailureEvent",
			event:    PullRequestMergeFailureEvent,
			typ:      PullRequestMergeFailureEventPayload{},
			filename: "testdata/pull-request-merge-failure.json",
			headers: http.Header{
				"X-Src-Event": []string{"pull_request.merge_failure"},
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		client := &http.Client{}
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert := require.New(t)
			payload, err := os.ReadFile(tc.filename)
			assert.NoError(err)

			var parseError error
			var results any
			server := newServer(func(_ http.ResponseWriter, r *http.Request) {
				results, parseError = hook.Parse(r, tc.event)
			})
			defer server.Close()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+path, bytes.NewReader(payload))
			assert.NoError(err)
			req.Header = tc.headers
			mac := hmac.New(sha256.New, []byte(hook.secret))
			mac.Write(payload)

			req.Header.Set("X-Src-Signature", hex.EncodeToString(mac.Sum(nil)))

			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			assert.NoError(err)
			assert.Equal(http.StatusOK, resp.StatusCode)
			assert.NoError(parseError)
			assert.Equal(reflect.TypeOf(tc.typ), reflect.TypeOf(results))
		})
	}
}

func TestWebhook_PullRequestAggregate(t *testing.T) {
	t.Parallel()
	assert := require.New(t)

	tests := []struct {
		name            string
		filename        string
		eventHeader     string
		expectedEvent   Event
		expectedRawType any
	}{
		{
			name:            "CreateEventMatchesPullRequestAggregate",
			filename:        "testdata/pull-request-create.json",
			eventHeader:     "pull_request.create",
			expectedEvent:   PullRequestCreateEvent,
			expectedRawType: PullRequestCreateEventPayload{},
		},
		{
			name:            "UpdateEventMatchesPullRequestAggregate",
			filename:        "testdata/pull-request-update.json",
			eventHeader:     "pull_request.update",
			expectedEvent:   PullRequestUpdateEvent,
			expectedRawType: PullRequestUpdateEventPayload{},
		},
		{
			name:            "PublishEventMatchesPullRequestAggregate",
			filename:        "testdata/pull-request-publish.json",
			eventHeader:     "pull_request.publish",
			expectedEvent:   PullRequestPublishEvent,
			expectedRawType: PullRequestPublishEventPayload{},
		},
		{
			name:            "RefreshEventMatchesPullRequestAggregate",
			filename:        "testdata/pull-request-refresh.json",
			eventHeader:     "pull_request.refresh",
			expectedEvent:   PullRequestRefreshEvent,
			expectedRawType: PullRequestRefreshEventPayload{},
		},
		{
			name:            "MergeEventMatchesPullRequestAggregate",
			filename:        "testdata/pull-request-merge.json",
			eventHeader:     "pull_request.merge",
			expectedEvent:   PullRequestMergeEvent,
			expectedRawType: PullRequestMergeEventPayload{},
		},
		{
			name:            "MergeFailureEventMatchesPullRequestAggregate",
			filename:        "testdata/pull-request-merge-failure.json",
			eventHeader:     "pull_request.merge_failure",
			expectedEvent:   PullRequestMergeFailureEvent,
			expectedRawType: PullRequestMergeFailureEventPayload{},
		},
		{
			name:            "NewIterationEventMatchesPullRequestAggregate",
			filename:        "testdata/pull-request-new-iteration.json",
			eventHeader:     "pull_request.new_iteration",
			expectedEvent:   PullRequestNewIterationEvent,
			expectedRawType: PullRequestNewIterationEventPayload{},
		},
		{
			name:            "ReviewAssignmentEventMatchesPullRequestAggregate",
			filename:        "testdata/pull-request-review-assignment.json",
			eventHeader:     "pull_request.review_assignment",
			expectedEvent:   PullRequestReviewAssignmentEvent,
			expectedRawType: PullRequestReviewAssignmentEventPayload{},
		},
		{
			name:            "ReviewDecisionEventMatchesPullRequestAggregate",
			filename:        "testdata/pull-request-review-decision.json",
			eventHeader:     "pull_request.review_decision",
			expectedEvent:   PullRequestReviewDecisionEvent,
			expectedRawType: PullRequestReviewDecisionEventPaylaod{},
		},
	}

	for _, tt := range tests {
		tc := tt
		client := &http.Client{}
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			payload, err := os.ReadFile(tc.filename)
			assert.NoError(err)

			var parseError error
			var results any
			server := newServer(func(_ http.ResponseWriter, r *http.Request) {
				results, parseError = hook.Parse(r, PullRequestAggregate)
			})
			defer server.Close()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+path, bytes.NewReader(payload))
			assert.NoError(err)
			req.Header.Set("X-Src-Event", tc.eventHeader)
			req.Header.Set("Content-Type", "application/json")

			mac := hmac.New(sha256.New, []byte(hook.secret))
			mac.Write(payload)
			req.Header.Set("X-Src-Signature", hex.EncodeToString(mac.Sum(nil)))

			resp, err := client.Do(req)
			assert.NoError(err)
			assert.Equal(http.StatusOK, resp.StatusCode)
			assert.NoError(parseError, "Should successfully parse with PullRequestAggregate")

			// All PR events parsed via PullRequestAggregate return PullRequestEventAggregate
			prPayload, ok := results.(PullRequestEventAggregate)
			assert.True(ok, "Result should be PullRequestEventAggregate struct, got %T", results)

			// Verify common fields
			assert.NotNil(prPayload.Header)
			assert.NotNil(prPayload.Repository)
			assert.NotNil(prPayload.PullRequest)

			// Verify EventType is set correctly
			assert.Equal(tc.expectedEvent, prPayload.EventType, "EventType should match the incoming event")

			// Verify RawEvent holds the correct specific type
			assert.Equal(reflect.TypeOf(tc.expectedRawType), reflect.TypeOf(prPayload.RawEvent),
				"RawEvent should hold the specific payload type")

			t.Logf("✓ PullRequestAggregate matched: %s", tc.eventHeader)
		})
	}
}

func TestWebhook_PullRequestAggregateDoesNotMatchNonPREvents(t *testing.T) {
	t.Parallel()
	assert := require.New(t)

	tests := []struct {
		name        string
		filename    string
		eventHeader string
	}{
		{
			name:        "PingEventDoesNotMatch",
			filename:    "testdata/ping.json",
			eventHeader: "webhook.ping",
		},
		{
			name:        "PushEventDoesNotMatch",
			filename:    "testdata/push-event.json",
			eventHeader: "repository.push",
		},
	}

	for _, tt := range tests {
		tc := tt
		client := &http.Client{}
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			payload, err := os.ReadFile(tc.filename)
			assert.NoError(err)

			var parseError error
			server := newServer(func(_ http.ResponseWriter, r *http.Request) {
				// Try to parse with only the PullRequestAggregate type
				_, parseError = hook.Parse(r, PullRequestAggregate)
			})
			defer server.Close()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+path, bytes.NewReader(payload))
			assert.NoError(err)
			req.Header.Set("X-Src-Event", tc.eventHeader)
			req.Header.Set("Content-Type", "application/json")

			mac := hmac.New(sha256.New, []byte(hook.secret))
			mac.Write(payload)
			req.Header.Set("X-Src-Signature", hex.EncodeToString(mac.Sum(nil)))

			resp, err := client.Do(req)
			assert.NoError(err)
			assert.Equal(http.StatusOK, resp.StatusCode)

			// Non-PR events should not match the PullRequestAggregate
			assert.Error(parseError, "Non-PR event should not match PullRequestAggregate")
			assert.Equal(ErrEventNotFound, parseError, "Should return ErrEventNotFound for non-PR events")

			t.Logf("✓ Correctly rejected non-PR event: %s", tc.eventHeader)
		})
	}
}

func TestWebhook_MultipleSecretsSupport(t *testing.T) {
	t.Parallel()
	assert := require.New(t)

	// Define multiple secrets for testing
	secret1 := "95bd75d617a34488bf9c334d8a590232"
	secret2 := "aaaabbbbccccddddeeeeffffgggghhhhiiiijjjj"
	secret3 := "1234567890abcdef1234567890abcdef12345678"

	tests := []struct {
		name          string
		secrets       string
		signSecret    string
		filename      string
		event         Event
		eventHeader   string
		expectSuccess bool
	}{
		{
			name:          "FirstSecretMatches",
			secrets:       secret1 + "," + secret2 + "," + secret3,
			signSecret:    secret1,
			filename:      "testdata/ping.json",
			event:         PingEvent,
			eventHeader:   "webhook.ping",
			expectSuccess: true,
		},
		{
			name:          "MiddleSecretMatches",
			secrets:       secret1 + "," + secret2 + "," + secret3,
			signSecret:    secret2,
			filename:      "testdata/ping.json",
			event:         PingEvent,
			eventHeader:   "webhook.ping",
			expectSuccess: true,
		},
		{
			name:          "LastSecretMatches",
			secrets:       secret1 + "," + secret2 + "," + secret3,
			signSecret:    secret3,
			filename:      "testdata/ping.json",
			event:         PingEvent,
			eventHeader:   "webhook.ping",
			expectSuccess: true,
		},
		{
			name:          "SecretsWithSpaces",
			secrets:       secret1 + " , " + secret2 + " , " + secret3,
			signSecret:    secret2,
			filename:      "testdata/ping.json",
			event:         PingEvent,
			eventHeader:   "webhook.ping",
			expectSuccess: true,
		},
		{
			name:          "NoSecretMatches",
			secrets:       secret1 + "," + secret2,
			signSecret:    secret3,
			filename:      "testdata/ping.json",
			event:         PingEvent,
			eventHeader:   "webhook.ping",
			expectSuccess: false,
		},
		{
			name:          "SingleSecretMatches",
			secrets:       secret1,
			signSecret:    secret1,
			filename:      "testdata/push-event.json",
			event:         PushEvent,
			eventHeader:   "repository.push",
			expectSuccess: true,
		},
		{
			name:          "SecretRotationScenario",
			secrets:       secret2 + "," + secret1, // New secret first, old secret second
			signSecret:    secret1,                 // Webhook still signed with old secret
			filename:      "testdata/pull-request-create.json",
			event:         PullRequestCreateEvent,
			eventHeader:   "pull_request.create",
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		tc := tt
		client := &http.Client{}
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a webhook instance with the specified secrets
			testHook, err := New(Options.Secret(tc.secrets))
			assert.NoError(err)

			payload, err := os.ReadFile(tc.filename)
			assert.NoError(err)

			var parseError error
			var results any
			server := newServer(func(_ http.ResponseWriter, r *http.Request) {
				results, parseError = testHook.Parse(r, tc.event)
			})
			defer server.Close()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+path, bytes.NewReader(payload))
			assert.NoError(err)
			req.Header.Set("X-Src-Event", tc.eventHeader)
			req.Header.Set("Content-Type", "application/json")

			// Sign with the specified secret
			mac := hmac.New(sha256.New, []byte(tc.signSecret))
			mac.Write(payload)
			req.Header.Set("X-Src-Signature", hex.EncodeToString(mac.Sum(nil)))

			resp, err := client.Do(req)
			assert.NoError(err)
			assert.Equal(http.StatusOK, resp.StatusCode)

			if tc.expectSuccess {
				assert.NoError(parseError, "Should successfully verify signature with one of the secrets")
				assert.NotNil(results, "Should return parsed payload")
				t.Logf("✓ Signature verified with secret (secrets list: %d)", len(tc.secrets))
			} else {
				assert.Error(parseError, "Should fail to verify signature when no secret matches")
				assert.Equal(ErrHMACVerificationFailed, parseError, "Should return ErrHMACVerificationFailed")
				t.Logf("✓ Correctly rejected signature with non-matching secret")
			}
		})
	}
}
