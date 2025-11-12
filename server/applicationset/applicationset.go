package applicationset

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/argoproj/pkg/v2/sync"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"

	appsetstatus "github.com/argoproj/argo-cd/v3/applicationset/status"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/applicationset"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned"
	applisters "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
	repoapiclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/rbac"
	"github.com/argoproj/argo-cd/v3/util/security"
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
	auditLogger              *argo.AuditLogger
	projectLock              sync.KeyLock
	enabledNamespaces        []string
	GitSubmoduleEnabled      bool
	EnableNewGitFileGlobbing bool
	ScmRootCAPath            string
	AllowedScmProviders      []string
	EnableScmProviders       bool
	EnableGitHubAPIMetrics   bool
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
	namespace string,
	projectLock sync.KeyLock,
	enabledNamespaces []string,
	gitSubmoduleEnabled bool,
	enableNewGitFileGlobbing bool,
	scmRootCAPath string,
	allowedScmProviders []string,
	enableScmProviders bool,
	enableGitHubAPIMetrics bool,
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
		projectLock:              projectLock,
		auditLogger:              argo.NewAuditLogger(kubeclientset, "argocd-server", enableK8sEvent),
		enabledNamespaces:        enabledNamespaces,
		GitSubmoduleEnabled:      gitSubmoduleEnabled,
		EnableNewGitFileGlobbing: enableNewGitFileGlobbing,
		ScmRootCAPath:            scmRootCAPath,
		AllowedScmProviders:      allowedScmProviders,
		EnableScmProviders:       enableScmProviders,
		EnableGitHubAPIMetrics:   enableGitHubAPIMetrics,
	}
	return s
}

// Get returns an applicationset by name
func (s *Server) Get(ctx context.Context, q *applicationset.ApplicationSetGetQuery) (*v1alpha1.ApplicationSet, error) {
	namespace := s.appsetNamespaceOrDefault(q.AppsetNamespace)

	if !s.isNamespaceEnabled(namespace) {
		return nil, security.NamespaceNotPermittedError(namespace)
	}

	a, err := s.appsetLister.ApplicationSets(namespace).Get(q.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting ApplicationSet: %w", err)
	}

	err = s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplicationSets, rbac.ActionGet, a.RBACName(s.ns))
	if err != nil {
		return nil, err
	}

	// Project filtering
	if q.Project != "" {
		templateProject := a.Spec.Template.Spec.Project
		if templateProject != q.Project {
			return nil, fmt.Errorf("ApplicationSet %q does not belong to project %q", q.Name, q.Project)
		}
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

		if s.enf.Enforce(ctx.Value("claims"), rbac.ResourceApplicationSets, rbac.ActionGet, a.RBACName(s.ns)) {
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

// Create creates an ApplicationSet
func (s *Server) Create(ctx context.Context, q *applicationset.ApplicationSetCreateRequest) (*v1alpha1.ApplicationSet, error) {
	appset := q.GetApplicationset()

	if appset == nil {
		return nil, errors.New("error creating ApplicationSets: ApplicationSets is nil in request")
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
		apps, err := s.generateApplicationSetApps(ctx, log.WithField("applicationset", appset.Name), *appset)
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
		s.logAppSetEvent(ctx, created, argo.EventReasonResourceCreated, "created ApplicationSet")
		s.waitSync(created)
		return created, nil
	}

	if !apierrors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("error creating ApplicationSet: %w", err)
	}
	// act idempotent if existing spec matches new spec
	existing, err := s.appclientset.ArgoprojV1alpha1().ApplicationSets(namespace).Get(ctx, appset.Name, metav1.GetOptions{
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
	err = s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplicationSets, rbac.ActionUpdate, appset.RBACName(s.ns))
	if err != nil {
		return nil, err
	}
	updated, err := s.updateAppSet(ctx, existing, appset, true)
	if err != nil {
		return nil, fmt.Errorf("error updating ApplicationSets: %w", err)
	}
	return updated, nil
}

// Delete deletes and ApplicationSet
func (s *Server) Delete(ctx context.Context, q *applicationset.ApplicationSetDeleteRequest) (*applicationset.ApplicationSetResponse, error) {
	namespace := s.appsetNamespaceOrDefault(q.AppsetNamespace)

	appset, err := s.appclientset.ArgoprojV1alpha1().ApplicationSets(namespace).Get(ctx, q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting ApplicationSets: %w", err)
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplicationSets, rbac.ActionDelete, appset.RBACName(s.ns)); err != nil {
		return nil, err
	}

	s.projectLock.RLock(appset.Spec.Template.Spec.Project)
	defer s.projectLock.RUnlock(appset.Spec.Template.Spec.Project)

	err = s.appclientset.ArgoprojV1alpha1().ApplicationSets(namespace).Delete(ctx, q.Name, metav1.DeleteOptions{})
	if err != nil {
		return nil, fmt.Errorf("error deleting ApplicationSets: %w", err)
	}
	s.logAppSetEvent(ctx, appset, argo.EventReasonResourceDeleted, "deleted ApplicationSets")
	return &applicationset.ApplicationSetResponse{}, nil
}

// ResourceTree creates a resource tree for the ApplicationSet
func (s *Server) ResourceTree(ctx context.Context, q *applicationset.ApplicationSetTreeQuery) (*v1alpha1.ApplicationSetTree, error) {
	namespace := s.appsetNamespaceOrDefault(q.AppsetNamespace)

	if !s.isNamespaceEnabled(namespace) {
		return nil, security.NamespaceNotPermittedError(namespace)
	}

	a, err := s.appclientset.ArgoprojV1alpha1().ApplicationSets(namespace).Get(ctx, q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting ApplicationSet: %w", err)
	}
	err = s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplicationSets, rbac.ActionGet, a.RBACName(s.ns))
	if err != nil {
		return nil, err
	}

	return s.buildApplicationSetTree(a)
}

// Generate generates a ApplicationSet
func (s *Server) Generate(ctx context.Context, q *applicationset.ApplicationSetGenerateRequest) (*applicationset.ApplicationSetGenerateResponse, error) {
	appset := q.GetApplicationSet()

	if appset == nil {
		return nil, errors.New("error creating ApplicationSets: ApplicationSets is nil in request")
	}

	// The RBAC check needs to be performed against the appset namespace
	// However, when trying to generate params, the server namespace needs
	// to be passed.
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

	// The server namespace will be used in the function
	// since this is the exact namespace that is being used
	// to generate parameters (especially for git generator).
	//
	// In case of Git generator, if the namespace is set to
	// appset namespace, we'll look for a project in the appset
	// namespace that would lead to error when generating params
	// for an appset in any namespace feature.
	// See https://github.com/argoproj/argo-cd/issues/22942
	apps, err := s.generateApplicationSetApps(ctx, logger.WithField("applicationset", appset.Name), *appset)
	if err != nil {
		return nil, fmt.Errorf("unable to generate Applications of ApplicationSet: %w\n%s", err, logs.String())
	}
	res := &applicationset.ApplicationSetGenerateResponse{}
	for i := range apps {
		res.Applications = append(res.Applications, &apps[i])
	}
	return res, nil
}
