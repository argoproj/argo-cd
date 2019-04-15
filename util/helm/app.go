package helm

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/config"
	"github.com/argoproj/argo-cd/util/kube"
)

// App provides wrapper functionality around the `helm` command.
type App interface {
	// Template returns a list of unstructured objects from a `helm template` command
	Template(appName string, namespace string, opts *argoappv1.ApplicationSourceHelm) ([]*unstructured.Unstructured, error)
	// GetParameters returns a list of chart parameters taking into account Values in provided YAML files.
	GetParameters(valuesFiles []string) ([]*argoappv1.HelmParameter, error)
	// DependencyBuild runs `helm dependency build` to download a chart's dependencies
	DependencyBuild() error
	Dispose()
}

// NewHelmApp create a new wrapper to run commands on the `helm` command-line tool.
func NewApp(path string, repos []*argoappv1.Repository) (App, error) {
	cmd, err := newCmd(path)
	if err != nil {
		return nil, err
	}
	_, err = cmd.init()
	return &app{path: path, cmd: *cmd, repos: repos}, err
}

type app struct {
	path             string
	cmd              cmd
	repos            []*argoappv1.Repository
	reposInitialized bool
}

func (h *app) Dispose() {
	h.cmd.Close()
}

// IsMissingDependencyErr tests if the error is related to a missing chart dependency
func IsMissingDependencyErr(err error) bool {
	return strings.Contains(err.Error(), "found in requirements.yaml, but missing in charts")
}

func (h *app) Template(appName string, namespace string, source *argoappv1.ApplicationSourceHelm) ([]*unstructured.Unstructured, error) {

	opts := templateOpts{
		Name:      appName,
		Namespace: namespace,
		Set:       make(map[string]string),
		Values:    []string{},
	}

	if source != nil {
		opts.Values = append(opts.Values, source.ValueFiles...)
		for _, p := range source.Parameters {
			opts.Set[p.Name] = p.Value
		}
	}
	out, err := h.cmd.template(".", opts)
	if err != nil {
		return nil, err
	}
	return kube.SplitYAML(out)
}

func (h *app) DependencyBuild() error {
	if !h.reposInitialized {
		for _, repo := range h.repos {

			if repo.Type != "helm" {
				continue
			}

			_, err := h.cmd.repoAdd(repo.Name, repo.Repo, repoAddOpts{
				Username: repo.Username, Password: repo.Password, CAData: repo.CAData, CertData: repo.CertData, KeyData: repo.KeyData,
			})

			if err != nil {
				return err
			}
		}
	}
	_, err := h.cmd.dependencyBuild()
	return err
}

func (h *app) GetParameters(valuesFiles []string) ([]*argoappv1.HelmParameter, error) {
	out, err := h.cmd.inspectValues(".")
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

	params := make([]*argoappv1.HelmParameter, 0)
	for key, val := range output {
		params = append(params, &argoappv1.HelmParameter{
			Name:  key,
			Value: val,
		})
	}
	sort.Slice(params, func(i, j int) bool {
		return params[i].Name < params[j].Name
	})
	return params, nil
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
