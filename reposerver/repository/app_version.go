package repository

import (
	"os"
	"path/filepath"
	"reflect"
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

func getVersionFromYaml(appPath, jsonPathExpression string) (*string, error) {
	content, err := os.ReadFile(appPath)
	if err != nil {
		return nil, err
	}

	log.Infof("AppVersion source content: %s", string(content))

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
	log.Infof("AppVersion source content appVersion: %s", appVersion)
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

func readFileContent(result *Result, appPath, fileName, fieldName string) {
	content, err := os.ReadFile(filepath.Join(appPath, fileName))
	if err == nil && content != nil {
		v := reflect.ValueOf(result).Elem()
		f := v.FieldByName("Dependencies").FieldByName(fieldName)
		if f.IsValid() && f.CanSet() && f.Kind() == reflect.String {
			f.SetString(string(content))
		}
	}
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
	log.Infof("appVersion value: %v (appPath=%s)", *appVersion, appPath)

	result := &Result{
		AppVersion:   *appVersion,
		Dependencies: DependenciesMap{},
	}

	readFileContent(result, appPath, "Chart.lock", "Lock")
	readFileContent(result, appPath, "Chart.yaml", "Deps")
	readFileContent(result, appPath, "requirements.yaml", "Requirements")

	return result, nil
}
