package repository

import (
	"os"
	"path"
	"strings"

	"fmt"

	"encoding/json"

	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/git"
	ksutil "github.com/argoproj/argo-cd/util/ksonnet"
	"golang.org/x/net/context"
	"k8s.io/client-go/kubernetes"
)

// Service implements ManifestService interface
type Service struct {
	ns         string
	kubeClient kubernetes.Interface
	gitClient  git.Client
	repoLock   *util.KeyLock
}

// NewService returns a new instance of the Manifest service
func NewService(namespace string, kubeClient kubernetes.Interface, gitClient git.Client) *Service {
	return &Service{
		ns:         namespace,
		kubeClient: kubeClient,
		gitClient:  gitClient,
		repoLock:   util.NewKeyLock(),
	}
}

// GenerateManifest generate a manifest for application in specified repo name and revision
func (s *Service) GenerateManifest(c context.Context, q *ManifestRequest) (*ManifestResponse, error) {
	appRepoPath := path.Join(os.TempDir(), strings.Replace(q.Repo.Repo, "/", "_", -1))
	s.repoLock.Lock(appRepoPath)
	defer s.repoLock.Unlock(appRepoPath)

	err := s.gitClient.CloneOrFetch(q.Repo.Repo, q.Repo.Username, q.Repo.Password, q.Repo.SSHPrivateKey, appRepoPath)
	if err != nil {
		return nil, err
	}

	err = s.gitClient.Checkout(appRepoPath, q.Revision)
	if err != nil {
		return nil, err
	}
	appPath := path.Join(appRepoPath, q.Path)
	ksApp, err := ksutil.NewKsonnetApp(appPath)
	if err != nil {
		return nil, err
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
		manifestStr, err := json.Marshal(target.Object)
		if err != nil {
			return nil, err
		}
		manifests[i] = string(manifestStr)
	}
	return &ManifestResponse{
		Manifests: manifests,
		Namespace: env.Destination.Namespace,
		Server:    env.Destination.Server,
	}, nil
}

// GetEnvParams retrieves Ksonnet environment params in specified repo name and revision
func (s *Service) GetEnvParams(c context.Context, q *EnvParamsRequest) (*EnvParamsResponse, error) {
	appRepoPath := path.Join(os.TempDir(), strings.Replace(q.Repo.Repo, "/", "_", -1))
	s.repoLock.Lock(appRepoPath)
	defer s.repoLock.Unlock(appRepoPath)

	err := s.gitClient.CloneOrFetch(q.Repo.Repo, q.Repo.Username, q.Repo.Password, q.Repo.SSHPrivateKey, appRepoPath)
	if err != nil {
		return nil, err
	}

	err = s.gitClient.Checkout(appRepoPath, q.Revision)
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
