package generators

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"time"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v3/applicationset/services"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

var _ Generator = (*OciGenerator)(nil)

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
	_ client.Client,
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

	noRevisionCache := applicationSetInfo.RefreshRequired()

	project := resolveProjectName(applicationSetInfo.Spec.Template.Spec.Project)

	var err error
	var res []map[string]any
	switch {
	case len(appSetGenerator.Oci.Directories) != 0:
		res, err = o.generateParamsForOciDirectories(appSetGenerator, noRevisionCache, applicationSetInfo.Spec.GoTemplate, project, applicationSetInfo.Spec.GoTemplateOptions)
	case len(appSetGenerator.Oci.Files) != 0:
		res, err = o.generateParamsForOciFiles(appSetGenerator, noRevisionCache, applicationSetInfo.Spec.GoTemplate, project, applicationSetInfo.Spec.GoTemplateOptions)
	default:
		return nil, ErrEmptyAppSetGenerator
	}
	if err != nil {
		return nil, fmt.Errorf("error generating params from OCI: %w", err)
	}

	return res, nil
}

// generateParamsForOciDirectories generates parameters for an ApplicationSet using a directory-based OCI generator.
// It fetches all directories from the given OCI artifact and revision, optionally using a revision cache.
// It then filters the directories based on the generator's configuration and renders parameters for the resulting applications.
func (o *OciGenerator) generateParamsForOciDirectories(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, noRevisionCache, useGoTemplate bool, project string, goTemplateOptions []string) ([]map[string]any, error) {
	allPaths, err := o.repos.GetOciDirectories(context.TODO(), appSetGenerator.Oci.RepoURL, appSetGenerator.Oci.Revision, project, noRevisionCache)
	if err != nil {
		return nil, fmt.Errorf("error getting directories from OCI artifact: %w", err)
	}

	log.WithFields(log.Fields{
		"allPaths":        allPaths,
		"total":           len(allPaths),
		"repoURL":         appSetGenerator.Oci.RepoURL,
		"revision":        appSetGenerator.Oci.Revision,
		"pathParamPrefix": appSetGenerator.Oci.PathParamPrefix,
	}).Info("applications result from the OCI repo service")

	requestedApps := filterOciPaths(appSetGenerator.Oci.Directories, allPaths)

	res, err := generateParamsFromPaths(requestedApps, appSetGenerator.Oci.PathParamPrefix, appSetGenerator.Oci.Values, useGoTemplate, goTemplateOptions)
	if err != nil {
		return nil, fmt.Errorf("error generating params from apps: %w", err)
	}

	return res, nil
}

// generateParamsForOciFiles generates parameters for an ApplicationSet using a file-based OCI generator.
// It retrieves and processes specified files from the OCI artifact, supporting both YAML and JSON formats,
// and returns a list of parameter maps extracted from the content.
func (o *OciGenerator) generateParamsForOciFiles(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, noRevisionCache, useGoTemplate bool, project string, goTemplateOptions []string) ([]map[string]any, error) {
	fileContentMap := make(map[string][]byte)
	var includePatterns []string
	var excludePatterns []string

	for _, req := range appSetGenerator.Oci.Files {
		if req.Exclude {
			excludePatterns = append(excludePatterns, req.Path)
		} else {
			includePatterns = append(includePatterns, req.Path)
		}
	}

	for _, includePattern := range includePatterns {
		retrievedFiles, err := o.repos.GetOciFiles(
			context.TODO(),
			appSetGenerator.Oci.RepoURL,
			appSetGenerator.Oci.Revision,
			project,
			includePattern,
			noRevisionCache,
		)
		if err != nil {
			return nil, err
		}
		maps.Copy(fileContentMap, retrievedFiles)
	}

	for _, excludePattern := range excludePatterns {
		matchingFiles, err := o.repos.GetOciFiles(
			context.TODO(),
			appSetGenerator.Oci.RepoURL,
			appSetGenerator.Oci.Revision,
			project,
			excludePattern,
			noRevisionCache,
		)
		if err != nil {
			return nil, err
		}
		for absPath := range matchingFiles {
			delete(fileContentMap, absPath)
		}
	}

	var filePaths []string
	for path := range fileContentMap {
		filePaths = append(filePaths, path)
	}
	sort.Strings(filePaths)

	var allParams []map[string]any
	for _, filePath := range filePaths {
		paramsFromFileArray, err := parseFileParams(filePath, fileContentMap[filePath], appSetGenerator.Oci.PathParamPrefix, appSetGenerator.Oci.Values, useGoTemplate, goTemplateOptions)
		if err != nil {
			return nil, fmt.Errorf("unable to process file '%s': %w", filePath, err)
		}
		allParams = append(allParams, paramsFromFileArray...)
	}

	return allParams, nil
}
