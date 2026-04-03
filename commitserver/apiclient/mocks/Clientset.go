package mocks

import (
	"github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

type Clientset struct {
	CommitServiceClient apiclient.CommitServiceClient
}

func (c *Clientset) NewCommitServerClient() (utilio.Closer, apiclient.CommitServiceClient, error) {
	return utilio.NopCloser, c.CommitServiceClient, nil
}
