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
	config_util "github.com/argoproj/argo-cd/util/config"
	grpc_util "github.com/argoproj/argo-cd/util/grpc"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	// EnvArgoCDServer is the environment variable to look for an ArgoCD server address
	EnvArgoCDServer = "ARGOCD_SERVER"
)

// ServerClient defines an interface for interaction with an Argo CD server.
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

// ClientOptions hold address, security, and other settings for the API client.
type ClientOptions struct {
	ServerAddr string
	Insecure   bool
	CertFile   string
	AuthToken  string
}

type client struct {
	ClientOptions
}

// NewClient creates a new API client from a set of config options.
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

// NewClientOrDie creates a new API client from a set of config options, or fails fatally if the new client creation fails.
func NewClientOrDie(opts *ClientOptions) ServerClient {
	client, err := NewClient(opts)
	if err != nil {
		log.Fatal(err)
	}
	return client
}

// JwtCredentials holds a token for authentication.
type jwtCredentials struct {
	Token string
}

func (c jwtCredentials) RequireTransportSecurity() bool {
	return false
}

func (c jwtCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{
		"tokens": c.Token,
	}, nil
}

// firstEndpointTokenFrom iterates through given endpoint names and returns the first non-blank token, if any, that it finds.
// This function will always return a manually-specified auth token, if it is provided on the command-line.
func (c *client) firstEndpointTokenFrom(endpoints ...string) string {
	if token := c.ClientOptions.AuthToken; token != "" {
		return token
	}

	// Only look up credentials if the auth token isn't overridden
	if localConfig, err := config_util.ReadLocalConfig(); err == nil {
		for _, endpoint := range endpoints {
			if token, ok := localConfig.Sessions[endpoint]; ok {
				return token
			}
		}
	}

	return ""
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

	endpointCredentials := jwtCredentials{
		Token: c.firstEndpointTokenFrom(c.ServerAddr, ""),
	}
	return grpc_util.BlockingDial(context.Background(), "tcp", c.ServerAddr, creds, grpc.WithPerRPCCredentials(endpointCredentials))
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
