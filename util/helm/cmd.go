package helm

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"

	"github.com/argoproj/argo-cd/util"

	argoexec "github.com/argoproj/pkg/exec"
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
		Redactor: redactor,
	})
}

func (c *Cmd) Init() (string, error) {
	return c.run("init", "--client-only", "--skip-refresh")
}

type RepoAddOpts struct {
	Username, Password        string
	CAData, CertData, KeyData []byte
}

func (c *Cmd) RepoAdd(name, url string, opts RepoAddOpts) (string, error) {

	tmp, err := ioutil.TempDir("", "helm")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	args := []string{"repo", "add"}

	if opts.Username != "" {
		args = append([]string{"--username", opts.Username}, args...)
	}

	if opts.Password != "" {
		args = append([]string{"--password", opts.Password}, args...)
	}

	if len(opts.CAData) > 0 {
		caFile, err := ioutil.TempFile("", "helm")
		if err != nil {
			return "", err
		}
		_, err = caFile.Write(opts.CAData)
		if err != nil {
			return "", err
		}
		args = append([]string{"--ca-file", caFile.Name()}, args...)
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
		args = append([]string{"--cert-file", certFile.Name()}, args...)
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
		args = append([]string{"--key-file", keyFile.Name()}, args...)
	}

	args = append(args, name, url)

	return c.run(args...)
}

func (c *Cmd) RepoUpdate() (string, error) {
	return c.run("repo", "update")
}

type FetchOpts struct {
	Version  string
	CAData   string
	CertData string
	CertKey  string
	Password string
	Username string
}

func writeToTmp(data string) (string, io.Closer, error) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		return "", nil, err
	}
	err = ioutil.WriteFile(file.Name(), []byte(data), 0644)
	if err != nil {
		_ = os.RemoveAll(file.Name())
		return "", nil, err
	}
	return file.Name(), util.NewCloser(func() error {
		return os.RemoveAll(file.Name())
	}), nil
}

func (c *Cmd) Fetch(repo, chartName string, opts FetchOpts) (string, error) {
	args := []string{"fetch"}

	if opts.Version != "" {
		args = append(args, "--version", opts.Version)
	}
	if opts.Username != "" {
		args = append(args, "--username", opts.Username)
	}
	if opts.Password != "" {
		args = append(args, "--password", opts.Password)
	}
	if opts.CAData != "" {
		filePath, closer, err := writeToTmp(opts.CAData)
		if err != nil {
			return "", err
		}
		defer util.Close(closer)
		args = append(args, "--ca-file", filePath)
	}
	if opts.CertData != "" {
		filePath, closer, err := writeToTmp(opts.CertData)
		if err != nil {
			return "", err
		}
		defer util.Close(closer)
		args = append(args, "--cert-file", filePath)
	}
	if opts.CertKey != "" {
		filePath, closer, err := writeToTmp(opts.CertKey)
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

type templateOpts struct {
	name        string
	namespace   string
	kubeVersion string
	set         map[string]string
	setString   map[string]string
	values      []string
}

func (c *Cmd) template(chart string, opts templateOpts) (string, error) {
	args := []string{"template", chart, "--name", opts.name}

	if opts.namespace != "" {
		args = append(args, "--namespace", opts.namespace)
	}
	if opts.kubeVersion != "" {
		args = append(args, "--kube-version", opts.kubeVersion)
	}
	for key, val := range opts.set {
		args = append(args, "--set", key+"="+val)
	}
	for key, val := range opts.setString {
		args = append(args, "--set-string", key+"="+val)
	}
	for _, val := range opts.values {
		args = append(args, "--values", val)
	}

	return c.run(args...)
}

func (c *Cmd) Close() {
	_ = os.RemoveAll(c.helmHome)
}
