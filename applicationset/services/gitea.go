package services

import (
	"strconv"

	"code.gitea.io/sdk/gitea"
)

const (
	// GiteaPageSize is the number of items requested per Gitea API call. Gitea
	// clamps the page size to its MAX_RESPONSE_ITEMS setting, so a short page
	// does not mean the last page.
	GiteaPageSize = 50
	// GiteaMaxPages bounds the paging loops so that a server which never reports
	// the end of a list fails loudly instead of looping forever.
	GiteaMaxPages = 1000
)

// GiteaAllCollected reports whether every item of a list endpoint has been
// collected, based on the X-Total-Count header Gitea sets on its list
// responses. When the header is absent the caller keeps paging until it gets an
// empty page.
func GiteaAllCollected(resp *gitea.Response, collected int) bool {
	if resp == nil {
		return false
	}
	total, err := strconv.Atoi(resp.Header.Get("X-Total-Count"))
	if err != nil {
		return false
	}
	return collected >= total
}
