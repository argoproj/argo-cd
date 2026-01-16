package controller

import (
	"fmt"
	"strings"

	"github.com/argoproj/gitops-engine/pkg/health"
	hookutil "github.com/argoproj/gitops-engine/pkg/sync/hook"
	"github.com/argoproj/gitops-engine/pkg/sync/ignore"
	kubeutil "github.com/argoproj/gitops-engine/pkg/utils/kube"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	applog "github.com/argoproj/argo-cd/v3/util/app/log"
	"github.com/argoproj/argo-cd/v3/util/lua"
)

// setApplicationHealth updates the health statuses of all resources performed in the comparison
func setApplicationHealth(resources []managedResource, statuses []appv1.ResourceStatus, resourceOverrides map[string]appv1.ResourceOverride, app *appv1.Application, persistResourceHealth bool) (health.HealthStatusCode, error) {
	var savedErr error
	var errCount uint
	var containsResources, containsLiveResources bool

	appHealthStatus := health.HealthStatusHealthy
	for i, res := range resources {
		if res.Target != nil && hookutil.Skip(res.Target) {
			continue
		}
		if res.Live != nil && (hookutil.IsHook(res.Live) || ignore.Ignore(res.Live)) {
			continue
		}

		// Contains actual resources that are not hooks
		containsResources = true
		if res.Live != nil {
			containsLiveResources = true
		}

		var healthStatus *health.HealthStatus
		var err error
		healthOverrides := lua.ResourceHealthOverrides(resourceOverrides)
		if res.Live == nil {
			healthStatus = &health.HealthStatus{Status: health.HealthStatusMissing}
		} else {
			// App that manages itself should not affect own health
			if isSelfReferencedApp(app, kubeutil.GetObjectRef(res.Live)) {
				continue
			}
			healthStatus, err = health.GetResourceHealth(res.Live, healthOverrides)
			if err != nil {
				errCount++
				savedErr = fmt.Errorf("failed to get resource health for %q with name %q in namespace %q: %w", res.Live.GetKind(), res.Live.GetName(), res.Live.GetNamespace(), err)
				log.WithFields(applog.GetAppLogFields(app)).Warn(savedErr)
			}
		}

		if healthStatus == nil {
			continue
		}

		if persistResourceHealth {
			resHealth := appv1.HealthStatus{Status: healthStatus.Status, Message: healthStatus.Message}
			statuses[i].Health = &resHealth
		} else {
			statuses[i].Health = nil
		}

		// Missing resources should not affect parent app health - the OutOfSync status already indicates resources are missing
		if res.Live == nil && healthStatus.Status == health.HealthStatusMissing {
			continue
		}

		// Missing or Unknown health status of child Argo CD app should not affect parent
		if res.Kind == application.ApplicationKind && res.Group == application.Group && (healthStatus.Status == health.HealthStatusMissing || healthStatus.Status == health.HealthStatusUnknown) {
			continue
		}

		// Use AggregateAs status if specified, otherwise use the resource status
		aggregationStatus := healthStatus.AggregateAs
		if aggregationStatus == "" {
			aggregationStatus = healthStatus.Status
		}

		// Check if resource has annotation override
		if res.Live != nil && res.Live.GetAnnotations() != nil {
			// Ignore check from aggregation if annotation is set
			if res.Live.GetAnnotations()[common.AnnotationIgnoreHealthCheck] == "true" {
				continue
			}
			// Apply health aggregate overrides if annotation is set
			if overrideStr, ok := res.Live.GetAnnotations()[common.AnnotationHealthAggregateOverrides]; ok {
				overrideMap, err := parseHealthAggregateOverrides(overrideStr)
				if err != nil {
					errCount++
					savedErr = fmt.Errorf("failed to parse health aggregate overrides annotation for %q with name %q in namespace %q: %w", res.Live.GetKind(), res.Live.GetName(), res.Live.GetNamespace(), err)
					log.WithFields(applog.GetAppLogFields(app)).Warn(savedErr)
					continue
				}
				// Apply the overrides to the current resource health status
				if mapped, ok := overrideMap[string(healthStatus.Status)]; ok {
					aggregationStatus = mapped
				}
			}
		}

		if health.IsWorse(appHealthStatus, aggregationStatus) {
			appHealthStatus = aggregationStatus
		}
	}

	// If the app is expected to have resources but does not contain any live resources, set the app health to missing
	if containsResources && !containsLiveResources && health.IsWorse(appHealthStatus, health.HealthStatusMissing) {
		appHealthStatus = health.HealthStatusMissing
	}

	if persistResourceHealth {
		app.Status.ResourceHealthSource = appv1.ResourceHealthLocationInline
	} else {
		app.Status.ResourceHealthSource = appv1.ResourceHealthLocationAppTree
	}
	if savedErr != nil && errCount > 1 {
		savedErr = fmt.Errorf("see application-controller logs for %d other errors; most recent error was: %w", errCount-1, savedErr)
	}
	return appHealthStatus, savedErr
}

// parseHealthAggregateOverrides parses the health aggregate overrides annotation
// Format: "SourceStatus=TargetStatus" or "Status1=Target1,Status2=Target2"
// Returns a map of source status to target status
func parseHealthAggregateOverrides(overrideStr string) (map[string]health.HealthStatusCode, error) {
	result := make(map[string]health.HealthStatusCode)

	if overrideStr == "" {
		return result, nil
	}

	// Split by comma for multiple mappings
	mappings := strings.Split(overrideStr, ",")

	for _, mapping := range mappings {
		mapping = strings.TrimSpace(mapping)
		if mapping == "" {
			continue
		}

		// Split by equals sign
		parts := strings.Split(mapping, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid mapping format: %s (expected SourceStatus=TargetStatus)", mapping)
		}

		sourceStatus := strings.TrimSpace(parts[0])
		targetStatus := strings.TrimSpace(parts[1])

		if sourceStatus == "" || targetStatus == "" {
			return nil, fmt.Errorf("invalid mapping format: %s (source and target cannot be empty)", mapping)
		}

		result[sourceStatus] = health.HealthStatusCode(targetStatus)
	}

	return result, nil
}
