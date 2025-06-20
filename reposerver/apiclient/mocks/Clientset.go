package mocks

import (
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

type Clientset struct {
	RepoServerServiceClient apiclient.RepoServerServiceClient
}

func (c *Clientset) NewRepoServerClient() (utilio.Closer, apiclient.RepoServerServiceClient, error) {
	return utilio.NopCloser, c.RepoServerServiceClient, nil
}
