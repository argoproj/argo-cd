package helm

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os/exec"
	"path"
	"strings"

	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/config"
	"github.com/argoproj/argo-cd/util/kube"
)

// Helm provides wrapper functionality around the `helm` command.
type Helm interface {
	// Template returns a list of unstructured objects from a `helm template` command
	Template(name, namespace string, valuesFiles []string, overrides []*argoappv1.ComponentParameter) ([]*unstructured.Unstructured, error)
	// GetParameters returns a list of chart parameters taking into account values in provided YAML files.
	GetParameters(valuesFiles []string) ([]*argoappv1.ComponentParameter, error)
	// DependencyBuild runs `helm dependency build` to download a chart's dependencies
	DependencyBuild() error
	// SetHome sets the helm home location (default "~/.helm")
	SetHome(path string)
	// Init runs `helm init --client-only`
	Init() error
}

// NewHelmApp create a new wrapper to run commands on the `helm` command-line tool.
func NewHelmApp(path string) Helm {
	return &helm{path: path}
}

type helm struct {
	path string
	home string
}

func (h *helm) Template(name, namespace string, valuesFiles []string, overrides []*argoappv1.ComponentParameter) ([]*unstructured.Unstructured, error) {
	args := []string{
		"template", ".", "--name", name,
	}
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}
	for _, valuesFile := range valuesFiles {
		args = append(args, "-f", valuesFile)
	}
	for _, p := range overrides {
		args = append(args, "--set", fmt.Sprintf("%s=%s", p.Name, p.Value))
	}
	out, err := h.helmCmd(args...)
	if err != nil {
		return nil, err
	}
	return kube.SplitYAML(out)
}

func (h *helm) DependencyBuild() error {
	_, err := h.helmCmd("dependency", "build")
	return err
}

func (h *helm) SetHome(home string) {
	h.home = home
}

func (h *helm) Init() error {
	_, err := h.helmCmd("init", "--client-only")
	return err
}

func (h *helm) GetParameters(valuesFiles []string) ([]*argoappv1.ComponentParameter, error) {
	out, err := h.helmCmd("inspect", "values", ".")
	if err != nil {
		return nil, err
	}
	values := append([]string{out})
	for _, file := range valuesFiles {
		var fileValues []byte
		parsedURL, err := url.ParseRequestURI(file)
		if err == nil && (parsedURL.Scheme == "http" || parsedURL.Scheme == "https") {
			fileValues, err = config.ReadRemoteFile(file)
		} else {
			fileValues, err = ioutil.ReadFile(path.Join(h.path, file))
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read value file %s: %s", file, err)
		}
		values = append(values, string(fileValues))
	}

	output := map[string]string{}
	for _, file := range values {
		values := map[string]interface{}{}
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

func (h *helm) helmCmd(args ...string) (string, error) {
	cmd := exec.Command("helm", args...)
	cmd.Dir = h.path
	if h.home != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("HELM_HOME=%s", h.home))
	}
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

func flatVals(input map[string]interface{}, output map[string]string, prefixes ...string) {
	for key, val := range input {
		if subMap, ok := val.(map[string]interface{}); ok {
			flatVals(subMap, output, append(prefixes, fmt.Sprintf("%v", key))...)
		} else {
			output[strings.Join(append(prefixes, fmt.Sprintf("%v", key)), ".")] = fmt.Sprintf("%v", val)
		}
	}
}
