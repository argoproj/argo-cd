package helm

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/config"
	"github.com/argoproj/argo-cd/util/kube"
)

// Helm provides wrapper functionality around the `helm` command.
type Helm interface {
	// Template returns a list of unstructured objects from a `helm template` command
	Template(appName string, namespace string, opts *argoappv1.ApplicationSourceHelm) ([]*unstructured.Unstructured, error)
	// GetParameters returns a list of chart parameters taking into account values in provided YAML files.
	GetParameters(valuesFiles []string) ([]*argoappv1.HelmParameter, error)
	// DependencyBuild runs `helm dependency build` to download a chart's dependencies
	DependencyBuild() error
	// Init runs `helm init --client-only`
	Init() error
	// Dispose deletes temp resources
	Dispose()
}

// NewHelmApp create a new wrapper to run commands on the `helm` command-line tool.
func NewHelmApp(workDir string, repos argoappv1.Repositories) (Helm, error) {
	cmd, err := newCmd(workDir)
	if err != nil {
		return nil, err
	}
	return &helm{repos: &repos, cmd: *cmd}, nil
}

type helm struct {
	cmd   cmd
	repos *argoappv1.Repositories
}

// IsMissingDependencyErr tests if the error is related to a missing chart dependency
func IsMissingDependencyErr(err error) bool {
	return strings.Contains(err.Error(), "found in requirements.yaml, but missing in charts")
}

func (h *helm) Template(appName string, namespace string, opts *argoappv1.ApplicationSourceHelm) ([]*unstructured.Unstructured, error) {
	templateOpts := templateOpts{
		set: map[string]string{},
	}
	templateOpts.namespace = namespace
	if opts != nil {
		templateOpts.name = opts.ReleaseName
		templateOpts.values = opts.ValueFiles
		for _, p := range opts.Parameters {
			templateOpts.set[p.Name] = p.Value
		}
	}
	if templateOpts.name == "" {
		templateOpts.name = appName
	}

	out, err := h.cmd.template(".", templateOpts)
	if err != nil {
		return nil, err
	}
	return kube.SplitYAML(out)
}

func (h *helm) DependencyBuild() error {
	if h.repos != nil {
		for _, repo := range h.repos.Filter(func(r *argoappv1.Repository) bool { return r.Type == "helm" }) {
			_, err := h.cmd.repoAdd(repo.Name, repo.Repo, repoAddOpts{
				username: repo.Username,
				password: repo.Password,
				caData:   repo.CAData,
				certData: repo.CertData,
				keyData:  repo.KeyData,
			})

			if err != nil {
				return err
			}
		}
		h.repos = nil
	}
	_, err := h.cmd.dependencyBuild()
	return err
}

func (h *helm) Init() error {
	_, err := h.cmd.init()
	return err
}

func (h *helm) Dispose() {
	h.cmd.Close()
}

func (h *helm) GetParameters(valuesFiles []string) ([]*argoappv1.HelmParameter, error) {
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
			fileValues, err = ioutil.ReadFile(path.Join(h.cmd.workDir, file))
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

func cleanHelmParameters(params []string) {
	re := regexp.MustCompile(`([^\\]),`)
	for i, param := range params {
		params[i] = re.ReplaceAllString(param, `$1\,`)
	}
}
