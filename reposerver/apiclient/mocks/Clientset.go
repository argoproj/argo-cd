package mocks

import (
	"context"

	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/util/io"
)

type Clientset struct {
	RepoServerServiceClient apiclient.RepoServerServiceClient
}

func (c *Clientset) NewRepoServerClient(_ context.Context) (io.Closer, apiclient.RepoServerServiceClient, error) {
	return io.NopCloser, c.RepoServerServiceClient, nil
}
