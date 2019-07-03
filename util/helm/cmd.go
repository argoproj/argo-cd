package helm

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	argoexec "github.com/argoproj/pkg/exec"
)

// A thin wrapper around the "helm" command, adding logging and error translation.
type cmd struct {
	helmHome string
	workDir  string
}

func newCmd(workDir string) (*cmd, error) {
	tmpDir, err := ioutil.TempDir("", "helm")
	if err != nil {
		return nil, err
	}
	return &cmd{workDir: workDir, helmHome: tmpDir}, err
}

func (c cmd) run(args ...string) (string, error) {
	cmd := exec.Command("helm", args...)
	cmd.Dir = c.workDir
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("HELM_HOME=%s", c.helmHome))
	return argoexec.RunCommandExt(cmd, argoexec.CmdOpts{})
}

func (c *cmd) init() (string, error) {
	return c.run("init", "--client-only", "--skip-refresh")
}

type repoAddOpts struct {
	username, password        string
	caData, certData, keyData []byte
}

func (c *cmd) repoAdd(name, url string, opts repoAddOpts) (string, error) {

	tmp, err := ioutil.TempDir("", "helm")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	args := []string{"repo", "add"}

	if opts.username != "" {
		args = append([]string{"--username", opts.username}, args...)
	}

	if opts.password != "" {
		args = append([]string{"--password", opts.password}, args...)
	}

	if opts.caData != nil {
		caFile, err := ioutil.TempFile("", "helm")
		if err != nil {
			return "", err
		}
		_, err = caFile.Write(opts.caData)
		if err != nil {
			return "", err
		}
		args = append([]string{"--ca-file", caFile.Name()}, args...)
	}

	if opts.certData != nil {
		certFile, err := ioutil.TempFile("", "helm")
		if err != nil {
			return "", err
		}
		_, err = certFile.Write(opts.certData)
		if err != nil {
			return "", err
		}
		args = append([]string{"--cert-file", certFile.Name()}, args...)
	}

	if opts.keyData != nil {
		keyFile, err := ioutil.TempFile("", "helm")
		if err != nil {
			return "", err
		}
		_, err = keyFile.Write(opts.keyData)
		if err != nil {
			return "", err
		}
		args = append([]string{"--key-file", keyFile.Name()}, args...)
	}

	args = append(args, name, url)

	return c.run(args...)
}

func (c *cmd) repoUpdate() (string, error) {
	return c.run("repo", "update")
}

type fetchOpts struct {
	version, destination string
}

func (c *cmd) fetch(repo, chartName string, opts fetchOpts) (string, error) {
	args := []string{"fetch", "--untar", "--untardir", opts.destination}

	if opts.version != "" {
		args = append(args, "--version", opts.version)
	}

	args = append(args, repo+"/"+chartName)
	return c.run(args...)
}

func (c *cmd) dependencyBuild() (string, error) {
	return c.run("dependency", "build")
}

func (c *cmd) inspectValues(values string) (string, error) {
	return c.run("inspect", "values", values)
}

type templateOpts struct {
	name      string
	namespace string
	set       map[string]string
	values    []string
}

func (c *cmd) template(chart string, opts templateOpts) (string, error) {
	args := []string{"template", chart, "--name", opts.name}

	if opts.namespace != "" {
		args = append(args, "--namespace", opts.namespace)
	}
	for key, val := range opts.set {
		args = append(args, "--set", key+"="+val)
	}
	for _, set := range opts.values {
		args = append(args, "--values", set)
	}

	return c.run(args...)
}

func (c *cmd) Close() {
	_ = os.RemoveAll(c.helmHome)
}
