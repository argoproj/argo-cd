package sourcecraft

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Event represents a webhook event type from SourceCraft.
type Event string

// EventAggregate represents an aggregate type that matches multiple related events.
type EventAggregate string

// EventMatcher is an interface that both Event and EventAggregate implement.
// This allows the Parse method to accept both specific events and event aggregates.
type EventMatcher interface {
	matchesEvent(event string) bool
}

// matchesEvent implements EventMatcher for Event.
func (e Event) matchesEvent(event string) bool {
	return string(e) == event
}

// matchesEvent implements EventMatcher for EventAggregate.
func (e EventAggregate) matchesEvent(event string) bool {
	// Match any event that starts with the aggregate prefix followed by a dot
	return strings.HasPrefix(event, string(e)+".")
}

// Webhook parsing errors.
var (
	ErrEventNotSpecifiedToParse = errors.New("no Event specified to parse")
	ErrInvalidHTTPMethod        = errors.New("invalid HTTP Method")
	ErrMissingSrcEventHeader    = errors.New("missing X-Src-Event Header")
	ErrMissingSignatureHeader   = errors.New("missing X-Src-Signature Header")
	ErrEventNotFound            = errors.New("event not defined to be parsed")
	ErrParsingPayload           = errors.New("error parsing payload")
	ErrHMACVerificationFailed   = errors.New("HMAC verification failed")
)

// Supported webhook event aggregates.
const (
	// PullRequestAggregate matches all pull request events (pull_request.*).
	// Use this to subscribe to all pull request events without listing each one.
	PullRequestAggregate EventAggregate = "pull_request"
)

// Supported webhook event types.
const (
	// PingEvent is sent when a webhook is first created or tested.
	PingEvent Event = "webhook.ping"
	// PushEvent is sent when commits are pushed to a repository.
	PushEvent Event = "repository.push"
	// PullRequestCreateEvent is sent when a pull request is created.
	PullRequestCreateEvent Event = "pull_request.create"
	// PullRequestUpdateEvent is sent when a pull request is updated.
	PullRequestUpdateEvent Event = "pull_request.update"
	// PullRequestPublishEvent is sent when a pull request is published.
	PullRequestPublishEvent Event = "pull_request.publish"
	// PullRequestRefreshEvent is sent when a pull request is refreshed.
	PullRequestRefreshEvent Event = "pull_request.refresh"
	// PullRequestMergeFailureEvent is sent when a pull request merge fails.
	PullRequestMergeFailureEvent Event = "pull_request.merge_failure"
	// PullRequestMergeEvent is sent when a pull request is merged.
	PullRequestMergeEvent Event = "pull_request.merge"
	// PullRequestNewIterationEvent is sent when a new iteration is added to a pull request.
	PullRequestNewIterationEvent Event = "pull_request.new_iteration"
	// PullRequestReviewAssignmentEvent is sent when a reviewer is assigned to a pull request.
	PullRequestReviewAssignmentEvent Event = "pull_request.review_assignment"
	// PullRequestReviewDecisionEvent is sent when a review decision is made on a pull request.
	PullRequestReviewDecisionEvent Event = "pull_request.review_decision"
)

// WebhookOption is a functional option for configuring a Webhook.
type WebhookOption func(*Webhook) error

// Options provides a namespace for webhook configuration options.
var Options = WebhookOptions{}

// WebhookOptions provides methods for creating webhook configuration options.
type WebhookOptions struct{}

// Secret returns an Option that sets the webhook secret for HMAC signature verification.
// Multiple secrets can be provided as a comma-separated string to support secret rotation.
func (WebhookOptions) Secret(secret string) WebhookOption {
	return func(hook *Webhook) error {
		hook.secret = secret
		return nil
	}
}

// Webhook handles parsing and verification of SourceCraft webhook requests.
type Webhook struct {
	secret string
}

// New creates a new Webhook instance with the provided options.
// Returns an error if any option fails to apply.
func New(options ...WebhookOption) (*Webhook, error) {
	hook := new(Webhook)
	for _, opt := range options {
		if err := opt(hook); err != nil {
			return nil, errors.New("option error")
		}
	}
	return hook, nil
}

// Parse parses a webhook HTTP request and returns the corresponding event payload.
// It validates the HTTP method, event type, and HMAC signature (if a secret is configured).
// The events parameter specifies which event types or aggregates are allowed to be parsed.
// You can pass specific Event types (e.g., PullRequestCreateEvent) or EventAggregate types
// (e.g., PullRequestAggregate) to match multiple events at once.
// Returns the parsed payload as an interface{} that can be type-asserted to the specific payload type,
// or an error if validation or parsing fails.
func (hook Webhook) Parse(r *http.Request, events ...EventMatcher) (any, error) {
	defer func() {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
	}()

	if len(events) == 0 {
		return nil, ErrEventNotSpecifiedToParse
	}
	if r.Method != http.MethodPost {
		return nil, ErrInvalidHTTPMethod
	}

	event := r.Header.Get("X-Src-Event")
	if event == "" {
		return nil, ErrMissingSrcEventHeader
	}

	// Check if the incoming event matches any of the provided matchers
	var found bool
	for _, matcher := range events {
		if matcher.matchesEvent(event) {
			found = true
			break
		}
	}
	if !found {
		return nil, ErrEventNotFound
	}

	srcEvent := Event(event)

	payload, err := io.ReadAll(r.Body)
	if err != nil || len(payload) == 0 {
		return nil, ErrParsingPayload
	}

	if hook.secret != "" {
		err := hook.verifySignature(r, payload)
		if err != nil {
			return nil, err
		}
	}

	switch srcEvent {
	case PingEvent:
		var pl PingEventPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case PushEvent:
		var pl PushEventPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case PullRequestCreateEvent:
		var pl PullRequestCreateEventPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case PullRequestUpdateEvent:
		var pl PullRequestUpdateEventPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case PullRequestPublishEvent:
		var pl PullRequestPublishEventPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case PullRequestRefreshEvent:
		var pl PullRequestRefreshEventPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case PullRequestMergeFailureEvent:
		var pl PullRequestMergeFailureEventPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case PullRequestMergeEvent:
		var pl PullRequestMergeEventPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case PullRequestNewIterationEvent:
		var pl PullRequestNewIterationEventPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case PullRequestReviewAssignmentEvent:
		var pl PullRequestReviewAssignmentEventPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case PullRequestReviewDecisionEvent:
		var pl PullRequestReviewDecisionEventPaylaod
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	default:
		return nil, fmt.Errorf("unknown event %s", srcEvent)
	}
}

func (hook Webhook) verifySignature(r *http.Request, payload []byte) error {
	signature := r.Header.Get("X-Src-Signature")
	if signature == "" {
		return ErrMissingSignatureHeader
	}
	secrets := strings.SplitSeq(hook.secret, ",")
	for secret := range secrets {
		s := strings.TrimSpace(secret)
		mac := hmac.New(sha256.New, []byte(s))
		_, _ = mac.Write(payload)
		expectedMAC := hex.EncodeToString(mac.Sum(nil))
		if hmac.Equal([]byte(signature), []byte(expectedMAC)) {
			return nil
		}
	}
	return ErrHMACVerificationFailed
}
