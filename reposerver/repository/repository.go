package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/argoproj/argo-cd/util/security"

	"github.com/Masterminds/semver"
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
	reposervercache "github.com/argoproj/argo-cd/reposerver/cache"
	"github.com/argoproj/argo-cd/reposerver/metrics"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/app/discovery"
	argopath "github.com/argoproj/argo-cd/util/app/path"
	"github.com/argoproj/argo-cd/util/config"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/helm"
	"github.com/argoproj/argo-cd/util/ksonnet"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/kustomize"
	"github.com/argoproj/argo-cd/util/text"
)

// Service implements ManifestService interface
type Service struct {
	repoLock                  *util.KeyLock
	cache                     *reposervercache.Cache
	parallelismLimitSemaphore *semaphore.Weighted
	metricsServer             *metrics.MetricsServer
	newGitClient              func(rawRepoURL string, creds git.Creds, insecure bool, enableLfs bool) (git.Client, error)
	newHelmClient             func(repoURL string, creds helm.Creds) helm.Client
}

// NewService returns a new instance of the Manifest service
func NewService(metricsServer *metrics.MetricsServer, cache *reposervercache.Cache, parallelismLimit int64) *Service {
	var parallelismLimitSemaphore *semaphore.Weighted
	if parallelismLimit > 0 {
		parallelismLimitSemaphore = semaphore.NewWeighted(parallelismLimit)
	}
	repoLock := util.NewKeyLock()
	return &Service{
		parallelismLimitSemaphore: parallelismLimitSemaphore,
		repoLock:                  repoLock,
		cache:                     cache,
		metricsServer:             metricsServer,
		newGitClient:              git.NewClient,
		newHelmClient: func(repoURL string, creds helm.Creds) helm.Client {
			return helm.NewClientWithLock(repoURL, creds, repoLock)
		},
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

type operationSettings struct {
	sem     *semaphore.Weighted
	noCache bool
}

// runRepoOperation downloads either git folder or helm chart and executes specified operation
func (s *Service) runRepoOperation(
	ctx context.Context,
	revision string,
	repo *v1alpha1.Repository,
	source *v1alpha1.ApplicationSource,
	getCached func(revision string) bool,
	operation func(appPath string, revision string) error,
	settings operationSettings) error {

	var gitClient git.Client
	var helmClient helm.Client
	var err error
	revision = util.FirstNonEmpty(revision, source.TargetRevision)
	if source.IsHelm() {
		helmClient, revision, err = s.newHelmClientResolveRevision(repo, revision, source.Chart)
		if err != nil {
			return err
		}
	} else {
		gitClient, revision, err = s.newClientResolveRevision(repo, revision)
		if err != nil {
			return err
		}
	}

	if !settings.noCache && getCached(revision) {
		return nil
	}

	s.metricsServer.IncPendingRepoRequest(repo.Repo)
	defer s.metricsServer.DecPendingRepoRequest(repo.Repo)

	if settings.sem != nil {
		err = settings.sem.Acquire(ctx, 1)
		if err != nil {
			return err
		}
		defer settings.sem.Release(1)
	}

	if source.IsHelm() {
		version, err := semver.NewVersion(revision)
		if err != nil {
			return err
		}
		if settings.noCache {
			err = helmClient.CleanChartCache(source.Chart, version)
			if err != nil {
				return err
			}
		}
		chartPath, closer, err := helmClient.ExtractChart(source.Chart, version)
		if err != nil {
			return err
		}
		defer util.Close(closer)
		return operation(chartPath, revision)
	} else {
		s.repoLock.Lock(gitClient.Root())
		defer s.repoLock.Unlock(gitClient.Root())
		// double-check locking
		if !settings.noCache && getCached(revision) {
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
}

func (s *Service) GenerateManifest(ctx context.Context, q *apiclient.ManifestRequest) (*apiclient.ManifestResponse, error) {
	res := &apiclient.ManifestResponse{}

	getCached := func(revision string) bool {
		err := s.cache.GetManifests(revision, q.ApplicationSource, q.Namespace, q.AppLabelKey, q.AppLabelValue, &res)
		if err == nil {
			log.Infof("manifest cache hit: %s/%s", q.ApplicationSource.String(), revision)
			return true
		}
		if err != reposervercache.ErrCacheMiss {
			log.Warnf("manifest cache error %s: %v", q.ApplicationSource.String(), err)
		} else {
			log.Infof("manifest cache miss: %s/%s", q.ApplicationSource.String(), revision)
		}
		return false
	}
	err := s.runRepoOperation(ctx, q.Revision, q.Repo, q.ApplicationSource, getCached, func(appPath string, revision string) error {
		var err error
		res, err = GenerateManifests(appPath, revision, q)
		if err != nil {
			return err
		}
		res.Revision = revision
		err = s.cache.SetManifests(revision, q.ApplicationSource, q.Namespace, q.AppLabelKey, q.AppLabelValue, &res)
		if err != nil {
			log.Warnf("manifest cache set error %s/%s: %v", q.ApplicationSource.String(), revision, err)
		}
		return nil
	}, operationSettings{sem: s.parallelismLimitSemaphore, noCache: q.NoCache})
	return res, err
}

func getHelmRepos(repositories []*v1alpha1.Repository) []helm.HelmRepository {
	repos := make([]helm.HelmRepository, 0)
	for _, repo := range repositories {
		repos = append(repos, helm.HelmRepository{Name: repo.Name, Repo: repo.Repo, Creds: repo.GetHelmCreds()})
	}
	return repos
}

func helmTemplate(appPath string, env *v1alpha1.Env, q *apiclient.ManifestRequest) ([]*unstructured.Unstructured, error) {
	templateOpts := &helm.TemplateOpts{
		Name:        q.AppLabelValue,
		Namespace:   q.Namespace,
		KubeVersion: text.SemVer(q.KubeVersion),
		Set:         map[string]string{},
		SetString:   map[string]string{},
	}

	appHelm := q.ApplicationSource.Helm
	if appHelm != nil {
		if appHelm.ReleaseName != "" {
			templateOpts.Name = appHelm.ReleaseName
		}

		for _, val := range appHelm.ValueFiles {
			// If val is not a URL, run it against the directory enforcer. If it is a URL, use it without checking
			if _, err := url.ParseRequestURI(val); err != nil {
				baseDirectoryPath, err := security.SubtractRelativeFromAbsolutePath(appPath, q.ApplicationSource.Path)
				if err != nil {
					return nil, err
				}
				absBaseDir, err := filepath.Abs(baseDirectoryPath)
				if err != nil {
					return nil, err
				}
				if !filepath.IsAbs(val) {
					absWorkDir, err := filepath.Abs(appPath)
					if err != nil {
						return nil, err
					}
					val = filepath.Join(absWorkDir, val)
				}
				_, err = security.EnforceToCurrentRoot(absBaseDir, val)
				if err != nil {
					return nil, err
				}
			}
			templateOpts.Values = append(templateOpts.Values, val)
		}

		if appHelm.Values != "" {
			file, err := ioutil.TempFile("", "values-*.yaml")
			if err != nil {
				return nil, err
			}
			p := file.Name()
			defer func() { _ = os.RemoveAll(p) }()
			err = ioutil.WriteFile(p, []byte(appHelm.Values), 0644)
			if err != nil {
				return nil, err
			}
			templateOpts.Values = append(templateOpts.Values, p)
		}

		for _, p := range appHelm.Parameters {
			if p.ForceString {
				templateOpts.SetString[p.Name] = p.Value
			} else {
				templateOpts.Set[p.Name] = p.Value
			}
		}
	}
	if templateOpts.Name == "" {
		templateOpts.Name = q.AppLabelValue
	}
	for i, j := range templateOpts.Set {
		templateOpts.Set[i] = env.Envsubst(j)
	}
	for i, j := range templateOpts.SetString {
		templateOpts.SetString[i] = env.Envsubst(j)
	}
	h, err := helm.NewHelmApp(appPath, getHelmRepos(q.Repos))
	if err != nil {
		return nil, err
	}
	defer h.Dispose()
	err = h.Init()
	if err != nil {
		return nil, err
	}
	out, err := h.Template(templateOpts)
	if err != nil {
		if !helm.IsMissingDependencyErr(err) {
			return nil, err
		}
		err = h.DependencyBuild()
		if err != nil {
			return nil, err
		}
		out, err = h.Template(templateOpts)
		if err != nil {
			return nil, err
		}
	}
	return kube.SplitYAML(out)
}

// GenerateManifests generates manifests from a path
func GenerateManifests(appPath, revision string, q *apiclient.ManifestRequest) (*apiclient.ManifestResponse, error) {
	var targetObjs []*unstructured.Unstructured
	var dest *v1alpha1.ApplicationDestination

	appSourceType, err := GetAppSourceType(q.ApplicationSource, appPath)
	if err != nil {
		return nil, err
	}
	repoURL := ""
	if q.Repo != nil {
		repoURL = q.Repo.Repo
	}
	env := newEnv(q, revision)

	switch appSourceType {
	case v1alpha1.ApplicationSourceTypeKsonnet:
		targetObjs, dest, err = ksShow(q.AppLabelKey, appPath, q.ApplicationSource.Ksonnet)
	case v1alpha1.ApplicationSourceTypeHelm:
		targetObjs, err = helmTemplate(appPath, env, q)
	case v1alpha1.ApplicationSourceTypeKustomize:
		k := kustomize.NewKustomizeApp(appPath, q.Repo.GetGitCreds(), repoURL)
		targetObjs, _, err = k.Build(q.ApplicationSource.Kustomize, q.KustomizeOptions)
	case v1alpha1.ApplicationSourceTypePlugin:
		targetObjs, err = runConfigManagementPlugin(appPath, env, q, q.Repo.GetGitCreds())
	case v1alpha1.ApplicationSourceTypeDirectory:
		var directory *v1alpha1.ApplicationSourceDirectory
		if directory = q.ApplicationSource.Directory; directory == nil {
			directory = &v1alpha1.ApplicationSourceDirectory{}
		}
		targetObjs, err = findManifests(appPath, env, *directory)
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

func newEnv(q *apiclient.ManifestRequest, revision string) *v1alpha1.Env {
	return &v1alpha1.Env{
		&v1alpha1.EnvEntry{Name: "ARGOCD_APP_NAME", Value: q.AppLabelValue},
		&v1alpha1.EnvEntry{Name: "ARGOCD_APP_NAMESPACE", Value: q.Namespace},
		&v1alpha1.EnvEntry{Name: "ARGOCD_APP_REVISION", Value: revision},
		&v1alpha1.EnvEntry{Name: "ARGOCD_APP_SOURCE_REPO_URL", Value: q.Repo.Repo},
		&v1alpha1.EnvEntry{Name: "ARGOCD_APP_SOURCE_PATH", Value: q.ApplicationSource.Path},
		&v1alpha1.EnvEntry{Name: "ARGOCD_APP_SOURCE_TARGET_REVISION", Value: q.ApplicationSource.TargetRevision},
	}
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
func findManifests(appPath string, env *v1alpha1.Env, directory v1alpha1.ApplicationSourceDirectory) ([]*unstructured.Unstructured, error) {
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
			vm := makeJsonnetVm(directory.Jsonnet, env)
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

func makeJsonnetVm(sourceJsonnet v1alpha1.ApplicationSourceJsonnet, env *v1alpha1.Env) *jsonnet.VM {
	vm := jsonnet.MakeVM()
	for i, j := range sourceJsonnet.TLAs {
		sourceJsonnet.TLAs[i].Value = env.Envsubst(j.Value)
	}
	for i, j := range sourceJsonnet.ExtVars {
		sourceJsonnet.ExtVars[i].Value = env.Envsubst(j.Value)
	}
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

func runConfigManagementPlugin(appPath string, envVars *v1alpha1.Env, q *apiclient.ManifestRequest, creds git.Creds) ([]*unstructured.Unstructured, error) {
	plugin := findPlugin(q.Plugins, q.ApplicationSource.Plugin.Name)
	if plugin == nil {
		return nil, fmt.Errorf("Config management plugin with name '%s' is not supported.", q.ApplicationSource.Plugin.Name)
	}
	env := append(os.Environ(), envVars.Environ()...)
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
	res := &apiclient.RepoAppDetailsResponse{}
	getCached := func(revision string) bool {
		err := s.cache.GetAppDetails(revision, q.Source, &res)
		if err == nil {
			log.Infof("app details cache hit: %s/%s", revision, q.Source.Path)
			return true
		} else {
			if err != reposervercache.ErrCacheMiss {
				log.Warnf("app details cache error %s: %v", revision, q.Source)
			} else {
				log.Infof("app details cache miss: %s/%s", revision, q.Source)
			}
		}
		return false
	}

	err := s.runRepoOperation(ctx, q.Source.TargetRevision, q.Repo, q.Source, getCached, func(appPath string, revision string) error {
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
			h, err := helm.NewHelmApp(appPath, getHelmRepos(q.Repos))
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
			for k, v := range params {
				res.Helm.Parameters = append(res.Helm.Parameters, &v1alpha1.HelmParameter{
					Name:  k,
					Value: v,
				})
			}
		case v1alpha1.ApplicationSourceTypeKustomize:
			res.Kustomize = &apiclient.KustomizeAppSpec{}
			k := kustomize.NewKustomizeApp(appPath, q.Repo.GetGitCreds(), q.Repo.Repo)
			_, images, err := k.Build(nil, q.KustomizeOptions)
			if err != nil {
				return err
			}
			res.Kustomize.Images = images
		}
		_ = s.cache.SetAppDetails(revision, q.Source, res)
		return nil
	}, operationSettings{})

	return res, err
}

func (s *Service) GetRevisionMetadata(ctx context.Context, q *apiclient.RepoServerRevisionMetadataRequest) (*v1alpha1.RevisionMetadata, error) {
	if !git.IsCommitSHA(q.Revision) {
		return nil, fmt.Errorf("revision %s must be resolved", q.Revision)
	}
	metadata, err := s.cache.GetRevisionMetadata(q.Repo.Repo, q.Revision)
	if err == nil {
		log.Infof("revision metadata cache hit: %s/%s", q.Repo.Repo, q.Revision)
		return metadata, nil
	} else {
		if err != reposervercache.ErrCacheMiss {
			log.Warnf("revision metadata cache error %s/%s: %v", q.Repo.Repo, q.Revision, err)
		} else {
			log.Infof("revision metadata cache miss: %s/%s", q.Repo.Repo, q.Revision)
		}
	}
	gitClient, _, err := s.newClientResolveRevision(q.Repo, q.Revision)
	if err != nil {
		return nil, err
	}
	_, err = checkoutRevision(gitClient, q.Revision)
	if err != nil {
		return nil, err
	}
	m, err := gitClient.RevisionMetadata(q.Revision)
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

func (s *Service) newClient(repo *v1alpha1.Repository) (git.Client, error) {
	gitClient, err := s.newGitClient(repo.Repo, repo.GetGitCreds(), repo.IsInsecure(), repo.EnableLFS)
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

func (s *Service) newHelmClientResolveRevision(repo *v1alpha1.Repository, revision string, chart string) (helm.Client, string, error) {
	helmClient := s.newHelmClient(repo.Repo, repo.GetHelmCreds())
	if helm.IsVersion(revision) {
		return helmClient, revision, nil
	}
	constraints, err := semver.NewConstraint(revision)
	if err != nil {
		return nil, "", fmt.Errorf("invalid revision '%s': %v", revision, err)
	}
	index, err := helmClient.GetIndex()
	if err != nil {
		return nil, "", err
	}
	entries, err := index.GetEntries(chart)
	if err != nil {
		return nil, "", err
	}
	version, err := entries.MaxVersion(constraints)
	if err != nil {
		return nil, "", err
	}
	return helmClient, version.String(), nil
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

func (s *Service) GetHelmCharts(ctx context.Context, q *apiclient.HelmChartsRequest) (*apiclient.HelmChartsResponse, error) {
	index, err := s.newHelmClient(q.Repo.Repo, q.Repo.GetHelmCreds()).GetIndex()
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
