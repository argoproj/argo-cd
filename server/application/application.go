package application

import (
	"fmt"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/server/cluster"
	"github.com/argoproj/argo-cd/util/diff"
	"github.com/argoproj/argo-cd/util/kube"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Server provides a Application service
type Server struct {
	ns            string
	kubeclientset kubernetes.Interface
	appclientset  appclientset.Interface
	// TODO(jessesuen): move common cluster code to shared libraries
	clusterService cluster.ClusterServiceServer
}

// NewServer returns a new instance of the Application service
func NewServer(namespace string, kubeclientset kubernetes.Interface, appclientset appclientset.Interface, clusterService cluster.ClusterServiceServer) ApplicationServiceServer {
	return &Server{
		ns:             namespace,
		appclientset:   appclientset,
		kubeclientset:  kubeclientset,
		clusterService: clusterService,
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

// Delete updates a application
func (s *Server) Delete(ctx context.Context, q *ApplicationQuery) (*ApplicationResponse, error) {
	err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Delete(q.Name, &metav1.DeleteOptions{})
	return &ApplicationResponse{}, err
}

// ListPods returns pods in a application
func (s *Server) ListPods(ctx context.Context, q *ApplicationQuery) (*apiv1.PodList, error) {
	// TODO: filter by the app label
	return s.kubeclientset.CoreV1().Pods(s.ns).List(metav1.ListOptions{})
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

// Sync syncs an application to its target state
func (s *Server) Sync(ctx context.Context, syncReq *ApplicationSyncRequest) (*ApplicationSyncResult, error) {
	log.Infof("Syncing application %s", syncReq.Name)
	app, err := s.Get(ctx, &ApplicationQuery{Name: syncReq.Name})
	if err != nil {
		return nil, err
	}
	var syncRes ApplicationSyncResult
	switch app.Status.ComparisonResult.Status {
	case appv1.ComparisonStatusSynced:
	case appv1.ComparisonStatusOutOfSync:
	default:
		appState := app.Status.ComparisonResult.Status
		if appState == "" {
			appState = "Unknown"
		}
		return nil, fmt.Errorf("Cannot sync application '%s' while in an '%s' state", app.ObjectMeta.Name, appState)
	}
	clst, err := s.clusterService.Get(ctx, &cluster.ClusterQuery{Server: app.Status.ComparisonResult.Server})
	if err != nil {
		return nil, err
	}
	config := clst.RESTConfig()
	targetNamespace := app.Status.ComparisonResult.Namespace
	targetObjs, err := app.Status.ComparisonResult.TargetObjects()
	if err != nil {
		return nil, err
	}
	liveObjs, err := kube.GetLiveResources(config, targetObjs, targetNamespace)
	if err != nil {
		return nil, err
	}
	diffResList, err := diff.DiffArray(targetObjs, liveObjs)
	if err != nil {
		return nil, err
	}
	syncRes.Resources = make([]*ResourceDetails, 0)
	for i, diffRes := range diffResList.Diffs {
		resDetails := ResourceDetails{
			Name:      targetObjs[i].GetName(),
			Kind:      targetObjs[i].GetKind(),
			Namespace: targetNamespace,
		}
		needsCreate := bool(liveObjs[i] == nil)
		if !diffRes.Modified {
			resDetails.Message = fmt.Sprintf("already synced")
		} else if syncReq.DryRun {
			if needsCreate {
				resDetails.Message = fmt.Sprintf("will create")
			} else {
				resDetails.Message = fmt.Sprintf("will update")
			}
		} else {
			_, err := kube.ApplyResource(config, targetObjs[i], targetNamespace)
			if err != nil {
				return nil, err
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
	return &syncRes, nil
}
