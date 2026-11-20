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
	"time"

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
	// forceRefresh bypasses freshness and validators when a build executes.
	// The resulting raw index still populates the isolated Helm snapshot.
	forceRefresh bool
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
	if useStore {
		if err := h.validateIndexRepositoryViews(ctx); err != nil {
			return err
		}
	}
	indexes := make(map[string][]byte)
	fetched := make(map[string]bool)
	clients := make(map[string]*nativeHelmChart)
	aliases := make(map[string]string)
	for _, repo := range h.repos {
		aliases[repo.Name] = repo.Repo
	}
	configurations := make(map[string]helmRepoEntry)
	type preparedIndex struct {
		url, name string
		identity  string
		body      []byte
	}
	var prepared []preparedIndex
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
		if snap.caPath != "" && len(snap.caData) == 0 {
			return errors.New("configured helm CA file is empty")
		}
		repo.Creds = snap
		scope, err := credsScopeContext(ctx, snap)
		if err != nil {
			return err
		}
		identity, err := scopedIndexIdentityContext(ctx, repo.Repo, scope, h.cmd.proxy, h.cmd.noProxy)
		if err != nil {
			return err
		}
		data, ok := indexes[identity]
		if !ok {
			client := h.nativeChartClientWithCreds(repo, snap, scope)
			clients[identity] = client
			// Bound each store fetch so one slow origin cannot hold the whole
			// dependency build (and the per-identity lock) indefinitely.
			fetchContext, cancelFetch := indexFetchContext(ctx)
			if h.forceRefresh {
				data, err = h.cmd.persistentIndexStore.ForceRefresh(fetchContext, identity, client)
				fetched[identity] = true
			} else {
				data, fetched[identity], err = h.cmd.persistentIndexStore.revalidate(fetchContext, identity, client, false)
			}
			cancelFetch()
			if err != nil {
				return fmt.Errorf("%s is not a valid chart repository or cannot be reached: %w", repo.Repo, err)
			}
			indexes[identity] = data
		}
		if err := h.cmd.prewarmIndexes(repo.Repo, data, repo.Name); err != nil {
			return fmt.Errorf("prepare isolated helm index for %s: %w", repo.Repo, err)
		}
		prepared = append(prepared, preparedIndex{url: repo.Repo, name: repo.Name, identity: identity, body: data})
		if h.cmd.persistentIndexStore.FilterEnabled {
			// Private snapshot only: disk/shared records keep the full index.
			// Group dependency names under the SAME scoped identity the store
			// lookup uses — dependencyNamesByRepository is credential-blind,
			// so its unscoped keys must be translated here; matching raw
			// unscoped identities against scoped ones silently disabled the
			// whole filter.
			allNames, err := dependencyNamesByRepository(ctx, h.cmd.WorkDir, h.repos)
			if err != nil {
				return fmt.Errorf("collect helm dependencies for %s: %w", repo.Repo, err)
			}
			// The selection map is keyed WITHOUT proxy settings, so the
			// lookup-side translation must strip them too: keeping proxy in
			// only one side of the comparison re-splits the keyspace and
			// silently disables filtering for explicitly proxied repos.
			unscopedIdentity, err := scopedIndexIdentityContext(ctx, repo.Repo, "", "", "")
			if err != nil {
				return fmt.Errorf("resolve dependency repository identity for %s: %w", repo.Repo, err)
			}
			if wanted := allNames[unscopedIdentity]; len(wanted) > 0 {
				filtered, replaced, err := dependencyIndexes.snapshot(data, wanted)
				switch {
				case err != nil:
					// Any projection failure (dependency absent from the
					// cached index, malformed or rejected projection body)
					// keeps the FULL index installed: the build must behave
					// exactly like a repo without filtering. Revalidation
					// (freshness expiry or the version-miss fallback) picks
					// the chart up on a later build.
					log.WithError(err).Debug("falling back to full helm index after snapshot failure")
				case replaced:
					if err := h.cmd.installIndexes(repo.Repo, filtered, repo.Name, true); err != nil {
						return fmt.Errorf("install filtered helm index for %s: %w", repo.Repo, err)
					}
				}
			}
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
			configuration.Name = repo.Name
			repoEntries = append(repoEntries, configuration)
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
	filteredSnapshot := useStore && h.cmd.persistentIndexStore.FilterEnabled
	if err != nil && filteredSnapshot && isIndexSnapshotError(err) {
		// A filtered snapshot can miss for three different reasons: the
		// projection dropped a version or a WHOLE CHART Helm selects
		// differently (snapshot error), or the snapshot was rendered from a
		// STALE index while the origin published a new one (version-lookup
		// error, including newly added dependencies absent from the cached
		// index). The first needs only the full original; the others need
		// revalidation, otherwise the fallback would retry the same stale
		// bytes forever. Revalidate EVERY miss against the origin and
		// publish the fresh body BEFORE restoring full indexes, so the
		// retry cannot repeat the stale miss. A chart absent from the cached
		// index revalidates to the same absent body — the retry then fails
		// again with the original error, which is the correct outcome for a
		// dependency the origin genuinely does not serve.
		revalidated := make(map[string][]byte)
		for _, index := range prepared {
			if fetched[index.identity] || revalidated[index.identity] != nil || !missingDependencyVersion(h.cmd.WorkDir, index.url, aliases, index.body) {
				continue
			}
			body, _, refreshErr := h.cmd.persistentIndexStore.revalidate(ctx, index.identity, clients[index.identity], true)
			if refreshErr != nil {
				return fmt.Errorf("revalidate Helm index after filtered snapshot version miss (%w): %w", err, refreshErr)
			}
			revalidated[index.identity] = body
		}
		for _, index := range prepared {
			// Publish the revalidated body when available; the stale snapshot
			// bytes (index.body) are only a last resort for identities the
			// revalidation pass could not refresh.
			body := revalidated[index.identity]
			if body == nil {
				body = index.body
			}
			if installErr := h.cmd.installIndexes(index.url, body, index.name, true); installErr != nil {
				return fmt.Errorf("restore full Helm index after filtered snapshot failure (%w): %w", err, installErr)
			}
		}
		if _, fallbackErr := h.cmd.dependencyBuild(ctx, h.insecure, plainHTTP, useStore); fallbackErr != nil {
			return fmt.Errorf("Helm index lookup failed (%w); full-index fallback failed: %w", err, fallbackErr)
		}
		return nil
	}
	refreshed := make(map[string][]byte)
	if err != nil && useStore && !filteredSnapshot && isIndexVersionLookupError(err) {
		for _, index := range prepared {
			if fetched[index.identity] || refreshed[index.identity] != nil || !missingDependencyVersion(h.cmd.WorkDir, index.url, aliases, index.body) {
				continue
			}
			body, _, refreshErr := h.cmd.persistentIndexStore.revalidate(ctx, index.identity, clients[index.identity], true)
			if refreshErr != nil {
				return fmt.Errorf("revalidate Helm index after version lookup miss: %w", refreshErr)
			}
			refreshed[index.identity] = body
		}
	}
	if len(refreshed) > 0 {
		// Retry only index lookup/load failures. Origin validation and archive
		// authentication/network failures must keep their normal failure path.
		log.WithError(err).Warn("retrying Helm dependency build with revalidated indexes")
		for _, index := range prepared {
			if body := refreshed[index.identity]; body != nil {
				if installErr := h.cmd.installIndexes(index.url, body, index.name, true); installErr != nil {
					return fmt.Errorf("restore revalidated Helm index after version lookup failure (%w): %w", err, installErr)
				}
			}
		}
		if _, fallbackErr := h.cmd.dependencyBuild(ctx, h.insecure, plainHTTP, useStore); fallbackErr != nil {
			return fmt.Errorf("Helm index lookup failed (%w); revalidated-index fallback failed: %w", err, fallbackErr)
		}
		return nil
	}
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

// Getter plugins must retain Helm's complete registration/refresh path.
// Classify the whole build before writing any part of the isolated config.
func (h *helm) supportsIndexPrewarm() bool {
	registered := make(map[string]struct{})
	names := make(map[string]string)
	for _, repo := range h.repos {
		if repo.EnableOci {
			if os.Getenv("HELM_REGISTRY_CONFIG") != "" {
				return false
			}
			continue
		}
		u, err := url.Parse(repo.Repo)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return false
		}
		registered[helmRepositoryURLKey(repo.Repo)] = struct{}{}
		if repo.Name != "" {
			if owner, exists := names[repo.Name]; exists && owner != helmRepositoryURLKey(repo.Repo) {
				return false
			}
			names[repo.Name] = helmRepositoryURLKey(repo.Repo)
		}
	}
	// Generated snapshots and configured names share one Helm cache directory.
	// Resolve collisions before installing any bytes or private configuration.
	for _, repo := range h.repos {
		if !repo.EnableOci {
			if owner, exists := names[managerKeyPrefix+managerKey(repo.Repo)]; exists && owner != helmRepositoryURLKey(repo.Repo) {
				return false
			}
		}
	}
	var chart struct {
		Dependencies []struct {
			Repository string
		}
	}
	for _, name := range []string{"Chart.yaml", "requirements.yaml"} {
		data, err := ReadChartMetadata(h.cmd.WorkDir, name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return false
		}
		if yaml.Unmarshal(data, &chart) != nil {
			return false
		}
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
		case "http", "https":
			if _, installed := registered[helmRepositoryURLKey(repository)]; !installed {
				return false
			}
		case "oci":
			if os.Getenv("HELM_REGISTRY_CONFIG") != "" {
				return false
			}
		case "file":
		default:
			return false
		}
	}
	return true
}

// Match the pinned Helm binary's urlutil.Equal, including path cleaning.
// Other URL components retain their original spelling and escaping.
func helmRepositoryURLKey(repository string) string {
	u, err := url.Parse(repository)
	if err != nil {
		return filepath.Clean(repository)
	}
	if u.Path == "" {
		u.Path = "/"
	}
	u.Path = filepath.Clean(u.Path)
	return u.String()
}

// Helm resolves aliases to URLs for archive lookup, so distinct credential
// views of one URL cannot remain isolated inside a single dependency build.
func (h *helm) validateIndexRepositoryViews(ctx context.Context) error {
	type view struct {
		name, scope string
	}
	views := make(map[string]view)
	for i := range h.repos {
		repo := &h.repos[i]
		if repo.EnableOci {
			continue
		}
		snapshot, err := snapshotCreds(repo.Creds)
		if err != nil {
			return err
		}
		repo.Creds = snapshot
		scope, err := credsScopeContext(ctx, snapshot)
		if err != nil {
			return err
		}
		key := helmRepositoryURLKey(repo.Repo)
		if previous, exists := views[key]; exists && previous.scope != scope {
			return fmt.Errorf("Helm repositories %q and %q use the same URL with different credentials; configure distinct repository URLs", previous.name, repo.Name)
		}
		views[key] = view{repo.Name, scope}
	}
	return nil
}

// buildRepoEntries returns the repositories.yaml entries for one warmed
// repository. Helm resolves registered URLs through their configured names;
// adding a shared generated name would collide for distinct credential views.
func buildRepoEntries(h *helm, repo HelmRepository) ([]helmRepoEntry, error) {
	if snapshot, ok := repo.Creds.(credsSnapshot); ok && snapshot.caPath != "" && len(snapshot.caData) == 0 {
		return nil, errors.New("configured helm CA file is empty")
	}
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
	caFile := repo.GetCAPath()
	if snapshot, ok := repo.Creds.(credsSnapshot); ok && snapshot.caPath != "" {
		caFile, err = h.cmd.writeRepoCredential(snapshot.caData)
		if err != nil {
			return nil, err
		}
	}
	entry := helmRepoEntry{
		Name:                  repo.Name,
		URL:                   repo.Repo,
		Username:              repo.GetUsername(),
		CAFile:                caFile,
		CertFile:              certFile,
		KeyFile:               keyFile,
		InsecureSkipTLSverify: repo.GetInsecureSkipVerify(),
		PassCredentialsAll:    h.passCredentials,
		Password:              password,
	}
	return []helmRepoEntry{entry}, nil
}

// nativeChartClientWithCreds builds a chart client pinned to an ALREADY
// RESOLVED credential snapshot: the DependencyBuild disk identity and the
// HTTP fetch must use the same resolved values even when the underlying
// provider rotates tokens between calls.
func (h *helm) nativeChartClientWithCreds(repo HelmRepository, creds Creds, scope string) *nativeHelmChart {
	client := NewClientWithLock(repo.Repo, creds, globalLock, false, h.cmd.proxy, h.cmd.noProxy).(*nativeHelmChart)
	client.indexCacheScope = scope
	return client
}

// indexFetchContext bounds one store fetch (origin request + publication) by
// a fixed budget, never extending past the caller's own deadline. Dependency
// builds have generous timeouts, but a hung origin must not consume them all
// on the first repository.
func indexFetchContext(ctx context.Context) (context.Context, context.CancelFunc) {
	const indexFetchBudget = 30 * time.Second
	budget := indexFetchBudget
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < budget {
			budget = remaining
		}
	}
	return context.WithTimeout(ctx, budget)
}
