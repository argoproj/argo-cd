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
	HelmVer
	helmHome  string
	WorkDir   string
	IsLocal   bool
	IsHelmOci bool
	proxy     string
}

func NewCmd(workDir string, version string, proxy string) (*Cmd, error) {

	switch version {
	// If v3 is specified (or by default, if no value is specified) then use v3
	case "", "v3":
		return NewCmdWithVersion(workDir, HelmV3, false, proxy)
	}
	return nil, fmt.Errorf("helm chart version '%s' is not supported", version)
}

func NewCmdWithVersion(workDir string, version HelmVer, isHelmOci bool, proxy string) (*Cmd, error) {
	tmpDir, err := os.MkdirTemp("", "helm")
	if err != nil {
		return nil, err
	}
	return &Cmd{WorkDir: workDir, helmHome: tmpDir, HelmVer: version, IsHelmOci: isHelmOci, proxy: proxy}, err
}

var redactor = func(text string) string {
	return regexp.MustCompile("(--username|--password) [^ ]*").ReplaceAllString(text, "$1 ******")
}

func (c Cmd) run(args ...string) (string, error) {
	cmd := exec.Command(c.binaryName, args...)
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

	cmd.Env = proxy.UpsertEnv(cmd, c.proxy)

	return executil.RunWithRedactor(cmd, redactor)
}

func (c *Cmd) Init() (string, error) {
	if c.initSupported {
		return c.run("init", "--client-only", "--skip-refresh")
	}
	return "", nil
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
			return "", err
		}
		defer argoio.Close(closer)
		args = append(args, "--cert-file", filePath)
	}

	if len(creds.KeyData) > 0 {
		filePath, closer, err := writeToTmp(creds.KeyData)
		if err != nil {
			return "", err
		}
		defer argoio.Close(closer)
		args = append(args, "--key-file", filePath)
	}

	if creds.InsecureSkipVerify {
		args = append(args, "--insecure")
	}
	return c.run(args...)
}

func (c *Cmd) RegistryLogout(repo string, creds Creds) (string, error) {
	args := []string{"registry", "logout"}
	args = append(args, repo)

	return c.run(args...)
}

func (c *Cmd) RepoAdd(name string, url string, opts Creds, passCredentials bool) (string, error) {
	tmp, err := os.MkdirTemp("", "helm")
	if err != nil {
		return "", err
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

	if opts.InsecureSkipVerify && c.insecureSkipVerifySupported {
		args = append(args, "--insecure-skip-tls-verify")
	}

	if len(opts.CertData) > 0 {
		certFile, err := os.CreateTemp("", "helm")
		if err != nil {
			return "", err
		}
		_, err = certFile.Write(opts.CertData)
		if err != nil {
			return "", err
		}
		defer certFile.Close()
		args = append(args, "--cert-file", certFile.Name())
	}

	if len(opts.KeyData) > 0 {
		keyFile, err := os.CreateTemp("", "helm")
		if err != nil {
			return "", err
		}
		_, err = keyFile.Write(opts.KeyData)
		if err != nil {
			return "", err
		}
		defer keyFile.Close()
		args = append(args, "--key-file", keyFile.Name())
	}

	if c.helmPassCredentialsSupported && passCredentials {
		args = append(args, "--pass-credentials")
	}

	args = append(args, name, url)

	return c.run(args...)
}

func writeToTmp(data []byte) (string, argoio.Closer, error) {
	file, err := os.CreateTemp("", "")
	if err != nil {
		return "", nil, err
	}
	err = os.WriteFile(file.Name(), data, 0644)
	if err != nil {
		_ = os.RemoveAll(file.Name())
		return "", nil, err
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
	args := []string{c.pullCommand, "--destination", destination}
	if version != "" {
		args = append(args, "--version", version)
	}
	if creds.Username != "" {
		args = append(args, "--username", creds.Username)
	}
	if creds.Password != "" {
		args = append(args, "--password", creds.Password)
	}
	if creds.InsecureSkipVerify && c.insecureSkipVerifySupported {
		args = append(args, "--insecure-skip-tls-verify")
	}

	args = append(args, "--repo", repo, chartName)

	if creds.CAPath != "" {
		args = append(args, "--ca-file", creds.CAPath)
	}
	if len(creds.CertData) > 0 {
		filePath, closer, err := writeToTmp(creds.CertData)
		if err != nil {
			return "", err
		}
		defer argoio.Close(closer)
		args = append(args, "--cert-file", filePath)
	}
	if len(creds.KeyData) > 0 {
		filePath, closer, err := writeToTmp(creds.KeyData)
		if err != nil {
			return "", err
		}
		defer argoio.Close(closer)
		args = append(args, "--key-file", filePath)
	}
	if passCredentials && c.helmPassCredentialsSupported {
		args = append(args, "--pass-credentials")
	}

	return c.run(args...)
}

func (c *Cmd) PullOCI(repo string, chart string, version string, destination string, creds Creds) (string, error) {
	args := []string{"pull", fmt.Sprintf("oci://%s/%s", repo, chart), "--version",
		version,
		"--destination",
		destination}
	if creds.CAPath != "" {
		args = append(args, "--ca-file", creds.CAPath)
	}

	if len(creds.CertData) > 0 {
		filePath, closer, err := writeToTmp(creds.CertData)
		if err != nil {
			return "", err
		}
		defer argoio.Close(closer)
		args = append(args, "--cert-file", filePath)
	}

	if len(creds.KeyData) > 0 {
		filePath, closer, err := writeToTmp(creds.KeyData)
		if err != nil {
			return "", err
		}
		defer argoio.Close(closer)
		args = append(args, "--key-file", filePath)
	}

	if creds.InsecureSkipVerify && c.insecureSkipVerifySupported {
		args = append(args, "--insecure-skip-tls-verify")
	}
	return c.run(args...)
}

func (c *Cmd) dependencyBuild() (string, error) {
	return c.run("dependency", "build")
}

func (c *Cmd) inspectValues(values string) (string, error) {
	return c.run(c.showCommand, "values", values)
}

func (c *Cmd) InspectChart() (string, error) {
	return c.run(c.showCommand, "chart", ".")
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

func (c *Cmd) template(chartPath string, opts *TemplateOpts) (string, error) {
	if c.HelmVer.getPostTemplateCallback != nil {
		if callback, err := c.HelmVer.getPostTemplateCallback(filepath.Clean(path.Join(c.WorkDir, chartPath))); err == nil {
			defer callback()
		} else {
			return "", err
		}
	}

	args := []string{"template", chartPath, c.templateNameArg, opts.Name}

	if opts.Namespace != "" {
		args = append(args, "--namespace", opts.Namespace)
	}
	if opts.KubeVersion != "" && c.kubeVersionSupported {
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
	for _, v := range opts.APIVersions {
		args = append(args, "--api-versions", v)
	}
	if c.HelmVer.includeCrds && !opts.SkipCrds {
		args = append(args, "--include-crds")
	}

	out, err := c.run(args...)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "--api-versions") {
			log.Debug(msg)
			msg = apiVersionsRemover.ReplaceAllString(msg, "<api versions removed> ")
		}
		return "", errors.New(msg)
	}
	return out, nil
}

func (c *Cmd) Freestyle(args ...string) (string, error) {
	return c.run(args...)
}

func (c *Cmd) Close() {
	_ = os.RemoveAll(c.helmHome)
}
