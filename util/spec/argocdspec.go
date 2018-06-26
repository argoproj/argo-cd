package spec

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"

	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/common"
)

const (
	appSpecName = "argocd.yaml"
)

const (
	manifestVariable = "{{manifest}}"
)

var workflowParamAnnotation = regexp.MustCompile(common.AnnotationWorkflowParamPrefix + "\\.([0-9A-Za-z_-]+)\\.([0-9A-Za-z_-]+)")

// ArgoCDSpec is the type definition for a argocd.yaml spec.
type ArgoCDSpec struct {
	Workflows []WorkflowRef `json:"workflows"`
}

// WorkflowRef is a reference to a workflow in a ksonnet app directory
type WorkflowRef struct {
	Name     string         `json:"name"`
	Path     string         `json:"path"`
	Workflow *wfv1.Workflow `json:"-"`
}

// GetApplicationWorkflows will return the application workflows from a ksonnet app dir (if existing)
func GetApplicationWorkflows(appDir string, resources []*unstructured.Unstructured) ([]WorkflowRef, error) {
	bytes, err := ioutil.ReadFile(path.Join(appDir, appSpecName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("%s failed to read: %v", appSpecName, err)
	}
	var argocdSpec ArgoCDSpec
	err = yaml.Unmarshal(bytes, &argocdSpec)
	if err != nil {
		return nil, fmt.Errorf("%s failed to parse: %v", appSpecName, err)
	}

	wfRefByName := make(map[string]WorkflowRef)
	for _, wfRef := range argocdSpec.Workflows {
		wfPath := path.Join(appDir, wfRef.Path)
		bytes, err := ioutil.ReadFile(wfPath)
		if err != nil {
			log.Warnf("Unable to read workflow from: %s: %v", wfPath, err)
			continue
		}
		var wf wfv1.Workflow
		err = yaml.Unmarshal(bytes, &wf)
		if err != nil {
			log.Warnf("Workflow at %s invalid: %v", wfPath, err)
			continue
		}
		wfRef.Workflow = &wf
		wfRefByName[wfRef.Name] = wfRef
	}

	// variables holds mapping of variable names to values
	// (e.g. "{{manifest}}" : <manifest json>)
	variables := make(map[string]string)

	for _, res := range resources {
		jsonBytes, err := res.MarshalJSON()
		if err != nil {
			log.Warnf("Could not marshal %s/%s as JSON: %v", res.GetKind(), res.GetName(), err)
			continue
		}
		variables[manifestVariable] = string(jsonBytes)
		annotations := res.GetAnnotations()
		for key, val := range annotations {
			parts := workflowParamAnnotation.FindStringSubmatch(key)
			if len(parts) != 3 {
				continue
			}
			wfName := parts[1]
			wfRef, ok := wfRefByName[wfName]
			if !ok {
				log.Warnf("Annotation %s refers to a undefined workflow: %s", key, wfName)
				continue
			}
			paramName := parts[2]
			setWorkflowParameter(wfRef.Workflow, paramName, val, variables)
		}
	}
	// return the list of workflows (in the same order as specified in the argocd.yaml)
	// this also filters out invalid workflows
	appWfs := make([]WorkflowRef, 0)
	for _, wfRef := range argocdSpec.Workflows {
		if appWf, ok := wfRefByName[wfRef.Name]; ok {
			appWfs = append(appWfs, appWf)
		}
	}
	return appWfs, nil
}

// setWorkflowParameter sets a workflow parameter value (i.e. spec.arguments.parameters.XXX.value)
func setWorkflowParameter(wf *wfv1.Workflow, name, value string, variables map[string]string) {
	// substitute variables
	for varName, varVal := range variables {
		value = strings.Replace(value, varName, varVal, -1)
	}
	param := wfv1.Parameter{
		Name:  name,
		Value: &value,
	}
	for i, p := range wf.Spec.Arguments.Parameters {
		if p.Name != name {
			continue
		}
		wf.Spec.Arguments.Parameters[i] = param
		return
	}
	wf.Spec.Arguments.Parameters = append(wf.Spec.Arguments.Parameters, param)
}
