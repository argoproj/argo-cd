package grpc

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	pkghttp "github.com/argoproj/pkg/v2/grpc/http"
	//nolint:staticcheck // grpc-gateway v1 forwarders take messages of the old proto package
	"github.com/golang/protobuf/proto"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// watchServer is a server stream that runs the given handler; the health service is only a convenient
// streaming service to register it as.
type watchServer struct {
	grpc_health_v1.UnimplementedHealthServer
	watch func(stream grpc.ServerStream) error
}

func (s *watchServer) Watch(_ *grpc_health_v1.HealthCheckRequest, stream grpc_health_v1.Health_WatchServer) error {
	return s.watch(stream)
}

// openWatch starts a stream handled by watch and returns the headers its client is given, as grpc-gateway reads
// them before it starts the HTTP response.
func openWatch(t *testing.T, watch func(stream grpc.ServerStream) error) metadata.MD {
	t.Helper()
	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	grpc_health_v1.RegisterHealthServer(server, &watchServer{watch: watch})
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)
	stream, err := grpc_health_v1.NewHealthClient(conn).Watch(ctx, &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)
	header, err := stream.Header()
	require.NoError(t, err, "the stream must start although it has sent no message")
	return header
}

func TestStartStream(t *testing.T) {
	t.Run("starts a stream that has nothing to send", func(t *testing.T) {
		header := openWatch(t, func(stream grpc.ServerStream) error {
			StartStream(stream)
			<-stream.Context().Done()
			return nil
		})
		assert.NotEmpty(t, header, "StartEventStream relies on a started stream having headers")
	})

	t.Run("keeps the headers that were already sent", func(t *testing.T) {
		header := openWatch(t, func(stream grpc.ServerStream) error {
			if err := stream.SendHeader(metadata.Pairs("renew-token", "token")); err != nil {
				return err
			}
			StartStream(stream)
			<-stream.Context().Done()
			return nil
		})
		assert.Equal(t, []string{"token"}, header.Get("renew-token"))
	})

	t.Run("a stream that fails before it starts has no headers", func(t *testing.T) {
		header := openWatch(t, func(_ grpc.ServerStream) error {
			return status.Error(codes.PermissionDenied, "permission denied")
		})
		assert.Empty(t, header, "StartEventStream relies on this to leave the HTTP status to the error")
	})
}

// statusRecorder records the statuses the forwarder sets on the response.
type statusRecorder struct {
	*httptest.ResponseRecorder
	statuses []int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statuses = append(r.statuses, statusCode)
	r.ResponseRecorder.WriteHeader(statusCode)
}

type forwardResult struct {
	response *http.Response
	body     string
	// the statuses that were set explicitly; writing to the response sets an implicit 200
	statuses []int
	// what had been written to the response when the forwarder asked for the first message
	bodyAtFirstReceive    string
	flushedAtFirstReceive bool
}

// forward runs the wrapped event stream forwarder over the messages and returns what it wrote.
func forward(t *testing.T, accept string, header metadata.MD, messages []proto.Message, streamErr error) forwardResult {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/stream/applications", http.NoBody)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	rec := &statusRecorder{ResponseRecorder: httptest.NewRecorder()}
	ctx := gwruntime.NewServerMetadataContext(req.Context(), gwruntime.ServerMetadata{HeaderMD: header})

	result := forwardResult{}
	received := 0
	recv := func() (proto.Message, error) {
		if received == 0 {
			result.bodyAtFirstReceive = rec.Body.String()
			result.flushedAtFirstReceive = rec.Flushed
		}
		received++
		if received <= len(messages) {
			return messages[received-1], nil
		}
		if streamErr != nil {
			return nil, streamErr
		}
		return nil, io.EOF
	}
	StartEventStream(pkghttp.StreamForwarder)(ctx, gwruntime.NewServeMux(), &JSONMarshaler{}, rec, req, recv)

	result.response = rec.Result()
	t.Cleanup(func() { _ = result.response.Body.Close() })
	result.body = rec.Body.String()
	result.statuses = rec.statuses
	return result
}

func TestStartEventStream(t *testing.T) {
	started := metadata.Pairs("content-type", "application/grpc")
	message := &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}

	t.Run("starts the response before the first message", func(t *testing.T) {
		result := forward(t, "text/event-stream", started, nil, nil)

		assert.Equal(t, ":\n", result.bodyAtFirstReceive)
		assert.True(t, result.flushedAtFirstReceive)
		assert.Equal(t, http.StatusOK, result.response.StatusCode)
		// The headers are sent with the first write, so they must all have been set by then.
		assert.Equal(t, "text/event-stream", result.response.Header.Get("Content-Type"))
		assert.Equal(t, "application/grpc", result.response.Header.Get("Grpc-Metadata-Content-Type"))
	})

	t.Run("forwards the messages after starting the response", func(t *testing.T) {
		result := forward(t, "text/event-stream", started, []proto.Message{message, message}, nil)

		assert.Equal(t, ":\n", result.bodyAtFirstReceive)
		assert.Equal(t, ":\ndata: {\"result\":{\"status\":1}} \n\n\ndata: {\"result\":{\"status\":1}} \n\n\n", result.body)
	})

	t.Run("leaves a response that is not an event stream alone", func(t *testing.T) {
		result := forward(t, "", started, []proto.Message{message}, nil)

		assert.Empty(t, result.bodyAtFirstReceive)
		assert.False(t, result.flushedAtFirstReceive)
		assert.Equal(t, "application/json", result.response.Header.Get("Content-Type"))
		assert.JSONEq(t, `{"result":{"status":1}}`, result.body)
	})

	t.Run("reports an error after the response started in the stream only", func(t *testing.T) {
		result := forward(t, "text/event-stream", started, nil, status.Error(codes.Unavailable, "stream closed"))

		assert.Equal(t, http.StatusOK, result.response.StatusCode)
		assert.Empty(t, result.statuses, "the response has its status; setting another one is logged by net/http as superfluous")
		assert.True(t, strings.HasPrefix(result.body, ":\ndata: {\"error\":"), "unexpected body: %s", result.body)
		assert.Contains(t, result.body, "stream closed")
	})

	t.Run("reports a stream that failed before it started with its HTTP status", func(t *testing.T) {
		result := forward(t, "text/event-stream", nil, nil, status.Error(codes.PermissionDenied, "permission denied"))

		assert.Empty(t, result.bodyAtFirstReceive)
		assert.False(t, result.flushedAtFirstReceive)
		assert.Equal(t, http.StatusForbidden, result.response.StatusCode)
		assert.Equal(t, []int{http.StatusForbidden}, result.statuses)
		assert.Contains(t, result.body, "permission denied")
	})
}
