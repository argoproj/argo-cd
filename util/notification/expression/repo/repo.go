package repo

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"

	service "github.com/argoproj/argo-cd/v3/util/notification/argocd"

	"github.com/argoproj/argo-cd/v3/util/notification/expression/shared"

	"github.com/argoproj/notifications-engine/pkg/util/text"
	giturls "github.com/chainguard-dev/git-urls"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

var gitSuffix = regexp.MustCompile(`\.git$`)

func getApplication(obj *unstructured.Unstructured) (*v1alpha1.Application, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	application := &v1alpha1.Application{}
	err = json.Unmarshal(data, application)
	if err != nil {
		return nil, err
	}
	return application, nil
}

func getAppDetails(un *unstructured.Unstructured, argocdService service.Service) (*shared.AppDetail, error) {
	app, err := getApplication(un)
	if err != nil {
		return nil, err
	}
	appDetail, err := argocdService.GetAppDetails(context.Background(), app)
	if err != nil {
		return nil, err
	}
	return appDetail, nil
}

func getCommitMetadata(commitSHA string, app *unstructured.Unstructured, argocdService service.Service) (*shared.CommitMetadata, error) {
	repoURL, ok, err := unstructured.NestedString(app.Object, "spec", "source", "repoURL")
	if err != nil {
		return nil, err
	}
	if !ok {
		panic(errors.New("failed to get application source repo URL"))
	}
	project, ok, err := unstructured.NestedString(app.Object, "spec", "project")
	if err != nil {
		return nil, err
	}
	if !ok {
		panic(errors.New("failed to get application project"))
	}

	meta, err := argocdService.GetCommitMetadata(context.Background(), repoURL, commitSHA, project)
	if err != nil {
		return nil, err
	}
	return meta, nil
}

func getCommitAuthorsBetween(fromRevision string, toRevision string, app *unstructured.Unstructured, argocdService service.Service) ([]string, error) {
	// Validate inputs
	if fromRevision == "" || toRevision == "" {
		return []string{}, nil
	}
	if fromRevision == toRevision {
		return []string{}, nil
	}
	
	repoURL, ok, err := unstructured.NestedString(app.Object, "spec", "source", "repoURL")
	if err != nil {
		return []string{}, nil
	}
	if !ok || repoURL == "" {
		return []string{}, nil
	}
	project, ok, err := unstructured.NestedString(app.Object, "spec", "project")
	if err != nil {
		return []string{}, nil
	}
	if !ok {
		// Default project if not specified
		project = "default"
	}

	authors, err := argocdService.GetCommitAuthorsBetween(context.Background(), repoURL, fromRevision, toRevision, project)
	if err != nil {
		// Return empty on error to avoid breaking notifications
		return []string{}, nil
	}
	return authors, nil
}

// getCommitAuthorsFromPreviousSync gets commit authors between the previous sync and current sync
// Returns empty slice if there's no history, no current revision, or if revisions are the same
func getCommitAuthorsFromPreviousSync(app *unstructured.Unstructured, argocdService service.Service) ([]string, error) {
	// Get current revision(s)
	currentRevisions, ok, err := unstructured.NestedStringSlice(app.Object, "status", "sync", "revisions")
	if err != nil {
		// Log error but return empty to avoid breaking notifications
		return []string{}, nil
	}
	var currentRevision string
	if !ok || len(currentRevisions) == 0 {
		// Try single revision field
		var ok2 bool
		currentRevision, ok2, err = unstructured.NestedString(app.Object, "status", "sync", "revision")
		if err != nil {
			return []string{}, nil
		}
		if !ok2 || currentRevision == "" {
			// No current revision, return empty
			return []string{}, nil
		}
	} else {
		// For multisource, use the first source's revision
		currentRevision = currentRevisions[0]
		if currentRevision == "" {
			return []string{}, nil
		}
	}

	// Get previous revision from history
	var previousRevision string
	history, ok, err := unstructured.NestedSlice(app.Object, "status", "history")
	if err != nil {
		return []string{}, nil
	}
	if ok && len(history) > 0 {
		// Get the last history entry (most recent previous sync)
		lastHistory := history[len(history)-1]
		lastHistoryMap, ok := lastHistory.(map[string]interface{})
		if ok {
			// Try revisions field first (for multisource)
			if revs, exists := lastHistoryMap["revisions"]; exists {
				if revsSlice, ok := revs.([]interface{}); ok && len(revsSlice) > 0 {
					if revStr, ok := revsSlice[0].(string); ok && revStr != "" {
						previousRevision = revStr
					}
				}
			} else if rev, exists := lastHistoryMap["revision"]; exists {
				// Fall back to single revision field
				if revStr, ok := rev.(string); ok && revStr != "" {
					previousRevision = revStr
				}
			}
		}
	}

	// If no previous revision, return empty (first sync or no history)
	if previousRevision == "" {
		return []string{}, nil
	}

	// If previous and current are the same, return empty (no new commits)
	if previousRevision == currentRevision {
		return []string{}, nil
	}

	// Get authors between revisions
	// Errors are handled gracefully - if there's an issue, return empty
	authors, err := getCommitAuthorsBetween(previousRevision, currentRevision, app, argocdService)
	if err != nil {
		// Log error but return empty to avoid breaking notifications
		return []string{}, nil
	}
	return authors, nil
}

// extractEmailFromAuthor extracts email from "Name <email>" format
// Handles edge cases like missing brackets, empty strings, etc.
func extractEmailFromAuthor(author string) string {
	if author == "" {
		return ""
	}
	author = strings.TrimSpace(author)
	
	// Format is typically "Name <email>" or just "email"
	start := strings.Index(author, "<")
	end := strings.Index(author, ">")
	if start != -1 && end != -1 && end > start {
		email := strings.TrimSpace(author[start+1 : end])
		if email != "" {
			return email
		}
	}
	// If no angle brackets, check if it looks like an email
	// Otherwise return the whole string trimmed
	trimmed := strings.TrimSpace(author)
	if strings.Contains(trimmed, "@") {
		return trimmed
	}
	// If it doesn't look like an email, return empty
	return ""
}

// formatSlackMentions formats a list of authors as Slack mentions
// This is a helper that extracts emails - users will need to map emails to Slack user IDs
// Returns comma-separated list of emails
func formatSlackMentions(authors []string) string {
	if len(authors) == 0 {
		return ""
	}
	mentions := make([]string, 0, len(authors))
	seen := make(map[string]bool)
	for _, author := range authors {
		email := extractEmailFromAuthor(author)
		if email != "" && !seen[email] {
			mentions = append(mentions, email)
			seen[email] = true
		}
	}
	if len(mentions) == 0 {
		return ""
	}
	return strings.Join(mentions, ", ")
}

func FullNameByRepoURL(rawURL string) string {
	parsed, err := giturls.Parse(rawURL)
	if err != nil {
		panic(err)
	}

	path := gitSuffix.ReplaceAllString(parsed.Path, "")
	if pathParts := text.SplitRemoveEmpty(path, "/"); len(pathParts) >= 2 {
		return strings.Join(pathParts[:2], "/")
	}

	return path
}

func repoURLToHTTPS(rawURL string) string {
	parsed, err := giturls.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	parsed.Scheme = "https"
	parsed.User = nil
	return parsed.String()
}

func NewExprs(argocdService service.Service, app *unstructured.Unstructured) map[string]any {
	return map[string]any{
		"RepoURLToHTTPS":    repoURLToHTTPS,
		"FullNameByRepoURL": FullNameByRepoURL,
		"QueryEscape":       url.QueryEscape,
		"GetCommitMetadata": func(commitSHA string) any {
			meta, err := getCommitMetadata(commitSHA, app, argocdService)
			if err != nil {
				panic(err)
			}

			return *meta
		},
		"GetCommitAuthorsBetween": func(fromRevision string, toRevision string) any {
			authors, err := getCommitAuthorsBetween(fromRevision, toRevision, app, argocdService)
			if err != nil {
				panic(err)
			}

			return authors
		},
		"GetCommitAuthorsFromPreviousSync": func() any {
			authors, err := getCommitAuthorsFromPreviousSync(app, argocdService)
			if err != nil {
				panic(err)
			}

			return authors
		},
		"ExtractEmailFromAuthor": extractEmailFromAuthor,
		"FormatSlackMentions":    formatSlackMentions,
		"GetAppDetails": func() any {
			appDetails, err := getAppDetails(app, argocdService)
			if err != nil {
				panic(err)
			}

			return *appDetails
		},
	}
}
