package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/ksonnet/ksonnet/pkg/app"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/helm"
	ksutil "github.com/argoproj/argo-cd/util/ksonnet"
	"github.com/argoproj/argo-cd/util/kube"
)

const (
	// DefaultRepoCacheExpiration is the duration for items to live in the repo cache
	DefaultRepoCacheExpiration = 24 * time.Hour
)

type AppSourceType string

const (
	AppSourceKsonnet   AppSourceType = "ksonnet"
	AppSourceHelm      AppSourceType = "helm"
	AppSourceDirectory AppSourceType = "directory"
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
		log.Infof("listdir cache hit: %s", cacheKey)
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
		log.Warnf("listdir cache set error %s: %v", cacheKey, err)
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
	var res ManifestResponse
	if git.IsCommitSHA(q.Revision) {
		cacheKey := manifestCacheKey(q.Revision, q)
		err := s.cache.Get(cacheKey, res)
		if err == nil {
			log.Infof("manifest cache hit: %s", cacheKey)
			return &res, nil
		}
	}
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

	genRes, err := generateManifests(appPath, q)
	if err != nil {
		return nil, err
	}
	res = *genRes
	res.Revision = commitSHA
	err = s.cache.Set(&cache.Item{
		Key:        cacheKey,
		Object:     res,
		Expiration: DefaultRepoCacheExpiration,
	})
	if err != nil {
		log.Warnf("manifest cache set error %s: %v", cacheKey, err)
	}
	return &res, nil
}

// generateManifests generates manifests from a path
func generateManifests(appPath string, q *ManifestRequest) (*ManifestResponse, error) {
	var targetObjs []*unstructured.Unstructured
	var params []*v1alpha1.ComponentParameter
	var env *app.EnvironmentSpec
	var err error

	appSourceType := identifyAppSourceType(appPath)
	switch appSourceType {
	case AppSourceKsonnet:
		targetObjs, params, env, err = ksShow(appPath, q.Environment, q.ComponentParameterOverrides)
	case AppSourceHelm:
		h := helm.NewHelmApp(appPath)
		targetObjs, err = h.Template(q.AppLabel, q.ValueFiles, q.ComponentParameterOverrides)
	case AppSourceDirectory:
		targetObjs, err = findManifests(appPath)
	}
	if err != nil {
		return nil, err
	}
	// TODO(jessesuen): we need to sort objects based on their dependency order of creation

	manifests := make([]string, len(targetObjs))
	for i, target := range targetObjs {
		if q.AppLabel != "" {
			err = kube.SetLabel(target, common.LabelApplicationName, q.AppLabel)
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

	res := ManifestResponse{
		Manifests: manifests,
		Params:    params,
	}
	if env != nil {
		res.Namespace = env.Destination.Namespace
		res.Server = env.Destination.Server
	}
	return &res, nil
}

// tempRepoPath returns a formulated temporary directory location to clone a repository
func tempRepoPath(repo string) string {
	return path.Join(os.TempDir(), strings.Replace(repo, "/", "_", -1))
}

// identifyAppSourceType examines a directory and determines its application source type
func identifyAppSourceType(appPath string) AppSourceType {
	if pathExists(path.Join(appPath, "app.yaml")) {
		return AppSourceKsonnet
	}
	if pathExists(path.Join(appPath, "Chart.yaml")) {
		return AppSourceHelm
	}
	return AppSourceDirectory
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
	valuesFiles := strings.Join(q.ValueFiles, ",")
	return fmt.Sprintf("mfst|%s|%s|%s|%s|%s|%s", q.AppLabel, q.Path, q.Environment, commitSHA, string(pStr), valuesFiles)
}

func listDirCacheKey(commitSHA string, q *ListDirRequest) string {
	return fmt.Sprintf("ldir|%s|%s", q.Path, commitSHA)
}

// ksShow runs `ks show` in an app directory after setting any component parameter overrides
func ksShow(appPath, envName string, overrides []*v1alpha1.ComponentParameter) ([]*unstructured.Unstructured, []*v1alpha1.ComponentParameter, *app.EnvironmentSpec, error) {
	ksApp, err := ksutil.NewKsonnetApp(appPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unable to load application from %s: %v", appPath, err)
	}
	params, err := ksApp.ListEnvParams(envName)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Failed to list ksonnet app params: %v", err)
	}
	if overrides != nil {
		for _, override := range overrides {
			err = ksApp.SetComponentParams(envName, override.Component, override.Name, override.Value)
			if err != nil {
				return nil, nil, nil, err
			}
		}
	}
	appSpec := ksApp.App()
	env, err := appSpec.Environment(envName)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("environment '%s' does not exist in ksonnet app", envName)
	}
	targetObjs, err := ksApp.Show(envName)
	if err != nil {
		return nil, nil, nil, err
	}
	return targetObjs, params, env, nil
}

var manifestFile = regexp.MustCompile(`^.*\.(yaml|yml|json)$`)

// findManifests looks at all yaml files in a directory and unmarshals them into a list of unstructured objects
func findManifests(appPath string) ([]*unstructured.Unstructured, error) {
	files, err := ioutil.ReadDir(appPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to read dir %s: %v", appPath, err)
	}
	var objs []*unstructured.Unstructured
	for _, f := range files {
		if f.IsDir() || !manifestFile.MatchString(f.Name()) {
			continue
		}
		out, err := ioutil.ReadFile(path.Join(appPath, f.Name()))
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(f.Name(), ".json") {
			var obj unstructured.Unstructured
			err = json.Unmarshal(out, &obj)
			if err != nil {
				return nil, fmt.Errorf("Failed to unmarshal '%s': %v", f.Name(), err)
			}
			objs = append(objs, &obj)
		} else {
			yamlObjs, err := kube.SplitYAML(string(out))
			if err != nil {
				if len(yamlObjs) > 0 {
					// If we get here, we had a multiple objects in a single YAML file which had some
					// valid k8s objects, but errors parsing others (within the same file). It's very
					// likely the user messed up a portion of the YAML, so report on that.
					return nil, fmt.Errorf("Failed to unmarshal '%s': %v", f.Name(), err)
				}
				// Otherwise, it might be a unrelated YAML file which we will ignore
				continue
			}
			objs = append(objs, yamlObjs...)
		}
	}
	return objs, nil
}

// pathExists reports whether the named file or directory exists.
func pathExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
