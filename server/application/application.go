package application

import (
	"bufio"
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/controller"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/server/cluster"
	apirepository "github.com/argoproj/argo-cd/server/repository"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/ghodss/yaml"
	"github.com/ksonnet/ksonnet/pkg/app"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Server provides a Application service
type Server struct {
	ns            string
	kubeclientset kubernetes.Interface
	appclientset  appclientset.Interface
	repoClientset reposerver.Clientset
	// TODO(jessesuen): move common cluster code to shared libraries
	clusterService cluster.ClusterServiceServer
	repoService    apirepository.RepositoryServiceServer
	appComparator  controller.AppStateManager
}

// NewServer returns a new instance of the Application service
func NewServer(
	namespace string,
	kubeclientset kubernetes.Interface,
	appclientset appclientset.Interface,
	repoClientset reposerver.Clientset,
	repoService apirepository.RepositoryServiceServer,
	clusterService cluster.ClusterServiceServer) ApplicationServiceServer {

	return &Server{
		ns:             namespace,
		appclientset:   appclientset,
		kubeclientset:  kubeclientset,
		clusterService: clusterService,
		repoClientset:  repoClientset,
		repoService:    repoService,
		appComparator:  controller.NewAppStateManager(clusterService, repoService, appclientset, repoClientset, namespace),
	}
}

// List returns list of applications
func (s *Server) List(ctx context.Context, q *ApplicationQuery) (*appv1.ApplicationList, error) {
	return s.appclientset.ArgoprojV1alpha1().Applications(s.ns).List(metav1.ListOptions{})
}

// Create creates an application
func (s *Server) Create(ctx context.Context, a *appv1.Application) (*appv1.Application, error) {
	err := s.validateApp(ctx, &a.Spec)
	if err != nil {
		return nil, err
	}
	out, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Create(a)
	if apierr.IsAlreadyExists(err) {
		// act idempotent if existing spec matches new spec
		existing, err2 := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(a.Name, metav1.GetOptions{})
		if err2 == nil {
			if reflect.DeepEqual(existing.Spec, a.Spec) {
				return existing, nil
			}
		}
	}
	return out, err
}

// Get returns an application by name
func (s *Server) Get(ctx context.Context, q *ApplicationQuery) (*appv1.Application, error) {
	return s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(q.Name, metav1.GetOptions{})
}

// Update updates an application
func (s *Server) Update(ctx context.Context, a *appv1.Application) (*appv1.Application, error) {
	err := s.validateApp(ctx, &a.Spec)
	if err != nil {
		return nil, err
	}
	return s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Update(a)
}

// UpdateSpec updates an application spec
func (s *Server) UpdateSpec(ctx context.Context, q *ApplicationSpecRequest) (*appv1.ApplicationSpec, error) {
	err := s.validateApp(ctx, q.Spec)
	if err != nil {
		return nil, err
	}
	patch, err := json.Marshal(map[string]appv1.ApplicationSpec{
		"spec": *q.Spec,
	})
	if err != nil {
		return nil, err
	}
	_, err = s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Patch(q.AppName, types.MergePatchType, patch)
	return q.Spec, err
}

// Delete removes an application and all associated resources
func (s *Server) Delete(ctx context.Context, q *DeleteApplicationRequest) (*ApplicationResponse, error) {
	var err error
	server := q.Server
	namespace := q.Namespace
	if server == "" || namespace == "" {
		server, namespace, err = s.getApplicationDestination(ctx, q.Name)
		if err != nil && !apierr.IsNotFound(err) && !q.Force {
			return nil, err
		}
	}

	if server != "" && namespace != "" {
		clst, err := s.clusterService.Get(ctx, &cluster.ClusterQuery{Server: server})
		if err != nil && !q.Force {
			return nil, err
		}
		if clst != nil {
			config := clst.RESTConfig()
			err = kube.DeleteResourceWithLabel(config, namespace, common.LabelApplicationName, q.Name)
			if err != nil && !q.Force {
				return nil, err
			}
		}
	}

	err = s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Delete(q.Name, &metav1.DeleteOptions{})
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
	done := make(chan bool)
	go func() {
		for next := range w.ResultChan() {
			app := *next.Object.(*appv1.Application)
			if q.Name == "" || q.Name == app.Name {
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
	repoRes, err := s.repoService.Get(ctx, &apirepository.RepoQuery{Repo: spec.Source.RepoURL})
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

	// Verify the specified environment is defined in it
	envSpec, ok := appSpec.Environments[spec.Source.Environment]
	if !ok || envSpec == nil {
		return status.Errorf(codes.InvalidArgument, "environment '%s' does not exist in app", spec.Source.Environment)
	}
	// Ensure the k8s cluster the app is referencing, is configured in ArgoCD
	// NOTE: need to check if it was overridden in the destination spec
	clusterURL := envSpec.Destination.Server
	if spec.Destination.Server != "" {
		clusterURL = spec.Destination.Server
	}
	_, err = s.clusterService.Get(ctx, &cluster.ClusterQuery{Server: clusterURL})
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) getApplicationClusterConfig(applicationName string) (*rest.Config, string, error) {
	server, namespace, err := s.getApplicationDestination(context.Background(), applicationName)
	if err != nil {
		return nil, "", err
	}
	clst, err := s.clusterService.Get(context.Background(), &cluster.ClusterQuery{Server: server})
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
	wrongPodError := fmt.Errorf("pod %s does not belong to application %s", podName, applicationName)
	if pod.Labels == nil {
		return wrongPodError
	}
	if value, ok := pod.Labels[common.LabelApplicationName]; !ok || value != applicationName {
		return wrongPodError
	}
	return nil
}

func (s *Server) DeletePod(ctx context.Context, q *DeletePodQuery) (*ApplicationResponse, error) {
	config, namespace, err := s.getApplicationClusterConfig(q.ApplicationName)
	if err != nil {
		return nil, err
	}
	kubeClientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	err = s.ensurePodBelongsToApp(q.ApplicationName, q.PodName, namespace, kubeClientset)
	if err != nil {
		return nil, err
	}
	err = kubeClientset.CoreV1().Pods(namespace).Delete(q.PodName, &metav1.DeleteOptions{})
	if err != nil {
		return nil, err
	}
	return &ApplicationResponse{}, nil
}

func (s *Server) PodLogs(q *PodLogsQuery, ws ApplicationService_PodLogsServer) error {
	config, namespace, err := s.getApplicationClusterConfig(q.ApplicationName)
	if err != nil {
		return err
	}
	kubeClientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	err = s.ensurePodBelongsToApp(q.ApplicationName, q.PodName, namespace, kubeClientset)
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
	stream, err := kubeClientset.CoreV1().Pods(namespace).GetLogs(q.PodName, &v1.PodLogOptions{
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
							TimeStamp: &metaLogTime,
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
	repo, err := s.repoService.Get(ctx, &apirepository.RepoQuery{Repo: repoURL})
	if err != nil {
		// If we couldn't retrieve from the repo service, assume public repositories
		repo = &appv1.Repository{Repo: repoURL}
	}
	return repo
}

// Sync syncs an application to its target state
func (s *Server) Sync(ctx context.Context, syncReq *ApplicationSyncRequest) (*appv1.Application, error) {
	return s.setAppOperation(ctx, syncReq.Name, func(app *appv1.Application) (*appv1.Operation, error) {
		return &appv1.Operation{
			Sync: &appv1.SyncOperation{
				NoOverrides: true,
				Revision:    syncReq.Revision,
				Prune:       syncReq.Prune,
				DryRun:      syncReq.DryRun,
			},
		}, nil
	})
}

func (s *Server) Rollback(ctx context.Context, rollbackReq *ApplicationRollbackRequest) (*appv1.Application, error) {
	return s.setAppOperation(ctx, rollbackReq.Name, func(app *appv1.Application) (*appv1.Operation, error) {

		var deploymentInfo *appv1.DeploymentInfo
		for _, info := range app.Status.RecentDeployments {
			if info.ID == rollbackReq.ID {
				deploymentInfo = &info
				break
			}
		}
		if deploymentInfo == nil {
			return nil, status.Errorf(codes.InvalidArgument, "application %s does not have deployment with id %v", rollbackReq.Name, rollbackReq.ID)
		}

		return &appv1.Operation{
			Sync: &appv1.SyncOperation{
				NoOverrides: false,
				Overrides:   deploymentInfo.ComponentParameterOverrides,
				Revision:    deploymentInfo.Revision,
				Prune:       rollbackReq.Prune,
				DryRun:      rollbackReq.DryRun,
			},
		}, nil
	})
}

func (s *Server) setAppOperation(ctx context.Context, appName string, operationCreator func(app *appv1.Application) (*appv1.Operation, error)) (*appv1.Application, error) {
	app, err := s.Get(ctx, &ApplicationQuery{Name: appName})
	if err != nil {
		return nil, err
	}
	if app.Operation != nil {
		return nil, status.Errorf(codes.InvalidArgument, "other operation is already in progress")
	}
	op, err := operationCreator(app)
	if err != nil {
		return nil, err
	}
	app.Operation = op
	app.Status.OperationState = nil
	_, err = s.Update(ctx, app)
	return app, err
}
