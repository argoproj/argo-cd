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

	"github.com/google/go-jsonnet"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/config"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/helm"
	"github.com/argoproj/argo-cd/util/ksonnet"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/kustomize"
)

const (
	// DefaultRepoCacheExpiration is the duration for items to live in the repo cache
	DefaultRepoCacheExpiration = 24 * time.Hour
)

type AppSourceType string

const (
	AppSourceKsonnet   AppSourceType = "ksonnet"
	AppSourceHelm      AppSourceType = "helm"
	AppSourceKustomize AppSourceType = "kustomize"
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
	gitClient, commitSHA, err := s.newClientResolveRevision(q.Repo, q.Revision)
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

	s.repoLock.Lock(gitClient.Root())
	defer s.repoLock.Unlock(gitClient.Root())
	commitSHA, err = checkoutRevision(gitClient, commitSHA)
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
		Key:        listDirCacheKey(commitSHA, q),
		Object:     &res,
		Expiration: DefaultRepoCacheExpiration,
	})
	if err != nil {
		log.Warnf("listdir cache set error %s: %v", cacheKey, err)
	}
	return &res, nil
}

func (s *Service) GetFile(ctx context.Context, q *GetFileRequest) (*GetFileResponse, error) {
	gitClient, commitSHA, err := s.newClientResolveRevision(q.Repo, q.Revision)
	if err != nil {
		return nil, err
	}
	cacheKey := getFileCacheKey(commitSHA, q)
	var res GetFileResponse
	err = s.cache.Get(cacheKey, &res)
	if err == nil {
		log.Infof("getfile cache hit: %s", cacheKey)
		return &res, nil
	}

	s.repoLock.Lock(gitClient.Root())
	defer s.repoLock.Unlock(gitClient.Root())
	commitSHA, err = checkoutRevision(gitClient, commitSHA)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadFile(path.Join(gitClient.Root(), q.Path))
	if err != nil {
		return nil, err
	}
	res = GetFileResponse{
		Data: data,
	}
	err = s.cache.Set(&cache.Item{
		Key:        getFileCacheKey(commitSHA, q),
		Object:     &res,
		Expiration: DefaultRepoCacheExpiration,
	})
	if err != nil {
		log.Warnf("getfile cache set error %s: %v", cacheKey, err)
	}
	return &res, nil
}

func (s *Service) GenerateManifest(c context.Context, q *ManifestRequest) (*ManifestResponse, error) {
	gitClient, commitSHA, err := s.newClientResolveRevision(q.Repo, q.Revision)
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

	s.repoLock.Lock(gitClient.Root())
	defer s.repoLock.Unlock(gitClient.Root())
	commitSHA, err = checkoutRevision(gitClient, commitSHA)
	if err != nil {
		return nil, err
	}
	appPath := path.Join(gitClient.Root(), q.Path)

	genRes, err := generateManifests(appPath, q)
	if err != nil {
		return nil, err
	}
	res = *genRes
	res.Revision = commitSHA
	err = s.cache.Set(&cache.Item{
		Key:        manifestCacheKey(commitSHA, q),
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
	var dest *v1alpha1.ApplicationDestination

	appSourceType, err := IdentifyAppSourceTypeByAppDir(appPath)
	if err != nil {
		return nil, err
	}
	switch appSourceType {
	case AppSourceKsonnet:
		targetObjs, params, dest, err = ksShow(appPath, q.Environment, q.ComponentParameterOverrides)
	case AppSourceHelm:
		h := helm.NewHelmApp(appPath)
		err = h.DependencyBuild()
		if err != nil {
			return nil, err
		}
		targetObjs, err = h.Template(q.AppLabel, q.Namespace, q.ValueFiles, q.ComponentParameterOverrides)
		if err != nil {
			return nil, err
		}
		params, err = h.GetParameters(q.ValueFiles)
		if err != nil {
			return nil, err
		}
	case AppSourceKustomize:
		k := kustomize.NewKustomizeApp(appPath)
		targetObjs, err = k.Build()
	case AppSourceDirectory:
		targetObjs, err = findManifests(appPath)
	}
	if err != nil {
		return nil, err
	}

	manifests := make([]string, 0)
	for _, obj := range targetObjs {
		var targets []*unstructured.Unstructured
		if obj.IsList() {
			err = obj.EachListItem(func(object runtime.Object) error {
				unstructuredObj, ok := object.(*unstructured.Unstructured)
				if ok {
					targets = append(targets, unstructuredObj)
					return nil
				} else {
					return fmt.Errorf("resource list item has unexpected type")
				}
			})
			if err != nil {
				return nil, err
			}
		} else {
			targets = []*unstructured.Unstructured{obj}
		}

		for _, target := range targets {
			if q.AppLabel != "" && !kube.IsCRD(target) {
				err = kube.SetLabel(target, common.LabelApplicationName, q.AppLabel)
				if err != nil {
					return nil, err
				}
			}
			manifestStr, err := json.Marshal(target.Object)
			if err != nil {
				return nil, err
			}
			manifests = append(manifests, string(manifestStr))
		}
	}

	res := ManifestResponse{
		Manifests: manifests,
		Params:    params,
	}
	if dest != nil {
		res.Namespace = dest.Namespace
		res.Server = dest.Server
	}
	return &res, nil
}

// tempRepoPath returns a formulated temporary directory location to clone a repository
func tempRepoPath(repo string) string {
	return path.Join(os.TempDir(), strings.Replace(repo, "/", "_", -1))
}

func isValidKsonnet(appDirPath string) bool {
	var m map[string]interface{}
	err := config.UnmarshalLocalFile(appDirPath, &m)
	if err != nil {
		return false
	}
	v, ok := m["kind"]
	return ok && v == "ksonnet.io/app"
}

// IdentifyAppSourceTypeByAppDir examines a directory and determines its application source type
func IdentifyAppSourceTypeByAppDir(appDirPath string) (AppSourceType, error) {
	if p := path.Join(appDirPath, "app.yaml"); pathExists(p) {
		if !isValidKsonnet(p) {
			return "", fmt.Errorf("invalid Ksonnet manifest at path %s", p)
		}
		return AppSourceKsonnet, nil
	}
	if pathExists(path.Join(appDirPath, "Chart.yaml")) {
		return AppSourceHelm, nil
	}
	if pathExists(path.Join(appDirPath, "kustomization.yaml")) {
		return AppSourceKustomize, nil
	}
	return AppSourceDirectory, nil
}

// IdentifyAppSourceTypeByAppPath determines application source type by app file path
func IdentifyAppSourceTypeByAppPath(appFilePath string) (AppSourceType, error) {
	if strings.HasSuffix(appFilePath, "app.yaml") {
		if !isValidKsonnet(appFilePath) {
			return "", fmt.Errorf("invalid Ksonnet manifest at path %s", appFilePath)
		}
		return AppSourceKsonnet, nil
	}
	if strings.HasSuffix(appFilePath, "Chart.yaml") {
		return AppSourceHelm, nil
	}
	if strings.HasSuffix(appFilePath, "kustomization.yaml") {
		return AppSourceKustomize, nil
	}
	return AppSourceDirectory, nil
}

// checkoutRevision is a convenience function to initialize a repo, fetch, and checkout a revision
// Returns the 40 character commit SHA after the checkout has been performed
func checkoutRevision(gitClient git.Client, commitSHA string) (string, error) {
	err := gitClient.Init()
	if err != nil {
		return "", status.Errorf(codes.Internal, "Failed to initialize git repo: %v", err)
	}
	err = gitClient.Fetch()
	if err != nil {
		return "", status.Errorf(codes.Internal, "Failed to fetch git repo: %v", err)
	}
	err = gitClient.Checkout(commitSHA)
	if err != nil {
		return "", status.Errorf(codes.Internal, "Failed to checkout %s: %v", commitSHA, err)
	}
	return gitClient.CommitSHA()
}

func manifestCacheKey(commitSHA string, q *ManifestRequest) string {
	pStr, _ := json.Marshal(q.ComponentParameterOverrides)
	valuesFiles := strings.Join(q.ValueFiles, ",")
	return fmt.Sprintf("mfst|%s|%s|%s|%s|%s|%s|%s", q.AppLabel, q.Path, q.Environment, commitSHA, string(pStr), valuesFiles, q.Namespace)
}

func listDirCacheKey(commitSHA string, q *ListDirRequest) string {
	return fmt.Sprintf("ldir|%s|%s", q.Path, commitSHA)
}

func getFileCacheKey(commitSHA string, q *GetFileRequest) string {
	return fmt.Sprintf("gfile|%s|%s", q.Path, commitSHA)
}

// ksShow runs `ks show` in an app directory after setting any component parameter overrides
func ksShow(appPath, envName string, overrides []*v1alpha1.ComponentParameter) ([]*unstructured.Unstructured, []*v1alpha1.ComponentParameter, *v1alpha1.ApplicationDestination, error) {
	ksApp, err := ksonnet.NewKsonnetApp(appPath)
	if err != nil {
		return nil, nil, nil, status.Errorf(codes.FailedPrecondition, "unable to load application from %s: %v", appPath, err)
	}
	params, err := ksApp.ListEnvParams(envName)
	if err != nil {
		return nil, nil, nil, status.Errorf(codes.InvalidArgument, "Failed to list ksonnet app params: %v", err)
	}
	if overrides != nil {
		for _, override := range overrides {
			err = ksApp.SetComponentParams(envName, override.Component, override.Name, override.Value)
			if err != nil {
				return nil, nil, nil, err
			}
		}
	}
	dest, err := ksApp.Destination(envName)
	if err != nil {
		return nil, nil, nil, status.Errorf(codes.NotFound, "environment %q does not exist in ksonnet app", envName)
	}
	targetObjs, err := ksApp.Show(envName)
	if err != nil {
		return nil, nil, nil, err
	}
	return targetObjs, params, dest, nil
}

var manifestFile = regexp.MustCompile(`^.*\.(yaml|yml|json|jsonnet)$`)

// findManifests looks at all yaml files in a directory and unmarshals them into a list of unstructured objects
func findManifests(appPath string) ([]*unstructured.Unstructured, error) {
	files, err := ioutil.ReadDir(appPath)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "Failed to read dir %s: %v", appPath, err)
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
				return nil, status.Errorf(codes.FailedPrecondition, "Failed to unmarshal %q: %v", f.Name(), err)
			}
			objs = append(objs, &obj)
		} else if strings.HasSuffix(f.Name(), ".jsonnet") {
			vm := jsonnet.MakeVM()
			vm.Importer(&jsonnet.FileImporter{
				JPaths: []string{appPath},
			})
			jsonStr, err := vm.EvaluateSnippet(f.Name(), string(out))
			if err != nil {
				return nil, status.Errorf(codes.FailedPrecondition, "Failed to evaluate jsonnet %q: %v", f.Name(), err)
			}

			// attempt to unmarshal either array or single object
			var jsonObjs []*unstructured.Unstructured
			err = json.Unmarshal([]byte(jsonStr), &jsonObjs)
			if err == nil {
				objs = append(objs, jsonObjs...)
			} else {
				var jsonObj unstructured.Unstructured
				err = json.Unmarshal([]byte(jsonStr), &jsonObj)
				if err != nil {
					return nil, status.Errorf(codes.FailedPrecondition, "Failed to unmarshal generated json %q: %v", f.Name(), err)
				}
				objs = append(objs, &jsonObj)
			}
		} else {
			yamlObjs, err := kube.SplitYAML(string(out))
			if err != nil {
				if len(yamlObjs) > 0 {
					// If we get here, we had a multiple objects in a single YAML file which had some
					// valid k8s objects, but errors parsing others (within the same file). It's very
					// likely the user messed up a portion of the YAML, so report on that.
					return nil, status.Errorf(codes.FailedPrecondition, "Failed to unmarshal %q: %v", f.Name(), err)
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

// newClientResolveRevision is a helper to perform the common task of instantiating a git client
// and resolving a revision to a commit SHA
func (s *Service) newClientResolveRevision(repo *v1alpha1.Repository, revision string) (git.Client, string, error) {
	appRepoPath := tempRepoPath(repo.Repo)
	gitClient, err := s.gitFactory.NewClient(repo.Repo, appRepoPath, repo.Username, repo.Password, repo.SSHPrivateKey)
	if err != nil {
		return nil, "", err
	}
	commitSHA, err := gitClient.LsRemote(revision)
	if err != nil {
		return nil, "", err
	}
	return gitClient, commitSHA, nil
}
