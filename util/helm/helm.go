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

	"github.com/argoproj/argo-cd/v3/util/cert"
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
	// RegistryLoginOCI logs into OCI registries for repos that have credentials configured.
	RegistryLoginOCI(ctx context.Context) error
	// Environ returns the environment variables needed for a child process to use
	// the Helm configuration home managed by this instance (e.g. for registry auth
	// and TLS certificates).
	Environ() []string
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

func (h *helm) RegistryLoginOCI(ctx context.Context) error {
	for i := range h.repos {
		repo := h.repos[i]
		if !repo.EnableOci {
			continue
		}
		helmPassword, err := repo.GetPassword()
		if err != nil {
			return fmt.Errorf("failed to get password for helm registry: %w", err)
		}
		if repo.GetUsername() != "" && helmPassword != "" {
			_, err := h.cmd.RegistryLogin(ctx, repo.Repo, repo.Creds, repo.InsecureOCIForceHttp)
			if err != nil {
				return fmt.Errorf("failed to login to registry %s: %w", repo.Repo, err)
			}
		}
	}
	return nil
}

func (h *helm) Environ() []string {
	env := h.cmd.Environ()
	hasOCIRepoWithCustomCA := false
	for i := range h.repos {
		repo := h.repos[i]
		if repo.EnableOci && repo.GetCAPath() != "" {
			hasOCIRepoWithCustomCA = true
		}
	}
	if hasOCIRepoWithCustomCA {
		// Kustomize does not expose Helm's --ca-file option. The configured
		// repository CAs are already mounted in this directory, which Helm's
		// Go TLS client loads through SSL_CERT_DIR.
		env = append(env, "SSL_CERT_DIR="+cert.GetTLSCertificateDataPath())
	}
	return env
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
	var values []string
	// Don't load values.yaml if it's an out-of-bounds link.
	if _, _, err := pathutil.ResolveValueFilePathOrUrl(appPath, repoRoot, "values.yaml", []string{}); err == nil {
		out, err := h.cmd.inspectValues(".")
		if err != nil {
			return nil, fmt.Errorf("failed to execute helm inspect values command: %w", err)
		}
		values = append(values, out)
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

	output := map[string]string{}
	for _, file := range values {
		values := map[string]any{}
		if err := yaml.Unmarshal([]byte(file), &values); err != nil {
			return nil, fmt.Errorf("failed to parse values: %w", err)
		}
		flatVals(values, output)
	}

	return output, nil
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
