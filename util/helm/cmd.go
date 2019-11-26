package helm

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"

	argoexec "github.com/argoproj/pkg/exec"

	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/config"
)

// A thin wrapper around the "helm" command, adding logging and error translation.
type Cmd struct {
	helmHome string
	WorkDir  string
}

func NewCmd(workDir string) (*Cmd, error) {
	tmpDir, err := ioutil.TempDir("", "helm")
	if err != nil {
		return nil, err
	}
	return &Cmd{WorkDir: workDir, helmHome: tmpDir}, err
}

var redactor = func(text string) string {
	return regexp.MustCompile("(--username|--password) [^ ]*").ReplaceAllString(text, "$1 ******")
}

func (c Cmd) run(args ...string) (string, error) {
	cmd := exec.Command("helm", args...)
	cmd.Dir = c.WorkDir
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("HELM_HOME=%s", c.helmHome))
	return argoexec.RunCommandExt(cmd, argoexec.CmdOpts{
		Timeout:  config.CmdOpts().Timeout,
		Redactor: redactor,
	})
}

func (c *Cmd) Init() (string, error) {
	return c.run("init", "--client-only", "--skip-refresh")
}

func (c *Cmd) RepoAdd(name, url string, opts Creds) (string, error) {

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

func (c *Cmd) RepoUpdate() (string, error) {
	return c.run("repo", "update")
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
	return file.Name(), util.NewCloser(func() error {
		return os.RemoveAll(file.Name())
	}), nil
}

func (c *Cmd) Fetch(repo, chartName, version, destination string, creds Creds) (string, error) {
	args := []string{"fetch", "--destination", destination}

	if version != "" {
		args = append(args, "--version", version)
	}
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
		defer util.Close(closer)
		args = append(args, "--cert-file", filePath)
	}
	if len(creds.KeyData) > 0 {
		filePath, closer, err := writeToTmp(creds.KeyData)
		if err != nil {
			return "", err
		}
		defer util.Close(closer)
		args = append(args, "--key-file", filePath)
	}

	args = append(args, "--repo", repo, chartName)
	return c.run(args...)
}

func (c *Cmd) dependencyBuild() (string, error) {
	return c.run("dependency", "build")
}

func (c *Cmd) inspectValues(values string) (string, error) {
	return c.run("inspect", "values", values)
}

type TemplateOpts struct {
	Name        string
	Namespace   string
	KubeVersion string
	Set         map[string]string
	SetString   map[string]string
	Values      []string
}

var (
	re = regexp.MustCompile(`([^\\]),`)
)

func cleanSetParameters(val string) string {
	return re.ReplaceAllString(val, `$1\,`)
}

func (c *Cmd) template(chart string, opts *TemplateOpts) (string, error) {
	args := []string{"template", chart, "--name", opts.Name}

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
	for _, val := range opts.Values {
		args = append(args, "--values", val)
	}

	return c.run(args...)
}

func (c *Cmd) Close() {
	_ = os.RemoveAll(c.helmHome)
}
