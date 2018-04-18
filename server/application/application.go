package application

import (
	"fmt"

	"time"

	"bufio"
	"strings"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/server/cluster"
	apirepository "github.com/argoproj/argo-cd/server/repository"
	"github.com/argoproj/argo-cd/util"
	argoutil "github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/diff"
	"github.com/argoproj/argo-cd/util/kube"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	maxRecentDeploymentsCnt = 5
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
	}
}

// List returns list of applications
func (s *Server) List(ctx context.Context, q *ApplicationQuery) (*appv1.ApplicationList, error) {
	return s.appclientset.ArgoprojV1alpha1().Applications(s.ns).List(metav1.ListOptions{})
}

// Create creates a application
func (s *Server) Create(ctx context.Context, a *appv1.Application) (*appv1.Application, error) {
	return s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Create(a)
}

// Get returns a application by name
func (s *Server) Get(ctx context.Context, q *ApplicationQuery) (*appv1.Application, error) {
	return s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(q.Name, metav1.GetOptions{})
}

// Update updates a application
func (s *Server) Update(ctx context.Context, a *appv1.Application) (*appv1.Application, error) {
	return s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Update(a)
}

// Delete removes an application and all associated resources
func (s *Server) Delete(ctx context.Context, q *DeleteApplicationRequest) (*ApplicationResponse, error) {
	var err error
	server := q.Server
	namespace := q.Namespace
	if server == "" || namespace == "" {
		server, namespace, err = s.getApplicationDestination(ctx, q.Name)
		if err != nil && !apierr.IsNotFound(err) && !q.IgnoreResourceDeletionError {
			return nil, err
		}
	}

	if server != "" && namespace != "" {
		clst, err := s.clusterService.Get(ctx, &cluster.ClusterQuery{Server: server})
		if err != nil && !q.IgnoreResourceDeletionError {
			return nil, err
		}
		config := clst.RESTConfig()
		err = kube.DeleteResourceWithLabel(config, namespace, common.LabelApplicationName, q.Name)
		if err != nil && !q.IgnoreResourceDeletionError {
			return nil, err
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

// Sync syncs an application to its target state
func (s *Server) Sync(ctx context.Context, syncReq *ApplicationSyncRequest) (*ApplicationSyncResult, error) {
	log.Infof("Syncing application %s", syncReq.Name)
	app, err := s.Get(ctx, &ApplicationQuery{Name: syncReq.Name})
	if err != nil {
		return nil, err
	}
	revision := syncReq.Revision
	if revision == "" {
		app.Spec.Source.TargetRevision = revision
	}

	repo := s.getRepo(ctx, app.Spec.Source.RepoURL)
	conn, repoClient, err := s.repoClientset.NewRepositoryClient()
	if err != nil {
		return nil, err
	}
	defer util.Close(conn)
	// set fields in v1alpha/types.go
	log.Infof("Retrieving deployment params for application %s", syncReq.Name)
	envParams, err := repoClient.GetEnvParams(ctx, &repository.EnvParamsRequest{
		Repo:        repo,
		Environment: app.Spec.Source.Environment,
		Path:        app.Spec.Source.Path,
		Revision:    revision,
	})

	if err != nil {
		return nil, err
	}
	log.Infof("Received deployment params: %s", envParams.Params)

	res, manifest, err := s.deploy(ctx, app.Spec.Source, app.Spec.Destination, app.Name, syncReq.DryRun)
	if err == nil {
		// Persist app deployment info
		params := make([]appv1.ComponentParameter, len(envParams.Params))
		for i := range envParams.Params {
			param := *envParams.Params[i]
			params[i] = param
		}
		app, err = s.Get(ctx, &ApplicationQuery{Name: syncReq.Name})
		if err != nil {
			return nil, err
		}
		app.Status.RecentDeployments = append(app.Status.RecentDeployments, appv1.DeploymentInfo{
			ComponentParameterOverrides: app.Spec.Source.ComponentParameterOverrides,
			Revision:                    manifest.Revision,
			Params:                      params,
			DeployedAt:                  metav1.NewTime(time.Now()),
		})
		if len(app.Status.RecentDeployments) > maxRecentDeploymentsCnt {
			app.Status.RecentDeployments = app.Status.RecentDeployments[:maxRecentDeploymentsCnt]
		}
		_, err = s.Update(ctx, app)
		if err != nil {
			return nil, err
		}
	}
	return res, err
}

func (s *Server) getApplicationDestination(ctx context.Context, name string) (string, string, error) {
	app, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Get(name, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	} else {
		if app.Spec.Destination != nil {
			return app.Spec.Destination.Server, app.Spec.Destination.Namespace, nil
		} else {
			repo := s.getRepo(ctx, app.Spec.Source.RepoURL)
			conn, repoClient, err := s.repoClientset.NewRepositoryClient()
			if err != nil {
				return "", "", err
			}
			defer util.Close(conn)
			manifestInfo, err := repoClient.GenerateManifest(ctx, &repository.ManifestRequest{
				Repo:        repo,
				Environment: app.Spec.Source.Environment,
				Path:        app.Spec.Source.Path,
				Revision:    app.Spec.Source.TargetRevision,
				AppLabel:    app.Name,
			})
			if err != nil {
				return "", "", err
			}
			return manifestInfo.Server, manifestInfo.Namespace, nil
		}
	}
}

func (s *Server) getRepo(ctx context.Context, repoURL string) *appv1.Repository {
	repo, err := s.repoService.Get(ctx, &apirepository.RepoQuery{Repo: repoURL})
	if err != nil {
		// If we couldn't retrieve from the repo service, assume public repositories
		repo = &appv1.Repository{Repo: repoURL}
	}
	return repo
}

func (s *Server) deploy(
	ctx context.Context,
	source appv1.ApplicationSource,
	destination *appv1.ApplicationDestination,
	appLabel string,
	dryRun bool) (*ApplicationSyncResult, *repository.ManifestResponse, error) {

	repo := s.getRepo(ctx, source.RepoURL)
	conn, repoClient, err := s.repoClientset.NewRepositoryClient()
	if err != nil {
		return nil, nil, err
	}
	defer util.Close(conn)
	overrides := make([]*appv1.ComponentParameter, len(source.ComponentParameterOverrides))
	if source.ComponentParameterOverrides != nil {
		for i := range source.ComponentParameterOverrides {
			item := source.ComponentParameterOverrides[i]
			overrides[i] = &item
		}
	}

	manifestInfo, err := repoClient.GenerateManifest(ctx, &repository.ManifestRequest{
		Repo:                        repo,
		Environment:                 source.Environment,
		Path:                        source.Path,
		Revision:                    source.TargetRevision,
		ComponentParameterOverrides: overrides,
		AppLabel:                    appLabel,
	})
	if err != nil {
		return nil, nil, err
	}
	server, namespace := argoutil.ResolveServerNamespace(destination, manifestInfo)

	clst, err := s.clusterService.Get(ctx, &cluster.ClusterQuery{Server: server})
	if err != nil {
		return nil, nil, err
	}
	config := clst.RESTConfig()

	targetObjs := make([]*unstructured.Unstructured, len(manifestInfo.Manifests))
	for i, manifest := range manifestInfo.Manifests {
		obj, err := appv1.UnmarshalToUnstructured(manifest)
		if err != nil {
			return nil, nil, err
		}
		targetObjs[i] = obj
	}

	liveObjs, err := kube.GetLiveResources(config, targetObjs, namespace)
	if err != nil {
		return nil, nil, err
	}
	diffResList, err := diff.DiffArray(targetObjs, liveObjs)
	if err != nil {
		return nil, nil, err
	}
	var syncRes ApplicationSyncResult
	syncRes.Resources = make([]*ResourceDetails, 0)
	for i, diffRes := range diffResList.Diffs {
		resDetails := ResourceDetails{
			Name:      targetObjs[i].GetName(),
			Kind:      targetObjs[i].GetKind(),
			Namespace: namespace,
		}
		needsCreate := bool(liveObjs[i] == nil)
		if !diffRes.Modified {
			resDetails.Message = fmt.Sprintf("already synced")
		} else if dryRun {
			if needsCreate {
				resDetails.Message = fmt.Sprintf("will create")
			} else {
				resDetails.Message = fmt.Sprintf("will update")
			}
		} else {
			_, err := kube.ApplyResource(config, targetObjs[i], namespace)
			if err != nil {
				return nil, nil, err
			}
			if needsCreate {
				resDetails.Message = fmt.Sprintf("created")
			} else {
				resDetails.Message = fmt.Sprintf("updated")
			}
		}
		syncRes.Resources = append(syncRes.Resources, &resDetails)
	}
	syncRes.Message = "successfully synced"
	return &syncRes, manifestInfo, nil
}
