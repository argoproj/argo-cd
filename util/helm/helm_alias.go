package helm

import "strings"

// Get Repo alias from the repo provided if it starts with @ or alias:
// Returns an empty string otherwise
func GetRepoNameFromAlias(repo string) string {
	repoName := ""
	if strings.HasPrefix(repo, "@") {
		repoName = repo[1:]
	} else if strings.HasPrefix(repo, "alias:") {
		repoName = strings.TrimPrefix(repo, "alias:")
	}

	return repoName
}
