package sourcecraft

import (
	"time"
)

// UserEmbedded represents a minimal user reference with ID and slug.
type UserEmbedded struct {
	// Id is the unique identifier of the user.
	Id string `json:"id"`
	// Slug is the URL-friendly identifier of the user.
	Slug string `json:"slug"`
}

// EventHeader is the common header included in all public events.
type EventHeader struct {
	// Id is the unique identifier for this event instance.
	Id string `json:"id"`
	// Type is the event type.
	Type string `json:"type"`
	// OccurredAt is when the event occurred.
	OccurredAt time.Time `json:"occurred_at"`
	// AggregateId is the ID of the primary entity this event is about.
	AggregateId string `json:"aggregate_id"`
	// AggregateType is the type of the primary entity (derived from event_type).
	AggregateType string `json:"aggregate_type"`
	// Metadata is the public metadata.
	Metadata map[string]string `json:"metadata"`
	// OrganizationId is the organization context.
	OrganizationId string `json:"organization_id"`
	// RepositoryId is the repository context.
	RepositoryId *string `json:"repository_id"`
	// TriggeredBy is the user that triggered event.
	TriggeredBy *UserEmbedded `json:"triggered_by"`
}

// Repository represents a source code repository with its metadata and settings.
type Repository struct {
	// Id is the unique identifier of the repository.
	Id string `json:"id"`
	// Name is the display name of the repository.
	Name string `json:"name"`
	// DefaultBranch is the name of the default branch (e.g., "main" or "master").
	DefaultBranch string `json:"default_branch"`
	// Organization is the organization that owns the repository.
	Organization *OrganizationEmbedded `json:"organization"`
	// Slug is the URL-friendly identifier of the repository.
	Slug string `json:"slug"`
	// TemplateType specifies the template used to create the repository, if any.
	TemplateType string `json:"template_type"`
	// IsEmpty indicates whether the repository has no commits.
	IsEmpty bool `json:"is_empty"`
	// Description is the repository's description text.
	Description string `json:"description"`
	// Visibility specifies the repository's visibility (e.g., "public", "private").
	Visibility string `json:"visibility"`
	// Logo is the repository's logo image, if set.
	Logo *Image `json:"logo"`
	// CloneURL contains the URLs for cloning the repository.
	CloneURL *CloneURL `json:"clone_url"`
	// WebURL is the URL to view the repository in a web browser.
	WebURL string `json:"web_url"`
	// Links contains additional related links for the repository.
	Links []*Link `json:"links"`
	// Counters contains various statistics about the repository.
	Counters *RepositoryCounters `json:"counters"`
	// LastUpdated is the timestamp of the last update to the repository.
	LastUpdated *time.Time `json:"last_updated"`
	// Language is the primary programming language used in the repository.
	Language *Language `json:"language"`
	// Parent is the parent repository if this is a fork.
	Parent *RepositoryEmbedded `json:"parent"`
}

// OrganizationEmbedded represents a minimal organization reference with ID and slug.
type OrganizationEmbedded struct {
	// Id is the unique identifier of the organization.
	Id string `json:"id"`
	// Slug is the URL-friendly identifier of the organization.
	Slug string `json:"slug"`
}

// RepositoryEmbedded represents a minimal repository reference with ID and slug.
type RepositoryEmbedded struct {
	// Id is the unique identifier of the repository.
	Id string `json:"id"`
	// Slug is the URL-friendly identifier of the repository.
	Slug string `json:"slug"`
}

// Image represents an image with its URL.
type Image struct {
	// URL is the URL where the image can be accessed.
	URL string `json:"url"`
}

// CloneURL contains the URLs for cloning a repository via different protocols.
type CloneURL struct {
	// HTTPS is the HTTPS clone URL.
	HTTPS string `json:"https"`
	// SSH is the SSH clone URL.
	SSH string `json:"ssh"`
}

// Link represents a related link with its type.
type Link struct {
	// Link is the URL of the link.
	Link string `json:"link"`
	// Type specifies the type or category of the link.
	Type string `json:"type"`
}

// RepositoryCounters contains various statistics about a repository.
type RepositoryCounters struct {
	// Forks is the number of forks of the repository.
	Forks string `json:"forks"`
	// PullRequests is the number of pull requests in the repository.
	PullRequests string `json:"pull_requests"`
	// Issues is the number of issues in the repository.
	Issues string `json:"issues"`
	// Tags is the number of tags in the repository.
	Tags string `json:"tags"`
	// Branches is the number of branches in the repository.
	Branches string `json:"branches"`
}

// Language represents a programming language with its display properties.
type Language struct {
	// Name is the name of the programming language.
	Name string `json:"name"`
	// Color is the hex color code associated with the language.
	Color string `json:"color"`
}

// RefUpdate represents a single ref update within a push.
type RefUpdate struct {
	// Ref is the git reference being updated (e.g., "refs/heads/main").
	Ref string `json:"ref"`
	// Operation is the operation type.
	Operation string `json:"operation"`
	// BeforeSha is the commit SHA before the update (zeros for new ref).
	BeforeSha string `json:"before_sha"`
	// AfterSha is the commit SHA after the update (zeros for deleted ref).
	AfterSha string `json:"after_sha"`
	// CheckoutSha is for annotated tags: the peeled SHA (commit the tag points to).
	CheckoutSha string `json:"checkout_sha"`
}

// Branch represents a Git branch with its latest commit.
type Branch struct {
	// Name is the name of the branch.
	Name string `json:"name"`
	// Commit is the latest commit on the branch.
	Commit *Commit `json:"commit"`
}

// Label represents a label that can be applied to issues and pull requests.
type Label struct {
	// Id is the unique identifier of the label.
	Id string `json:"id"`
	// Name is the display name of the label.
	Name string `json:"name"`
	// Slug is the URL-friendly identifier of the label.
	Slug string `json:"slug"`
	// Color is the hex color code for the label.
	Color string `json:"color"`
	// Author is the user who created the label.
	Author *UserEmbedded `json:"author"`
	// UpdatedBy is the user who last updated the label.
	UpdatedBy *UserEmbedded `json:"updated_by"`
	// CreatedAt is the timestamp when the label was created.
	CreatedAt *time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the label was last updated.
	UpdatedAt *time.Time `json:"updated_at"`
}

// TreeEntry represents an entry in a Git tree (file or directory).
type TreeEntry struct {
	// Name is the name of the file or directory.
	Name string `json:"name"`
	// Path is the full path to the file or directory.
	Path string `json:"path"`
	// Type specifies whether this is a file, directory, or other type.
	Type string `json:"type"`
}

// Commit represents a Git commit with its metadata and changes.
type Commit struct {
	// Hash is the SHA-1 hash of the commit.
	Hash string `json:"hash"`
	// Message is the commit message.
	Message string `json:"message"`
	// Author is the author of the commit.
	Author *Signature `json:"author"`
	// Committer is the committer of the commit.
	Committer *Signature `json:"committer"`
	// TreeHash is the SHA-1 hash of the commit's tree object.
	TreeHash string `json:"tree_hash"`
	// ParentHashes are the SHA-1 hashes of the parent commits.
	ParentHashes []string `json:"parent_hashes"`
	// MergeTag is the merge tag, if this is a merge commit.
	MergeTag string `json:"merge_tag"`
	// FileChanges contains the files that were changed in this commit.
	FileChanges *CommitFileChanges `json:"file_changes"`
}

// Signature represents a Git signature (author or committer) with timestamp.
type Signature struct {
	// Name is the name of the person.
	Name string `json:"name"`
	// Email is the email address of the person.
	Email string `json:"email"`
	// Date is the timestamp of the signature.
	Date *time.Time `json:"date"`
}

// CommitFileChanges represents the files changed in a commit.
type CommitFileChanges struct {
	// Added contains the paths of files that were added.
	Added []string `json:"added"`
	// Modified contains the paths of files that were modified.
	Modified []string `json:"modified"`
	// Removed contains the paths of files that were removed.
	Removed []string `json:"removed"`
}

// PullRequest represents a pull request with its metadata and status.
type PullRequest struct {
	// Id is the unique identifier of the pull request.
	Id string `json:"id"`
	// Slug is the URL-friendly identifier of the pull request.
	Slug string `json:"slug"`
	// Author is the user who created the pull request.
	Author *UserEmbedded `json:"author"`
	// UpdatedBy is the user who last updated the pull request.
	UpdatedBy *UserEmbedded `json:"updated_by"`
	// Title is the title of the pull request.
	Title string `json:"title"`
	// Description is the description text of the pull request.
	Description string `json:"description"`
	// Repository is the repository where the pull request exists.
	Repository *RepositoryEmbedded `json:"repository"`
	// MergeInfo contains information about the merge status and parameters.
	MergeInfo *MergeInfo `json:"merge_info"`
	// SourceBranch is the branch being merged from.
	SourceBranch string `json:"source_branch"`
	// TargetBranch is the branch being merged into.
	TargetBranch string `json:"target_branch"`
	// Status is the current status of the pull request (e.g., "open", "merged", "closed").
	Status string `json:"status"`
	// CreatedAt is the timestamp when the pull request was created.
	CreatedAt *time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the pull request was last updated.
	UpdatedAt *time.Time `json:"updated_at"`
	// LinkedIssues contains issues linked to this pull request.
	LinkedIssues []*IssueEmbedded `json:"linked_issues"`
}

// IssueEmbedded represents a minimal issue reference with ID and slug.
type IssueEmbedded struct {
	// Id is the unique identifier of the issue.
	Id string `json:"id"`
	// Slug is the URL-friendly identifier of the issue.
	Slug string `json:"slug"`
}

// MergeInfo contains information about a pull request merge operation.
type MergeInfo struct {
	// Merger is the user who performed the merge.
	Merger *UserEmbedded `json:"merger"`
	// MergeParameters contains the parameters used for the merge operation.
	MergeParameters *MergeParameters `json:"merge_parameters"`
	// TargetCommitHash is the commit hash of the target branch at merge time.
	TargetCommitHash string `json:"target_commit_hash"`
	// Error contains the error message if the merge failed.
	Error *string `json:"error"`
	// MergeCommitHash is the commit hash created by the merge, if successful.
	MergeCommitHash *string `json:"merge_commit_hash"`
}

// MergeParameters specifies the options for merging a pull request.
type MergeParameters struct {
	// Rebase indicates whether to rebase the source branch before merging.
	Rebase *bool `json:"rebase"`
	// Squash indicates whether to squash commits before merging.
	Squash *bool `json:"squash"`
	// DeleteBranch indicates whether to delete the source branch after merging.
	DeleteBranch *bool `json:"delete_branch"`
}

// ReviewerDelta represents a change in the reviewer list for a pull request.
type ReviewerDelta struct {
	// Action is the action performed.
	Action string `json:"action"`
	// UserId is the user ID.
	UserId string `json:"user_id"`
}

// PingEventPayload is sent when a webhook is tested.
type PingEventPayload struct {
	// Header is the event header with common fields.
	Header *EventHeader `json:"header"`
	// Repository is the repository that the webhook is attached to.
	Repository *RepositoryEmbedded `json:"repository"`
	// WebhookSlug is the webhook slug identifier.
	WebhookSlug string `json:"webhook_slug"`
	// PingedAt is when the ping was sent.
	PingedAt *time.Time `json:"pinged_at"`
	// Organization is the organization that the repository belongs to.
	Organization *OrganizationEmbedded `json:"organization"`
}

// PushEventPayload represents git push operation to a repository.
type PushEventPayload struct {
	// Header is the event header with common fields.
	Header *EventHeader `json:"header"`
	// Repository is the repository that received the push.
	Repository *Repository `json:"repository"`
	// RefUpdate is the push details - single ref update in a single push.
	RefUpdate *RefUpdate `json:"ref_update"`
	// Pusher is who performed the push.
	Pusher *UserEmbedded `json:"pusher"`
	// PushedAt is the timestamp when the push occurred.
	PushedAt *time.Time `json:"pushed_at"`
	// Commits contains commit details for this event, from before_sha to after_sha.
	// In case of tag push or new branch creation before_sha is zero
	// and commits contain only single commit.
	Commits []*Commit `json:"commits"`
	// IsDefaultBranchUpdated indicates whether the default branch was updated in this push.
	IsDefaultBranchUpdated bool `json:"is_default_branch_updated"`
	// DefaultBranch is the default branch reference name (e.g., "refs/heads/main").
	DefaultBranch string `json:"default_branch"`
	// HasMoreCommits indicates whether there are more commits (due to commits limit).
	HasMoreCommits bool `json:"has_more_commits"`
}

// PullRequestCreateEventPayload represents creation of a new pull request.
type PullRequestCreateEventPayload struct {
	// Header is the event header with common fields.
	Header *EventHeader `json:"header"`
	// Repository is the repository where the pull request was created.
	Repository *Repository `json:"repository"`
	// PullRequest is the pull request that was created.
	PullRequest *PullRequest `json:"pull_request"`
	// CreatedAt is when the pull request was created.
	CreatedAt *time.Time `json:"created_at"`
}

// PullRequestUpdateEventPayload represents update of a pull request.
type PullRequestUpdateEventPayload struct {
	// Header is the event header with common fields.
	Header *EventHeader `json:"header"`
	// Repository is the repository where the pull request is located.
	Repository *Repository `json:"repository"`
	// PullRequest is the pull request that was updated.
	PullRequest *PullRequest `json:"pull_request"`
}

// PullRequestPublishEventPayload represents publish of a pull request.
type PullRequestPublishEventPayload struct {
	// Header is the event header with common fields.
	Header *EventHeader `json:"header"`
	// Repository is the repository where the pull request was published.
	Repository *Repository `json:"repository"`
	// PullRequest is the pull request that was published.
	PullRequest *PullRequest `json:"pull_request"`
	// PreviousStatus is the pull request previous status.
	PreviousStatus string `json:"previous_status"`
}

// PullRequestRefreshEventPayload represents refresh of a pull request.
type PullRequestRefreshEventPayload struct {
	// Header is the event header with common fields.
	Header *EventHeader `json:"header"`
	// Repository is the repository where the pull request is located.
	Repository *Repository `json:"repository"`
	// PullRequest is the pull request that is being refreshed.
	PullRequest *PullRequest `json:"pull_request"`
	// PreviousStatus is the pull request previous status.
	PreviousStatus string `json:"previous_status"`
	// HeadSha is the head SHA of the pull request.
	HeadSha string `json:"head_sha"`
	// MergeBaseSha is the merge base SHA of the pull request.
	MergeBaseSha string `json:"merge_base_sha"`
}

// PullRequestMergeFailureEventPayload represents merge failure of a pull request.
type PullRequestMergeFailureEventPayload struct {
	// Header is the event header with common fields.
	Header *EventHeader `json:"header"`
	// Repository is the repository where the pull request is located.
	Repository *Repository `json:"repository"`
	// PullRequest is the pull request that is being refreshed.
	PullRequest *PullRequest `json:"pull_request"`
	// ErrorMessage is the error message of the merge failure.
	ErrorMessage string `json:"error_message"`
}

// PullRequestMergeEventPayload represents merge success of a pull request.
type PullRequestMergeEventPayload struct {
	// Header is the event header with common fields.
	Header *EventHeader `json:"header"`
	// Repository is the repository where the pull request is located.
	Repository *Repository `json:"repository"`
	// PullRequest is the pull request that is being refreshed.
	PullRequest *PullRequest `json:"pull_request"`
	// MergeHash is the merge hash of the pull request.
	MergeHash string `json:"merge_hash"`
}

// PullRequestNewIterationEventPayload represents new iteration on a pull request.
type PullRequestNewIterationEventPayload struct {
	// Header is the event header with common fields.
	Header *EventHeader `json:"header"`
	// Repository is the repository where the pull request is located.
	Repository *Repository `json:"repository"`
	// PullRequest is the pull request that is being refreshed.
	PullRequest *PullRequest `json:"pull_request"`
	// CommitSha is the commit SHA of the iteration.
	CommitSha string `json:"commit_sha"`
	// MergeBaseSha is the merge base SHA of the iteration.
	MergeBaseSha string `json:"merge_base_sha"`
	// CreatedAt is the iteration created at timestamp.
	CreatedAt *time.Time `json:"created_at"`
	// UpdatedAt is the iteration updated at timestamp.
	UpdatedAt *time.Time `json:"updated_at"`
}

// PullRequestReviewAssignmentEventPayload represents changes to pull request reviewers.
type PullRequestReviewAssignmentEventPayload struct {
	// Header is the event header with common fields.
	Header *EventHeader `json:"header"`
	// Repository is the repository where the pull request is located.
	Repository *Repository `json:"repository"`
	// PullRequest is the pull request that had reviewers modified.
	PullRequest *PullRequest `json:"pull_request"`
	// User is the user who made the assignment.
	User *UserEmbedded `json:"user"`
	// ReviewerDeltas is the reviewers deltas.
	ReviewerDeltas []*ReviewerDelta `json:"reviewer_deltas"`
}

// PullRequestReviewDecisionEventPaylaod represents a review decision on a pull request.
type PullRequestReviewDecisionEventPaylaod struct {
	// Header is the event header with common fields.
	Header *EventHeader `json:"header"`
	// Repository is the repository where the pull request is located.
	Repository *Repository `json:"repository"`
	// PullRequest is the pull request that received the decision.
	PullRequest *PullRequest `json:"pull_request"`
	// User is the user who made the decision.
	User *UserEmbedded `json:"user"`
	// Decision is the review decision (ship, sticky_ship, block, abstain, or null for withdrawal).
	Decision string `json:"decision"`
}
