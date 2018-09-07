package reposerver

import (
	"crypto/tls"

	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/util"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Clientset represets repository server api clients
type Clientset interface {
	NewRepositoryClient() (util.Closer, repository.RepositoryServiceClient, error)
}

type clientSet struct {
	address string
}

func (c *clientSet) NewRepositoryClient() (util.Closer, repository.RepositoryServiceClient, error) {
	conn, err := grpc.Dial(c.address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})))
	if err != nil {
		log.Errorf("Unable to connect to repository service with address %s", c.address)
		return nil, nil, err
	}
	return conn, repository.NewRepositoryServiceClient(conn), nil
}

// NewRepositoryServerClientset creates new instance of repo server Clientset
func NewRepositoryServerClientset(address string) Clientset {
	return &clientSet{address: address}
}
