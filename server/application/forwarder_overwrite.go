package application

import (
	"net/http"

	"github.com/golang/protobuf/proto"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"golang.org/x/net/context"
)

func init() {
	forward_ApplicationService_Watch_0 = func(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, req *http.Request, recv func() (proto.Message, error), opts ...func(context.Context, http.ResponseWriter, proto.Message) error) {
		opts = append(opts, func(i context.Context, writer http.ResponseWriter, message proto.Message) error {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Transfer-Encoding", "chunked")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			return nil
		})
		runtime.ForwardResponseStream(ctx, mux, marshaler, w, req, recv, opts...)
	}
}
