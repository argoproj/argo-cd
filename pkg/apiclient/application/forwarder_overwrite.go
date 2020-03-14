package application

import (
	"github.com/argoproj/pkg/grpc/http"
)

func init() {
	forward_ApplicationService_PodLogs_0 = http.StreamForwarder
	forward_ApplicationService_Watch_0 = http.StreamForwarder
	forward_ApplicationService_List_0 = http.UnaryForwarder
	forward_ApplicationService_ManagedResources_0 = http.UnaryForwarder
}
