package health

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	hookutil "github.com/argoproj/argo-cd/util/hook"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/resource/ignore"
)

// SetApplicationHealth updates the health statuses of all resources performed in the comparison
func SetApplicationHealth(resStatuses []appv1.ResourceStatus, liveObjs []*unstructured.Unstructured, resourceOverrides map[string]appv1.ResourceOverride, filter func(obj *unstructured.Unstructured) bool) (*appv1.HealthStatus, error) {
	var savedErr error
	appHealth := appv1.HealthStatus{Status: appv1.HealthStatusHealthy}
	for i, liveObj := range liveObjs {
		var resHealth *appv1.HealthStatus
		var err error
		if liveObj == nil {
			resHealth = &appv1.HealthStatus{Status: appv1.HealthStatusMissing}
		} else {
			if filter(liveObj) {
				resHealth, err = GetResourceHealth(liveObj, resourceOverrides)
				if err != nil && savedErr == nil {
					savedErr = err
				}
			}
		}
		if resHealth != nil {
			resStatuses[i].Health = resHealth
			ignore := ignoreLiveObjectHealth(liveObj, *resHealth)
			if !ignore && IsWorse(appHealth.Status, resHealth.Status) {
				appHealth.Status = resHealth.Status
			}
		}
	}
	return &appHealth, savedErr
}

// ignoreLiveObjectHealth determines if we should not allow the live object to affect the overall
// health of the application (e.g. hooks, missing child applications)
func ignoreLiveObjectHealth(liveObj *unstructured.Unstructured, resHealth appv1.HealthStatus) bool {
	if liveObj != nil {
		if hookutil.IsHook(liveObj) {
			// Don't allow resource hooks to affect health status
			return true
		}
		if ignore.Ignore(liveObj) {
			return true
		}
		gvk := liveObj.GroupVersionKind()
		if gvk.Group == "argoproj.io" && gvk.Kind == "Application" && (resHealth.Status == appv1.HealthStatusMissing || resHealth.Status == appv1.HealthStatusUnknown) {
			// Covers the app-of-apps corner case where child app is deployed but that app itself
			// has a status of 'Missing', which we don't want to cause the parent's health status
			// to also be Missing
			return true
		}
	}
	return false
}

// GetResourceHealth returns the health of a k8s resource
func GetResourceHealth(obj *unstructured.Unstructured, resourceOverrides map[string]appv1.ResourceOverride) (*appv1.HealthStatus, error) {
	if obj.GetDeletionTimestamp() != nil {
		return &appv1.HealthStatus{Status: appv1.HealthStatusProgressing, Message: "Pending deletion"}, nil
	}
	health, err := getResourceHealthFromLuaScript(obj, resourceOverrides)
	if err != nil {
		return &appv1.HealthStatus{Status: appv1.HealthStatusUnknown, Message: err.Error()}, err
	}
	if health != nil {
		return health, nil
	}
	health, err = getHealthFromResource(obj)
	if err != nil {
		return &appv1.HealthStatus{Status: appv1.HealthStatusUnknown, Message: err.Error()}, err
	}
	if health != nil {
		return health, nil
	}
	return &appv1.HealthStatus{Status: appv1.HealthStatusUnknown}, nil
}

func getHealthFromResource(obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	gvk := obj.GroupVersionKind()
	switch gvk.Group {
	case "apps", "extensions":
		switch gvk.Kind {
		case kube.DeploymentKind:
			return getDeploymentHealth(obj)
		case kube.IngressKind:
			return getIngressHealth(obj)
		case kube.StatefulSetKind:
			return getStatefulSetHealth(obj)
		case kube.ReplicaSetKind:
			return getReplicaSetHealth(obj)
		case kube.DaemonSetKind:
			return getDaemonSetHealth(obj)
		}
	case "argoproj.io":
		switch gvk.Kind {
		case "Application":
			return getApplicationHealth(obj)
		}
	case "apiregistration.k8s.io":
		switch gvk.Kind {
		case kube.APIServiceKind:
			return getAPIServiceHealth(obj)
		}
	case "":
		switch gvk.Kind {
		case kube.ServiceKind:
			return getServiceHealth(obj)
		case kube.PersistentVolumeClaimKind:
			return getPVCHealth(obj)
		case kube.PodKind:
			return getPodHealth(obj)
		}
	case "batch":
		switch gvk.Kind {
		case kube.JobKind:
			return getJobHealth(obj)
		}
	}

	return getGenericHealth(obj)
}

// healthOrder is a list of health codes in order of most healthy to least healthy
var healthOrder = []appv1.HealthStatusCode{
	appv1.HealthStatusHealthy,
	appv1.HealthStatusSuspended,
	appv1.HealthStatusProgressing,
	appv1.HealthStatusDegraded,
	appv1.HealthStatusMissing,
	appv1.HealthStatusUnknown,
}

// IsWorse returns whether or not the new health status code is a worser condition than the current
func IsWorse(current, new appv1.HealthStatusCode) bool {
	currentIndex := 0
	newIndex := 0
	for i, code := range healthOrder {
		if current == code {
			currentIndex = i
		}
		if new == code {
			newIndex = i
		}
	}
	return newIndex > currentIndex
}
func init() {
	_ = appv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	_ = apiregistrationv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	_ = apiregistrationv1beta1.SchemeBuilder.AddToScheme(scheme.Scheme)
}
