package project

import (
	"context"
	gohttp "net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	googproto "google.golang.org/protobuf/proto"
)

// unaryForwarder is a simple unary forwarder
func unaryForwarder(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w gohttp.ResponseWriter, req *gohttp.Request, resp googproto.Message, opts ...func(context.Context, gohttp.ResponseWriter, googproto.Message) error) {
	runtime.ForwardResponseMessage(ctx, mux, marshaler, w, req, resp, opts...)
}

func init() {
	forward_ProjectService_List_0 = unaryForwarder
}
