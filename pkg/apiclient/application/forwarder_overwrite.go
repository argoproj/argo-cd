package application

import (
	"errors"

	// nolint:staticcheck
	"github.com/golang/protobuf/proto"

	"github.com/argoproj/pkg/grpc/http"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func init() {
	forward_ApplicationService_PodLogs_0 = http.StreamForwarder
	forward_ApplicationService_PodLogs_1 = http.StreamForwarder
	forward_ApplicationService_WatchResourceTree_0 = http.StreamForwarder
	forward_ApplicationService_Watch_0 = http.NewStreamForwarder(func(message proto.Message) (string, error) {
		event, ok := message.(*v1alpha1.ApplicationWatchEvent)
		if !ok {
			return "", errors.New("unexpected message type")
		}
		return event.Application.Name, nil
	})
	forward_ApplicationService_List_0 = http.UnaryForwarder
	forward_ApplicationService_ManagedResources_0 = http.UnaryForwarder
}
