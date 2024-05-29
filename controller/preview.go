package controller

import (
	"context"
	"fmt"
	"github.com/argoproj/argo-cd/v2/applicationset/services/github_app_auth"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	argodiff "github.com/argoproj/argo-cd/v2/util/argo/diff"
	"github.com/argoproj/argo-cd/v2/util/argo/normalizers"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/errors"
	logutils "github.com/argoproj/argo-cd/v2/util/log"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v62/github"
	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
	"time"
)

const ArgoCDGitHubUsername = "gitops-promoter[bot]"
const PreviewSleepDuration = 10 * time.Second

type Previewer struct {
	appLister       *applisters.ApplicationLister
	appStateManager *AppStateManager
	settingsManager *settings.SettingsManager
	getAppProject   func(app *v1alpha1.Application) (*v1alpha1.AppProject, error)
	ghContext       context.Context
	appLabelKey     string
	diffConfig      argodiff.DiffConfig
	db              db.ArgoDB
}

func NewPreviewer(
	appLister *applisters.ApplicationLister,
	appStateManager *AppStateManager,
	settingsManager *settings.SettingsManager,
	getAppProject func(app *v1alpha1.Application) (*v1alpha1.AppProject, error),
	db db.ArgoDB,
) (p *Previewer) {
	p = &Previewer{}
	p.appLister = appLister
	p.appStateManager = appStateManager
	p.settingsManager = settingsManager
	p.getAppProject = getAppProject
	p.db = db

	p.ghContext = context.Background()
	appLabelKey, err := p.settingsManager.GetAppInstanceLabelKey()
	errors.CheckError(err)
	p.appLabelKey = appLabelKey
	errors.CheckError(err)
	trackingMethod, err := p.settingsManager.GetTrackingMethod()
	errors.CheckError(err)
	p.diffConfig, err = argodiff.NewDiffConfigBuilder().
		WithDiffSettings(nil, nil, false, normalizers.IgnoreNormalizerOpts{}).
		WithTracking(p.appLabelKey, trackingMethod).
		WithNoCache().
		WithLogger(logutils.NewLogrusLogger(logutils.NewWithCurrentConfig())).
		Build()
	errors.CheckError(err)
	return p
}

// Run is the main loop for the preview controller
func (p *Previewer) Run() {
	for {
		repoMap, err := p.getRepoMap()
		if err != nil {
			log.Errorf("failed to get repo map: %v", err)
		}
		// Poll for new PR/PR Commit on listened to repos to dry manifest branch
		for repoURL, apps := range repoMap {
			owner, repoName := p.getOwnerRepo(repoURL)

			ghClient, err := p.getClient(repoURL)
			if err != nil {
				log.Errorf("failed to get GitHub client: %v", err)
				continue
			}

			baseRevision := apps[0].Spec.SourceHydrator.DrySource.TargetRevision
			if baseRevision == "HEAD" {
				repo, _, err := ghClient.Repositories.Get(p.ghContext, owner, repoName)
				if err != nil {
					log.Errorf("failed to get repo %s/%s: %v", owner, repoName, err)
					continue
				}
				baseRevision = repo.GetDefaultBranch()
			}

			opts := &github.PullRequestListOptions{
				State: "open",
				Base:  baseRevision,
			}

			pullRequests, _, err := ghClient.PullRequests.List(p.ghContext, owner, repoName, opts)
			if err != nil {
				log.Errorf("failed to get PRs: %v", err)
				continue
			}
			for _, pr := range pullRequests {
				comment, err := p.getComment(owner, repoName, pr)
				if err != nil {
					log.Errorf("failed to get comment: %v", err)
					continue
				}
				commentBody, err := p.makeComment(apps, pr.Base.GetRef(), pr.Head.GetRef())
				if err != nil {
					log.Errorf("failed to make comment: %v", err)
					continue
				}
				newComment := &github.IssueComment{
					// pr.Base is PR Target (branch that will receive changes)
					// pr.Head is PR Source (changes we want to integrate)
					Body: github.String(commentBody),
				}
				if comment != nil {
					if comment.GetBody() == newComment.GetBody() {
						continue
					}
					_, _, err := ghClient.Issues.EditComment(p.ghContext, owner, repoName, comment.GetID(), newComment)
					if err != nil {
						log.Errorf("failed to edit comment %d: %v", comment.GetID(), err)
					}
				} else {
					// 4. create
					_, _, err := ghClient.Issues.CreateComment(p.ghContext, owner, repoName, pr.GetNumber(), newComment)
					if err != nil {
						log.Errorf("failed to create comment: %v", err)
					}
				}
			}
		}
		time.Sleep(PreviewSleepDuration)
	}
}

func (p *Previewer) getClient(repoURL string) (*github.Client, error) {
	repo, err := p.db.GetHydratorCredentials(context.Background(), repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo %s: %w", repoURL, err)
	}
	if !isGitHubApp(repo) {
		panic("Only GitHub App credentials are supported")
	}
	info := github_app_auth.Authentication{
		Id:             repo.GithubAppId,
		InstallationId: repo.GithubAppInstallationId,
		PrivateKey:     repo.GithubAppPrivateKey,
	}
	client, err := getGitHubAppClient(info)
	if err != nil {
		return nil, fmt.Errorf("failed to get GitHub App client: %w", err)
	}
	return client, nil
}

func isGitHubApp(cred *v1alpha1.Repository) bool {
	return cred.GithubAppPrivateKey != "" && cred.GithubAppId != 0 && cred.GithubAppInstallationId != 0
}

func getGitHubAppClient(g github_app_auth.Authentication) (*github.Client, error) {
	// This creates the app authenticated with the bearer JWT, not the installation token.
	rt, err := ghinstallation.New(http.DefaultTransport, g.Id, g.InstallationId, []byte(g.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create github app transport: %w", err)
	}

	httpClient := http.Client{Transport: rt}
	client := github.NewClient(&httpClient)
	return client, nil
}

func (p *Previewer) getRepoMap() (map[string][]*v1alpha1.Application, error) {
	// Get list of unique Repos from all Applications
	var repoMap = map[string][]*v1alpha1.Application{}

	apps, err := (*p.appLister).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list applications: %w", err)
	}
	for i := 0; i < len(apps); i++ {
		if apps[i].Spec.SourceHydrator == nil {
			continue
		}

		app := apps[i]
		repoURL := app.Spec.SourceHydrator.DrySource.RepoURL
		if repoMap[repoURL] == nil {
			repoMap[repoURL] = make([]*v1alpha1.Application, 0, 1)
		}
		repoMap[repoURL] = append(repoMap[repoURL], app)
	}
	return repoMap, nil
}

func (p *Previewer) getOwnerRepo(repoUrl string) (string, string) {
	u, err := url.Parse(repoUrl)
	if err != nil {
		panic(err)
	}
	parts := strings.Split(u.Path, "/")
	if len(parts) < 2 {
		panic("incorrect Git URL")
	}
	return parts[1], parts[2]
}

func (p *Previewer) getComment(owner string, repo string, pr *github.PullRequest) (*github.IssueComment, error) {
	ghClient, err := p.getClient(fmt.Sprintf("https://github.com/%s/%s", owner, repo))
	if err != nil {
		return nil, fmt.Errorf("failed to get GitHub client: %w", err)
	}

	prComments, resp, err := ghClient.Issues.ListComments(p.ghContext, owner, repo, pr.GetNumber(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR comments: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get PR comments: %d", resp.StatusCode)
	}

	for i := 0; i < len(prComments); i++ {
		if prComments[i].GetUser().GetLogin() == ArgoCDGitHubUsername {
			return prComments[i], nil
		}
	}
	return nil, nil
}

func (p *Previewer) makeComment(apps []*v1alpha1.Application, baseBranch string, headBranch string) (string, error) {

	commentBody := fmt.Sprintf("\n## From branch %s to branch %s\n", headBranch, baseBranch)

	// Sort the apps by name.
	// This is to ensure that the diff is consistent across runs.
	sort.Slice(apps, func(i, j int) bool {
		return apps[i].Name < apps[j].Name
	})

	for i := 0; i < len(apps); i++ {
		// Produce diff
		app := apps[i]
		project, err := p.getAppProject(app)
		if err != nil {
			return "", fmt.Errorf("failed to get app project: %w", err)
		}

		baseUnstructured, err := p.getBranchManifest(app, project, baseBranch)
		if err != nil {
			return "", fmt.Errorf("failed to get base branch manifest: %w", err)
		}

		headUnstructured, err := p.getBranchManifest(app, project, headBranch)
		if err != nil {
			return "", fmt.Errorf("failed to get head branch manifest: %w", err)
		}

		commentBody += fmt.Sprintf("\n### for target application %s\n", app.Name)

		tempDir, err := os.MkdirTemp("", "argocd-diff")
		if err != nil {
			return "", fmt.Errorf("failed to create temp dir: %w", err)
		}
		targetFile := path.Join(tempDir, "target.yaml")
		targetData := []byte("")
		if baseUnstructured != nil {
			targetData, err = yaml.Marshal(baseUnstructured)
			if err != nil {
				return "", fmt.Errorf("failed to marshal base unstructured: %w", err)
			}
		}
		err = os.WriteFile(targetFile, targetData, 0644)
		if err != nil {
			return "", fmt.Errorf("failed to write target file: %w", err)
		}
		liveFile := path.Join(tempDir, "base.yaml")
		liveData := []byte("")
		if headUnstructured != nil {
			liveData, err = yaml.Marshal(headUnstructured)
			if err != nil {
				return "", fmt.Errorf("failed to marshal head unstructured: %w", err)
			}
		}
		err = os.WriteFile(liveFile, liveData, 0644)
		if err != nil {
			return "", fmt.Errorf("failed to write live file: %w", err)
		}
		cmdBinary := "diff"
		var args []string
		if envDiff := os.Getenv("KUBECTL_EXTERNAL_DIFF"); envDiff != "" {
			parts, err := shlex.Split(envDiff)
			if err != nil {
				return "", fmt.Errorf("failed to split env diff: %w", err)
			}
			cmdBinary = parts[0]
			args = parts[1:]
		}
		cmd := exec.Command(cmdBinary, append(args, liveFile, targetFile)...)
		out, _ := cmd.CombinedOutput()

		commentBody += "```diff\n" + string(out) + "```\n"
	}
	return commentBody, nil
}

// Get Hydrated Branch's manifest.yaml
func (p *Previewer) getBranchManifest(
	app *v1alpha1.Application,
	project *v1alpha1.AppProject,
	branch string,
) (unstructured []*unstructured.Unstructured, err error) {

	unstructured, _, err = (*p.appStateManager).GetRepoObjs(
		app,
		[]v1alpha1.ApplicationSource{{
			RepoURL:        app.Spec.SourceHydrator.DrySource.RepoURL,
			Path:           app.Spec.SourceHydrator.DrySource.Path,
			TargetRevision: branch,
		}},
		p.appLabelKey,
		[]string{branch},
		false,
		true, // disable revision cache since we're using branch names instead of SHAs
		false,
		project,
		false,
	)
	return unstructured, err
}
