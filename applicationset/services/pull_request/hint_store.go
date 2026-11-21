package pull_request

import (
	"strings"
	"sync"
)

// PRHintStore is a per-repo accumulator seeded from webhook payloads, letting
// List() return newly-opened PRs before Bitbucket's list API reflects them.
type PRHintStore struct {
	// Plain map under a mutex, not sync.Map: the values are slices, which are
	// uncomparable and so cannot be used with sync.Map's CompareAndSwap.
	mu sync.Mutex
	m  map[string][]*PullRequest
}

// hintKey is case-insensitive, like the owner/repo matching that decides
// whether a webhook refreshes an ApplicationSet.
func hintKey(owner, repo string) string {
	return strings.ToLower(owner + "/" + repo)
}

// Set adds prs to any already-stored hints for owner/repo so that rapid
// back-to-back webhooks before the next reconcile don't drop earlier hints.
// A PR already hinted is replaced, so a later event's head SHA wins.
func (s *PRHintStore) Set(owner, repo string, prs []*PullRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = make(map[string][]*PullRequest)
	}
	key := hintKey(owner, repo)
	hints := s.m[key]
	for _, pr := range prs {
		replaced := false
		for i, existing := range hints {
			if existing.Number == pr.Number {
				hints[i] = pr
				replaced = true
				break
			}
		}
		if !replaced {
			hints = append(hints, pr)
		}
	}
	s.m[key] = hints
}

// Take consumes and returns all hints for the given repo. Returns nil if none are set.
func (s *PRHintStore) Take(owner, repo string) []*PullRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := hintKey(owner, repo)
	prs := s.m[key]
	delete(s.m, key)
	return prs
}
