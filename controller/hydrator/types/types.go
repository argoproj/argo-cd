package types

type HydrationQueueKey struct {
	// SourceRepoURL must be normalized with git.NormalizeGitURL to ensure that we don't double-queue a single hydration
	// operation because two apps have different URL formats.
	SourceRepoURL        string
	SourceTargetRevision string
	DestinationBranch    string
}
