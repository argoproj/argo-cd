package repos

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	helmcmd "github.com/argoproj/argo-cd/util/helm/cmd"
)

type helmClient struct {
	repoURL                   string
	name                      string
	helm                      helmcmd.Helm
	workDir                   string
	username, password        string
	caData, certData, keyData []byte
}

func (f factory) newHelmClient(repoURL, name, workDir, username, password string, caData, certData, keyData []byte) (Client, error) {
	return helmClient{
		repoURL, name, helmcmd.NewHelm(workDir), workDir,
		username, password,
		caData, certData, keyData,
	}, nil
}

func (c helmClient) Test() error {

	_, err := c.helm.Init()
	if err != nil {
		return err
	}

	_, err = c.repoAdd()
	if err != nil {
		return err
	}

	_, err = c.helm.RepoRm(c.name)
	return err
}

func (c helmClient) repoAdd() (string, error) {
	return c.helm.RepoAdd(c.name, c.repoURL, helmcmd.RepoAddOpts{
		Username: c.username, Password: c.password,
		CAData: c.caData, CertData: c.certData, KeyData: c.keyData,
	})
}

func (c helmClient) WorkDir() string {
	return c.workDir
}

func (c helmClient) Checkout(path, chartVersion string) (string, error) {

	_, err := c.repoAdd()
	if err != nil {
		return "", err
	}

	_, err = c.helm.RepoUpdate()
	if err != nil {
		return "", err
	}

	_, err = c.helm.Fetch(c.name, chartName(path), helmcmd.FetchOpts{
		Version: chartVersion, Destination: path,
	})
	return chartVersion, err
}

func chartName(path string) string {
	return strings.Split(path, "/")[0]
}

func (c helmClient) ResolveRevision(revision string) (string, error) {
	return revision, nil
}

func (c helmClient) LsFiles(path string) ([]string, error) {

	chartName := chartName(path)
	files, err := ioutil.ReadDir(filepath.Join(c.workDir, chartName))
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
