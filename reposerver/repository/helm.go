package repository

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	argoexec "github.com/argoproj/pkg/exec"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/helm"
)

func (s *Service) GetHelmCharts(ctx context.Context, q *apiclient.HelmChartsRequest) (*apiclient.HelmChartsResponse, error) {
	index, err := helm.GetIndex(q.Repo.Repo, q.Repo.Username, q.Repo.Password)
	if err != nil {
		return nil, err
	}
	res := apiclient.HelmChartsResponse{}
	for chartName, entries := range index.Entries {
		chart := apiclient.HelmChart{
			Name: chartName,
		}
		for _, entry := range entries {
			chart.Versions = append(chart.Versions, entry.Version)
		}
		res.Items = append(res.Items, &chart)
	}
	return &res, nil
}

func (s *Service) helmChartRepoPath(repo string) (string, error) {
	repoPath := tempRepoPath(repo)

	s.repoLock.Lock(repoPath)
	defer s.repoLock.Unlock(repoPath)

	err := os.Mkdir(repoPath, 0700)
	if err != nil && !os.IsExist(err) {
		return "", err
	}
	return repoPath, nil
}

func fileExist(filePath string) (bool, error) {
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		} else {
			return false, err
		}
	}
	return true, nil
}

func (s *Service) checkoutChart(repo *v1alpha1.Repository, chartName string, version string) (string, util.Closer, error) {
	repoPath, err := s.helmChartRepoPath(repo.Repo)
	if err != nil {
		return "", nil, err
	}
	chartFile := fmt.Sprintf("%s-%s.tgz", chartName, version)
	chartPath := path.Join(repoPath, chartFile)

	s.repoLock.Lock(chartPath)
	defer s.repoLock.Unlock(chartPath)

	exists, err := fileExist(chartPath)
	if err != nil {
		return "", nil, err
	}
	if !exists {
		helmCmd, err := helm.NewCmd(repoPath)
		if err != nil {
			return "", nil, err
		}
		defer helmCmd.Close()

		_, err = helmCmd.Init()
		if err != nil {
			return "", nil, err
		}

		_, err = helmCmd.RepoUpdate()
		if err != nil {
			return "", nil, err
		}

		// download chart tar file into persistent helm repository directory
		_, err = helmCmd.Fetch(
			repo.Repo, chartName, helm.FetchOpts{Version: version, CAData: repo.TLSClientCAData, CertData: repo.TLSClientCertData, CertKey: repo.TLSClientCertKey, Username: repo.Username, Password: repo.Password})
		if err != nil {
			return "", nil, err
		}
	}
	// untar helm chart into throw away temp directory which should be deleted as soon as no longer needed
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", nil, err
	}
	cmd := exec.Command("tar", "-zxvf", chartPath)
	cmd.Dir = tempDir
	_, err = argoexec.RunCommandExt(cmd, argoexec.CmdOpts{})
	if err != nil {
		_ = os.RemoveAll(tempDir)
	}
	return path.Join(tempDir, chartName), util.NewCloser(func() error {
		return os.RemoveAll(tempDir)
	}), nil
}
