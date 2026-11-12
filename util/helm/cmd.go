package helm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/common"
	executil "github.com/argoproj/argo-cd/v3/util/exec"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	pathutil "github.com/argoproj/argo-cd/v3/util/io/path"
	"github.com/argoproj/argo-cd/v3/util/proxy"
)

// A thin wrapper around the "helm" command, adding logging and error translation.
type Cmd struct {
	helmHome        string
	WorkDir         string
	IsLocal         bool
	IsHelmOci       bool
	proxy           string
	noProxy         string
	runWithRedactor func(cmd *exec.Cmd, redactor func(text string) string) (string, error)
	// persistentIndexStore, when non-nil, holds a persistent, expiring copy of
	// chart repository index files. It is copied into the ephemeral helm cache
	// home before each invocation so that helm resolves chart versions locally
	// instead of re-downloading index.yaml (see IndexStore).
	persistentIndexStore *IndexStore
}

func NewCmd(workDir string, version string, proxy string, noProxy string) (*Cmd, error) {
	switch version {
	// There was a point in time when we supported both v2 and v3, and at some point we stopped
	// supporting v2.
	// We now support v4 only, but from Helm docs v4 is backwards compatible with v3 charts.
	// Removing the v3 value would break existing applications that specify v3.
	// There is also no reason for users to specify v4 explicitly, but in case someone does it, we support the value.
	// TODO: Remove support for v3 value in a major release.
	case "", "v3", "v4":
		return NewCmdWithVersion(workDir, false, proxy, noProxy)
	}
	return nil, fmt.Errorf("helm version '%s' is not supported", version)
}

func NewCmdWithVersion(workDir string, isHelmOci bool, proxy string, noProxy string) (*Cmd, error) {
	return newCmdWithVersion(workDir, isHelmOci, proxy, noProxy, executil.RunWithRedactor)
}

func newCmdWithVersion(workDir string, isHelmOci bool, proxy string, noProxy string, runWithRedactor func(cmd *exec.Cmd, redactor func(text string) string) (string, error)) (*Cmd, error) {
	tmpDir, err := os.MkdirTemp("", "helm")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory for helm: %w", err)
	}
	return &Cmd{WorkDir: workDir, helmHome: tmpDir, IsHelmOci: isHelmOci, proxy: proxy, noProxy: noProxy, runWithRedactor: runWithRedactor}, err
}

var redactor = func(text string) string {
	return regexp.MustCompile("(--username|--password) [^ ]*").ReplaceAllString(text, "$1 ******")
}

func (c Cmd) run(ctx context.Context, args ...string) (string, string, error) {
	return c.runWithStdin(ctx, nil, args...)
}

func (c Cmd) runWithStdin(ctx context.Context, stdin io.Reader, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Dir = c.WorkDir
	cmd.Env = os.Environ()
	cmd.Stdin = stdin
	if !c.IsLocal || c.persistentIndexStore != nil {
		cacheHome := c.helmHome + "/cache"

		cmd.Env = append(cmd.Env,
			"XDG_CACHE_HOME="+cacheHome,
			"XDG_CONFIG_HOME="+c.helmHome+"/config",
			"XDG_DATA_HOME="+c.helmHome+"/data")
		if c.persistentIndexStore != nil {
			// Explicit Helm variables take precedence over XDG. Override inherited
			// values too, using the same paths into which we prewarm snapshots.
			cmd.Env = append(cmd.Env,
				"HELM_CONFIG_HOME="+c.helmHome+"/config/helm",
				"HELM_CACHE_HOME="+c.helmHome+"/cache/helm",
				"HELM_DATA_HOME="+c.helmHome+"/data/helm",
				"HELM_REPOSITORY_CONFIG="+c.helmHome+"/config/helm/repositories.yaml",
				"HELM_REPOSITORY_CACHE="+c.helmHome+"/cache/helm/repository",
				"HELM_REGISTRY_CONFIG="+c.helmHome+"/config/helm/registry/config.json")
		}
	}

	cmd.Env = proxy.UpsertEnv(cmd, c.proxy, c.noProxy)
	fullCommand := executil.GetCommandArgsToLog(cmd)

	out, err := c.runWithRedactor(cmd, redactor)
	if err != nil {
		return out, fullCommand, fmt.Errorf("failed running helm: %w", err)
	}
	return out, fullCommand, nil
}

// helmRepoEntry mirrors helm's repo.Entry JSON schema (pkg/repo/chartrepo.go):
// it is what helm writes to repositories.yaml on `helm repo add` and what the
// dependency downloader reads credentials from when pulling chart tarballs.
type helmRepoEntry struct {
	Name                  string `json:"name"`
	URL                   string `json:"url"`
	Username              string `json:"username,omitempty"`
	Password              string `json:"password,omitempty"`
	CertFile              string `json:"certFile,omitempty"`
	KeyFile               string `json:"keyFile,omitempty"`
	CAFile                string `json:"caFile,omitempty"`
	InsecureSkipTLSverify bool   `json:"insecure_skip_tls_verify,omitempty"`
	PassCredentialsAll    bool   `json:"pass_credentials_all,omitempty"`
}

// prewarmIndexes installs a repository's cached index into the ephemeral helm
// cache home under BOTH names helm resolves indexes by: the registered
// repository name (used when the URL matches a repositories.yaml entry) and
// the dependency manager's generated name "helm-manager-<sha256(url)>" (used
// when a Chart.yaml dependency references the URL directly). Callers must
// register both names in repositories.yaml (see writeRepoEntries callers).
func (c *Cmd) prewarmIndexes(repoURL string, indexData []byte, repoName string) error {
	destDir := path.Join(c.helmHome, "cache", "helm", "repository")
	if err := c.persistentIndexStore.CopyTo(repoURL, indexData, destDir, repoName); err != nil {
		return err
	}
	managerName := managerKeyPrefix + managerKey(repoURL)
	return c.persistentIndexStore.CopyTo(repoURL, indexData, destDir, managerName)
}

// Repository client certificates and keys live for the entire subprocess
// lifetime and are removed with the private Helm home by Close.
func (c *Cmd) writeRepoCredential(data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	dir := filepath.Join(c.helmHome, "config")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(dir, "repo-credential-*")
	if err != nil {
		return "", err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return file.Name(), nil
}

// managerKeyPrefix mirrors helm's downloader.managerKeyPrefix: the generated
// repository cache name for unmanaged repositories.
const managerKeyPrefix = "helm-manager-"

// managerKey mirrors helm's downloader.key: a hex sha256 of the repository
// URL, used in the generated "helm-manager-<key>" cache name.
func managerKey(name string) string {
	sum := sha256.Sum256([]byte(name))
	return hex.EncodeToString(sum[:])
}

// repoFileEntries is the subset of helm's repositories.yaml schema needed to
// enumerate configured repositories (name + URL).
type repoFileEntries struct {
	APIVersion   string          `json:"apiVersion,omitempty"`
	Generated    string          `json:"generated,omitempty"`
	Repositories []helmRepoEntry `json:"repositories"`
}

// writeRepoEntries writes the given repository entries into the ephemeral
// helm config home's repositories.yaml, equivalent to a `helm repo add` per
// entry that skips the index download. The file lives at
// <XDG_CONFIG_HOME>/helm/repositories.yaml (helmpath.ConfigPath, no
// "repository" subdirectory — that one belongs to the cache home).
func (c *Cmd) writeRepoEntries(entries []helmRepoEntry) error {
	configRepoDir := path.Join(c.helmHome, "config", "helm")
	repoFile := path.Join(configRepoDir, "repositories.yaml")

	rf := repoFileEntries{
		APIVersion: "v1", // helm's repo.File default APIVersion
		Generated:  time.Now().UTC().Format(time.RFC3339),
	}
	if data, err := os.ReadFile(repoFile); err == nil {
		if err := yaml.Unmarshal(data, &rf); err != nil {
			return fmt.Errorf("failed to parse helm repositories.yaml: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read helm repositories.yaml: %w", err)
	}

	known := make(map[string]helmRepoEntry, len(rf.Repositories))
	for _, entry := range rf.Repositories {
		known[entry.Name] = entry
	}
	for _, entry := range entries {
		if existing, ok := known[entry.Name]; ok {
			if existing.URL != entry.URL {
				return fmt.Errorf("repository name %s already registered with a different URL", entry.Name)
			}
			if existing != entry {
				return fmt.Errorf("repository name %s has conflicting credentials or TLS settings", entry.Name)
			}
			continue // already registered
		}
		rf.Repositories = append(rf.Repositories, entry)
		known[entry.Name] = entry
	}

	if err := os.MkdirAll(configRepoDir, 0o700); err != nil {
		return fmt.Errorf("failed to create helm config dir: %w", err)
	}
	out, err := yaml.Marshal(rf)
	if err != nil {
		return fmt.Errorf("failed to serialize helm repositories.yaml: %w", err)
	}
	if err := os.WriteFile(repoFile, out, 0o600); err != nil {
		return fmt.Errorf("failed to write helm repositories.yaml: %w", err)
	}
	return nil
}

func (c *Cmd) RegistryLogin(ctx context.Context, repo string, creds Creds, plainHTTP bool) (string, error) {
	args := []string{"registry", "login"}
	registry, err := c.getHelmRegistry(repo)
	if err != nil {
		return "", fmt.Errorf("failed to parse registry URL: %w", err)
	}
	args = append(args, registry)
	if plainHTTP {
		args = append(args, "--plain-http")
	}

	if creds.GetUsername() != "" {
		args = append(args, "--username", creds.GetUsername())
	}

	helmPassword, err := creds.GetPassword()
	if err != nil {
		return "", fmt.Errorf("failed to get password for helm registry: %w", err)
	}
	if helmPassword != "" {
		args = append(args, "--password-stdin")
	}

	if creds.GetCAPath() != "" {
		args = append(args, "--ca-file", creds.GetCAPath())
	}

	if len(creds.GetCertData()) > 0 {
		filePath, closer, err := writeToTmp(creds.GetCertData())
		if err != nil {
			return "", fmt.Errorf("failed to write certificate data to temporary file: %w", err)
		}
		defer utilio.Close(closer)
		args = append(args, "--cert-file", filePath)
	}

	if len(creds.GetKeyData()) > 0 {
		filePath, closer, err := writeToTmp(creds.GetKeyData())
		if err != nil {
			return "", fmt.Errorf("failed to write key data to temporary file: %w", err)
		}
		defer utilio.Close(closer)
		args = append(args, "--key-file", filePath)
	}

	if creds.GetInsecureSkipVerify() {
		args = append(args, "--insecure")
	}
	var stdin io.Reader
	if helmPassword != "" {
		stdin = strings.NewReader(helmPassword)
	}
	out, _, err := c.runWithStdin(ctx, stdin, args...)
	if err != nil {
		return "", fmt.Errorf("failed to login to registry: %w", err)
	}
	return out, nil
}

func (c *Cmd) RegistryLogout(repo string, _ Creds) (string, error) {
	args := []string{"registry", "logout"}
	registry, err := c.getHelmRegistry(repo)
	if err != nil {
		return "", fmt.Errorf("failed to parse registry URL: %w", err)
	}
	args = append(args, registry)
	out, _, err := c.run(context.Background(), args...)
	if err != nil {
		return "", fmt.Errorf("failed to logout from registry: %w", err)
	}
	return out, nil
}

func (c *Cmd) RepoAdd(name string, url string, opts Creds, passCredentials bool) (string, error) {
	tmp, err := os.MkdirTemp("", "helm")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory for repo: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	args := []string{"repo", "add"}

	if opts.GetUsername() != "" {
		args = append(args, "--username", opts.GetUsername())
	}

	helmPassword, err := opts.GetPassword()
	if err != nil {
		return "", fmt.Errorf("failed to get password for helm registry: %w", err)
	}
	if helmPassword != "" {
		args = append(args, "--password", helmPassword)
	}

	if opts.GetCAPath() != "" {
		args = append(args, "--ca-file", opts.GetCAPath())
	}

	if opts.GetInsecureSkipVerify() {
		args = append(args, "--insecure-skip-tls-verify")
	}

	if len(opts.GetCertData()) > 0 {
		certFile, err := os.CreateTemp("", "helm")
		if err != nil {
			return "", fmt.Errorf("failed to create temporary certificate file: %w", err)
		}
		_, err = certFile.Write(opts.GetCertData())
		if err != nil {
			return "", fmt.Errorf("failed to write certificate data: %w", err)
		}
		defer certFile.Close()
		args = append(args, "--cert-file", certFile.Name())
	}

	if len(opts.GetKeyData()) > 0 {
		keyFile, err := os.CreateTemp("", "helm")
		if err != nil {
			return "", fmt.Errorf("failed to create temporary key file: %w", err)
		}
		_, err = keyFile.Write(opts.GetKeyData())
		if err != nil {
			return "", fmt.Errorf("failed to write key data: %w", err)
		}
		defer keyFile.Close()
		args = append(args, "--key-file", keyFile.Name())
	}

	if passCredentials {
		args = append(args, "--pass-credentials")
	}

	args = append(args, name, url)

	out, _, err := c.run(context.Background(), args...)
	if err != nil {
		return "", fmt.Errorf("failed to add repository: %w", err)
	}
	return out, err
}

func writeToTmp(data []byte) (string, utilio.Closer, error) {
	file, err := os.CreateTemp("", "")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temporary file: %w", err)
	}
	err = os.WriteFile(file.Name(), data, 0o644)
	if err != nil {
		_ = os.RemoveAll(file.Name())
		return "", nil, fmt.Errorf("failed to write data to temporary file: %w", err)
	}
	defer func() {
		if err = file.Close(); err != nil {
			log.WithFields(log.Fields{
				common.SecurityField:    common.SecurityMedium,
				common.SecurityCWEField: common.SecurityCWEMissingReleaseOfFileDescriptor,
			}).Errorf("error closing file %q: %v", file.Name(), err)
		}
	}()
	return file.Name(), utilio.NewCloser(func() error {
		return os.RemoveAll(file.Name())
	}), nil
}

func (c *Cmd) Fetch(repo, chartName, version, destination string, creds Creds, passCredentials bool) (string, error) {
	args := []string{"pull", "--destination", destination}
	if version != "" {
		args = append(args, "--version", version)
	}
	if creds.GetUsername() != "" {
		args = append(args, "--username", creds.GetUsername())
	}

	helmPassword, err := creds.GetPassword()
	if err != nil {
		return "", fmt.Errorf("failed to get password for helm registry: %w", err)
	}
	if helmPassword != "" {
		args = append(args, "--password", helmPassword)
	}
	if creds.GetInsecureSkipVerify() {
		args = append(args, "--insecure-skip-tls-verify")
	}

	args = append(args, "--repo", repo, chartName)

	if creds.GetCAPath() != "" {
		args = append(args, "--ca-file", creds.GetCAPath())
	}
	if len(creds.GetCertData()) > 0 {
		filePath, closer, err := writeToTmp(creds.GetCertData())
		if err != nil {
			return "", fmt.Errorf("failed to write certificate data to temporary file: %w", err)
		}
		defer utilio.Close(closer)
		args = append(args, "--cert-file", filePath)
	}
	if len(creds.GetKeyData()) > 0 {
		filePath, closer, err := writeToTmp(creds.GetKeyData())
		if err != nil {
			return "", fmt.Errorf("failed to write key data to temporary file: %w", err)
		}
		defer utilio.Close(closer)
		args = append(args, "--key-file", filePath)
	}
	if passCredentials {
		args = append(args, "--pass-credentials")
	}

	out, _, err := c.run(context.Background(), args...)
	if err != nil {
		return "", fmt.Errorf("failed to fetch chart: %w", err)
	}
	return out, nil
}

func (c *Cmd) PullOCI(repo string, chart string, version string, destination string, creds Creds, plainHTTP bool) (string, error) {
	args := []string{
		"pull", fmt.Sprintf("oci://%s/%s", repo, chart), "--version",
		version,
		"--destination",
		destination,
	}
	if creds.GetCAPath() != "" {
		args = append(args, "--ca-file", creds.GetCAPath())
	}

	if len(creds.GetCertData()) > 0 {
		filePath, closer, err := writeToTmp(creds.GetCertData())
		if err != nil {
			return "", fmt.Errorf("failed to write certificate data to temporary file: %w", err)
		}
		defer utilio.Close(closer)
		args = append(args, "--cert-file", filePath)
	}

	if len(creds.GetKeyData()) > 0 {
		filePath, closer, err := writeToTmp(creds.GetKeyData())
		if err != nil {
			return "", fmt.Errorf("failed to write key data to temporary file: %w", err)
		}
		defer utilio.Close(closer)
		args = append(args, "--key-file", filePath)
	}

	if creds.GetInsecureSkipVerify() {
		args = append(args, "--insecure-skip-tls-verify")
	}
	if plainHTTP {
		args = append(args, "--plain-http")
	}
	out, _, err := c.run(context.Background(), args...)
	if err != nil {
		return "", fmt.Errorf("failed to pull OCI chart: %w", err)
	}
	return out, nil
}

func (c *Cmd) dependencyBuild(ctx context.Context, insecure bool, plainHTTP bool, skipRefresh bool) (string, error) {
	args := []string{"dependency", "build"}
	if insecure {
		args = append(args, "--insecure-skip-tls-verify")
	}
	if plainHTTP {
		args = append(args, "--plain-http")
	}
	// skipRefresh is only set when the ephemeral helm cache home has been
	// pre-populated with recently downloaded repository indexes for every
	// dependency repository (see DependencyBuild). Skipping helm's repo
	// refresh then makes the build resolve dependency versions from those
	// local indexes instead of re-downloading every repository's index.yaml.
	if skipRefresh {
		args = append(args, "--skip-refresh")
	}
	out, _, err := c.run(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("failed to build dependencies: %w", err)
	}
	return out, nil
}

func (c *Cmd) inspectValues(values string) (string, error) {
	out, _, err := c.run(context.Background(), "show", "values", values)
	if err != nil {
		return "", fmt.Errorf("failed to inspect values: %w", err)
	}
	return out, nil
}

func (c *Cmd) InspectChart() (string, error) {
	out, _, err := c.run(context.Background(), "show", "chart", ".")
	if err != nil {
		return "", fmt.Errorf("failed to inspect chart: %w", err)
	}
	return out, nil
}

type TemplateOpts struct {
	Name        string
	Namespace   string
	KubeVersion string
	APIVersions []string
	Set         map[string]string
	SetString   map[string]string
	SetFile     map[string]pathutil.ResolvedFilePath
	Values      []pathutil.ResolvedFilePath
	// ExtraValues is the randomly-generated path to the temporary values file holding the contents of
	// spec.source.helm.values/valuesObject.
	ExtraValues          pathutil.ResolvedFilePath
	SkipCrds             bool
	SkipSchemaValidation bool
	SkipTests            bool
}

func cleanSetParameters(val string) string {
	// `{}` equal helm list parameters format, so don't escape `,`.
	if strings.HasPrefix(val, `{`) && strings.HasSuffix(val, `}`) {
		return val
	}

	val = replaceAllWithLookbehind(val, ',', `\,`, '\\')
	return val
}

func replaceAllWithLookbehind(val string, old rune, newV string, lookbehind rune) string {
	var result strings.Builder
	var prevR rune
	for _, r := range val {
		if r == old {
			if prevR != lookbehind {
				result.WriteString(newV)
			} else {
				result.WriteRune(old)
			}
		} else {
			result.WriteRune(r)
		}
		prevR = r
	}
	return result.String()
}

var apiVersionsRemover = regexp.MustCompile(`(--api-versions [^ ]+ )+`)

func (c *Cmd) template(chartPath string, opts *TemplateOpts) (string, string, error) {
	if callback, err := cleanupChartLockFile(filepath.Clean(path.Join(c.WorkDir, chartPath))); err == nil {
		defer callback()
	} else {
		return "", "", fmt.Errorf("failed to clean up chart lock file: %w", err)
	}

	args := []string{"template", chartPath, "--name-template", opts.Name}

	if opts.Namespace != "" {
		args = append(args, "--namespace", opts.Namespace)
	}
	if opts.KubeVersion != "" {
		args = append(args, "--kube-version", opts.KubeVersion)
	}
	for key, val := range opts.Set {
		args = append(args, "--set", key+"="+cleanSetParameters(val))
	}
	for key, val := range opts.SetString {
		args = append(args, "--set-string", key+"="+cleanSetParameters(val))
	}
	for key, val := range opts.SetFile {
		args = append(args, "--set-file", key+"="+cleanSetParameters(string(val)))
	}
	for _, val := range opts.Values {
		args = append(args, "--values", string(val))
	}
	if opts.ExtraValues != "" {
		args = append(args, "--values", string(opts.ExtraValues))
	}
	for _, v := range opts.APIVersions {
		args = append(args, "--api-versions", v)
	}
	if !opts.SkipCrds {
		args = append(args, "--include-crds")
	}
	if opts.SkipSchemaValidation {
		args = append(args, "--skip-schema-validation")
	}
	if opts.SkipTests {
		args = append(args, "--skip-tests")
	}

	out, command, err := c.run(context.Background(), args...)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "--api-versions") {
			log.Debug(msg)
			msg = apiVersionsRemover.ReplaceAllString(msg, "<api versions removed> ")
		}
		return "", command, errors.New(msg)
	}
	return out, command, nil
}

// Workaround for Helm3 behavior (see https://github.com/helm/helm/issues/6870).
// The `helm template` command generates Chart.lock after which `helm dependency build` does not work
// As workaround removing lock file unless it exists before running helm template
func cleanupChartLockFile(chartPath string) (func(), error) {
	exists := true
	lockPath := path.Join(chartPath, "Chart.lock")
	if _, err := os.Stat(lockPath); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to check lock file status: %w", err)
		}
		exists = false
	}
	return func() {
		if !exists {
			_ = os.Remove(lockPath)
		}
	}, nil
}

func (c *Cmd) Freestyle(args ...string) (string, error) {
	out, _, err := c.run(context.Background(), args...)
	if err != nil {
		return "", fmt.Errorf("failed to execute freestyle helm command: %w", err)
	}
	return out, nil
}

func (c *Cmd) Close() {
	_ = os.RemoveAll(c.helmHome)
}

// getHelmRegistry extracts the registry host from a Helm repository URL. This is because it is required for the
// `helm registry login` command to use the registry host rather than the full URL.
func (c *Cmd) getHelmRegistry(repo string) (string, error) {
	if !strings.Contains(repo, "//") {
		repo = "//" + repo
	}

	uri, err := url.Parse(repo)
	if err != nil {
		return "", err
	}

	return uri.Host, nil
}
