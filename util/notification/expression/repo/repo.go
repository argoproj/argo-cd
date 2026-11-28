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

// FullNameByRepoURL returns the first path segments of a repository URL, with any .git suffix removed.
// By default it returns two segments (owner/repo), matching GitHub and Bitbucket Cloud. An optional
// segments argument changes that count; a value <= 0 returns every path segment (e.g. GitLab nested groups).
// HTTPS, SSH and scp-style URLs give the same result. It panics if rawURL cannot be parsed.
func FullNameByRepoURL(rawURL string, segments ...int) string {
	parsed, err := giturls.Parse(rawURL)
	if err != nil {
		panic(err)
	}

	n := 2
	if len(segments) > 0 {
		n = segments[0]
	}
	path := gitSuffix.ReplaceAllString(parsed.Path, "")
	pathParts := text.SplitRemoveEmpty(path, "/")
	if n <= 0 {
		return strings.Join(pathParts, "/")
	}
	if len(pathParts) >= n {
		return strings.Join(pathParts[:n], "/")
	}
	if len(segments) > 0 {
		return strings.Join(pathParts, "/")
	}

	// Legacy result for one-argument calls on short paths, kept unchanged for compatibility.
	return path
}

// repoURLToHTTPS rewrites a Git URL to HTTPS and drops its credentials,
// so git@github.com:owner/repo.git becomes
// https://github.com/owner/repo.git.
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
		"GetAppDetails": func() any {
			appDetails, err := getAppDetails(app, argocdService)
			if err != nil {
				panic(err)
			}

			return *appDetails
		},
	}
}
