package services

import (
	"context"
	"net/http"

	bitbucketv1 "github.com/gfleury/go-bitbucket-v1"

	"github.com/argoproj/argo-cd/v3/applicationset/utils"
	"github.com/argoproj/argo-cd/v3/util/proxy"
)

// SetupBitbucketClient configures and creates a Bitbucket API client with TLS settings
func SetupBitbucketClient(ctx context.Context, config *bitbucketv1.Configuration, scmRootCAPath string, insecure bool, caCerts []byte, proxyURL, noProxy string) *bitbucketv1.APIClient {
	config.BasePath = utils.NormalizeBitbucketBasePath(config.BasePath)
	tlsConfig := utils.GetTlsConfig(scmRootCAPath, insecure, caCerts)

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	transport.Proxy = proxy.GetCallback(proxyURL, noProxy)
	config.HTTPClient = &http.Client{Transport: transport}

	return bitbucketv1.NewAPIClient(ctx, config)
}
