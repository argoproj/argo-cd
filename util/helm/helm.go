package helm

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/util/config"
	executil "github.com/argoproj/argo-cd/v3/util/exec"
	pathutil "github.com/argoproj/argo-cd/v3/util/io/path"
)

const (
	ResourcePolicyAnnotation = "helm.sh/resource-policy"
	ResourcePolicyKeep       = "keep"
)

type HelmRepository struct {
	Creds
	Name                 string
	Repo                 string
	EnableOci            bool
	InsecureOCIForceHttp bool
}

// Helm provides wrapper functionality around the `helm` command.
type Helm interface {
	// Template returns a list of unstructured objects from a `helm template` command
	Template(opts *TemplateOpts) (string, string, error)
	// GetParameters returns a list of chart parameters taking into account values in provided YAML files.
	GetParameters(valuesFiles []pathutil.ResolvedFilePath, appPath, repoRoot string) (map[string]string, error)
	// DependencyBuild runs `helm dependency build` to download a chart's dependencies
	DependencyBuild(ctx context.Context) error
	// Dispose deletes temp resources
	Dispose()
}

// NewHelmApp create a new wrapper to run commands on the `helm` command-line tool.
func NewHelmApp(workDir string, repos []HelmRepository, isLocal bool, version string, proxy string, noProxy string, passCredentials bool, insecure bool) (Helm, error) {
	cmd, err := NewCmd(workDir, version, proxy, noProxy)
	if err != nil {
		return nil, fmt.Errorf("failed to create new helm command: %w", err)
	}
	cmd.IsLocal = isLocal

	return &helm{repos: repos, cmd: *cmd, passCredentials: passCredentials, insecure: insecure}, nil
}

type helm struct {
	cmd             Cmd
	repos           []HelmRepository
	passCredentials bool
	insecure        bool
}

var _ Helm = &helm{}

// IsMissingDependencyErr tests if the error is related to a missing chart dependency
func IsMissingDependencyErr(err error) bool {
	return strings.Contains(err.Error(), "found in requirements.yaml, but missing in charts") ||
		strings.Contains(err.Error(), "found in Chart.yaml, but missing in charts/ directory")
}

func (h *helm) Template(templateOpts *TemplateOpts) (string, string, error) {
	out, command, err := h.cmd.template(".", templateOpts)
	if err != nil {
		return "", command, fmt.Errorf("failed to execute helm template command: %w", err)
	}
	return out, command, nil
}

func (h *helm) DependencyBuild(ctx context.Context) error {
	isHelmOci := h.cmd.IsHelmOci
	defer func() {
		h.cmd.IsHelmOci = isHelmOci
	}()

	for i := range h.repos {
		repo := h.repos[i]
		if repo.EnableOci {
			h.cmd.IsHelmOci = true
			helmPassword, err := repo.GetPassword()
			if err != nil {
				return fmt.Errorf("failed to get password for helm registry: %w", err)
			}
			if repo.GetUsername() != "" && helmPassword != "" {
				_, err := h.cmd.RegistryLogin(ctx, repo.Repo, repo.Creds, repo.InsecureOCIForceHttp)

				defer func() {
					_, _ = h.cmd.RegistryLogout(repo.Repo, repo.Creds)
				}()

				if err != nil {
					return fmt.Errorf("failed to login to registry %s: %w", repo.Repo, err)
				}
			}
		} else {
			_, err := h.cmd.RepoAdd(repo.Name, repo.Repo, repo.Creds, h.passCredentials)
			if err != nil {
				return fmt.Errorf("failed to add helm repository %s: %w", repo.Repo, err)
			}
		}
	}

	// Check if any dependent repository has insecureOCIForceHttp set to true
	// If so, set the command to use plain HTTP, as helm dependency build command doesn't support mixed TLS and non TLS dependencies
	// Please note that the fact we logged in earlier to each dependent repo with it's own InsecureOCIForceHttp either enabled or disabled
	// is unrelated to how we perform helm dependency build, which does not have an option to set --plain-http per repo
	plainHTTP := false
	for i := range h.repos {
		if h.repos[i].InsecureOCIForceHttp {
			plainHTTP = true
			break
		}
	}
	h.repos = nil
	_, err := h.cmd.dependencyBuild(h.insecure, plainHTTP)
	if err != nil {
		return fmt.Errorf("failed to build helm dependencies: %w", err)
	}
	return nil
}

func (h *helm) Dispose() {
	h.cmd.Close()
}

func Version() (string, error) {
	cmd := exec.CommandContext(context.Background(), "helm", "version", "--short")
	// example version output for helm v3 and higher:
	// short: "v3.3.1+g249e521"
	version, err := executil.RunWithRedactor(cmd, redactor)
	if err != nil {
		return "", fmt.Errorf("could not get helm version: %w", err)
	}
	return strings.TrimSpace(version), nil
}

func (h *helm) GetParameters(valuesFiles []pathutil.ResolvedFilePath, appPath, repoRoot string) (map[string]string, error) {
	var defaultValues string
	var values []string
	// Don't load values.yaml if it's an out-of-bounds link.
	if _, _, err := pathutil.ResolveValueFilePathOrUrl(appPath, repoRoot, "values.yaml", []string{}); err == nil {
		out, err := h.cmd.inspectValues(".")
		if err != nil {
			return nil, fmt.Errorf("failed to execute helm inspect values command: %w", err)
		}
		defaultValues = out
	} else {
		log.Warnf("Values file %s is not allowed: %v", filepath.Join(appPath, "values.yaml"), err)
	}
	for i := range valuesFiles {
		file := string(valuesFiles[i])
		var fileValues []byte
		parsedURL, err := url.ParseRequestURI(file)
		if err == nil && (parsedURL.Scheme == "http" || parsedURL.Scheme == "https") {
			fileValues, err = config.ReadRemoteFile(file)
		} else {
			_, fileReadErr := os.Stat(file)
			if os.IsNotExist(fileReadErr) {
				log.Debugf("File not found %s", file)
				continue
			}
			if errors.Is(fileReadErr, os.ErrPermission) {
				log.Debugf("File does not have permissions %s", file)
				continue
			}
			fileValues, err = os.ReadFile(file)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read value file %s: %w", file, err)
		}
		values = append(values, string(fileValues))
	}

	// Helm merges the supplied values files first and then coalesces the result with the
	// chart defaults, so a null or a replaced map in an explicit file never hides defaults
	// that a later file does not override.
	explicitValues := map[string]any{}
	for _, file := range values {
		parsed := map[string]any{}
		if err := yaml.Unmarshal([]byte(file), &parsed); err != nil {
			return nil, fmt.Errorf("failed to parse values: %w", err)
		}
		mergeValues(explicitValues, parsed)
	}

	mergedValues := map[string]any{}
	if err := yaml.Unmarshal([]byte(defaultValues), &mergedValues); err != nil {
		return nil, fmt.Errorf("failed to parse values: %w", err)
	}
	if mergedValues == nil {
		mergedValues = map[string]any{}
	}
	coalesceValues(mergedValues, explicitValues)

	output := map[string]string{}
	flatVals(mergedValues, output)
	return output, nil
}

// mergeValues merges maps recursively while replacing arrays and scalar values,
// matching Helm's value-file precedence before parameter names are flattened.
func mergeValues(dst, src map[string]any) {
	for key, value := range src {
		if incoming, ok := value.(map[string]any); ok {
			if existing, ok := dst[key].(map[string]any); ok {
				mergeValues(existing, incoming)
				continue
			}
		}
		dst[key] = value
	}
}

// coalesceValues overlays the explicit values onto the chart defaults: maps merge
// recursively, a null removes the default key, and any other value replaces it.
func coalesceValues(defaults, explicit map[string]any) {
	for key, value := range explicit {
		switch v := value.(type) {
		case nil:
			delete(defaults, key)
		case map[string]any:
			if existing, ok := defaults[key].(map[string]any); ok {
				coalesceValues(existing, v)
				continue
			}
			defaults[key] = v
		default:
			defaults[key] = value
		}
	}
}

func flatVals(input any, output map[string]string, prefixes ...string) {
	switch i := input.(type) {
	case map[string]any:
		for k, v := range i {
			flatVals(v, output, append(prefixes, k)...)
		}
	case []any:
		p := append([]string(nil), prefixes...)
		for j, v := range i {
			flatVals(v, output, append(p[0:len(p)-1], fmt.Sprintf("%s[%v]", prefixes[len(p)-1], j))...)
		}
	default:
		output[strings.Join(prefixes, ".")] = fmt.Sprintf("%v", i)
	}
}
