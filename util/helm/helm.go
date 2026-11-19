package helm

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/util/config"
	cryptoutil "github.com/argoproj/argo-cd/v3/util/crypto"
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

// scopedIndexIdentity is shared by the disk provider and Redis clients. URL
// path escaping remains significant. Queries, URL credentials and transport
// credentials are hashed, so they cannot appear as plaintext in cache keys.
func scopedIndexIdentity(repoURL, scope, proxy, noProxy string) string {
	identity, _ := scopedIndexIdentityContext(context.Background(), repoURL, scope, proxy, noProxy)
	return identity
}

func scopedIndexIdentityContext(ctx context.Context, repoURL, scope, proxy, noProxy string) (string, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		identity, err := cryptoutil.CredentialScopeContext(ctx, "helm-invalid-repository", repoURL, scope, proxy, noProxy)
		return "invalid:" + identity, err
	}
	userinfo := ""
	if u.User != nil {
		userinfo = u.User.String()
		u.User = nil
	}
	query := u.RawQuery
	u.RawQuery = ""
	u.Fragment = ""
	if u.Scheme == "http" || u.Scheme == "https" {
		u.Host = strings.ToLower(u.Host)
		if u.RawPath == "" {
			u.Path = strings.TrimSuffix(u.Path, "/")
		}
	}
	if proxy != "" || noProxy != "" || userinfo != "" || query != "" {
		scope, err = cryptoutil.CredentialScopeContext(ctx, "helm-repository-transport", scope, proxy, noProxy, userinfo, query)
		if err != nil {
			return "", err
		}
	}
	return u.String() + "\x00" + scope, nil
}

// credsScope returns a memory-hard fingerprint of the effective authentication and
// TLS transport context of a credential view. It scopes index-cache keys so
// that a fetch made with private credentials cannot publish cache entries
// that an anonymous (or differently-credentialed) reader would consume.
// Plaintext secrets never enter the fingerprint output.
func credsScope(creds Creds) string {
	scope, _ := credsScopeContext(context.Background(), creds)
	return scope
}

func credsScopeContext(ctx context.Context, creds Creds) (string, error) {
	// Resolve through the snapshot so the fingerprint uses the SAME resolved
	// values as the HTTP request of the same operation — a rotating provider
	// (Azure workload identity) must not produce a scope from token A while
	// the request authenticates with token B. On resolution failure, fall
	// back to the getters' best effort (auth paths propagate the error; the
	// scope only degrades to a still-deterministic fingerprint).
	snap, err := snapshotCreds(creds)
	if err == nil {
		creds = snap
	}
	password, _ := creds.GetPassword()
	// Include inherited proxy policy because shared Redis and disk
	// records can outlive a process or be read by differently configured pods.
	caData := ""
	if snapshot, ok := creds.(credsSnapshot); ok {
		caData = string(snapshot.caData)
	}
	return cryptoutil.CredentialScopeContext(ctx, "helm-credentials-v2",
		creds.GetUsername(), password, creds.GetCAPath(), string(creds.GetCertData()), string(creds.GetKeyData()),
		caData,
		strconv.FormatBool(creds.GetInsecureSkipVerify()),
		os.Getenv("HTTP_PROXY"), os.Getenv("http_proxy"),
		os.Getenv("HTTPS_PROXY"), os.Getenv("https_proxy"),
		os.Getenv("NO_PROXY"), os.Getenv("no_proxy"), os.Getenv("REQUEST_METHOD"),
	)
}
