package generators

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v3/applicationset/services"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/gpg"
)

var _ Generator = (*GitGenerator)(nil)

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

	noRevisionCache := appSet.RefreshRequired()

	verifyCommit := false

	// When the project field is templated, the contents of the git repo are required to run the git generator and get the templated value,
	// but git generator cannot be called without verifying the commit signature.
	// In this case, we skip the signature verification.
	// If the project is templated, we skip the commit verification
	if !strings.Contains(appSet.Spec.Template.Spec.Project, "{{") {
		project := appSet.Spec.Template.Spec.Project
		appProject := &argoprojiov1alpha1.AppProject{}
		controllerNamespace := g.namespace
		if controllerNamespace == "" {
			controllerNamespace = appSet.Namespace
		}
		if err := client.Get(context.TODO(), types.NamespacedName{Name: project, Namespace: controllerNamespace}, appProject); err != nil {
			return nil, fmt.Errorf("error getting project %s: %w", project, err)
		}
		// we need to verify the signature on the Git revision if GPG is enabled
		verifyCommit = len(appProject.Spec.SignatureKeys) > 0 && gpg.IsGPGEnabled()
	}

	// If the project field is templated, we cannot resolve the project name, so we pass an empty string to the repo-server.
	// This means only "globally-scoped" repo credentials can be used for such appsets.
	project := resolveProjectName(appSet.Spec.Template.Spec.Project)

	var err error
	var res []map[string]any
	switch {
	case len(appSetGenerator.Git.Directories) != 0:
		res, err = g.generateParamsForGitDirectories(appSetGenerator, noRevisionCache, verifyCommit, appSet.Spec.GoTemplate, project, appSet.Spec.GoTemplateOptions)
	case len(appSetGenerator.Git.Files) != 0:
		res, err = g.generateParamsForGitFiles(appSetGenerator, noRevisionCache, verifyCommit, appSet.Spec.GoTemplate, project, appSet.Spec.GoTemplateOptions)
	default:
		return nil, ErrEmptyAppSetGenerator
	}
	if err != nil {
		return nil, fmt.Errorf("error generating params from git: %w", err)
	}

	return res, nil
}

// generateParamsForGitDirectories generates parameters for an ApplicationSet using a directory-based Git generator.
// It fetches all directories from the given Git repository and revision, optionally using a revision cache and verifying commits.
// It then filters the directories based on the generator's configuration and renders parameters for the resulting applications
func (g *GitGenerator) generateParamsForGitDirectories(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, noRevisionCache, verifyCommit, useGoTemplate bool, project string, goTemplateOptions []string) ([]map[string]any, error) {
	allPaths, err := g.repos.GetDirectories(context.TODO(), appSetGenerator.Git.RepoURL, appSetGenerator.Git.Revision, project, noRevisionCache, verifyCommit)
	if err != nil {
		return nil, fmt.Errorf("error getting directories from repo: %w", err)
	}

	log.WithFields(log.Fields{
		"allPaths":        allPaths,
		"total":           len(allPaths),
		"repoURL":         appSetGenerator.Git.RepoURL,
		"revision":        appSetGenerator.Git.Revision,
		"pathParamPrefix": appSetGenerator.Git.PathParamPrefix,
	}).Info("applications result from the repo service")

	requestedApps := filterGitPaths(appSetGenerator.Git.Directories, allPaths)

	res, err := generateParamsFromPaths(requestedApps, appSetGenerator.Git.PathParamPrefix, appSetGenerator.Git.Values, useGoTemplate, goTemplateOptions)
	if err != nil {
		return nil, fmt.Errorf("error generating params from apps: %w", err)
	}

	return res, nil
}

// generateParamsForGitFiles generates parameters for an ApplicationSet using a file-based Git generator.
// It retrieves and processes specified files from the Git repository, supporting both YAML and JSON formats,
// and returns a list of parameter maps extracted from the content.
func (g *GitGenerator) generateParamsForGitFiles(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, noRevisionCache, verifyCommit, useGoTemplate bool, project string, goTemplateOptions []string) ([]map[string]any, error) {
	// fileContentMap maps absolute file paths to their byte content
	fileContentMap := make(map[string][]byte)
	var includePatterns []string
	var excludePatterns []string

	for _, req := range appSetGenerator.Git.Files {
		if req.Exclude {
			excludePatterns = append(excludePatterns, req.Path)
		} else {
			includePatterns = append(includePatterns, req.Path)
		}
	}

	// Fetch all files from include patterns
	for _, includePattern := range includePatterns {
		retrievedFiles, err := g.repos.GetFiles(
			context.TODO(),
			appSetGenerator.Git.RepoURL,
			appSetGenerator.Git.Revision,
			project,
			includePattern,
			noRevisionCache,
			verifyCommit,
		)
		if err != nil {
			return nil, err
		}
		maps.Copy(fileContentMap, retrievedFiles)
	}

	// Now remove files matching any exclude pattern
	for _, excludePattern := range excludePatterns {
		matchingFiles, err := g.repos.GetFiles(
			context.TODO(),
			appSetGenerator.Git.RepoURL,
			appSetGenerator.Git.Revision,
			project,
			excludePattern,
			noRevisionCache,
			verifyCommit,
		)
		if err != nil {
			return nil, err
		}
		for absPath := range matchingFiles {
			// if the file doesn't exist already and you try to delete it from the map
			// the operation is a no-op. It’s safe and doesn't return an error or panic.
			// Hence, we can simply try to delete the file from the path without checking
			// if that file already exists in the map.
			delete(fileContentMap, absPath)
		}
	}

	// Get a sorted list of file paths to ensure deterministic processing order
	var filePaths []string
	for path := range fileContentMap {
		filePaths = append(filePaths, path)
	}
	sort.Strings(filePaths)

	var allParams []map[string]any
	for _, filePath := range filePaths {
		// A JSON / YAML file path can contain multiple sets of parameters (ie it is an array)
		paramsFromFileArray, err := parseFileParams(filePath, fileContentMap[filePath], appSetGenerator.Git.PathParamPrefix, appSetGenerator.Git.Values, useGoTemplate, goTemplateOptions)
		if err != nil {
			return nil, fmt.Errorf("unable to process file '%s': %w", filePath, err)
		}
		allParams = append(allParams, paramsFromFileArray...)
	}

	return allParams, nil
}
