package types

// HydrationQueueKey is used to uniquely identify a hydration operation in the queue. If several applications request
// hydration, but they have the same queue key, only one hydration operation will be performed.
type HydrationQueueKey struct {
	// SourceRepoURL must be normalized with git.NormalizeGitURL to ensure that we don't double-queue a single hydration
	// operation because two apps have different URL formats.
	SourceRepoURL        string
	SourceTargetRevision string
	DestinationBranch    string
}
