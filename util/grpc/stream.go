package grpc

import (
	"context"
	"net/http"
	"sync"

	//nolint:staticcheck // grpc-gateway v1 forwarders take messages of the old proto package
	"github.com/golang/protobuf/proto"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc"
)

// StreamForwarder is the signature of the function grpc-gateway calls to write a server stream to an HTTP response.
type StreamForwarder = func(
	ctx context.Context,
	mux *gwruntime.ServeMux,
	marshaler gwruntime.Marshaler,
	w http.ResponseWriter,
	req *http.Request,
	recv func() (proto.Message, error),
	opts ...func(context.Context, http.ResponseWriter, proto.Message) error,
)

// StartStream sends the response headers of a server stream now rather than with its first message.
//
// grpc-gateway waits for those headers before it starts the HTTP response. A stream that has nothing to send yet,
// such as a watch of resources that are not changing, otherwise leaves the HTTP request without any response,
// keepalives included, until something changes. Proxies answer such a request with a gateway timeout.
//
// Call it once the request has been validated: an error returned before any header is still reported with its
// own HTTP status.
func StartStream(stream grpc.ServerStream) {
	// The only failure is that the headers have been sent already, which is what this is after.
	_ = stream.SendHeader(nil)
}

// StartEventStream wraps a stream forwarder to start a server-sent events response as soon as the stream has
// started (see StartStream), without waiting for the first message. The browser then sees the stream open and
// can tell a stream with nothing to send from one the server never answered.
func StartEventStream(forward StreamForwarder) StreamForwarder {
	return func(
		ctx context.Context,
		mux *gwruntime.ServeMux,
		marshaler gwruntime.Marshaler,
		w http.ResponseWriter,
		req *http.Request,
		recv func() (proto.Message, error),
		opts ...func(context.Context, http.ResponseWriter, proto.Message) error,
	) {
		md, _ := gwruntime.ServerMetadataFromContext(ctx)
		// A stream without headers ended before it started, typically because the request was not permitted.
		// Leave the response to the forwarder, which reports that with an HTTP status.
		if req.Header.Get("Accept") != "text/event-stream" || len(md.HeaderMD) == 0 {
			forward(ctx, mux, marshaler, w, req, recv, opts...)
			return
		}
		stream := &eventStream{ResponseWriter: w}
		forward(ctx, mux, marshaler, stream, req, func() (proto.Message, error) {
			// The forwarder has set every response header by the time it asks for the first message.
			stream.start()
			return recv()
		}, opts...)
	}
}

// eventStream is the response of an event stream that starts before its first message.
type eventStream struct {
	http.ResponseWriter
	started bool
	// writing serializes the write that starts the response with the keepalives, which the forwarder writes
	// from another goroutine.
	writing sync.Mutex
}

// start sends the response headers followed by a comment, which is what event stream clients take a line
// starting with a colon for and ignore.
func (s *eventStream) start() {
	if s.started {
		return
	}
	s.started = true
	if _, err := s.Write([]byte(":\n")); err == nil {
		s.Flush()
	}
}

func (s *eventStream) Write(p []byte) (int, error) {
	s.writing.Lock()
	defer s.writing.Unlock()
	return s.ResponseWriter.Write(p)
}

// WriteHeader drops the status of an error that ends the stream after it started. The response has its status by
// then, and the forwarder writes the error to the stream as well.
func (s *eventStream) WriteHeader(statusCode int) {
	if !s.started {
		s.ResponseWriter.WriteHeader(statusCode)
	}
}

// Flush implements http.Flusher, which stream forwarders require of the response.
func (s *eventStream) Flush() {
	if flusher, ok := s.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Unwrap gives http.ResponseController access to the wrapped response.
func (s *eventStream) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}
