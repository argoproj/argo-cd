package application

import (
	"io"
	"net/http"

	"encoding/json"

	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"golang.org/x/net/context"
)

type SSEMarshaler struct {
}

func (m *SSEMarshaler) Marshal(v interface{}) ([]byte, error) {
	str, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("data: %s \n\n", str)), nil
}

func (m *SSEMarshaler) Unmarshal(data []byte, v interface{}) error {
	return nil
}

func (m *SSEMarshaler) NewDecoder(r io.Reader) runtime.Decoder {
	return nil
}

func (m *SSEMarshaler) NewEncoder(w io.Writer) runtime.Encoder {
	return nil
}

func (m *SSEMarshaler) ContentType() string {
	return "text/event-stream"
}

var (
	sseMarshaler SSEMarshaler
)

func init() {
	forward_ApplicationService_Watch_0 = func(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, req *http.Request, recv func() (proto.Message, error), opts ...func(context.Context, http.ResponseWriter, proto.Message) error) {
		if req.Header.Get("Accept") == "text/event-stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Transfer-Encoding", "chunked")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			runtime.ForwardResponseStream(ctx, mux, &sseMarshaler, w, req, recv, opts...)
		} else {
			runtime.ForwardResponseStream(ctx, mux, marshaler, w, req, recv, opts...)
		}
	}
}
