package apiclient

import (
	"fmt"
	"math"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/util/env"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

// MaxGRPCMessageSize contains max grpc message size
var MaxGRPCMessageSize = env.ParseNumFromEnv(common.EnvGRPCMaxSizeMB, 100, 0, math.MaxInt32) * 1024 * 1024

// Clientset represents commit server api clients
type Clientset interface {
	NewCommitServerClient() (utilio.Closer, CommitServiceClient, error)
}

type clientSet struct {
	address string
}

// NewCommitServerClient creates new instance of commit server client
func (c *clientSet) NewCommitServerClient() (utilio.Closer, CommitServiceClient, error) {
	conn, err := NewConnection(c.address)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open a new connection to commit server: %w", err)
	}
	return conn, NewCommitServiceClient(conn), nil
}

// NewConnection creates new connection to commit server
func NewConnection(address string) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	conn, err := grpc.NewClient(address, opts...)
	if err != nil {
		log.Errorf("Unable to connect to commit service with address %s", address)
		return nil, err
	}
	return conn, nil
}

// NewCommitServerClientset creates new instance of commit server Clientset
func NewCommitServerClientset(address string) Clientset {
	return &clientSet{address: address}
}
