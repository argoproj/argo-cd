package controller

import (
	"crypto/tls"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/controller/services"
	"github.com/argoproj/argo-cd/util"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Clientset represents controller server api clients
type Clientset interface {
	NewApplicationServiceClient() (util.Closer, services.ApplicationServiceClient, error)
}

type clientSet struct {
	address string
}

func (c *clientSet) NewApplicationServiceClient() (util.Closer, services.ApplicationServiceClient, error) {
	conn, err := grpc.Dial(c.address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})))
	if err != nil {
		log.Errorf("Unable to connect to repository service with address %s", c.address)
		return nil, nil, err
	}
	return conn, services.NewApplicationServiceClient(conn), nil
}

// NewAppControllerClientset creates new instance of controller server Clientset
func NewAppControllerClientset(address string) Clientset {
	return &clientSet{address: address}
}
