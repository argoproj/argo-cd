package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

// A thin wrapper around the "helm" command, adding logging and error translation.
type Helm struct {
	workDir string
}

func NewHelm(workDir string) Helm {
	return Helm{workDir: workDir}
}

func (h Helm) run(args ...string) (string, error) {

	log.Infof("%s: helm %s", h.workDir, redact(args))
	cmd := exec.Command("helm", args...)
	cmd.Dir = h.workDir
	bytes, err := cmd.Output()

	output := string(bytes)

	for lineNo, line := range strings.Split(output, "\n") {
		log.Debugf("output %d: %s", lineNo, line)
	}

	if err != nil {
		exErr, ok := err.(*exec.ExitError)
		if !ok {
			return "", err
		}
		errOutput := string(exErr.Stderr)
		log.Errorf("`%s", errOutput)
		return "", fmt.Errorf(strings.TrimSpace(errOutput))
	}

	return output, err
}

func redact(args []string) []byte {
	line := []byte(strings.Join(args, " "))
	line = regexp.MustCompile("--username [^ ] ").ReplaceAll(line, []byte("--username ******"))
	line = regexp.MustCompile("--password [^ ] ").ReplaceAll(line, []byte("--password ******"))
	return line
}

func (h Helm) Init() (string, error) {
	return h.run("init", "--client-only", "--skip-refresh")
}

type RepoAddOpts struct {
	Username, Password        string
	CAData, CertData, KeyData []byte
}

func (h Helm) RepoAdd(name, url string, opts RepoAddOpts) (string, error) {

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

func (h Helm) RepoRm(name string) (string, error) {
	return h.run("repo", "rm", name)
}

func (h Helm) RepoUpdate() (string, error) {
	return h.run("repo", "update")
}

type FetchOpts struct {
	Version, Destination string
}

func (h Helm) Fetch(repo, chartName string, opts FetchOpts) (string, error) {
	return h.run("fetch", "--untar", "--untardir", opts.Destination, "--version", opts.Version, repo+"/"+chartName)
}

func (h Helm) DependencyBuild() (string, error) {
	return h.run("dependency", "build")
}

func (h Helm) InspectValues(values string) (string, error) {
	return h.run("inspect", "values", values)
}

type TemplateOpts struct {
	Name      string
	Namespace string
	Set       map[string]string
	Values    []string
}

func (h Helm) Template(chart string, opts TemplateOpts) (string, error) {
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
