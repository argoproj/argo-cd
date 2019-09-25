package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/TomOnTime/utfutil"
	argoexec "github.com/argoproj/pkg/exec"
	"github.com/ghodss/yaml"
	"github.com/google/go-jsonnet"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/reposerver/metrics"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/app/discovery"
	argopath "github.com/argoproj/argo-cd/util/app/path"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/config"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/helm"
	"github.com/argoproj/argo-cd/util/ksonnet"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/kustomize"
	"github.com/argoproj/argo-cd/util/text"
)

const (
	PluginEnvAppName      = "ARGOCD_APP_NAME"
	PluginEnvAppNamespace = "ARGOCD_APP_NAMESPACE"
)

// Service implements ManifestService interface
type Service struct {
	repoLock                  *util.KeyLock
	cache                     *cache.Cache
	parallelismLimitSemaphore *semaphore.Weighted
	metricsServer             *metrics.MetricsServer
}

// NewService returns a new instance of the Manifest service
func NewService(metricsServer *metrics.MetricsServer, cache *cache.Cache, parallelismLimit int64) *Service {
	var parallelismLimitSemaphore *semaphore.Weighted
	if parallelismLimit > 0 {
		parallelismLimitSemaphore = semaphore.NewWeighted(parallelismLimit)
	}
	return &Service{
		parallelismLimitSemaphore: parallelismLimitSemaphore,
		repoLock:                  util.NewKeyLock(),
		cache:                     cache,
		metricsServer:             metricsServer,
	}
}

// ListDir lists the contents of a GitHub repo
func (s *Service) ListApps(ctx context.Context, q *apiclient.ListAppsRequest) (*apiclient.AppList, error) {
	gitClient, commitSHA, err := s.newClientResolveRevision(q.Repo, q.Revision)
	if err != nil {
		return nil, err
	}
	if apps, err := s.cache.ListApps(q.Repo.Repo, commitSHA); err == nil {
		log.Infof("cache hit: %s/%s", q.Repo.Repo, q.Revision)
		return &apiclient.AppList{Apps: apps}, nil
	}
	s.repoLock.Lock(gitClient.Root())
	defer s.repoLock.Unlock(gitClient.Root())

	commitSHA, err = checkoutRevision(gitClient, commitSHA)
	if err != nil {
		return nil, err
	}
	apps, err := discovery.Discover(gitClient.Root())
	if err != nil {
		return nil, err
	}
	err = s.cache.SetApps(q.Repo.Repo, commitSHA, apps)
	if err != nil {
		log.Warnf("cache set error %s/%s: %v", q.Repo.Repo, commitSHA, err)
	}
	res := apiclient.AppList{Apps: apps}
	return &res, nil
}

// runRepoOperation downloads either git folder or helm chart and executes specified operation
func (s *Service) runRepoOperation(
	c context.Context,
	repo *v1alpha1.Repository,
	source *v1alpha1.ApplicationSource,
	getCached func(revision string) bool,
	operation func(appPath string, revision string) error,
	sem *semaphore.Weighted) error {

	var gitClient git.Client
	var err error
	revision := source.TargetRevision
	if !source.IsHelm() {
		gitClient, revision, err = s.newClientResolveRevision(repo, source.TargetRevision)
		if err != nil {
			return err
		}
	}

	if getCached(revision) {
		return nil
	}

	if sem != nil {
		err = sem.Acquire(c, 1)
		if err != nil {
			return err
		}
		defer sem.Release(1)
	}

	if source.IsHelm() {
		chartPath, closer, err := s.checkoutChart(repo, source.Chart, revision)
		if err != nil {
			return err
		}
		defer util.Close(closer)
		return operation(chartPath, revision)
	}
	s.repoLock.Lock(gitClient.Root())
	defer s.repoLock.Unlock(gitClient.Root())

	if getCached(revision) {
		return nil
	}

	revision, err = checkoutRevision(gitClient, revision)
	if err != nil {
		return err
	}
	appPath, err := argopath.Path(gitClient.Root(), source.Path)
	if err != nil {
		return err
	}
	return operation(appPath, revision)
}

func (s *Service) GenerateManifest(c context.Context, q *apiclient.ManifestRequest) (*apiclient.ManifestResponse, error) {
	var res *apiclient.ManifestResponse

	getCached := func(revision string) bool {
		err := s.cache.GetManifests(revision, q.ApplicationSource, q.Namespace, q.AppLabelKey, q.AppLabelValue, &res)
		if err == nil {
			log.Infof("manifest cache hit: %s/%s", q.ApplicationSource.String(), revision)
			return true
		}
		if err != cache.ErrCacheMiss {
			log.Warnf("manifest cache error %s: %v", q.ApplicationSource.String(), err)
		} else {
			log.Infof("manifest cache miss: %s/%s", q.ApplicationSource.String(), revision)
		}
		return false
	}
	err := s.runRepoOperation(c, q.Repo, q.ApplicationSource, getCached, func(appPath string, revision string) error {
		var err error
		res, err = GenerateManifests(appPath, q)
		if err != nil {
			return err
		}
		res.Revision = revision
		err = s.cache.SetManifests(revision, q.ApplicationSource, q.Namespace, q.AppLabelKey, q.AppLabelValue, &res)
		if err != nil {
			log.Warnf("manifest cache set error %s/%s: %v", q.ApplicationSource.String(), revision, err)
		}
		return nil
	}, s.parallelismLimitSemaphore)
	return res, err
}

// GenerateManifests generates manifests from a path
func GenerateManifests(appPath string, q *apiclient.ManifestRequest) (*apiclient.ManifestResponse, error) {
	var targetObjs []*unstructured.Unstructured
	var dest *v1alpha1.ApplicationDestination

	appSourceType, err := GetAppSourceType(q.ApplicationSource, appPath)
	creds := argo.GetRepoCreds(q.Repo)
	repoURL := ""
	if q.Repo != nil {
		repoURL = q.Repo.Repo
	}
	switch appSourceType {
	case v1alpha1.ApplicationSourceTypeKsonnet:
		targetObjs, dest, err = ksShow(q.AppLabelKey, appPath, q.ApplicationSource.Ksonnet)
	case v1alpha1.ApplicationSourceTypeHelm:
		h, err := helm.NewHelmApp(appPath, q.Repos)
		if err != nil {
			return nil, err
		}
		defer h.Dispose()
		err = h.Init()
		if err != nil {
			return nil, err
		}
		targetObjs, err = h.Template(q.AppLabelValue, q.Namespace, q.KubeVersion, q.ApplicationSource.Helm)
		if err != nil {
			if !helm.IsMissingDependencyErr(err) {
				return nil, err
			}
			err = h.DependencyBuild()
			if err != nil {
				return nil, err
			}
			targetObjs, err = h.Template(q.AppLabelValue, q.Namespace, q.KubeVersion, q.ApplicationSource.Helm)
			if err != nil {
				return nil, err
			}
		}
	case v1alpha1.ApplicationSourceTypeKustomize:
		k := kustomize.NewKustomizeApp(appPath, creds, repoURL)
		targetObjs, _, err = k.Build(q.ApplicationSource.Kustomize, q.KustomizeOptions)
	case v1alpha1.ApplicationSourceTypePlugin:
		targetObjs, err = runConfigManagementPlugin(appPath, q, creds)
	case v1alpha1.ApplicationSourceTypeDirectory:
		var directory *v1alpha1.ApplicationSourceDirectory
		if directory = q.ApplicationSource.Directory; directory == nil {
			directory = &v1alpha1.ApplicationSourceDirectory{}
		}
		targetObjs, err = findManifests(appPath, *directory)
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

	res := apiclient.ManifestResponse{
		Manifests:  manifests,
		SourceType: string(appSourceType),
	}
	if dest != nil {
		res.Namespace = dest.Namespace
		res.Server = dest.Server
	}
	return &res, nil
}

// GetAppSourceType returns explicit application source type or examines a directory and determines its application source type
func GetAppSourceType(source *v1alpha1.ApplicationSource, path string) (v1alpha1.ApplicationSourceType, error) {
	appSourceType, err := source.ExplicitType()
	if err != nil {
		return "", err
	}
	if appSourceType != nil {
		return *appSourceType, nil
	}
	appType, err := discovery.AppType(path)
	if err != nil {
		return "", err
	}
	return v1alpha1.ApplicationSourceType(appType), nil
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

// ksShow runs `ks show` in an app directory after setting any component parameter overrides
func ksShow(appLabelKey, appPath string, ksonnetOpts *v1alpha1.ApplicationSourceKsonnet) ([]*unstructured.Unstructured, *v1alpha1.ApplicationDestination, error) {
	ksApp, err := ksonnet.NewKsonnetApp(appPath)
	if err != nil {
		return nil, nil, status.Errorf(codes.FailedPrecondition, "unable to load application from %s: %v", appPath, err)
	}
	if ksonnetOpts == nil {
		return nil, nil, status.Errorf(codes.InvalidArgument, "Ksonnet environment not set")
	}
	for _, override := range ksonnetOpts.Parameters {
		err = ksApp.SetComponentParams(ksonnetOpts.Environment, override.Component, override.Name, override.Value)
		if err != nil {
			return nil, nil, err
		}
	}
	dest, err := ksApp.Destination(ksonnetOpts.Environment)
	if err != nil {
		return nil, nil, status.Errorf(codes.InvalidArgument, err.Error())
	}
	targetObjs, err := ksApp.Show(ksonnetOpts.Environment)
	if err == nil && appLabelKey == common.LabelKeyLegacyApplicationName {
		// Address https://github.com/ksonnet/ksonnet/issues/707
		for _, d := range targetObjs {
			kube.UnsetLabel(d, "ksonnet.io/component")
		}
	}
	if err != nil {
		return nil, nil, err
	}
	return targetObjs, dest, nil
}

var manifestFile = regexp.MustCompile(`^.*\.(yaml|yml|json|jsonnet)$`)

// findManifests looks at all yaml files in a directory and unmarshals them into a list of unstructured objects
func findManifests(appPath string, directory v1alpha1.ApplicationSourceDirectory) ([]*unstructured.Unstructured, error) {
	var objs []*unstructured.Unstructured
	err := filepath.Walk(appPath, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if f.IsDir() {
			if path != appPath && !directory.Recurse {
				return filepath.SkipDir
			} else {
				return nil
			}
		}

		if !manifestFile.MatchString(f.Name()) {
			return nil
		}
		out, err := utfutil.ReadFile(path, utfutil.UTF8)
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
			vm := makeJsonnetVm(directory.Jsonnet)
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

func makeJsonnetVm(sourceJsonnet v1alpha1.ApplicationSourceJsonnet) *jsonnet.VM {
	vm := jsonnet.MakeVM()

	for _, arg := range sourceJsonnet.TLAs {
		if arg.Code {
			vm.TLACode(arg.Name, arg.Value)
		} else {
			vm.TLAVar(arg.Name, arg.Value)
		}
	}
	for _, extVar := range sourceJsonnet.ExtVars {
		if extVar.Code {
			vm.ExtCode(extVar.Name, extVar.Value)
		} else {
			vm.ExtVar(extVar.Name, extVar.Value)
		}
	}

	return vm
}

func runCommand(command v1alpha1.Command, path string, env []string) (string, error) {
	if len(command.Command) == 0 {
		return "", fmt.Errorf("Command is empty")
	}
	cmd := exec.Command(command.Command[0], append(command.Command[1:], command.Args...)...)
	cmd.Env = env
	cmd.Dir = path
	return argoexec.RunCommandExt(cmd, config.CmdOpts())
}

func findPlugin(plugins []*v1alpha1.ConfigManagementPlugin, name string) *v1alpha1.ConfigManagementPlugin {
	for _, plugin := range plugins {
		if plugin.Name == name {
			return plugin
		}
	}
	return nil
}

func runConfigManagementPlugin(appPath string, q *apiclient.ManifestRequest, creds git.Creds) ([]*unstructured.Unstructured, error) {
	plugin := findPlugin(q.Plugins, q.ApplicationSource.Plugin.Name)
	if plugin == nil {
		return nil, fmt.Errorf("Config management plugin with name '%s' is not supported.", q.ApplicationSource.Plugin.Name)
	}
	env := append(os.Environ(), fmt.Sprintf("%s=%s", PluginEnvAppName, q.AppLabelValue), fmt.Sprintf("%s=%s", PluginEnvAppNamespace, q.Namespace))
	if creds != nil {
		closer, environ, err := creds.Environ()
		if err != nil {
			return nil, err
		}
		defer func() { _ = closer.Close() }()
		env = append(env, environ...)
	}
	env = append(env, q.ApplicationSource.Plugin.Env.Environ()...)
	if plugin.Init != nil {
		_, err := runCommand(*plugin.Init, appPath, env)
		if err != nil {
			return nil, err
		}
	}
	out, err := runCommand(plugin.Generate, appPath, env)
	if err != nil {
		return nil, err
	}
	return kube.SplitYAML(out)
}

func (s *Service) GetAppDetails(ctx context.Context, q *apiclient.RepoServerAppDetailsQuery) (*apiclient.RepoAppDetailsResponse, error) {
	var res apiclient.RepoAppDetailsResponse
	getCached := func(revision string) bool {
		err := s.cache.GetAppDetails(revision, q.Source, &res)
		if err == nil {
			log.Infof("manifest cache hit: %s/%s", revision, q.Source.Path)
			return true
		} else {
			if err != cache.ErrCacheMiss {
				log.Warnf("manifest cache error %s: %v", revision, q.Source)
			} else {
				log.Infof("manifest cache miss: %s/%s", revision, q.Source)
			}
		}
		return false
	}

	err := s.runRepoOperation(ctx, q.Repo, q.Source, getCached, func(appPath string, revision string) error {
		appSourceType, err := GetAppSourceType(q.Source, appPath)
		if err != nil {
			return err
		}

		res.Type = string(appSourceType)

		switch appSourceType {
		case v1alpha1.ApplicationSourceTypeKsonnet:
			var ksonnetAppSpec apiclient.KsonnetAppSpec
			data, err := ioutil.ReadFile(filepath.Join(appPath, "app.yaml"))
			if err != nil {
				return err
			}
			err = yaml.Unmarshal(data, &ksonnetAppSpec)
			if err != nil {
				return err
			}
			ksApp, err := ksonnet.NewKsonnetApp(appPath)
			if err != nil {
				return status.Errorf(codes.FailedPrecondition, "unable to load application from %s: %v", appPath, err)
			}
			env := ""
			if q.Source.Ksonnet != nil {
				env = q.Source.Ksonnet.Environment
			}
			params, err := ksApp.ListParams(env)
			if err != nil {
				return err
			}
			ksonnetAppSpec.Parameters = params
			res.Ksonnet = &ksonnetAppSpec
		case v1alpha1.ApplicationSourceTypeHelm:
			res.Helm = &apiclient.HelmAppSpec{}
			files, err := ioutil.ReadDir(appPath)
			if err != nil {
				return err
			}
			for _, f := range files {
				if f.IsDir() {
					continue
				}
				fName := f.Name()
				if strings.Contains(fName, "values") && (filepath.Ext(fName) == ".yaml" || filepath.Ext(fName) == ".yml") {
					res.Helm.ValueFiles = append(res.Helm.ValueFiles, fName)
				}
			}
			h, err := helm.NewHelmApp(appPath, q.Repos)
			if err != nil {
				return err
			}
			defer h.Dispose()
			err = h.Init()
			if err != nil {
				return err
			}
			valuesPath := filepath.Join(appPath, "values.yaml")
			info, err := os.Stat(valuesPath)
			if err == nil && !info.IsDir() {
				bytes, err := ioutil.ReadFile(valuesPath)
				if err != nil {
					return err
				}
				res.Helm.Values = string(bytes)
			}
			params, err := h.GetParameters(valueFiles(q))
			if err != nil {
				return err
			}
			res.Helm.Parameters = params
		case v1alpha1.ApplicationSourceTypeKustomize:
			res.Kustomize = &apiclient.KustomizeAppSpec{}
			k := kustomize.NewKustomizeApp(appPath, argo.GetRepoCreds(q.Repo), q.Repo.Repo)
			_, images, err := k.Build(nil, q.KustomizeOptions)
			if err != nil {
				return err
			}
			res.Kustomize.Images = images
		}
		_ = s.cache.SetAppDetails(revision, q.Source, res)
		return nil
	}, nil)

	return &res, err
}

func (s *Service) GetRevisionMetadata(ctx context.Context, q *apiclient.RepoServerRevisionMetadataRequest) (*v1alpha1.RevisionMetadata, error) {
	gitClient, commitSHA, err := s.newClientResolveRevision(q.Repo, q.Revision)
	if err != nil {
		return nil, err
	}

	metadata, err := s.cache.GetRevisionMetadata(q.Repo.Repo, commitSHA)
	if err == nil {
		log.Infof("manifest cache hit: %s/%s", q.Repo.Repo, commitSHA)
		return metadata, nil
	} else {
		if err != cache.ErrCacheMiss {
			log.Warnf("manifest cache error %s/%s: %v", q.Repo.Repo, commitSHA, err)
		} else {
			log.Infof("manifest cache miss: %s/%s", q.Repo.Repo, commitSHA)
		}
	}

	commitSHA, err = checkoutRevision(gitClient, commitSHA)
	if err != nil {
		return nil, err
	}
	m, err := gitClient.RevisionMetadata(commitSHA)
	if err != nil {
		return nil, err
	}
	// discard anything after the first new line and then truncate to 64 chars
	message := text.Trunc(strings.SplitN(m.Message, "\n", 2)[0], 64)
	metadata = &v1alpha1.RevisionMetadata{Author: m.Author, Date: metav1.Time{Time: m.Date}, Tags: m.Tags, Message: message}
	_ = s.cache.SetRevisionMetadata(q.Repo.Repo, q.Revision, metadata)
	return metadata, nil
}

func valueFiles(q *apiclient.RepoServerAppDetailsQuery) []string {
	if q.Source.Helm == nil {
		return nil
	}
	return q.Source.Helm.ValueFiles
}

// tempRepoPath returns a formulated temporary directory location to clone a repository
func tempRepoPath(repo string) string {
	return filepath.Join(os.TempDir(), strings.Replace(repo, "/", "_", -1))
}

func (s *Service) newClient(repo *v1alpha1.Repository) (git.Client, error) {
	appPath := tempRepoPath(git.NormalizeGitURL(repo.Repo))
	gitClient, err := git.NewClient(repo.Repo, appPath, argo.GetRepoCreds(repo), repo.IsInsecure(), repo.EnableLFS)
	if err != nil {
		return nil, err
	}
	return metrics.WrapGitClient(repo.Repo, s.metricsServer, gitClient), nil
}

// newClientResolveRevision is a helper to perform the common task of instantiating a git client
// and resolving a revision to a commit SHA
func (s *Service) newClientResolveRevision(repo *v1alpha1.Repository, revision string) (git.Client, string, error) {
	gitClient, err := s.newClient(repo)
	if err != nil {
		return nil, "", err
	}
	commitSHA, err := gitClient.LsRemote(revision)
	if err != nil {
		return nil, "", err
	}
	return gitClient, commitSHA, nil
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
