package commit

import (
	"context"
	"fmt"
	"os"
	"path"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/git"
)

type Service struct {
	gitCredsStore git.CredsStore
}

func NewService(gitCredsStore git.CredsStore) *Service {
	return &Service{gitCredsStore: gitCredsStore}
}

func (s *Service) Commit(ctx context.Context, r *apiclient.ManifestsRequest) (*apiclient.ManifestsResponse, error) {

	logCtx := log.WithFields(log.Fields{"repo": r.RepoUrl, "branch": r.TargetBranch, "drySHA": r.DrySha})

	logCtx.Debug("Creating temp dir")
	// The UUID is an important security mechanism to help mitigate path traversal attacks.
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

	gitCreds := r.Repo.GetGitCreds(s.gitCredsStore)
	gitClient, err := git.NewClientExt(r.RepoUrl, dirPath, gitCreds, r.Repo.IsInsecure(), r.Repo.IsLFSEnabled(), r.Repo.Proxy)
	if err != nil {
		logCtx.WithError(err).Error("failed to create git client")
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to create git client: %w", err)
	}

	err = gitClient.Init()
	if err != nil {
		logCtx.WithError(err).Error("failed to initialize git client")
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to init git client: %w", err)
	}

	// Clone the repo into the temp dir using the git CLI
	logCtx.Debugf("Cloning repo %s", r.RepoUrl)
	err = gitClient.Fetch("")
	if err != nil {
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to clone repo: %w", err)
	}

	authorName, authorEmail, err := gitCreds.GetUserInfo(ctx)
	if err != nil {
		logCtx.WithError(err).Error("failed to get github app info")
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to get github app info: %w", err)
	}

	out, err := gitClient.SetAuthor(authorName, authorEmail)
	if err != nil {
		logCtx.WithError(err).WithField("output", out).Error("failed to set author")
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to set author: %w", err)
	}

	// Checkout the sync branch
	logCtx.Debugf("Checking out sync branch %s", r.SyncBranch)
	out, err = gitClient.CheckoutOrOrphan(r.SyncBranch, false)
	if err != nil {
		logCtx.WithError(err).WithField("output", out).Error("failed to checkout sync branch")
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to checkout sync branch: %w", err)
	}

	// Checkout the target branch
	logCtx.Debugf("Checking out target branch %s", r.TargetBranch)
	out, err = gitClient.CheckoutOrNew(r.TargetBranch, r.SyncBranch, false)
	if err != nil {
		logCtx.WithError(err).WithField("output", out).Error("failed to checkout target branch")
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to checkout target branch: %w", err)
	}

	// Clear the repo contents using git rm
	logCtx.Debug("Clearing repo contents")
	out, err = gitClient.RemoveContents()
	if err != nil {
		logCtx.WithError(err).WithField("output", out).Error("failed to clear repo")
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
	out, err = gitClient.CommitAndPush(r.TargetBranch, r.CommitMessage)
	if err != nil {
		logCtx.WithError(err).WithField("output", out).Error("failed to commit and push")
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to commit and push: %w", err)
	}

	logCtx.WithField("output", out).Debug("pushed manifests to git")

	return &apiclient.ManifestsResponse{}, nil
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
