package sourcecraft_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/sourcecraft"
)

// TestPullRequestEventAggregate_TypeSwitch demonstrates how to use PullRequestEventAggregate
// with a type switch when using PullRequestAggregate to handle all PR events in one case.
func TestPullRequestEventAggregate_TypeSwitch(t *testing.T) {
	assert := require.New(t)

	hook, err := sourcecraft.New(sourcecraft.Options.Secret("your-webhook-secret"))
	assert.NoError(err)

	tests := []struct {
		name              string
		eventHeader       string
		filename          string
		expectPRPayload   bool
		validatePRPayload func(*testing.T, sourcecraft.PullRequestEventAggregate)
		validateOther     func(*testing.T, any)
	}{
		{
			name:            "PullRequestCreateEvent",
			eventHeader:     "pull_request.create",
			filename:        "testdata/pull-request-create.json",
			expectPRPayload: true,
			validatePRPayload: func(_ *testing.T, p sourcecraft.PullRequestEventAggregate) {
				assert.Equal("pull_request.create", p.Header.Type)
				assert.NotEmpty(p.Repository.Slug)
				assert.NotEmpty(p.PullRequest.Title)
				assert.Equal(sourcecraft.PullRequestCreateEvent, p.EventType)
			},
		},
		{
			name:            "PullRequestMergeEvent",
			eventHeader:     "pull_request.merge",
			filename:        "testdata/pull-request-merge.json",
			expectPRPayload: true,
			validatePRPayload: func(_ *testing.T, p sourcecraft.PullRequestEventAggregate) {
				assert.Equal("pull_request.merge", p.Header.Type)
				assert.Equal(sourcecraft.PullRequestMergeEvent, p.EventType)
				// Access event-specific fields via RawEvent
				if mergeEvent, ok := p.RawEvent.(sourcecraft.PullRequestMergeEventPayload); ok {
					assert.NotEmpty(mergeEvent.MergeHash, "MergeHash should be present")
				}
			},
		},
		{
			name:            "PullRequestMergeFailureEvent",
			eventHeader:     "pull_request.merge_failure",
			filename:        "testdata/pull-request-merge-failure.json",
			expectPRPayload: true,
			validatePRPayload: func(_ *testing.T, p sourcecraft.PullRequestEventAggregate) {
				assert.Equal("pull_request.merge_failure", p.Header.Type)
				assert.Equal(sourcecraft.PullRequestMergeFailureEvent, p.EventType)
				// Access event-specific fields via RawEvent
				if failureEvent, ok := p.RawEvent.(sourcecraft.PullRequestMergeFailureEventPayload); ok {
					assert.NotEmpty(failureEvent.ErrorMessage, "ErrorMessage should be present")
				}
			},
		},
		{
			name:            "PullRequestUpdateEvent",
			eventHeader:     "pull_request.update",
			filename:        "testdata/pull-request-update.json",
			expectPRPayload: true,
			validatePRPayload: func(_ *testing.T, p sourcecraft.PullRequestEventAggregate) {
				assert.Equal("pull_request.update", p.Header.Type)
				assert.Equal(sourcecraft.PullRequestUpdateEvent, p.EventType)
			},
		},
		{
			name:            "PushEvent",
			eventHeader:     "repository.push",
			filename:        "testdata/push-event.json",
			expectPRPayload: false,
			validateOther: func(_ *testing.T, payload any) {
				pushEvent, ok := payload.(sourcecraft.PushEventPayload)
				assert.True(ok, "Should be PushEventPayload")
				assert.NotNil(pushEvent.Repository)
				assert.NotEmpty(pushEvent.Repository.Slug)
			},
		},
		{
			name:            "PingEvent",
			eventHeader:     "webhook.ping",
			filename:        "testdata/ping.json",
			expectPRPayload: false,
			validateOther: func(_ *testing.T, payload any) {
				pingEvent, ok := payload.(sourcecraft.PingEventPayload)
				assert.True(ok, "Should be PingEventPayload")
				assert.NotEmpty(pingEvent.WebhookSlug)
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tt.name, func(t *testing.T) {
			payloadBytes, err := os.ReadFile(tc.filename)
			assert.NoError(err)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", bytes.NewReader(payloadBytes))
			req.Header.Set("X-Src-Event", tc.eventHeader)
			req.Header.Set("Content-Type", "application/json")

			mac := hmac.New(sha256.New, []byte("your-webhook-secret"))
			mac.Write(payloadBytes)
			req.Header.Set("X-Src-Signature", hex.EncodeToString(mac.Sum(nil)))

			// Use PullRequestAggregate to receive PullRequestEventAggregate for all PR events,
			// combined with specific non-PR events.
			payload, err := hook.Parse(req, sourcecraft.PullRequestAggregate, sourcecraft.PushEvent, sourcecraft.PingEvent)
			assert.NoError(err)
			assert.NotNil(payload)

			switch event := payload.(type) {
			case sourcecraft.PullRequestEventAggregate:
				assert.True(tc.expectPRPayload, "Expected PullRequestEventAggregate for event %s", tc.eventHeader)

				assert.NotNil(event.Header, "Header should not be nil")
				assert.NotEmpty(event.Header.Type, "Header Type should not be empty")
				assert.NotNil(event.Repository, "Repository should not be nil")
				assert.NotEmpty(event.Repository.Slug, "Repository Slug should not be empty")
				assert.NotNil(event.PullRequest, "PullRequest should not be nil")
				assert.NotEmpty(event.PullRequest.Slug, "PullRequest Slug should not be empty")
				assert.NotNil(event.RawEvent, "RawEvent should not be nil")

				if tc.validatePRPayload != nil {
					tc.validatePRPayload(t, event)
				}

				t.Logf("✓ Handled PR event: %s (EventType: %s)", event.Header.Type, event.EventType)

			case sourcecraft.PushEventPayload:
				assert.False(tc.expectPRPayload, "Did not expect PullRequestEventAggregate for PushEvent")
				if tc.validateOther != nil {
					tc.validateOther(t, event)
				}
				t.Logf("✓ Push Event to %s", event.Repository.Slug)

			case sourcecraft.PingEventPayload:
				assert.False(tc.expectPRPayload, "Did not expect PullRequestEventAggregate for PingEvent")
				if tc.validateOther != nil {
					tc.validateOther(t, event)
				}
				t.Logf("✓ Webhook ping from %s", event.WebhookSlug)

			default:
				t.Fatalf("Unexpected payload type: %T", payload)
			}
		})
	}
}

// TestPullRequestEventAggregate_SingleCaseHandling verifies that all PR events parsed
// via PullRequestAggregate are returned as PullRequestEventAggregate and handled in one case.
func TestPullRequestEventAggregate_SingleCaseHandling(t *testing.T) {
	assert := require.New(t)

	hook, err := sourcecraft.New(sourcecraft.Options.Secret("test-secret"))
	assert.NoError(err)

	allPREvents := []struct {
		eventHeader string
		filename    string
		eventType   sourcecraft.Event
	}{
		{"pull_request.create", "testdata/pull-request-create.json", sourcecraft.PullRequestCreateEvent},
		{"pull_request.update", "testdata/pull-request-update.json", sourcecraft.PullRequestUpdateEvent},
		{"pull_request.publish", "testdata/pull-request-publish.json", sourcecraft.PullRequestPublishEvent},
		{"pull_request.refresh", "testdata/pull-request-refresh.json", sourcecraft.PullRequestRefreshEvent},
		{"pull_request.merge", "testdata/pull-request-merge.json", sourcecraft.PullRequestMergeEvent},
		{"pull_request.merge_failure", "testdata/pull-request-merge-failure.json", sourcecraft.PullRequestMergeFailureEvent},
		{"pull_request.new_iteration", "testdata/pull-request-new-iteration.json", sourcecraft.PullRequestNewIterationEvent},
		{"pull_request.review_assignment", "testdata/pull-request-review-assignment.json", sourcecraft.PullRequestReviewAssignmentEvent},
		{"pull_request.review_decision", "testdata/pull-request-review-decision.json", sourcecraft.PullRequestReviewDecisionEvent},
	}

	handledCount := 0

	for _, tc := range allPREvents {
		payloadBytes, err := os.ReadFile(tc.filename)
		assert.NoError(err)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", bytes.NewReader(payloadBytes))
		req.Header.Set("X-Src-Event", tc.eventHeader)
		req.Header.Set("Content-Type", "application/json")

		mac := hmac.New(sha256.New, []byte("test-secret"))
		mac.Write(payloadBytes)
		req.Header.Set("X-Src-Signature", hex.EncodeToString(mac.Sum(nil)))

		payload, err := hook.Parse(req, sourcecraft.PullRequestAggregate)
		assert.NoError(err)

		// Single case handles ALL PR events - this is the key benefit!
		switch prEvent := payload.(type) {
		case sourcecraft.PullRequestEventAggregate:
			handledCount++
			assert.NotNil(prEvent.Header)
			assert.NotNil(prEvent.Repository)
			assert.NotNil(prEvent.PullRequest)
			assert.Equal(tc.eventType, prEvent.EventType)
			assert.NotNil(prEvent.RawEvent)
		default:
			t.Fatalf("Expected PullRequestEventAggregate, got %T for event %s", payload, tc.eventHeader)
		}
	}

	// Verify all 9 PR events were handled in the single case branch
	assert.Equal(9, handledCount, "All 9 PR event types should be handled in single case")
	t.Logf("✓ Successfully handled all %d PR event types in a single switch case", handledCount)
}

// TestPullRequestEventAggregate_EventType verifies that EventType is set correctly
// for each PR event when parsed via PullRequestAggregate.
func TestPullRequestEventAggregate_EventType(t *testing.T) {
	assert := require.New(t)

	hook, err := sourcecraft.New(sourcecraft.Options.Secret("test-secret"))
	assert.NoError(err)

	tests := []struct {
		name          string
		eventHeader   string
		filename      string
		expectedEvent sourcecraft.Event
	}{
		{
			name:          "PullRequestCreateEvent",
			eventHeader:   "pull_request.create",
			filename:      "testdata/pull-request-create.json",
			expectedEvent: sourcecraft.PullRequestCreateEvent,
		},
		{
			name:          "PullRequestUpdateEvent",
			eventHeader:   "pull_request.update",
			filename:      "testdata/pull-request-update.json",
			expectedEvent: sourcecraft.PullRequestUpdateEvent,
		},
		{
			name:          "PullRequestPublishEvent",
			eventHeader:   "pull_request.publish",
			filename:      "testdata/pull-request-publish.json",
			expectedEvent: sourcecraft.PullRequestPublishEvent,
		},
		{
			name:          "PullRequestRefreshEvent",
			eventHeader:   "pull_request.refresh",
			filename:      "testdata/pull-request-refresh.json",
			expectedEvent: sourcecraft.PullRequestRefreshEvent,
		},
		{
			name:          "PullRequestMergeEvent",
			eventHeader:   "pull_request.merge",
			filename:      "testdata/pull-request-merge.json",
			expectedEvent: sourcecraft.PullRequestMergeEvent,
		},
		{
			name:          "PullRequestMergeFailureEvent",
			eventHeader:   "pull_request.merge_failure",
			filename:      "testdata/pull-request-merge-failure.json",
			expectedEvent: sourcecraft.PullRequestMergeFailureEvent,
		},
		{
			name:          "PullRequestNewIterationEvent",
			eventHeader:   "pull_request.new_iteration",
			filename:      "testdata/pull-request-new-iteration.json",
			expectedEvent: sourcecraft.PullRequestNewIterationEvent,
		},
		{
			name:          "PullRequestReviewAssignmentEvent",
			eventHeader:   "pull_request.review_assignment",
			filename:      "testdata/pull-request-review-assignment.json",
			expectedEvent: sourcecraft.PullRequestReviewAssignmentEvent,
		},
		{
			name:          "PullRequestReviewDecisionEvent",
			eventHeader:   "pull_request.review_decision",
			filename:      "testdata/pull-request-review-decision.json",
			expectedEvent: sourcecraft.PullRequestReviewDecisionEvent,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tt.name, func(t *testing.T) {
			payloadBytes, err := os.ReadFile(tc.filename)
			assert.NoError(err)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", bytes.NewReader(payloadBytes))
			req.Header.Set("X-Src-Event", tc.eventHeader)
			req.Header.Set("Content-Type", "application/json")

			mac := hmac.New(sha256.New, []byte("test-secret"))
			mac.Write(payloadBytes)
			req.Header.Set("X-Src-Signature", hex.EncodeToString(mac.Sum(nil)))

			payload, err := hook.Parse(req, sourcecraft.PullRequestAggregate)
			assert.NoError(err)

			prEvent, ok := payload.(sourcecraft.PullRequestEventAggregate)
			assert.True(ok, "Payload should be PullRequestEventAggregate struct")

			assert.Equal(tc.expectedEvent, prEvent.EventType, "EventType should be %s", tc.expectedEvent)
			t.Logf("✓ EventType correctly set to: %s", prEvent.EventType)
		})
	}
}
