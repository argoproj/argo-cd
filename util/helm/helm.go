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

// NewHelmAppWithIndexStore creates a new helm wrapper like NewHelmApp, and
// attaches a raw-index provider. Validated original bytes are copied into the
// private Helm home; Helm never reads or writes the persistent directory.
func NewHelmAppWithIndexStore(workDir string, repos []HelmRepository, isLocal bool, version string, proxy string, noProxy string, passCredentials bool, insecure bool, indexStore *IndexStore, forceRefresh bool) (Helm, error) {
	h, err := NewHelmApp(workDir, repos, isLocal, version, proxy, noProxy, passCredentials, insecure)
	if err != nil {
		return nil, err
	}
	if indexStore != nil && indexStore.dir != "" && h.(*helm).supportsIndexPrewarm() {
		h.(*helm).cmd.persistentIndexStore = indexStore
		h.(*helm).forceRefresh = forceRefresh
	}
	return h, nil
}

type helm struct {
	cmd             Cmd
	repos           []HelmRepository
	passCredentials bool
	insecure        bool
	// forceRefresh bypasses freshness and validators when a build executes.
	// The resulting raw index still populates the isolated Helm snapshot.
	forceRefresh bool
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

	useStore := h.cmd.persistentIndexStore != nil && h.cmd.persistentIndexStore.dir != "" && h.supportsIndexPrewarm()
	if !useStore {
		h.cmd.persistentIndexStore = nil
	}
	indexes := make(map[string][]byte)
	configurations := make(map[string]helmRepoEntry)
	var entries []helmRepoEntry
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
			continue
		}
		if !useStore {
			if _, err := h.cmd.RepoAdd(repo.Name, repo.Repo, repo.Creds, h.passCredentials); err != nil {
				return fmt.Errorf("failed to add helm repository %s: %w", repo.Repo, err)
			}
			continue
		}
		if repo.Name == "" {
			repo.Name = managerKeyPrefix + managerKey(repo.Repo)
		}
		snap, err := snapshotCreds(repo.Creds)
		if err != nil {
			return fmt.Errorf("resolve credentials for helm repository %s: %w", repo.Repo, err)
		}
		repo.Creds = snap
		identity := scopedIndexIdentity(repo.Repo, credsScope(snap), h.cmd.proxy, h.cmd.noProxy)
		data, ok := indexes[identity]
		if !ok {
			client := h.nativeChartClientWithCreds(repo, snap)
			if h.forceRefresh {
				data, err = h.cmd.persistentIndexStore.ForceRefresh(ctx, identity, client)
			} else {
				data, err = h.cmd.persistentIndexStore.Revalidate(ctx, identity, client)
			}
			if err != nil {
				return fmt.Errorf("%s is not a valid chart repository or cannot be reached: %w", repo.Repo, err)
			}
			indexes[identity] = data
		}
		if err := h.cmd.prewarmIndexes(repo.Repo, data, repo.Name); err != nil {
			return fmt.Errorf("prepare isolated helm index for %s: %w", repo.Repo, err)
		}
		configuration, exists := configurations[identity]
		var repoEntries []helmRepoEntry
		if !exists {
			repoEntries, err = buildRepoEntries(h, repo)
			if err != nil {
				return err
			}
			configurations[identity] = repoEntries[0]
		} else {
			configuration.URL = repo.Repo
			for _, name := range []string{repo.Name, managerKeyPrefix + managerKey(repo.Repo)} {
				entry := configuration
				entry.Name = name
				repoEntries = append(repoEntries, entry)
			}
		}
		entries = append(entries, repoEntries...)
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
	// Persist any directly-written repo entries before the dependency build
	// reads repositories.yaml.
	if len(entries) > 0 {
		if err := h.cmd.writeRepoEntries(entries); err != nil {
			// Without the entries file, helm cannot resolve versions from the
			// prewarmed indexes; --skip-refresh would fail the build. But the
			// entries were only written when ALL repos were warm, and if the
			// write fails after warm reads, safest is to error clearly.
			return fmt.Errorf("failed to write helm repositories file: %w", err)
		}
	}
	_, err := h.cmd.dependencyBuild(ctx, h.insecure, plainHTTP, useStore)
	if err != nil {
		return fmt.Errorf("failed to build helm dependencies: %w", err)
	}

	return nil
}

// Getter plugins must retain Helm's complete registration/refresh path.
// Classify the whole build before writing any part of the isolated config.
func (h *helm) supportsIndexPrewarm() bool {
	for _, repo := range h.repos {
		if repo.EnableOci {
			continue
		}
		u, err := url.Parse(repo.Repo)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return false
		}
	}
	for _, name := range []string{"Chart.yaml", "requirements.yaml"} {
		data, err := os.ReadFile(filepath.Join(h.cmd.WorkDir, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return false
		}
		var chart struct{ Dependencies []struct{ Repository string } }
		if yaml.Unmarshal(data, &chart) != nil {
			return false
		}
		for _, dependency := range chart.Dependencies {
			repository := dependency.Repository
			if repository == "" || strings.HasPrefix(repository, "@") || strings.HasPrefix(repository, "alias:") {
				continue
			}
			u, err := url.Parse(repository)
			if err != nil {
				return false
			}
			switch u.Scheme {
			case "http", "https", "file", "oci":
			default:
				return false
			}
		}
	}
	return true
}

// buildRepoEntries returns the repositories.yaml entries for one warmed
// repository: under the user-facing name and under helm's generated
// helm-manager-<sha> name (the name helm's dependency manager resolves
// unmanaged dependencies by).
func buildRepoEntries(h *helm, repo HelmRepository) ([]helmRepoEntry, error) {
	password, err := repo.GetPassword()
	if err != nil {
		return nil, err
	}
	certFile, err := h.cmd.writeRepoCredential(repo.GetCertData())
	if err != nil {
		return nil, err
	}
	keyFile, err := h.cmd.writeRepoCredential(repo.GetKeyData())
	if err != nil {
		return nil, err
	}
	managerName := managerKeyPrefix + managerKey(repo.Repo)
	result := make([]helmRepoEntry, 0, 2)
	for _, name := range []string{repo.Name, managerName} {
		entry := helmRepoEntry{
			Name:                  name,
			URL:                   repo.Repo,
			Username:              repo.GetUsername(),
			CAFile:                repo.GetCAPath(),
			CertFile:              certFile,
			KeyFile:               keyFile,
			InsecureSkipTLSverify: repo.GetInsecureSkipVerify(),
			PassCredentialsAll:    h.passCredentials,
		}
		entry.Password = password
		result = append(result, entry)
	}
	return result, nil
}

// repoIdentity returns the persistent-store cache identity for a repository:
// the URL scoped by a fingerprint of the effective authentication and TLS
// transport context. Two views of the same URL with different credentials
// therefore never share a cached index. Credential fingerprints use a
// memory-hard derivation; plaintext secrets never enter keys or paths.
func repoIdentity(repo HelmRepository) string {
	return scopedIndexIdentity(repo.Repo, credsScope(repo.Creds), "", "")
}

// scopedIndexIdentity is shared by the disk provider and Redis clients. URL
// path escaping remains significant. Queries, URL credentials and transport
// credentials are hashed, so they cannot appear as plaintext in cache keys.
func scopedIndexIdentity(repoURL, scope, proxy, noProxy string) string {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "invalid:" + cryptoutil.CredentialScope("helm-invalid-repository", repoURL, scope, proxy, noProxy)
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
			u.Path = strings.TrimRight(u.Path, "/")
		}
	}
	if proxy != "" || noProxy != "" || userinfo != "" || query != "" {
		scope = cryptoutil.CredentialScope("helm-repository-transport", scope, proxy, noProxy, userinfo, query)
	}
	return u.String() + "\x00" + scope
}

// CredsScope exposes the credential-view fingerprint used to scope index-cache
// keys (WithIndexCacheScopedByCreds). Exported for tests and callers that
// must construct the same scoped keys the client uses.
func CredsScope(creds Creds) string {
	return credsScope(creds)
}

// credsScope returns a memory-hard fingerprint of the effective authentication and
// TLS transport context of a credential view. It scopes index-cache keys so
// that a fetch made with private credentials cannot publish cache entries
// that an anonymous (or differently-credentialed) reader would consume.
// Plaintext secrets never enter the fingerprint output.
func credsScope(creds Creds) string {
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
	return cryptoutil.CredentialScope("helm-credentials",
		creds.GetUsername(), password, creds.GetCAPath(), string(creds.GetCertData()), string(creds.GetKeyData()),
		strconv.FormatBool(creds.GetInsecureSkipVerify()),
		os.Getenv("HTTP_PROXY"), os.Getenv("http_proxy"),
		os.Getenv("HTTPS_PROXY"), os.Getenv("https_proxy"),
		os.Getenv("NO_PROXY"), os.Getenv("no_proxy"), os.Getenv("REQUEST_METHOD"),
	)
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

// nativeChartClient builds a chart client for Go-side index revalidation of
// the given repository (conditional GET against the origin with the creds the
// build itself would use).
// nativeChartClientWithCreds builds a chart client pinned to an ALREADY
// RESOLVED credential snapshot: the DependencyBuild disk identity and the
// HTTP fetch must use the same resolved values even when the underlying
// provider rotates tokens between calls.
func (h *helm) nativeChartClientWithCreds(repo HelmRepository, creds Creds) *nativeHelmChart {
	return NewClientWithLock(repo.Repo, creds, globalLock, false, h.cmd.proxy, h.cmd.noProxy).(*nativeHelmChart)
}
