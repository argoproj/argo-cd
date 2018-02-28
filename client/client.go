package client

import (
	"errors"

	"github.com/argoproj/argo-cd/server/application"
	"github.com/argoproj/argo-cd/server/cluster"
	"github.com/argoproj/argo-cd/server/repository"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type Client interface {
	NewConn() (*grpc.ClientConn, error)
	NewRepoClient() (*grpc.ClientConn, repository.RepositoryServiceClient, error)
	NewRepoClientOrDie() (*grpc.ClientConn, repository.RepositoryServiceClient)
	NewClusterClient() (*grpc.ClientConn, cluster.ClusterServiceClient, error)
	NewClusterClientOrDie() (*grpc.ClientConn, cluster.ClusterServiceClient)
	NewApplicationClient() (*grpc.ClientConn, application.ApplicationServiceClient, error)
	NewApplicationClientOrDie() (*grpc.ClientConn, application.ApplicationServiceClient)
}

type ClientOptions struct {
	ServerAddr string
	Insecure   bool
}

type client struct {
	ClientOptions
}

func NewClient(opts *ClientOptions) Client {
	return &client{
		ClientOptions: *opts,
	}
}

func (c *client) NewConn() (*grpc.ClientConn, error) {
	var dialOpts []grpc.DialOption
	if c.Insecure {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	} else {
		return nil, errors.New("secure authentication unsupported")
	} // else if opts.Credentials != nil {
	//	dialOpts = append(dialOpts, grpc.WithTransportCredentials(opts.Credentials))
	//}
	return grpc.Dial(c.ServerAddr, dialOpts...)
}

func (c *client) NewRepoClient() (*grpc.ClientConn, repository.RepositoryServiceClient, error) {
	conn, err := c.NewConn()
	if err != nil {
		return nil, nil, err
	}
	repoIf := repository.NewRepositoryServiceClient(conn)
	return conn, repoIf, nil
}

func (c *client) NewRepoClientOrDie() (*grpc.ClientConn, repository.RepositoryServiceClient) {
	conn, repoIf, err := c.NewRepoClient()
	if err != nil {
		log.Fatalf("Failed to establish connection to %s: %v", c.ServerAddr, err)
	}
	return conn, repoIf
}

func (c *client) NewClusterClient() (*grpc.ClientConn, cluster.ClusterServiceClient, error) {
	conn, err := c.NewConn()
	if err != nil {
		return nil, nil, err
	}
	clusterIf := cluster.NewClusterServiceClient(conn)
	return conn, clusterIf, nil
}

func (c *client) NewClusterClientOrDie() (*grpc.ClientConn, cluster.ClusterServiceClient) {
	conn, clusterIf, err := c.NewClusterClient()
	if err != nil {
		log.Fatalf("Failed to establish connection to %s: %v", c.ServerAddr, err)
	}
	return conn, clusterIf
}

func (c *client) NewApplicationClient() (*grpc.ClientConn, application.ApplicationServiceClient, error) {
	conn, err := c.NewConn()
	if err != nil {
		return nil, nil, err
	}
	appIf := application.NewApplicationServiceClient(conn)
	return conn, appIf, nil
}

func (c *client) NewApplicationClientOrDie() (*grpc.ClientConn, application.ApplicationServiceClient) {
	conn, repoIf, err := c.NewApplicationClient()
	if err != nil {
		log.Fatalf("Failed to establish connection to %s: %v", c.ServerAddr, err)
	}
	return conn, repoIf
}
