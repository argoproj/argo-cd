package application

import (
	"bufio"
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/ksonnet/ksonnet/pkg/app"
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
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/rbac"
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
}

// NewServer returns a new instance of the Application service
func NewServer(
	namespace string,
	kubeclientset kubernetes.Interface,
	appclientset appclientset.Interface,
	repoClientset reposerver.Clientset,
	db db.ArgoDB,
	enf *rbac.Enforcer,
) ApplicationServiceServer {

	return &Server{
		ns:            namespace,
		appclientset:  appclientset,
		kubeclientset: kubeclientset,
		db:            db,
		repoClientset: repoClientset,
		appComparator: controller.NewAppStateManager(db, appclientset, repoClientset, namespace),
		enf:           enf,
	}
}

// List returns list of applications
func (s *Server) List(ctx context.Context, q *ApplicationQuery) (*appv1.ApplicationList, error) {
	appList, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	newItems := make([]appv1.Application, 0)
	for _, app := range appList.Items {
		if s.enf.EnforceClaims(ctx.Value("claims"), "applications", "get", fmt.Sprintf("*/%s", app.Name)) {
			newItems = append(newItems, app)
		}
	}
	appList.Items = newItems
	return appList, nil
}

// Create creates an application
func (s *Server) Create(ctx context.Context, q *ApplicationCreateRequest) (*appv1.Application, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "applications", "create", fmt.Sprintf("*/%s", q.Application.Name)) {
		return nil, grpc.ErrPermissionDenied
	}
	a := q.Application
	err := s.validateApp(ctx, &a.Spec)
	if err != nil {
		return nil, err
	}
	a.SetCascadedDeletion(true)
	out, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Create(&a)
	if apierr.IsAlreadyExists(err) {
		// act idempotent if existing spec matches new spec
		existing, getErr := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(a.Name, metav1.GetOptions{})
		if getErr != nil {
			return nil, status.Errorf(codes.Internal, "unable to check existing application details: %v", err)
		}
		if q.Upsert != nil && *q.Upsert {
			if !s.enf.EnforceClaims(ctx.Value("claims"), "applications", "update", fmt.Sprintf("*/%s", q.Application.Name)) {
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
	return out, err
}

// GetManifests returns application manifests
func (s *Server) GetManifests(ctx context.Context, q *ApplicationManifestQuery) (*repository.ManifestResponse, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "applications/manifests", "get", fmt.Sprintf("*/%s", *q.Name)) {
		return nil, grpc.ErrPermissionDenied
	}
	app, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	repo := s.getRepo(ctx, app.Spec.Source.RepoURL)

	conn, repoClient, err := s.repoClientset.NewRepositoryClient()
	if err != nil {
		return nil, err
	}
	defer util.Close(conn)
	overrides := make([]*appv1.ComponentParameter, len(app.Spec.Source.ComponentParameterOverrides))
	if app.Spec.Source.ComponentParameterOverrides != nil {
		for i := range app.Spec.Source.ComponentParameterOverrides {
			item := app.Spec.Source.ComponentParameterOverrides[i]
			overrides[i] = &item
		}
	}

	revision := app.Spec.Source.TargetRevision
	if q.Revision != "" {
		revision = q.Revision
	}
	manifestInfo, err := repoClient.GenerateManifest(context.Background(), &repository.ManifestRequest{
		Repo:                        repo,
		Environment:                 app.Spec.Source.Environment,
		Path:                        app.Spec.Source.Path,
		Revision:                    revision,
		ComponentParameterOverrides: overrides,
		AppLabel:                    app.Name,
	})
	if err != nil {
		return nil, err
	}

	return manifestInfo, nil
}

// Get returns an application by name
func (s *Server) Get(ctx context.Context, q *ApplicationQuery) (*appv1.Application, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "applications", "get", fmt.Sprintf("*/%s", *q.Name)) {
		return nil, grpc.ErrPermissionDenied
	}
	return s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.Name, metav1.GetOptions{})
}

// ListResourceEvents returns a list of event resources
func (s *Server) ListResourceEvents(ctx context.Context, q *ApplicationResourceEventsQuery) (*v1.EventList, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "applications/events", "get", fmt.Sprintf("*/%s", *q.Name)) {
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

	fieldSelector := fields.SelectorFromSet(map[string]string{
		"involvedObject.name":      q.ResourceName,
		"involvedObject.uid":       q.ResourceUID,
		"involvedObject.namespace": namespace,
	}).String()

	log.Infof("Querying for resource events with field selector: %s", fieldSelector)
	opts := metav1.ListOptions{FieldSelector: fieldSelector}

	return kubeClientset.CoreV1().Events(namespace).List(opts)
}

// Update updates an application
func (s *Server) Update(ctx context.Context, q *ApplicationUpdateRequest) (*appv1.Application, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "applications", "update", fmt.Sprintf("*/%s", q.Application.Name)) {
		return nil, grpc.ErrPermissionDenied
	}
	a := q.Application
	err := s.validateApp(ctx, &a.Spec)
	if err != nil {
		return nil, err
	}
	return s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Update(a)
}

// UpdateSpec updates an application spec
func (s *Server) UpdateSpec(ctx context.Context, q *ApplicationUpdateSpecRequest) (*appv1.ApplicationSpec, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "applications", "update", fmt.Sprintf("*/%s", *q.Name)) {
		return nil, grpc.ErrPermissionDenied
	}
	err := s.validateApp(ctx, &q.Spec)
	if err != nil {
		return nil, err
	}
	patch, err := json.Marshal(map[string]appv1.ApplicationSpec{
		"spec": q.Spec,
	})
	if err != nil {
		return nil, err
	}
	_, err = s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Patch(*q.Name, types.MergePatchType, patch)
	return &q.Spec, err
}

// Delete removes an application and all associated resources
func (s *Server) Delete(ctx context.Context, q *ApplicationDeleteRequest) (*ApplicationResponse, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "applications", "delete", fmt.Sprintf("*/%s", *q.Name)) {
		return nil, grpc.ErrPermissionDenied
	}
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(*q.Name, metav1.GetOptions{})
	if err != nil && !apierr.IsNotFound(err) {
		return nil, err
	}

	if q.Cascade != nil && *q.Cascade != a.CascadedDeletion() {
		a.SetCascadedDeletion(*q.Cascade)
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
			app := *next.Object.(*appv1.Application)
			if q.Name == nil || *q.Name == "" || *q.Name == app.Name {
				if !s.enf.EnforceClaims(claims, "applications", "get", fmt.Sprintf("*/%s", app.Name)) {
					// do not emit apps user does not have accessing
					continue
				}
				err = ws.Send(&appv1.ApplicationWatchEvent{
					Type:        next.Type,
					Application: app,
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
	}
	return nil
}

// validateApp will ensure:
// * the git repository is accessible
// * the git path contains a valid app.yaml
// * the specified environment exists
// * the referenced cluster has been added to ArgoCD
func (s *Server) validateApp(ctx context.Context, spec *appv1.ApplicationSpec) error {
	// Test the repo
	conn, repoClient, err := s.repoClientset.NewRepositoryClient()
	if err != nil {
		return err
	}
	defer util.Close(conn)
	repoRes, err := s.db.GetRepository(ctx, spec.Source.RepoURL)
	if err != nil {
		if errStatus, ok := status.FromError(err); ok && errStatus.Code() == codes.NotFound {
			// The repo has not been added to ArgoCD so we do not have credentials to access it.
			// We support the mode where apps can be created from public repositories. Test the
			// repo to make sure it is publicly accessible
			err = git.TestRepo(spec.Source.RepoURL, "", "", "")
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// Verify app.yaml is functional
	req := repository.GetFileRequest{
		Repo: &appv1.Repository{
			Repo: spec.Source.RepoURL,
		},
		Revision: spec.Source.TargetRevision,
		Path:     path.Join(spec.Source.Path, "app.yaml"),
	}
	if repoRes != nil {
		req.Repo.Username = repoRes.Username
		req.Repo.Password = repoRes.Password
		req.Repo.SSHPrivateKey = repoRes.SSHPrivateKey
	}
	getRes, err := repoClient.GetFile(ctx, &req)
	if err != nil {
		return err
	}
	var appSpec app.Spec
	err = yaml.Unmarshal(getRes.Data, &appSpec)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "app.yaml is not a valid ksonnet app spec")
	}

	// Default revision to HEAD if unspecified
	if spec.Source.TargetRevision == "" {
		spec.Source.TargetRevision = "HEAD"
	}

	// Verify the specified environment is defined in it
	envSpec, ok := appSpec.Environments[spec.Source.Environment]
	if !ok || envSpec == nil {
		return status.Errorf(codes.InvalidArgument, "environment '%s' does not exist in ksonnet app", spec.Source.Environment)
	}

	// If server and namespace are not supplied, pull it from the app.yaml
	if spec.Destination.Server == "" {
		spec.Destination.Server = envSpec.Destination.Server
	}
	if spec.Destination.Namespace == "" {
		spec.Destination.Namespace = envSpec.Destination.Namespace
	}

	// Ensure the k8s cluster the app is referencing, is configured in ArgoCD
	_, err = s.db.GetCluster(ctx, spec.Destination.Server)
	if err != nil {
		if apierr.IsNotFound(err) {
			return status.Errorf(codes.InvalidArgument, "cluster '%s' has not been configured", spec.Destination.Server)
		}
		return err
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
	if !s.enf.EnforceClaims(ctx.Value("claims"), "applications/pods", "delete", fmt.Sprintf("*/%s", *q.Name)) {
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
	claims := ws.Context().Value("claims")
	if !s.enf.EnforceClaims(claims, "applications/logs", "get", fmt.Sprintf("*/%s", *q.Name)) {
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
	app, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(name, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}
	server, namespace := app.Spec.Destination.Server, app.Spec.Destination.Namespace
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
	if !s.enf.EnforceClaims(ctx.Value("claims"), "applications", "sync", fmt.Sprintf("*/%s", *syncReq.Name)) {
		return nil, grpc.ErrPermissionDenied
	}
	return s.setAppOperation(ctx, *syncReq.Name, func(app *appv1.Application) (*appv1.Operation, error) {
		return &appv1.Operation{
			Sync: &appv1.SyncOperation{
				Revision: syncReq.Revision,
				Prune:    syncReq.Prune,
				DryRun:   syncReq.DryRun,
			},
		}, nil
	})
}

func (s *Server) Rollback(ctx context.Context, rollbackReq *ApplicationRollbackRequest) (*appv1.Application, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "applications", "rollback", fmt.Sprintf("*/%s", *rollbackReq.Name)) {
		return nil, grpc.ErrPermissionDenied
	}
	return s.setAppOperation(ctx, *rollbackReq.Name, func(app *appv1.Application) (*appv1.Operation, error) {
		return &appv1.Operation{
			Rollback: &appv1.RollbackOperation{
				ID:     rollbackReq.ID,
				Prune:  rollbackReq.Prune,
				DryRun: rollbackReq.DryRun,
			},
		}, nil
	})
}

func (s *Server) setAppOperation(ctx context.Context, appName string, operationCreator func(app *appv1.Application) (*appv1.Operation, error)) (*appv1.Application, error) {
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
			return a, err
		}
	}
}
