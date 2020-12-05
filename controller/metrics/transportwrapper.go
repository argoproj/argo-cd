package metrics

import (
	"strconv"

	"github.com/argoproj/pkg/kubeclientmetrics"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

// AddMetricsTransportWrapper adds a transport wrapper which increments 'argocd_app_k8s_request_total' counter on each kubernetes request
func AddMetricsTransportWrapper(server *MetricsServer, app *v1alpha1.Application, config *rest.Config) *rest.Config {
	inc := func(resourceInfo kubeclientmetrics.ResourceInfo) error {
		namespace := resourceInfo.Namespace
		kind := resourceInfo.Kind
		statusCode := strconv.Itoa(resourceInfo.StatusCode)
		server.IncKubernetesRequest(app, resourceInfo.Server, statusCode, string(resourceInfo.Verb), kind, namespace)
		return nil
	}

	newConfig := kubeclientmetrics.AddMetricsTransportWrapper(config, inc)
	return newConfig
}
