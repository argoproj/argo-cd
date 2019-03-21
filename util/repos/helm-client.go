package repos

import (
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	glob2 "github.com/gobwas/glob"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
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

func (c helmClient) checkKnownChart(chartName string) error {
	knownChart, err := c.isKnownChart(chartName)
	if err != nil {
		return err
	}
	if !knownChart {
		return errors.Errorf("unknown chart chartName=%s", chartName)
	}
	return nil
}

func (c helmClient) isKnownChart(chartName string) (bool, error) {

	index, err := c.getIndex()
	if err != nil {
		return false, err
	}

	_, ok := index.Entries[chartName]

	return ok, nil
}

func (c helmClient) Checkout(path, revision string) (string, error) {

	_, err := c.repoAdd()
	if err != nil {
		return "", err
	}

	_, err = c.helm.RepoUpdate()
	if err != nil {
		return "", err
	}

	chartName := chartName(path)

	err = c.checkKnownChart(chartName)
	if err != nil {
		return "", err
	}

	log.WithFields(log.Fields{"chartName": chartName}).Debug("chart name")

	_, err = c.helm.Fetch(c.name, chartName, helmcmd.FetchOpts{
		Version: revision, Destination: ".",
	})

	// short cut
	if revision == "" {
		revision, err = c.getChartVersion(filepath.Join(c.workDir, chartName))
		if err != nil {
			return "", err
		}
	}

	return revision, err
}

func chartName(path string) string {
	return strings.Split(path, "/")[0]
}

type chart struct {
	Version string
}

func (c helmClient) getChartVersion(chartName string) (string, error) {
	bytes, err := ioutil.ReadFile(filepath.Join(chartName, "Chart.yaml"))
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

func (c helmClient) ResolveRevision(glob, revision string) (string, error) {

	// empty string string means latest, otherwise we have a non-ambiguous version
	if revision != "" {
		return revision, nil
	}

	return c.Checkout(glob, revision)
}

type entry struct {
	Version string
}

type index struct {
	Entries map[string][]entry
}

func (c helmClient) getIndex() (*index, error) {
	resp, err := http.Get(c.repoURL + "/index.yaml")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return nil, errors.New("failed to get index: " + resp.Status)
	}

	index := &index{}
	err = yaml.NewDecoder(resp.Body).Decode(index)
	if err != nil {
		return nil, err
	}
	return index, nil
}

func (c helmClient) LsFiles(glob string) ([]string, error) {

	matcher, err := glob2.Compile(glob)
	if err != nil {
		return nil, err
	}

	index, err := c.getIndex()
	if err != nil {
		return nil, err
	}

	// we assume that we'll have at least a Chart.yaml in each chart, and return that
	// if the glob matches it. If this is queries for (say) kustomize.yml it will never return anything.
	var paths = make([]string, 0)
	for chartName := range index.Entries {
		path := filepath.Join(chartName, "Chart.yaml")
		if matcher.Match(path) {
			paths = append(paths, path)
		}
	}

	return paths, nil
}
