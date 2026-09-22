package pull_request

import "sync"

// PRHintStore is a per-repo accumulator seeded from webhook payloads, letting
// List() return newly-opened PRs before Bitbucket's list API reflects them.
type PRHintStore struct {
	// Plain map under a mutex, not sync.Map: the values are slices, which are
	// uncomparable and so cannot be used with sync.Map's CompareAndSwap.
	mu sync.Mutex
	m  map[string][]*PullRequest
}

// Set appends prs to any already-stored hints for owner/repo so that rapid
// back-to-back webhooks before the next reconcile don't drop earlier hints.
func (s *PRHintStore) Set(owner, repo string, prs []*PullRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = make(map[string][]*PullRequest)
	}
	key := owner + "/" + repo
	s.m[key] = append(s.m[key], prs...)
}

// Take consumes and returns all hints for the given repo. Returns nil if none are set.
func (s *PRHintStore) Take(owner, repo string) []*PullRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := owner + "/" + repo
	prs := s.m[key]
	delete(s.m, key)
	return prs
}
