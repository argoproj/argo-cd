package apiclient

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/argoproj/argo-cd/v2/util/io"
)

// Clientset represents commit server api clients
type Clientset interface {
	NewCommitServerClient() (io.Closer, CommitServiceClient, error)
}

type clientSet struct {
	address string
}

// NewCommitServerClient creates new instance of commit server client
func (c *clientSet) NewCommitServerClient() (io.Closer, CommitServiceClient, error) {
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

	// TODO: switch to grpc.NewClient.
	// nolint:staticcheck
	conn, err := grpc.Dial(address, opts...)
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
