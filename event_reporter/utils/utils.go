package utils

import (
	"fmt"

	"github.com/argoproj/gitops-engine/pkg/health"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func SetHealthStatusIfMissing(rs *appv1.ResourceStatus) {
	if rs.Health == nil && rs.Status == appv1.SyncStatusCodeSynced {
		// for resources without health status we need to add 'Healthy' status
		// when they are synced because we might have sent an event with 'Missing'
		// status earlier and they would be stuck in it if we don't switch to 'Healthy'
		rs.Health = &appv1.HealthStatus{
			Status: health.HealthStatusHealthy,
		}
	}
}

func IsApp(rs appv1.ResourceStatus) bool {
	return rs.GroupVersionKind().String() == appv1.ApplicationSchemaGroupVersionKind.String()
}

func LogWithAppStatus(a *appv1.Application, logCtx *log.Entry, ts string) *log.Entry {
	return logCtx.WithFields(log.Fields{
		"sync":            a.Status.Sync.Status,
		"health":          a.Status.Health.Status,
		"resourceVersion": a.ResourceVersion,
		"ts":              ts,
	})
}

func LogWithResourceStatus(logCtx *log.Entry, rs appv1.ResourceStatus) *log.Entry {
	logCtx = logCtx.WithField("sync", rs.Status)
	if rs.Health != nil {
		logCtx = logCtx.WithField("health", rs.Health.Status)
	}

	return logCtx
}

func SafeString(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func GetAppAsResource(a *appv1.Application) *appv1.ResourceStatus {
	return &appv1.ResourceStatus{
		Name:            a.Name,
		Namespace:       a.Namespace,
		Version:         "v1alpha1",
		Kind:            "Application",
		Group:           "argoproj.io",
		Status:          a.Status.Sync.Status,
		Health:          &a.Status.Health,
		RequiresPruning: a.DeletionTimestamp != nil,
	}
}

func AddDestNamespaceToManifest(resourceManifest []byte, rs *appv1.ResourceStatus) (*unstructured.Unstructured, error) {
	u, err := appv1.UnmarshalToUnstructured(string(resourceManifest))
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	if u.GetNamespace() == rs.Namespace {
		return u, nil
	}

	// need to change namespace
	u.SetNamespace(rs.Namespace)

	return u, nil
}
