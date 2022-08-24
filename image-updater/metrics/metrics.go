package metrics

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// TODO: These should not be global vars with this package
var epm *EndpointMetrics
var apm *ApplicationMetrics
var cpm *ClientMetrics

// EndpointMetrics stores metrics for registry endpoints
type EndpointMetrics struct {
	requestsTotal  *prometheus.CounterVec
	requestsFailed *prometheus.CounterVec
}

// ApplicationMetrics stores metrics for applications
type ApplicationMetrics struct {
	applicationsTotal        prometheus.Gauge
	imagesWatchedTotal       *prometheus.GaugeVec
	imagesUpdatedTotal       *prometheus.CounterVec
	imagesUpdatedErrorsTotal *prometheus.CounterVec
}

// ClientMetrics stores metrics for K8s and ArgoCD clients
type ClientMetrics struct {
	argoCDRequestsTotal        *prometheus.CounterVec
	argoCDRequestsErrorsTotal  *prometheus.CounterVec
	kubeAPIRequestsTotal       prometheus.Counter
	kubeAPIRequestsErrorsTotal prometheus.Counter
}

// StartMetricsServer starts a new HTTP server for metrics on given port
func StartMetricsServer(port int) chan error {
	errCh := make(chan error)
	go func() {
		sm := http.NewServeMux()
		sm.Handle("/metrics", promhttp.Handler())
		errCh <- http.ListenAndServe(fmt.Sprintf(":%d", port), sm)
	}()
	return errCh
}

// NewEndpointMetrics returns a new endpoint metrics object
func NewEndpointMetrics() *EndpointMetrics {
	metrics := &EndpointMetrics{}

	metrics.requestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "argocd_image_updater_registry_requests_total",
		Help: "The total number of requests to this endpoint",
	}, []string{"registry"})
	metrics.requestsFailed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "argocd_image_updater_registry_requests_failed_total",
		Help: "The number of failed requests to this endpoint",
	}, []string{"registry"})

	return metrics
}

// NewApplicationsMetrics returns a new application metrics object
func NewApplicationsMetrics() *ApplicationMetrics {
	metrics := &ApplicationMetrics{}

	metrics.applicationsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "argocd_image_updater_applications_watched_total",
		Help: "The total number of applications watched by Argo CD Image Updater",
	})

	metrics.imagesWatchedTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "argocd_image_updater_images_watched_total",
		Help: "Number of images watched by Argo CD Image Updater",
	}, []string{"application"})

	metrics.imagesUpdatedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "argocd_image_updater_images_updated_total",
		Help: "Number of images updates by Argo CD Image Updater",
	}, []string{"application"})

	metrics.imagesUpdatedErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "argocd_image_updater_images_errors_total",
		Help: "Number of errors reported by Argo CD Image Updater",
	}, []string{"application"})

	return metrics
}

// NewClientMetrics returns a new client metrics object
func NewClientMetrics() *ClientMetrics {
	metrics := &ClientMetrics{}

	metrics.argoCDRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "argocd_image_updater_argocd_api_requests_total",
		Help: "The total number of Argo CD API requests performed by the Argo CD Image Updater",
	}, []string{"argocd_server"})

	metrics.argoCDRequestsErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "argocd_image_updater_argocd_api_errors_total",
		Help: "The total number of Argo CD API requests resulting in error",
	}, []string{"argocd_server"})

	metrics.kubeAPIRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "argocd_image_updater_k8s_api_requests_total",
		Help: "The total number of Argo CD API requests resulting in error",
	})

	metrics.kubeAPIRequestsErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "argocd_image_updater_k8s_api_errors_total",
		Help: "The total number of Argo CD API requests resulting in error",
	})

	return metrics
}

// Endpoint returns the global EndpointMetrics object
func Endpoint() *EndpointMetrics {
	return epm
}

// Applications returns the global ApplicationMetrics object
func Applications() *ApplicationMetrics {
	return apm
}

// Clients returns the global ClientMetrics object
func Clients() *ClientMetrics {
	return cpm
}

// IncreaseRequest increases the request counter of EndpointMetrics object
func (epm *EndpointMetrics) IncreaseRequest(registryURL string, isFailed bool) {
	epm.requestsTotal.WithLabelValues(registryURL).Inc()
	if isFailed {
		epm.requestsFailed.WithLabelValues(registryURL).Inc()
	}
}

// SetNumberOfApplications sets the total number of currently watched applications
func (apm *ApplicationMetrics) SetNumberOfApplications(num int) {
	apm.applicationsTotal.Set(float64(num))
}

// SetNumberOfImagesWatched sets the total number of currently watched images for given application
func (apm *ApplicationMetrics) SetNumberOfImagesWatched(application string, num int) {
	apm.imagesWatchedTotal.WithLabelValues(application).Set(float64(num))
}

// IncreaseImageUpdate increases the number of image updates for given application
func (apm *ApplicationMetrics) IncreaseImageUpdate(application string, by int) {
	apm.imagesUpdatedTotal.WithLabelValues(application).Add(float64(by))
}

// IncreaseUpdateErrors increases the number of errors for given application occured during update process
func (apm *ApplicationMetrics) IncreaseUpdateErrors(application string, by int) {
	apm.imagesUpdatedErrorsTotal.WithLabelValues(application).Add(float64(by))
}

// IncreaseArgoCDClientRequest increases the number of Argo CD API requests for given server
func (cpm *ClientMetrics) IncreaseArgoCDClientRequest(server string, by int) {
	cpm.argoCDRequestsTotal.WithLabelValues(server).Add(float64(by))
}

// IncreaseArgoCDClientError increases the number of failed Argo CD API requests for given server
func (cpm *ClientMetrics) IncreaseArgoCDClientError(server string, by int) {
	cpm.argoCDRequestsErrorsTotal.WithLabelValues(server).Add(float64(by))
}

// IncreaseK8sClientRequest increases the number of K8s API requests
func (cpm *ClientMetrics) IncreaseK8sClientRequest(by int) {
	cpm.kubeAPIRequestsTotal.Add(float64(by))
}

// IncreaseK8sClientRequest increases the number of failed K8s API requests
func (cpm *ClientMetrics) IncreaseK8sClientError(by int) {
	cpm.kubeAPIRequestsErrorsTotal.Add(float64(by))
}

// TODO: This is a lazy workaround, better initialize it somehwere else
func init() {
	epm = NewEndpointMetrics()
	apm = NewApplicationsMetrics()
	cpm = NewClientMetrics()
}
