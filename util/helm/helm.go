package helm

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os/exec"
	"path"
	"regexp"
	"strings"

	argoexec "github.com/argoproj/pkg/exec"
	"github.com/ghodss/yaml"

	"github.com/argoproj/argo-cd/util/config"
)

type HelmRepository struct {
	Creds
	Name string
	Repo string
}

// Helm provides wrapper functionality around the `helm` command.
type Helm interface {
	// Template returns a list of unstructured objects from a `helm template` command
	Template(opts *TemplateOpts) (string, error)
	// GetParameters returns a list of chart parameters taking into account values in provided YAML files.
	GetParameters(valuesFiles []string) (map[string]string, error)
	// DependencyBuild runs `helm dependency build` to download a chart's dependencies
	DependencyBuild() error
	// Init runs `helm init --client-only`
	Init() error
	// Dispose deletes temp resources
	Dispose()
}

// NewHelmApp create a new wrapper to run commands on the `helm` command-line tool.
func NewHelmApp(workDir string, repos []HelmRepository) (Helm, error) {
	cmd, err := NewCmd(workDir)
	if err != nil {
		return nil, err
	}
	return &helm{repos: repos, cmd: *cmd}, nil
}

type helm struct {
	cmd   Cmd
	repos []HelmRepository
}

// IsMissingDependencyErr tests if the error is related to a missing chart dependency
func IsMissingDependencyErr(err error) bool {
	return strings.Contains(err.Error(), "found in requirements.yaml, but missing in charts")
}

func (h *helm) Template(templateOpts *TemplateOpts) (string, error) {
	out, err := h.cmd.template(".", templateOpts)
	if err != nil {
		return "", err
	}
	return out, nil
}

func (h *helm) DependencyBuild() error {
	for _, repo := range h.repos {
		_, err := h.cmd.RepoAdd(repo.Name, repo.Repo, repo.Creds)

		if err != nil {
			return err
		}
	}
	h.repos = nil
	_, err := h.cmd.dependencyBuild()
	return err
}

func (h *helm) Init() error {
	_, err := h.cmd.Init()
	return err
}

func (h *helm) Dispose() {
	h.cmd.Close()
}

func Version() (string, error) {
	cmd := exec.Command("helm", "version", "--client")
	out, err := argoexec.RunCommandExt(cmd, argoexec.CmdOpts{
		Timeout:  config.CmdOpts().Timeout,
		Redactor: redactor,
	})
	if err != nil {
		return "", fmt.Errorf("could not get helm version: %s", err)
	}
	re := regexp.MustCompile(`SemVer:"([a-zA-Z0-9\.]+)"`)
	matches := re.FindStringSubmatch(out)
	if len(matches) != 2 {
		return "", errors.New("could not get helm version")
	}
	version := matches[1]
	if version[0] != 'v' {
		version = "v" + version
	}
	return strings.TrimSpace(version), nil
}

func (h *helm) GetParameters(valuesFiles []string) (map[string]string, error) {
	out, err := h.cmd.inspectValues(".")
	if err != nil {
		return nil, err
	}
	values := []string{out}
	for _, file := range valuesFiles {
		var fileValues []byte
		parsedURL, err := url.ParseRequestURI(file)
		if err == nil && (parsedURL.Scheme == "http" || parsedURL.Scheme == "https") {
			fileValues, err = config.ReadRemoteFile(file)
		} else {
			fileValues, err = ioutil.ReadFile(path.Join(h.cmd.WorkDir, file))
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

	return output, nil
}

func flatVals(input interface{}, output map[string]string, prefixes ...string) {
	switch i := input.(type) {
	case map[string]interface{}:
		for k, v := range i {
			flatVals(v, output, append(prefixes, k)...)
		}
	case []interface{}:
		for j, v := range i {
			flatVals(v, output, append(prefixes[0:len(prefixes)-1], fmt.Sprintf("%s[%v]", prefixes[len(prefixes)-1], j))...)
		}
	default:
		output[strings.Join(prefixes, ".")] = fmt.Sprintf("%v", i)
	}
}
