package repos

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"

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
	helm, err := helmcmd.NewHelm(workDir)
	if err != nil {
		return nil, err
	}
	_, err = helm.Init()
	if err != nil {
		return nil, err
	}
	return helmClient{
		repoURL, name, *helm, workDir,
		username, password,
		caData, certData, keyData,
	}, nil
}

func (c helmClient) Test() error {

	_, err := c.repoAdd()
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

	chartName := chartName(path)
	_, err = c.helm.Fetch(c.name, chartName, helmcmd.FetchOpts{
		Version: chartVersion, Destination: ".",
	})

	// short cut
	if chartVersion == "" {
		chartVersion, err = c.getChartVersion(filepath.Join(c.workDir, chartName))
		if err != nil {
			return "", err
		}
	}

	return chartVersion, err
}

type chart struct {
	Version string
}

func (c helmClient) getChartVersion(dir string) (string, error) {
	bytes, err := ioutil.ReadFile(filepath.Join(dir, "Chart.yaml"))
	if err != nil {
		return "", err
	}

	chart := chart{}
	err = yaml.Unmarshal(bytes, &chart)
	if err != nil {
		return "", err
	}

	return chart.Version, nil
}

func chartName(path string) string {
	return strings.Split(path, "/")[0]
}

func (c helmClient) ResolveRevision(path, revision string) (string, error) {

	// empty string string means latest, otherwise we have a non-ambiguous version
	if revision == "" {
		return c.Checkout(path, revision)
	} else {
		return revision, nil
	}
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
