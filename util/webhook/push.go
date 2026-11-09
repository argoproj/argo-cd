package webhook

import (
	"github.com/go-playground/webhooks/v6/azuredevops"
	"github.com/go-playground/webhooks/v6/bitbucket"
	bitbucketserver "github.com/go-playground/webhooks/v6/bitbucket-server"
	"github.com/go-playground/webhooks/v6/github"
	"github.com/go-playground/webhooks/v6/gitlab"
	gogsclient "github.com/gogits/go-gogs-client"
)

// PushEventInfo is the provider-independent view of a push event that both the
// Argo CD and ApplicationSet webhook handlers act on.
type PushEventInfo struct {
	// WebURLs are the repository URLs from the payload, without the .git suffix.
	WebURLs      []string
	Revision     string
	SHABefore    string
	SHAAfter     string
	TouchedHead  bool
	ChangedFiles []string
}

// ParsePushEvent extracts the push details carried in the payload itself. It
// returns nil when payload is not a supported push event.
func ParsePushEvent(payload any) *PushEventInfo {
	var info PushEventInfo
	switch payload := payload.(type) {
	case github.PushPayload:
		// See: https://developer.github.com/v3/activity/events/types/#pushevent
		info.WebURLs = []string{payload.Repository.HTMLURL}
		info.Revision = ParseRevision(payload.Ref)
		info.SHAAfter = ParseRevision(payload.After)
		info.SHABefore = ParseRevision(payload.Before)
		info.TouchedHead = payload.Repository.DefaultBranch == info.Revision
		for _, commit := range payload.Commits {
			info.ChangedFiles = append(info.ChangedFiles, commit.Added...)
			info.ChangedFiles = append(info.ChangedFiles, commit.Modified...)
			info.ChangedFiles = append(info.ChangedFiles, commit.Removed...)
		}
	case azuredevops.GitPushEvent:
		// See: https://learn.microsoft.com/en-us/azure/devops/service-hooks/events?view=azure-devops#git.push
		info.WebURLs = []string{payload.Resource.Repository.RemoteURL}
		if len(payload.Resource.RefUpdates) > 0 {
			info.Revision = ParseRevision(payload.Resource.RefUpdates[0].Name)
			info.SHAAfter = ParseRevision(payload.Resource.RefUpdates[0].NewObjectID)
			info.SHABefore = ParseRevision(payload.Resource.RefUpdates[0].OldObjectID)
			info.TouchedHead = payload.Resource.RefUpdates[0].Name == payload.Resource.Repository.DefaultBranch
		}
		// unfortunately, Azure DevOps doesn't provide a list of changed files
	case gitlab.PushEventPayload:
		// See: https://docs.gitlab.com/ee/user/project/integrations/webhooks.html
		info.WebURLs = []string{payload.Project.WebURL}
		info.Revision = ParseRevision(payload.Ref)
		info.SHAAfter = ParseRevision(payload.After)
		info.SHABefore = ParseRevision(payload.Before)
		info.TouchedHead = payload.Project.DefaultBranch == info.Revision
		for _, commit := range payload.Commits {
			info.ChangedFiles = append(info.ChangedFiles, commit.Added...)
			info.ChangedFiles = append(info.ChangedFiles, commit.Modified...)
			info.ChangedFiles = append(info.ChangedFiles, commit.Removed...)
		}
	case gitlab.TagEventPayload:
		// See: https://docs.gitlab.com/ee/user/project/integrations/webhooks.html
		info.WebURLs = []string{payload.Project.WebURL}
		info.Revision = ParseRevision(payload.Ref)
		info.SHAAfter = ParseRevision(payload.After)
		info.SHABefore = ParseRevision(payload.Before)
		// A tag push never moves the default branch, so it only affects sources
		// whose revision names or matches the tag.
		info.TouchedHead = false
		for _, commit := range payload.Commits {
			info.ChangedFiles = append(info.ChangedFiles, commit.Added...)
			info.ChangedFiles = append(info.ChangedFiles, commit.Modified...)
			info.ChangedFiles = append(info.ChangedFiles, commit.Removed...)
		}
	case bitbucket.RepoPushPayload:
		// See: https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-Push
		info.WebURLs = []string{payload.Repository.Links.HTML.Href}
		for _, change := range payload.Push.Changes {
			info.Revision = change.New.Name
			info.SHABefore = change.Old.Target.Hash
			info.SHAAfter = change.New.Target.Hash
			break
		}
		// The payload alone doesn't say whether the push moved the default branch,
		// so report true and let the consumer check, or refine it via the API.
		info.TouchedHead = true
	case bitbucketserver.RepositoryReferenceChangedPayload:
		_, info.WebURLs = bitbucketServerCloneURLs(payload.Repository)
		// TODO: bitbucket includes multiple changes as part of a single event.
		// We only pick the first but need to consider how to handle multiple
		for _, refChange := range payload.Changes {
			info.Revision = ParseRevision(refChange.Reference.ID)
			info.SHABefore = refChange.FromHash
			info.SHAAfter = refChange.ToHash
			break
		}
		// The payload doesn't carry the default branch, so report true as a safe
		// fallback; the consumer can refine it via the Bitbucket Server API.
		info.TouchedHead = true
	case gogsclient.PushPayload:
		info.Revision = ParseRevision(payload.Ref)
		info.SHAAfter = ParseRevision(payload.After)
		info.SHABefore = ParseRevision(payload.Before)
		if payload.Repo != nil {
			info.WebURLs = []string{payload.Repo.HTMLURL}
			info.TouchedHead = payload.Repo.DefaultBranch == info.Revision
		}
		for _, commit := range payload.Commits {
			info.ChangedFiles = append(info.ChangedFiles, commit.Added...)
			info.ChangedFiles = append(info.ChangedFiles, commit.Modified...)
			info.ChangedFiles = append(info.ChangedFiles, commit.Removed...)
		}
	default:
		return nil
	}
	return &info
}

// bitbucketServerCloneURLs returns the repository's HTTP and SSH clone URLs, and
// separately the HTTP one, which the Bitbucket Server API client needs.
func bitbucketServerCloneURLs(repo bitbucketserver.Repository) (httpURL string, urls []string) {
	// Webhook module does not parse the inner links
	clone, ok := repo.Links["clone"].([]any)
	if !ok {
		return "", nil
	}
	for _, l := range clone {
		link, ok := l.(map[string]any)
		if !ok {
			continue
		}
		href, ok := link["href"].(string)
		if !ok {
			continue
		}
		switch link["name"] {
		case "http":
			httpURL = href
			urls = append(urls, href)
		case "ssh":
			urls = append(urls, href)
		}
	}
	return httpURL, urls
}
