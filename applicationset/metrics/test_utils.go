package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

// Initialize metrics from fake client and clear registry before each test run
func FakeAppsetMetrics(client ctrlclient.WithWatch) ApplicationsetMetrics {
	metrics.Registry = prometheus.NewRegistry()
	return NewApplicationsetMetrics(
		utils.NewAppsetLister(client),
		[]string{},
		func(appset *argoappv1.ApplicationSet) bool {
			return true
		})
}
