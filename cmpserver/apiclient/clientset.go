package apiclient

import (
	"context"
	"math"
	"time"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/env"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpc_util "github.com/argoproj/argo-cd/v2/util/grpc"
	"github.com/argoproj/argo-cd/v2/util/io"
)

// MaxGRPCMessageSize contains max grpc message size
var MaxGRPCMessageSize = env.ParseNumFromEnv(common.EnvGRPCMaxSizeMB, 100, 0, math.MaxInt32) * 1024 * 1024

// Clientset represents config management plugin server api clients
type Clientset interface {
	NewConfigManagementPluginClient() (io.Closer, ConfigManagementPluginServiceClient, error)
}

type clientSet struct {
	address string
}

func (c *clientSet) NewConfigManagementPluginClient() (io.Closer, ConfigManagementPluginServiceClient, error) {
	conn, err := NewConnection(c.address)
	if err != nil {
		return nil, nil, err
	}
	return conn, NewConfigManagementPluginServiceClient(conn), nil
}

func NewConnection(address string) (*grpc.ClientConn, error) {
	retryOpts := []grpc_retry.CallOption{
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffLinear(1000 * time.Millisecond)),
	}
	unaryInterceptors := []grpc.UnaryClientInterceptor{grpc_retry.UnaryClientInterceptor(retryOpts...)}
	dialOpts := []grpc.DialOption{
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(retryOpts...)),
		grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(unaryInterceptors...)),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxGRPCMessageSize), grpc.MaxCallSendMsgSize(MaxGRPCMessageSize)),
		grpc.WithUnaryInterceptor(grpc_util.OTELUnaryClientInterceptor()),
		grpc.WithStreamInterceptor(grpc_util.OTELStreamClientInterceptor()),
	}

	dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	conn, err := grpc_util.BlockingDial(context.Background(), "unix", address, nil, dialOpts...)
	if err != nil {
		log.Errorf("Unable to connect to config management plugin service with address %s", address)
		return nil, err
	}
	return conn, nil
}

// NewConfigManagementPluginClientSet creates new instance of config management plugin server Clientset
func NewConfigManagementPluginClientSet(address string) Clientset {
	return &clientSet{address: address}
}
