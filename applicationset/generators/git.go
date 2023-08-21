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
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/applicationset/services"
	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

var _ Generator = (*GitGenerator)(nil)

type GitGenerator struct {
	repos services.Repos
}

func NewGitGenerator(repos services.Repos) Generator {
	g := &GitGenerator{
		repos: repos,
	}
	return g
}

func (g *GitGenerator) GetTemplate(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) *argoprojiov1alpha1.ApplicationSetTemplate {
	return &appSetGenerator.Git.Template
}

func (g *GitGenerator) GetRequeueAfter(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) time.Duration {

	// Return a requeue default of 3 minutes, if no default is specified.

	if appSetGenerator.Git.RequeueAfterSeconds != nil {
		return time.Duration(*appSetGenerator.Git.RequeueAfterSeconds) * time.Second
	}

	return DefaultRequeueAfterSeconds
}

func (g *GitGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, appSet *argoprojiov1alpha1.ApplicationSet) ([]map[string]interface{}, error) {

	if appSetGenerator == nil {
		return nil, EmptyAppSetGeneratorError
	}

	if appSetGenerator.Git == nil {
		return nil, EmptyAppSetGeneratorError
	}

	var err error
	var res []map[string]interface{}
	if len(appSetGenerator.Git.Directories) != 0 {
		res, err = g.generateParamsForGitDirectories(appSetGenerator, appSet.Spec.GoTemplate, appSet.Spec.GoTemplateOptions)
	} else if len(appSetGenerator.Git.Files) != 0 {
		res, err = g.generateParamsForGitFiles(appSetGenerator, appSet.Spec.GoTemplate, appSet.Spec.GoTemplateOptions)
	} else {
		return nil, EmptyAppSetGeneratorError
	}
	if err != nil {
		return nil, fmt.Errorf("error generating params from git: %w", err)
	}

	return res, nil
}

func (g *GitGenerator) generateParamsForGitDirectories(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, useGoTemplate bool, goTemplateOptions []string) ([]map[string]interface{}, error) {

	commitSHA, err := g.repos.CommitSHA(context.TODO(), appSetGenerator.Git.RepoURL, appSetGenerator.Git.Revision)
	if err != nil {
		return nil, err
	}

	// Directories, not files
	allPaths, err := g.repos.GetDirectories(context.TODO(), appSetGenerator.Git.RepoURL, commitSHA)
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

	res, err := g.generateParamsFromApps(commitSHA, requestedApps, appSetGenerator, useGoTemplate, goTemplateOptions)
	if err != nil {
		return nil, fmt.Errorf("error generating params from apps: %w", err)
	}

	return res, nil
}

func (g *GitGenerator) generateParamsForGitFiles(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, useGoTemplate bool, goTemplateOptions []string) ([]map[string]interface{}, error) {

	// Get all files that match the requested path string, removing duplicates
	allFiles := make(map[string][]byte)
	commitSHA, err := g.repos.CommitSHA(context.TODO(), appSetGenerator.Git.RepoURL, appSetGenerator.Git.Revision)
	if err != nil {
		return nil, err
	}

	for _, requestedPath := range appSetGenerator.Git.Files {
		files, err := g.repos.GetFiles(context.TODO(), appSetGenerator.Git.RepoURL, commitSHA, requestedPath.Path)
		if err != nil {
			return nil, err
		}

		for filePath, content := range files {
			allFiles[filePath] = content
		}
	}

	// Extract the unduplicated map into a list, and sort by path to ensure a deterministic
	// processing order in the subsequent step
	allPaths := []string{}
	for path := range allFiles {
		allPaths = append(allPaths, path)
	}
	sort.Strings(allPaths)

	// Generate params from each path, and return
	res := []map[string]interface{}{}
	for _, path := range allPaths {

		// A JSON / YAML file path can contain multiple sets of parameters (ie it is an array)
		paramsArray, err := g.generateParamsFromGitFile(commitSHA, path, allFiles[path], appSetGenerator.Git.Values, useGoTemplate, goTemplateOptions, appSetGenerator.Git.PathParamPrefix)
		if err != nil {
			return nil, fmt.Errorf("unable to process file '%s': %v", path, err)
		}

		res = append(res, paramsArray...)
	}
	return res, nil
}

func (g *GitGenerator) generateParamsFromGitFile(commitSHA, filePath string, fileContent []byte, values map[string]string, useGoTemplate bool, goTemplateOptions []string, pathParamPrefix string) ([]map[string]interface{}, error) {
	objectsFound := []map[string]interface{}{}

	// First, we attempt to parse as an array
	err := yaml.Unmarshal(fileContent, &objectsFound)
	if err != nil {
		// If unable to parse as an array, attempt to parse as a single object
		singleObj := make(map[string]interface{})
		err = yaml.Unmarshal(fileContent, &singleObj)
		if err != nil {
			return nil, fmt.Errorf("unable to parse file: %v", err)
		}
		objectsFound = append(objectsFound, singleObj)
	}

	pathParamName := "path"
	commitSHAParamName := "commitSHA"

	res := []map[string]interface{}{}

	for _, objectFound := range objectsFound {

		params := map[string]interface{}{}

		if useGoTemplate {
			for k, v := range objectFound {
				params[k] = v
			}

			paramPath := map[string]interface{}{}

			paramPath[pathParamName] = path.Dir(filePath)
			paramPath["basename"] = path.Base(paramPath[pathParamName].(string))
			paramPath["filename"] = path.Base(filePath)
			paramPath["basenameNormalized"] = utils.SanitizeName(path.Base(paramPath[pathParamName].(string)))
			paramPath["filenameNormalized"] = utils.SanitizeName(path.Base(paramPath["filename"].(string)))
			paramPath["segments"] = strings.Split(paramPath[pathParamName].(string), "/")
			if pathParamPrefix != "" {
				params[pathParamPrefix] = map[string]interface{}{
					pathParamName:      paramPath,
					commitSHAParamName: commitSHA,
				}
			} else {
				params[pathParamName] = paramPath
				params[commitSHAParamName] = commitSHA
			}
		} else {
			paramNames := map[string]string{
				pathParamName:      pathParamName,
				commitSHAParamName: commitSHAParamName,
			}
			flat, err := flatten.Flatten(objectFound, "", flatten.DotStyle)
			if err != nil {
				return nil, fmt.Errorf("error flattening object: %w", err)
			}
			for k, v := range flat {
				params[k] = fmt.Sprintf("%v", v)
			}
			if pathParamPrefix != "" {
				paramNames[pathParamName] = pathParamPrefix + "." + pathParamName
				paramNames[commitSHAParamName] = pathParamPrefix + "." + commitSHAParamName
			}
			params[paramNames[commitSHAParamName]] = commitSHA
			params[paramNames[pathParamName]] = path.Dir(filePath)
			params[paramNames[pathParamName]+".basename"] = path.Base(params[paramNames[pathParamName]].(string))
			params[paramNames[pathParamName]+".filename"] = path.Base(filePath)
			params[paramNames[pathParamName]+".basenameNormalized"] = utils.SanitizeName(path.Base(params[paramNames[pathParamName]].(string)))
			params[paramNames[pathParamName]+".filenameNormalized"] = utils.SanitizeName(path.Base(params[paramNames[pathParamName]+".filename"].(string)))
			for k, v := range strings.Split(params[paramNames[pathParamName]].(string), "/") {
				if len(v) > 0 {
					params[paramNames[pathParamName]+"["+strconv.Itoa(k)+"]"] = v
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

func (g *GitGenerator) filterApps(Directories []argoprojiov1alpha1.GitDirectoryGeneratorItem, allPaths []string) []string {
	res := []string{}
	for _, appPath := range allPaths {
		appInclude := false
		appExclude := false
		// Iterating over each appPath and check whether directories object has requestedPath that matches the appPath
		for _, requestedPath := range Directories {
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

func (g *GitGenerator) generateParamsFromApps(commitSHA string, requestedApps []string, appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, useGoTemplate bool, goTemplateOptions []string) ([]map[string]interface{}, error) {
	pathParamName := "path"
	commitSHAParamName := "commitSHA"
	res := make([]map[string]interface{}, len(requestedApps))
	for i, a := range requestedApps {

		params := make(map[string]interface{}, 5)

		if useGoTemplate {
			paramPath := map[string]interface{}{}
			paramPath[pathParamName] = a
			paramPath["basename"] = path.Base(a)
			paramPath["basenameNormalized"] = utils.SanitizeName(path.Base(a))
			paramPath["segments"] = strings.Split(paramPath[pathParamName].(string), "/")
			if appSetGenerator.Git.PathParamPrefix != "" {
				params[appSetGenerator.Git.PathParamPrefix] = map[string]interface{}{
					pathParamName:      paramPath,
					commitSHAParamName: commitSHA,
				}
			} else {
				params[pathParamName] = paramPath
				params[commitSHAParamName] = commitSHA
			}
		} else {
			paramNames := map[string]string{
				pathParamName:      pathParamName,
				commitSHAParamName: commitSHAParamName,
			}
			if appSetGenerator.Git.PathParamPrefix != "" {
				paramNames[pathParamName] = appSetGenerator.Git.PathParamPrefix + "." + pathParamName
				paramNames[commitSHAParamName] = appSetGenerator.Git.PathParamPrefix + "." + commitSHAParamName
			}
			params[paramNames[commitSHAParamName]] = commitSHA
			params[paramNames[pathParamName]] = a
			params[paramNames[pathParamName]+".basename"] = path.Base(a)
			params[paramNames[pathParamName]+".basenameNormalized"] = utils.SanitizeName(path.Base(a))
			for k, v := range strings.Split(params[paramNames[pathParamName]].(string), "/") {
				if len(v) > 0 {
					params[paramNames[pathParamName]+"["+strconv.Itoa(k)+"]"] = v
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
