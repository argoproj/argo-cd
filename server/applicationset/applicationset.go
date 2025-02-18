package applicationset

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/argoproj/pkg/sync"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsettemplate "github.com/argoproj/argo-cd/v2/applicationset/controllers/template"
	"github.com/argoproj/argo-cd/v2/applicationset/generators"
	"github.com/argoproj/argo-cd/v2/applicationset/services"
	appsetstatus "github.com/argoproj/argo-cd/v2/applicationset/status"
	appsetutils "github.com/argoproj/argo-cd/v2/applicationset/utils"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/applicationset"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	repoapiclient "github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/collections"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/github_app"
	"github.com/argoproj/argo-cd/v2/util/rbac"
	"github.com/argoproj/argo-cd/v2/util/security"
	"github.com/argoproj/argo-cd/v2/util/session"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

type Server struct {
	ns                       string
	db                       db.ArgoDB
	enf                      *rbac.Enforcer
	k8sClient                kubernetes.Interface
	dynamicClient            dynamic.Interface
	client                   client.Client
	repoClientSet            repoapiclient.Clientset
	appclientset             appclientset.Interface
	appsetInformer           cache.SharedIndexInformer
	appsetLister             applisters.ApplicationSetLister
	projLister               applisters.AppProjectNamespaceLister
	auditLogger              *argo.AuditLogger
	settings                 *settings.SettingsManager
	projectLock              sync.KeyLock
	enabledNamespaces        []string
	GitSubmoduleEnabled      bool
	EnableNewGitFileGlobbing bool
	ScmRootCAPath            string
	AllowedScmProviders      []string
	EnableScmProviders       bool
}

// NewServer returns a new instance of the ApplicationSet service
func NewServer(
	db db.ArgoDB,
	kubeclientset kubernetes.Interface,
	dynamicClientset dynamic.Interface,
	kubeControllerClientset client.Client,
	enf *rbac.Enforcer,
	repoClientSet repoapiclient.Clientset,
	appclientset appclientset.Interface,
	appsetInformer cache.SharedIndexInformer,
	appsetLister applisters.ApplicationSetLister,
	projLister applisters.AppProjectNamespaceLister,
	settings *settings.SettingsManager,
	namespace string,
	projectLock sync.KeyLock,
	enabledNamespaces []string,
	gitSubmoduleEnabled bool,
	enableNewGitFileGlobbing bool,
	scmRootCAPath string,
	allowedScmProviders []string,
	enableScmProviders bool,
	enableK8sEvent []string,
) applicationset.ApplicationSetServiceServer {
	s := &Server{
		ns:                       namespace,
		db:                       db,
		enf:                      enf,
		dynamicClient:            dynamicClientset,
		client:                   kubeControllerClientset,
		k8sClient:                kubeclientset,
		repoClientSet:            repoClientSet,
		appclientset:             appclientset,
		appsetInformer:           appsetInformer,
		appsetLister:             appsetLister,
		projLister:               projLister,
		settings:                 settings,
		projectLock:              projectLock,
		auditLogger:              argo.NewAuditLogger(namespace, kubeclientset, "argocd-server", enableK8sEvent),
		enabledNamespaces:        enabledNamespaces,
		GitSubmoduleEnabled:      gitSubmoduleEnabled,
		EnableNewGitFileGlobbing: enableNewGitFileGlobbing,
		ScmRootCAPath:            scmRootCAPath,
		AllowedScmProviders:      allowedScmProviders,
		EnableScmProviders:       enableScmProviders,
	}
	return s
}

func (s *Server) Get(ctx context.Context, q *applicationset.ApplicationSetGetQuery) (*v1alpha1.ApplicationSet, error) {
	namespace := s.appsetNamespaceOrDefault(q.AppsetNamespace)

	if !s.isNamespaceEnabled(namespace) {
		return nil, security.NamespaceNotPermittedError(namespace)
	}

	a, err := s.appsetLister.ApplicationSets(namespace).Get(q.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting ApplicationSet: %w", err)
	}
	if err = s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplicationSets, rbacpolicy.ActionGet, a.RBACName(s.ns)); err != nil {
		return nil, err
	}

	return a, nil
}

// List returns list of ApplicationSets
func (s *Server) List(ctx context.Context, q *applicationset.ApplicationSetListQuery) (*v1alpha1.ApplicationSetList, error) {
	selector, err := labels.Parse(q.GetSelector())
	if err != nil {
		return nil, fmt.Errorf("error parsing the selector: %w", err)
	}

	var appsets []*v1alpha1.ApplicationSet
	if q.AppsetNamespace == "" {
		appsets, err = s.appsetLister.List(selector)
	} else {
		appsets, err = s.appsetLister.ApplicationSets(q.AppsetNamespace).List(selector)
	}

	if err != nil {
		return nil, fmt.Errorf("error listing ApplicationSets with selectors: %w", err)
	}

	newItems := make([]v1alpha1.ApplicationSet, 0)
	for _, a := range appsets {
		// Skip any application that is neither in the conrol plane's namespace
		// nor in the list of enabled namespaces.
		if !security.IsNamespaceEnabled(a.Namespace, s.ns, s.enabledNamespaces) {
			continue
		}

		if s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplicationSets, rbacpolicy.ActionGet, a.RBACName(s.ns)) {
			newItems = append(newItems, *a)
		}
	}

	newItems = argo.FilterAppSetsByProjects(newItems, q.Projects)

	// Sort found applicationsets by name
	sort.Slice(newItems, func(i, j int) bool {
		return newItems[i].Name < newItems[j].Name
	})

	appsetList := &v1alpha1.ApplicationSetList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: s.appsetInformer.LastSyncResourceVersion(),
		},
		Items: newItems,
	}
	return appsetList, nil
}

func (s *Server) Create(ctx context.Context, q *applicationset.ApplicationSetCreateRequest) (*v1alpha1.ApplicationSet, error) {
	appset := q.GetApplicationset()

	if appset == nil {
		return nil, fmt.Errorf("error creating ApplicationSets: ApplicationSets is nil in request")
	}

	projectName, err := s.validateAppSet(appset)
	if err != nil {
		return nil, fmt.Errorf("error validating ApplicationSets: %w", err)
	}

	namespace := s.appsetNamespaceOrDefault(appset.Namespace)

	if !s.isNamespaceEnabled(namespace) {
		return nil, security.NamespaceNotPermittedError(namespace)
	}

	if err := s.checkCreatePermissions(ctx, appset, projectName); err != nil {
		return nil, fmt.Errorf("error checking create permissions for ApplicationSets %s : %w", appset.Name, err)
	}

	if q.GetDryRun() {
		apps, err := s.generateApplicationSetApps(ctx, log.WithField("applicationset", appset.Name), *appset, namespace)
		if err != nil {
			return nil, fmt.Errorf("unable to generate Applications of ApplicationSet: %w", err)
		}

		statusMap := appsetstatus.GetResourceStatusMap(appset)
		statusMap = appsetstatus.BuildResourceStatus(statusMap, apps)

		statuses := []v1alpha1.ResourceStatus{}
		for _, status := range statusMap {
			statuses = append(statuses, status)
		}
		appset.Status.Resources = statuses
		return appset, nil
	}

	s.projectLock.RLock(projectName)
	defer s.projectLock.RUnlock(projectName)

	created, err := s.appclientset.ArgoprojV1alpha1().ApplicationSets(namespace).Create(ctx, appset, metav1.CreateOptions{})
	if err == nil {
		s.logAppSetEvent(created, ctx, argo.EventReasonResourceCreated, "created ApplicationSet")
		s.waitSync(created)
		return created, nil
	}

	if !apierr.IsAlreadyExists(err) {
		return nil, fmt.Errorf("error creating ApplicationSet: %w", err)
	}
	// act idempotent if existing spec matches new spec
	existing, err := s.appclientset.ArgoprojV1alpha1().ApplicationSets(s.ns).Get(ctx, appset.Name, metav1.GetOptions{
		ResourceVersion: "",
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to check existing ApplicationSet details: %v", err)
	}

	equalSpecs := reflect.DeepEqual(existing.Spec, appset.Spec) &&
		reflect.DeepEqual(existing.Labels, appset.Labels) &&
		reflect.DeepEqual(existing.Annotations, appset.Annotations) &&
		reflect.DeepEqual(existing.Finalizers, appset.Finalizers)

	if equalSpecs {
		return existing, nil
	}

	if !q.Upsert {
		return nil, status.Errorf(codes.InvalidArgument, "existing ApplicationSet spec is different, use upsert flag to force update")
	}
	if err = s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplicationSets, rbacpolicy.ActionUpdate, appset.RBACName(s.ns)); err != nil {
		return nil, err
	}
	updated, err := s.updateAppSet(existing, appset, ctx, true)
	if err != nil {
		return nil, fmt.Errorf("error updating ApplicationSets: %w", err)
	}
	return updated, nil
}

func (s *Server) generateApplicationSetApps(ctx context.Context, logEntry *log.Entry, appset v1alpha1.ApplicationSet, namespace string) ([]v1alpha1.Application, error) {
	argoCDDB := s.db

	scmConfig := generators.NewSCMConfig(s.ScmRootCAPath, s.AllowedScmProviders, s.EnableScmProviders, github_app.NewAuthCredentials(argoCDDB.(db.RepoCredsDB)))

	getRepository := func(ctx context.Context, url, project string) (*v1alpha1.Repository, error) {
		return s.db.GetRepository(ctx, url, project)
	}
	argoCDService, err := services.NewArgoCDService(getRepository, s.GitSubmoduleEnabled, s.repoClientSet, s.EnableNewGitFileGlobbing)
	if err != nil {
		return nil, fmt.Errorf("error creating ArgoCDService: %w", err)
	}

	appSetGenerators := generators.GetGenerators(ctx, s.client, s.k8sClient, namespace, argoCDService, s.dynamicClient, scmConfig)

	apps, _, err := appsettemplate.GenerateApplications(logEntry, appset, appSetGenerators, &appsetutils.Render{}, s.client)
	if err != nil {
		return nil, fmt.Errorf("error generating applications: %w", err)
	}
	return apps, nil
}

func (s *Server) updateAppSet(appset *v1alpha1.ApplicationSet, newAppset *v1alpha1.ApplicationSet, ctx context.Context, merge bool) (*v1alpha1.ApplicationSet, error) {
	if appset != nil && appset.Spec.Template.Spec.Project != newAppset.Spec.Template.Spec.Project {
		// When changing projects, caller must have applicationset create and update privileges in new project
		// NOTE: the update check was already verified in the caller to this function
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplicationSets, rbacpolicy.ActionCreate, newAppset.RBACName(s.ns)); err != nil {
			return nil, err
		}
		// They also need 'update' privileges in the old project
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplicationSets, rbacpolicy.ActionUpdate, appset.RBACName(s.ns)); err != nil {
			return nil, err
		}
	}

	for i := 0; i < 10; i++ {
		appset.Spec = newAppset.Spec
		if merge {
			appset.Labels = collections.MergeStringMaps(appset.Labels, newAppset.Labels)
			appset.Annotations = collections.MergeStringMaps(appset.Annotations, newAppset.Annotations)
		} else {
			appset.Labels = newAppset.Labels
			appset.Annotations = newAppset.Annotations
		}

		res, err := s.appclientset.ArgoprojV1alpha1().ApplicationSets(s.ns).Update(ctx, appset, metav1.UpdateOptions{})
		if err == nil {
			s.logAppSetEvent(appset, ctx, argo.EventReasonResourceUpdated, "updated ApplicationSets spec")
			s.waitSync(res)
			return res, nil
		}
		if !apierr.IsConflict(err) {
			return nil, err
		}

		appset, err = s.appclientset.ArgoprojV1alpha1().ApplicationSets(s.ns).Get(ctx, newAppset.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error getting ApplicationSets: %w", err)
		}
	}
	return nil, status.Errorf(codes.Internal, "Failed to update ApplicationSets. Too many conflicts")
}

func (s *Server) Delete(ctx context.Context, q *applicationset.ApplicationSetDeleteRequest) (*applicationset.ApplicationSetResponse, error) {
	namespace := s.appsetNamespaceOrDefault(q.AppsetNamespace)

	appset, err := s.appclientset.ArgoprojV1alpha1().ApplicationSets(namespace).Get(ctx, q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting ApplicationSets: %w", err)
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplicationSets, rbacpolicy.ActionDelete, appset.RBACName(s.ns)); err != nil {
		return nil, err
	}

	s.projectLock.RLock(appset.Spec.Template.Spec.Project)
	defer s.projectLock.RUnlock(appset.Spec.Template.Spec.Project)

	err = s.appclientset.ArgoprojV1alpha1().ApplicationSets(namespace).Delete(ctx, q.Name, metav1.DeleteOptions{})
	if err != nil {
		return nil, fmt.Errorf("error deleting ApplicationSets: %w", err)
	}
	s.logAppSetEvent(appset, ctx, argo.EventReasonResourceDeleted, "deleted ApplicationSets")
	return &applicationset.ApplicationSetResponse{}, nil
}

func (s *Server) ResourceTree(ctx context.Context, q *applicationset.ApplicationSetTreeQuery) (*v1alpha1.ApplicationSetTree, error) {
	namespace := s.appsetNamespaceOrDefault(q.AppsetNamespace)

	if !s.isNamespaceEnabled(namespace) {
		return nil, security.NamespaceNotPermittedError(namespace)
	}

	a, err := s.appclientset.ArgoprojV1alpha1().ApplicationSets(namespace).Get(ctx, q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting ApplicationSet: %w", err)
	}
	if err = s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplicationSets, rbacpolicy.ActionGet, a.RBACName(s.ns)); err != nil {
		return nil, err
	}

	return s.buildApplicationSetTree(a)
}

func (s *Server) Generate(ctx context.Context, q *applicationset.ApplicationSetGenerateRequest) (*applicationset.ApplicationSetGenerateResponse, error) {
	appset := q.GetApplicationSet()

	if appset == nil {
		return nil, fmt.Errorf("error creating ApplicationSets: ApplicationSets is nil in request")
	}
	namespace := s.appsetNamespaceOrDefault(appset.Namespace)

	if !s.isNamespaceEnabled(namespace) {
		return nil, security.NamespaceNotPermittedError(namespace)
	}
	projectName, err := s.validateAppSet(appset)
	if err != nil {
		return nil, fmt.Errorf("error validating ApplicationSets: %w", err)
	}
	if err := s.checkCreatePermissions(ctx, appset, projectName); err != nil {
		return nil, fmt.Errorf("error checking create permissions for ApplicationSets %s : %w", appset.Name, err)
	}

	logs := bytes.NewBuffer(nil)
	logger := log.New()
	logger.SetOutput(logs)

	apps, err := s.generateApplicationSetApps(ctx, logger.WithField("applicationset", appset.Name), *appset, namespace)
	if err != nil {
		return nil, fmt.Errorf("unable to generate Applications of ApplicationSet: %w\n%s", err, logs.String())
	}
	res := &applicationset.ApplicationSetGenerateResponse{}
	for i := range apps {
		res.Applications = append(res.Applications, &apps[i])
	}
	return res, nil
}

func (s *Server) buildApplicationSetTree(a *v1alpha1.ApplicationSet) (*v1alpha1.ApplicationSetTree, error) {
	var tree v1alpha1.ApplicationSetTree

	gvk := v1alpha1.ApplicationSetSchemaGroupVersionKind
	parentRefs := []v1alpha1.ResourceRef{
		{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind, Name: a.Name, Namespace: a.Namespace, UID: string(a.UID)},
	}

	apps := a.Status.Resources
	for _, app := range apps {
		tree.Nodes = append(tree.Nodes, v1alpha1.ResourceNode{
			Health: app.Health,
			ResourceRef: v1alpha1.ResourceRef{
				Name:      app.Name,
				Group:     app.Group,
				Version:   app.Version,
				Kind:      app.Kind,
				Namespace: a.Namespace,
			},
			ParentRefs: parentRefs,
		})
	}
	tree.Normalize()

	return &tree, nil
}

func (s *Server) validateAppSet(appset *v1alpha1.ApplicationSet) (string, error) {
	if appset == nil {
		return "", fmt.Errorf("ApplicationSet cannot be validated for nil value")
	}

	projectName := appset.Spec.Template.Spec.Project

	if strings.Contains(projectName, "{{") {
		return "", fmt.Errorf("the Argo CD API does not currently support creating ApplicationSets with templated `project` fields")
	}

	if err := appsetutils.CheckInvalidGenerators(appset); err != nil {
		return "", err
	}

	return projectName, nil
}

func (s *Server) checkCreatePermissions(ctx context.Context, appset *v1alpha1.ApplicationSet, projectName string) error {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplicationSets, rbacpolicy.ActionCreate, appset.RBACName(s.ns)); err != nil {
		return err
	}

	_, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(ctx, projectName, metav1.GetOptions{})
	if err != nil {
		if apierr.IsNotFound(err) {
			return status.Errorf(codes.InvalidArgument, "ApplicationSet references project %s which does not exist", projectName)
		}
		return fmt.Errorf("error getting ApplicationSet's project %q: %w", projectName, err)
	}

	return nil
}

var informerSyncTimeout = 2 * time.Second

// waitSync is a helper to wait until the application informer cache is synced after create/update.
// It waits until the app in the informer, has a resource version greater than the version in the
// supplied app, or after 2 seconds, whichever comes first. Returns true if synced.
// We use an informer cache for read operations (Get, List). Since the cache is only
// eventually consistent, it is possible that it doesn't reflect an application change immediately
// after a mutating API call (create/update). This function should be called after a creates &
// update to give a probable (but not guaranteed) chance of being up-to-date after the create/update.
func (s *Server) waitSync(appset *v1alpha1.ApplicationSet) {
	logCtx := log.WithField("applicationset", appset.Name)
	deadline := time.Now().Add(informerSyncTimeout)
	minVersion, err := strconv.Atoi(appset.ResourceVersion)
	if err != nil {
		logCtx.Warnf("waitSync failed: could not parse resource version %s", appset.ResourceVersion)
		time.Sleep(50 * time.Millisecond) // sleep anyway
		return
	}
	for {
		if currAppset, err := s.appsetLister.ApplicationSets(appset.Namespace).Get(appset.Name); err == nil {
			currVersion, err := strconv.Atoi(currAppset.ResourceVersion)
			if err == nil && currVersion >= minVersion {
				return
			}
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	logCtx.Warnf("waitSync failed: timed out")
}

func (s *Server) logAppSetEvent(a *v1alpha1.ApplicationSet, ctx context.Context, reason string, action string) {
	eventInfo := argo.EventInfo{Type: v1.EventTypeNormal, Reason: reason}
	user := session.Username(ctx)
	if user == "" {
		user = "Unknown user"
	}
	message := fmt.Sprintf("%s %s", user, action)
	s.auditLogger.LogAppSetEvent(a, eventInfo, message, user)
}

func (s *Server) appsetNamespaceOrDefault(appNs string) string {
	if appNs == "" {
		return s.ns
	} else {
		return appNs
	}
}

func (s *Server) isNamespaceEnabled(namespace string) bool {
	return security.IsNamespaceEnabled(namespace, s.ns, s.enabledNamespaces)
}
