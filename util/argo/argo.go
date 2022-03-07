package argo

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/r3labs/diff"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/typed/application/v1alpha1"
	applicationsv1 "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

const (
	errDestinationMissing = "Destination server missing from app spec"
)

// FormatAppConditions returns string representation of give app condition list
func FormatAppConditions(conditions []argoappv1.ApplicationCondition) string {
	formattedConditions := make([]string, 0)
	for _, condition := range conditions {
		formattedConditions = append(formattedConditions, fmt.Sprintf("%s: %s", condition.Type, condition.Message))
	}
	return strings.Join(formattedConditions, ";")
}

// FilterByProjects returns applications which belongs to the specified project
func FilterByProjects(apps []argoappv1.Application, projects []string) []argoappv1.Application {
	if len(projects) == 0 {
		return apps
	}
	projectsMap := make(map[string]bool)
	for i := range projects {
		projectsMap[projects[i]] = true
	}
	items := make([]argoappv1.Application, 0)
	for i := 0; i < len(apps); i++ {
		a := apps[i]
		if _, ok := projectsMap[a.Spec.GetProject()]; ok {
			items = append(items, a)
		}
	}
	return items

}

// FilterByRepo returns an application
func FilterByRepo(apps []argoappv1.Application, repo string) []argoappv1.Application {
	if repo == "" {
		return apps
	}
	items := make([]argoappv1.Application, 0)
	for i := 0; i < len(apps); i++ {
		if apps[i].Spec.Source.RepoURL == repo {
			items = append(items, apps[i])
		}
	}
	return items
}

// FilterByName returns an application
func FilterByName(apps []argoappv1.Application, name string) ([]argoappv1.Application, error) {
	if name == "" {
		return apps, nil
	}
	items := make([]argoappv1.Application, 0)
	for i := 0; i < len(apps); i++ {
		if apps[i].Name == name {
			items = append(items, apps[i])
			return items, nil
		}
	}
	return items, status.Errorf(codes.NotFound, "application '%s' not found", name)
}

// RefreshApp updates the refresh annotation of an application to coerce the controller to process it
func RefreshApp(appIf v1alpha1.ApplicationInterface, name string, refreshType argoappv1.RefreshType) (*argoappv1.Application, error) {
	metadata := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				argoappv1.AnnotationKeyRefresh: string(refreshType),
			},
		},
	}
	var err error
	patch, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 5; attempt++ {
		app, err := appIf.Patch(context.Background(), name, types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			if !apierr.IsConflict(err) {
				return nil, err
			}
		} else {
			log.Infof("Requested app '%s' refresh", name)
			return app, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, err
}

// WaitForRefresh watches an application until its comparison timestamp is after the refresh timestamp
// If refresh timestamp is not present, will use current timestamp at time of call
func WaitForRefresh(ctx context.Context, appIf v1alpha1.ApplicationInterface, name string, timeout *time.Duration) (*argoappv1.Application, error) {
	var cancel context.CancelFunc
	if timeout != nil {
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}
	ch := kube.WatchWithRetry(ctx, func() (i watch.Interface, e error) {
		fieldSelector := fields.ParseSelectorOrDie(fmt.Sprintf("metadata.name=%s", name))
		listOpts := metav1.ListOptions{FieldSelector: fieldSelector.String()}
		return appIf.Watch(ctx, listOpts)
	})
	for next := range ch {
		if next.Error != nil {
			return nil, next.Error
		}
		app, ok := next.Object.(*argoappv1.Application)
		if !ok {
			return nil, fmt.Errorf("Application event object failed conversion: %v", next)
		}
		annotations := app.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		if _, ok := annotations[argoappv1.AnnotationKeyRefresh]; !ok {
			return app, nil
		}
	}
	return nil, fmt.Errorf("application refresh deadline exceeded")
}

func TestRepoWithKnownType(ctx context.Context, repoClient apiclient.RepoServerServiceClient, repo *argoappv1.Repository, isHelm bool, isHelmOci bool) error {
	repo = repo.DeepCopy()
	if isHelm {
		repo.Type = "helm"
	} else {
		repo.Type = "git"
	}
	repo.EnableOCI = repo.EnableOCI || isHelmOci

	_, err := repoClient.TestRepository(ctx, &apiclient.TestRepositoryRequest{
		Repo: repo,
	})

	return err
}

// ValidateRepo validates the repository specified in application spec. Following is checked:
// * the repository is accessible
// * the path contains valid manifests
// * there are parameters of only one app source type
func ValidateRepo(
	ctx context.Context,
	app *argoappv1.Application,
	repoClientset apiclient.Clientset,
	db db.ArgoDB,
	kustomizeOptions *argoappv1.KustomizeOptions,
	plugins []*argoappv1.ConfigManagementPlugin,
	kubectl kube.Kubectl,
	proj *argoappv1.AppProject,
	settingsMgr *settings.SettingsManager,
) ([]argoappv1.ApplicationCondition, error) {
	spec := &app.Spec
	conditions := make([]argoappv1.ApplicationCondition, 0)

	// Test the repo
	conn, repoClient, err := repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, err
	}
	defer io.Close(conn)
	repo, err := db.GetRepository(ctx, spec.Source.RepoURL)
	if err != nil {
		return nil, err
	}

	repoAccessible := false

	err = TestRepoWithKnownType(ctx, repoClient, repo, app.Spec.Source.IsHelm(), app.Spec.Source.IsHelmOci())
	if err != nil {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: fmt.Sprintf("repository not accessible: %v", err),
		})
	} else {
		repoAccessible = true
	}

	// Verify only one source type is defined
	_, err = spec.Source.ExplicitType()
	if err != nil {
		return nil, err
	}

	// is the repo inaccessible - abort now
	if !repoAccessible {
		return conditions, nil
	}

	helmOptions, err := settingsMgr.GetHelmSettings()
	if err != nil {
		return nil, err
	}

	helmRepos, err := db.ListHelmRepositories(ctx)
	if err != nil {
		return nil, err
	}
	permittedHelmRepos, err := GetPermittedRepos(proj, helmRepos)
	if err != nil {
		return nil, err
	}
	helmRepositoryCredentials, err := db.GetAllHelmRepositoryCredentials(ctx)
	if err != nil {
		return nil, err
	}
	permittedHelmCredentials, err := GetPermittedReposCredentials(proj, helmRepositoryCredentials)
	if err != nil {
		return nil, err
	}

	cluster, err := db.GetCluster(context.Background(), spec.Destination.Server)
	if err != nil {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: fmt.Sprintf("Unable to get cluster: %v", err),
		})
		return conditions, nil
	}
	config := cluster.RESTConfig()
	cluster.ServerVersion, err = kubectl.GetServerVersion(config)
	if err != nil {
		return nil, err
	}
	apiGroups, err := kubectl.GetAPIResources(config, false, cache.NewNoopSettings())
	if err != nil {
		return nil, err
	}
	enabledSourceTypes, err := settingsMgr.GetEnabledSourceTypes()
	if err != nil {
		return nil, err
	}
	conditions = append(conditions, verifyGenerateManifests(
		ctx,
		repo,
		permittedHelmRepos,
		helmOptions,
		app,
		repoClient,
		kustomizeOptions,
		plugins,
		cluster.ServerVersion,
		APIResourcesToStrings(apiGroups, true),
		permittedHelmCredentials,
		enabledSourceTypes,
		settingsMgr)...)

	return conditions, nil
}

// ValidateDestination sets the 'Server' value of the ApplicationDestination, if it is not set.
// NOTE: this function WILL write to the object pointed to by the 'dest' parameter.
//
// If an ApplicationDestination has a Name field, but has an empty Server (URL) field,
// ValidationDestination will look up the cluster by name (to get the server URL), and
// set the corresponding Server field value.
//
// It also checks:
// - If we used both name and server then we return an invalid spec error
func ValidateDestination(ctx context.Context, dest *argoappv1.ApplicationDestination, db db.ArgoDB) error {
	if dest.Name != "" {
		if dest.Server == "" {
			server, err := getDestinationServer(ctx, db, dest.Name)
			if err != nil {
				return fmt.Errorf("unable to find destination server: %v", err)
			}
			if server == "" {
				return fmt.Errorf("application references destination cluster %s which does not exist", dest.Name)
			}
			dest.SetInferredServer(server)
		} else {
			if !dest.IsServerInferred() {
				return fmt.Errorf("application destination can't have both name and server defined: %s %s", dest.Name, dest.Server)
			}
		}
	}
	return nil
}

// ValidatePermissions ensures that the referenced cluster has been added to Argo CD and the app source repo and destination namespace/cluster are permitted in app project
func ValidatePermissions(ctx context.Context, spec *argoappv1.ApplicationSpec, proj *argoappv1.AppProject, db db.ArgoDB) ([]argoappv1.ApplicationCondition, error) {
	conditions := make([]argoappv1.ApplicationCondition, 0)
	if spec.Source.RepoURL == "" || (spec.Source.Path == "" && spec.Source.Chart == "") {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: "spec.source.repoURL and spec.source.path either spec.source.chart are required",
		})
		return conditions, nil
	}
	if spec.Source.Chart != "" && spec.Source.TargetRevision == "" {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: "spec.source.targetRevision is required if the manifest source is a helm chart",
		})
		return conditions, nil
	}

	if !proj.IsSourcePermitted(spec.Source) {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: fmt.Sprintf("application repo %s is not permitted in project '%s'", spec.Source.RepoURL, spec.Project),
		})
	}

	// ValidateDestination will resolve the destination's server address from its name for us, if possible
	if err := ValidateDestination(ctx, &spec.Destination, db); err != nil {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: err.Error(),
		})
		return conditions, nil
	}

	if spec.Destination.Server != "" {
		if !proj.IsDestinationPermitted(spec.Destination) {
			conditions = append(conditions, argoappv1.ApplicationCondition{
				Type:    argoappv1.ApplicationConditionInvalidSpecError,
				Message: fmt.Sprintf("application destination {%s %s} is not permitted in project '%s'", spec.Destination.Server, spec.Destination.Namespace, spec.Project),
			})
		}
		// Ensure the k8s cluster the app is referencing, is configured in Argo CD
		_, err := db.GetCluster(ctx, spec.Destination.Server)
		if err != nil {
			if errStatus, ok := status.FromError(err); ok && errStatus.Code() == codes.NotFound {
				conditions = append(conditions, argoappv1.ApplicationCondition{
					Type:    argoappv1.ApplicationConditionInvalidSpecError,
					Message: fmt.Sprintf("cluster '%s' has not been configured", spec.Destination.Server),
				})
			} else {
				return nil, err
			}
		}
	} else if spec.Destination.Server == "" {
		conditions = append(conditions, argoappv1.ApplicationCondition{Type: argoappv1.ApplicationConditionInvalidSpecError, Message: errDestinationMissing})
	}
	return conditions, nil
}

// APIResourcesToStrings converts list of API Resources list into string list
func APIResourcesToStrings(resources []kube.APIResourceInfo, includeKinds bool) []string {
	resMap := map[string]bool{}
	for _, r := range resources {
		groupVersion := r.GroupVersionResource.GroupVersion().String()
		resMap[groupVersion] = true
		if includeKinds {
			resMap[groupVersion+"/"+r.GroupKind.Kind] = true
		}

	}
	var res []string
	for k := range resMap {
		res = append(res, k)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i] < res[j]
	})
	return res
}

// GetAppProjectWithScopedResources returns a project from an application with scoped resources
func GetAppProjectWithScopedResources(name string, projLister applicationsv1.AppProjectLister, ns string, settingsManager *settings.SettingsManager, db db.ArgoDB, ctx context.Context) (*argoappv1.AppProject, argoappv1.Repositories, []*argoappv1.Cluster, error) {
	projOrig, err := projLister.AppProjects(ns).Get(name)
	if err != nil {
		return nil, nil, nil, err
	}

	project, err := GetAppVirtualProject(projOrig, projLister, settingsManager)

	if err != nil {
		return nil, nil, nil, err
	}

	clusters, err := db.GetProjectClusters(ctx, project.Name)
	if err != nil {
		return nil, nil, nil, err
	}
	repos, err := db.GetProjectRepositories(ctx, name)
	if err != nil {
		return nil, nil, nil, err
	}
	return project, repos, clusters, nil

}

// GetAppProjectByName returns a project from an application based on name
func GetAppProjectByName(name string, projLister applicationsv1.AppProjectLister, ns string, settingsManager *settings.SettingsManager, db db.ArgoDB, ctx context.Context) (*argoappv1.AppProject, error) {
	projOrig, err := projLister.AppProjects(ns).Get(name)
	if err != nil {
		return nil, err
	}
	project := projOrig.DeepCopy()
	repos, err := db.GetProjectRepositories(ctx, name)
	if err != nil {
		return nil, err
	}
	for _, repo := range repos {
		project.Spec.SourceRepos = append(project.Spec.SourceRepos, repo.Repo)
	}
	clusters, err := db.GetProjectClusters(ctx, name)
	if err != nil {
		return nil, err
	}
	for _, cluster := range clusters {
		if len(cluster.Namespaces) == 0 {
			project.Spec.Destinations = append(project.Spec.Destinations, argoappv1.ApplicationDestination{Server: cluster.Server, Namespace: "*"})
		} else {
			for _, ns := range cluster.Namespaces {
				project.Spec.Destinations = append(project.Spec.Destinations, argoappv1.ApplicationDestination{Server: cluster.Server, Namespace: ns})
			}
		}
	}
	return GetAppVirtualProject(project, projLister, settingsManager)
}

// GetAppProject returns a project from an application
func GetAppProject(spec *argoappv1.ApplicationSpec, projLister applicationsv1.AppProjectLister, ns string, settingsManager *settings.SettingsManager, db db.ArgoDB, ctx context.Context) (*argoappv1.AppProject, error) {
	return GetAppProjectByName(spec.GetProject(), projLister, ns, settingsManager, db, ctx)
}

// verifyGenerateManifests verifies a repo path can generate manifests
func verifyGenerateManifests(ctx context.Context, repoRes *argoappv1.Repository, helmRepos argoappv1.Repositories, helmOptions *argoappv1.HelmOptions, app *argoappv1.Application, repoClient apiclient.RepoServerServiceClient, kustomizeOptions *argoappv1.KustomizeOptions, plugins []*argoappv1.ConfigManagementPlugin, kubeVersion string, apiVersions []string, repositoryCredentials []*argoappv1.RepoCreds, enableGenerateManifests map[string]bool, settingsMgr *settings.SettingsManager) []argoappv1.ApplicationCondition {
	spec := &app.Spec
	var conditions []argoappv1.ApplicationCondition
	if spec.Destination.Server == "" {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: errDestinationMissing,
		})
	}

	req := apiclient.ManifestRequest{
		Repo: &argoappv1.Repository{
			Repo:  spec.Source.RepoURL,
			Type:  repoRes.Type,
			Name:  repoRes.Name,
			Proxy: repoRes.Proxy,
		},
		Repos:              helmRepos,
		Revision:           spec.Source.TargetRevision,
		AppName:            app.Name,
		Namespace:          spec.Destination.Namespace,
		ApplicationSource:  &spec.Source,
		Plugins:            plugins,
		KustomizeOptions:   kustomizeOptions,
		KubeVersion:        kubeVersion,
		ApiVersions:        apiVersions,
		HelmOptions:        helmOptions,
		HelmRepoCreds:      repositoryCredentials,
		TrackingMethod:     string(GetTrackingMethod(settingsMgr)),
		EnabledSourceTypes: enableGenerateManifests,
		NoRevisionCache:    true,
	}
	req.Repo.CopyCredentialsFromRepo(repoRes)
	req.Repo.CopySettingsFrom(repoRes)

	// Only check whether we can access the application's path,
	// and not whether it actually contains any manifests.
	_, err := repoClient.GenerateManifest(ctx, &req)
	if err != nil {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: fmt.Sprintf("Unable to generate manifests in %s: %v", spec.Source.Path, err),
		})
	}

	return conditions
}

// SetAppOperation updates an application with the specified operation, retrying conflict errors
func SetAppOperation(appIf v1alpha1.ApplicationInterface, appName string, op *argoappv1.Operation) (*argoappv1.Application, error) {
	for {
		a, err := appIf.Get(context.Background(), appName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if a.Operation != nil {
			return nil, status.Errorf(codes.FailedPrecondition, "another operation is already in progress")
		}
		a.Operation = op
		a.Status.OperationState = nil
		a, err = appIf.Update(context.Background(), a, metav1.UpdateOptions{})
		if op.Sync == nil {
			return nil, status.Errorf(codes.InvalidArgument, "Operation unspecified")
		}
		if err == nil {
			return a, nil
		}
		if !apierr.IsConflict(err) {
			return nil, err
		}
		log.Warnf("Failed to set operation for app '%s' due to update conflict. Retrying again...", appName)
	}
}

// ContainsSyncResource determines if the given resource exists in the provided slice of sync operation resources.
func ContainsSyncResource(name string, namespace string, gvk schema.GroupVersionKind, rr []argoappv1.SyncOperationResource) bool {
	for _, r := range rr {
		if r.HasIdentity(name, namespace, gvk) {
			return true
		}
	}
	return false
}

// NormalizeApplicationSpec will normalize an application spec to a preferred state. This is used
// for migrating application objects which are using deprecated legacy fields into the new fields,
// and defaulting fields in the spec (e.g. spec.project)
func NormalizeApplicationSpec(spec *argoappv1.ApplicationSpec) *argoappv1.ApplicationSpec {
	spec = spec.DeepCopy()
	if spec.Project == "" {
		spec.Project = argoappv1.DefaultAppProjectName
	}

	// 3. If any app sources are their zero values, then nil out the pointers to the source spec.
	// This makes it easier for users to switch between app source types if they are not using
	// any of the source-specific parameters.
	if spec.Source.Kustomize != nil && spec.Source.Kustomize.IsZero() {
		spec.Source.Kustomize = nil
	}
	if spec.Source.Helm != nil && spec.Source.Helm.IsZero() {
		spec.Source.Helm = nil
	}
	if spec.Source.Directory != nil && spec.Source.Directory.IsZero() {
		if spec.Source.Directory.Exclude != "" && spec.Source.Directory.Include != "" {
			spec.Source.Directory = &argoappv1.ApplicationSourceDirectory{Exclude: spec.Source.Directory.Exclude, Include: spec.Source.Directory.Include}
		} else if spec.Source.Directory.Exclude != "" {
			spec.Source.Directory = &argoappv1.ApplicationSourceDirectory{Exclude: spec.Source.Directory.Exclude}
		} else if spec.Source.Directory.Include != "" {
			spec.Source.Directory = &argoappv1.ApplicationSourceDirectory{Include: spec.Source.Directory.Include}
		} else {
			spec.Source.Directory = nil
		}
	}
	return spec
}

func GetPermittedReposCredentials(proj *argoappv1.AppProject, repoCreds []*argoappv1.RepoCreds) ([]*argoappv1.RepoCreds, error) {
	var permittedRepoCreds []*argoappv1.RepoCreds
	for _, v := range repoCreds {
		if proj.IsSourcePermitted(argoappv1.ApplicationSource{RepoURL: v.URL}) {
			permittedRepoCreds = append(permittedRepoCreds, v)
		}
	}
	return permittedRepoCreds, nil
}

func GetPermittedRepos(proj *argoappv1.AppProject, repos []*argoappv1.Repository) ([]*argoappv1.Repository, error) {
	var permittedRepos []*argoappv1.Repository
	for _, v := range repos {
		if proj.IsSourcePermitted(argoappv1.ApplicationSource{RepoURL: v.Repo}) {
			permittedRepos = append(permittedRepos, v)
		}
	}
	return permittedRepos, nil
}

func getDestinationServer(ctx context.Context, db db.ArgoDB, clusterName string) (string, error) {
	servers, err := db.GetClusterServersByName(ctx, clusterName)
	if err != nil {
		return "", err
	}
	if len(servers) > 1 {
		return "", fmt.Errorf("there are %d clusters with the same name: %v", len(servers), servers)
	} else if len(servers) == 0 {
		return "", fmt.Errorf("there are no clusters with this name: %s", clusterName)
	}
	return servers[0], nil
}

func GetGlobalProjects(proj *argoappv1.AppProject, projLister applicationsv1.AppProjectLister, settingsManager *settings.SettingsManager) []*argoappv1.AppProject {
	gps, err := settingsManager.GetGlobalProjectsSettings()
	globalProjects := make([]*argoappv1.AppProject, 0)

	if err != nil {
		log.Warnf("Failed to get global project settings: %v", err)
		return globalProjects
	}

	for _, gp := range gps {
		//The project itself is not its own the global project
		if proj.Name == gp.ProjectName {
			continue
		}

		selector, err := metav1.LabelSelectorAsSelector(&gp.LabelSelector)
		if err != nil {
			break
		}
		//Get projects which match the label selector, then see if proj is a match
		projList, err := projLister.AppProjects(proj.Namespace).List(selector)
		if err != nil {
			break
		}
		var matchMe bool
		for _, item := range projList {
			if item.Name == proj.Name {
				matchMe = true
				break
			}
		}
		if !matchMe {
			continue
		}
		//If proj is a match for this global project setting, then it is its global project
		globalProj, err := projLister.AppProjects(proj.Namespace).Get(gp.ProjectName)
		if err != nil {
			break
		}
		globalProjects = append(globalProjects, globalProj)

	}
	return globalProjects
}

func GetAppVirtualProject(proj *argoappv1.AppProject, projLister applicationsv1.AppProjectLister, settingsManager *settings.SettingsManager) (*argoappv1.AppProject, error) {
	virtualProj := proj.DeepCopy()
	globalProjects := GetGlobalProjects(proj, projLister, settingsManager)

	for _, gp := range globalProjects {
		virtualProj = mergeVirtualProject(virtualProj, gp)
	}
	return virtualProj, nil
}

func mergeVirtualProject(proj *argoappv1.AppProject, globalProj *argoappv1.AppProject) *argoappv1.AppProject {
	if globalProj == nil {
		return proj
	}
	proj.Spec.ClusterResourceWhitelist = append(proj.Spec.ClusterResourceWhitelist, globalProj.Spec.ClusterResourceWhitelist...)
	proj.Spec.ClusterResourceBlacklist = append(proj.Spec.ClusterResourceBlacklist, globalProj.Spec.ClusterResourceBlacklist...)

	proj.Spec.NamespaceResourceWhitelist = append(proj.Spec.NamespaceResourceWhitelist, globalProj.Spec.NamespaceResourceWhitelist...)
	proj.Spec.NamespaceResourceBlacklist = append(proj.Spec.NamespaceResourceBlacklist, globalProj.Spec.NamespaceResourceBlacklist...)

	proj.Spec.SyncWindows = append(proj.Spec.SyncWindows, globalProj.Spec.SyncWindows...)

	proj.Spec.SourceRepos = append(proj.Spec.SourceRepos, globalProj.Spec.SourceRepos...)

	proj.Spec.Destinations = append(proj.Spec.Destinations, globalProj.Spec.Destinations...)

	return proj
}

func GenerateSpecIsDifferentErrorMessage(entity string, a, b interface{}) string {
	basicMsg := fmt.Sprintf("existing %s spec is different; use upsert flag to force update", entity)
	difference, _ := GetDifferentPathsBetweenStructs(a, b)
	if len(difference) == 0 {
		return basicMsg
	}
	return fmt.Sprintf("%s; difference in keys \"%s\"", basicMsg, strings.Join(difference[:], ","))
}

func GetDifferentPathsBetweenStructs(a, b interface{}) ([]string, error) {
	var difference []string
	changelog, err := diff.Diff(a, b)
	if err != nil {
		return nil, err
	}
	for _, changeItem := range changelog {
		difference = append(difference, changeItem.Path...)
	}
	return difference, nil
}
