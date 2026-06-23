package pull_request

import (
	"sync"
)

// PRHintStore is a one-shot cache seeded by the webhook handler from pullrequest:created/updated
// payloads. Bitbucket Cloud's list API lags 2-6 min after creation; hints let List() return the
// new PR immediately without an API round-trip. Each entry is consumed once and deleted.
type PRHintStore struct {
	m sync.Map // key: "owner/repo" → []*PullRequest
}

// Set stores hint PRs for a given owner/repo pair, replacing any existing entry.
func (s *PRHintStore) Set(owner, repo string, prs []*PullRequest) {
	s.m.Store(owner+"/"+repo, prs)
}

// Take returns and deletes the hint for owner/repo. Returns nil if none is set.
func (s *PRHintStore) Take(owner, repo string) []*PullRequest {
	key := owner + "/" + repo
	v, ok := s.m.LoadAndDelete(key)
	if !ok {
		return nil
	}
	prs, _ := v.([]*PullRequest)
	return prs
}
