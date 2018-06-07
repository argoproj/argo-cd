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
	deploy, err := clientSet.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	health := appv1.HealthStatus{
		Status: appv1.HealthStatusUnknown,
	}

	isProgressing := true
	notProgressingReason := ""
	for _, condition := range deploy.Status.Conditions {
		if condition.Type == v1.DeploymentProgressing && condition.Status == "False" {
			isProgressing = false
			notProgressingReason = fmt.Sprintf("%s:%s", condition.Reason, condition.Message)
		}
	}

	if !isProgressing {
		health.Status = appv1.HealthStatusDegraded
		health.StatusDetails = notProgressingReason
	} else if deploy.Status.UnavailableReplicas > 0 {
		health.Status = appv1.HealthStatusDegraded
		health.StatusDetails = fmt.Sprintf("Deployment has %v unavailable replicas", deploy.Status.UnavailableReplicas)
	} else {
		health.Status = appv1.HealthStatusHealthy
		health.StatusDetails = ""
	}

	return &health, nil
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
