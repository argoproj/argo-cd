package commit

import (
	"context"
	"fmt"
	"github.com/argoproj/argo-cd/v2/applicationset/services/github_app_auth"
	"github.com/argoproj/argo-cd/v2/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"os"
	"path"
	"strings"
)

type Service struct {
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Commit(ctx context.Context, r *apiclient.ManifestsRequest) (*apiclient.ManifestsResponse, error) {
	var authorName, authorEmail, basicAuth string

	logCtx := log.WithFields(log.Fields{"repo": r.RepoUrl, "branch": r.TargetBranch, "drySHA": r.DrySha})

	if isGitHubApp(r.Repo) {
		var err error
		authorName, authorEmail, basicAuth, err = getGitHubAppInfo(ctx, r.Repo)
		if err != nil {
			logCtx.WithError(err).Error("failed to get github app info")
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to get github app info: %w", err)
		}
	} else {
		logCtx.Warn("No github app credentials were found")
	}

	logCtx.Debug("Creating temp dir")
	dirName, err := uuid.NewRandom()
	if err != nil {
		logCtx.WithError(err).Error("failed to generate uuid")
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to generate a uuid to create temp dir: %w", err)
	}
	// Don't need SecureJoin here, both parts are safe.
	dirPath := path.Join("/tmp/_commit-service", dirName.String())
	err = os.MkdirAll(dirPath, os.ModePerm)
	if err != nil {
		logCtx.WithError(err).Error("failed to create temp dir")
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() {
		err := os.RemoveAll(dirPath)
		if err != nil {
			logCtx.WithError(err).Errorf("failed to remove temp dir %s", dirPath)
		}
	}()

	git := newGitHelper(dirPath, r.SyncBranch, r.TargetBranch)

	// Clone the repo into the temp dir using the git CLI
	logCtx.Debugf("Cloning repo %s", r.RepoUrl)
	authRepoUrl := r.RepoUrl
	if basicAuth != "" && strings.HasPrefix(authRepoUrl, "https://github.com/") {
		authRepoUrl = fmt.Sprintf("https://%s@github.com/%s", basicAuth, strings.TrimPrefix(authRepoUrl, "https://github.com/"))
	}
	out, err := git.Clone(authRepoUrl)
	if err != nil {
		logCtx.WithError(err).WithField("output", string(out)).Error("failed to clone repo")
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to clone repo: %w", err)
	}

	if basicAuth != "" {
		// This is the dumbest kind of auth and should never make it in main branch
		// git config url."https://${TOKEN}@github.com/".insteadOf "https://github.com/"
		logCtx.Debugf("Setting auth")
		out, err = git.Config(fmt.Sprintf("url.\"https://%s@github.com/\".insteadOf", basicAuth), "https://github.com/")
		if err != nil {
			logCtx.WithError(err).WithField("output", string(out)).Error("failed to set auth")
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to set auth: %w", err)
		}
	}

	out, err = git.SetAuthor(authorName, authorEmail)
	if err != nil {
		logCtx.WithError(err).WithField("output", string(out)).Error("failed to set author")
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to set author: %w", err)
	}

	// Checkout the sync branch
	logCtx.Debugf("Checking out sync branch %s", r.SyncBranch)
	out, err = git.CheckoutSyncBranch()
	if err != nil {
		logCtx.WithError(err).WithField("output", string(out)).Error("failed to checkout sync branch")
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to checkout sync branch: %w", err)
	}

	// Checkout the target branch
	logCtx.Debugf("Checking out target branch %s", r.TargetBranch)
	out, err = git.CheckoutTargetBranch()
	if err != nil {
		logCtx.WithError(err).WithField("output", string(out)).Error("failed to checkout target branch")
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to checkout target branch: %w", err)
	}

	// Clear the repo contents using git rm
	logCtx.Debug("Clearing repo contents")
	out, err = git.RemoveContents()
	if err != nil {
		logCtx.WithError(err).WithField("output", string(out)).Error("failed to clear repo")
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to clear repo: %w", err)
	}

	h := newHydratorHelper(dirPath)

	// Write hydrator.metadata containing information about the hydration process. This top-level metadata file is used
	// for the promoter. An additional metadata file is placed in each hydration destination directory, if applicable.
	logCtx.Debug("Writing top-level hydrator metadata")
	err = h.WriteMetadata(hydratorMetadataFile{DrySHA: r.DrySha, RepoURL: r.RepoUrl}, "")
	if err != nil {
		logCtx.WithError(err).Error("failed to write top-level hydrator metadata")
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to write top-level hydrator metadata: %w", err)
	}

	// Write the manifests to the temp dir
	for _, p := range r.Paths {
		hydratePath := p.Path
		if hydratePath == "." {
			hydratePath = ""
		}
		logCtx.Debugf("Writing manifests to %s", hydratePath)
		fullHydratePath, err := securejoin.SecureJoin(dirPath, hydratePath)
		if err != nil {
			logCtx.WithError(err).Error("failed to construct hydrate path")
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to construct hydrate path: %w", err)
		}
		err = os.MkdirAll(fullHydratePath, os.ModePerm)
		if err != nil {
			logCtx.WithError(err).Error("failed to create path")
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to create path: %w", err)
		}

		// Write the manifests
		err = h.WriteManifests(p.Manifests, hydratePath)
		if err != nil {
			logCtx.WithError(err).Error("failed to write manifests")
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to write manifests: %w", err)
		}

		// Write hydrator.metadata containing information about the hydration process.
		logCtx.Debug("Writing hydrator metadata")
		hydratorMetadata := hydratorMetadataFile{
			Commands: p.Commands,
			DrySHA:   r.DrySha,
			RepoURL:  r.RepoUrl,
		}
		err = h.WriteMetadata(hydratorMetadata, hydratePath)
		if err != nil {
			logCtx.WithError(err).Error("failed to write hydrator metadata")
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to write hydrator metadata: %w", err)
		}

		// Write README
		logCtx.Debugf("Writing README")
		err = h.WriteReadme(hydratorMetadata, hydratePath)
		if err != nil {
			logCtx.WithError(err).Error("failed to write readme")
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to write readme: %w", err)
		}
	}

	// Commit the changes
	logCtx.Debugf("Committing and pushing changes")
	out, err = git.CommitAndPush(r.CommitMessage)
	if err != nil {
		logCtx.WithError(err).WithField("output", string(out)).Error("failed to commit and push")
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to commit and push: %w", err)
	}

	logCtx.WithField("output", string(out)).Debug("pushed manifests to git")

	return &apiclient.ManifestsResponse{}, nil
}

// getGitHubAppInfo retrieves the author name, author email, and basic auth header for a GitHub App.
func getGitHubAppInfo(ctx context.Context, repo *v1alpha1.Repository) (string, string, string, error) {
	info := github_app_auth.Authentication{
		Id:             repo.GithubAppId,
		InstallationId: repo.GithubAppInstallationId,
		PrivateKey:     repo.GithubAppPrivateKey,
	}
	appInstall, err := getAppInstallation(info)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get app installation: %w", err)
	}
	token, err := appInstall.Token(ctx)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get access token: %w", err)
	}
	client, err := getGitHubAppClient(info)
	if err != nil {
		return "", "", "", fmt.Errorf("cannot create github client: %w", err)
	}
	app, _, err := client.Apps.Get(ctx, "")
	if err != nil {
		return "", "", "", fmt.Errorf("cannot get app info: %w", err)
	}
	appLogin := fmt.Sprintf("%s[bot]", app.GetSlug())
	user, _, err := getGitHubInstallationClient(appInstall).Users.Get(ctx, appLogin)
	if err != nil {
		return "", "", "", fmt.Errorf("cannot get app user info: %w", err)
	}
	authorName := user.GetLogin()
	authorEmail := fmt.Sprintf("%d+%s@users.noreply.github.com", user.GetID(), user.GetLogin())
	basicAuth := fmt.Sprintf("x-access-token:%s", token)
	return authorName, authorEmail, basicAuth, nil
}

type hydratorMetadataFile struct {
	Commands []string `json:"commands"`
	RepoURL  string   `json:"repoURL"`
	DrySHA   string   `json:"drySha"`
}

var manifestHydrationReadmeTemplate = `
# Manifest Hydration

To hydrate the manifests in this repository, run the following commands:

` + "```shell\n" + `
git clone {{ .RepoURL }}
# cd into the cloned directory
git checkout {{ .DrySHA }}
{{ range $command := .Commands -}}
{{ $command }}
{{ end -}}` + "```"
