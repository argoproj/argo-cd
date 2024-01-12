package applicationset

import (
	"context"
	"fmt"
	"math"
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
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	appsetutils "github.com/argoproj/argo-cd/v2/applicationset/utils"
	argocommon "github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/applicationset"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/collections"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/rbac"
	"github.com/argoproj/argo-cd/v2/util/security"
	"github.com/argoproj/argo-cd/v2/util/session"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

var (
	watchAPIBufferSize = env.ParseNumFromEnv(argocommon.EnvWatchAPIBufferSize, 1000, 0, math.MaxInt32)
)

type Server struct {
	ns                string
	db                db.ArgoDB
	enf               *rbac.Enforcer
	appclientset      appclientset.Interface
	appsetInformer    cache.SharedIndexInformer
	appsetBroadcaster Broadcaster
	appsetLister      applisters.ApplicationSetLister
	projLister        applisters.AppProjectNamespaceLister
	auditLogger       *argo.AuditLogger
	settings          *settings.SettingsManager
	projectLock       sync.KeyLock
	enabledNamespaces []string
}

// NewServer returns a new instance of the ApplicationSet service
func NewServer(
	db db.ArgoDB,
	kubeclientset kubernetes.Interface,
	enf *rbac.Enforcer,
	appclientset appclientset.Interface,
	appsetInformer cache.SharedIndexInformer,
	appsetLister applisters.ApplicationSetLister,
	projLister applisters.AppProjectNamespaceLister,
	settings *settings.SettingsManager,
	namespace string,
	projectLock sync.KeyLock,
	enabledNamespaces []string,
) applicationset.ApplicationSetServiceServer {
	appsetBroadcaster := &broadcasterHandler{}
	_, err := appsetInformer.AddEventHandler(appsetBroadcaster)
	if err != nil {
		log.Error(err)
	}
	s := &Server{
		ns:                namespace,
		db:                db,
		enf:               enf,
		appclientset:      appclientset,
		appsetInformer:    appsetInformer,
        appsetBroadcaster: appsetBroadcaster,
		appsetLister:      appsetLister,
		projLister:        projLister,
		settings:          settings,
		projectLock:       projectLock,
		auditLogger:       argo.NewAuditLogger(namespace, kubeclientset, "argocd-server"),
		enabledNamespaces: enabledNamespaces,
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

func (s *Server) Watch(q *applicationset.ApplicationSetWatchQuery, ws applicationset.ApplicationSetService_WatchServer) error {
    ctx := ws.Context()
	namespace := s.appsetNamespaceOrDefault(q.AppsetNamespace)

	if !s.isNamespaceEnabled(namespace) {
		return security.NamespaceNotPermittedError(namespace)
	}

	projects := map[string]bool{}
	for _, project := range q.GetProjects() {
		projects[project] = true
	}

	claims := ws.Context().Value("claims")
	selector, err := labels.Parse(q.GetSelector())
	if err != nil {
		return fmt.Errorf("error parsing the selector: %w", err)
	}

	logCtx := log.NewEntry(log.New())
	if q.Name != "" {
		logCtx = logCtx.WithField("application", q.GetName())
	}

	minVersion := 0
	if q.GetResourceVersion() != "" {
		if minVersion, err = strconv.Atoi(q.GetResourceVersion()); err != nil {
			minVersion = 0
		}
	}

	// sendIfPermitted is a helper to send the applicationset to the client's streaming channel if the
	// caller has RBAC privileges permissions to view it
	sendIfPermitted := func(ctx context.Context, a v1alpha1.ApplicationSet, eventType watch.EventType) {
		if !s.isApplicationSetPermitted(selector, minVersion, claims, q.GetName(), namespace, projects, a) {
			return
		}

		err := ws.Send(&v1alpha1.ApplicationSetWatchEvent{
			Type:           eventType,
			ApplicationSet: a,
		})
		if err != nil {
			logCtx.Warnf("Unable to send stream message: %v", err)
			return
		}
	}

	events := make(chan *v1alpha1.ApplicationSetWatchEvent, watchAPIBufferSize)
	// Mimic watch API behavior: send ADDED events if no resource version provided
	// If watch API is executed for one application when emit event even if resource version is provided
	// This is required since single app watch API is used for during operations like app syncing and it is
	// critical to never miss events.
	if q.GetResourceVersion() == "" || q.GetName() != "" {
		appsets, err := s.appsetLister.List(selector)
		if err != nil {
			return fmt.Errorf("error listing appssets with selector: %w", err)
		}
		sort.Slice(appsets, func(i, j int) bool {
			return appsets[i].QualifiedName() < appsets[j].QualifiedName()
		})
		for i := range appsets {
			sendIfPermitted(ctx, *appsets[i], watch.Added)
		}
	}
	unsubscribe := s.appsetBroadcaster.Subscribe(events)
	defer unsubscribe()
	for {
		select {
		case event := <-events:
			sendIfPermitted(ctx, event.ApplicationSet, event.Type)
		case <-ws.Context().Done():
			return nil
		}
	}
}

func (s *Server) isApplicationSetPermitted(selector labels.Selector, minVersion int, claims any, appsetName, appsetNs string, projects map[string]bool, a v1alpha1.ApplicationSet) bool {
	if len(projects) > 0 && !projects[a.Spec.Template.Spec.GetProject()] {
		return false
	}

	if appVersion, err := strconv.Atoi(a.ResourceVersion); err == nil && appVersion < minVersion {
		return false
	}
	matchedEvent := (appsetName == "" || (a.Name == appsetName && a.Namespace == appsetNs)) && selector.Matches(labels.Set(a.Labels))
	if !matchedEvent {
		return false
	}

	if !s.isNamespaceEnabled(a.Namespace) {
		return false
	}

	if !s.enf.Enforce(claims, rbacpolicy.ResourceApplicationSets, rbacpolicy.ActionGet, a.RBACName(s.ns)) {
		// do not emit appsets user does not have accessing
		return false
	}

	return true
}

func (s *Server) Create(ctx context.Context, q *applicationset.ApplicationSetCreateRequest) (*v1alpha1.ApplicationSet, error) {
	appset := q.GetApplicationset()

	if appset == nil {
		return nil, fmt.Errorf("error creating ApplicationSets: ApplicationSets is nil in request")
	}

	projectName, err := s.validateAppSet(ctx, appset)
	if err != nil {
		return nil, fmt.Errorf("error validating ApplicationSets: %w", err)
	}

	namespace := s.appsetNamespaceOrDefault(appset.Namespace)

	if !s.isNamespaceEnabled(namespace) {
		return nil, security.NamespaceNotPermittedError(namespace)
	}

	if err := s.checkCreatePermissions(ctx, appset, projectName); err != nil {
		return nil, fmt.Errorf("error checking create permissions for ApplicationSets %s : %s", appset.Name, err)
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

func (s *Server) validateAppSet(ctx context.Context, appset *v1alpha1.ApplicationSet) (string, error) {
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
