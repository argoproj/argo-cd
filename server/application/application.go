package application

import (
	"bufio"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/controller"
	"github.com/argoproj/argo-cd/controller/services"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/argo"
	argoutil "github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/rbac"
	"github.com/argoproj/argo-cd/util/session"
	"github.com/argoproj/argo-cd/util/settings"
)

// Server provides a Application service
type Server struct {
	ns                  string
	kubeclientset       kubernetes.Interface
	appclientset        appclientset.Interface
	repoClientset       reposerver.Clientset
	controllerClientset controller.Clientset
	kubectl             kube.Kubectl
	db                  db.ArgoDB
	enf                 *rbac.Enforcer
	projectLock         *util.KeyLock
	auditLogger         *argo.AuditLogger
	gitFactory          git.ClientFactory
	settingsMgr         *settings.SettingsManager
}

// NewServer returns a new instance of the Application service
func NewServer(
	namespace string,
	kubeclientset kubernetes.Interface,
	appclientset appclientset.Interface,
	repoClientset reposerver.Clientset,
	controllerClientset controller.Clientset,
	kubectl kube.Kubectl,
	db db.ArgoDB,
	enf *rbac.Enforcer,
	projectLock *util.KeyLock,
	settingsMgr *settings.SettingsManager,
) ApplicationServiceServer {

	return &Server{
		ns:                  namespace,
		appclientset:        appclientset,
		kubeclientset:       kubeclientset,
		controllerClientset: controllerClientset,
		db:                  db,
		repoClientset:       repoClientset,
		kubectl:             kubectl,
		enf:                 enf,
		projectLock:         projectLock,
		auditLogger:         argo.NewAuditLogger(namespace, kubeclientset, "argocd-server"),
		gitFactory:          git.NewFactory(),
		settingsMgr:         settingsMgr,
	}
}

// appRBACName formats fully qualified application name for RBAC check
func appRBACName(app appv1.Application) string {
	return fmt.Sprintf("%s/%s", app.Spec.GetProject(), app.Name)
}

// List returns list of applications
func (s *Server) List(ctx context.Context, q *ApplicationQuery) (*appv1.ApplicationList, error) {
	appList, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	newItems := make([]appv1.Application, 0)
	for _, a := range appList.Items {
		if s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName(a)) {
			newItems = append(newItems, a)
		}
	}
	newItems = argoutil.FilterByProjects(newItems, q.Projects)
	for i := range newItems {
		app := newItems[i]
		newItems[i] = app
	}
	appList.Items = newItems
	return appList, nil
}

// Create creates an application
func (s *Server) Create(ctx context.Context, q *ApplicationCreateRequest) (*appv1.Application, error) {
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionCreate, appRBACName(q.Application)) {
		return nil, grpc.ErrPermissionDenied
	}

	s.projectLock.Lock(q.Application.Spec.Project)
	defer s.projectLock.Unlock(q.Application.Spec.Project)

	a := q.Application
	a.Spec = *argo.NormalizeApplicationSpec(&a.Spec)
	err := s.validateApp(ctx, &a)
	if err != nil {
		return nil, err
	}
	out, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Create(&a)
	if apierr.IsAlreadyExists(err) {
		// act idempotent if existing spec matches new spec
		existing, getErr := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(a.Name, metav1.GetOptions{})
		if getErr != nil {
			return nil, status.Errorf(codes.Internal, "unable to check existing application details: %v", getErr)
		}
		if q.Upsert != nil && *q.Upsert {
			if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionUpdate, appRBACName(a)) {
				return nil, grpc.ErrPermissionDenied
			}
			existing.Spec = a.Spec
			out, err = s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Update(existing)
		} else {
			if !reflect.DeepEqual(existing.Spec, a.Spec) {
				return nil, status.Errorf(codes.InvalidArgument, "existing application spec is different, use upsert flag to force update")
			}
			return existing, nil
		}
	}

	if err == nil {
		s.logEvent(out, ctx, argo.EventReasonResourceCreated, "created application")
	}
	return out, err
}

// GetManifests returns application manifests
func (s *Server) GetManifests(ctx context.Context, q *ApplicationManifestQuery) (*repository.ManifestResponse, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName(*a)) {
		return nil, grpc.ErrPermissionDenied
	}
	repo := s.getRepo(ctx, a.Spec.Source.RepoURL)

	conn, repoClient, err := s.repoClientset.NewRepositoryClient()
	if err != nil {
		return nil, err
	}
	defer util.Close(conn)
	overrides := make([]*appv1.ComponentParameter, len(a.Spec.Source.ComponentParameterOverrides))
	if a.Spec.Source.ComponentParameterOverrides != nil {
		for i := range a.Spec.Source.ComponentParameterOverrides {
			item := a.Spec.Source.ComponentParameterOverrides[i]
			overrides[i] = &item
		}
	}

	revision := a.Spec.Source.TargetRevision
	if q.Revision != "" {
		revision = q.Revision
	}
	settings, err := s.settingsMgr.GetSettings()
	if err != nil {
		return nil, err
	}
	helmRepos, err := s.db.ListHelmRepos(ctx)
	if err != nil {
		return nil, err
	}
	manifestInfo, err := repoClient.GenerateManifest(ctx, &repository.ManifestRequest{
		Repo:                        repo,
		Revision:                    revision,
		ComponentParameterOverrides: overrides,
		AppLabelKey:                 settings.GetAppInstanceLabelKey(),
		AppLabelValue:               a.Name,
		Namespace:                   a.Spec.Destination.Namespace,
		ApplicationSource:           &a.Spec.Source,
		HelmRepos:                   helmRepos,
	})
	if err != nil {
		return nil, err
	}

	return manifestInfo, nil
}

// Get returns an application by name
func (s *Server) Get(ctx context.Context, q *ApplicationQuery) (*appv1.Application, error) {
	appIf := s.appclientset.ArgoprojV1alpha1().Applications(s.ns)
	a, err := appIf.Get(*q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName(*a)) {
		return nil, grpc.ErrPermissionDenied
	}
	if q.Refresh != nil {
		refreshType := appv1.RefreshTypeNormal
		if *q.Refresh == string(appv1.RefreshTypeHard) {
			refreshType = appv1.RefreshTypeHard
		}
		_, err = argoutil.RefreshApp(appIf, *q.Name, refreshType)
		if err != nil {
			return nil, err
		}
		a, err = argoutil.WaitForRefresh(ctx, appIf, *q.Name, nil)
		if err != nil {
			return nil, err
		}
	}
	return a, nil
}

// ListResourceEvents returns a list of event resources
func (s *Server) ListResourceEvents(ctx context.Context, q *ApplicationResourceEventsQuery) (*v1.EventList, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName(*a)) {
		return nil, grpc.ErrPermissionDenied
	}
	var (
		kubeClientset kubernetes.Interface
		fieldSelector string
		namespace     string
	)
	// There are two places where we get events. If we are getting application events, we query
	// our own cluster. If it is events on a resource on an external cluster, then we query the
	// external cluster using its rest.Config
	if q.ResourceName == "" && q.ResourceUID == "" {
		kubeClientset = s.kubeclientset
		namespace = a.Namespace
		fieldSelector = fields.SelectorFromSet(map[string]string{
			"involvedObject.name":      a.Name,
			"involvedObject.uid":       string(a.UID),
			"involvedObject.namespace": a.Namespace,
		}).String()
	} else {
		namespace = q.ResourceNamespace
		var config *rest.Config
		config, _, err = s.getApplicationClusterConfig(*q.Name)
		if err != nil {
			return nil, err
		}
		kubeClientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			return nil, err
		}
		fieldSelector = fields.SelectorFromSet(map[string]string{
			"involvedObject.name":      q.ResourceName,
			"involvedObject.uid":       q.ResourceUID,
			"involvedObject.namespace": namespace,
		}).String()
	}

	log.Infof("Querying for resource events with field selector: %s", fieldSelector)
	opts := metav1.ListOptions{FieldSelector: fieldSelector}
	return kubeClientset.CoreV1().Events(namespace).List(opts)
}

// Update updates an application
func (s *Server) Update(ctx context.Context, q *ApplicationUpdateRequest) (*appv1.Application, error) {
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionUpdate, appRBACName(*q.Application)) {
		return nil, grpc.ErrPermissionDenied
	}

	s.projectLock.Lock(q.Application.Spec.Project)
	defer s.projectLock.Unlock(q.Application.Spec.Project)

	a := q.Application
	a.Spec = *argo.NormalizeApplicationSpec(&a.Spec)
	err := s.validateApp(ctx, a)
	if err != nil {
		return nil, err
	}
	out, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Update(a)
	if err == nil {
		s.logEvent(a, ctx, argo.EventReasonResourceUpdated, "updated application")
	}
	return out, err
}

// UpdateSpec updates an application spec and filters out any invalid parameter overrides
func (s *Server) UpdateSpec(ctx context.Context, q *ApplicationUpdateSpecRequest) (*appv1.ApplicationSpec, error) {
	s.projectLock.Lock(q.Spec.Project)
	defer s.projectLock.Unlock(q.Spec.Project)

	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionUpdate, appRBACName(*a)) {
		return nil, grpc.ErrPermissionDenied
	}
	q.Spec = *argo.NormalizeApplicationSpec(&q.Spec)
	a.Spec = q.Spec
	err = s.validateApp(ctx, a)
	if err != nil {
		return nil, err
	}

	for i := 0; i < 10; i++ {
		a.Spec = q.Spec
		_, err = s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Update(a)
		if err == nil {
			s.logEvent(a, ctx, argo.EventReasonResourceUpdated, "updated application spec")
			return &q.Spec, nil
		}
		if !apierr.IsConflict(err) {
			return nil, err
		}
		a, err = s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}
	return nil, status.Errorf(codes.Internal, "Failed to update application spec. Too many conflicts")
}

// Delete removes an application and all associated resources
func (s *Server) Delete(ctx context.Context, q *ApplicationDeleteRequest) (*ApplicationResponse, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.Name, metav1.GetOptions{})
	if err != nil && !apierr.IsNotFound(err) {
		return nil, err
	}

	s.projectLock.Lock(a.Spec.Project)
	defer s.projectLock.Unlock(a.Spec.Project)

	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionDelete, appRBACName(*a)) {
		return nil, grpc.ErrPermissionDenied
	}

	patchFinalizer := false
	if q.Cascade == nil || *q.Cascade {
		if !a.CascadedDeletion() {
			a.SetCascadedDeletion(true)
			patchFinalizer = true
		}
	} else {
		if a.CascadedDeletion() {
			a.SetCascadedDeletion(false)
			patchFinalizer = true
		}
	}

	if patchFinalizer {
		// Prior to v0.6, the cascaded deletion finalizer was set during app creation.
		// For backward compatibility, we always calculate the patch to see if we need to
		// set/unset the finalizer (in case we are dealing with an app created prior to v0.6)
		patch, err := json.Marshal(map[string]interface{}{
			"metadata": map[string]interface{}{
				"finalizers": a.Finalizers,
			},
		})
		if err != nil {
			return nil, err
		}
		_, err = s.appclientset.ArgoprojV1alpha1().Applications(a.Namespace).Patch(a.Name, types.MergePatchType, patch)
		if err != nil {
			return nil, err
		}
	}

	err = s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Delete(*q.Name, &metav1.DeleteOptions{})
	if err != nil && !apierr.IsNotFound(err) {
		return nil, err
	}

	s.logEvent(a, ctx, argo.EventReasonResourceDeleted, "deleted application")
	return &ApplicationResponse{}, nil
}

func (s *Server) Watch(q *ApplicationQuery, ws ApplicationService_WatchServer) error {
	w, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Watch(metav1.ListOptions{})
	if err != nil {
		return err
	}
	claims := ws.Context().Value("claims")
	done := make(chan bool)
	go func() {
		for next := range w.ResultChan() {
			a := *next.Object.(*appv1.Application)
			if q.Name == nil || *q.Name == "" || *q.Name == a.Name {
				if !s.enf.Enforce(claims, rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName(a)) {
					// do not emit apps user does not have accessing
					continue
				}
				err = ws.Send(&appv1.ApplicationWatchEvent{
					Type:        next.Type,
					Application: a,
				})
				if err != nil {
					log.Warnf("Unable to send stream message: %v", err)
				}
			}
		}
		done <- true
	}()
	select {
	case <-ws.Context().Done():
		w.Stop()
	case <-done:
		w.Stop()
	}
	return nil
}

func (s *Server) validateApp(ctx context.Context, app *appv1.Application) error {
	proj, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(app.Spec.GetProject(), metav1.GetOptions{})
	if err != nil {
		if apierr.IsNotFound(err) {
			return status.Errorf(codes.InvalidArgument, "application references project %s which does not exist", app.Spec.Project)
		}
		return err
	}
	currApp, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(app.Name, metav1.GetOptions{})
	if err != nil {
		if !apierr.IsNotFound(err) {
			return err
		}
		// Kubernetes go-client will return a pointer to a zero-value app instead of nil, even
		// though the API response was NotFound. This behavior was confirmed via logs.
		currApp = nil
	}
	if currApp != nil && currApp.Spec.GetProject() != app.Spec.GetProject() {
		// When changing projects, caller must have application create & update privileges in new project
		// NOTE: the update check was already verified in the caller to this function
		if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionCreate, appRBACName(*app)) {
			return grpc.ErrPermissionDenied
		}
		// They also need 'update' privileges in the old project
		if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionUpdate, appRBACName(*currApp)) {
			return grpc.ErrPermissionDenied
		}
	}
	conditions, err := argo.GetSpecErrors(ctx, &app.Spec, proj, s.repoClientset, s.db)
	if err != nil {
		return err
	}
	if len(conditions) > 0 {
		return status.Errorf(codes.InvalidArgument, "application spec is invalid: %s", argo.FormatAppConditions(conditions))
	}
	return nil
}

func (s *Server) getApplicationClusterConfig(applicationName string) (*rest.Config, string, error) {
	server, namespace, err := s.getApplicationDestination(context.Background(), applicationName)
	if err != nil {
		return nil, "", err
	}
	clst, err := s.db.GetCluster(context.Background(), server)
	if err != nil {
		return nil, "", err
	}
	config := clst.RESTConfig()
	return config, namespace, err
}

func (s *Server) getAppResources(ctx context.Context, q *services.ResourcesQuery) (*services.ResourceTreeResponse, error) {
	closer, client, err := s.controllerClientset.NewApplicationServiceClient()
	if err != nil {
		return nil, err
	}
	defer util.Close(closer)
	return client.ResourceTree(ctx, q)
}

func (s *Server) getAppResource(ctx context.Context, action string, q *ApplicationResourceRequest) (*appv1.ResourceNode, *rest.Config, *appv1.Application, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, nil, err
	}
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplications, action, appRBACName(*a)) {
		return nil, nil, nil, grpc.ErrPermissionDenied
	}

	resources, err := s.getAppResources(ctx, &services.ResourcesQuery{ApplicationName: a.Name})
	if err != nil {
		return nil, nil, nil, err
	}

	found := findResource(resources.Items, q)
	if found == nil {
		return nil, nil, nil, status.Errorf(codes.InvalidArgument, "%s %s %s not found as part of application %s", q.Kind, q.Group, q.ResourceName, *q.Name)
	}
	config, _, err := s.getApplicationClusterConfig(*q.Name)
	if err != nil {
		return nil, nil, nil, err
	}
	return found, config, a, nil
}

func (s *Server) GetResource(ctx context.Context, q *ApplicationResourceRequest) (*ApplicationResourceResponse, error) {
	res, config, _, err := s.getAppResource(ctx, rbacpolicy.ActionGet, q)
	if err != nil {
		return nil, err
	}
	obj, err := s.kubectl.GetResource(config, res.GroupKindVersion(), res.Name, res.Namespace)
	if err != nil {
		return nil, err
	}
	err = replaceSecretValues(obj)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(obj.Object)
	if err != nil {
		return nil, err
	}
	return &ApplicationResourceResponse{Manifest: string(data)}, nil
}

func replaceSecretValues(obj *unstructured.Unstructured) error {
	if obj.GetKind() == kube.SecretKind && obj.GroupVersionKind().Group == "" {
		data, _, _ := unstructured.NestedMap(obj.Object, "data")
		if data != nil {
			for k := range data {
				data[k] = "**********"
			}
			err := unstructured.SetNestedField(obj.Object, data, "data")
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// PatchResource patches a resource
func (s *Server) PatchResource(ctx context.Context, q *ApplicationResourcePatchRequest) (*ApplicationResourceResponse, error) {
	resourceRequest := &ApplicationResourceRequest{
		Name:         q.Name,
		Namespace:    q.Namespace,
		ResourceName: q.ResourceName,
		Kind:         q.Kind,
		Version:      q.Version,
		Group:        q.Group,
	}
	res, config, a, err := s.getAppResource(ctx, rbacpolicy.ActionUpdate, resourceRequest)
	if err != nil {
		return nil, err
	}
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionUpdate, appRBACName(*a)) {
		return nil, grpc.ErrPermissionDenied
	}

	manifest, err := s.kubectl.PatchResource(config, res.GroupKindVersion(), res.Name, res.Namespace, types.PatchType(q.PatchType), []byte(q.Patch))
	if err != nil {
		return nil, err
	}
	err = replaceSecretValues(manifest)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(manifest.Object)
	if err != nil {
		return nil, err
	}
	return &ApplicationResourceResponse{
		Manifest: string(data),
	}, nil
}

// DeleteResource deletes a specificed resource
func (s *Server) DeleteResource(ctx context.Context, q *ApplicationResourceDeleteRequest) (*ApplicationResponse, error) {
	resourceRequest := &ApplicationResourceRequest{
		Name:         q.Name,
		Namespace:    q.Namespace,
		ResourceName: q.ResourceName,
		Kind:         q.Kind,
		Version:      q.Version,
		Group:        q.Group,
	}
	res, config, a, err := s.getAppResource(ctx, rbacpolicy.ActionDelete, resourceRequest)
	if err != nil {
		return nil, err
	}

	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionDelete, appRBACName(*a)) {
		return nil, grpc.ErrPermissionDenied
	}
	var force bool
	if q.Force != nil {
		force = *q.Force
	}
	err = s.kubectl.DeleteResource(config, res.GroupKindVersion(), res.Name, res.Namespace, force)
	if err != nil {
		return nil, err
	}
	s.logEvent(a, ctx, argo.EventReasonResourceDeleted, fmt.Sprintf("deleted resource %s/%s '%s'", q.Group, q.Kind, q.ResourceName))
	return &ApplicationResponse{}, nil
}

func (s *Server) ResourceTree(ctx context.Context, q *services.ResourcesQuery) (*services.ResourceTreeResponse, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(q.ApplicationName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName(*a)) {
		return nil, grpc.ErrPermissionDenied
	}
	return s.getAppResources(ctx, q)
}

func (s *Server) ManagedResources(ctx context.Context, q *services.ResourcesQuery) (*services.ManagedResourcesResponse, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(q.ApplicationName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName(*a)) {
		return nil, grpc.ErrPermissionDenied
	}
	closer, client, err := s.controllerClientset.NewApplicationServiceClient()
	if err != nil {
		return nil, err
	}
	defer util.Close(closer)
	return client.ManagedResources(ctx, &services.ResourcesQuery{ApplicationName: a.Name})
}

func findResource(resources []*appv1.ResourceNode, q *ApplicationResourceRequest) *appv1.ResourceNode {
	for i := range resources {
		node := resources[i].FindNode(q.Group, q.Kind, q.Namespace, q.ResourceName)
		if node != nil {
			return node
		}
	}
	return nil
}

func (s *Server) PodLogs(q *ApplicationPodLogsQuery, ws ApplicationService_PodLogsServer) error {
	pod, config, _, err := s.getAppResource(ws.Context(), rbacpolicy.ActionGet, &ApplicationResourceRequest{
		Name:         q.Name,
		Namespace:    q.Namespace,
		Kind:         kube.PodKind,
		Group:        "",
		Version:      "v1",
		ResourceName: *q.PodName,
	})

	if err != nil {
		return err
	}

	kubeClientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	var sinceSeconds, tailLines *int64
	if q.SinceSeconds > 0 {
		sinceSeconds = &q.SinceSeconds
	}
	if q.TailLines > 0 {
		tailLines = &q.TailLines
	}
	stream, err := kubeClientset.CoreV1().Pods(pod.Namespace).GetLogs(*q.PodName, &v1.PodLogOptions{
		Container:    q.Container,
		Follow:       q.Follow,
		Timestamps:   true,
		SinceSeconds: sinceSeconds,
		SinceTime:    q.SinceTime,
		TailLines:    tailLines,
	}).Stream()
	if err != nil {
		return err
	}
	done := make(chan bool)
	go func() {
		scanner := bufio.NewScanner(stream)
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.Split(line, " ")
			logTime, err := time.Parse(time.RFC3339, parts[0])
			metaLogTime := metav1.NewTime(logTime)
			if err == nil {
				lines := strings.Join(parts[1:], " ")
				for _, line := range strings.Split(lines, "\r") {
					if line != "" {
						err = ws.Send(&LogEntry{
							Content:   line,
							TimeStamp: metaLogTime,
						})
						if err != nil {
							log.Warnf("Unable to send stream message: %v", err)
						}
					}
				}
			}
		}

		done <- true
	}()
	select {
	case <-ws.Context().Done():
		util.Close(stream)
	case <-done:
	}
	return nil
}

func (s *Server) getApplicationDestination(ctx context.Context, name string) (string, string, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(name, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}
	server, namespace := a.Spec.Destination.Server, a.Spec.Destination.Namespace
	return server, namespace, nil
}

func (s *Server) getRepo(ctx context.Context, repoURL string) *appv1.Repository {
	repo, err := s.db.GetRepository(ctx, repoURL)
	if err != nil {
		// If we couldn't retrieve from the repo service, assume public repositories
		repo = &appv1.Repository{Repo: repoURL}
	}
	return repo
}

// Sync syncs an application to its target state
func (s *Server) Sync(ctx context.Context, syncReq *ApplicationSyncRequest) (*appv1.Application, error) {
	appIf := s.appclientset.ArgoprojV1alpha1().Applications(s.ns)
	a, err := appIf.Get(*syncReq.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionSync, appRBACName(*a)) {
		return nil, grpc.ErrPermissionDenied
	}
	if a.DeletionTimestamp != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "application is deleting")
	}
	if a.Spec.SyncPolicy != nil && a.Spec.SyncPolicy.Automated != nil {
		if syncReq.Revision != "" && syncReq.Revision != a.Spec.Source.TargetRevision {
			return nil, status.Errorf(codes.FailedPrecondition, "Cannot sync to %s: auto-sync currently set to %s", syncReq.Revision, a.Spec.Source.TargetRevision)
		}
	}

	parameterOverrides := make(appv1.ParameterOverrides, 0)
	if syncReq.Parameter != nil {
		// If parameter overrides are supplied, the caller explicitly states to use the provided
		// list of overrides. NOTE: gogo/protobuf cannot currently distinguish between empty arrays
		// vs nil arrays, which is why the wrapping syncReq.Parameter is examined for intent.
		// See: https://github.com/gogo/protobuf/issues/181
		for _, p := range syncReq.Parameter.Overrides {
			parameterOverrides = append(parameterOverrides, appv1.ComponentParameter{
				Name:      p.Name,
				Value:     p.Value,
				Component: p.Component,
			})
		}
	} else {
		// If parameter overrides are omitted completely, we use what is set in the application
		if a.Spec.Source.ComponentParameterOverrides != nil {
			parameterOverrides = appv1.ParameterOverrides(a.Spec.Source.ComponentParameterOverrides)
		}
	}

	commitSHA, displayRevision, err := s.resolveRevision(ctx, a, syncReq)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, err.Error())
	}

	op := appv1.Operation{
		Sync: &appv1.SyncOperation{
			Revision:           commitSHA,
			Prune:              syncReq.Prune,
			DryRun:             syncReq.DryRun,
			SyncStrategy:       syncReq.Strategy,
			ParameterOverrides: parameterOverrides,
			Resources:          syncReq.Resources,
		},
	}
	a, err = argo.SetAppOperation(appIf, *syncReq.Name, &op)
	if err == nil {
		partial := ""
		if len(syncReq.Resources) > 0 {
			partial = "partial "
		}
		s.logEvent(a, ctx, argo.EventReasonOperationStarted, fmt.Sprintf("initiated %ssync to %s", partial, displayRevision))
	}
	return a, err
}

func (s *Server) Rollback(ctx context.Context, rollbackReq *ApplicationRollbackRequest) (*appv1.Application, error) {
	appIf := s.appclientset.ArgoprojV1alpha1().Applications(s.ns)
	a, err := appIf.Get(*rollbackReq.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionSync, appRBACName(*a)) {
		return nil, grpc.ErrPermissionDenied
	}
	if a.DeletionTimestamp != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "application is deleting")
	}
	if a.Spec.SyncPolicy != nil && a.Spec.SyncPolicy.Automated != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "rollback cannot be initiated when auto-sync is enabled")
	}

	var deploymentInfo *appv1.RevisionHistory
	for _, info := range a.Status.History {
		if info.ID == rollbackReq.ID {
			deploymentInfo = &info
			break
		}
	}
	if deploymentInfo == nil {
		return nil, fmt.Errorf("application %s does not have deployment with id %v", a.Name, rollbackReq.ID)
	}
	overrides := deploymentInfo.ComponentParameterOverrides
	// Nil overrides in deployment history means no overrides, so sync operation request should contains empty overrides set
	if overrides == nil {
		overrides = make([]appv1.ComponentParameter, 0)
	}
	// Rollback is just a convenience around Sync
	op := appv1.Operation{
		Sync: &appv1.SyncOperation{
			Revision:           deploymentInfo.Revision,
			DryRun:             rollbackReq.DryRun,
			Prune:              rollbackReq.Prune,
			SyncStrategy:       &appv1.SyncStrategy{Apply: &appv1.SyncStrategyApply{}},
			ParameterOverrides: overrides,
		},
	}
	a, err = argo.SetAppOperation(appIf, *rollbackReq.Name, &op)
	if err == nil {
		s.logEvent(a, ctx, argo.EventReasonOperationStarted, fmt.Sprintf("initiated rollback to %d", rollbackReq.ID))
	}
	return a, err
}

// resolveRevision resolves the git revision specified either in the sync request, or the
// application source, into a concrete commit SHA that will be used for a sync operation.
func (s *Server) resolveRevision(ctx context.Context, app *appv1.Application, syncReq *ApplicationSyncRequest) (string, string, error) {
	ambiguousRevision := syncReq.Revision
	if ambiguousRevision == "" {
		ambiguousRevision = app.Spec.Source.TargetRevision
	}
	if git.IsCommitSHA(ambiguousRevision) {
		// If it's already a commit SHA, then no need to look it up
		return ambiguousRevision, ambiguousRevision, nil
	}
	repo, err := s.db.GetRepository(ctx, app.Spec.Source.RepoURL)
	if err != nil {
		// If we couldn't retrieve from the repo service, assume public repositories
		repo = &appv1.Repository{Repo: app.Spec.Source.RepoURL}
	}
	gitClient, err := s.gitFactory.NewClient(repo.Repo, "", repo.Username, repo.Password, repo.SSHPrivateKey)
	if err != nil {
		return "", "", err
	}
	commitSHA, err := gitClient.LsRemote(ambiguousRevision)
	if err != nil {
		return "", "", err
	}
	displayRevision := fmt.Sprintf("%s (%s)", ambiguousRevision, commitSHA)
	return commitSHA, displayRevision, nil
}

func (s *Server) TerminateOperation(ctx context.Context, termOpReq *OperationTerminateRequest) (*OperationTerminateResponse, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*termOpReq.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionSync, appRBACName(*a)) {
		return nil, grpc.ErrPermissionDenied
	}

	for i := 0; i < 10; i++ {
		if a.Operation == nil || a.Status.OperationState == nil {
			return nil, status.Errorf(codes.InvalidArgument, "Unable to terminate operation. No operation is in progress")
		}
		a.Status.OperationState.Phase = appv1.OperationTerminating
		_, err = s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Update(a)
		if err == nil {
			return &OperationTerminateResponse{}, nil
		}
		if !apierr.IsConflict(err) {
			return nil, err
		}
		log.Warnf("Failed to set operation for app '%s' due to update conflict. Retrying again...", *termOpReq.Name)
		time.Sleep(100 * time.Millisecond)
		a, err = s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*termOpReq.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		s.logEvent(a, ctx, argo.EventReasonResourceUpdated, "terminated running operation")
	}
	return nil, status.Errorf(codes.Internal, "Failed to terminate app. Too many conflicts")
}

func (s *Server) logEvent(a *appv1.Application, ctx context.Context, reason string, action string) {
	eventInfo := argo.EventInfo{Type: v1.EventTypeNormal, Reason: reason}
	user := session.Username(ctx)
	if user == "" {
		user = "Unknown user"
	}
	message := fmt.Sprintf("%s %s", user, action)
	s.auditLogger.LogAppEvent(a, eventInfo, message)
}
