package generators

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jeremywohl/flatten"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/applicationset/services"
	"github.com/argoproj/argo-cd/v3/applicationset/utils"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/gpg"
)

var _ Generator = (*GitGenerator)(nil)

type GitGenerator struct {
	repos     services.Repos
	namespace string
}

// NewGitGenerator creates a new instance of Git Generator
func NewGitGenerator(repos services.Repos, namespace string) Generator {
	g := &GitGenerator{
		repos:     repos,
		namespace: namespace,
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
		namespace := g.namespace
		if namespace == "" {
			namespace = appSet.Namespace
		}
		if err := client.Get(context.TODO(), types.NamespacedName{Name: project, Namespace: namespace}, appProject); err != nil {
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

	requestedApps := g.filterApps(appSetGenerator.Git.Directories, allPaths)

	res, err := g.generateParamsFromApps(requestedApps, appSetGenerator, useGoTemplate, goTemplateOptions)
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
		for absPath, content := range retrievedFiles {
			fileContentMap[absPath] = content
		}
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
		paramsFromFileArray, err := g.generateParamsFromGitFile(filePath, fileContentMap[filePath], appSetGenerator.Git.Values, useGoTemplate, goTemplateOptions, appSetGenerator.Git.PathParamPrefix)
		if err != nil {
			return nil, fmt.Errorf("unable to process file '%s': %w", filePath, err)
		}
		allParams = append(allParams, paramsFromFileArray...)
	}

	return allParams, nil
}

// generateParamsFromGitFile parses the content of a Git-tracked file and generates a slice of parameter maps.
// The file can contain a single YAML/JSON object or an array of such objects. Depending on the useGoTemplate flag,
// it either preserves structure for Go templating or flattens the objects for use as plain key-value parameters.
func (g *GitGenerator) generateParamsFromGitFile(filePath string, fileContent []byte, values map[string]string, useGoTemplate bool, goTemplateOptions []string, pathParamPrefix string) ([]map[string]any, error) {
	objectsFound := []map[string]any{}

	// First, we attempt to parse as an array
	err := yaml.Unmarshal(fileContent, &objectsFound)
	if err != nil {
		// If unable to parse as an array, attempt to parse as a single object
		singleObj := make(map[string]any)
		err = yaml.Unmarshal(fileContent, &singleObj)
		if err != nil {
			return nil, fmt.Errorf("unable to parse file: %w", err)
		}
		objectsFound = append(objectsFound, singleObj)
	} else if len(objectsFound) == 0 {
		// If file is valid but empty, add a default empty item
		objectsFound = append(objectsFound, map[string]any{})
	}

	res := []map[string]any{}

	for _, objectFound := range objectsFound {
		params := map[string]any{}

		if useGoTemplate {
			for k, v := range objectFound {
				params[k] = v
			}

			paramPath := map[string]any{}

			paramPath["path"] = path.Dir(filePath)
			paramPath["basename"] = path.Base(paramPath["path"].(string))
			paramPath["filename"] = path.Base(filePath)
			paramPath["basenameNormalized"] = utils.SanitizeName(path.Base(paramPath["path"].(string)))
			paramPath["filenameNormalized"] = utils.SanitizeName(path.Base(paramPath["filename"].(string)))
			paramPath["segments"] = strings.Split(paramPath["path"].(string), "/")
			if pathParamPrefix != "" {
				params[pathParamPrefix] = map[string]any{"path": paramPath}
			} else {
				params["path"] = paramPath
			}
		} else {
			flat, err := flatten.Flatten(objectFound, "", flatten.DotStyle)
			if err != nil {
				return nil, fmt.Errorf("error flattening object: %w", err)
			}
			for k, v := range flat {
				params[k] = fmt.Sprintf("%v", v)
			}
			pathParamName := "path"
			if pathParamPrefix != "" {
				pathParamName = pathParamPrefix + "." + pathParamName
			}
			params[pathParamName] = path.Dir(filePath)
			params[pathParamName+".basename"] = path.Base(params[pathParamName].(string))
			params[pathParamName+".filename"] = path.Base(filePath)
			params[pathParamName+".basenameNormalized"] = utils.SanitizeName(path.Base(params[pathParamName].(string)))
			params[pathParamName+".filenameNormalized"] = utils.SanitizeName(path.Base(params[pathParamName+".filename"].(string)))
			for k, v := range strings.Split(params[pathParamName].(string), "/") {
				if v != "" {
					params[pathParamName+"["+strconv.Itoa(k)+"]"] = v
				}
			}
		}

		err := appendTemplatedValues(values, params, useGoTemplate, goTemplateOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to append templated values: %w", err)
		}

		res = append(res, params)
	}

	return res, nil
}

// filterApps filters the list of all application paths based on inclusion and exclusion rules
// defined in GitDirectoryGeneratorItems. Each item can either include or exclude matching paths.
func (g *GitGenerator) filterApps(directories []argoprojiov1alpha1.GitDirectoryGeneratorItem, allPaths []string) []string {
	var res []string
	for _, appPath := range allPaths {
		appInclude := false
		appExclude := false
		// Iterating over each appPath and check whether directories object has requestedPath that matches the appPath
		for _, requestedPath := range directories {
			match, err := path.Match(requestedPath.Path, appPath)
			if err != nil {
				log.WithError(err).WithField("requestedPath", requestedPath).
					WithField("appPath", appPath).Error("error while matching appPath to requestedPath")
				continue
			}
			if match && !requestedPath.Exclude {
				appInclude = true
			}
			if match && requestedPath.Exclude {
				appExclude = true
			}
		}
		// Whenever there is a path with exclude: true it wont be included, even if it is included in a different path pattern
		if appInclude && !appExclude {
			res = append(res, appPath)
		}
	}
	return res
}

// generateParamsFromApps generates a list of parameter maps based on the given app paths.
// Each app path is converted into a parameter object with path metadata (basename, segments, etc.).
// It supports both Go templates and flat key-value parameters.
func (g *GitGenerator) generateParamsFromApps(requestedApps []string, appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, useGoTemplate bool, goTemplateOptions []string) ([]map[string]any, error) {
	res := make([]map[string]any, len(requestedApps))
	for i, a := range requestedApps {
		params := make(map[string]any, 5)

		if useGoTemplate {
			paramPath := map[string]any{}
			paramPath["path"] = a
			paramPath["basename"] = path.Base(a)
			paramPath["basenameNormalized"] = utils.SanitizeName(path.Base(a))
			paramPath["segments"] = strings.Split(paramPath["path"].(string), "/")
			if appSetGenerator.Git.PathParamPrefix != "" {
				params[appSetGenerator.Git.PathParamPrefix] = map[string]any{"path": paramPath}
			} else {
				params["path"] = paramPath
			}
		} else {
			pathParamName := "path"
			if appSetGenerator.Git.PathParamPrefix != "" {
				pathParamName = appSetGenerator.Git.PathParamPrefix + "." + pathParamName
			}
			params[pathParamName] = a
			params[pathParamName+".basename"] = path.Base(a)
			params[pathParamName+".basenameNormalized"] = utils.SanitizeName(path.Base(a))
			for k, v := range strings.Split(params[pathParamName].(string), "/") {
				if v != "" {
					params[pathParamName+"["+strconv.Itoa(k)+"]"] = v
				}
			}
		}

		err := appendTemplatedValues(appSetGenerator.Git.Values, params, useGoTemplate, goTemplateOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to append templated values: %w", err)
		}

		res[i] = params
	}

	return res, nil
}

// resolveProjectName resolves a project name whether templated or not
func resolveProjectName(project string) string {
	if strings.Contains(project, "{{") {
		return ""
	}

	return project
}
