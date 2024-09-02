package repo

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"

	service "github.com/argoproj/argo-cd/v2/util/notification/argocd"

	"github.com/argoproj/argo-cd/v2/util/notification/expression/shared"

	"github.com/argoproj/notifications-engine/pkg/util/text"
	giturls "github.com/chainguard-dev/git-urls"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
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

func NewExprs(argocdService service.Service, app *unstructured.Unstructured) map[string]interface{} {
	return map[string]interface{}{
		"RepoURLToHTTPS":    repoURLToHTTPS,
		"FullNameByRepoURL": FullNameByRepoURL,
		"QueryEscape":       url.QueryEscape,
		"GetCommitMetadata": func(commitSHA string) interface{} {
			meta, err := getCommitMetadata(commitSHA, app, argocdService)
			if err != nil {
				panic(err)
			}

			return *meta
		},
		"GetAppDetails": func() interface{} {
			appDetails, err := getAppDetails(app, argocdService)
			if err != nil {
				panic(err)
			}

			return *appDetails
		},
	}
}
