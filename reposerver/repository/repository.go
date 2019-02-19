package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/go-jsonnet"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/helm"
	"github.com/argoproj/argo-cd/util/ksonnet"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/kustomize"
)

// Service implements ManifestService interface
type Service struct {
	repoLock                  *util.KeyLock
	gitFactory                git.ClientFactory
	cache                     *cache.Cache
	parallelismLimitSemaphore *semaphore.Weighted
}

// NewService returns a new instance of the Manifest service
func NewService(gitFactory git.ClientFactory, cache *cache.Cache, parallelismLimit int64) *Service {
	var parallelismLimitSemaphore *semaphore.Weighted
	if parallelismLimit > 0 {
		parallelismLimitSemaphore = semaphore.NewWeighted(parallelismLimit)
	}
	return &Service{
		parallelismLimitSemaphore: parallelismLimitSemaphore,

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
	if files, err := s.cache.GetGitListDir(commitSHA, q.Path); err == nil {
		log.Infof("listdir cache hit: %s/%s", commitSHA, q.Path)
		return &FileList{Items: files}, nil
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

	res := FileList{Items: lsFiles}
	err = s.cache.SetListDir(commitSHA, q.Path, res.Items)
	if err != nil {
		log.Warnf("listdir cache set error %s/%s: %v", commitSHA, q.Path, err)
	}
	return &res, nil
}

func (s *Service) GetFile(ctx context.Context, q *GetFileRequest) (*GetFileResponse, error) {
	gitClient, commitSHA, err := s.newClientResolveRevision(q.Repo, q.Revision)
	if err != nil {
		return nil, err
	}

	if data, err := s.cache.GetGitFile(commitSHA, q.Path); err == nil {
		log.Infof("getfile cache hit: %s/%s", commitSHA, q.Path)
		return &GetFileResponse{Data: data}, nil
	}

	s.repoLock.Lock(gitClient.Root())
	defer s.repoLock.Unlock(gitClient.Root())
	commitSHA, err = checkoutRevision(gitClient, commitSHA)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadFile(filepath.Join(gitClient.Root(), q.Path))
	if err != nil {
		return nil, err
	}
	res := GetFileResponse{
		Data: data,
	}
	err = s.cache.SetGitFile(commitSHA, q.Path, data)
	if err != nil {
		log.Warnf("getfile cache set error %s/%s: %v", commitSHA, q.Path, err)
	}
	return &res, nil
}

func (s *Service) GenerateManifest(c context.Context, q *ManifestRequest) (*ManifestResponse, error) {
	gitClient, commitSHA, err := s.newClientResolveRevision(q.Repo, q.Revision)
	if err != nil {
		return nil, err
	}
	getCached := func() *ManifestResponse {
		var res ManifestResponse
		if !q.NoCache {
			err = s.cache.GetManifests(commitSHA, q.ApplicationSource, q.ComponentParameterOverrides, q.Namespace, q.AppLabelKey, q.AppLabelValue, &res)
			if err == nil {
				log.Infof("manifest cache hit: %s/%s", q.ApplicationSource.String(), commitSHA)
				return &res
			}
			if err != cache.ErrCacheMiss {
				log.Warnf("manifest cache error %s: %v", q.ApplicationSource.String(), err)
			} else {
				log.Infof("manifest cache miss: %s/%s", q.ApplicationSource.String(), commitSHA)
			}
		}
		return nil
	}

	cached := getCached()
	if cached != nil {
		return cached, nil
	}

	s.repoLock.Lock(gitClient.Root())
	defer s.repoLock.Unlock(gitClient.Root())

	cached = getCached()
	if cached != nil {
		return cached, nil
	}

	if s.parallelismLimitSemaphore != nil {
		err = s.parallelismLimitSemaphore.Acquire(c, 1)
		if err != nil {
			return nil, err
		}
		defer s.parallelismLimitSemaphore.Release(1)
	}

	commitSHA, err = checkoutRevision(gitClient, commitSHA)
	if err != nil {
		return nil, err
	}
	appPath := filepath.Join(gitClient.Root(), q.ApplicationSource.Path)

	genRes, err := generateManifests(appPath, q)
	if err != nil {
		return nil, err
	}
	res := *genRes
	res.Revision = commitSHA
	err = s.cache.SetManifests(commitSHA, q.ApplicationSource, q.ComponentParameterOverrides, q.Namespace, q.AppLabelKey, q.AppLabelValue, &res)
	if err != nil {
		log.Warnf("manifest cache set error %s/%s: %v", q.ApplicationSource.String(), commitSHA, err)
	}
	return &res, nil
}

// helper to formulate helm template options from a manifest request
func helmOpts(q *ManifestRequest) helm.HelmTemplateOpts {
	opts := helm.HelmTemplateOpts{
		Namespace: q.Namespace,
	}
	valueFiles := v1alpha1.HelmValueFiles(q.ApplicationSource)
	if q.ApplicationSource.Helm != nil {
		opts.ValueFiles = valueFiles
	}
	return opts
}

func kustomizeOpts(q *ManifestRequest) kustomize.KustomizeBuildOpts {
	opts := kustomize.KustomizeBuildOpts{}
	if q.ApplicationSource.Kustomize != nil {
		opts.NamePrefix = q.ApplicationSource.Kustomize.NamePrefix
	}
	return opts
}

// generateManifests generates manifests from a path
func generateManifests(appPath string, q *ManifestRequest) (*ManifestResponse, error) {
	var targetObjs []*unstructured.Unstructured
	var params []*v1alpha1.ComponentParameter
	var dest *v1alpha1.ApplicationDestination
	var err error

	appSourceType := IdentifyAppSourceTypeByAppDir(appPath)
	switch appSourceType {
	case v1alpha1.ApplicationSourceTypeKsonnet:
		env := v1alpha1.KsonnetEnv(q.ApplicationSource)
		targetObjs, params, dest, err = ksShow(q.AppLabelKey, appPath, env, q.ComponentParameterOverrides)
	case v1alpha1.ApplicationSourceTypeHelm:
		h := helm.NewHelmApp(appPath, q.HelmRepos)
		err := h.Init()
		if err != nil {
			return nil, err
		}
		opts := helmOpts(q)
		targetObjs, err = h.Template(q.AppLabelValue, opts, q.ComponentParameterOverrides)
		if err != nil {
			if !helm.IsMissingDependencyErr(err) {
				return nil, err
			}
			err = h.DependencyBuild()
			if err != nil {
				return nil, err
			}
			targetObjs, err = h.Template(q.AppLabelValue, opts, q.ComponentParameterOverrides)
			if err != nil {
				return nil, err
			}
		}
		params, err = h.GetParameters(opts.ValueFiles)
		if err != nil {
			return nil, err
		}
	case v1alpha1.ApplicationSourceTypeKustomize:
		k := kustomize.NewKustomizeApp(appPath)
		opts := kustomizeOpts(q)
		targetObjs, params, err = k.Build(opts, q.ComponentParameterOverrides)
	case v1alpha1.ApplicationSourceTypeDirectory:
		directoryRecurse := q.ApplicationSource.Directory != nil && q.ApplicationSource.Directory.Recurse
		targetObjs, err = FindManifests(appPath, directoryRecurse)
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
				}
				return fmt.Errorf("resource list item has unexpected type")
			})
			if err != nil {
				return nil, err
			}
		} else if isNullList(obj) {
			// noop
		} else {
			targets = []*unstructured.Unstructured{obj}
		}

		for _, target := range targets {
			if q.AppLabelKey != "" && q.AppLabelValue != "" && !kube.IsCRD(target) {
				err = kube.SetAppInstanceLabel(target, q.AppLabelKey, q.AppLabelValue)
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
	return filepath.Join(os.TempDir(), strings.Replace(repo, "/", "_", -1))
}

// IdentifyAppSourceTypeByAppDir examines a directory and determines its application source type
func IdentifyAppSourceTypeByAppDir(appDirPath string) v1alpha1.ApplicationSourceType {
	if pathExists(appDirPath, "app.yaml") {
		return v1alpha1.ApplicationSourceTypeKsonnet
	}
	if pathExists(appDirPath, "Chart.yaml") {
		return v1alpha1.ApplicationSourceTypeHelm
	}
	for _, kustomization := range kustomize.KustomizationNames {
		if pathExists(appDirPath, kustomization) {
			return v1alpha1.ApplicationSourceTypeKustomize
		}
	}
	return v1alpha1.ApplicationSourceTypeDirectory
}

// isNullList checks if the object is a "List" type where items is null instead of an empty list.
// Handles a corner case where obj.IsList() returns false when a manifest is like:
// ---
// apiVersion: v1
// items: null
// kind: ConfigMapList
func isNullList(obj *unstructured.Unstructured) bool {
	if _, ok := obj.Object["spec"]; ok {
		return false
	}
	if _, ok := obj.Object["status"]; ok {
		return false
	}
	field, ok := obj.Object["items"]
	if !ok {
		return false
	}
	return field == nil
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

// ksShow runs `ks show` in an app directory after setting any component parameter overrides
func ksShow(appLabelKey, appPath, envName string, overrides []*v1alpha1.ComponentParameter) ([]*unstructured.Unstructured, []*v1alpha1.ComponentParameter, *v1alpha1.ApplicationDestination, error) {
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
		return nil, nil, nil, status.Errorf(codes.InvalidArgument, err.Error())
	}
	targetObjs, err := ksApp.Show(envName)
	if err == nil && appLabelKey == common.LabelKeyLegacyApplicationName {
		// Address https://github.com/ksonnet/ksonnet/issues/707
		for _, d := range targetObjs {
			kube.UnsetLabel(d, "ksonnet.io/component")
		}
	}
	if err != nil {
		return nil, nil, nil, err
	}
	return targetObjs, params, dest, nil
}

var manifestFile = regexp.MustCompile(`^.*\.(yaml|yml|json|jsonnet)$`)

// FindManifests looks at all yaml files in a directory and unmarshals them into a list of unstructured objects
func FindManifests(appPath string, directoryRecurse bool) ([]*unstructured.Unstructured, error) {
	var objs []*unstructured.Unstructured
	err := filepath.Walk(appPath, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if f.IsDir() {
			if path != appPath && !directoryRecurse {
				return filepath.SkipDir
			} else {
				return nil
			}
		}

		if !manifestFile.MatchString(f.Name()) {
			return nil
		}
		out, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.HasSuffix(f.Name(), ".json") {
			var obj unstructured.Unstructured
			err = json.Unmarshal(out, &obj)
			if err != nil {
				return status.Errorf(codes.FailedPrecondition, "Failed to unmarshal %q: %v", f.Name(), err)
			}
			objs = append(objs, &obj)
		} else if strings.HasSuffix(f.Name(), ".jsonnet") {
			vm := jsonnet.MakeVM()
			vm.Importer(&jsonnet.FileImporter{
				JPaths: []string{appPath},
			})
			jsonStr, err := vm.EvaluateSnippet(f.Name(), string(out))
			if err != nil {
				return status.Errorf(codes.FailedPrecondition, "Failed to evaluate jsonnet %q: %v", f.Name(), err)
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
					return status.Errorf(codes.FailedPrecondition, "Failed to unmarshal generated json %q: %v", f.Name(), err)
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
					return status.Errorf(codes.FailedPrecondition, "Failed to unmarshal %q: %v", f.Name(), err)
				}
				// Otherwise, it might be a unrelated YAML file which we will ignore
				return nil
			}
			objs = append(objs, yamlObjs...)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return objs, nil
}

// pathExists reports whether the file or directory at the named concatenation of paths exists.
func pathExists(ss ...string) bool {
	name := filepath.Join(ss...)
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
	repoURL := git.NormalizeGitURL(repo.Repo)
	appRepoPath := tempRepoPath(repoURL)
	gitClient, err := s.gitFactory.NewClient(repoURL, appRepoPath, repo.Username, repo.Password, repo.SSHPrivateKey)
	if err != nil {
		return nil, "", err
	}
	commitSHA, err := gitClient.LsRemote(revision)
	if err != nil {
		return nil, "", err
	}
	return gitClient, commitSHA, nil
}
