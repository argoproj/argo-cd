package metrics

import (
	"strconv"

	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

// AddAppMetricsTransportWrapper adds a transport wrapper which increments 'argocd_app_k8s_request_total' counter on each kubernetes request
func AddAppMetricsTransportWrapper(server *MetricsServer, app *v1alpha1.Application, config *rest.Config) *rest.Config {
	inc := func(resourceInfo ResourceInfo) error {
		namespace := resourceInfo.Namespace
		kind := resourceInfo.Kind
		statusCode := strconv.Itoa(resourceInfo.StatusCode)
		if resourceInfo.Verb == Unknown {
			namespace = "Unknown"
			kind = "Unknown"
		}
		server.IncKubernetesRequest(app, statusCode, string(resourceInfo.Verb), kind, namespace)
		return nil
	}

	newConfig := AddMetricsTransportWrapper(config, inc)
	return newConfig
}
