package apiclient

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/env"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	grpc_util "github.com/argoproj/argo-cd/v2/util/grpc"
	"github.com/argoproj/argo-cd/v2/util/io"
)

// MaxGRPCMessageSize contains max grpc message size
var MaxGRPCMessageSize = env.ParseNumFromEnv(common.EnvGRPCMaxSizeMB, 100, 0, math.MaxInt32) * 1024 * 1024

// Clientset represents config management plugin server api clients
type Clientset interface {
	NewConfigManagementPluginClient() (io.Closer, ConfigManagementPluginServiceClient, error)
}

type ClientType int

const (
	Sidecar ClientType = iota
	Service
)

func (ct *ClientType) addrType() string {
	switch *ct {
	case Sidecar:
		return "unix"
	case Service:
		return "tcp"
	default:
		return ""
	}
}

func (ct *ClientType) String() string {
	switch *ct {
	case Sidecar:
		return "sidecar"
	case Service:
		return "service"
	default:
		return "unknown"
	}
}

type clientSet struct {
	address    string
	secretPath string
	clientType ClientType
}

func (c *clientSet) addrType() string {
	return c.clientType.addrType()
}

func (c *clientSet) NewConfigManagementPluginClient() (io.Closer, ConfigManagementPluginServiceClient, error) {
	conn, err := c.newConnection()
	if err != nil {
		return nil, nil, err
	}
	return conn, NewConfigManagementPluginServiceClient(conn), nil
}

func (c *clientSet) newConnection() (*grpc.ClientConn, error) {
	retryOpts := []grpc_retry.CallOption{
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffLinear(1000 * time.Millisecond)),
	}
	unaryInterceptors := []grpc.UnaryClientInterceptor{
		grpc_retry.UnaryClientInterceptor(retryOpts...),
		grpc_util.OTELUnaryClientInterceptor(),
	}
	streamInterceptors := []grpc.StreamClientInterceptor{
		grpc_retry.StreamClientInterceptor(retryOpts...),
		grpc_util.OTELStreamClientInterceptor(),
		c.authStreamInterceptor,
	}
	dialOpts := []grpc.DialOption{
		grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(unaryInterceptors...)),
		grpc.WithStreamInterceptor(grpc_middleware.ChainStreamClient(streamInterceptors...)),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxGRPCMessageSize), grpc.MaxCallSendMsgSize(MaxGRPCMessageSize)),
	}

	dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	conn, err := grpc_util.BlockingDial(context.Background(), c.addrType(), c.address, nil, dialOpts...)
	if err != nil {
		log.Errorf("Unable to connect to config management plugin with address %s (type %s)", c.address, c.clientType.String())
		return nil, err
	}
	return conn, nil
}

// NewConfigManagementPluginClientSet creates new instance of config management plugin server Clientset
func NewConfigManagementPluginClientSet(address string, secretPath string, clientType ClientType) Clientset {
	return &clientSet{address: address, secretPath: secretPath, clientType: clientType}
}

// wrappedStream  wraps around the embedded grpc.ClientStream, and intercepts the RecvMsg and
// SendMsg method call.
type wrappedStream struct {
	c *clientSet
	grpc.ClientStream
}

func (w *wrappedStream) RecvMsg(m any) error {
	err := w.ClientStream.RecvMsg(m)
	if err != nil {
		return err
	}
	header, err := w.Header()
	if err != nil {
		return err
	}
	return w.c.authenticate(header[common.PluginAuthTokenHeader])
}

func (c *clientSet) newWrappedStream(s grpc.ClientStream) grpc.ClientStream {
	return &wrappedStream{ClientStream: s, c: c}
}

func (c *clientSet) authStreamInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	s, err := streamer(ctx, desc, cc, method, opts...)
	if err != nil {
		return nil, err
	}
	return c.newWrappedStream(s), nil
}

func (c *clientSet) authenticate(authSecret []string) error {
	if authSecret == nil {
		return status.Errorf(codes.InvalidArgument, "no authtoken header in rpc context")
	}

	switch c.clientType {
	case Sidecar:
		// Sidecars are trusted
		return nil
	case Service:
		secret, err := c.readAuthSecret(common.PluginAuthSecretsPath)
		if err != nil {
			return err
		}
		if secret != authSecret[0] {
			return status.Errorf(codes.Unauthenticated, "Client secret doesn't match")
		}
		return nil
	default:
		return status.Errorf(codes.Unauthenticated, "Unknown client type %d attempting authentication", c.clientType)
	}
}

func (c *clientSet) readAuthSecret(root string) (string, error) {
	tryPath := c.secretPath
	for {
		path := fmt.Sprintf(filepath.Join(root, tryPath, common.PluginAuthSecretName))
		content, err := os.ReadFile(path)
		if err == nil {
			return strings.TrimSpace(string(content)), nil
		}
		// If we've just tried the root, give up
		if tryPath == `.` {
			break
		}
		tryPath = filepath.Dir(tryPath)
	}
	return ``, status.Errorf(codes.Unauthenticated, "No authentication secret present at %s or parents", filepath.Join(root, c.secretPath))
}
