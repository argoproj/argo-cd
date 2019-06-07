package application

import (
	"github.com/argoproj/argo-cd/util/http"
)

func init() {
	forward_ApplicationService_PodLogs_0 = http.StreamForwarder
	forward_ApplicationService_Watch_0 = http.StreamForwarder
	forward_ApplicationService_List_0 = http.UnaryForwarder
}
