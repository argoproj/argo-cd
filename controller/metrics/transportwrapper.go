package metrics

import (
	"net/http"

	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

type metricsRoundTripper struct {
	roundTripper  http.RoundTripper
	app           *v1alpha1.Application
	metricsServer *MetricsServer
}

func (mrt *metricsRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := mrt.roundTripper.RoundTrip(r)
	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	}
	mrt.metricsServer.IncKubernetesRequest(mrt.app, statusCode)
	return resp, err
}

// AddMetricsTransportWrapper adds a transport wrapper which increments 'argocd_app_k8s_request_total' counter on each kubernetes request
func AddMetricsTransportWrapper(server *MetricsServer, app *v1alpha1.Application, config *rest.Config) *rest.Config {
	wrap := config.WrapTransport
	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		if wrap != nil {
			rt = wrap(rt)
		}
		return &metricsRoundTripper{roundTripper: rt, metricsServer: server, app: app}
	}
	return config
}
