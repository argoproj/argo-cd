package argo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/r3labs/diff/v3"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/argoproj/argo-cd/v3/util/gpg"

	argoappv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/typed/application/v1alpha1"
	applicationsv1 "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/glob"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

const (
	ErrDestinationMissing = "Destination server missing from app spec"
)

var ErrAnotherOperationInProgress = status.Errorf(codes.FailedPrecondition, "another operation is already in progress")

// AugmentSyncMsg enrich the K8s message with user-relevant information
func AugmentSyncMsg(res common.ResourceSyncResult, apiResourceInfoGetter func() ([]kube.APIResourceInfo, error)) (string, error) {
	if strings.Contains(res.Message, "the server could not find the requested resource") {
		resource, err := getAPIResourceInfo(res.ResourceKey.Group, res.ResourceKey.Kind, apiResourceInfoGetter)
		if err != nil {
			return "", fmt.Errorf("failed to get API resource info for group %q and kind %q: %w", res.ResourceKey.Group, res.ResourceKey.Kind, err)
		}
		if resource == nil {
			return fmt.Sprintf("The Kubernetes API could not find %s/%s for requested resource %s/%s. Make sure the %q CRD is installed on the destination cluster.", res.ResourceKey.Group, res.ResourceKey.Kind, res.ResourceKey.Namespace, res.ResourceKey.Name, res.ResourceKey.Kind), nil
		}
		return fmt.Sprintf("The Kubernetes API could not find version %q of %s/%s for requested resource %s/%s. Version %q of %s/%s is installed on the destination cluster.", res.Version, res.ResourceKey.Group, res.ResourceKey.Kind, res.ResourceKey.Namespace, res.ResourceKey.Name, resource.GroupVersionResource.Version, resource.GroupKind.Group, resource.GroupKind.Kind), nil
	}
	// Check if the message contains "metadata.annotation: Too long"
	if strings.Contains(res.Message, "metadata.annotations: Too long: must have at most 262144 bytes") {
		return res.Message + " \n -Additional Info: This error usually means that you are trying to add a large resource on client side. Consider using Server-side apply or syncing with replace enabled. Note: Syncing with Replace enabled is potentially destructive as it may cause resource deletion and re-creation.", nil
	}

	return res.Message, nil
}

// getAPIResourceInfo gets Kubernetes API resource info for the given group and kind. If there's a matching resource
// group _and_ kind, it will return the resource info. If there's a matching kind but no matching group, it will
// return the first resource info that matches the kind. If there's no matching kind, it will return nil.
func getAPIResourceInfo(group, kind string, getAPIResourceInfo func() ([]kube.APIResourceInfo, error)) (*kube.APIResourceInfo, error) {
	apiResources, err := getAPIResourceInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get API resource info: %w", err)
	}

	for _, r := range apiResources {
		if r.GroupKind.Group == group && r.GroupKind.Kind == kind {
			return &r, nil
		}
	}

	for _, r := range apiResources {
		if r.GroupKind.Kind == kind {
			return &r, nil
		}
	}

	return nil, nil
}

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

// FilterByProjectsP returns application pointers which belongs to the specified project
func FilterByProjectsP(apps []*argoappv1.Application, projects []string) []*argoappv1.Application {
	if len(projects) == 0 {
		return apps
	}
	projectsMap := make(map[string]bool)
	for i := range projects {
		projectsMap[projects[i]] = true
	}
	items := make([]*argoappv1.Application, 0)
	for i := 0; i < len(apps); i++ {
		a := apps[i]
		if _, ok := projectsMap[a.Spec.GetProject()]; ok {
			items = append(items, a)
		}
	}
	return items
}

// FilterAppSetsByProjects returns applications which belongs to the specified project
func FilterAppSetsByProjects(appsets []argoappv1.ApplicationSet, projects []string) []argoappv1.ApplicationSet {
	if len(projects) == 0 {
		return appsets
	}
	projectsMap := make(map[string]bool)
	for i := range projects {
		projectsMap[projects[i]] = true
	}
	items := make([]argoappv1.ApplicationSet, 0)
	for i := 0; i < len(appsets); i++ {
		a := appsets[i]
		if _, ok := projectsMap[a.Spec.Template.Spec.GetProject()]; ok {
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
		if apps[i].Spec.GetSource().RepoURL == repo {
			items = append(items, apps[i])
		}
	}
	return items
}

// FilterByRepoP returns application pointers
func FilterByRepoP(apps []*argoappv1.Application, repo string) []*argoappv1.Application {
	if repo == "" {
		return apps
	}
	items := make([]*argoappv1.Application, 0)
	for i := 0; i < len(apps); i++ {
		if apps[i].Spec.GetSource().RepoURL == repo {
			items = append(items, apps[i])
		}
	}
	return items
}

// FilterByCluster returns an application
func FilterByCluster(apps []argoappv1.Application, cluster string) []argoappv1.Application {
	if cluster == "" {
		return apps
	}
	items := make([]argoappv1.Application, 0)
	for i := 0; i < len(apps); i++ {
		if apps[i].Spec.Destination.Server == cluster || apps[i].Spec.Destination.Name == cluster {
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

// FilterByNameP returns pointer applications
// This function is for the changes in #12985.
func FilterByNameP(apps []*argoappv1.Application, name string) []*argoappv1.Application {
	if name == "" {
		return apps
	}
	items := make([]*argoappv1.Application, 0)
	for i := 0; i < len(apps); i++ {
		if apps[i].Name == name {
			items = append(items, apps[i])
			return items
		}
	}
	return items
}

// RefreshApp updates the refresh annotation of an application to coerce the controller to process it
func RefreshApp(appIf v1alpha1.ApplicationInterface, name string, refreshType argoappv1.RefreshType, hydrate bool) (*argoappv1.Application, error) {
	metadata := map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]string{
				argoappv1.AnnotationKeyRefresh: string(refreshType),
			},
		},
	}
	if hydrate {
		metadata["metadata"].(map[string]any)["annotations"].(map[string]string)[argoappv1.AnnotationKeyHydrate] = string(argoappv1.HydrateTypeNormal)
	}

	var err error
	patch, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("error marshaling metadata: %w", err)
	}
	for attempt := 0; attempt < 5; attempt++ {
		app, err := appIf.Patch(context.Background(), name, types.MergePatchType, patch, metav1.PatchOptions{})
		if err == nil {
			log.Infof("Requested app '%s' refresh", name)
			return app.DeepCopy(), nil
		}
		if !apierrors.IsConflict(err) {
			return nil, fmt.Errorf("error patching annotations in application %q: %w", name, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, err
}

func TestRepoWithKnownType(ctx context.Context, repoClient apiclient.RepoServerServiceClient, repo *argoappv1.Repository, isHelm bool, isHelmOci bool, isOCI bool) error {
	repo = repo.DeepCopy()
	switch {
	case isHelm:
		repo.Type = "helm"
	case isOCI:
		repo.Type = "oci"
	case repo.Type != "oci":
		repo.Type = "git"
	}
	repo.EnableOCI = repo.EnableOCI || isHelmOci

	_, err := repoClient.TestRepository(ctx, &apiclient.TestRepositoryRequest{
		Repo: repo,
	})
	if err != nil {
		return fmt.Errorf("repo client error while testing repository: %w", err)
	}

	return nil
}

// ValidateRepo validates the repository specified in application spec. Following is checked:
// * the repository is accessible
// * the path contains valid manifests
// * there are parameters of only one app source type
//
// The plugins parameter is no longer used. It is kept for compatibility with the old signature until Argo CD v3.0.
func ValidateRepo(
	ctx context.Context,
	app *argoappv1.Application,
	repoClientset apiclient.Clientset,
	db db.ArgoDB,
	kubectl kube.Kubectl,
	proj *argoappv1.AppProject,
	settingsMgr *settings.SettingsManager,
) ([]argoappv1.ApplicationCondition, error) {
	spec := &app.Spec

	conditions := make([]argoappv1.ApplicationCondition, 0)

	// Test the repo
	conn, repoClient, err := repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, fmt.Errorf("error instantiating new repo server client: %w", err)
	}
	defer utilio.Close(conn)

	helmOptions, err := settingsMgr.GetHelmSettings()
	if err != nil {
		return nil, fmt.Errorf("error getting helm settings: %w", err)
	}

	helmRepos, err := db.ListHelmRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("error listing helm repos: %w", err)
	}
	permittedHelmRepos, err := GetPermittedRepos(proj, helmRepos)
	if err != nil {
		return nil, fmt.Errorf("error getting permitted repos: %w", err)
	}
	helmRepositoryCredentials, err := db.GetAllHelmRepositoryCredentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting helm repo creds: %w", err)
	}
	ociRepos, err := db.ListOCIRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list oci repositories: %w", err)
	}
	permittedOCIRepos, err := GetPermittedRepos(proj, ociRepos)
	if err != nil {
		return nil, fmt.Errorf("failed to get permitted oci repositories for project %q: %w", proj.Name, err)
	}
	permittedHelmCredentials, err := GetPermittedReposCredentials(proj, helmRepositoryCredentials)
	if err != nil {
		return nil, fmt.Errorf("error getting permitted repo creds: %w", err)
	}
	ociRepositoryCredentials, err := db.GetAllOCIRepositoryCredentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OCI credentials: %w", err)
	}
	permittedOCICredentials, err := GetPermittedReposCredentials(proj, ociRepositoryCredentials)
	if err != nil {
		return nil, fmt.Errorf("failed to get permitted OCI credentials for project %q: %w", proj.Name, err)
	}

	destCluster, err := GetDestinationCluster(ctx, spec.Destination, db)
	if err != nil {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: fmt.Sprintf("Unable to get cluster: %v", err),
		})
		return conditions, nil
	}
	config, err := destCluster.RESTConfig()
	if err != nil {
		return nil, fmt.Errorf("error getting cluster REST config: %w", err)
	}
	//nolint:staticcheck
	destCluster.ServerVersion, err = kubectl.GetServerVersion(config)
	if err != nil {
		return nil, fmt.Errorf("error getting k8s server version: %w", err)
	}
	apiGroups, err := kubectl.GetAPIResources(config, false, cache.NewNoopSettings())
	if err != nil {
		return nil, fmt.Errorf("error getting API resources: %w", err)
	}
	enabledSourceTypes, err := settingsMgr.GetEnabledSourceTypes()
	if err != nil {
		return nil, fmt.Errorf("error getting enabled source types: %w", err)
	}

	sourceCondition, err := validateRepo(
		ctx,
		app,
		db,
		app.Spec.GetSources(),
		repoClient,
		permittedHelmRepos,
		permittedOCIRepos,
		helmOptions,
		destCluster,
		apiGroups,
		proj,
		permittedHelmCredentials,
		permittedOCICredentials,
		enabledSourceTypes,
		settingsMgr)
	if err != nil {
		return nil, err
	}
	conditions = append(conditions, sourceCondition...)

	return conditions, nil
}

func validateRepo(ctx context.Context,
	app *argoappv1.Application,
	db db.ArgoDB,
	sources []argoappv1.ApplicationSource,
	repoClient apiclient.RepoServerServiceClient,
	permittedHelmRepos []*argoappv1.Repository,
	permittedOCIRepos []*argoappv1.Repository,
	helmOptions *argoappv1.HelmOptions,
	cluster *argoappv1.Cluster,
	apiGroups []kube.APIResourceInfo,
	proj *argoappv1.AppProject,
	permittedHelmCredentials []*argoappv1.RepoCreds,
	permittedOCICredentials []*argoappv1.RepoCreds,
	enabledSourceTypes map[string]bool,
	settingsMgr *settings.SettingsManager,
) ([]argoappv1.ApplicationCondition, error) {
	conditions := make([]argoappv1.ApplicationCondition, 0)
	errMessage := ""

	for _, source := range sources {
		repo, err := db.GetRepository(ctx, source.RepoURL, proj.Name)
		if err != nil {
			return nil, err
		}
		if err := TestRepoWithKnownType(ctx, repoClient, repo, source.IsHelm(), source.IsHelmOci(), source.IsOCI()); err != nil {
			errMessage = fmt.Sprintf("repositories not accessible: %v: %v", repo.StringForLogging(), err)
		}
		repoAccessible := false

		if errMessage != "" {
			conditions = append(conditions, argoappv1.ApplicationCondition{
				Type:    argoappv1.ApplicationConditionInvalidSpecError,
				Message: fmt.Sprintf("repository not accessible: %v", errMessage),
			})
		} else {
			repoAccessible = true
		}

		// Verify only one source type is defined
		_, err = source.ExplicitType()
		if err != nil {
			return nil, fmt.Errorf("error verifying source type: %w", err)
		}

		// is the repo inaccessible - abort now
		if !repoAccessible {
			return conditions, nil
		}
	}

	// If using the source hydrator, check the dry source instead of the sync source, since the sync source branch may
	// not exist yet.
	if app.Spec.SourceHydrator != nil {
		sources = []argoappv1.ApplicationSource{app.Spec.SourceHydrator.GetDrySource()}
	}

	refSources, err := GetRefSources(ctx, sources, app.Spec.Project, db.GetRepository, []string{}, false)
	if err != nil {
		return nil, fmt.Errorf("error getting ref sources: %w", err)
	}
	conditions = append(conditions, verifyGenerateManifests(
		ctx,
		db,
		permittedHelmRepos,
		permittedOCIRepos,
		helmOptions,
		app,
		proj,
		sources,
		repoClient,
		//nolint:staticcheck
		cluster.ServerVersion,
		APIResourcesToStrings(apiGroups, true),
		permittedHelmCredentials,
		permittedOCICredentials,
		enabledSourceTypes,
		settingsMgr,
		refSources)...)

	return conditions, nil
}

// GetRefSources creates a map of ref keys (from the sources' 'ref' fields) to information about the referenced source.
// This function also validates the references use allowed characters and does not define the same ref key more than
// once (which would lead to ambiguous references).
func GetRefSources(ctx context.Context, sources argoappv1.ApplicationSources, project string, getRepository func(ctx context.Context, url string, project string) (*argoappv1.Repository, error), revisions []string, isRollback bool) (argoappv1.RefTargetRevisionMapping, error) {
	refSources := make(argoappv1.RefTargetRevisionMapping)
	if len(sources) > 1 {
		// Validate first to avoid unnecessary DB calls.
		refKeys := make(map[string]bool)
		for _, source := range sources {
			if source.Ref == "" {
				continue
			}
			isValidRefKey := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString
			if !isValidRefKey(source.Ref) {
				return nil, fmt.Errorf("sources.ref %s cannot contain any special characters except '_' and '-'", source.Ref)
			}
			refKey := "$" + source.Ref
			if _, ok := refKeys[refKey]; ok {
				return nil, errors.New("invalid sources: multiple sources had the same `ref` key")
			}
			refKeys[refKey] = true
		}
		// Get Repositories for all sources before generating Manifests
		for i, source := range sources {
			if source.Ref == "" {
				continue
			}
			repo, err := getRepository(ctx, source.RepoURL, project)
			if err != nil {
				return nil, fmt.Errorf("failed to get repository %s: %w", source.RepoURL, err)
			}
			refKey := "$" + source.Ref
			revision := source.TargetRevision
			if isRollback {
				revision = revisions[i]
			}
			refSources[refKey] = &argoappv1.RefTarget{
				Repo:           *repo,
				TargetRevision: revision,
				Chart:          source.Chart,
			}
		}
	}
	return refSources, nil
}

func validateSourcePermissions(source argoappv1.ApplicationSource, hasMultipleSources bool) []argoappv1.ApplicationCondition {
	var conditions []argoappv1.ApplicationCondition
	if hasMultipleSources {
		if source.RepoURL == "" || (source.Path == "" && source.Chart == "" && source.Ref == "") {
			conditions = append(conditions, argoappv1.ApplicationCondition{
				Type:    argoappv1.ApplicationConditionInvalidSpecError,
				Message: fmt.Sprintf("spec.source.repoURL and either source.path, source.chart, or source.ref are required for source %s", source),
			})
			return conditions
		}
	} else {
		if source.RepoURL == "" || (source.Path == "" && source.Chart == "") {
			conditions = append(conditions, argoappv1.ApplicationCondition{
				Type:    argoappv1.ApplicationConditionInvalidSpecError,
				Message: "spec.source.repoURL and either spec.source.path or spec.source.chart are required",
			})
			return conditions
		}
	}
	if source.Chart != "" && source.TargetRevision == "" {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: "spec.source.targetRevision is required if the manifest source is a helm chart",
		})
		return conditions
	}

	return conditions
}

func validateSourceHydrator(hydrator *argoappv1.SourceHydrator) []argoappv1.ApplicationCondition {
	var conditions []argoappv1.ApplicationCondition
	if hydrator.DrySource.RepoURL == "" {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: "spec.sourceHydrator.drySource.repoURL is required",
		})
	}
	if hydrator.SyncSource.TargetBranch == "" {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: "spec.sourceHydrator.syncSource.targetBranch is required",
		})
	}
	if hydrator.HydrateTo != nil && hydrator.HydrateTo.TargetBranch == "" {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: "when spec.sourceHydrator.hydrateTo is set, spec.sourceHydrator.hydrateTo.path is required",
		})
	}
	return conditions
}

// ValidatePermissions ensures that the referenced cluster has been added to Argo CD and the app source repo and destination namespace/cluster are permitted in app project
func ValidatePermissions(ctx context.Context, spec *argoappv1.ApplicationSpec, proj *argoappv1.AppProject, db db.ArgoDB) ([]argoappv1.ApplicationCondition, error) {
	conditions := make([]argoappv1.ApplicationCondition, 0)

	switch {
	case spec.SourceHydrator != nil:
		condition := validateSourceHydrator(spec.SourceHydrator)
		if len(condition) > 0 {
			conditions = append(conditions, condition...)
			return conditions, nil
		}
		if !proj.IsSourcePermitted(spec.SourceHydrator.GetDrySource()) {
			conditions = append(conditions, argoappv1.ApplicationCondition{
				Type:    argoappv1.ApplicationConditionInvalidSpecError,
				Message: fmt.Sprintf("application repo %s is not permitted in project '%s'", spec.GetSource().RepoURL, spec.Project),
			})
		}
	case spec.HasMultipleSources():
		for _, source := range spec.Sources {
			condition := validateSourcePermissions(source, spec.HasMultipleSources())
			if len(condition) > 0 {
				conditions = append(conditions, condition...)
				return conditions, nil
			}

			if !proj.IsSourcePermitted(source) {
				conditions = append(conditions, argoappv1.ApplicationCondition{
					Type:    argoappv1.ApplicationConditionInvalidSpecError,
					Message: fmt.Sprintf("application repo %s is not permitted in project '%s'", source.RepoURL, spec.Project),
				})
			}
		}
	default:
		conditions = validateSourcePermissions(spec.GetSource(), spec.HasMultipleSources())
		if len(conditions) > 0 {
			return conditions, nil
		}

		if !proj.IsSourcePermitted(spec.GetSource()) {
			conditions = append(conditions, argoappv1.ApplicationCondition{
				Type:    argoappv1.ApplicationConditionInvalidSpecError,
				Message: fmt.Sprintf("application repo %s is not permitted in project '%s'", spec.GetSource().RepoURL, spec.Project),
			})
		}
	}

	destCluster, err := GetDestinationCluster(ctx, spec.Destination, db)
	if err != nil {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: err.Error(),
		})
		return conditions, nil
	}

	if destCluster.Server != "" {
		permitted, err := proj.IsDestinationPermitted(destCluster, spec.Destination.Namespace, func(project string) ([]*argoappv1.Cluster, error) {
			return db.GetProjectClusters(ctx, project)
		})
		if err != nil {
			return nil, err
		}
		if !permitted {
			conditions = append(conditions, argoappv1.ApplicationCondition{
				Type:    argoappv1.ApplicationConditionInvalidSpecError,
				Message: fmt.Sprintf("application destination server '%s' and namespace '%s' do not match any of the allowed destinations in project '%s'", spec.Destination.Server, spec.Destination.Namespace, spec.Project),
			})
		}
	} else if destCluster.Server == "" {
		conditions = append(conditions, argoappv1.ApplicationCondition{Type: argoappv1.ApplicationConditionInvalidSpecError, Message: ErrDestinationMissing})
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
func GetAppProjectWithScopedResources(ctx context.Context, name string, projLister applicationsv1.AppProjectLister, ns string, settingsManager *settings.SettingsManager, db db.ArgoDB) (*argoappv1.AppProject, argoappv1.Repositories, []*argoappv1.Cluster, error) {
	projOrig, err := projLister.AppProjects(ns).Get(name)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error getting app project %q: %w", name, err)
	}

	project, err := GetAppVirtualProject(projOrig, projLister, settingsManager)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error getting app virtual project: %w", err)
	}

	clusters, err := db.GetProjectClusters(ctx, project.Name)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error getting project clusters: %w", err)
	}
	repos, err := db.GetProjectRepositories(name)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error getting project repos: %w", err)
	}
	return project, repos, clusters, nil
}

// GetAppProjectByName returns a project from an application based on name
func GetAppProjectByName(ctx context.Context, name string, projLister applicationsv1.AppProjectLister, ns string, settingsManager *settings.SettingsManager, db db.ArgoDB) (*argoappv1.AppProject, error) {
	projOrig, err := projLister.AppProjects(ns).Get(name)
	if err != nil {
		return nil, fmt.Errorf("error getting app project %q: %w", name, err)
	}
	project := projOrig.DeepCopy()
	repos, err := db.GetProjectRepositories(name)
	if err != nil {
		return nil, fmt.Errorf("error getting project repositories: %w", err)
	}
	for _, repo := range repos {
		project.Spec.SourceRepos = append(project.Spec.SourceRepos, repo.Repo)
	}
	clusters, err := db.GetProjectClusters(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("error getting project clusters: %w", err)
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

// GetAppProject returns a project from an application. It will also ensure
// that the application is allowed to use the project.
func GetAppProject(ctx context.Context, app *argoappv1.Application, projLister applicationsv1.AppProjectLister, ns string, settingsManager *settings.SettingsManager, db db.ArgoDB) (*argoappv1.AppProject, error) {
	proj, err := GetAppProjectByName(ctx, app.Spec.GetProject(), projLister, ns, settingsManager, db)
	if err != nil {
		return nil, err
	}
	if !proj.IsAppNamespacePermitted(app, ns) {
		return nil, argoappv1.NewErrApplicationNotAllowedToUseProject(app.Name, app.Namespace, proj.Name)
	}
	return proj, nil
}

// verifyGenerateManifests verifies a repo path can generate manifests
func verifyGenerateManifests(
	ctx context.Context,
	db db.ArgoDB,
	helmRepos argoappv1.Repositories,
	ociRepos argoappv1.Repositories,
	helmOptions *argoappv1.HelmOptions,
	app *argoappv1.Application,
	proj *argoappv1.AppProject,
	sources []argoappv1.ApplicationSource,
	repoClient apiclient.RepoServerServiceClient,
	kubeVersion string,
	apiVersions []string,
	repositoryCredentials []*argoappv1.RepoCreds,
	ociRepositoryCredentials []*argoappv1.RepoCreds,
	enableGenerateManifests map[string]bool,
	settingsMgr *settings.SettingsManager,
	refSources argoappv1.RefTargetRevisionMapping,
) []argoappv1.ApplicationCondition {
	var conditions []argoappv1.ApplicationCondition
	// If source is Kustomize add build options
	kustomizeSettings, err := settingsMgr.GetKustomizeSettings()
	if err != nil {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: fmt.Sprintf("Error getting Kustomize settings: %v", err),
		})
		return conditions // Can't perform the next check without settings.
	}

	for _, source := range sources {
		repoRes, err := db.GetRepository(ctx, source.RepoURL, proj.Name)
		if err != nil {
			conditions = append(conditions, argoappv1.ApplicationCondition{
				Type:    argoappv1.ApplicationConditionInvalidSpecError,
				Message: fmt.Sprintf("Unable to get repository: %v", err),
			})
			continue
		}
		kustomizeOptions, err := kustomizeSettings.GetOptions(source)
		if err != nil {
			conditions = append(conditions, argoappv1.ApplicationCondition{
				Type:    argoappv1.ApplicationConditionInvalidSpecError,
				Message: fmt.Sprintf("Error getting Kustomize options: %v", err),
			})
			continue
		}
		installationID, err := settingsMgr.GetInstallationID()
		if err != nil {
			conditions = append(conditions, argoappv1.ApplicationCondition{
				Type:    argoappv1.ApplicationConditionInvalidSpecError,
				Message: fmt.Sprintf("Error getting installation ID: %v", err),
			})
			continue
		}

		appLabelKey, err := settingsMgr.GetAppInstanceLabelKey()
		if err != nil {
			conditions = append(conditions, argoappv1.ApplicationCondition{
				Type:    argoappv1.ApplicationConditionInvalidSpecError,
				Message: fmt.Sprintf("Error getting app label key ID: %v", err),
			})
			continue
		}

		trackingMethod, err := settingsMgr.GetTrackingMethod()
		if err != nil {
			conditions = append(conditions, argoappv1.ApplicationCondition{
				Type:    argoappv1.ApplicationConditionInvalidSpecError,
				Message: fmt.Sprintf("Error getting trackingMethod: %v", err),
			})
			continue
		}

		verifySignature := false
		if len(proj.Spec.SignatureKeys) > 0 && gpg.IsGPGEnabled() {
			verifySignature = true
		}

		repos := helmRepos
		helmRepoCreds := repositoryCredentials
		// If the source is OCI, there is a potential for an OCI image to be a Helm chart and that said chart in
		// turn would have OCI dependencies. To ensure that those dependencies can be resolved, add them to the repos
		// list.
		if source.IsOCI() {
			repos = slices.Clone(helmRepos)
			helmRepoCreds = slices.Clone(repositoryCredentials)
			repos = append(repos, ociRepos...)
			helmRepoCreds = append(helmRepoCreds, ociRepositoryCredentials...)
		}

		req := apiclient.ManifestRequest{
			Repo: &argoappv1.Repository{
				Repo:    source.RepoURL,
				Type:    repoRes.Type,
				Name:    repoRes.Name,
				Proxy:   repoRes.Proxy,
				NoProxy: repoRes.NoProxy,
			},
			VerifySignature:                 verifySignature,
			Repos:                           repos,
			Revision:                        source.TargetRevision,
			AppName:                         app.Name,
			Namespace:                       app.Spec.Destination.Namespace,
			ApplicationSource:               &source,
			AppLabelKey:                     appLabelKey,
			KustomizeOptions:                kustomizeOptions,
			KubeVersion:                     kubeVersion,
			ApiVersions:                     apiVersions,
			HelmOptions:                     helmOptions,
			HelmRepoCreds:                   helmRepoCreds,
			TrackingMethod:                  trackingMethod,
			EnabledSourceTypes:              enableGenerateManifests,
			NoRevisionCache:                 true,
			HasMultipleSources:              app.Spec.HasMultipleSources(),
			RefSources:                      refSources,
			ProjectName:                     proj.Name,
			ProjectSourceRepos:              proj.Spec.SourceRepos,
			AnnotationManifestGeneratePaths: app.GetAnnotation(argoappv1.AnnotationKeyManifestGeneratePaths),
			InstallationID:                  installationID,
		}
		req.Repo.CopyCredentialsFromRepo(repoRes)
		req.Repo.CopySettingsFrom(repoRes)

		// Only check whether we can access the application's path,
		// and not whether it actually contains any manifests.
		_, err = repoClient.GenerateManifest(ctx, &req)
		if err != nil {
			errMessage := fmt.Sprintf("Unable to generate manifests in %s: %s", source.Path, err)
			conditions = append(conditions, argoappv1.ApplicationCondition{
				Type:    argoappv1.ApplicationConditionInvalidSpecError,
				Message: errMessage,
			})
		}
	}

	return conditions
}

// SetAppOperation updates an application with the specified operation, retrying conflict errors
func SetAppOperation(appIf v1alpha1.ApplicationInterface, appName string, op *argoappv1.Operation) (*argoappv1.Application, error) {
	for {
		a, err := appIf.Get(context.Background(), appName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error getting application %q: %w", appName, err)
		}
		a = a.DeepCopy()
		if a.Operation != nil {
			return nil, ErrAnotherOperationInProgress
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
		if !apierrors.IsConflict(err) {
			return nil, fmt.Errorf("error updating application %q: %w", appName, err)
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

// IncludeResource The app resource is checked against the include or exclude filters.
// If exclude filters are present, they are evaluated only after all include filters have been assessed.
func IncludeResource(resourceName string, resourceNamespace string, gvk schema.GroupVersionKind,
	syncOperationResources []*argoappv1.SyncOperationResource,
) bool {
	includeResource := false
	foundIncludeRule := false
	// Evaluate include filters only in this loop.
	for _, syncOperationResource := range syncOperationResources {
		if syncOperationResource.Exclude {
			continue
		}
		foundIncludeRule = true
		includeResource = syncOperationResource.Compare(resourceName, resourceNamespace, gvk)
		if includeResource {
			break
		}
	}

	// if a resource is present both in include and in exclude, the exclude wins.
	// that including it here is a temporary decision for the use case when no include rules exist,
	// but it still might be excluded later if it matches an exclude rule:
	if !foundIncludeRule {
		includeResource = true
	}
	// No needs to evaluate exclude filters when the resource is not included.
	if !includeResource {
		return false
	}
	// Evaluate exclude filters only in this loop.
	for _, syncOperationResource := range syncOperationResources {
		if syncOperationResource.Exclude && syncOperationResource.Compare(resourceName, resourceNamespace, gvk) {
			return false
		}
	}
	return true
}

// NormalizeApplicationSpec will normalize an application spec to a preferred state. This is used
// for migrating application objects which are using deprecated legacy fields into the new fields,
// and defaulting fields in the spec (e.g. spec.project)
func NormalizeApplicationSpec(spec *argoappv1.ApplicationSpec) *argoappv1.ApplicationSpec {
	spec = spec.DeepCopy()
	if spec.Project == "" {
		spec.Project = argoappv1.DefaultAppProjectName
	}
	if spec.SyncPolicy.IsZero() {
		spec.SyncPolicy = nil
	}
	if len(spec.Sources) > 0 {
		for _, source := range spec.Sources {
			NormalizeSource(&source)
		}
	} else if spec.Source != nil {
		// In practice, spec.Source should never be nil.
		NormalizeSource(spec.Source)
	}
	return spec
}

func NormalizeSource(source *argoappv1.ApplicationSource) *argoappv1.ApplicationSource {
	// 3. If any app sources are their zero values, then nil out the pointers to the source spec.
	// This makes it easier for users to switch between app source types if they are not using
	// any of the source-specific parameters.
	if source.Kustomize != nil && source.Kustomize.IsZero() {
		source.Kustomize = nil
	}
	if source.Helm != nil && source.Helm.IsZero() {
		source.Helm = nil
	}
	if source.Directory != nil && source.Directory.IsZero() {
		switch {
		case source.Directory.Exclude != "" && source.Directory.Include != "":
			source.Directory = &argoappv1.ApplicationSourceDirectory{Exclude: source.Directory.Exclude, Include: source.Directory.Include}
		case source.Directory.Exclude != "":
			source.Directory = &argoappv1.ApplicationSourceDirectory{Exclude: source.Directory.Exclude}
		case source.Directory.Include != "":
			source.Directory = &argoappv1.ApplicationSourceDirectory{Include: source.Directory.Include}
		default:
			source.Directory = nil
		}
	}
	return source
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

type ClusterGetter interface {
	GetCluster(ctx context.Context, name string) (*argoappv1.Cluster, error)
	GetClusterServersByName(ctx context.Context, server string) ([]string, error)
}

// GetDestinationCluster returns the cluster object based on the destination server or name. If both are provided or
// both are empty, an error is returned. If the destination server is provided, the cluster is fetched by the server
// URL. If the destination name is provided, the cluster is fetched by the name. If multiple clusters have the specified
// name, an error is returned.
func GetDestinationCluster(ctx context.Context, destination argoappv1.ApplicationDestination, db ClusterGetter) (*argoappv1.Cluster, error) {
	if destination.Name != "" && destination.Server != "" {
		return nil, fmt.Errorf("application destination can't have both name and server defined: %s %s", destination.Name, destination.Server)
	}
	if destination.Server != "" {
		cluster, err := db.GetCluster(ctx, destination.Server)
		if err != nil {
			return nil, fmt.Errorf("error getting cluster by server %q: %w", destination.Server, err)
		}
		return cluster, nil
	} else if destination.Name != "" {
		clusterURLs, err := db.GetClusterServersByName(ctx, destination.Name)
		if err != nil {
			return nil, fmt.Errorf("error getting cluster by name %q: %w", destination.Name, err)
		}
		if len(clusterURLs) == 0 {
			return nil, fmt.Errorf("there are no clusters with this name: %s", destination.Name)
		}
		if len(clusterURLs) > 1 {
			return nil, fmt.Errorf("there are %d clusters with the same name: [%s]", len(clusterURLs), strings.Join(clusterURLs, " "))
		}
		cluster, err := db.GetCluster(ctx, clusterURLs[0])
		if err != nil {
			return nil, fmt.Errorf("error getting cluster by URL: %w", err)
		}
		return cluster, nil
	}
	// nolint:staticcheck // Error constant is very old, shouldn't lowercase the first letter.
	return nil, errors.New(ErrDestinationMissing)
}

func GetGlobalProjects(proj *argoappv1.AppProject, projLister applicationsv1.AppProjectLister, settingsManager *settings.SettingsManager) []*argoappv1.AppProject {
	gps, err := settingsManager.GetGlobalProjectsSettings()
	globalProjects := make([]*argoappv1.AppProject, 0)

	if err != nil {
		log.Warnf("Failed to get global project settings: %v", err)
		return globalProjects
	}

	for _, gp := range gps {
		// The project itself is not its own the global project
		if proj.Name == gp.ProjectName {
			continue
		}

		selector, err := metav1.LabelSelectorAsSelector(&gp.LabelSelector)
		if err != nil {
			break
		}
		// Get projects which match the label selector, then see if proj is a match
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
		// If proj is a match for this global project setting, then it is its global project
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

func GenerateSpecIsDifferentErrorMessage(entity string, a, b any) string {
	basicMsg := fmt.Sprintf("existing %s spec is different; use upsert flag to force update", entity)
	difference, _ := GetDifferentPathsBetweenStructs(a, b)
	if len(difference) == 0 {
		return basicMsg
	}
	return fmt.Sprintf("%s; difference in keys %q", basicMsg, strings.Join(difference, ","))
}

func GetDifferentPathsBetweenStructs(a, b any) ([]string, error) {
	var difference []string
	changelog, err := diff.Diff(a, b)
	if err != nil {
		return nil, fmt.Errorf("error during diff: %w", err)
	}
	for _, changeItem := range changelog {
		difference = append(difference, changeItem.Path...)
	}
	return difference, nil
}

// parseName will split the qualified name into its components, which are separated by the delimiter.
// If delimiter is not contained in the string qualifiedName then returned namespace is defaultNs.
func parseName(qualifiedName string, defaultNs string, delim string) (name string, namespace string) {
	t := strings.SplitN(qualifiedName, delim, 2)
	if len(t) == 2 {
		namespace = t[0]
		name = t[1]
	} else {
		namespace = defaultNs
		name = t[0]
	}
	return
}

// ParseAppNamespacedName parses a namespaced name in the format namespace/name
// and returns the components. If name wasn't namespaced, defaultNs will be
// returned as namespace component.
func ParseFromQualifiedName(qualifiedAppName string, defaultNs string) (appName string, appNamespace string) {
	return parseName(qualifiedAppName, defaultNs, "/")
}

// ParseInstanceName parses a namespaced name in the format namespace_name
// and returns the components. If name wasn't namespaced, defaultNs will be
// returned as namespace component.
func ParseInstanceName(appName string, defaultNs string) (string, string) {
	return parseName(appName, defaultNs, "_")
}

// AppInstanceName returns the value to be used for app instance labels from
// the combination of appName, appNs and defaultNs.
func AppInstanceName(appName, appNs, defaultNs string) string {
	if appNs == "" || appNs == defaultNs {
		return appName
	}
	return appNs + "_" + appName
}

// InstanceNameFromQualified returns the value to be used for app
func InstanceNameFromQualified(name string, defaultNs string) string {
	appName, appNs := ParseFromQualifiedName(name, defaultNs)
	return AppInstanceName(appName, appNs, defaultNs)
}

// ErrProjectNotPermitted returns an error to indicate that an application
// identified by appName and appNamespace is not allowed to use the project
// identified by projName.
func ErrProjectNotPermitted(appName, appNamespace, projName string) error {
	return fmt.Errorf("application '%s' in namespace '%s' is not permitted to use project '%s'", appName, appNamespace, projName)
}

// IsValidPodName checks that a podName is valid
func IsValidPodName(name string) bool {
	// https://github.com/kubernetes/kubernetes/blob/976a940f4a4e84fe814583848f97b9aafcdb083f/pkg/apis/core/validation/validation.go#L241
	validationErrors := apimachineryvalidation.NameIsDNSSubdomain(name, false)
	return len(validationErrors) == 0
}

// IsValidAppName checks if the name can be used as application name
func IsValidAppName(name string) bool {
	// app names have the same rules as pods.
	return IsValidPodName(name)
}

// IsValidProjectName checks if the name can be used as project name
func IsValidProjectName(name string) bool {
	// project names have the same rules as pods.
	return IsValidPodName(name)
}

// IsValidNamespaceName checks that a namespace name is valid
func IsValidNamespaceName(name string) bool {
	// https://github.com/kubernetes/kubernetes/blob/976a940f4a4e84fe814583848f97b9aafcdb083f/pkg/apis/core/validation/validation.go#L262
	validationErrors := apimachineryvalidation.ValidateNamespaceName(name, false)
	return len(validationErrors) == 0
}

// IsValidContainerName checks that a containerName is valid
func IsValidContainerName(name string) bool {
	// https://github.com/kubernetes/kubernetes/blob/53a9d106c4aabcd550cc32ae4e8004f32fb0ae7b/pkg/api/validation/validation.go#L280
	validationErrors := apimachineryvalidation.NameIsDNSLabel(name, false)
	return len(validationErrors) == 0
}

// GetAppEventLabels returns a map of labels to add to a K8s event.
// The Application and its AppProject labels are compared against the `resource.includeEventLabelKeys` key in argocd-cm.
// If matched, the corresponding labels are returned to be added to the generated event. In case of a conflict
// between labels on the Application and AppProject, the Application label values are prioritized and added to the event.
// Furthermore, labels specified in `resource.excludeEventLabelKeys` in argocd-cm are removed from the event labels, if they were included.
func GetAppEventLabels(ctx context.Context, app *argoappv1.Application, projLister applicationsv1.AppProjectLister, ns string, settingsManager *settings.SettingsManager, db db.ArgoDB) map[string]string {
	eventLabels := make(map[string]string)

	// Get all app & app-project labels
	labels := app.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	proj, err := GetAppProject(ctx, app, projLister, ns, settingsManager, db)
	if err == nil {
		for k, v := range proj.Labels {
			_, found := labels[k]
			if !found {
				labels[k] = v
			}
		}
	} else {
		log.Warn(err)
	}

	// Filter out event labels to include
	inKeys := settingsManager.GetIncludeEventLabelKeys()
	for k, v := range labels {
		found := glob.MatchStringInList(inKeys, k, glob.GLOB)
		if found {
			eventLabels[k] = v
		}
	}

	// Remove excluded event labels
	exKeys := settingsManager.GetExcludeEventLabelKeys()
	for k := range eventLabels {
		found := glob.MatchStringInList(exKeys, k, glob.GLOB)
		if found {
			delete(eventLabels, k)
		}
	}

	return eventLabels
}
