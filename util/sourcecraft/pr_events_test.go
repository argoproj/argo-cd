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

// TestPullRequestEventPayload_TypeSwitch demonstrates how to use the
// PullRequestEventPayload interface with a type switch to handle all
// pull request events in a single case branch.
func TestPullRequestEventPayload_TypeSwitch(t *testing.T) {
	assert := require.New(t)

	// Create a webhook handler with a secret
	hook, err := sourcecraft.New(sourcecraft.Options.Secret("your-webhook-secret"))
	assert.NoError(err)

	tests := []struct {
		name                 string
		event                sourcecraft.Event
		filename             string
		expectPRInterface    bool
		expectSpecificType   string
		validatePRPayload    func(*testing.T, sourcecraft.PullRequestEventPayload)
		validateOtherPayload func(*testing.T, any)
	}{
		{
			name:               "PullRequestCreateEvent",
			event:              sourcecraft.PullRequestCreateEvent,
			filename:           "testdata/pull-request-create.json",
			expectPRInterface:  true,
			expectSpecificType: "PullRequestCreateEventPayload",
			validatePRPayload: func(_ *testing.T, payload sourcecraft.PullRequestEventPayload) {
				assert.Equal("pull_request.create", payload.GetHeader().Type)
				assert.NotEmpty(payload.GetRepository().Slug)
				assert.NotEmpty(payload.GetPullRequest().Title)
			},
		},
		{
			name:               "PullRequestMergeEvent",
			event:              sourcecraft.PullRequestMergeEvent,
			filename:           "testdata/pull-request-merge.json",
			expectPRInterface:  true,
			expectSpecificType: "PullRequestMergeEventPayload",
			validatePRPayload: func(_ *testing.T, payload sourcecraft.PullRequestEventPayload) {
				assert.Equal("pull_request.merge", payload.GetHeader().Type)
				// Test specific type assertion
				if mergeEvent, ok := payload.(sourcecraft.PullRequestMergeEventPayload); ok {
					assert.NotEmpty(mergeEvent.MergeHash, "MergeHash should be present")
				}
			},
		},
		{
			name:               "PullRequestMergeFailureEvent",
			event:              sourcecraft.PullRequestMergeFailureEvent,
			filename:           "testdata/pull-request-merge-failure.json",
			expectPRInterface:  true,
			expectSpecificType: "PullRequestMergeFailureEventPayload",
			validatePRPayload: func(_ *testing.T, payload sourcecraft.PullRequestEventPayload) {
				assert.Equal("pull_request.merge_failure", payload.GetHeader().Type)
				// Test specific type assertion
				if failureEvent, ok := payload.(sourcecraft.PullRequestMergeFailureEventPayload); ok {
					assert.NotEmpty(failureEvent.ErrorMessage, "ErrorMessage should be present")
				}
			},
		},
		{
			name:               "PullRequestUpdateEvent",
			event:              sourcecraft.PullRequestUpdateEvent,
			filename:           "testdata/pull-request-update.json",
			expectPRInterface:  true,
			expectSpecificType: "PullRequestUpdateEventPayload",
			validatePRPayload: func(_ *testing.T, payload sourcecraft.PullRequestEventPayload) {
				assert.Equal("pull_request.update", payload.GetHeader().Type)
			},
		},
		{
			name:               "PushEvent",
			event:              sourcecraft.PushEvent,
			filename:           "testdata/push-event.json",
			expectPRInterface:  false,
			expectSpecificType: "PushEventPayload",
			validateOtherPayload: func(_ *testing.T, payload any) {
				pushEvent, ok := payload.(sourcecraft.PushEventPayload)
				assert.True(ok, "Should be PushEventPayload")
				assert.NotNil(pushEvent.Repository)
				assert.NotEmpty(pushEvent.Repository.Slug)
			},
		},
		{
			name:               "PingEvent",
			event:              sourcecraft.PingEvent,
			filename:           "testdata/ping.json",
			expectPRInterface:  false,
			expectSpecificType: "PingEventPayload",
			validateOtherPayload: func(_ *testing.T, payload any) {
				pingEvent, ok := payload.(sourcecraft.PingEventPayload)
				assert.True(ok, "Should be PingEventPayload")
				assert.NotEmpty(pingEvent.WebhookSlug)
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tt.name, func(t *testing.T) {
			// Read test payload
			payloadBytes, err := os.ReadFile(tc.filename)
			assert.NoError(err)

			// Create test HTTP request
			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payloadBytes))
			req.Header.Set("X-Src-Event", string(tc.event))
			req.Header.Set("Content-Type", "application/json")

			// Sign the payload
			mac := hmac.New(sha256.New, []byte("your-webhook-secret"))
			mac.Write(payloadBytes)
			req.Header.Set("X-Src-Signature", hex.EncodeToString(mac.Sum(nil)))

			// Parse the webhook payload
			payload, err := hook.Parse(req, tc.event)
			assert.NoError(err)
			assert.NotNil(payload)

			// Use type switch to handle different event types
			// This demonstrates the single case branch for all PR events
			switch event := payload.(type) {
			case sourcecraft.PullRequestEventPayload:
				assert.True(tc.expectPRInterface, "Expected PR interface for event %s", tc.event)

				// All PR events can be handled here with access to common fields
				pr := event.GetPullRequest()
				repo := event.GetRepository()
				header := event.GetHeader()

				// Validate common fields
				assert.NotNil(header, "Header should not be nil")
				assert.NotEmpty(header.Type, "Header Type should not be empty")
				assert.NotNil(repo, "Repository should not be nil")
				assert.NotEmpty(repo.Slug, "Repository Slug should not be empty")
				assert.NotNil(pr, "PullRequest should not be nil")
				assert.NotEmpty(pr.Slug, "PullRequest Slug should not be empty")

				// Run custom validation if provided
				if tc.validatePRPayload != nil {
					tc.validatePRPayload(t, event)
				}

				t.Logf("✓ Successfully handled PR event through interface: %s", header.Type)
				t.Logf("  Repository: %s", repo.Slug)
				t.Logf("  Pull Request: %s - %s", pr.Slug, pr.Title)
				t.Logf("  Status: %s", pr.Status)

			case sourcecraft.PushEventPayload:
				assert.False(tc.expectPRInterface, "Did not expect PR interface for PushEvent")
				assert.Equal("PushEventPayload", tc.expectSpecificType)
				if tc.validateOtherPayload != nil {
					tc.validateOtherPayload(t, event)
				}
				t.Logf("✓ Push Event to %s", event.Repository.Slug)

			case sourcecraft.PingEventPayload:
				assert.False(tc.expectPRInterface, "Did not expect PR interface for PingEvent")
				assert.Equal("PingEventPayload", tc.expectSpecificType)
				if tc.validateOtherPayload != nil {
					tc.validateOtherPayload(t, event)
				}
				t.Logf("✓ Webhook ping from %s", event.WebhookSlug)

			default:
				t.Fatalf("Unexpected payload type: %T", payload)
			}
		})
	}
}

// TestPullRequestEventPayload_SingleCaseHandling verifies that all PR events
// can be handled in a single switch case, which is the main benefit of the interface.
func TestPullRequestEventPayload_SingleCaseHandling(t *testing.T) {
	assert := require.New(t)

	hook, err := sourcecraft.New(sourcecraft.Options.Secret("test-secret"))
	assert.NoError(err)

	allPREvents := []struct {
		event    sourcecraft.Event
		filename string
	}{
		{sourcecraft.PullRequestCreateEvent, "testdata/pull-request-create.json"},
		{sourcecraft.PullRequestUpdateEvent, "testdata/pull-request-update.json"},
		{sourcecraft.PullRequestPublishEvent, "testdata/pull-request-publish.json"},
		{sourcecraft.PullRequestRefreshEvent, "testdata/pull-request-refresh.json"},
		{sourcecraft.PullRequestMergeEvent, "testdata/pull-request-merge.json"},
		{sourcecraft.PullRequestMergeFailureEvent, "testdata/pull-request-merge-failure.json"},
		{sourcecraft.PullRequestNewIterationEvent, "testdata/pull-request-new-iteration.json"},
		{sourcecraft.PullRequestReviewAssignmentEvent, "testdata/pull-request-review-assignment.json"},
		{sourcecraft.PullRequestReviewDecisionEvent, "testdata/pull-request-review-decision.json"},
	}

	handledCount := 0

	for _, tc := range allPREvents {
		payloadBytes, err := os.ReadFile(tc.filename)
		assert.NoError(err)

		req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payloadBytes))
		req.Header.Set("X-Src-Event", string(tc.event))
		req.Header.Set("Content-Type", "application/json")

		mac := hmac.New(sha256.New, []byte("test-secret"))
		mac.Write(payloadBytes)
		req.Header.Set("X-Src-Signature", hex.EncodeToString(mac.Sum(nil)))

		payload, err := hook.Parse(req, tc.event)
		assert.NoError(err)

		// Single case handles ALL PR events - this is the key benefit!
		switch prEvent := payload.(type) {
		case sourcecraft.PullRequestEventPayload:
			handledCount++
			assert.NotNil(prEvent.GetHeader())
			assert.NotNil(prEvent.GetRepository())
			assert.NotNil(prEvent.GetPullRequest())
		default:
			t.Fatalf("Expected PullRequestEventPayload, got %T for event %s", payload, tc.event)
		}
	}

	// Verify all 9 PR events were handled in the single case branch
	assert.Equal(9, handledCount, "All 9 PR event types should be handled in single case")
	t.Logf("✓ Successfully handled all %d PR event types in a single switch case", handledCount)
}

// TestPullRequestEventPayload_GetEventType verifies that the GetEventType method
// returns the correct event type for each payload.
func TestPullRequestEventPayload_GetEventType(t *testing.T) {
	assert := require.New(t)

	hook, err := sourcecraft.New(sourcecraft.Options.Secret("test-secret"))
	assert.NoError(err)

	tests := []struct {
		name         string
		event        sourcecraft.Event
		filename     string
		expectedType sourcecraft.Event
	}{
		{
			name:         "PullRequestCreateEvent",
			event:        sourcecraft.PullRequestCreateEvent,
			filename:     "testdata/pull-request-create.json",
			expectedType: sourcecraft.PullRequestCreateEvent,
		},
		{
			name:         "PullRequestUpdateEvent",
			event:        sourcecraft.PullRequestUpdateEvent,
			filename:     "testdata/pull-request-update.json",
			expectedType: sourcecraft.PullRequestUpdateEvent,
		},
		{
			name:         "PullRequestPublishEvent",
			event:        sourcecraft.PullRequestPublishEvent,
			filename:     "testdata/pull-request-publish.json",
			expectedType: sourcecraft.PullRequestPublishEvent,
		},
		{
			name:         "PullRequestRefreshEvent",
			event:        sourcecraft.PullRequestRefreshEvent,
			filename:     "testdata/pull-request-refresh.json",
			expectedType: sourcecraft.PullRequestRefreshEvent,
		},
		{
			name:         "PullRequestMergeEvent",
			event:        sourcecraft.PullRequestMergeEvent,
			filename:     "testdata/pull-request-merge.json",
			expectedType: sourcecraft.PullRequestMergeEvent,
		},
		{
			name:         "PullRequestMergeFailureEvent",
			event:        sourcecraft.PullRequestMergeFailureEvent,
			filename:     "testdata/pull-request-merge-failure.json",
			expectedType: sourcecraft.PullRequestMergeFailureEvent,
		},
		{
			name:         "PullRequestNewIterationEvent",
			event:        sourcecraft.PullRequestNewIterationEvent,
			filename:     "testdata/pull-request-new-iteration.json",
			expectedType: sourcecraft.PullRequestNewIterationEvent,
		},
		{
			name:         "PullRequestReviewAssignmentEvent",
			event:        sourcecraft.PullRequestReviewAssignmentEvent,
			filename:     "testdata/pull-request-review-assignment.json",
			expectedType: sourcecraft.PullRequestReviewAssignmentEvent,
		},
		{
			name:         "PullRequestReviewDecisionEvent",
			event:        sourcecraft.PullRequestReviewDecisionEvent,
			filename:     "testdata/pull-request-review-decision.json",
			expectedType: sourcecraft.PullRequestReviewDecisionEvent,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tt.name, func(t *testing.T) {
			payloadBytes, err := os.ReadFile(tc.filename)
			assert.NoError(err)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payloadBytes))
			req.Header.Set("X-Src-Event", string(tc.event))
			req.Header.Set("Content-Type", "application/json")

			mac := hmac.New(sha256.New, []byte("test-secret"))
			mac.Write(payloadBytes)
			req.Header.Set("X-Src-Signature", hex.EncodeToString(mac.Sum(nil)))

			payload, err := hook.Parse(req, tc.event)
			assert.NoError(err)

			prEvent, ok := payload.(sourcecraft.PullRequestEventPayload)
			assert.True(ok, "Payload should implement PullRequestEventPayload interface")

			// Test GetEventType method
			eventType := prEvent.GetEventType()
			assert.Equal(tc.expectedType, eventType, "GetEventType should return %s", tc.expectedType)
			t.Logf("✓ GetEventType() correctly returns: %s", eventType)
		})
	}
}
