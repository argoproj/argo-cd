package helm

import (
	"context"
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

	log "github.com/sirupsen/logrus"

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
	if !c.IsLocal {
		cmd.Env = append(cmd.Env,
			fmt.Sprintf("XDG_CACHE_HOME=%s/cache", c.helmHome),
			fmt.Sprintf("XDG_CONFIG_HOME=%s/config", c.helmHome),
			fmt.Sprintf("XDG_DATA_HOME=%s/data", c.helmHome),
			fmt.Sprintf("HELM_CONFIG_HOME=%s/config", c.helmHome))
	}

	cmd.Env = proxy.UpsertEnv(cmd, c.proxy, c.noProxy)
	fullCommand := executil.GetCommandArgsToLog(cmd)

	out, err := c.runWithRedactor(cmd, redactor)
	if err != nil {
		return out, fullCommand, fmt.Errorf("failed running helm: %w", err)
	}
	return out, fullCommand, nil
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

func (c *Cmd) dependencyBuild(insecure bool, plainHTTP bool) (string, error) {
	args := []string{"dependency", "build"}
	if insecure {
		args = append(args, "--insecure-skip-tls-verify")
	}
	if plainHTTP {
		args = append(args, "--plain-http")
	}
	out, _, err := c.run(context.Background(), args...)
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

// Environ returns the environment variables needed so that a child process
// invoking Helm indirectly (e.g. kustomize build --enable-helm) picks up
// registry credentials from prior helm registry login calls.
//
// We use HELM_REGISTRY_CONFIG rather than HELM_CONFIG_HOME because kustomize
// creates its own temporary helm home and overrides HELM_CONFIG_HOME,
// HELM_CACHE_HOME, and HELM_DATA_HOME when spawning the child helm process.
// HELM_REGISTRY_CONFIG is not overridden by kustomize and takes precedence
// in helm's credential resolution, so the child process will find the
// registry credentials from our prior helm registry login calls.
func (c *Cmd) Environ() []string {
	return []string{
		fmt.Sprintf("HELM_REGISTRY_CONFIG=%s/config/registry/config.json", c.helmHome),
	}
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
