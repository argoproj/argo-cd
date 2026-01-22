package generators

import (
	"fmt"
	"maps"
	"path"
	"strconv"
	"strings"

	"github.com/jeremywohl/flatten"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/applicationset/utils"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// filterGitPaths filters paths based on Git directory inclusion/exclusion rules.
func filterGitPaths(directories []argoprojiov1alpha1.GitDirectoryGeneratorItem, allPaths []string) []string {
	return filterPaths(allPaths, len(directories),
		func(i int) (string, bool) {
			return directories[i].Path, directories[i].Exclude
		})
}

// filterOciPaths filters paths based on OCI directory inclusion/exclusion rules.
func filterOciPaths(directories []argoprojiov1alpha1.OciDirectoryGeneratorItem, allPaths []string) []string {
	return filterPaths(allPaths, len(directories),
		func(i int) (string, bool) {
			return directories[i].Path, directories[i].Exclude
		})
}

// filterPaths is the common filtering logic extracted from both Git and OCI generators.
func filterPaths(allPaths []string, numPatterns int, getPattern func(int) (path string, exclude bool)) []string {
	var res []string
	for _, appPath := range allPaths {
		appInclude := false
		appExclude := false
		// Iterating over each appPath and check whether directories object has requestedPath that matches the appPath
		for i := range numPatterns {
			patternPath, exclude := getPattern(i)
			match, err := path.Match(patternPath, appPath)
			if err != nil {
				log.WithError(err).
					WithField("requestedPath", patternPath).
					WithField("appPath", appPath).
					Error("error while matching appPath to requestedPath")
				continue
			}
			if match && !exclude {
				appInclude = true
			}
			if match && exclude {
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
