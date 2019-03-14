package repos

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/argoproj/argo-cd/util"

	log "github.com/sirupsen/logrus"
)

type helmClient struct {
	// URL
	repoURL string
	// destination
	name                      string
	root                      string
	username, password        string
	caData, certData, keyData []byte
}

func (f factory) newHelmClient(repoURL, name, path, username, password string, caData, certData, keyData []byte) (Client, error) {
	return helmClient{repoURL, name, path, username, password, caData, certData, keyData}, nil
}

func (c helmClient) Test() error {

	err := runHelmCommand("init", "--client-only", "--skip-refresh")
	if err != nil {
		return err
	}

	err = c.addRepo(c.name, c.repoURL)
	if err != nil {
		return err
	}

	return runHelmCommand("repo", "rm", c.name)
}

func (c helmClient) addRepo(name, url string) error {

	tmp, err := ioutil.TempFile(util.TempDir, "helm")
	if err != nil {
		return err
	}
	defer func() { util.DeleteFile(tmp.Name()) }()

	args := []string{"repo", "add"}

	if c.username != "" {
		args = append([]string{"--username", c.username}, args...)
	}

	if c.password != "" {
		args = append([]string{"--password", c.password}, args...)
	}

	if c.caData != nil {
		caFile, err := ioutil.TempFile(util.TempDir, "")
		if err != nil {
			return err
		}
		_, err = caFile.Write(c.caData)
		if err != nil {
			return err
		}
		args = append([]string{"--ca-file", caFile.Name()}, args...)
	}

	if c.certData != nil {
		certFile, err := ioutil.TempFile(util.TempDir, "")
		if err != nil {
			return err
		}
		_, err = certFile.Write(c.certData)
		if err != nil {
			return err
		}
		args = append([]string{"--cert-file", certFile.Name()}, args...)
	}

	if c.keyData != nil {
		keyFile, err := ioutil.TempFile(util.TempDir, "")
		if err != nil {
			return err
		}
		_, err = keyFile.Write(c.keyData)
		if err != nil {
			return err
		}
		args = append([]string{"--key-file", keyFile.Name()}, args...)
	}

	args = append(args, name, url)

	return runHelmCommand(args...)
}

func runHelmCommand(args ...string) error {

	log.Infof("helm args=%v", args)
	bytes, err := exec.Command("helm", args...).CombinedOutput()
	log.Infof("output=%s", bytes)

	return err
}

func (c helmClient) Root() string {
	return c.root
}

func (c helmClient) Checkout(path, chartVersion string) (string, error) {

	chartName := c.chartName(path)

	url := c.repoURL + "/" + chartName + "-" + chartVersion + ".tgz"

	log.Infof("Helm checkout url=%s, root=%s, path=%s", url, c.root, path)

	_, err := exec.Command("rm", "-rf", c.root).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("unable to clean repo at %s: %v", c.root, err)
	}

	err = c.addRepo(c.name, c.repoURL)
	if err != nil {
		return "", err
	}
	err = runHelmCommand("repo", "update")
	if err != nil {
		return "", err
	}

	err = runHelmCommand("fetch", "--untar", "--untardir", c.root, url)

	return chartVersion, err
}

func (c helmClient) chartName(path string) string {
	return strings.Split(path, "/")[0]
}

func (c helmClient) ResolveRevision(revision string) (string, error) {
	return revision, nil
}

func (c helmClient) LsFiles(path string) ([]string, error) {

	chartName := c.chartName(path)
	files, err := ioutil.ReadDir(filepath.Join(c.root, chartName))
	if err != nil {
		return nil, err
	}

	var names = make([]string, 0)
	for _, f := range files {
		name := filepath.Join(chartName, f.Name())
		if ok, _ := filepath.Match(path, name); ok {
			names = append(names, name)
		}
	}

	return names, nil
}
