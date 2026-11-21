package repo

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"

	service "github.com/argoproj/argo-cd/v3/util/notification/argocd"

	"github.com/argoproj/argo-cd/v3/util/notification/expression/shared"

	"github.com/argoproj/notifications-engine/pkg/util/text"
	giturls "github.com/chainguard-dev/git-urls"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var gitSuffix = regexp.MustCompile(`\.git$`)

func getAppDetails(un *unstructured.Unstructured, argocdService service.Service) (*shared.AppDetail, error) {
	app, err := shared.GetApplication(un)
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
		"GetAppDetails": func() any {
			appDetails, err := getAppDetails(app, argocdService)
			if err != nil {
				panic(err)
			}

			return *appDetails
		},
	}
}
