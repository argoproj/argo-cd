package generators

import (
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

// Generator defines the interface implemented by all ApplicationSet generators.
type Generator interface {
	// GenerateParams interprets the ApplicationSet and generates all relevant parameters for the application template.
	// The expected / desired list of parameters is returned, it then will be render and reconciled
	// against the current state of the Applications in the cluster.
	GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, applicationSetInfo *argoprojiov1alpha1.ApplicationSet, client client.Client) ([]map[string]interface{}, error)

	// GetRequeueAfter is the generator can controller the next reconciled loop
	// In case there is more then one generator the time will be the minimum of the times.
	// In case NoRequeueAfter is empty, it will be ignored
	GetRequeueAfter(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) time.Duration

	// GetTemplate returns the inline template from the spec if there is any, or an empty object otherwise
	GetTemplate(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) *argoprojiov1alpha1.ApplicationSetTemplate
}

var (
	EmptyAppSetGeneratorError = fmt.Errorf("ApplicationSet is empty")
	NoRequeueAfter            time.Duration
)

// DefaultRequeueAfterSeconds is used when GetRequeueAfter is not specified, it is the default time to wait before the next reconcile loop
const (
	DefaultRequeueAfterSeconds = 3 * time.Minute
)
