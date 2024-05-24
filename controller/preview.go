package controller

import (
	"context"
	"fmt"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	argodiff "github.com/argoproj/argo-cd/v2/util/argo/diff"
	"github.com/argoproj/argo-cd/v2/util/errors"
	logutils "github.com/argoproj/argo-cd/v2/util/log"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/google/go-github/v62/github"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"net/url"
	"strings"
	"time"
)

const ArgoCDGitHubUsername = "argocd"
const PreviewSleepDuration = 60 * time.Second

type Previewer struct {
	appLister       *applisters.ApplicationLister
	appStateManager *AppStateManager
	settingsManager *settings.SettingsManager
	getAppProject   func(app *v1alpha1.Application) (*v1alpha1.AppProject, error)
	ghClient        *github.Client
	ghContext       context.Context
	appLabelKey     string
	diffConfig      argodiff.DiffConfig
}

func NewPreviewer(
	appLister *applisters.ApplicationLister,
	appStateManager *AppStateManager,
	settingsManager *settings.SettingsManager,
	getAppProject func(app *v1alpha1.Application) (*v1alpha1.AppProject, error),
) (p *Previewer) {
	p = &Previewer{}
	p.appLister = appLister
	p.appStateManager = appStateManager
	p.settingsManager = settingsManager
	p.getAppProject = getAppProject
	p.ghClient = github.NewClient(nil)
	p.ghContext = context.Background()
	appLabelKey, err := p.settingsManager.GetAppInstanceLabelKey()
	errors.CheckError(err)
	p.appLabelKey = appLabelKey
	errors.CheckError(err)
	trackingMethod, err := p.settingsManager.GetTrackingMethod()
	errors.CheckError(err)
	p.diffConfig, err = argodiff.NewDiffConfigBuilder().
		WithDiffSettings(nil, nil, false).
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
		// Poll for new PR/PR Commit on listened to repos to dry manifest branch
		for repoURL, apps := range p.getRepoMap() {
			owner, repo := p.getOwnerRepo(repoURL)

			opts := &github.PullRequestListOptions{
				State: "open",
				Base:  apps[0].Spec.SourceHydrator.DrySource.TargetRevision,
			}

			pullRequests, _, _ := p.ghClient.PullRequests.List(p.ghContext, owner, repo, opts)
			for _, pr := range pullRequests {
				comment, found := p.getComment(owner, repo, pr)
				newComment := &github.IssueComment{
					// pr.Base is PR Target (branch that will receive changes)
					// pr.Head is PR Source (changes we want to integrate)
					Body: github.String(p.makeComment(apps, pr.Base.Ref, pr.Head.Ref)),
				}
				if found {
					// 4. do update
					_, _, err := p.ghClient.Issues.EditComment(p.ghContext, owner, repo, comment.GetID(), newComment)
					errors.CheckError(err)
				} else {
					// 4. create
					_, _, err := p.ghClient.Issues.CreateComment(p.ghContext, owner, repo, pr.GetNumber() /* PR Issue ID */, newComment)
					errors.CheckError(err)
				}
			}
		}
		time.Sleep(time.Duration(PreviewSleepDuration))
	}
}

func (p *Previewer) getRepoMap() map[string][]*v1alpha1.Application {
	// Get list of unique Repos from all Applications
	var repoMap = map[string][]*v1alpha1.Application{}

	apps, err := (*p.appLister).List(labels.Everything())
	if err != nil {
		panic(fmt.Errorf("error while fetching the apps list: %w", err))
	}
	for i := 0; i < len(apps); i++ {
		app := apps[i]
		repoURL := app.Spec.SourceHydrator.DrySource.RepoURL
		if repoMap[repoURL] == nil {
			repoMap[repoURL] = make([]*v1alpha1.Application, 0, 1)
		}
		repoMap[repoURL] = append(repoMap[repoURL], app)
	}
	return repoMap
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
	return parts[0], parts[1]
}

func (p *Previewer) getComment(owner string, repo string, pr *github.PullRequest) (*github.PullRequestComment, bool) {
	prComments, _, err := p.ghClient.PullRequests.ListComments(p.ghContext, owner, repo, pr.GetNumber(), nil)
	errors.CheckError(err)

	for i := 0; i < len(prComments); i++ {
		if prComments[i].GetAuthorAssociation() == ArgoCDGitHubUsername {
			return prComments[i], true
		}
	}
	return nil, false
}

func (p *Previewer) makeComment(apps []*v1alpha1.Application, baseBranch *string, headBranch *string) (commentBody string) {

	commentBody = fmt.Sprintf("\n===== from branch %s to branch %s ======\n", *headBranch, *baseBranch)
	for i := 0; i < len(apps); i++ {
		// Produce diff
		app := apps[i]
		project, err := p.getAppProject(app)
		errors.CheckError(err)

		baseUnstructured, err := p.getBranchManifest(app, project, baseBranch)
		errors.CheckError(err)

		headUnstructured, err := p.getBranchManifest(app, project, headBranch)
		errors.CheckError(err)

		commentBody += fmt.Sprintf("\n===== for target application %s ======\n", app.Name)

		appDiff, err := argodiff.StateDiffs(baseUnstructured, headUnstructured, p.diffConfig)
		errors.CheckError(err)

		diffBytes, err := yaml.Marshal(appDiff)
		errors.CheckError(err)

		commentBody += string(diffBytes)
	}
	return commentBody
}

// Get Hydrated Branch's manifest.yaml
func (p *Previewer) getBranchManifest(
	app *v1alpha1.Application,
	project *v1alpha1.AppProject,
	branch *string,
) (unstructured []*unstructured.Unstructured, err error) {

	unstructured, _, err = (*p.appStateManager).GetRepoObjs(
		app,
		app.Spec.Sources,
		p.appLabelKey,
		[]string{*branch},
		true,
		true,
		false,
		project,
	)
	return unstructured, err
}
