package application

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	goio "io"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	gosync "sync"
	"time"

	"github.com/Masterminds/semver"
	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/text"
	"github.com/argoproj/pkg/sync"
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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"

	argocommon "github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	applisters "github.com/argoproj/argo-cd/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	servercache "github.com/argoproj/argo-cd/server/cache"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/util/argo"
	argoutil "github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/env"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/helm"
	"github.com/argoproj/argo-cd/util/io"
	"github.com/argoproj/argo-cd/util/lua"
	"github.com/argoproj/argo-cd/util/rbac"
	"github.com/argoproj/argo-cd/util/session"
	"github.com/argoproj/argo-cd/util/settings"
)

const maxPodLogsToRender = 10

var (
	watchAPIBufferSize = env.ParseNumFromEnv(argocommon.EnvWatchAPIBufferSize, 1000, 0, math.MaxInt32)
)

// Server provides a Application service
type Server struct {
	ns             string
	kubeclientset  kubernetes.Interface
	appclientset   appclientset.Interface
	appLister      applisters.ApplicationNamespaceLister
	appInformer    cache.SharedIndexInformer
	appBroadcaster *broadcasterHandler
	repoClientset  apiclient.Clientset
	kubectl        kube.Kubectl
	db             db.ArgoDB
	enf            *rbac.Enforcer
	projectLock    sync.KeyLock
	auditLogger    *argo.AuditLogger
	settingsMgr    *settings.SettingsManager
	cache          *servercache.Cache
	projInformer   cache.SharedIndexInformer
}

// NewServer returns a new instance of the Application service
func NewServer(
	namespace string,
	kubeclientset kubernetes.Interface,
	appclientset appclientset.Interface,
	appLister applisters.ApplicationNamespaceLister,
	appInformer cache.SharedIndexInformer,
	repoClientset apiclient.Clientset,
	cache *servercache.Cache,
	kubectl kube.Kubectl,
	db db.ArgoDB,
	enf *rbac.Enforcer,
	projectLock sync.KeyLock,
	settingsMgr *settings.SettingsManager,
	projInformer cache.SharedIndexInformer,
) application.ApplicationServiceServer {
	appBroadcaster := &broadcasterHandler{}
	appInformer.AddEventHandler(appBroadcaster)
	return &Server{
		ns:             namespace,
		appclientset:   appclientset,
		appLister:      appLister,
		appInformer:    appInformer,
		appBroadcaster: appBroadcaster,
		kubeclientset:  kubeclientset,
		cache:          cache,
		db:             db,
		repoClientset:  repoClientset,
		kubectl:        kubectl,
		enf:            enf,
		projectLock:    projectLock,
		auditLogger:    argo.NewAuditLogger(namespace, kubeclientset, "argocd-server"),
		settingsMgr:    settingsMgr,
		projInformer:   projInformer,
	}
}

// appRBACName formats fully qualified application name for RBAC check
func appRBACName(app appv1.Application) string {
	return fmt.Sprintf("%s/%s", app.Spec.GetProject(), app.Name)
}

// List returns list of applications
func (s *Server) List(ctx context.Context, q *application.ApplicationQuery) (*appv1.ApplicationList, error) {
	labelsMap, err := labels.ConvertSelectorToLabelsMap(q.Selector)
	if err != nil {
		return nil, err
	}
	apps, err := s.appLister.List(labelsMap.AsSelector())
	if err != nil {
		return nil, err
	}
	newItems := make([]appv1.Application, 0)
	for _, a := range apps {
		if s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName(*a)) {
			newItems = append(newItems, *a)
		}
	}
	if q.Name != nil {
		newItems, err = argoutil.FilterByName(newItems, *q.Name)
		if err != nil {
			return nil, err
		}
	}
	newItems = argoutil.FilterByProjects(newItems, q.Projects)
	sort.Slice(newItems, func(i, j int) bool {
		return newItems[i].Name < newItems[j].Name
	})
	appList := appv1.ApplicationList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: s.appInformer.LastSyncResourceVersion(),
		},
		Items: newItems,
	}
	return &appList, nil
}

// Create creates an application
func (s *Server) Create(ctx context.Context, q *application.ApplicationCreateRequest) (*appv1.Application, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionCreate, appRBACName(q.Application)); err != nil {
		return nil, err
	}

	s.projectLock.Lock(q.Application.Spec.Project)
	defer s.projectLock.Unlock(q.Application.Spec.Project)

	a := q.Application
	validate := true
	if q.Validate != nil {
		validate = *q.Validate
	}
	err := s.validateAndNormalizeApp(ctx, &a, validate)
	if err != nil {
		return nil, err
	}
	created, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Create(ctx, &a, metav1.CreateOptions{})
	if err == nil {
		s.logAppEvent(created, ctx, argo.EventReasonResourceCreated, "created application")
		s.waitSync(created)
		return created, nil
	}
	if !apierr.IsAlreadyExists(err) {
		return nil, err
	}
	// act idempotent if existing spec matches new spec
	existing, err := s.appLister.Get(a.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to check existing application details: %v", err)
	}
	equalSpecs := reflect.DeepEqual(existing.Spec, a.Spec) &&
		reflect.DeepEqual(existing.Labels, a.Labels) &&
		reflect.DeepEqual(existing.Annotations, a.Annotations) &&
		reflect.DeepEqual(existing.Finalizers, a.Finalizers)

	if equalSpecs {
		return existing, nil
	}
	if q.Upsert == nil || !*q.Upsert {
		return nil, status.Errorf(codes.InvalidArgument, "existing application spec is different, use upsert flag to force update")
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionUpdate, appRBACName(a)); err != nil {
		return nil, err
	}
	updated, err := s.updateApp(existing, &a, ctx, true)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// GetManifests returns application manifests
func (s *Server) GetManifests(ctx context.Context, q *application.ApplicationManifestQuery) (*apiclient.ManifestResponse, error) {
	a, err := s.appLister.Get(*q.Name)
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
	defer io.Close(conn)
	revision := a.Spec.Source.TargetRevision
	if q.Revision != "" {
		revision = q.Revision
	}
	appInstanceLabelKey, err := s.settingsMgr.GetAppInstanceLabelKey()
	if err != nil {
		return nil, err
	}
	helmRepos, err := s.db.ListHelmRepositories(ctx)
	if err != nil {
		return nil, err
	}

	plugins, err := s.plugins()
	if err != nil {
		return nil, err
	}
	// If source is Kustomize add build options
	kustomizeSettings, err := s.settingsMgr.GetKustomizeSettings()
	if err != nil {
		return nil, err
	}
	kustomizeOptions, err := kustomizeSettings.GetOptions(a.Spec.Source)
	if err != nil {
		return nil, err
	}
	config, err := s.getApplicationClusterConfig(ctx, a)
	if err != nil {
		return nil, err
	}

	serverVersion, err := s.kubectl.GetServerVersion(config)
	if err != nil {
		return nil, err
	}

	apiGroups, err := s.kubectl.GetAPIGroups(config)
	if err != nil {
		return nil, err
	}
	manifestInfo, err := repoClient.GenerateManifest(ctx, &apiclient.ManifestRequest{
		Repo:              repo,
		Revision:          revision,
		AppLabelKey:       appInstanceLabelKey,
		AppName:           a.Name,
		Namespace:         a.Spec.Destination.Namespace,
		ApplicationSource: &a.Spec.Source,
		Repos:             helmRepos,
		Plugins:           plugins,
		KustomizeOptions:  kustomizeOptions,
		KubeVersion:       serverVersion,
		ApiVersions:       argo.APIGroupsToVersions(apiGroups),
	})
	if err != nil {
		return nil, err
	}
	for i, manifest := range manifestInfo.Manifests {
		obj := &unstructured.Unstructured{}
		err = json.Unmarshal([]byte(manifest), obj)
		if err != nil {
			return nil, err
		}
		if obj.GetKind() == kube.SecretKind && obj.GroupVersionKind().Group == "" {
			obj, _, err = diff.HideSecretData(obj, nil)
			if err != nil {
				return nil, err
			}
			data, err := json.Marshal(obj)
			if err != nil {
				return nil, err
			}
			manifestInfo.Manifests[i] = string(data)
		}
	}

	return manifestInfo, nil
}

// Get returns an application by name
func (s *Server) Get(ctx context.Context, q *application.ApplicationQuery) (*appv1.Application, error) {
	// We must use a client Get instead of an informer Get, because it's common to call Get immediately
	// following a Watch (which is not yet powered by an informer), and the Get must reflect what was
	// previously seen by the client.
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(ctx, q.GetName(), metav1.GetOptions{
		ResourceVersion: q.ResourceVersion,
	})

	if err != nil {
		return nil, err
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName(*a)); err != nil {
		return nil, err
	}
	if q.Refresh == nil {
		return a, nil
	}

	refreshType := appv1.RefreshTypeNormal
	if *q.Refresh == string(appv1.RefreshTypeHard) {
		refreshType = appv1.RefreshTypeHard
	}
	appIf := s.appclientset.ArgoprojV1alpha1().Applications(s.ns)

	// subscribe early with buffered channel to ensure we don't miss events
	events := make(chan *appv1.ApplicationWatchEvent, watchAPIBufferSize)
	unsubscribe := s.appBroadcaster.Subscribe(events, func(event *appv1.ApplicationWatchEvent) bool {
		return event.Application.Name == q.GetName()
	})
	defer unsubscribe()

	app, err := argoutil.RefreshApp(appIf, *q.Name, refreshType)
	if err != nil {
		return nil, err
	}

	minVersion := 0
	if minVersion, err = strconv.Atoi(app.ResourceVersion); err != nil {
		minVersion = 0
	}

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("application refresh deadline exceeded")
		case event := <-events:
			if appVersion, err := strconv.Atoi(event.Application.ResourceVersion); err == nil && appVersion > minVersion {
				annotations := event.Application.GetAnnotations()
				if annotations == nil {
					annotations = make(map[string]string)
				}
				if _, ok := annotations[argocommon.AnnotationKeyRefresh]; !ok {
					return &event.Application, nil
				}
			}
		}
	}
}

// ListResourceEvents returns a list of event resources
func (s *Server) ListResourceEvents(ctx context.Context, q *application.ApplicationResourceEventsQuery) (*v1.EventList, error) {
	a, err := s.appLister.Get(*q.Name)
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
		config, err = s.getApplicationClusterConfig(ctx, a)
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
	return kubeClientset.CoreV1().Events(namespace).List(ctx, opts)
}

func (s *Server) validateAndUpdateApp(ctx context.Context, newApp *appv1.Application, merge bool, validate bool) (*appv1.Application, error) {
	s.projectLock.Lock(newApp.Spec.GetProject())
	defer s.projectLock.Unlock(newApp.Spec.GetProject())

	app, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(ctx, newApp.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	err = s.validateAndNormalizeApp(ctx, newApp, validate)
	if err != nil {
		return nil, err
	}

	return s.updateApp(app, newApp, ctx, merge)
}

func mergeStringMaps(items ...map[string]string) map[string]string {
	res := make(map[string]string)
	for _, m := range items {
		if m == nil {
			continue
		}
		for k, v := range m {
			res[k] = v
		}
	}
	return res
}

var informerSyncTimeout = 2 * time.Second

// waitSync is a helper to wait until the application informer cache is synced after create/update.
// It waits until the app in the informer, has a resource version greater than the version in the
// supplied app, or after 2 seconds, whichever comes first. Returns true if synced.
// We use an informer cache for read operations (Get, List). Since the cache is only
// eventually consistent, it is possible that it doesn't reflect an application change immediately
// after a mutating API call (create/update). This function should be called after a creates &
// update to give a probable (but not guaranteed) chance of being up-to-date after the create/update.
func (s *Server) waitSync(app *appv1.Application) {
	logCtx := log.WithField("application", app.Name)
	deadline := time.Now().Add(informerSyncTimeout)
	minVersion, err := strconv.Atoi(app.ResourceVersion)
	if err != nil {
		logCtx.Warnf("waitSync failed: could not parse resource version %s", app.ResourceVersion)
		time.Sleep(50 * time.Millisecond) // sleep anyways
		return
	}
	for {
		if currApp, err := s.appLister.Get(app.Name); err == nil {
			currVersion, err := strconv.Atoi(currApp.ResourceVersion)
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

func (s *Server) updateApp(app *appv1.Application, newApp *appv1.Application, ctx context.Context, merge bool) (*appv1.Application, error) {
	for i := 0; i < 10; i++ {
		app.Spec = newApp.Spec
		if merge {
			app.Labels = mergeStringMaps(app.Labels, newApp.Labels)
			app.Annotations = mergeStringMaps(app.Annotations, newApp.Annotations)
		} else {
			app.Labels = newApp.Labels
			app.Annotations = newApp.Annotations
		}

		app.Finalizers = newApp.Finalizers

		res, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Update(ctx, app, metav1.UpdateOptions{})
		if err == nil {
			s.logAppEvent(app, ctx, argo.EventReasonResourceUpdated, "updated application spec")
			s.waitSync(res)
			return res, nil
		}
		if !apierr.IsConflict(err) {
			return nil, err
		}

		app, err = s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(ctx, newApp.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}
	return nil, status.Errorf(codes.Internal, "Failed to update application. Too many conflicts")
}

// Update updates an application
func (s *Server) Update(ctx context.Context, q *application.ApplicationUpdateRequest) (*appv1.Application, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionUpdate, appRBACName(*q.Application)); err != nil {
		return nil, err
	}

	validate := true
	if q.Validate != nil {
		validate = *q.Validate
	}
	return s.validateAndUpdateApp(ctx, q.Application, false, validate)
}

// UpdateSpec updates an application spec and filters out any invalid parameter overrides
func (s *Server) UpdateSpec(ctx context.Context, q *application.ApplicationUpdateSpecRequest) (*appv1.ApplicationSpec, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(ctx, *q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionUpdate, appRBACName(*a)); err != nil {
		return nil, err
	}
	a.Spec = q.Spec
	validate := true
	if q.Validate != nil {
		validate = *q.Validate
	}
	a, err = s.validateAndUpdateApp(ctx, a, false, validate)
	if err != nil {
		return nil, err
	}
	return &a.Spec, nil
}

// Patch patches an application
func (s *Server) Patch(ctx context.Context, q *application.ApplicationPatchRequest) (*appv1.Application, error) {

	app, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(ctx, *q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if err = s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionUpdate, appRBACName(*app)); err != nil {
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

	newApp := &v1alpha1.Application{}
	err = json.Unmarshal(patchApp, newApp)
	if err != nil {
		return nil, err
	}
	return s.validateAndUpdateApp(ctx, newApp, false, true)
}

// Delete removes an application and all associated resources
func (s *Server) Delete(ctx context.Context, q *application.ApplicationDeleteRequest) (*application.ApplicationResponse, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(ctx, *q.Name, metav1.GetOptions{})
	if err != nil {
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
		_, err = s.appclientset.ArgoprojV1alpha1().Applications(a.Namespace).Patch(ctx, a.Name, types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			return nil, err
		}
	}

	err = s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Delete(ctx, *q.Name, metav1.DeleteOptions{})
	if err != nil {
		return nil, err
	}
	s.logAppEvent(a, ctx, argo.EventReasonResourceDeleted, "deleted application")
	return &application.ApplicationResponse{}, nil
}

func (s *Server) Watch(q *application.ApplicationQuery, ws application.ApplicationService_WatchServer) error {
	logCtx := log.NewEntry(log.New())
	if q.Name != nil {
		logCtx = logCtx.WithField("application", *q.Name)
	}
	claims := ws.Context().Value("claims")
	selector, err := labels.Parse(q.Selector)
	if err != nil {
		return err
	}
	minVersion := 0
	if q.ResourceVersion != "" {
		if minVersion, err = strconv.Atoi(q.ResourceVersion); err != nil {
			minVersion = 0
		}
	}

	// sendIfPermitted is a helper to send the application to the client's streaming channel if the
	// caller has RBAC privileges permissions to view it
	sendIfPermitted := func(a appv1.Application, eventType watch.EventType) {
		if appVersion, err := strconv.Atoi(a.ResourceVersion); err == nil && appVersion < minVersion {
			return
		}
		matchedEvent := q.GetName() == "" || a.Name == q.GetName() && selector.Matches(labels.Set(a.Labels))
		if !matchedEvent {
			return
		}

		if !s.enf.Enforce(claims, rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName(a)) {
			// do not emit apps user does not have accessing
			return
		}
		err := ws.Send(&appv1.ApplicationWatchEvent{
			Type:        eventType,
			Application: a,
		})
		if err != nil {
			logCtx.Warnf("Unable to send stream message: %v", err)
			return
		}
	}

	events := make(chan *appv1.ApplicationWatchEvent, watchAPIBufferSize)
	// Mimic watch API behavior: send ADDED events if no resource version provided
	// If watch API is executed for one application when emit event even if resource version is provided
	// This is required since single app watch API is used for during operations like app syncing and it is
	// critical to never miss events.
	if q.ResourceVersion == "" || q.GetName() != "" {
		apps, err := s.appLister.List(selector)
		if err != nil {
			return err
		}
		for i := range apps {
			sendIfPermitted(*apps[i], watch.Added)
		}
	}
	unsubscribe := s.appBroadcaster.Subscribe(events)
	defer unsubscribe()
	for {
		select {
		case event := <-events:
			sendIfPermitted(event.Application, event.Type)
		case <-ws.Context().Done():
			return nil
		}
	}
}

func (s *Server) validateAndNormalizeApp(ctx context.Context, app *appv1.Application, validate bool) error {
	proj, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(ctx, app.Spec.GetProject(), metav1.GetOptions{})
	if err != nil {
		if apierr.IsNotFound(err) {
			return status.Errorf(codes.InvalidArgument, "application references project %s which does not exist", app.Spec.Project)
		}
		return err
	}
	currApp, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(ctx, app.Name, metav1.GetOptions{})
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

	// If source is Kustomize add build options
	kustomizeSettings, err := s.settingsMgr.GetKustomizeSettings()
	if err != nil {
		return err
	}
	kustomizeOptions, err := kustomizeSettings.GetOptions(app.Spec.Source)
	if err != nil {
		return err
	}
	plugins, err := s.plugins()
	if err != nil {
		return err
	}

	if err := argo.ValidateDestination(ctx, &app.Spec.Destination, s.db); err != nil {
		return status.Errorf(codes.InvalidArgument, "application destination spec is invalid: %s", err.Error())
	}

	var conditions []appv1.ApplicationCondition
	if validate {
		conditions, err = argo.ValidateRepo(ctx, app, s.repoClientset, s.db, kustomizeOptions, plugins, s.kubectl)
		if err != nil {
			return err
		}
		if len(conditions) > 0 {
			return status.Errorf(codes.InvalidArgument, "application spec is invalid: %s", argo.FormatAppConditions(conditions))
		}
	}

	conditions, err = argo.ValidatePermissions(ctx, &app.Spec, proj, s.db)
	if err != nil {
		return err
	}
	if len(conditions) > 0 {
		return status.Errorf(codes.InvalidArgument, "application spec is invalid: %s", argo.FormatAppConditions(conditions))
	}

	app.Spec = *argo.NormalizeApplicationSpec(&app.Spec)
	return nil
}

func (s *Server) getApplicationClusterConfig(ctx context.Context, a *appv1.Application) (*rest.Config, error) {
	if err := argo.ValidateDestination(ctx, &a.Spec.Destination, s.db); err != nil {
		return nil, err
	}
	clst, err := s.db.GetCluster(ctx, a.Spec.Destination.Server)
	if err != nil {
		return nil, err
	}
	config := clst.RESTConfig()
	return config, err
}

// getCachedAppState loads the cached state and trigger app refresh if cache is missing
func (s *Server) getCachedAppState(ctx context.Context, a *appv1.Application, getFromCache func() error) error {
	err := getFromCache()
	if err != nil && err == servercache.ErrCacheMiss {
		conditions := a.Status.GetConditions(map[appv1.ApplicationConditionType]bool{
			appv1.ApplicationConditionComparisonError:  true,
			appv1.ApplicationConditionInvalidSpecError: true,
		})
		if len(conditions) > 0 {
			return errors.New(argoutil.FormatAppConditions(conditions))
		}
		_, err = s.Get(ctx, &application.ApplicationQuery{
			Name:    pointer.StringPtr(a.Name),
			Refresh: pointer.StringPtr(string(appv1.RefreshTypeNormal)),
		})
		if err != nil {
			return err
		}
		return getFromCache()
	}
	return err
}

func (s *Server) getAppResources(ctx context.Context, a *appv1.Application) (*appv1.ApplicationTree, error) {
	var tree appv1.ApplicationTree
	err := s.getCachedAppState(ctx, a, func() error {
		return s.cache.GetAppResourcesTree(a.Name, &tree)
	})
	return &tree, err
}

func (s *Server) getAppResource(ctx context.Context, action string, q *application.ApplicationResourceRequest) (*appv1.ResourceNode, *rest.Config, *appv1.Application, error) {
	a, err := s.appLister.Get(*q.Name)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, action, appRBACName(*a)); err != nil {
		return nil, nil, nil, err
	}

	tree, err := s.getAppResources(ctx, a)
	if err != nil {
		return nil, nil, nil, err
	}

	found := tree.FindNode(q.Group, q.Kind, q.Namespace, q.ResourceName)
	if found == nil {
		return nil, nil, nil, status.Errorf(codes.InvalidArgument, "%s %s %s not found as part of application %s", q.Kind, q.Group, q.ResourceName, *q.Name)
	}
	config, err := s.getApplicationClusterConfig(ctx, a)
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

	// make sure to use specified resource version if provided
	if q.Version != "" {
		res.Version = q.Version
	}
	obj, err := s.kubectl.GetResource(ctx, config, res.GroupKindVersion(), res.Name, res.Namespace)
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

	manifest, err := s.kubectl.PatchResource(ctx, config, res.GroupKindVersion(), res.Name, res.Namespace, types.PatchType(q.PatchType), []byte(q.Patch))
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
	s.logAppEvent(a, ctx, argo.EventReasonResourceUpdated, fmt.Sprintf("patched resource %s/%s '%s'", q.Group, q.Kind, q.ResourceName))
	return &application.ApplicationResourceResponse{
		Manifest: string(data),
	}, nil
}

// DeleteResource deletes a specified resource
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
	err = s.kubectl.DeleteResource(ctx, config, res.GroupKindVersion(), res.Name, res.Namespace, force)
	if err != nil {
		return nil, err
	}
	s.logAppEvent(a, ctx, argo.EventReasonResourceDeleted, fmt.Sprintf("deleted resource %s/%s '%s'", q.Group, q.Kind, q.ResourceName))
	return &application.ApplicationResponse{}, nil
}

func (s *Server) ResourceTree(ctx context.Context, q *application.ResourcesQuery) (*appv1.ApplicationTree, error) {
	a, err := s.appLister.Get(q.GetApplicationName())
	if err != nil {
		return nil, err
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName(*a)); err != nil {
		return nil, err
	}
	return s.getAppResources(ctx, a)
}

func (s *Server) WatchResourceTree(q *application.ResourcesQuery, ws application.ApplicationService_WatchResourceTreeServer) error {
	a, err := s.appLister.Get(q.GetApplicationName())
	if err != nil {
		return err
	}

	if err := s.enf.EnforceErr(ws.Context().Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName(*a)); err != nil {
		return err
	}

	return s.cache.OnAppResourcesTreeChanged(ws.Context(), q.GetApplicationName(), func() error {
		var tree appv1.ApplicationTree
		err := s.cache.GetAppResourcesTree(q.GetApplicationName(), &tree)
		if err != nil {
			return err
		}
		return ws.Send(&tree)
	})
}

func (s *Server) RevisionMetadata(ctx context.Context, q *application.RevisionMetadataQuery) (*v1alpha1.RevisionMetadata, error) {
	a, err := s.appLister.Get(q.GetName())
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
	// We need to get some information with the project associated to the app,
	// so we'll know whether GPG signatures are enforced.
	proj, err := argo.GetAppProject(&a.Spec, applisters.NewAppProjectLister(s.projInformer.GetIndexer()), a.Namespace, s.settingsMgr)
	if err != nil {
		return nil, err
	}
	conn, repoClient, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, err
	}
	defer io.Close(conn)
	return repoClient.GetRevisionMetadata(ctx, &apiclient.RepoServerRevisionMetadataRequest{
		Repo:           repo,
		Revision:       q.GetRevision(),
		CheckSignature: len(proj.Spec.SignatureKeys) > 0,
	})
}

func isMatchingResource(q *application.ResourcesQuery, key kube.ResourceKey) bool {
	return (q.Name == "" || q.Name == key.Name) &&
		(q.Namespace == "" || q.Namespace == key.Namespace) &&
		(q.Group == "" || q.Group == key.Group) &&
		(q.Kind == "" || q.Kind == key.Kind)
}

func (s *Server) ManagedResources(ctx context.Context, q *application.ResourcesQuery) (*application.ManagedResourcesResponse, error) {
	a, err := s.appLister.Get(*q.ApplicationName)
	if err != nil {
		return nil, err
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName(*a)); err != nil {
		return nil, err
	}
	items := make([]*appv1.ResourceDiff, 0)
	err = s.getCachedAppState(ctx, a, func() error {
		return s.cache.GetAppManagedResources(a.Name, &items)
	})
	if err != nil {
		return nil, err
	}
	res := &application.ManagedResourcesResponse{}
	for i := range items {
		item := items[i]
		if isMatchingResource(q, kube.ResourceKey{Name: item.Name, Namespace: item.Namespace, Kind: item.Kind, Group: item.Group}) {
			res.Items = append(res.Items, item)
		}
	}

	return res, nil
}

func (s *Server) PodLogs(q *application.ApplicationPodLogsQuery, ws application.ApplicationService_PodLogsServer) error {
	if q.PodName != nil {
		podKind := "Pod"
		q.Kind = &podKind
		q.ResourceName = q.PodName
	}

	// get all nodes
	tree, err := s.ResourceTree(ws.Context(), &application.ResourcesQuery{ApplicationName: q.Name})
	if err != nil {
		return err
	}

	// from the tree find pods which match query of kind, group, and resource name
	pods := getSelectedPods(tree.Nodes, q)
	if len(pods) == 0 {
		return nil
	}

	if len(pods) > maxPodLogsToRender {
		return errors.New("Max pods to view logs are reached. Please provide more granular query.")
	}

	var wg gosync.WaitGroup
	wg.Add(len(pods))
	errorsCh := make(chan error, len(pods))
	responseCh := make(chan PodLogsResponse, len(pods))

	for _, pod := range pods {
		go func(i appv1.ResourceNode) {
			data, err := q.Marshal()
			if err != nil {
				errorsCh <- err
				responseCh <- ErrorAccured
				wg.Done()
				return
			}

			var copiedQuery application.ApplicationPodLogsQuery
			err = copiedQuery.Unmarshal(data)
			if err != nil {
				errorsCh <- err
				responseCh <- ErrorAccured
				wg.Done()
				return
			}

			copiedQuery.Namespace = i.Namespace
			copiedQuery.PodName = &i.Name

			log.Debug("processing pod logs for ", i.Name)
			response, err := s.processOnePodLog(&copiedQuery, ws)
			errorsCh <- err
			responseCh <- response
			log.Debug("processing pod logs done ", i.Name)
			wg.Done()

		}(pod)
	}

	wg.Wait()

	allDone := true
	for i := 0; i < len(pods); i++ {
		err := <-errorsCh
		if err != nil {
			return err
		}
		//also check if all exist with no errors, then send the last message.
		response := <-responseCh
		if response == ErrorAccured {
			allDone = false
		}
	}

	if allDone {
		if err := ws.Send(&application.LogEntry{Last: true}); err != nil {
			log.Warnf("Unable to send stream message notifying about last log message: %v", err)
			return err
		}
	}

	return nil
}

// from all of the treeNodes, get the pod who meets the criteria or whose parents meets the criteria
func getSelectedPods(treeNodes []appv1.ResourceNode, q *application.ApplicationPodLogsQuery) []appv1.ResourceNode {
	var pods []appv1.ResourceNode
	isTheOneMap := make(map[string]bool)
	for _, treeNode := range treeNodes {
		if treeNode.Kind == kube.PodKind && treeNode.Group == "" {
			if isTheSelectedOne(&treeNode, q, treeNodes, isTheOneMap) {
				pods = append(pods, treeNode)
			}
		}
	}
	return pods
}

// check is currentNode is matching with group, kind, and name, or if any of its parents matches
func isTheSelectedOne(currentNode *appv1.ResourceNode, q *application.ApplicationPodLogsQuery, resourceNodes []appv1.ResourceNode, isTheOneMap map[string]bool) bool {
	exist, value := isTheOneMap[currentNode.UID]
	if exist {
		return value
	}

	if (q.ResourceName == nil || *q.ResourceName == "" || currentNode.Name == *q.ResourceName) &&
		(q.Kind == nil || *q.Kind == "" || currentNode.Kind == *q.Kind) &&
		(q.Group == nil || *q.Group == "" || currentNode.Group == *q.Group) &&
		(q.Namespace == "" || currentNode.Namespace == q.Namespace) {
		isTheOneMap[currentNode.UID] = true
		return true
	}

	if len(currentNode.ParentRefs) == 0 {
		isTheOneMap[currentNode.UID] = false
		return false
	}

	for _, parentResource := range currentNode.ParentRefs {
		//look up parentResource from resourceNodes
		//then check if the parent isTheSelectedOne
		for _, resourceNode := range resourceNodes {
			if resourceNode.Namespace == parentResource.Namespace &&
				resourceNode.Name == parentResource.Name &&
				resourceNode.Group == parentResource.Group &&
				resourceNode.Kind == parentResource.Kind {
				if isTheSelectedOne(&resourceNode, q, resourceNodes, isTheOneMap) {
					isTheOneMap[currentNode.UID] = true
					return true
				}
			}
		}
	}

	isTheOneMap[currentNode.UID] = false
	return false
}

type PodLogsResponse int

const (
	ErrorAccured     = -1
	ReachedEOF       = 0
	UntilTimeReached = 1
	GracefulExit     = 2
)

func (s *Server) processOnePodLog(q *application.ApplicationPodLogsQuery, ws application.ApplicationService_PodLogsServer) (PodLogsResponse, error) {
	var response PodLogsResponse
	response = ErrorAccured

	var untilTime *metav1.Time
	if q.GetUntilTime() != "" {
		if val, err := time.Parse(time.RFC3339Nano, q.GetUntilTime()); err != nil {
			return response, fmt.Errorf("invalid untilTime parameter value: %v", err)
		} else {
			untilTimeVal := metav1.NewTime(val)
			untilTime = &untilTimeVal
		}
	}

	pod, config, _, err := s.getAppResource(ws.Context(), rbacpolicy.ActionGet, &application.ApplicationResourceRequest{
		Name:         q.Name,
		Namespace:    q.Namespace,
		Kind:         kube.PodKind,
		Group:        "",
		Version:      "v1",
		ResourceName: *q.PodName,
	})

	if err != nil {
		return response, err
	}

	kubeClientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return response, err
	}

	var sinceSeconds, tailLines *int64
	if q.SinceSeconds > 0 {
		sinceSeconds = &q.SinceSeconds
	}
	if q.TailLines > 0 {
		tailLines = &q.TailLines
	}

	literal := ""
	inverse := false
	if q.GetFilter() != "" {
		literal = *q.Filter
		if literal[0] == '!' {
			literal = literal[1:]
			inverse = true
		}
	}
	stream, err := kubeClientset.CoreV1().Pods(pod.Namespace).GetLogs(*q.PodName, &v1.PodLogOptions{
		Container:    q.Container,
		Follow:       q.Follow,
		Timestamps:   true,
		SinceSeconds: sinceSeconds,
		SinceTime:    q.SinceTime,
		TailLines:    tailLines,
	}).Stream(ws.Context())
	if err != nil {
		return response, err
	}
	logCtx := log.WithField("application", q.Name)
	defer io.Close(stream)
	done := make(chan bool)
	gracefulExit := false
	go func() {
		bufReader := bufio.NewReader(stream)
		for {
			line, err := bufReader.ReadString('\n')
			if err != nil {
				// Error or io.EOF
				break
			}
			line = strings.TrimSpace(line) // Remove trailing line ending
			parts := strings.Split(line, " ")
			timeStampStr := parts[0]
			logTime, err := time.Parse(time.RFC3339Nano, timeStampStr)
			metaLogTime := metav1.NewTime(logTime)
			if err == nil {
				lines := strings.Join(parts[1:], " ")
				for _, line := range strings.Split(lines, "\r") {
					if q.Filter != nil {
						lineContainsFilter := strings.Contains(line, literal)
						if (inverse && lineContainsFilter) || (!inverse && !lineContainsFilter) {
							continue
						}
					}
					if q.GetUntilTime() != "" && !metaLogTime.Before(untilTime) {
						response = UntilTimeReached
						close(done)
						return
					}
					err = ws.Send(&application.LogEntry{
						Content:      line,
						TimeStamp:    metaLogTime,
						TimeStampStr: timeStampStr,
					})
					if err != nil {
						logCtx.Warnf("Unable to send stream message: %v", err)
					}
				}
			}
		}
		if gracefulExit {
			logCtx.Info("k8s pod logs reader completed due to closed grpc context")
		} else if err != nil && err != goio.EOF {
			logCtx.Warnf("k8s pod logs reader failed with error: %v", err)
		} else {
			logCtx.Info("k8s pod logs reader completed with EOF")
			response = ReachedEOF
		}
		close(done)
	}()

	select {
	case <-ws.Context().Done():
		logCtx.Info("client pod logs grpc context closed")
		gracefulExit = true
		response = GracefulExit
	case <-done:
	}

	return response, nil
}

// Sync syncs an application to its target state
func (s *Server) Sync(ctx context.Context, syncReq *application.ApplicationSyncRequest) (*appv1.Application, error) {
	appIf := s.appclientset.ArgoprojV1alpha1().Applications(s.ns)
	a, err := appIf.Get(ctx, *syncReq.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	proj, err := argo.GetAppProject(&a.Spec, applisters.NewAppProjectLister(s.projInformer.GetIndexer()), a.Namespace, s.settingsMgr)
	if err != nil {
		if apierr.IsNotFound(err) {
			return a, status.Errorf(codes.InvalidArgument, "application references project %s which does not exist", a.Spec.Project)
		}
		return a, err
	}

	if !proj.Spec.SyncWindows.Matches(a).CanSync(true) {
		return a, status.Errorf(codes.PermissionDenied, "Cannot sync: Blocked by sync window")
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionSync, appRBACName(*a)); err != nil {
		return nil, err
	}
	if syncReq.Manifests != nil {
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionOverride, appRBACName(*a)); err != nil {
			return nil, err
		}
		if a.Spec.SyncPolicy != nil && a.Spec.SyncPolicy.Automated != nil && !syncReq.DryRun {
			return nil, status.Error(codes.FailedPrecondition, "Cannot use local sync when Automatic Sync Policy is enabled unless for dry run")
		}
	}
	if a.DeletionTimestamp != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "application is deleting")
	}
	if a.Spec.SyncPolicy != nil && a.Spec.SyncPolicy.Automated != nil {
		if syncReq.Revision != "" && syncReq.Revision != text.FirstNonEmpty(a.Spec.Source.TargetRevision, "HEAD") {
			return nil, status.Errorf(codes.FailedPrecondition, "Cannot sync to %s: auto-sync currently set to %s", syncReq.Revision, a.Spec.Source.TargetRevision)
		}
	}
	revision, displayRevision, err := s.resolveRevision(ctx, a, syncReq)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, err.Error())
	}

	var retry *appv1.RetryStrategy
	var syncOptions appv1.SyncOptions
	if a.Spec.SyncPolicy != nil {
		syncOptions = a.Spec.SyncPolicy.SyncOptions
		retry = a.Spec.SyncPolicy.Retry
	}
	if syncReq.RetryStrategy != nil {
		retry = syncReq.RetryStrategy
	}

	// We cannot use local manifests if we're only allowed to sync to signed commits
	if syncReq.Manifests != nil && len(proj.Spec.SignatureKeys) > 0 {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot use local sync when signature keys are required.")
	}

	op := appv1.Operation{
		Sync: &appv1.SyncOperation{
			Revision:     revision,
			Prune:        syncReq.Prune,
			DryRun:       syncReq.DryRun,
			SyncOptions:  syncOptions,
			SyncStrategy: syncReq.Strategy,
			Resources:    syncReq.Resources,
			Manifests:    syncReq.Manifests,
		},
		InitiatedBy: appv1.OperationInitiator{Username: session.Username(ctx)},
		Info:        syncReq.Infos,
	}
	if retry != nil {
		op.Retry = *retry
	}

	a, err = argo.SetAppOperation(appIf, *syncReq.Name, &op)
	if err == nil {
		partial := ""
		if len(syncReq.Resources) > 0 {
			partial = "partial "
		}
		s.logAppEvent(a, ctx, argo.EventReasonOperationStarted, fmt.Sprintf("initiated %ssync to %s", partial, displayRevision))
	}
	return a, err
}

func (s *Server) Rollback(ctx context.Context, rollbackReq *application.ApplicationRollbackRequest) (*appv1.Application, error) {
	appIf := s.appclientset.ArgoprojV1alpha1().Applications(s.ns)
	a, err := appIf.Get(ctx, *rollbackReq.Name, metav1.GetOptions{})
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

	var syncOptions appv1.SyncOptions
	if a.Spec.SyncPolicy != nil {
		syncOptions = a.Spec.SyncPolicy.SyncOptions
	}

	// Rollback is just a convenience around Sync
	op := appv1.Operation{
		Sync: &appv1.SyncOperation{
			Revision:     deploymentInfo.Revision,
			DryRun:       rollbackReq.DryRun,
			Prune:        rollbackReq.Prune,
			SyncOptions:  syncOptions,
			SyncStrategy: &appv1.SyncStrategy{Apply: &appv1.SyncStrategyApply{}},
			Source:       &deploymentInfo.Source,
		},
	}
	a, err = argo.SetAppOperation(appIf, *rollbackReq.Name, &op)
	if err == nil {
		s.logAppEvent(a, ctx, argo.EventReasonOperationStarted, fmt.Sprintf("initiated rollback to %d", rollbackReq.ID))
	}
	return a, err
}

// resolveRevision resolves the revision specified either in the sync request, or the
// application source, into a concrete revision that will be used for a sync operation.
func (s *Server) resolveRevision(ctx context.Context, app *appv1.Application, syncReq *application.ApplicationSyncRequest) (string, string, error) {
	ambiguousRevision := syncReq.Revision
	if ambiguousRevision == "" {
		ambiguousRevision = app.Spec.Source.TargetRevision
	}
	var revision string
	if app.Spec.Source.IsHelm() {
		repo, err := s.db.GetRepository(ctx, app.Spec.Source.RepoURL)
		if err != nil {
			return "", "", err
		}
		if helm.IsVersion(ambiguousRevision) {
			return ambiguousRevision, ambiguousRevision, nil
		}
		client := helm.NewClient(repo.Repo, repo.GetHelmCreds(), repo.EnableOCI || app.Spec.Source.IsHelmOci())
		index, err := client.GetIndex()
		if err != nil {
			return "", "", err
		}
		entries, err := index.GetEntries(app.Spec.Source.Chart)
		if err != nil {
			return "", "", err
		}
		constraints, err := semver.NewConstraint(ambiguousRevision)
		if err != nil {
			return "", "", err
		}
		version, err := entries.MaxVersion(constraints)
		if err != nil {
			return "", "", err
		}
		return version.String(), fmt.Sprintf("%v (%v)", ambiguousRevision, version.String()), nil
	} else {
		if git.IsCommitSHA(ambiguousRevision) {
			// If it's already a commit SHA, then no need to look it up
			return ambiguousRevision, ambiguousRevision, nil
		}
		repo, err := s.db.GetRepository(ctx, app.Spec.Source.RepoURL)
		if err != nil {
			return "", "", err
		}
		gitClient, err := git.NewClient(repo.Repo, repo.GetGitCreds(), repo.IsInsecure(), repo.IsLFSEnabled())
		if err != nil {
			return "", "", err
		}
		revision, err = gitClient.LsRemote(ambiguousRevision)
		if err != nil {
			return "", "", err
		}
		return revision, fmt.Sprintf("%s (%s)", ambiguousRevision, revision), nil
	}
}

func (s *Server) TerminateOperation(ctx context.Context, termOpReq *application.OperationTerminateRequest) (*application.OperationTerminateResponse, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(ctx, *termOpReq.Name, metav1.GetOptions{})
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
		a.Status.OperationState.Phase = common.OperationTerminating
		updated, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Update(ctx, a, metav1.UpdateOptions{})
		if err == nil {
			s.waitSync(updated)
			s.logAppEvent(a, ctx, argo.EventReasonResourceUpdated, "terminated running operation")
			return &application.OperationTerminateResponse{}, nil
		}
		if !apierr.IsConflict(err) {
			return nil, err
		}
		log.Warnf("Failed to set operation for app '%s' due to update conflict. Retrying again...", *termOpReq.Name)
		time.Sleep(100 * time.Millisecond)
		a, err = s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(ctx, *termOpReq.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}
	return nil, status.Errorf(codes.Internal, "Failed to terminate app. Too many conflicts")
}

func (s *Server) logAppEvent(a *appv1.Application, ctx context.Context, reason string, action string) {
	eventInfo := argo.EventInfo{Type: v1.EventTypeNormal, Reason: reason}
	user := session.Username(ctx)
	if user == "" {
		user = "Unknown user"
	}
	message := fmt.Sprintf("%s %s", user, action)
	s.auditLogger.LogAppEvent(a, eventInfo, message)
}

func (s *Server) logResourceEvent(res *appv1.ResourceNode, ctx context.Context, reason string, action string) {
	eventInfo := argo.EventInfo{Type: v1.EventTypeNormal, Reason: reason}
	user := session.Username(ctx)
	if user == "" {
		user = "Unknown user"
	}
	message := fmt.Sprintf("%s %s", user, action)
	s.auditLogger.LogResourceEvent(res, eventInfo, message)
}

func (s *Server) ListResourceActions(ctx context.Context, q *application.ApplicationResourceRequest) (*application.ResourceActionsListResponse, error) {
	res, config, _, err := s.getAppResource(ctx, rbacpolicy.ActionGet, q)
	if err != nil {
		return nil, err
	}
	obj, err := s.kubectl.GetResource(ctx, config, res.GroupKindVersion(), res.Name, res.Namespace)
	if err != nil {
		return nil, err
	}
	resourceOverrides, err := s.settingsMgr.GetResourceOverrides()
	if err != nil {
		return nil, err
	}

	availableActions, err := s.getAvailableActions(resourceOverrides, obj)
	if err != nil {
		return nil, err
	}

	return &application.ResourceActionsListResponse{Actions: availableActions}, nil
}

func (s *Server) getAvailableActions(resourceOverrides map[string]appv1.ResourceOverride, obj *unstructured.Unstructured) ([]appv1.ResourceAction, error) {
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
	actionRequest := fmt.Sprintf("%s/%s/%s/%s", rbacpolicy.ActionAction, q.Group, q.Kind, q.Action)
	res, config, a, err := s.getAppResource(ctx, actionRequest, resourceRequest)
	if err != nil {
		return nil, err
	}
	liveObj, err := s.kubectl.GetResource(ctx, config, res.GroupKindVersion(), res.Name, res.Namespace)
	if err != nil {
		return nil, err
	}

	resourceOverrides, err := s.settingsMgr.GetResourceOverrides()
	if err != nil {
		return nil, err
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

	// The following logic detects if the resource action makes a modification to status and/or spec.
	// If status was modified, we attempt to patch the status using status subresource, in case the
	// CRD is configured using the status subresource feature. See:
	// https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#status-subresource
	// If status subresource is in use, the patch has to be split into two:
	// * one to update spec (and other non-status fields)
	// * the other to update only status.
	nonStatusPatch, statusPatch, err := splitStatusPatch(diffBytes)
	if err != nil {
		return nil, err
	}
	if statusPatch != nil {
		_, err = s.kubectl.PatchResource(ctx, config, newObj.GroupVersionKind(), newObj.GetName(), newObj.GetNamespace(), types.MergePatchType, diffBytes, "status")
		if err != nil {
			if !apierr.IsNotFound(err) {
				return nil, err
			}
			// K8s API server returns 404 NotFound when the CRD does not support the status subresource
			// if we get here, the CRD does not use the status subresource. We will fall back to a normal patch
		} else {
			// If we get here, the CRD does use the status subresource, so we must patch status and
			// spec separately. update the diffBytes to the spec-only patch and fall through.
			diffBytes = nonStatusPatch
		}
	}
	if diffBytes != nil {
		_, err = s.kubectl.PatchResource(ctx, config, newObj.GroupVersionKind(), newObj.GetName(), newObj.GetNamespace(), types.MergePatchType, diffBytes)
		if err != nil {
			return nil, err
		}
	}

	s.logAppEvent(a, ctx, argo.EventReasonResourceActionRan, fmt.Sprintf("ran action %s on resource %s/%s/%s", q.Action, res.Group, res.Kind, res.Name))
	s.logResourceEvent(res, ctx, argo.EventReasonResourceActionRan, fmt.Sprintf("ran action %s", q.Action))
	return &application.ApplicationResponse{}, nil
}

// splitStatusPatch splits a patch into two: one for a non-status patch, and the status-only patch.
// Returns nil for either if the patch doesn't have modifications to non-status, or status, respectively.
func splitStatusPatch(patch []byte) ([]byte, []byte, error) {
	var obj map[string]interface{}
	err := json.Unmarshal(patch, &obj)
	if err != nil {
		return nil, nil, err
	}
	var nonStatusPatch, statusPatch []byte
	if statusVal, ok := obj["status"]; ok {
		// calculate the status-only patch
		statusObj := map[string]interface{}{
			"status": statusVal,
		}
		statusPatch, err = json.Marshal(statusObj)
		if err != nil {
			return nil, nil, err
		}
		// remove status, and calculate the non-status patch
		delete(obj, "status")
		if len(obj) > 0 {
			nonStatusPatch, err = json.Marshal(obj)
			if err != nil {
				return nil, nil, err
			}
		}
	} else {
		// status was not modified in patch
		nonStatusPatch = patch
	}
	return nonStatusPatch, statusPatch, nil
}

func (s *Server) plugins() ([]*v1alpha1.ConfigManagementPlugin, error) {
	plugins, err := s.settingsMgr.GetConfigManagementPlugins()
	if err != nil {
		return nil, err
	}
	tools := make([]*v1alpha1.ConfigManagementPlugin, len(plugins))
	for i, p := range plugins {
		p := p
		tools[i] = &p
	}
	return tools, nil
}

func (s *Server) GetApplicationSyncWindows(ctx context.Context, q *application.ApplicationSyncWindowsQuery) (*application.ApplicationSyncWindowsResponse, error) {
	appIf := s.appclientset.ArgoprojV1alpha1().Applications(s.ns)
	a, err := appIf.Get(ctx, *q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName(*a)); err != nil {
		return nil, err
	}

	proj, err := argo.GetAppProject(&a.Spec, applisters.NewAppProjectLister(s.projInformer.GetIndexer()), a.Namespace, s.settingsMgr)
	if err != nil {
		return nil, err
	}

	windows := proj.Spec.SyncWindows.Matches(a)
	sync := windows.CanSync(true)

	res := &application.ApplicationSyncWindowsResponse{
		ActiveWindows:   convertSyncWindows(windows.Active()),
		AssignedWindows: convertSyncWindows(windows),
		CanSync:         &sync,
	}

	return res, nil
}

func convertSyncWindows(w *v1alpha1.SyncWindows) []*application.ApplicationSyncWindow {
	if w != nil {
		var windows []*application.ApplicationSyncWindow
		for _, w := range *w {
			nw := &application.ApplicationSyncWindow{
				Kind:       &w.Kind,
				Schedule:   &w.Schedule,
				Duration:   &w.Duration,
				ManualSync: &w.ManualSync,
			}
			windows = append(windows, nw)
		}
		if len(windows) > 0 {
			return windows
		}
	}
	return nil
}
