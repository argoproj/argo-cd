package apiclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/argoproj/argo-cd/server/application"
	"github.com/argoproj/argo-cd/server/cluster"
	"github.com/argoproj/argo-cd/server/repository"
	"github.com/argoproj/argo-cd/server/session"
	grpc_util "github.com/argoproj/argo-cd/util/grpc"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	// EnvArgoCDServer is the environment variable to look for an ArgoCD server address
	EnvArgoCDServer = "ARGOCD_SERVER"
)

type ServerClient interface {
	NewConn() (*grpc.ClientConn, error)
	NewRepoClient() (*grpc.ClientConn, repository.RepositoryServiceClient, error)
	NewRepoClientOrDie() (*grpc.ClientConn, repository.RepositoryServiceClient)
	NewClusterClient() (*grpc.ClientConn, cluster.ClusterServiceClient, error)
	NewClusterClientOrDie() (*grpc.ClientConn, cluster.ClusterServiceClient)
	NewApplicationClient() (*grpc.ClientConn, application.ApplicationServiceClient, error)
	NewApplicationClientOrDie() (*grpc.ClientConn, application.ApplicationServiceClient)
	NewSessionClient() (*grpc.ClientConn, session.SessionServiceClient, error)
	NewSessionClientOrDie() (*grpc.ClientConn, session.SessionServiceClient)
}

type ClientOptions struct {
	ServerAddr string
	Insecure   bool
	CertFile   string
}

type client struct {
	ClientOptions
}

func NewClient(opts *ClientOptions) (ServerClient, error) {
	clientOpts := *opts
	if clientOpts.ServerAddr == "" {
		clientOpts.ServerAddr = os.Getenv(EnvArgoCDServer)
	}
	if clientOpts.ServerAddr == "" {
		return nil, errors.New("Argo CD server address not supplied")
	}
	return &client{
		ClientOptions: clientOpts,
	}, nil
}

func NewClientOrDie(opts *ClientOptions) ServerClient {
	client, err := NewClient(opts)
	if err != nil {
		log.Fatal(err)
	}
	return client
}

func (c *client) NewConn() (*grpc.ClientConn, error) {
	var creds credentials.TransportCredentials
	if c.CertFile != "" {
		b, err := ioutil.ReadFile(c.CertFile)
		if err != nil {
			return nil, err
		}
		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM(b) {
			return nil, fmt.Errorf("credentials: failed to append certificates")
		}
		tlsConfig := tls.Config{
			RootCAs: cp,
		}
		if c.Insecure {
			tlsConfig.InsecureSkipVerify = true
		}
		creds = credentials.NewTLS(&tlsConfig)
	} else {
		if c.Insecure {
			tlsConfig := tls.Config{
				InsecureSkipVerify: true,
			}
			creds = credentials.NewTLS(&tlsConfig)
		}
	}
	return grpc_util.BlockingDial(context.Background(), "tcp", c.ServerAddr, creds)
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

func (c *client) NewSessionClient() (*grpc.ClientConn, session.SessionServiceClient, error) {
	conn, err := c.NewConn()
	if err != nil {
		return nil, nil, err
	}
	sessionIf := session.NewSessionServiceClient(conn)
	return conn, sessionIf, nil
}

func (c *client) NewSessionClientOrDie() (*grpc.ClientConn, session.SessionServiceClient) {
	conn, sessionIf, err := c.NewSessionClient()
	if err != nil {
		log.Fatalf("Failed to establish connection to %s: %v", c.ServerAddr, err)
	}
	return conn, sessionIf
}
