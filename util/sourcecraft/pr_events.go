package sourcecraft

// PullRequestEventPayload is a common interface for all pull request event payloads.
// This interface allows using Go type switch to handle all PR events in a single case branch.
type PullRequestEventPayload interface {
	// GetHeader returns the event header with common fields.
	GetHeader() *EventHeader
	// GetRepository returns the repository where the pull request is located.
	GetRepository() *Repository
	// GetPullRequest returns the pull request.
	GetPullRequest() *PullRequest
	// GetEventType returns the event type.
	GetEventType() Event
}

// GetHeader returns the event header.
func (p PullRequestCreateEventPayload) GetHeader() *EventHeader {
	return p.Header
}

// GetRepository returns the repository.
func (p PullRequestCreateEventPayload) GetRepository() *Repository {
	return p.Repository
}

// GetPullRequest returns the pull request.
func (p PullRequestCreateEventPayload) GetPullRequest() *PullRequest {
	return p.PullRequest
}

// GetEventType returns the event type.
func (p PullRequestCreateEventPayload) GetEventType() Event {
	return PullRequestCreateEvent
}

// GetHeader returns the event header.
func (p PullRequestUpdateEventPayload) GetHeader() *EventHeader {
	return p.Header
}

// GetRepository returns the repository.
func (p PullRequestUpdateEventPayload) GetRepository() *Repository {
	return p.Repository
}

// GetPullRequest returns the pull request.
func (p PullRequestUpdateEventPayload) GetPullRequest() *PullRequest {
	return p.PullRequest
}

// GetEventType returns the event type.
func (p PullRequestUpdateEventPayload) GetEventType() Event {
	return PullRequestUpdateEvent
}

// GetHeader returns the event header.
func (p PullRequestPublishEventPayload) GetHeader() *EventHeader {
	return p.Header
}

// GetRepository returns the repository.
func (p PullRequestPublishEventPayload) GetRepository() *Repository {
	return p.Repository
}

// GetPullRequest returns the pull request.
func (p PullRequestPublishEventPayload) GetPullRequest() *PullRequest {
	return p.PullRequest
}

// GetEventType returns the event type.
func (p PullRequestPublishEventPayload) GetEventType() Event {
	return PullRequestPublishEvent
}

// GetHeader returns the event header.
func (p PullRequestRefreshEventPayload) GetHeader() *EventHeader {
	return p.Header
}

// GetRepository returns the repository.
func (p PullRequestRefreshEventPayload) GetRepository() *Repository {
	return p.Repository
}

// GetPullRequest returns the pull request.
func (p PullRequestRefreshEventPayload) GetPullRequest() *PullRequest {
	return p.PullRequest
}

// GetEventType returns the event type.
func (p PullRequestRefreshEventPayload) GetEventType() Event {
	return PullRequestRefreshEvent
}

// GetHeader returns the event header.
func (p PullRequestMergeFailureEventPayload) GetHeader() *EventHeader {
	return p.Header
}

// GetRepository returns the repository.
func (p PullRequestMergeFailureEventPayload) GetRepository() *Repository {
	return p.Repository
}

// GetPullRequest returns the pull request.
func (p PullRequestMergeFailureEventPayload) GetPullRequest() *PullRequest {
	return p.PullRequest
}

// GetEventType returns the event type.
func (p PullRequestMergeFailureEventPayload) GetEventType() Event {
	return PullRequestMergeFailureEvent
}

// GetHeader returns the event header.
func (p PullRequestMergeEventPayload) GetHeader() *EventHeader {
	return p.Header
}

// GetRepository returns the repository.
func (p PullRequestMergeEventPayload) GetRepository() *Repository {
	return p.Repository
}

// GetPullRequest returns the pull request.
func (p PullRequestMergeEventPayload) GetPullRequest() *PullRequest {
	return p.PullRequest
}

// GetEventType returns the event type.
func (p PullRequestMergeEventPayload) GetEventType() Event {
	return PullRequestMergeEvent
}

// GetHeader returns the event header.
func (p PullRequestNewIterationEventPayload) GetHeader() *EventHeader {
	return p.Header
}

// GetRepository returns the repository.
func (p PullRequestNewIterationEventPayload) GetRepository() *Repository {
	return p.Repository
}

// GetPullRequest returns the pull request.
func (p PullRequestNewIterationEventPayload) GetPullRequest() *PullRequest {
	return p.PullRequest
}

// GetEventType returns the event type.
func (p PullRequestNewIterationEventPayload) GetEventType() Event {
	return PullRequestNewIterationEvent
}

// GetHeader returns the event header.
func (p PullRequestReviewAssignmentEventPayload) GetHeader() *EventHeader {
	return p.Header
}

// GetRepository returns the repository.
func (p PullRequestReviewAssignmentEventPayload) GetRepository() *Repository {
	return p.Repository
}

// GetPullRequest returns the pull request.
func (p PullRequestReviewAssignmentEventPayload) GetPullRequest() *PullRequest {
	return p.PullRequest
}

// GetEventType returns the event type.
func (p PullRequestReviewAssignmentEventPayload) GetEventType() Event {
	return PullRequestReviewAssignmentEvent
}

// GetHeader returns the event header.
func (p PullRequestReviewDecisionEventPaylaod) GetHeader() *EventHeader {
	return p.Header
}

// GetRepository returns the repository.
func (p PullRequestReviewDecisionEventPaylaod) GetRepository() *Repository {
	return p.Repository
}

// GetPullRequest returns the pull request.
func (p PullRequestReviewDecisionEventPaylaod) GetPullRequest() *PullRequest {
	return p.PullRequest
}

// GetEventType returns the event type.
func (p PullRequestReviewDecisionEventPaylaod) GetEventType() Event {
	return PullRequestReviewDecisionEvent
}
