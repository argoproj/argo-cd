package repository

import (
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/util/jsonpath"
)

type DependenciesMap struct {
	Lock         string `json:"helm/Chart.lock"`
	Deps         string `json:"helm/dependencies"`
	Requirements string `json:"helm/requirements.yaml"`
}

type Result struct {
	AppVersion   string          `json:"appVersion"`
	Dependencies DependenciesMap `json:"dependencies"`
}

func getDependenciesFromChart(appPath string) (*string, error) {
	content, err := os.ReadFile(appPath)
	if err != nil {
		return nil, err
	}

	var obj map[interface{}]interface{}
	if err := yaml.Unmarshal(content, &obj); err != nil {
		return nil, err
	}

	var dependenciesStr string
	if obj["dependencies"] != nil {
		dependencies, err := yaml.Marshal(&map[interface{}]interface{}{"dependencies": obj["dependencies"]})
		if err != nil {
			return nil, err
		}
		dependenciesStr = string(dependencies)
	}

	return &dependenciesStr, nil
}

func getVersionFromYaml(appPath, jsonPathExpression string) (*string, error) {
	content, err := os.ReadFile(appPath)
	if err != nil {
		return nil, err
	}

	var obj interface{}
	if err := yaml.Unmarshal(content, &obj); err != nil {
		return nil, err
	}

	// Convert YAML to Map[interface{}]interface{}
	jsonObj, err := convertToJSONCompatible(obj)
	if err != nil {
		return nil, err
	}

	jp := jsonpath.New("jsonpathParser")
	jp.AllowMissingKeys(true)
	if err := jp.Parse(jsonPathExpression); err != nil {
		return nil, err
	}

	var buf strings.Builder
	err = jp.Execute(&buf, jsonObj)
	if err != nil {
		return nil, err
	}

	appVersion := buf.String()
	return &appVersion, nil
}

func convertToJSONCompatible(i interface{}) (interface{}, error) {
	data, err := yaml.Marshal(i)
	if err != nil {
		return nil, err
	}
	var obj interface{}
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func getAppVersions(appPath string, resourceName string, jsonPathExpression string) (*Result, error) {
	// Defaults
	if resourceName == "" {
		resourceName = "Chart.yaml"
	}
	if jsonPathExpression == "" {
		jsonPathExpression = "{.appVersion}"
	}

	// Get version of root
	appVersion, err := getVersionFromYaml(filepath.Join(appPath, resourceName), jsonPathExpression)
	if err != nil {
		return nil, err
	}
	log.Printf("appVersion value: %v\n", *appVersion)

	result := &Result{
		AppVersion:   *appVersion,
		Dependencies: DependenciesMap{},
	}

	// Get Chart.lock if exists
	lock, err := os.ReadFile(filepath.Join(appPath, "Chart.lock"))
	if err == nil && lock != nil {
		result.Dependencies.Lock = string(lock)
	}

	// Get `dependencies` property from Chart.yaml if exists
	deps, err := getDependenciesFromChart(filepath.Join(appPath, resourceName))
	if err == nil && deps != nil {
		result.Dependencies.Deps = *deps
	}

	// Get requirements.yaml if exists
	requirements, err := os.ReadFile(filepath.Join(appPath, "requirements.yaml"))
	if err == nil && requirements != nil {
		result.Dependencies.Requirements = string(requirements)
	}

	return result, nil
}
