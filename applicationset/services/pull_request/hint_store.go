package pull_request

import "sync"

// PRHintStore is a per-repo accumulator seeded from webhook payloads, letting
// List() return newly-opened PRs before Bitbucket's list API reflects them.
type PRHintStore struct {
	m sync.Map // key: "owner/repo" → []*PullRequest
}

// Set appends prs to any already-stored hints for owner/repo so that rapid
// back-to-back webhooks before the next reconcile don't drop earlier hints.
func (s *PRHintStore) Set(owner, repo string, prs []*PullRequest) {
	key := owner + "/" + repo
	for {
		existing, loaded := s.m.Load(key)
		var next []*PullRequest
		if loaded {
			next = append(existing.([]*PullRequest), prs...)
		} else {
			next = prs
		}
		if swapped := s.m.CompareAndSwap(key, existing, next); swapped || !loaded {
			if !loaded {
				s.m.Store(key, next)
			}
			return
		}
	}
}

// Take consumes and returns all hints for the given repo. Returns nil if none are set.
func (s *PRHintStore) Take(owner, repo string) []*PullRequest {
	v, ok := s.m.LoadAndDelete(owner + "/" + repo)
	if !ok {
		return nil
	}
	return v.([]*PullRequest)
}
