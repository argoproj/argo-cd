package mocks

import (
	apiclient "github.com/argoproj/argo-cd/reposerver/apiclient"

	io "github.com/argoproj/argo-cd/util/io"
)

type Clientset struct {
	RepoServerServiceClient apiclient.RepoServerServiceClient
}

func (c *Clientset) NewRepoServerClient() (io.Closer, apiclient.RepoServerServiceClient, error) {
	return io.NopCloser, c.RepoServerServiceClient, nil
}
