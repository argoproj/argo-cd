package generators

import (
	"context"
	"fmt"
	"maps"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/jeremywohl/flatten"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/applicationset/utils"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// repoSourceKind identifies the type of repository source backing a generator.
type repoSourceKind string

const (
	repoSourceKindGit repoSourceKind = "git"
	repoSourceKindOCI repoSourceKind = "oci"
)

// pathPattern is an include/exclude path rule for directory or file matching.
type pathPattern struct {
	Path    string
	Exclude bool
}

// repoSourceSpec holds the per-call configuration shared by the Git and OCI generators.
type repoSourceSpec struct {
	URL             string
	Revision        string
	PathParamPrefix string
	Values          map[string]string
	Directories     []pathPattern
	Files           []pathPattern
}

// repoSource is implemented by GitGenerator and OciGenerator to back the shared directory/file traversal logic.
type repoSource interface {
	// resolveSourceIntegrity returns the source-integrity policy for the generator's AppProject,
	// or nil if the backend doesn't support signing.
	resolveSourceIntegrity(ctx context.Context, appSet *argoprojiov1alpha1.ApplicationSet, client client.Client) (*argoprojiov1alpha1.SourceIntegrity, error)

	listDirectories(ctx context.Context, repoURL, revision, project string, noRevisionCache bool, sourceIntegrity *argoprojiov1alpha1.SourceIntegrity) ([]string, error)
	getFiles(ctx context.Context, repoURL, revision, project, pattern string, noRevisionCache bool, sourceIntegrity *argoprojiov1alpha1.SourceIntegrity) (map[string][]byte, error)
}

// filterPaths filters paths based on directory inclusion/exclusion rules.
func filterPaths(dirs []pathPattern, allPaths []string) []string {
	var res []string
	for _, appPath := range allPaths {
		appInclude := false
		appExclude := false
		for _, dir := range dirs {
			match, err := path.Match(dir.Path, appPath)
			if err != nil {
				log.WithError(err).
					WithField("requestedPath", dir.Path).
					WithField("appPath", appPath).
					Error("error while matching appPath to requestedPath")
				continue
			}
			if match && !dir.Exclude {
				appInclude = true
			}
			if match && dir.Exclude {
				appExclude = true
			}
		}
		// Whenever there is a path with exclude: true it won't be included, even if it is included in a different path pattern
		if appInclude && !appExclude {
			res = append(res, appPath)
		}
	}
	return res
}

// generateParamsFromPaths generates parameter maps from a list of paths.
// This is common logic shared between Git and OCI generators for directory-based generation.
func generateParamsFromPaths(
	paths []string,
	pathParamPrefix string,
	values map[string]string,
	useGoTemplate bool,
	goTemplateOptions []string,
) (
	[]map[string]any, error,
) {
	res := make([]map[string]any, len(paths))
	for i, appPath := range paths {
		params := buildPathParameters(appPath, pathParamPrefix, useGoTemplate)

		err := appendTemplatedValues(values, params, useGoTemplate, goTemplateOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to append templated values: %w", err)
		}

		res[i] = params
	}

	return res, nil
}

// buildPathParameters creates parameter map from a single path.
// Handles both Go template and flat parameter styles.
func buildPathParameters(appPath, pathParamPrefix string, useGoTemplate bool) map[string]any {
	params := make(map[string]any, 5)

	if useGoTemplate {
		paramPath := map[string]any{}
		paramPath["path"] = appPath
		paramPath["basename"] = path.Base(appPath)
		paramPath["basenameNormalized"] = utils.SanitizeName(path.Base(appPath))
		paramPath["segments"] = strings.Split(paramPath["path"].(string), "/")
		if pathParamPrefix != "" {
			params[pathParamPrefix] = map[string]any{"path": paramPath}
		} else {
			params["path"] = paramPath
		}
	} else {
		pathParamName := "path"
		if pathParamPrefix != "" {
			pathParamName = pathParamPrefix + "." + pathParamName
		}
		params[pathParamName] = appPath
		params[pathParamName+".basename"] = path.Base(appPath)
		params[pathParamName+".basenameNormalized"] = utils.SanitizeName(path.Base(appPath))
		for k, v := range strings.Split(params[pathParamName].(string), "/") {
			if v != "" {
				params[pathParamName+"["+strconv.Itoa(k)+"]"] = v
			}
		}
	}

	return params
}

// parseFileParams parses a YAML/JSON file and generates parameter maps.
// This is common logic shared between Git and OCI generators for file-based generation.
func parseFileParams(
	filePath string,
	fileContent []byte,
	pathParamPrefix string,
	values map[string]string,
	useGoTemplate bool,
	goTemplateOptions []string,
) (
	[]map[string]any,
	error,
) {
	objectsFound := []map[string]any{}

	// First, we attempt to parse as a single object.
	// This will also succeed for empty files.
	singleObj := map[string]any{}
	err := yaml.Unmarshal(fileContent, &singleObj)
	if err == nil {
		objectsFound = append(objectsFound, singleObj)
	} else {
		// If unable to parse as an object, try to parse as an array
		err = yaml.Unmarshal(fileContent, &objectsFound)
		if err != nil {
			return nil, fmt.Errorf("unable to parse file: %w", err)
		}
	}

	res := []map[string]any{}

	for _, objectFound := range objectsFound {
		params := map[string]any{}

		if useGoTemplate {
			maps.Copy(params, objectFound)

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

// resolveProjectName resolves a project name whether templated or not
func resolveProjectName(project string) string {
	if strings.Contains(project, "{{") {
		return ""
	}

	return project
}

// generateRepoSourceParams resolves source integrity and dispatches to directory or file-based generation.
func generateRepoSourceParams(src repoSource, kind repoSourceKind, spec repoSourceSpec, appSet *argoprojiov1alpha1.ApplicationSet, client client.Client) ([]map[string]any, error) {
	noRevisionCache := appSet.RefreshRequired()

	sourceIntegrity, err := src.resolveSourceIntegrity(context.TODO(), appSet, client)
	if err != nil {
		return nil, err
	}

	project := resolveProjectName(appSet.Spec.Template.Spec.Project)

	var res []map[string]any
	switch {
	case len(spec.Directories) != 0:
		res, err = generateRepoSourceDirectoryParams(context.TODO(), src, kind, spec, project, noRevisionCache, sourceIntegrity, appSet.Spec.GoTemplate, appSet.Spec.GoTemplateOptions)
	case len(spec.Files) != 0:
		res, err = generateRepoSourceFileParams(context.TODO(), src, spec, project, noRevisionCache, sourceIntegrity, appSet.Spec.GoTemplate, appSet.Spec.GoTemplateOptions)
	default:
		return nil, ErrEmptyAppSetGenerator
	}
	if err != nil {
		return nil, fmt.Errorf("error generating params from %s: %w", kind, err)
	}

	return res, nil
}

// generateRepoSourceDirectoryParams lists directories from src, applies path filters, and returns parameter maps.
func generateRepoSourceDirectoryParams(ctx context.Context, src repoSource, kind repoSourceKind, spec repoSourceSpec, project string, noRevisionCache bool, sourceIntegrity *argoprojiov1alpha1.SourceIntegrity, useGoTemplate bool, goTemplateOptions []string) ([]map[string]any, error) {
	allPaths, err := src.listDirectories(ctx, spec.URL, spec.Revision, project, noRevisionCache, sourceIntegrity)
	if err != nil {
		return nil, fmt.Errorf("error getting directories from %s: %w", kind, err)
	}

	log.WithFields(log.Fields{
		"allPaths":        allPaths,
		"total":           len(allPaths),
		"repoURL":         spec.URL,
		"revision":        spec.Revision,
		"pathParamPrefix": spec.PathParamPrefix,
	}).Info("applications result from the repo service")

	requestedApps := filterPaths(spec.Directories, allPaths)

	res, err := generateParamsFromPaths(requestedApps, spec.PathParamPrefix, spec.Values, useGoTemplate, goTemplateOptions)
	if err != nil {
		return nil, fmt.Errorf("error generating params from apps: %w", err)
	}

	return res, nil
}

// generateRepoSourceFileParams fetches files from src and parses each as YAML/JSON to produce parameter maps.
func generateRepoSourceFileParams(ctx context.Context, src repoSource, spec repoSourceSpec, project string, noRevisionCache bool, sourceIntegrity *argoprojiov1alpha1.SourceIntegrity, useGoTemplate bool, goTemplateOptions []string) ([]map[string]any, error) {
	fileContentMap := make(map[string][]byte)
	var includePatterns []string
	var excludePatterns []string

	for _, req := range spec.Files {
		if req.Exclude {
			excludePatterns = append(excludePatterns, req.Path)
		} else {
			includePatterns = append(includePatterns, req.Path)
		}
	}

	for _, includePattern := range includePatterns {
		retrievedFiles, err := src.getFiles(ctx, spec.URL, spec.Revision, project, includePattern, noRevisionCache, sourceIntegrity)
		if err != nil {
			return nil, err
		}
		maps.Copy(fileContentMap, retrievedFiles)
	}

	for _, excludePattern := range excludePatterns {
		matchingFiles, err := src.getFiles(ctx, spec.URL, spec.Revision, project, excludePattern, noRevisionCache, sourceIntegrity)
		if err != nil {
			return nil, err
		}
		for absPath := range matchingFiles {
			delete(fileContentMap, absPath)
		}
	}

	var filePaths []string
	for p := range fileContentMap {
		filePaths = append(filePaths, p)
	}
	sort.Strings(filePaths)

	var allParams []map[string]any
	for _, filePath := range filePaths {
		paramsFromFileArray, err := parseFileParams(filePath, fileContentMap[filePath], spec.PathParamPrefix, spec.Values, useGoTemplate, goTemplateOptions)
		if err != nil {
			return nil, fmt.Errorf("unable to process file '%s': %w", filePath, err)
		}
		allParams = append(allParams, paramsFromFileArray...)
	}

	return allParams, nil
}
