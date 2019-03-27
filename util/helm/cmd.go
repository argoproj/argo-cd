package helm

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
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

func (h cmd) run(args ...string) (string, error) {

	log.WithFields(log.Fields{"workDir": h.workDir, "redactedArgs": string(redact(args)), "helmHome": h.helmHome}).Info("running helm")

	cmd := exec.Command("helm", args...)
	cmd.Env = os.Environ()
	cmd.Dir = h.workDir
	if h.helmHome != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("HELM_HOME=%s", h.helmHome))
	}

	start := time.Now()
	bytes, err := cmd.Output()

	output := string(bytes)

	for lineNo, line := range strings.Split(output, "\n") {
		log.WithFields(log.Fields{"lineNo": lineNo, "line": line}).Debug("output")
	}

	log.WithFields(log.Fields{"seconds": time.Since(start).Seconds()}).Info("took")

	if err != nil {
		exErr, ok := err.(*exec.ExitError)
		if !ok {
			return "", err
		}
		errOutput := string(exErr.Stderr)
		err := fmt.Errorf(strings.TrimSpace(errOutput))

		log.Error(err)

		return "", err
	}

	return output, err
}

func redact(args []string) []byte {
	line := []byte(strings.Join(args, " "))
	line = regexp.MustCompile("--username [^ ] ").ReplaceAll(line, []byte("--username ******"))
	line = regexp.MustCompile("--password [^ ] ").ReplaceAll(line, []byte("--password ******"))
	return line
}

func (h cmd) Init() (string, error) {
	return h.run("init", "--client-only", "--skip-refresh")
}

type repoAddOpts struct {
	Username, Password        string
	CAData, CertData, KeyData []byte
}

func (h cmd) repoAdd(name, url string, opts repoAddOpts) (string, error) {

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

	if opts.CAData != nil {
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

	if opts.CertData != nil {
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

	if opts.KeyData != nil {
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

	return h.run(args...)
}

func (h cmd) repoRm(name string) (string, error) {
	return h.run("repo", "rm", name)
}

func (h cmd) repoUpdate() (string, error) {
	return h.run("repo", "update")
}

type fetchOpts struct {
	Version, Destination string
}

func (h cmd) fetch(repo, chartName string, opts fetchOpts) (string, error) {
	args := []string{"fetch", "--untar", "--untardir", opts.Destination}

	if opts.Version != "" {
		args = append(args, "--version", opts.Version)
	}

	args = append(args, repo+"/"+chartName)
	return h.run(args...)
}

func (h cmd) dependencyBuild() (string, error) {
	return h.run("dependency", "build")
}

func (h cmd) inspectValues(values string) (string, error) {
	return h.run("inspect", "values", values)
}

type templateOpts struct {
	Name      string
	Namespace string
	Set       map[string]string
	Values    []string
}

func (h cmd) template(chart string, opts templateOpts) (string, error) {
	args := []string{"template", "--name", opts.Name}

	if opts.Namespace != "" {
		args = append(args, "--namespace", opts.Namespace)
	}
	for key, val := range opts.Set {
		args = append(args, "--set", key+"="+val)
	}
	for _, set := range opts.Values {
		args = append(args, "--values", set)
	}

	args = append(args, chart)

	return h.run(args...)
}
