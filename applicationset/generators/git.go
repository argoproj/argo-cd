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

	"github.com/argoproj/argo-cd/v2/applicationset/services"
	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/gpg"
)

var _ Generator = (*GitGenerator)(nil)

type GitGenerator struct {
	repos     services.Repos
	namespace string
}

func NewGitGenerator(repos services.Repos, namespace string) Generator {
	g := &GitGenerator{
		repos:     repos,
		namespace: namespace,
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

func (g *GitGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, appSet *argoprojiov1alpha1.ApplicationSet, client client.Client) ([]map[string]interface{}, error) {
	if appSetGenerator == nil {
		return nil, EmptyAppSetGeneratorError
	}

	if appSetGenerator.Git == nil {
		return nil, EmptyAppSetGeneratorError
	}

	noRevisionCache := appSet.RefreshRequired()

	verifyCommit := false

	// When the project field is templated, the contents of the git repo are required to run the git generator and get the templated value,
	// but git generator cannot be called without verifying the commit signature.
	// In this case, we skip the signature verification.
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

	var err error
	var res []map[string]interface{}
	if len(appSetGenerator.Git.Directories) != 0 {
		res, err = g.generateParamsForGitDirectories(appSetGenerator, noRevisionCache, verifyCommit, appSet.Spec.GoTemplate, appSet.Spec.GoTemplateOptions)
	} else if len(appSetGenerator.Git.Files) != 0 {
		res, err = g.generateParamsForGitFiles(appSetGenerator, noRevisionCache, verifyCommit, appSet.Spec.GoTemplate, appSet.Spec.GoTemplateOptions)
	} else {
		return nil, EmptyAppSetGeneratorError
	}
	if err != nil {
		return nil, fmt.Errorf("error generating params from git: %w", err)
	}

	return res, nil
}

func (g *GitGenerator) generateParamsForGitDirectories(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, noRevisionCache, verifyCommit bool, useGoTemplate bool, goTemplateOptions []string) ([]map[string]interface{}, error) {
	// Directories, not files
	allPaths, err := g.repos.GetDirectories(context.TODO(), appSetGenerator.Git.RepoURL, appSetGenerator.Git.Revision, noRevisionCache, verifyCommit)
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

func (g *GitGenerator) generateParamsForGitFiles(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, noRevisionCache, verifyCommit bool, useGoTemplate bool, goTemplateOptions []string) ([]map[string]interface{}, error) {
	// Get all files that match the requested path string, removing duplicates
	allFiles := make(map[string][]byte)
	for _, requestedPath := range appSetGenerator.Git.Files {
		files, err := g.repos.GetFiles(context.TODO(), appSetGenerator.Git.RepoURL, appSetGenerator.Git.Revision, requestedPath.Path, noRevisionCache, verifyCommit)
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
		paramsArray, err := g.generateParamsFromGitFile(path, allFiles[path], appSetGenerator.Git.Values, useGoTemplate, goTemplateOptions, appSetGenerator.Git.PathParamPrefix)
		if err != nil {
			return nil, fmt.Errorf("unable to process file '%s': %w", path, err)
		}

		res = append(res, paramsArray...)
	}
	return res, nil
}

func (g *GitGenerator) generateParamsFromGitFile(filePath string, fileContent []byte, values map[string]string, useGoTemplate bool, goTemplateOptions []string, pathParamPrefix string) ([]map[string]interface{}, error) {
	objectsFound := []map[string]interface{}{}

	// First, we attempt to parse as an array
	err := yaml.Unmarshal(fileContent, &objectsFound)
	if err != nil {
		// If unable to parse as an array, attempt to parse as a single object
		singleObj := make(map[string]interface{})
		err = yaml.Unmarshal(fileContent, &singleObj)
		if err != nil {
			return nil, fmt.Errorf("unable to parse file: %w", err)
		}
		objectsFound = append(objectsFound, singleObj)
	} else if len(objectsFound) == 0 {
		// If file is valid but empty, add a default empty item
		objectsFound = append(objectsFound, map[string]interface{}{})
	}

	res := []map[string]interface{}{}

	for _, objectFound := range objectsFound {
		params := map[string]interface{}{}

		if useGoTemplate {
			for k, v := range objectFound {
				params[k] = v
			}

			paramPath := map[string]interface{}{}

			paramPath["path"] = path.Dir(filePath)
			paramPath["basename"] = path.Base(paramPath["path"].(string))
			paramPath["filename"] = path.Base(filePath)
			paramPath["basenameNormalized"] = utils.SanitizeName(path.Base(paramPath["path"].(string)))
			paramPath["filenameNormalized"] = utils.SanitizeName(path.Base(paramPath["filename"].(string)))
			paramPath["segments"] = strings.Split(paramPath["path"].(string), "/")
			if pathParamPrefix != "" {
				params[pathParamPrefix] = map[string]interface{}{"path": paramPath}
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
				if len(v) > 0 {
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

func (g *GitGenerator) filterApps(directories []argoprojiov1alpha1.GitDirectoryGeneratorItem, allPaths []string) []string {
	res := []string{}
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

func (g *GitGenerator) generateParamsFromApps(requestedApps []string, appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, useGoTemplate bool, goTemplateOptions []string) ([]map[string]interface{}, error) {
	res := make([]map[string]interface{}, len(requestedApps))
	for i, a := range requestedApps {
		params := make(map[string]interface{}, 5)

		if useGoTemplate {
			paramPath := map[string]interface{}{}
			paramPath["path"] = a
			paramPath["basename"] = path.Base(a)
			paramPath["basenameNormalized"] = utils.SanitizeName(path.Base(a))
			paramPath["segments"] = strings.Split(paramPath["path"].(string), "/")
			if appSetGenerator.Git.PathParamPrefix != "" {
				params[appSetGenerator.Git.PathParamPrefix] = map[string]interface{}{"path": paramPath}
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
				if len(v) > 0 {
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
