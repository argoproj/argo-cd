package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/git"
	ksutil "github.com/argoproj/argo-cd/util/ksonnet"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func (s *Service) GetKsonnetApp(ctx context.Context, in *KsonnetAppRequest) (*KsonnetAppResponse, error) {
	appRepoPath := tempRepoPath(in.Repo.Repo)
	s.repoLock.Lock(appRepoPath)
	defer s.unlockAndResetRepoPath(appRepoPath)
	ksApp, err := s.getAppSpec(*in.Repo, appRepoPath, in.Revision, in.Path)
	if err != nil {
		return nil, err
	}
	return ksAppToResponse(ksApp)
}

// ksAppToResponse converts a Ksonnet app instance to a API response object
func ksAppToResponse(ksApp ksutil.KsonnetApp) (*KsonnetAppResponse, error) {
	var appRes KsonnetAppResponse
	appRes.Environments = make(map[string]*KsonnetEnvironment)
	for envName, env := range ksApp.Spec().Environments {
		if env.Destination == nil {
			return nil, fmt.Errorf("Environment '%s' has no destination defined", envName)
		}
		envRes := KsonnetEnvironment{
			Name:       envName,
			K8SVersion: env.KubernetesVersion,
			Path:       env.Path,
			Destination: &KsonnetEnvironmentDestination{
				Server:    env.Destination.Server,
				Namespace: env.Destination.Namespace,
			},
		}
		appRes.Environments[envName] = &envRes
	}
	return &appRes, nil
}

func (s *Service) GenerateManifest(c context.Context, q *ManifestRequest) (*ManifestResponse, error) {
	appRepoPath := tempRepoPath(q.Repo.Repo)
	s.repoLock.Lock(appRepoPath)
	defer s.unlockAndResetRepoPath(appRepoPath)

	err := s.gitClient.CloneOrFetch(q.Repo.Repo, q.Repo.Username, q.Repo.Password, q.Repo.SSHPrivateKey, appRepoPath)
	if err != nil {
		return nil, err
	}

	revision, err := s.gitClient.Checkout(appRepoPath, q.Revision)
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
		Revision:  revision,
		Manifests: manifests,
		Namespace: env.Destination.Namespace,
		Server:    env.Destination.Server,
	}, nil
}

func (s *Service) getAppSpec(repo v1alpha1.Repository, appRepoPath, revision, subPath string) (ksutil.KsonnetApp, error) {
	err := s.gitClient.CloneOrFetch(repo.Repo, repo.Username, repo.Password, repo.SSHPrivateKey, appRepoPath)
	if err != nil {
		return nil, err
	}

	_, err = s.gitClient.Checkout(appRepoPath, revision)
	if err != nil {
		return nil, err
	}
	appPath := path.Join(appRepoPath, subPath)
	ksApp, err := ksutil.NewKsonnetApp(appPath)
	if err != nil {
		return nil, err
	}
	return ksApp, nil
}

func (s *Service) setAppLabels(target *unstructured.Unstructured, appName string) error {
	labels := target.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[common.LabelApplicationName] = appName
	target.SetLabels(labels)
	// special case for deployment: make sure that derived replicaset and pod has application label
	if target.GetKind() == "Deployment" {
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
	defer s.repoLock.Unlock(appRepoPath)

	err := s.gitClient.CloneOrFetch(q.Repo.Repo, q.Repo.Username, q.Repo.Password, q.Repo.SSHPrivateKey, appRepoPath)
	if err != nil {
		return nil, err
	}

	_, err = s.gitClient.Checkout(appRepoPath, q.Revision)
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
func (s *Service) unlockAndResetRepoPath(appRepoPath string) {
	err := s.gitClient.Reset(appRepoPath)
	if err != nil {
		log.Warn(err)
	}
	s.repoLock.Unlock(appRepoPath)
}
