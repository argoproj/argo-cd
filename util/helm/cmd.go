package helm

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"

	executil "github.com/argoproj/argo-cd/util/exec"
	"github.com/argoproj/argo-cd/util/io"
)

// A thin wrapper around the "helm" command, adding logging and error translation.
type Cmd struct {
	HelmVer
	helmHome  string
	WorkDir   string
	IsLocal   bool
	IsHelmOci bool
}

func NewCmd(workDir string, version string) (*Cmd, error) {
	if version != "" {
		switch version {
		case "v2":
			return NewCmdWithVersion(workDir, HelmV2, false)
		case "v3":
			return NewCmdWithVersion(workDir, HelmV3, false)
		}
	}
	helmVersion, err := getHelmVersion(workDir)
	if err != nil {
		return nil, err
	}

	return NewCmdWithVersion(workDir, *helmVersion, false)
}

func NewCmdWithVersion(workDir string, version HelmVer, isHelmOci bool) (*Cmd, error) {
	tmpDir, err := ioutil.TempDir("", "helm")
	if err != nil {
		return nil, err
	}
	return &Cmd{WorkDir: workDir, helmHome: tmpDir, HelmVer: version, IsHelmOci: isHelmOci}, err
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
			fmt.Sprintf("HELM_HOME=%s", c.helmHome))
	}

	if c.IsHelmOci {
		cmd.Env = append(cmd.Env, "HELM_EXPERIMENTAL_OCI=1")
	}
	return executil.RunWithRedactor(cmd, redactor)
}

func (c *Cmd) Init() (string, error) {
	if c.initSupported {
		return c.run("init", "--client-only", "--skip-refresh")
	}
	return "", nil
}

func (c *Cmd) Login(repo string, creds Creds) (string, error) {
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
		defer io.Close(closer)
		args = append(args, "--cert-file", filePath)
	}
	if len(creds.KeyData) > 0 {
		filePath, closer, err := writeToTmp(creds.KeyData)
		if err != nil {
			return "", err
		}
		defer io.Close(closer)
		args = append(args, "--key-file", filePath)
	}

	if creds.InsecureSkipVerify {
		args = append(args, "--insecure")
	}
	return c.run(args...)
}

func (c *Cmd) Logout(repo string, creds Creds) (string, error) {
	args := []string{"registry", "logout"}
	args = append(args, repo)

	if creds.CAPath != "" {
		args = append(args, "--ca-file", creds.CAPath)
	}
	if len(creds.CertData) > 0 {
		filePath, closer, err := writeToTmp(creds.CertData)
		if err != nil {
			return "", err
		}
		defer io.Close(closer)
		args = append(args, "--cert-file", filePath)
	}
	if len(creds.KeyData) > 0 {
		filePath, closer, err := writeToTmp(creds.KeyData)
		if err != nil {
			return "", err
		}
		defer io.Close(closer)
		args = append(args, "--key-file", filePath)
	}

	return c.run(args...)
}

func (c *Cmd) RepoAdd(name string, url string, opts Creds) (string, error) {
	tmp, err := ioutil.TempDir("", "helm")
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
		certFile, err := ioutil.TempFile("", "helm")
		if err != nil {
			return "", err
		}
		_, err = certFile.Write(opts.CertData)
		if err != nil {
			return "", err
		}
		args = append(args, "--cert-file", certFile.Name())
	}

	if len(opts.KeyData) > 0 {
		keyFile, err := ioutil.TempFile("", "helm")
		if err != nil {
			return "", err
		}
		_, err = keyFile.Write(opts.KeyData)
		if err != nil {
			return "", err
		}
		args = append(args, "--key-file", keyFile.Name())
	}

	args = append(args, name, url)

	return c.run(args...)
}

func writeToTmp(data []byte) (string, io.Closer, error) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		return "", nil, err
	}
	err = ioutil.WriteFile(file.Name(), data, 0644)
	if err != nil {
		_ = os.RemoveAll(file.Name())
		return "", nil, err
	}
	return file.Name(), io.NewCloser(func() error {
		return os.RemoveAll(file.Name())
	}), nil
}

func (c *Cmd) Fetch(repo, chartName, version, destination string, creds Creds) (string, error) {
	args := []string{}

	if _, _, isHelmOci := IsHelmOci(chartName); isHelmOci {
		args = append(args, "chart", "pull")
		repoUrl := fmt.Sprintf(repo + "/" + chartName + ":" + version)
		args = append(args, repoUrl)
	} else {
		args = append(args, c.pullCommand, "--destination", destination)
		if version != "" {
			args = append(args, "--version", version)
		}
		if creds.Username != "" {
			args = append(args, "--username", creds.Username)
		}
		if creds.Password != "" {
			args = append(args, "--password", creds.Password)
		}

		args = append(args, "--repo", repo, chartName)
	}

	if creds.CAPath != "" {
		args = append(args, "--ca-file", creds.CAPath)
	}
	if len(creds.CertData) > 0 {
		filePath, closer, err := writeToTmp(creds.CertData)
		if err != nil {
			return "", err
		}
		defer io.Close(closer)
		args = append(args, "--cert-file", filePath)
	}
	if len(creds.KeyData) > 0 {
		filePath, closer, err := writeToTmp(creds.KeyData)
		if err != nil {
			return "", err
		}
		defer io.Close(closer)
		args = append(args, "--key-file", filePath)
	}

	return c.run(args...)
}

func (c *Cmd) Export(repo, chartName, version, destination string) (string, error) {
	output := ""
	var err error
	args := []string{"chart", "export"}
	repoUrl := fmt.Sprintf(repo + "/" + chartName + ":" + version)
	args = append(args, repoUrl, "--destination", destination)

	output, err = c.run(args...)
	if err != nil {
		return "", err
	}

	// tar helm chart
	repoNamespace, repoName, _ := IsHelmOci(chartName)
	cmd := exec.Command("tar", "-zcvf", repoNamespace+"-"+repoName+"-"+version+".tgz", repoName)
	cmd.Dir = destination
	_, err = executil.Run(cmd)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(destination + "/" + repoName) }()

	return output, nil
}

func (c *Cmd) dependencyBuild() (string, error) {
	return c.run("dependency", "build")
}

func (c *Cmd) inspectValues(values string) (string, error) {
	return c.run(c.showCommand, "values", values)
}

type TemplateOpts struct {
	Name        string
	Namespace   string
	KubeVersion string
	APIVersions []string
	Set         map[string]string
	SetString   map[string]string
	SetFile     map[string]string
	Values      []string
}

var (
	re = regexp.MustCompile(`([^\\]),`)
)

func cleanSetParameters(val string) string {
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
		args = append(args, "--set-file", key+"="+cleanSetParameters(val))
	}
	for _, val := range opts.Values {
		args = append(args, "--values", val)
	}
	for _, v := range opts.APIVersions {
		args = append(args, "--api-versions", v)
	}
	if c.HelmVer.additionalTemplateArgs != nil {
		args = append(args, c.HelmVer.additionalTemplateArgs...)
	}

	return c.run(args...)
}

func (c *Cmd) Close() {
	_ = os.RemoveAll(c.helmHome)
}
