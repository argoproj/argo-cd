package helm

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/common"
	executil "github.com/argoproj/argo-cd/v2/util/exec"
	argoio "github.com/argoproj/argo-cd/v2/util/io"
	pathutil "github.com/argoproj/argo-cd/v2/util/io/path"
	"github.com/argoproj/argo-cd/v2/util/proxy"
)

// A thin wrapper around the "helm" command, adding logging and error translation.
type Cmd struct {
	helmHome  string
	WorkDir   string
	IsLocal   bool
	IsHelmOci bool
	proxy     string
	noProxy   string
}

func NewCmd(workDir string, version string, proxy string, noProxy string) (*Cmd, error) {
	switch version {
	// If v3 is specified (or by default, if no value is specified) then use v3
	case "", "v3":
		return NewCmdWithVersion(workDir, false, proxy, noProxy)
	}
	return nil, fmt.Errorf("helm chart version '%s' is not supported", version)
}

func NewCmdWithVersion(workDir string, isHelmOci bool, proxy string, noProxy string) (*Cmd, error) {
	tmpDir, err := os.MkdirTemp("", "helm")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory for helm: %w", err)
	}
	return &Cmd{WorkDir: workDir, helmHome: tmpDir, IsHelmOci: isHelmOci, proxy: proxy, noProxy: noProxy}, err
}

var redactor = func(text string) string {
	return regexp.MustCompile("(--username|--password) [^ ]*").ReplaceAllString(text, "$1 ******")
}

func (c Cmd) run(args ...string) (string, string, error) {
	cmd := exec.Command("helm", args...)
	cmd.Dir = c.WorkDir
	cmd.Env = os.Environ()
	if !c.IsLocal {
		cmd.Env = append(cmd.Env,
			fmt.Sprintf("XDG_CACHE_HOME=%s/cache", c.helmHome),
			fmt.Sprintf("XDG_CONFIG_HOME=%s/config", c.helmHome),
			fmt.Sprintf("XDG_DATA_HOME=%s/data", c.helmHome),
			fmt.Sprintf("HELM_CONFIG_HOME=%s/config", c.helmHome))
	}

	if c.IsHelmOci {
		cmd.Env = append(cmd.Env, "HELM_EXPERIMENTAL_OCI=1")
	}

	cmd.Env = proxy.UpsertEnv(cmd, c.proxy, c.noProxy)

	out, err := executil.RunWithRedactor(cmd, redactor)
	fullCommand := executil.GetCommandArgsToLog(cmd)
	if err != nil {
		return out, fullCommand, fmt.Errorf("failed to get command args to log: %w", err)
	}
	return out, fullCommand, nil
}

func (c *Cmd) RegistryLogin(repo string, creds Creds) (string, error) {
	args := []string{"registry", "login"}
	args = append(args, repo)

	if creds.Username != "" {
		args = append(args, "--username", creds.Username)
	}

	if creds.Password != "" {
		args = append(args, "--password", creds.Password)
	}

	if creds.CAPath != "" {
		args = append(args, "--ca-file", creds.CAPath)
	}

	if len(creds.CertData) > 0 {
		filePath, closer, err := writeToTmp(creds.CertData)
		if err != nil {
			return "", fmt.Errorf("failed to write certificate data to temporary file: %w", err)
		}
		defer argoio.Close(closer)
		args = append(args, "--cert-file", filePath)
	}

	if len(creds.KeyData) > 0 {
		filePath, closer, err := writeToTmp(creds.KeyData)
		if err != nil {
			return "", fmt.Errorf("failed to write key data to temporary file: %w", err)
		}
		defer argoio.Close(closer)
		args = append(args, "--key-file", filePath)
	}

	if creds.InsecureSkipVerify {
		args = append(args, "--insecure")
	}
	out, _, err := c.run(args...)
	if err != nil {
		return "", fmt.Errorf("failed to login to registry: %w", err)
	}
	return out, nil
}

func (c *Cmd) RegistryLogout(repo string, creds Creds) (string, error) {
	args := []string{"registry", "logout"}
	args = append(args, repo)
	out, _, err := c.run(args...)
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

	if opts.Username != "" {
		args = append(args, "--username", opts.Username)
	}

	if opts.Password != "" {
		args = append(args, "--password", opts.Password)
	}

	if opts.CAPath != "" {
		args = append(args, "--ca-file", opts.CAPath)
	}

	if opts.InsecureSkipVerify {
		args = append(args, "--insecure-skip-tls-verify")
	}

	if len(opts.CertData) > 0 {
		certFile, err := os.CreateTemp("", "helm")
		if err != nil {
			return "", fmt.Errorf("failed to create temporary certificate file: %w", err)
		}
		_, err = certFile.Write(opts.CertData)
		if err != nil {
			return "", fmt.Errorf("failed to write certificate data: %w", err)
		}
		defer certFile.Close()
		args = append(args, "--cert-file", certFile.Name())
	}

	if len(opts.KeyData) > 0 {
		keyFile, err := os.CreateTemp("", "helm")
		if err != nil {
			return "", fmt.Errorf("failed to create temporary key file: %w", err)
		}
		_, err = keyFile.Write(opts.KeyData)
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

	out, _, err := c.run(args...)
	if err != nil {
		return "", fmt.Errorf("failed to add repository: %w", err)
	}
	return out, err
}

func writeToTmp(data []byte) (string, argoio.Closer, error) {
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
	return file.Name(), argoio.NewCloser(func() error {
		return os.RemoveAll(file.Name())
	}), nil
}

func (c *Cmd) Fetch(repo, chartName, version, destination string, creds Creds, passCredentials bool) (string, error) {
	args := []string{"pull", "--destination", destination}
	if version != "" {
		args = append(args, "--version", version)
	}
	if creds.Username != "" {
		args = append(args, "--username", creds.Username)
	}
	if creds.Password != "" {
		args = append(args, "--password", creds.Password)
	}
	if creds.InsecureSkipVerify {
		args = append(args, "--insecure-skip-tls-verify")
	}

	args = append(args, "--repo", repo, chartName)

	if creds.CAPath != "" {
		args = append(args, "--ca-file", creds.CAPath)
	}
	if len(creds.CertData) > 0 {
		filePath, closer, err := writeToTmp(creds.CertData)
		if err != nil {
			return "", fmt.Errorf("failed to write certificate data to temporary file: %w", err)
		}
		defer argoio.Close(closer)
		args = append(args, "--cert-file", filePath)
	}
	if len(creds.KeyData) > 0 {
		filePath, closer, err := writeToTmp(creds.KeyData)
		if err != nil {
			return "", fmt.Errorf("failed to write key data to temporary file: %w", err)
		}
		defer argoio.Close(closer)
		args = append(args, "--key-file", filePath)
	}
	if passCredentials {
		args = append(args, "--pass-credentials")
	}

	out, _, err := c.run(args...)
	if err != nil {
		return "", fmt.Errorf("failed to fetch chart: %w", err)
	}
	return out, nil
}

func (c *Cmd) PullOCI(repo string, chart string, version string, destination string, creds Creds) (string, error) {
	args := []string{
		"pull", fmt.Sprintf("oci://%s/%s", repo, chart), "--version",
		version,
		"--destination",
		destination,
	}
	if creds.CAPath != "" {
		args = append(args, "--ca-file", creds.CAPath)
	}

	if len(creds.CertData) > 0 {
		filePath, closer, err := writeToTmp(creds.CertData)
		if err != nil {
			return "", fmt.Errorf("failed to write certificate data to temporary file: %w", err)
		}
		defer argoio.Close(closer)
		args = append(args, "--cert-file", filePath)
	}

	if len(creds.KeyData) > 0 {
		filePath, closer, err := writeToTmp(creds.KeyData)
		if err != nil {
			return "", fmt.Errorf("failed to write key data to temporary file: %w", err)
		}
		defer argoio.Close(closer)
		args = append(args, "--key-file", filePath)
	}

	if creds.InsecureSkipVerify {
		args = append(args, "--insecure-skip-tls-verify")
	}
	out, _, err := c.run(args...)
	if err != nil {
		return "", fmt.Errorf("failed to pull OCI chart: %w", err)
	}
	return out, nil
}

func (c *Cmd) dependencyBuild() (string, error) {
	out, _, err := c.run("dependency", "build")
	if err != nil {
		return "", fmt.Errorf("failed to build dependencies: %w", err)
	}
	return out, nil
}

func (c *Cmd) inspectValues(values string) (string, error) {
	out, _, err := c.run("show", "values", values)
	if err != nil {
		return "", fmt.Errorf("failed to inspect values: %w", err)
	}
	return out, nil
}

func (c *Cmd) InspectChart() (string, error) {
	out, _, err := c.run("show", "chart", ".")
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
	ExtraValues pathutil.ResolvedFilePath
	SkipCrds    bool
}

var (
	re                 = regexp.MustCompile(`([^\\]),`)
	apiVersionsRemover = regexp.MustCompile(`(--api-versions [^ ]+ )+`)
)

func cleanSetParameters(val string) string {
	// `{}` equal helm list parameters format, so don't escape `,`.
	if strings.HasPrefix(val, `{`) && strings.HasSuffix(val, `}`) {
		return val
	}
	return re.ReplaceAllString(val, `$1\,`)
}

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

	out, command, err := c.run(args...)
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
		if os.IsNotExist(err) {
			exists = false
		} else {
			return nil, fmt.Errorf("failed to check lock file status: %w", err)
		}
	}
	return func() {
		if !exists {
			_ = os.Remove(lockPath)
		}
	}, nil
}

func (c *Cmd) Freestyle(args ...string) (string, error) {
	out, _, err := c.run(args...)
	if err != nil {
		return "", fmt.Errorf("failed to execute freestyle helm command: %w", err)
	}
	return out, nil
}

func (c *Cmd) Close() {
	_ = os.RemoveAll(c.helmHome)
}
