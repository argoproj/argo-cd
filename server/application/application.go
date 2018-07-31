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
	"k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/controller"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/argo"
	argoutil "github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/rbac"
	"github.com/argoproj/argo-cd/util/session"
)

// Server provides a Application service
type Server struct {
	ns            string
	kubeclientset kubernetes.Interface
	appclientset  appclientset.Interface
	repoClientset reposerver.Clientset
	db            db.ArgoDB
	appComparator controller.AppStateManager
	enf           *rbac.Enforcer
	projectLock   *util.KeyLock
	auditLogger   *argo.AuditLogger
}

// NewServer returns a new instance of the Application service
func NewServer(
	namespace string,
	kubeclientset kubernetes.Interface,
	appclientset appclientset.Interface,
	repoClientset reposerver.Clientset,
	db db.ArgoDB,
	enf *rbac.Enforcer,
	projectLock *util.KeyLock,
) ApplicationServiceServer {

	return &Server{
		ns:            namespace,
		appclientset:  appclientset,
		kubeclientset: kubeclientset,
		db:            db,
		repoClientset: repoClientset,
		appComparator: controller.NewAppStateManager(db, appclientset, repoClientset, namespace),
		enf:           enf,
		projectLock:   projectLock,
		auditLogger:   argo.NewAuditLogger(namespace, kubeclientset, "argocd-server"),
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
		if s.enf.EnforceClaims(ctx.Value("claims"), "applications", "get", appRBACName(a)) {
			newItems = append(newItems, a)
		}
	}
	newItems = argoutil.FilterByProjects(newItems, q.Projects)
	appList.Items = newItems
	return appList, nil
}

// Create creates an application
func (s *Server) Create(ctx context.Context, q *ApplicationCreateRequest) (*appv1.Application, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "applications", "create", appRBACName(q.Application)) {
		return nil, grpc.ErrPermissionDenied
	}

	if !q.Application.Spec.BelongsToDefaultProject() {
		s.projectLock.Lock(q.Application.Spec.Project)
		defer s.projectLock.Unlock(q.Application.Spec.Project)
	}

	a := q.Application
	err := s.validateApp(ctx, &a.Spec)
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
			if !s.enf.EnforceClaims(ctx.Value("claims"), "applications", "update", appRBACName(a)) {
				return nil, grpc.ErrPermissionDenied
			}
			existing.Spec = a.Spec
			out, err = s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Update(existing)
		} else {
			if reflect.DeepEqual(existing.Spec, a.Spec) {
				return existing, nil
			} else {
				return nil, status.Errorf(codes.InvalidArgument, "existing application spec is different, use upsert flag to force update")
			}
		}
	}

	if err == nil {
		s.logEvent(out, ctx, argo.EventReasonResourceCreated, "create")
	}
	return out, err
}

// GetManifests returns application manifests
func (s *Server) GetManifests(ctx context.Context, q *ApplicationManifestQuery) (*repository.ManifestResponse, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !s.enf.EnforceClaims(ctx.Value("claims"), "applications/manifests", "get", appRBACName(*a)) {
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
	manifestInfo, err := repoClient.GenerateManifest(context.Background(), &repository.ManifestRequest{
		Repo:                        repo,
		Environment:                 a.Spec.Source.Environment,
		Path:                        a.Spec.Source.Path,
		Revision:                    revision,
		ComponentParameterOverrides: overrides,
		AppLabel:                    a.Name,
		ValueFiles:                  a.Spec.Source.ValuesFiles,
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
	if !s.enf.EnforceClaims(ctx.Value("claims"), "applications", "get", appRBACName(*a)) {
		return nil, grpc.ErrPermissionDenied
	}
	if q.Refresh {
		_, err = argoutil.RefreshApp(appIf, *q.Name)
		if err != nil {
			return nil, err
		}
		a, err = argoutil.WaitForRefresh(appIf, *q.Name, nil)
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
	if !s.enf.EnforceClaims(ctx.Value("claims"), "applications/events", "get", appRBACName(*a)) {
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
		var config *rest.Config
		config, namespace, err = s.getApplicationClusterConfig(*q.Name)
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
	if !s.enf.EnforceClaims(ctx.Value("claims"), "applications", "update", appRBACName(*q.Application)) {
		return nil, grpc.ErrPermissionDenied
	}

	if !q.Application.Spec.BelongsToDefaultProject() {
		s.projectLock.Lock(q.Application.Spec.Project)
		defer s.projectLock.Unlock(q.Application.Spec.Project)
	}

	a := q.Application
	err := s.validateApp(ctx, &a.Spec)
	if err != nil {
		return nil, err
	}
	return s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Update(a)
}

// removeInvalidOverrides removes any parameter overrides that are no longer valid
// drops old overrides that are invalid
// throws an error is passed override is invalid
// if passed override and old overrides are invalid, throws error, old overrides not dropped
func (s *Server) removeInvalidOverrides(a *appv1.Application, q *ApplicationUpdateSpecRequest) (*ApplicationUpdateSpecRequest, error) {
	if a.Spec.Source.Environment == "" {
		// this method is only valid for ksonnet apps
		return q, nil
	}
	oldParams := argo.ParamToMap(a.Spec.Source.ComponentParameterOverrides)
	validAppSet := argo.ParamToMap(a.Status.Parameters)

	params := make([]appv1.ComponentParameter, 0)
	for i := range q.Spec.Source.ComponentParameterOverrides {
		param := q.Spec.Source.ComponentParameterOverrides[i]
		if !argo.CheckValidParam(validAppSet, param) {
			alreadySet := argo.CheckValidParam(oldParams, param)
			if !alreadySet {
				return nil, status.Errorf(codes.InvalidArgument, "Parameter '%s' in '%s' does not exist in ksonnet app", param.Name, param.Component)
			}
		} else {
			params = append(params, param)
		}
	}
	q.Spec.Source.ComponentParameterOverrides = params
	return q, nil
}

// UpdateSpec updates an application spec and filters out any invalid parameter overrides
func (s *Server) UpdateSpec(ctx context.Context, q *ApplicationUpdateSpecRequest) (*appv1.ApplicationSpec, error) {

	if !q.Spec.BelongsToDefaultProject() {
		s.projectLock.Lock(q.Spec.Project)
		defer s.projectLock.Unlock(q.Spec.Project)
	}

	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !s.enf.EnforceClaims(ctx.Value("claims"), "applications", "update", appRBACName(*a)) {
		return nil, grpc.ErrPermissionDenied
	}
	err = s.validateApp(ctx, &q.Spec)
	if err != nil {
		return nil, err
	}
	q, err = s.removeInvalidOverrides(a, q)
	if err != nil {
		return nil, err
	}
	for {
		a.Spec = q.Spec
		_, err = s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Update(a)
		if err == nil {
			if err != nil {
				s.logEvent(a, ctx, argo.EventReasonResourceUpdated, "update")
			}
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
}

// Delete removes an application and all associated resources
func (s *Server) Delete(ctx context.Context, q *ApplicationDeleteRequest) (*ApplicationResponse, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.Name, metav1.GetOptions{})
	if err != nil && !apierr.IsNotFound(err) {
		return nil, err
	}

	if !a.Spec.BelongsToDefaultProject() {
		s.projectLock.Lock(a.Spec.Project)
		defer s.projectLock.Unlock(a.Spec.Project)
	}

	if !s.enf.EnforceClaims(ctx.Value("claims"), "applications", "delete", appRBACName(*a)) {
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

	s.logEvent(a, ctx, argo.EventReasonResourceDeleted, "delete")
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
				if !s.enf.EnforceClaims(claims, "applications", "get", appRBACName(a)) {
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

func (s *Server) validateApp(ctx context.Context, spec *appv1.ApplicationSpec) error {
	proj, err := argo.GetAppProject(spec, s.appclientset, s.ns)
	if err != nil {
		if apierr.IsNotFound(err) {
			return status.Errorf(codes.InvalidArgument, "application referencing project %s which does not exist", spec.Project)
		}
		return err
	}
	if !s.enf.EnforceClaims(ctx.Value("claims"), "projects", "get", proj.Name) {
		return status.Errorf(codes.PermissionDenied, "permission denied for project %s", proj.Name)
	}
	conditions, err := argo.GetSpecErrors(ctx, spec, proj, s.repoClientset, s.db)
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

func (s *Server) ensurePodBelongsToApp(applicationName string, podName, namespace string, kubeClientset *kubernetes.Clientset) error {
	pod, err := kubeClientset.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	wrongPodError := status.Errorf(codes.InvalidArgument, "pod %s does not belong to application %s", podName, applicationName)
	if pod.Labels == nil {
		return wrongPodError
	}
	if value, ok := pod.Labels[common.LabelApplicationName]; !ok || value != applicationName {
		return wrongPodError
	}
	return nil
}

func (s *Server) DeletePod(ctx context.Context, q *ApplicationDeletePodRequest) (*ApplicationResponse, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if !s.enf.EnforceClaims(ctx.Value("claims"), "applications/pods", "delete", appRBACName(*a)) {
		return nil, grpc.ErrPermissionDenied
	}

	config, namespace, err := s.getApplicationClusterConfig(*q.Name)
	if err != nil {
		return nil, err
	}
	kubeClientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	err = s.ensurePodBelongsToApp(*q.Name, *q.PodName, namespace, kubeClientset)
	if err != nil {
		return nil, err
	}
	err = kubeClientset.CoreV1().Pods(namespace).Delete(*q.PodName, &metav1.DeleteOptions{})
	if err != nil {
		return nil, err
	}
	return &ApplicationResponse{}, nil
}

func (s *Server) PodLogs(q *ApplicationPodLogsQuery, ws ApplicationService_PodLogsServer) error {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if !s.enf.EnforceClaims(ws.Context().Value("claims"), "applications/logs", "get", appRBACName(*a)) {
		return grpc.ErrPermissionDenied
	}
	config, namespace, err := s.getApplicationClusterConfig(*q.Name)
	if err != nil {
		return err
	}
	kubeClientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	err = s.ensurePodBelongsToApp(*q.Name, *q.PodName, namespace, kubeClientset)
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
	stream, err := kubeClientset.CoreV1().Pods(namespace).GetLogs(*q.PodName, &v1.PodLogOptions{
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
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*syncReq.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !s.enf.EnforceClaims(ctx.Value("claims"), "applications", "sync", appRBACName(*a)) {
		return nil, grpc.ErrPermissionDenied
	}
	return s.setAppOperation(ctx, *syncReq.Name, "sync", func(app *appv1.Application) (*appv1.Operation, error) {
		syncOp := appv1.SyncOperation{
			Revision:     syncReq.Revision,
			Prune:        syncReq.Prune,
			DryRun:       syncReq.DryRun,
			SyncStrategy: syncReq.Strategy,
		}
		return &appv1.Operation{
			Sync: &syncOp,
		}, nil
	})
}

func (s *Server) Rollback(ctx context.Context, rollbackReq *ApplicationRollbackRequest) (*appv1.Application, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*rollbackReq.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !s.enf.EnforceClaims(ctx.Value("claims"), "applications", "rollback", appRBACName(*a)) {
		return nil, grpc.ErrPermissionDenied
	}
	return s.setAppOperation(ctx, *rollbackReq.Name, "rollback", func(app *appv1.Application) (*appv1.Operation, error) {
		return &appv1.Operation{
			Rollback: &appv1.RollbackOperation{
				ID:     rollbackReq.ID,
				Prune:  rollbackReq.Prune,
				DryRun: rollbackReq.DryRun,
			},
		}, nil
	})
}

func (s *Server) setAppOperation(ctx context.Context, appName string, operationName string, operationCreator func(app *appv1.Application) (*appv1.Operation, error)) (*appv1.Application, error) {
	for {
		a, err := s.Get(ctx, &ApplicationQuery{Name: &appName})
		if err != nil {
			return nil, err
		}
		if a.Operation != nil {
			return nil, status.Errorf(codes.InvalidArgument, "another operation is already in progress")
		}
		op, err := operationCreator(a)
		if err != nil {
			return nil, err
		}
		a.Operation = op
		a.Status.OperationState = nil
		_, err = s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Update(a)
		if err != nil && apierr.IsConflict(err) {
			log.Warnf("Failed to set operation for app '%s' due to update conflict. Retrying again...", appName)
		} else {
			if err == nil {
				s.logEvent(a, ctx, argo.EventReasonResourceUpdated, operationName)
			}
			return a, err
		}
	}
}

func (s *Server) TerminateOperation(ctx context.Context, termOpReq *OperationTerminateRequest) (*OperationTerminateResponse, error) {
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*termOpReq.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !s.enf.EnforceClaims(ctx.Value("claims"), "applications", "terminateop", appRBACName(*a)) {
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
		log.Warnf("Failed to set operation for app '%s' due to update conflict. Retrying again...", termOpReq.Name)
		time.Sleep(100 * time.Millisecond)
		a, err = s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*termOpReq.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		} else {
			s.logEvent(a, ctx, argo.EventReasonResourceUpdated, "terminateop")
		}
	}
	return nil, status.Errorf(codes.Internal, "Failed to terminate app. Too many conflicts")
}

func (s *Server) logEvent(a *appv1.Application, ctx context.Context, reason string, action string) {
	s.auditLogger.LogAppEvent(a, argo.EventInfo{Reason: reason, Action: action, Username: session.Username(ctx)}, v1.EventTypeNormal)
}
