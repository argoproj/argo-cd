package repository

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/git"
	ksutil "github.com/argoproj/argo-cd/util/ksonnet"
	"github.com/argoproj/argo-cd/util/kube"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Service implements ManifestService interface
type Service struct {
	repoLock   *util.KeyLock
	gitFactory git.ClientFactory
}

// NewService returns a new instance of the Manifest service
func NewService(gitFactory git.ClientFactory) *Service {
	return &Service{
		repoLock:   util.NewKeyLock(),
		gitFactory: gitFactory,
	}
}

func (s *Service) GetFile(ctx context.Context, q *GetFileRequest) (*GetFileResponse, error) {
	appRepoPath := tempRepoPath(q.Repo.Repo)
	s.repoLock.Lock(appRepoPath)
	gitClient := s.gitFactory.NewClient(q.Repo.Repo, appRepoPath, q.Repo.Username, q.Repo.Password, q.Repo.SSHPrivateKey)
	defer s.unlockAndResetRepoPath(gitClient)

	err := checkoutRevision(gitClient, q.Revision)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadFile(path.Join(gitClient.Root(), q.Path))
	if err != nil {
		return nil, err
	}
	res := GetFileResponse{
		Data: data,
	}
	return &res, nil
}

func (s *Service) GenerateManifest(c context.Context, q *ManifestRequest) (*ManifestResponse, error) {
	appRepoPath := tempRepoPath(q.Repo.Repo)
	s.repoLock.Lock(appRepoPath)
	gitClient := s.gitFactory.NewClient(q.Repo.Repo, appRepoPath, q.Repo.Username, q.Repo.Password, q.Repo.SSHPrivateKey)
	defer s.unlockAndResetRepoPath(gitClient)

	err := checkoutRevision(gitClient, q.Revision)
	if err != nil {
		return nil, err
	}
	commitSHA, err := gitClient.CommitSHA()
	if err != nil {
		return nil, err
	}

	appPath := path.Join(appRepoPath, q.Path)
	ksApp, err := ksutil.NewKsonnetApp(appPath)
	if err != nil {
		return nil, fmt.Errorf("unable to load application from %s: %v", appPath, err)
	}

	if q.ComponentParameterOverrides != nil {
		for _, override := range q.ComponentParameterOverrides {
			err = ksApp.SetComponentParams(q.Environment, override.Component, override.Name, override.Value)
			if err != nil {
				return nil, err
			}
		}
	}

	appSpec := ksApp.App()
	env, err := appSpec.Environment(q.Environment)
	if err != nil {
		return nil, fmt.Errorf("environment '%s' does not exist in ksonnet app", q.Environment)
	}

	targetObjs, err := ksApp.Show(q.Environment)
	if err != nil {
		return nil, err
	}
	manifests := make([]string, len(targetObjs))
	for i, target := range targetObjs {
		if q.AppLabel != "" {
			err = s.setAppLabels(target, q.AppLabel)
			if err != nil {
				return nil, err
			}
		}
		manifestStr, err := json.Marshal(target.Object)
		if err != nil {
			return nil, err
		}
		manifests[i] = string(manifestStr)
	}
	return &ManifestResponse{
		Revision:  commitSHA,
		Manifests: manifests,
		Namespace: env.Destination.Namespace,
		Server:    env.Destination.Server,
	}, nil
}

// setAppLabels sets our app labels against an unstructured object
func (s *Service) setAppLabels(target *unstructured.Unstructured, appName string) error {
	labels := target.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[common.LabelApplicationName] = appName
	target.SetLabels(labels)
	// special case for deployment: make sure that derived replicaset and pod has application label
	if target.GetKind() == kube.DeploymentKind {
		labels, ok := unstructured.NestedMap(target.UnstructuredContent(), "spec", "template", "metadata", "labels")
		if ok {
			if labels == nil {
				labels = make(map[string]interface{})
			}
			labels[common.LabelApplicationName] = appName
		}
		unstructured.SetNestedMap(target.UnstructuredContent(), labels, "spec", "template", "metadata", "labels")
	}
	return nil
}

// GetEnvParams retrieves Ksonnet environment params in specified repo name and revision
func (s *Service) GetEnvParams(c context.Context, q *EnvParamsRequest) (*EnvParamsResponse, error) {
	appRepoPath := tempRepoPath(q.Repo.Repo)
	s.repoLock.Lock(appRepoPath)
	gitClient := s.gitFactory.NewClient(q.Repo.Repo, appRepoPath, q.Repo.Username, q.Repo.Password, q.Repo.SSHPrivateKey)
	defer s.unlockAndResetRepoPath(gitClient)

	err := checkoutRevision(gitClient, q.Revision)
	if err != nil {
		return nil, err
	}

	appPath := path.Join(appRepoPath, q.Path)
	ksApp, err := ksutil.NewKsonnetApp(appPath)
	if err != nil {
		return nil, err
	}
	target, err := ksApp.ListEnvParams(q.Environment)
	if err != nil {
		return nil, err
	}

	return &EnvParamsResponse{
		Params: target,
	}, nil
}

// tempRepoPath returns a formulated temporary directory location to clone a repository
func tempRepoPath(repo string) string {
	return path.Join(os.TempDir(), strings.Replace(repo, "/", "_", -1))
}

// unlockAndResetRepoPath will reset any local changes in a local git repo and unlock the path
// so that other workers can use the local repo
func (s *Service) unlockAndResetRepoPath(gitClient git.Client) {
	err := gitClient.Reset()
	if err != nil {
		log.Warn(err)
	}
	s.repoLock.Unlock(gitClient.Root())
}

// checkoutRevision is a convenience function to initialize a repo, fetch, and checkout a revision
func checkoutRevision(gitClient git.Client, revision string) error {
	err := gitClient.Init()
	if err != nil {
		return err
	}
	err = gitClient.Fetch()
	if err != nil {
		return err
	}
	err = gitClient.Checkout(revision)
	if err != nil {
		return err
	}
	return nil
}
