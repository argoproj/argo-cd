package application

import (
	"bufio"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
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
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/argo"
	argoutil "github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/diff"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/lua"
	"github.com/argoproj/argo-cd/util/rbac"
	"github.com/argoproj/argo-cd/util/session"
	"github.com/argoproj/argo-cd/util/settings"
)

// Server provides a Application service
type Server struct {
	ns            string
	kubeclientset kubernetes.Interface
	appclientset  appclientset.Interface
	repoClientset apiclient.Clientset
	kubectl       kube.Kubectl
	db            db.ArgoDB
	enf           *rbac.Enforcer
	projectLock   *util.KeyLock
	auditLogger   *argo.AuditLogger
	gitFactory    git.ClientFactory
	settingsMgr   *settings.SettingsManager
	cache         *cache.Cache
}

// NewServer returns a new instance of the Application service
func NewServer(
	namespace string,
	kubeclientset kubernetes.Interface,
	appclientset appclientset.Interface,
	repoClientset apiclient.Clientset,
	cache *cache.Cache,
	kubectl kube.Kubectl,
	db db.ArgoDB,
	enf *rbac.Enforcer,
	projectLock *util.KeyLock,
	settingsMgr *settings.SettingsManager,
) application.ApplicationServiceServer {

	return &Server{
		ns:            namespace,
		appclientset:  appclientset,
		kubeclientset: kubeclientset,
		cache:         cache,
		db:            db,
		repoClientset: repoClientset,
		kubectl:       kubectl,
		enf:           enf,
		projectLock:   projectLock,
		auditLogger:   argo.NewAuditLogger(namespace, kubeclientset, "argocd-server"),
		gitFactory:    git.NewFactory(),
		settingsMgr:   settingsMgr,
	}
}

// appRBACName formats fully qualified application name for RBAC check
func appRBACName(app appv1.Application) string {
	return fmt.Sprintf("%s/%s", app.Spec.GetProject(), app.Name)
}

// List returns list of applications
func (s *Server) List(ctx context.Context, q *application.ApplicationQuery) (*appv1.ApplicationList, error) {
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
func (s *Server) Create(ctx context.Context, q *application.ApplicationCreateRequest) (*appv1.Application, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionCreate, appRBACName(q.Application)); err != nil {
		return nil, err
	}

	s.projectLock.Lock(q.Application.Spec.Project)
	defer s.projectLock.Unlock(q.Application.Spec.Project)

	a := q.Application
	err := s.validateAndNormalizeApp(ctx, &a)
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
			if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionUpdate, appRBACName(a)); err != nil {
				return nil, err
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
func (s *Server) GetManifests(ctx context.Context, q *application.ApplicationManifestQuery) (*apiclient.ManifestResponse, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName(*a)); err != nil {
		return nil, err
	}
	repo, err := s.db.GetRepository(ctx, a.Spec.Source.RepoURL)
	if err != nil {
		return nil, err
	}
	conn, repoClient, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, err
	}
	defer util.Close(conn)
	revision := a.Spec.Source.TargetRevision
	if q.Revision != "" {
		revision = q.Revision
	}
	appInstanceLabelKey, err := s.settingsMgr.GetAppInstanceLabelKey()
	if err != nil {
		return nil, err
	}
	helmRepos, err := s.db.ListHelmRepos(ctx)
	if err != nil {
		return nil, err
	}

	plugins, err := s.settingsMgr.GetConfigManagementPlugins()
	if err != nil {
		return nil, err
	}

	tools := make([]*appv1.ConfigManagementPlugin, len(plugins))
	for i := range plugins {
		tools[i] = &plugins[i]
	}
	// If source is Kustomize add build options
	buildOptions, err := s.settingsMgr.GetKustomizeBuildOptions()
	if err != nil {
		return nil, err
	}
	kustomizeOptions := appv1.KustomizeOptions{
		BuildOptions: buildOptions,
	}

	manifestInfo, err := repoClient.GenerateManifest(ctx, &apiclient.ManifestRequest{
		Repo:              repo,
		Revision:          revision,
		AppLabelKey:       appInstanceLabelKey,
		AppLabelValue:     a.Name,
		Namespace:         a.Spec.Destination.Namespace,
		ApplicationSource: &a.Spec.Source,
		HelmRepos:         helmRepos,
		Plugins:           tools,
		KustomizeOptions:  &kustomizeOptions,
	})
	if err != nil {
		return nil, err
	}

	return manifestInfo, nil
}

// Get returns an application by name
func (s *Server) Get(ctx context.Context, q *application.ApplicationQuery) (*appv1.Application, error) {
	appIf := s.appclientset.ArgoprojV1alpha1().Applications(s.ns)
	a, err := appIf.Get(*q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName(*a)); err != nil {
		return nil, err
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
func (s *Server) ListResourceEvents(ctx context.Context, q *application.ApplicationResourceEventsQuery) (*v1.EventList, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName(*a)); err != nil {
		return nil, err
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
func (s *Server) Update(ctx context.Context, q *application.ApplicationUpdateRequest) (*appv1.Application, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionUpdate, appRBACName(*q.Application)); err != nil {
		return nil, err
	}

	s.projectLock.Lock(q.Application.Spec.Project)
	defer s.projectLock.Unlock(q.Application.Spec.Project)

	a := q.Application
	err := s.validateAndNormalizeApp(ctx, a)
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
func (s *Server) UpdateSpec(ctx context.Context, q *application.ApplicationUpdateSpecRequest) (*appv1.ApplicationSpec, error) {
	s.projectLock.Lock(q.Spec.Project)
	defer s.projectLock.Unlock(q.Spec.Project)

	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionUpdate, appRBACName(*a)); err != nil {
		return nil, err
	}
	a.Spec = q.Spec
	err = s.validateAndNormalizeApp(ctx, a)
	if err != nil {
		return nil, err
	}
	normalizedSpec := a.Spec.DeepCopy()

	for i := 0; i < 10; i++ {
		a.Spec = *normalizedSpec
		_, err = s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Update(a)
		if err == nil {
			s.logEvent(a, ctx, argo.EventReasonResourceUpdated, "updated application spec")
			return normalizedSpec, nil
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

// Patch patches an application
func (s *Server) Patch(ctx context.Context, q *application.ApplicationPatchRequest) (*appv1.Application, error) {

	app, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if err = s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionOverride, appRBACName(*app)); err != nil {
		return nil, err
	}

	jsonApp, err := json.Marshal(app)
	if err != nil {
		return nil, err
	}

	var patchApp []byte

	switch q.PatchType {
	case "json", "":
		patch, err := jsonpatch.DecodePatch([]byte(q.Patch))
		if err != nil {
			return nil, err
		}
		patchApp, err = patch.Apply(jsonApp)
		if err != nil {
			return nil, err
		}
	case "merge":
		patchApp, err = jsonpatch.MergePatch(jsonApp, []byte(q.Patch))
		if err != nil {
			return nil, err
		}
	default:
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Patch type '%s' is not supported", q.PatchType))
	}

	s.logEvent(app, ctx, argo.EventReasonResourceUpdated, fmt.Sprintf("patched application %s/%s", app.Namespace, app.Name))

	err = json.Unmarshal(patchApp, &app)
	if err != nil {
		return nil, err
	}

	err = s.validateAndNormalizeApp(ctx, app)
	if err != nil {
		return nil, err
	}

	return s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Update(app)
}

// Delete removes an application and all associated resources
func (s *Server) Delete(ctx context.Context, q *application.ApplicationDeleteRequest) (*application.ApplicationResponse, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.Name, metav1.GetOptions{})
	if err != nil && !apierr.IsNotFound(err) {
		return nil, err
	}

	s.projectLock.Lock(a.Spec.Project)
	defer s.projectLock.Unlock(a.Spec.Project)

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionDelete, appRBACName(*a)); err != nil {
		return nil, err
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
		// Although the cascaded deletion finalizer is not set when apps are created via API,
		// they will often be set by the user as part of declarative config. As part of a delete
		// request, we always calculate the patch to see if we need to set/unset the finalizer.
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
	return &application.ApplicationResponse{}, nil
}

func (s *Server) Watch(q *application.ApplicationQuery, ws application.ApplicationService_WatchServer) error {
	logCtx := log.NewEntry(log.New())
	if q.Name != nil {
		logCtx = logCtx.WithField("application", *q.Name)
	}
	claims := ws.Context().Value("claims")
	// sendIfPermitted is a helper to send the application to the client's streaming channel if the
	// caller has RBAC privileges permissions to view it
	sendIfPermitted := func(a appv1.Application, eventType watch.EventType) error {
		if !s.enf.Enforce(claims, rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName(a)) {
			// do not emit apps user does not have accessing
			return nil
		}
		err := ws.Send(&appv1.ApplicationWatchEvent{
			Type:        eventType,
			Application: a,
		})
		if err != nil {
			logCtx.Warnf("Unable to send stream message: %v", err)
			return err
		}
		return nil
	}

	var listOpts metav1.ListOptions
	if q.Name != nil && *q.Name != "" {
		listOpts.FieldSelector = fmt.Sprintf("metadata.name=%s", *q.Name)
	}
	listOpts.ResourceVersion = q.ResourceVersion
	if listOpts.ResourceVersion == "" {
		// If resourceVersion is not supplied, we need to get latest version of the apps by first
		// making a list request, which we then supply to the watch request. We always need to
		// supply a resourceVersion to watch requests since without it, the return values may return
		// stale data. See: https://github.com/argoproj/argo-cd/issues/1605
		appsList, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).List(listOpts)
		if err != nil {
			return err
		}
		for _, a := range appsList.Items {
			err = sendIfPermitted(a, watch.Modified)
			if err != nil {
				return err
			}
		}
		listOpts.ResourceVersion = appsList.ResourceVersion
	}

	w, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Watch(listOpts)
	if err != nil {
		return err
	}
	defer w.Stop()
	done := make(chan bool)
	go func() {
		for next := range w.ResultChan() {
			a, ok := next.Object.(*appv1.Application)
			if ok {
				_ = sendIfPermitted(*a, next.Type)
			} else {
				break
			}
		}
		logCtx.Info("k8s application watch event channel closed")
		close(done)
	}()
	select {
	case <-ws.Context().Done():
		logCtx.Info("client watch grpc context closed")
	case <-done:
	}
	return nil
}

func (s *Server) validateAndNormalizeApp(ctx context.Context, app *appv1.Application) error {
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
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionCreate, appRBACName(*app)); err != nil {
			return err
		}
		// They also need 'update' privileges in the old project
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionUpdate, appRBACName(*currApp)); err != nil {
			return err
		}
	}

	buildOptions, err := s.settingsMgr.GetKustomizeBuildOptions()
	if err != nil {
		return err
	}
	kustomizeOptions := appv1.KustomizeOptions{
		BuildOptions: buildOptions,
	}
	conditions, appSourceType, err := argo.ValidateRepo(ctx, &app.Spec, s.repoClientset, s.db, &kustomizeOptions)
	if err != nil {
		return err
	}
	if len(conditions) > 0 {
		return status.Errorf(codes.InvalidArgument, "application spec is invalid: %s", argo.FormatAppConditions(conditions))
	}

	conditions, err = argo.ValidatePermissions(ctx, &app.Spec, proj, s.db)
	if err != nil {
		return err
	}
	if len(conditions) > 0 {
		return status.Errorf(codes.InvalidArgument, "application spec is invalid: %s", argo.FormatAppConditions(conditions))
	}

	app.Spec = *argo.NormalizeApplicationSpec(&app.Spec, appSourceType)
	return nil
}

func (s *Server) getApplicationClusterConfig(applicationName string) (*rest.Config, string, error) {
	server, namespace, err := s.getApplicationDestination(applicationName)
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

func (s *Server) getAppResources(q *application.ResourcesQuery) (*appv1.ApplicationTree, error) {
	return s.cache.GetAppResourcesTree(*q.ApplicationName)
}

func (s *Server) getAppResource(ctx context.Context, action string, q *application.ApplicationResourceRequest) (*appv1.ResourceNode, *rest.Config, *appv1.Application, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, nil, err
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, action, appRBACName(*a)); err != nil {
		return nil, nil, nil, err
	}

	tree, err := s.getAppResources(&application.ResourcesQuery{ApplicationName: &a.Name})
	if err != nil {
		return nil, nil, nil, err
	}

	found := tree.FindNode(q.Group, q.Kind, q.Namespace, q.ResourceName)
	if found == nil {
		return nil, nil, nil, status.Errorf(codes.InvalidArgument, "%s %s %s not found as part of application %s", q.Kind, q.Group, q.ResourceName, *q.Name)
	}
	config, _, err := s.getApplicationClusterConfig(*q.Name)
	if err != nil {
		return nil, nil, nil, err
	}
	return found, config, a, nil
}

func (s *Server) GetResource(ctx context.Context, q *application.ApplicationResourceRequest) (*application.ApplicationResourceResponse, error) {
	res, config, _, err := s.getAppResource(ctx, rbacpolicy.ActionGet, q)
	if err != nil {
		return nil, err
	}
	obj, err := s.kubectl.GetResource(config, res.GroupKindVersion(), res.Name, res.Namespace)
	if err != nil {
		return nil, err
	}
	obj, err = replaceSecretValues(obj)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(obj.Object)
	if err != nil {
		return nil, err
	}
	return &application.ApplicationResourceResponse{Manifest: string(data)}, nil
}

func replaceSecretValues(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if obj.GetKind() == kube.SecretKind && obj.GroupVersionKind().Group == "" {
		_, obj, err := diff.HideSecretData(nil, obj)
		if err != nil {
			return nil, err
		}
		return obj, err
	}
	return obj, nil
}

// PatchResource patches a resource
func (s *Server) PatchResource(ctx context.Context, q *application.ApplicationResourcePatchRequest) (*application.ApplicationResourceResponse, error) {
	resourceRequest := &application.ApplicationResourceRequest{
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
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionUpdate, appRBACName(*a)); err != nil {
		return nil, err
	}

	manifest, err := s.kubectl.PatchResource(config, res.GroupKindVersion(), res.Name, res.Namespace, types.PatchType(q.PatchType), []byte(q.Patch))
	if err != nil {
		return nil, err
	}
	manifest, err = replaceSecretValues(manifest)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(manifest.Object)
	if err != nil {
		return nil, err
	}
	s.logEvent(a, ctx, argo.EventReasonResourceUpdated, fmt.Sprintf("patched resource %s/%s '%s'", q.Group, q.Kind, q.ResourceName))
	return &application.ApplicationResourceResponse{
		Manifest: string(data),
	}, nil
}

// DeleteResource deletes a specificed resource
func (s *Server) DeleteResource(ctx context.Context, q *application.ApplicationResourceDeleteRequest) (*application.ApplicationResponse, error) {
	resourceRequest := &application.ApplicationResourceRequest{
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

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionDelete, appRBACName(*a)); err != nil {
		return nil, err
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
	return &application.ApplicationResponse{}, nil
}

func (s *Server) ResourceTree(ctx context.Context, q *application.ResourcesQuery) (*appv1.ApplicationTree, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.ApplicationName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName(*a)); err != nil {
		return nil, err
	}
	return s.getAppResources(q)
}

func (s *Server) RevisionMetadata(ctx context.Context, q *application.RevisionMetadataQuery) (*v1alpha1.RevisionMetadata, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(q.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName(*a)); err != nil {
		return nil, err
	}
	repo, err := s.db.GetRepository(ctx, a.Spec.Source.RepoURL)
	if err != nil {
		return nil, err
	}
	conn, repoClient, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, err
	}
	defer util.Close(conn)
	return repoClient.GetRevisionMetadata(ctx, &apiclient.RepoServerRevisionMetadataRequest{Repo: repo, Revision: q.GetRevision()})
}

func (s *Server) ManagedResources(ctx context.Context, q *application.ResourcesQuery) (*application.ManagedResourcesResponse, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.ApplicationName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName(*a)); err != nil {
		return nil, err
	}
	items, err := s.cache.GetAppManagedResources(a.Name)
	if err != nil {
		return nil, err
	}
	return &application.ManagedResourcesResponse{Items: items}, nil
}

func (s *Server) PodLogs(q *application.ApplicationPodLogsQuery, ws application.ApplicationService_PodLogsServer) error {
	pod, config, _, err := s.getAppResource(ws.Context(), rbacpolicy.ActionGet, &application.ApplicationResourceRequest{
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
	logCtx := log.WithField("application", q.Name)
	defer util.Close(stream)
	done := make(chan bool)
	gracefulExit := false
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
						err = ws.Send(&application.LogEntry{
							Content:   line,
							TimeStamp: metaLogTime,
						})
						if err != nil {
							logCtx.Warnf("Unable to send stream message: %v", err)
						}
					}
				}
			}
		}
		if gracefulExit {
			logCtx.Info("k8s pod logs scanner completed due to closed grpc context")
		} else if err := scanner.Err(); err != nil {
			logCtx.Warnf("k8s pod logs scanner failed with error: %v", err)
		} else {
			logCtx.Info("k8s pod logs scanner completed with EOF")
		}
		close(done)
	}()
	select {
	case <-ws.Context().Done():
		logCtx.Info("client pod logs grpc context closed")
		gracefulExit = true
	case <-done:
	}
	return nil
}

func (s *Server) getApplicationDestination(name string) (string, string, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(name, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}
	server, namespace := a.Spec.Destination.Server, a.Spec.Destination.Namespace
	return server, namespace, nil
}

// Sync syncs an application to its target state
func (s *Server) Sync(ctx context.Context, syncReq *application.ApplicationSyncRequest) (*appv1.Application, error) {
	appIf := s.appclientset.ArgoprojV1alpha1().Applications(s.ns)
	a, err := appIf.Get(*syncReq.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionSync, appRBACName(*a)); err != nil {
		return nil, err
	}
	if syncReq.Manifests != nil {
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionOverride, appRBACName(*a)); err != nil {
			return nil, err
		}
		if a.Spec.SyncPolicy != nil {
			return nil, status.Error(codes.FailedPrecondition, "Cannot use local sync when Automatic Sync Policy is enabled")
		}
	}
	if a.DeletionTimestamp != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "application is deleting")
	}
	if a.Spec.SyncPolicy != nil && a.Spec.SyncPolicy.Automated != nil {
		if syncReq.Revision != "" && syncReq.Revision != util.FirstNonEmpty(a.Spec.Source.TargetRevision, "HEAD") {
			return nil, status.Errorf(codes.FailedPrecondition, "Cannot sync to %s: auto-sync currently set to %s", syncReq.Revision, a.Spec.Source.TargetRevision)
		}
	}

	commitSHA, displayRevision, err := s.resolveRevision(ctx, a, syncReq)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, err.Error())
	}

	op := appv1.Operation{
		Sync: &appv1.SyncOperation{
			Revision:     commitSHA,
			Prune:        syncReq.Prune,
			DryRun:       syncReq.DryRun,
			SyncStrategy: syncReq.Strategy,
			Resources:    syncReq.Resources,
			Manifests:    syncReq.Manifests,
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

func (s *Server) Rollback(ctx context.Context, rollbackReq *application.ApplicationRollbackRequest) (*appv1.Application, error) {
	appIf := s.appclientset.ArgoprojV1alpha1().Applications(s.ns)
	a, err := appIf.Get(*rollbackReq.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionSync, appRBACName(*a)); err != nil {
		return nil, err
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
		return nil, status.Errorf(codes.InvalidArgument, "application %s does not have deployment with id %v", a.Name, rollbackReq.ID)
	}
	if deploymentInfo.Source.IsZero() {
		// Since source type was introduced to history starting with v0.12, and is now required for
		// rollback, we cannot support rollback to revisions deployed using Argo CD v0.11 or below
		return nil, status.Errorf(codes.FailedPrecondition, "cannot rollback to revision deployed with Argo CD v0.11 or lower. sync to revision instead.")
	}

	// Rollback is just a convenience around Sync
	op := appv1.Operation{
		Sync: &appv1.SyncOperation{
			Revision:     deploymentInfo.Revision,
			DryRun:       rollbackReq.DryRun,
			Prune:        rollbackReq.Prune,
			SyncStrategy: &appv1.SyncStrategy{Apply: &appv1.SyncStrategyApply{}},
			Source:       &deploymentInfo.Source,
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
func (s *Server) resolveRevision(ctx context.Context, app *appv1.Application, syncReq *application.ApplicationSyncRequest) (string, string, error) {
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
		return "", "", err
	}
	gitClient, err := s.gitFactory.NewClient(repo.Repo, "", argoutil.GetRepoCreds(repo), repo.IsInsecure(), repo.IsLFSEnabled())
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

func (s *Server) TerminateOperation(ctx context.Context, termOpReq *application.OperationTerminateRequest) (*application.OperationTerminateResponse, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*termOpReq.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionSync, appRBACName(*a)); err != nil {
		return nil, err
	}

	for i := 0; i < 10; i++ {
		if a.Operation == nil || a.Status.OperationState == nil {
			return nil, status.Errorf(codes.InvalidArgument, "Unable to terminate operation. No operation is in progress")
		}
		a.Status.OperationState.Phase = appv1.OperationTerminating
		_, err = s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Update(a)
		if err == nil {
			return &application.OperationTerminateResponse{}, nil
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

func (s *Server) ListResourceActions(ctx context.Context, q *application.ApplicationResourceRequest) (*application.ResourceActionsListResponse, error) {
	res, config, _, err := s.getAppResource(ctx, rbacpolicy.ActionGet, q)
	if err != nil {
		return nil, err
	}
	obj, err := s.kubectl.GetResource(config, res.GroupKindVersion(), res.Name, res.Namespace)
	if err != nil {
		return nil, err
	}
	resourceOverrides, err := s.settingsMgr.GetResourceOverrides()
	if err != nil {
		return nil, err
	}

	availableActions, err := s.getAvailableActions(resourceOverrides, obj, "")
	if err != nil {
		return nil, err
	}
	return &application.ResourceActionsListResponse{Actions: availableActions}, nil
}

func (s *Server) getAvailableActions(resourceOverrides map[string]v1alpha1.ResourceOverride, obj *unstructured.Unstructured, filterAction string) ([]appv1.ResourceAction, error) {
	luaVM := lua.VM{
		ResourceOverrides: resourceOverrides,
	}
	discoveryScript, err := luaVM.GetResourceActionDiscovery(obj)
	if err != nil {
		return nil, err
	}
	if discoveryScript == "" {
		return []appv1.ResourceAction{}, nil
	}
	availableActions, err := luaVM.ExecuteResourceActionDiscovery(obj, discoveryScript)
	if err != nil {
		return nil, err
	}
	for i := range availableActions {
		action := availableActions[i]
		if action.Name == filterAction {
			return []appv1.ResourceAction{action}, nil
		}
	}
	return availableActions, nil

}

func (s *Server) RunResourceAction(ctx context.Context, q *application.ResourceActionRunRequest) (*application.ApplicationResponse, error) {
	resourceRequest := &application.ApplicationResourceRequest{
		Name:         q.Name,
		Namespace:    q.Namespace,
		ResourceName: q.ResourceName,
		Kind:         q.Kind,
		Version:      q.Version,
		Group:        q.Group,
	}
	res, config, _, err := s.getAppResource(ctx, rbacpolicy.ActionOverride, resourceRequest)
	if err != nil {
		return nil, err
	}
	liveObj, err := s.kubectl.GetResource(config, res.GroupKindVersion(), res.Name, res.Namespace)
	if err != nil {
		return nil, err
	}

	resourceOverrides, err := s.settingsMgr.GetResourceOverrides()
	if err != nil {
		return nil, err
	}
	filteredAvailableActions, err := s.getAvailableActions(resourceOverrides, liveObj, q.Action)
	if err != nil {
		return nil, err
	}
	if len(filteredAvailableActions) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "action not available on resource")
	}

	luaVM := lua.VM{
		ResourceOverrides: resourceOverrides,
	}
	action, err := luaVM.GetResourceAction(liveObj, q.Action)
	if err != nil {
		return nil, err
	}

	newObj, err := luaVM.ExecuteResourceAction(liveObj, action.ActionLua)
	if err != nil {
		return nil, err
	}

	newObjBytes, err := json.Marshal(newObj)
	if err != nil {
		return nil, err
	}

	liveObjBytes, err := json.Marshal(liveObj)
	if err != nil {
		return nil, err
	}

	diffBytes, err := jsonpatch.CreateMergePatch(liveObjBytes, newObjBytes)
	if err != nil {
		return nil, err
	}
	if string(diffBytes) == "{}" {
		return &application.ApplicationResponse{}, nil
	}

	_, err = s.kubectl.PatchResource(config, newObj.GroupVersionKind(), newObj.GetName(), newObj.GetNamespace(), types.MergePatchType, diffBytes)
	if err != nil {
		return nil, err
	}
	return &application.ApplicationResponse{}, nil
}
