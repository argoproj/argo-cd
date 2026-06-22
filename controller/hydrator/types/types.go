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

// Shard returns the deterministic shard number responsible for this hydration key, given the total
// number of controller replicas. A hydration group can span multiple clusters (which are sharded
// independently), so hydration must be assigned an owner by the key itself rather than by cluster.
// The mapping uses the same FNV-32a modulo scheme as the legacy cluster distribution function so the
// behavior is familiar and stable for a fixed replica count. With one (or fewer) replicas every key
// is owned by shard 0, matching the controller's "process all shards" mode.
func (k HydrationQueueKey) Shard(replicas int) int {
	if replicas <= 1 {
		return 0
	}
	h := fnv.New32a()
	// A NUL separator keeps field boundaries unambiguous so distinct keys cannot collide by
	// concatenation (e.g. {"ab", "c"} vs {"a", "bc"}).
	for _, field := range []string{k.SourceRepoURL, k.SourceTargetRevision, k.DestinationRepoURL, k.DestinationBranch} {
		_, _ = h.Write([]byte(field))
		_, _ = h.Write([]byte{0})
	}
	return int(h.Sum32() % uint32(replicas))
}
