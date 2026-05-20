package generators

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v3/applicationset/services"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

var (
	_ Generator  = (*OciGenerator)(nil)
	_ repoSource = (*OciGenerator)(nil)
)

type OciGenerator struct {
	repos services.Repos
}

// NewOciGenerator creates a new instance of OCI Generator
func NewOciGenerator(repos services.Repos) Generator {
	g := &OciGenerator{
		repos: repos,
	}
	return g
}

// GetRequeueAfter is the generator can controller the next reconciled loop
// In case there is more then one generator the time will be the minimum of the times.
// In case NoRequeueAfter is empty, it will be ignored
func (o *OciGenerator) GetRequeueAfter(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) time.Duration {
	if appSetGenerator.Oci.RequeueAfterSeconds != nil {
		return time.Duration(*appSetGenerator.Oci.RequeueAfterSeconds) * time.Second
	}

	return getDefaultRequeueAfter()
}

// GetTemplate returns the inline template from the spec if there is any, or an empty object otherwise
func (o *OciGenerator) GetTemplate(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) *argoprojiov1alpha1.ApplicationSetTemplate {
	return &appSetGenerator.Oci.Template
}

// GenerateParams generates a list of parameter maps for the ApplicationSet by evaluating the OCI generator's configuration.
// It supports both directory-based and file-based OCI generators.
func (o *OciGenerator) GenerateParams(
	appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator,
	applicationSetInfo *argoprojiov1alpha1.ApplicationSet,
	client client.Client,
) (
	[]map[string]any,
	error,
) {
	if appSetGenerator == nil {
		return nil, ErrEmptyAppSetGenerator
	}

	if appSetGenerator.Oci == nil {
		return nil, ErrEmptyAppSetGenerator
	}

	dirs := make([]pathPattern, len(appSetGenerator.Oci.Directories))
	for i, d := range appSetGenerator.Oci.Directories {
		dirs[i] = pathPattern{Path: d.Path, Exclude: d.Exclude}
	}
	files := make([]pathPattern, len(appSetGenerator.Oci.Files))
	for i, f := range appSetGenerator.Oci.Files {
		files[i] = pathPattern{Path: f.Path, Exclude: f.Exclude}
	}

	spec := repoSourceSpec{
		URL:             appSetGenerator.Oci.RepoURL,
		Revision:        appSetGenerator.Oci.Revision,
		PathParamPrefix: appSetGenerator.Oci.PathParamPrefix,
		Values:          appSetGenerator.Oci.Values,
		Directories:     dirs,
		Files:           files,
	}

	return generateRepoSourceParams(o, repoSourceKindOCI, spec, applicationSetInfo, client)
}

// resolveSourceIntegrity returns nil; OCI does not yet support commit signing verification.
func (o *OciGenerator) resolveSourceIntegrity(_ context.Context, _ *argoprojiov1alpha1.ApplicationSet, _ client.Client) (*argoprojiov1alpha1.SourceIntegrity, error) {
	return nil, nil
}

func (o *OciGenerator) listDirectories(ctx context.Context, repoURL, revision, project string, noRevisionCache bool, _ *argoprojiov1alpha1.SourceIntegrity) ([]string, error) {
	return o.repos.GetOciDirectories(ctx, repoURL, revision, project, noRevisionCache)
}

func (o *OciGenerator) getFiles(ctx context.Context, repoURL, revision, project, pattern string, noRevisionCache bool, _ *argoprojiov1alpha1.SourceIntegrity) (map[string][]byte, error) {
	return o.repos.GetOciFiles(ctx, repoURL, revision, project, pattern, noRevisionCache)
}
