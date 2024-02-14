package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/argoproj/argo-cd/v2/pkg/version_config_manager"
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

func getVersionFromFile(appPath, jsonPathExpression string) (*string, error) {
	content, err := os.ReadFile(appPath)
	if err != nil {
		return nil, err
	}

	log.Infof("AppVersion source content was read from %s", appPath)

	var obj interface{}
	var jsonObj interface{}

	// Determine the file type and unmarshal accordingly
	switch filepath.Ext(appPath) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(content, &obj); err != nil {
			return nil, err
		}
		// Convert YAML to Map[interface{}]interface{}
		jsonObj, err = convertToJSONCompatible(obj)
		if err != nil {
			return nil, err
		}
	case ".json":
		if err := json.Unmarshal(content, &obj); err != nil {
			return nil, err
		}
		jsonObj = obj
	default:
		return nil, fmt.Errorf("Unsupported file format of %s", appPath)
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
	log.Infof("Extracted appVersion: %s", appVersion)
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

func getAppVersions(appPath string, versionConfig *version_config_manager.VersionConfig) (*Result, error) {
	// Defaults
	resourceName := "Chart.yaml"
	jsonPathExpression := "{.appVersion}"

	if versionConfig != nil {
		if versionConfig.ResourceName != "" {
			resourceName = versionConfig.ResourceName
		}
		if versionConfig.JsonPath != "" {
			jsonPathExpression = versionConfig.JsonPath
		}
	}

	// Get version of root
	log.Infof("appVersion get from file: %s, jsonPath: %s", filepath.Join(appPath, resourceName), jsonPathExpression)
	appVersion, err := getVersionFromFile(filepath.Join(appPath, resourceName), jsonPathExpression)
	if err != nil {
		log.Errorf("Error in getVersionFromFile. %v", err)
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

	log.Infof("Return appVersion as: %v", result)
	return result, nil
}
