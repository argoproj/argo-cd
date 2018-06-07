package controller

import (
	"context"
	"encoding/json"
	"fmt"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/kube"
	"k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	maxHistoryCnt = 5
)

type AppHealthManager interface {
	GetAppHealth(server string, namespace string, comparisonResult *appv1.ComparisonResult) (*appv1.HealthStatus, error)
}

type kubeAppHealthManager struct {
	db        db.ArgoDB
	namespace string
}

func NewAppHealthManager(db db.ArgoDB, namespace string) AppHealthManager {
	return &kubeAppHealthManager{db: db, namespace: namespace}
}

func (ctrl *kubeAppHealthManager) getServiceHealth(config *rest.Config, namespace string, name string) (*appv1.HealthStatus, error) {
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	service, err := clientSet.CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	health := appv1.HealthStatus{Status: appv1.HealthStatusHealthy}
	if service.Spec.Type == coreV1.ServiceTypeLoadBalancer {
		health.Status = appv1.HealthStatusProgressing
		for _, ingress := range service.Status.LoadBalancer.Ingress {
			if ingress.Hostname != "" || ingress.IP != "" {
				health.Status = appv1.HealthStatusHealthy
				break
			}
		}
	}
	return &health, nil
}

func (ctrl *kubeAppHealthManager) getDeploymentHealth(config *rest.Config, namespace string, name string) (*appv1.HealthStatus, error) {
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	deployment, err := clientSet.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if deployment.Generation <= deployment.Status.ObservedGeneration {
		cond := getDeploymentCondition(deployment.Status, v1.DeploymentProgressing)
		if cond != nil && cond.Reason == "ProgressDeadlineExceeded" {
			return &appv1.HealthStatus{
				Status:        appv1.HealthStatusDegraded,
				StatusDetails: fmt.Sprintf("Deployment %q exceeded its progress deadline", name),
			}, nil
		} else if deployment.Spec.Replicas != nil && deployment.Status.UpdatedReplicas < *deployment.Spec.Replicas {
			return &appv1.HealthStatus{
				Status:        appv1.HealthStatusProgressing,
				StatusDetails: fmt.Sprintf("Waiting for rollout to finish: %d out of %d new replicas have been updated...\n", deployment.Status.UpdatedReplicas, *deployment.Spec.Replicas),
			}, nil
		} else if deployment.Status.Replicas > deployment.Status.UpdatedReplicas {
			return &appv1.HealthStatus{
				Status:        appv1.HealthStatusProgressing,
				StatusDetails: fmt.Sprintf("Waiting for rollout to finish: %d old replicas are pending termination...\n", deployment.Status.Replicas-deployment.Status.UpdatedReplicas),
			}, nil
		} else if deployment.Status.AvailableReplicas < deployment.Status.UpdatedReplicas {
			return &appv1.HealthStatus{
				Status:        appv1.HealthStatusProgressing,
				StatusDetails: fmt.Sprintf("Waiting for rollout to finish: %d of %d updated replicas are available...\n", deployment.Status.AvailableReplicas, deployment.Status.UpdatedReplicas),
			}, nil
		}
	} else {
		return &appv1.HealthStatus{
			Status:        appv1.HealthStatusProgressing,
			StatusDetails: "Waiting for rollout to finish: observed deployment generation less then desired generation",
		}, nil
	}

	return &appv1.HealthStatus{
		Status: appv1.HealthStatusHealthy,
	}, nil
}

func getDeploymentCondition(status v1.DeploymentStatus, condType v1.DeploymentConditionType) *v1.DeploymentCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

func (ctrl *kubeAppHealthManager) GetAppHealth(server string, namespace string, comparisonResult *appv1.ComparisonResult) (*appv1.HealthStatus, error) {
	clst, err := ctrl.db.GetCluster(context.Background(), server)
	if err != nil {
		return nil, err
	}
	restConfig := clst.RESTConfig()

	appHealth := appv1.HealthStatus{Status: appv1.HealthStatusHealthy}
	for i := range comparisonResult.Resources {
		resource := comparisonResult.Resources[i]
		if resource.LiveState == "null" {
			resource.Health = appv1.HealthStatus{Status: appv1.HealthStatusUnknown}
		} else {
			var obj unstructured.Unstructured
			err := json.Unmarshal([]byte(resource.LiveState), &obj)
			if err != nil {
				return nil, err
			}
			switch obj.GetKind() {
			case kube.DeploymentKind:
				state, err := ctrl.getDeploymentHealth(restConfig, namespace, obj.GetName())
				if err != nil {
					return nil, err
				}
				resource.Health = *state
			case kube.ServiceKind:
				state, err := ctrl.getServiceHealth(restConfig, namespace, obj.GetName())
				if err != nil {
					return nil, err
				}
				resource.Health = *state
			default:
				resource.Health = appv1.HealthStatus{Status: appv1.HealthStatusHealthy}
			}

			if resource.Health.Status == appv1.HealthStatusProgressing {
				if appHealth.Status == appv1.HealthStatusHealthy {
					appHealth.Status = appv1.HealthStatusProgressing
				}
			} else if resource.Health.Status == appv1.HealthStatusDegraded {
				if appHealth.Status == appv1.HealthStatusHealthy || appHealth.Status == appv1.HealthStatusProgressing {
					appHealth.Status = appv1.HealthStatusDegraded
				}
			}
		}
		comparisonResult.Resources[i] = resource
	}
	return &appHealth, nil
}
