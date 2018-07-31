package helm

import (
	"fmt"
	"os/exec"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"io/ioutil"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/kube"
)

// Helm provides wrapper functionality around the `helm` command.
type Helm interface {
	// Template returns a list of unstructured objects from a `helm template` command
	Template(name string, valuesFiles []string, overrides []*argoappv1.ComponentParameter) ([]*unstructured.Unstructured, []*argoappv1.ComponentParameter, error)
}

// NewHelmApp create a new wrapper to run commands on the `helm` command-line tool.
func NewHelmApp(path string) Helm {
	return &helm{path: path}
}

type helm struct {
	path string
}

func (h *helm) Template(name string, valuesFiles []string, overrides []*argoappv1.ComponentParameter) ([]*unstructured.Unstructured, []*argoappv1.ComponentParameter, error) {
	args := []string{
		"template", h.path, "--name", name,
	}
	for _, valuesFile := range valuesFiles {
		args = append(args, "-f", path.Join(h.path, valuesFile))
	}
	for _, p := range overrides {
		args = append(args, "--set", fmt.Sprintf("%s=%s", p.Name, p.Value))
	}
	out, err := helmCmd(args...)
	if err != nil {
		return nil, nil, err
	}
	objs, err := kube.SplitYAML(out)
	if err != nil {
		return nil, nil, err
	}
	params, err := h.getParams(valuesFiles)
	if err != nil {
		return nil, nil, err
	}
	return objs, params, nil
}

func (h *helm) getParams(valuesFiles []string) ([]*argoappv1.ComponentParameter, error) {
	out, err := helmCmd("inspect", "values", h.path)
	if err != nil {
		return nil, err
	}
	values := append([]string{out})
	for _, file := range valuesFiles {
		fileValues, err := ioutil.ReadFile(path.Join(h.path, file))
		if err != nil {
			return nil, fmt.Errorf("failed to read value file %s: %s", file, err)
		}
		values = append(values, string(fileValues))
	}

	output := map[string]string{}
	for _, file := range values {
		values := map[interface{}]interface{}{}
		if err = yaml.Unmarshal([]byte(file), &values); err != nil {
			return nil, fmt.Errorf("failed to parse values: %s", err)
		}
		flatVals(values, output)
	}

	params := make([]*argoappv1.ComponentParameter, 0)
	for key, val := range output {
		params = append(params, &argoappv1.ComponentParameter{
			Name:  key,
			Value: val,
		})
	}
	return params, nil
}

func helmCmd(args ...string) (string, error) {
	cmd := exec.Command("helm", args...)
	cmdStr := strings.Join(cmd.Args, " ")
	log.Info(cmdStr)
	outBytes, err := cmd.Output()
	if err != nil {
		exErr, ok := err.(*exec.ExitError)
		if !ok {
			return "", err
		}
		errOutput := string(exErr.Stderr)
		log.Errorf("`%s` failed: %s", cmdStr, errOutput)
		return "", fmt.Errorf(strings.TrimSpace(errOutput))
	}
	out := string(outBytes)
	log.Debug(out)
	return out, nil
}

func flatVals(input map[interface{}]interface{}, output map[string]string, prefixes ...string) {
	for key, val := range input {
		if subMap, ok := val.(map[interface{}]interface{}); ok {
			flatVals(subMap, output, append(prefixes, fmt.Sprintf("%v", key))...)
		} else {
			output[strings.Join(append(prefixes, fmt.Sprintf("%v", key)), ".")] = fmt.Sprintf("%v", val)
		}
	}
}
