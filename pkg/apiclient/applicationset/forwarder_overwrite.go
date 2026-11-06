package applicationset

import (
	"errors"

	"github.com/argoproj/pkg/v2/grpc/http"

	//nolint:staticcheck
	"github.com/golang/protobuf/proto"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func init() {
	forward_ApplicationSetService_Watch_0 = http.NewStreamForwarder(func(message proto.Message) (string, error) {
		event, ok := message.(*v1alpha1.ApplicationSetWatchEvent)
		if !ok {
			return "", errors.New("unexpected message type")
		}
		return event.ApplicationSet.Name, nil
	})
}
