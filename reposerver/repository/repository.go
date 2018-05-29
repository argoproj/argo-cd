package repository

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/git"
	ksutil "github.com/argoproj/argo-cd/util/ksonnet"
	"github.com/argoproj/argo-cd/util/kube"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	// DefaultRepoCacheExpiration is the duration for items to live in the repo cache
	DefaultRepoCacheExpiration = 24 * time.Hour
)

// Service implements ManifestService interface
type Service struct {
	repoLock   *util.KeyLock
	gitFactory git.ClientFactory
	cache      cache.Cache
}

// NewService returns a new instance of the Manifest service
func NewService(gitFactory git.ClientFactory, cache cache.Cache) *Service {
	return &Service{
		repoLock:   util.NewKeyLock(),
		gitFactory: gitFactory,
		cache:      cache,
	}
}

// ListDir lists the contents of a GitHub repo
func (s *Service) ListDir(ctx context.Context, q *ListDirRequest) (*FileList, error) {
	appRepoPath := tempRepoPath(q.Repo.Repo)
	s.repoLock.Lock(appRepoPath)
	defer s.repoLock.Unlock(appRepoPath)

	gitClient := s.gitFactory.NewClient(q.Repo.Repo, appRepoPath, q.Repo.Username, q.Repo.Password, q.Repo.SSHPrivateKey)
	err := gitClient.Init()
	if err != nil {
		return nil, err
	}

	commitSHA, err := gitClient.LsRemote(q.Revision)
	if err != nil {
		return nil, err
	}
	cacheKey := listDirCacheKey(commitSHA, q)
	var res FileList
	err = s.cache.Get(cacheKey, &res)
	if err == nil {
		log.Infof("manifest cache hit: %s", cacheKey)
		return &res, nil
	}

	err = checkoutRevision(gitClient, q.Revision)
	if err != nil {
		return nil, err
	}

	lsFiles, err := gitClient.LsFiles(q.Path)
	if err != nil {
		return nil, err
	}

	res = FileList{
		Items: lsFiles,
	}
	err = s.cache.Set(&cache.Item{
		Key:        cacheKey,
		Object:     &res,
		Expiration: DefaultRepoCacheExpiration,
	})
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (s *Service) GetFile(ctx context.Context, q *GetFileRequest) (*GetFileResponse, error) {
	appRepoPath := tempRepoPath(q.Repo.Repo)
	s.repoLock.Lock(appRepoPath)
	defer s.repoLock.Unlock(appRepoPath)

	gitClient := s.gitFactory.NewClient(q.Repo.Repo, appRepoPath, q.Repo.Username, q.Repo.Password, q.Repo.SSHPrivateKey)
	err := gitClient.Init()
	if err != nil {
		return nil, err
	}
	err = checkoutRevision(gitClient, q.Revision)
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
	defer s.repoLock.Unlock(appRepoPath)

	gitClient := s.gitFactory.NewClient(q.Repo.Repo, appRepoPath, q.Repo.Username, q.Repo.Password, q.Repo.SSHPrivateKey)
	err := gitClient.Init()
	if err != nil {
		return nil, err
	}
	commitSHA, err := gitClient.LsRemote(q.Revision)
	if err != nil {
		return nil, err
	}
	cacheKey := manifestCacheKey(commitSHA, q)
	var res ManifestResponse
	err = s.cache.Get(cacheKey, &res)
	if err == nil {
		log.Infof("manifest cache hit: %s", cacheKey)
		return &res, nil
	}
	if err != cache.ErrCacheMiss {
		log.Warnf("manifest cache error %s: %v", cacheKey, err)
	} else {
		log.Infof("manifest cache miss: %s", cacheKey)
	}

	err = checkoutRevision(gitClient, q.Revision)
	if err != nil {
		return nil, err
	}
	appPath := path.Join(appRepoPath, q.Path)
	ksApp, err := ksutil.NewKsonnetApp(appPath)
	if err != nil {
		return nil, fmt.Errorf("unable to load application from %s: %v", appPath, err)
	}

	params, err := ksApp.ListEnvParams(q.Environment)
	if err != nil {
		return nil, err
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
			err = setAppLabels(target, q.AppLabel)
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
	res = ManifestResponse{
		Revision:  commitSHA,
		Manifests: manifests,
		Namespace: env.Destination.Namespace,
		Server:    env.Destination.Server,
		Params:    params,
	}
	err = s.cache.Set(&cache.Item{
		Key:        cacheKey,
		Object:     &res,
		Expiration: DefaultRepoCacheExpiration,
	})
	if err != nil {
		log.Warnf("manifest cache set error %s: %v", cacheKey, err)
	}
	return &res, nil
}

// setAppLabels sets our app labels against an unstructured object
func setAppLabels(target *unstructured.Unstructured, appName string) error {
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

// tempRepoPath returns a formulated temporary directory location to clone a repository
func tempRepoPath(repo string) string {
	return path.Join(os.TempDir(), strings.Replace(repo, "/", "_", -1))
}

// checkoutRevision is a convenience function to initialize a repo, fetch, and checkout a revision
func checkoutRevision(gitClient git.Client, revision string) error {
	err := gitClient.Fetch()
	if err != nil {
		return err
	}
	err = gitClient.Reset()
	if err != nil {
		log.Warn(err)
	}
	err = gitClient.Checkout(revision)
	if err != nil {
		return err
	}
	return nil
}

func manifestCacheKey(commitSHA string, q *ManifestRequest) string {
	pStr, _ := json.Marshal(q.ComponentParameterOverrides)
	return fmt.Sprintf("mfst|%s|%s|%s|%s", q.Path, q.Environment, commitSHA, string(pStr))
}

func listDirCacheKey(commitSHA string, q *ListDirRequest) string {
	return fmt.Sprintf("ldir|%s|%s", q.Path, commitSHA)
}
