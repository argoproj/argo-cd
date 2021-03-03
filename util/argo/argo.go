package argo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/argoproj/argo-cd/common"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/pkg/client/clientset/versioned/typed/application/v1alpha1"
	applicationsv1 "github.com/argoproj/argo-cd/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/helm"
	"github.com/argoproj/argo-cd/util/io"
	"github.com/argoproj/argo-cd/util/settings"
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
				common.AnnotationKeyRefresh: string(refreshType),
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
		if _, ok := annotations[common.AnnotationKeyRefresh]; !ok {
			return app, nil
		}
	}
	return nil, fmt.Errorf("application refresh deadline exceeded")
}

func TestRepoWithKnownType(repo *argoappv1.Repository, isHelm bool, isHelmOci bool) error {
	repo = repo.DeepCopy()
	if isHelm {
		repo.Type = "helm"
	} else {
		repo.Type = "git"
	}
	repo.EnableOCI = repo.EnableOCI || isHelmOci

	return TestRepo(repo)
}

func TestRepo(repo *argoappv1.Repository) error {
	checks := map[string]func() error{
		"git": func() error {
			return git.TestRepo(repo.Repo, repo.GetGitCreds(), repo.IsInsecure(), repo.IsLFSEnabled())
		},
		"helm": func() error {
			if repo.EnableOCI {
				_, err := helm.NewClient(repo.Repo, repo.GetHelmCreds(), repo.EnableOCI).TestHelmOCI()
				return err
			} else {
				_, err := helm.NewClient(repo.Repo, repo.GetHelmCreds(), repo.EnableOCI).GetIndex(false)
				return err
			}
		},
	}
	if check, ok := checks[repo.Type]; ok {
		return check()
	}
	var err error
	for _, check := range checks {
		err = check()
		if err == nil {
			return nil
		}
	}
	return err
}

// ValidateRepo validates the repository specified in application spec. Following is checked:
// * the repository is accessible
// * the path contains valid manifests
// * there are parameters of only one app source type
// * ksonnet: the specified environment exists
func ValidateRepo(
	ctx context.Context,
	app *argoappv1.Application,
	repoClientset apiclient.Clientset,
	db db.ArgoDB,
	kustomizeOptions *argoappv1.KustomizeOptions,
	plugins []*argoappv1.ConfigManagementPlugin,
	kubectl kube.Kubectl,
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
	err = TestRepoWithKnownType(repo, app.Spec.Source.IsHelm(), app.Spec.Source.IsHelmOci())
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

	helmRepos, err := db.ListHelmRepositories(ctx)
	if err != nil {
		return nil, err
	}

	// get the app details, and populate the Ksonnet stuff from it
	appDetails, err := repoClient.GetAppDetails(ctx, &apiclient.RepoServerAppDetailsQuery{
		Repo:             repo,
		Source:           &spec.Source,
		Repos:            helmRepos,
		KustomizeOptions: kustomizeOptions,
	})
	if err != nil {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: fmt.Sprintf("Unable to get app details: %v", err),
		})
		return conditions, nil
	}

	enrichSpec(spec, appDetails)

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
	apiGroups, err := kubectl.GetAPIGroups(config)
	if err != nil {
		return nil, err
	}
	conditions = append(conditions, verifyGenerateManifests(
		ctx, repo, helmRepos, app, repoClient, kustomizeOptions, plugins, cluster.ServerVersion, APIGroupsToVersions(apiGroups))...)

	return conditions, nil
}

func enrichSpec(spec *argoappv1.ApplicationSpec, appDetails *apiclient.RepoAppDetailsResponse) {
	if spec.Source.Ksonnet != nil && appDetails.Ksonnet != nil {
		env, ok := appDetails.Ksonnet.Environments[spec.Source.Ksonnet.Environment]
		if ok {
			// If server and namespace are not supplied, pull it from the app.yaml
			if spec.Destination.Server == "" {
				spec.Destination.Server = env.Destination.Server
			}
			if spec.Destination.Namespace == "" {
				spec.Destination.Namespace = env.Destination.Namespace
			}
		}
	}
}

// ValidateDestination checks:
// if we used destination name we infer the server url
// if we used both name and server then we return an invalid spec error
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

// APIGroupsToVersions converts list of API Groups into versions string list
func APIGroupsToVersions(apiGroups []metav1.APIGroup) []string {
	var apiVersions []string
	for _, g := range apiGroups {
		for _, v := range g.Versions {
			apiVersions = append(apiVersions, v.GroupVersion)
		}
	}
	return apiVersions
}

// GetAppProject returns a project from an application
func GetAppProject(spec *argoappv1.ApplicationSpec, projLister applicationsv1.AppProjectLister, ns string, settingsManager *settings.SettingsManager) (*argoappv1.AppProject, error) {
	projOrig, err := projLister.AppProjects(ns).Get(spec.GetProject())
	if err != nil {
		return nil, err
	}
	return GetAppVirtualProject(projOrig, projLister, settingsManager)
}

// verifyGenerateManifests verifies a repo path can generate manifests
func verifyGenerateManifests(
	ctx context.Context,
	repoRes *argoappv1.Repository,
	helmRepos argoappv1.Repositories,
	app *argoappv1.Application,
	repoClient apiclient.RepoServerServiceClient,
	kustomizeOptions *argoappv1.KustomizeOptions,
	plugins []*argoappv1.ConfigManagementPlugin,
	kubeVersion string,
	apiVersions []string,
) []argoappv1.ApplicationCondition {
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
			Repo: spec.Source.RepoURL,
			Type: repoRes.Type,
			Name: repoRes.Name,
		},
		Repos:             helmRepos,
		Revision:          spec.Source.TargetRevision,
		AppName:           app.Name,
		Namespace:         spec.Destination.Namespace,
		ApplicationSource: &spec.Source,
		Plugins:           plugins,
		KustomizeOptions:  kustomizeOptions,
		KubeVersion:       kubeVersion,
		ApiVersions:       apiVersions,
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
		spec.Project = common.DefaultAppProjectName
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
	if spec.Source.Ksonnet != nil && spec.Source.Ksonnet.IsZero() {
		spec.Source.Ksonnet = nil
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

func getDestinationServer(ctx context.Context, db db.ArgoDB, clusterName string) (string, error) {
	clusterList, err := db.ListClusters(ctx)
	if err != nil {
		return "", err
	}
	var servers []string
	for _, c := range clusterList.Items {
		if c.Name == clusterName {
			servers = append(servers, c.Server)
		}
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
			break
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
