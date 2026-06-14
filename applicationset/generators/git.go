package generators

import (
	"context"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v3/applicationset/services"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// Compile-time assertions that GitGenerator satisfies two interfaces:
//   - Generator: the public interface used by the ApplicationSet controller.
//   - repoSource: the internal interface used by the shared directory/file traversal.
var (
	_ Generator  = (*GitGenerator)(nil)
	_ repoSource = (*GitGenerator)(nil)
)

type GitGenerator struct {
	repos     services.Repos
	namespace string
}

// NewGitGenerator creates a new instance of Git Generator
func NewGitGenerator(repos services.Repos, controllerNamespace string) Generator {
	g := &GitGenerator{
		repos:     repos,
		namespace: controllerNamespace,
	}

	return g
}

// GetTemplate returns the ApplicationSetTemplate associated with the Git generator
// from the provided ApplicationSetGenerator. This template defines how each
// generated Argo CD Application should be rendered.
func (g *GitGenerator) GetTemplate(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) *argoprojiov1alpha1.ApplicationSetTemplate {
	return &appSetGenerator.Git.Template
}

// GetRequeueAfter returns the duration after which the Git generator should be
// requeued for reconciliation. If RequeueAfterSeconds is set in the generator spec,
// it uses that value. Otherwise, it falls back to a default requeue interval (3 minutes).
func (g *GitGenerator) GetRequeueAfter(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) time.Duration {
	if appSetGenerator.Git.RequeueAfterSeconds != nil {
		return time.Duration(*appSetGenerator.Git.RequeueAfterSeconds) * time.Second
	}

	return getDefaultRequeueAfter()
}

// GenerateParams generates a list of parameter maps for the ApplicationSet by evaluating the Git generator's configuration.
// It supports both directory-based and file-based Git generators.
func (g *GitGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, appSet *argoprojiov1alpha1.ApplicationSet, client client.Client) ([]map[string]any, error) {
	if appSetGenerator == nil {
		return nil, ErrEmptyAppSetGenerator
	}

	if appSetGenerator.Git == nil {
		return nil, ErrEmptyAppSetGenerator
	}

	dirs := make([]pathPattern, len(appSetGenerator.Git.Directories))
	for i, d := range appSetGenerator.Git.Directories {
		dirs[i] = pathPattern{Path: d.Path, Exclude: d.Exclude}
	}
	files := make([]pathPattern, len(appSetGenerator.Git.Files))
	for i, f := range appSetGenerator.Git.Files {
		files[i] = pathPattern{Path: f.Path, Exclude: f.Exclude}
	}

	spec := repoSourceSpec{
		URL:             appSetGenerator.Git.RepoURL,
		Revision:        appSetGenerator.Git.Revision,
		PathParamPrefix: appSetGenerator.Git.PathParamPrefix,
		Values:          appSetGenerator.Git.Values,
		Directories:     dirs,
		Files:           files,
	}

	// TODO: propagate a real context once Generator.GenerateParams accepts ctx.
	return generateRepoSourceParams(context.TODO(), g, repoSourceKindGit, spec, appSet, client)
}

// resolveSourceIntegrity returns the SourceIntegrity policy from the associated AppProject.
// Returns nil when the project name is templated, since the project can only be resolved after generation.
func (g *GitGenerator) resolveSourceIntegrity(ctx context.Context, appSet *argoprojiov1alpha1.ApplicationSet, client client.Client) (*argoprojiov1alpha1.SourceIntegrity, error) {
	// Skip verification when the project is templated: the generator must run to resolve the name,
	// but running without verification is the required trade-off in that case.
	if strings.Contains(appSet.Spec.Template.Spec.Project, "{{") {
		log.WithField("appset", appSet.Name).Infof("Cannot enforce eventual source integrity, app project name is templated")
		return nil, nil
	}

	project := appSet.Spec.Template.Spec.Project
	appProject := &argoprojiov1alpha1.AppProject{}
	controllerNamespace := g.namespace
	if controllerNamespace == "" {
		controllerNamespace = appSet.Namespace
	}
	if err := client.Get(ctx, types.NamespacedName{Name: project, Namespace: controllerNamespace}, appProject); err != nil {
		return nil, fmt.Errorf("error getting project %s: %w", project, err)
	}
	return appProject.EffectiveSourceIntegrity(), nil
}

func (g *GitGenerator) listDirectories(ctx context.Context, repoURL, revision, project string, noRevisionCache bool, sourceIntegrity *argoprojiov1alpha1.SourceIntegrity) ([]string, error) {
	return g.repos.GetDirectories(ctx, repoURL, revision, project, noRevisionCache, sourceIntegrity)
}

func (g *GitGenerator) getFiles(ctx context.Context, repoURL, revision, project, pattern string, noRevisionCache bool, sourceIntegrity *argoprojiov1alpha1.SourceIntegrity) (map[string][]byte, error) {
	return g.repos.GetFiles(ctx, repoURL, revision, project, pattern, noRevisionCache, sourceIntegrity)
}
