package application

import (
	"context"
	"errors"
	"fmt"
	gohttp "net/http"

	"github.com/argoproj/argo-cd/v2/util/kube"

	"github.com/argoproj/pkg/grpc/http"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"

	// nolint:staticcheck
	"github.com/golang/protobuf/proto"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func init() {
	logsForwarder := func(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w gohttp.ResponseWriter, req *gohttp.Request, recv func() (proto.Message, error), opts ...func(context.Context, gohttp.ResponseWriter, proto.Message) error) {
		if req.URL.Query().Get("download") == "true" {
			w.Header().Set("Content-Type", "application/octet-stream")
			fileName := "log"
			if container := req.URL.Query().Get("container"); len(container) > 0 && kube.IsValidResourceName(container) {
				fileName = container
			}
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment;filename="%s.txt"`, fileName))
			for {
				msg, err := recv()
				if err != nil {
					_, _ = w.Write([]byte(err.Error()))
					return
				}
				if logEntry, ok := msg.(*LogEntry); ok {
					if logEntry.Last {
						return
					}
					if _, err = w.Write([]byte(logEntry.Content + "\n")); err != nil {
						return
					}
				}
			}
		} else {
			http.StreamForwarder(ctx, mux, marshaler, w, req, recv, opts...)
		}
	}
	forward_ApplicationService_PodLogs_0 = logsForwarder
	forward_ApplicationService_PodLogs_1 = logsForwarder
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
