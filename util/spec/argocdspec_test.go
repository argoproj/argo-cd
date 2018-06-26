package spec

import (
	"encoding/json"
	"io/ioutil"
	"regexp"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/errors"
)

var yamlSeparator = regexp.MustCompile("\\n---")

func yamlToUnstructured(path string) []*unstructured.Unstructured {
	bytes, err := ioutil.ReadFile(path)
	errors.CheckError(err)

	manifestsStrings := yamlSeparator.Split(string(bytes), -1)
	manifests := make([]*unstructured.Unstructured, 0)
	for _, manifestStr := range manifestsStrings {
		if strings.TrimSpace(manifestStr) == "" {
			continue
		}
		var un unstructured.Unstructured
		err := yaml.Unmarshal([]byte(manifestStr), &un)
		errors.CheckError(err)
		manifests = append(manifests, &un)
	}
	return manifests
}

func TestGetApplicationWorkflows(t *testing.T) {
	resources := yamlToUnstructured("testdata/bg-deploy/guestbook-ui.yaml")
	appWorkflows, err := GetApplicationWorkflows("testdata/bg-deploy", resources)
	assert.Nil(t, err)
	appWf := appWorkflows[0]
	assert.Equal(t, appWf.Name, "blue-green")

	// Verify parameter substitution for simple strings works
	param := appWf.Workflow.Spec.Arguments.GetParameterByName("service-name")
	assert.NotNil(t, param)
	assert.Equal(t, *param.Value, "bg-deploy")
	param = appWf.Workflow.Spec.Arguments.GetParameterByName("deployment-name")
	assert.NotNil(t, param)
	assert.Equal(t, *param.Value, "bg-deploy")

	// Verify the '{{manifest}}' variable is substituted
	param = appWf.Workflow.Spec.Arguments.GetParameterByName("new-deployment-manifest")
	assert.NotNil(t, param)
	var un unstructured.Unstructured
	err = json.Unmarshal([]byte(*param.Value), &un)
	assert.Nil(t, err)
	assert.Equal(t, un.GetKind(), "Deployment")
	assert.Equal(t, un.GetName(), "bg-deploy")
}

func TestMissingArgoCDYAML(t *testing.T) {
	appWfs, err := GetApplicationWorkflows(".", nil)
	assert.Nil(t, err)
	assert.Nil(t, appWfs)
}
