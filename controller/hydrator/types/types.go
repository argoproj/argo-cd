package types

import "hash/fnv"

// HydrationQueueKey is used to uniquely identify a hydration operation in the queue. If several applications request
// hydration, but they have the same queue key, only one hydration operation will be performed.
type HydrationQueueKey struct {
	// SourceRepoURL must be normalized with git.NormalizeGitURL to ensure that we don't double-queue a single hydration
	// operation because two apps have different URL formats.
	SourceRepoURL        string
	SourceTargetRevision string
	DestinationRepoURL   string
	DestinationBranch    string
}

// Shard returns the controller shard responsible for this hydration group.
// Hydration groups may contain Applications whose destination clusters belong
// to different shards, so ownership must be based on the group key rather than
// on any one Application's destination.
func (k HydrationQueueKey) Shard(replicas int) int {
	if replicas <= 1 {
		return 0
	}

	h := fnv.New32a()
	for _, field := range []string{k.SourceRepoURL, k.SourceTargetRevision, k.DestinationRepoURL, k.DestinationBranch} {
		_, _ = h.Write([]byte(field))
		// Keep field boundaries unambiguous.
		_, _ = h.Write([]byte{0})
	}
	return int(h.Sum32() % uint32(replicas))
}
