package service

import (
	"fmt"
	"log"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/tls"
)

type servicesFactory struct {
}

type ServicesFactory interface {
	CreateRepoClientset(repoServerAddress string, repoServerTimeoutSeconds int, repoServerPlaintext, repoServerStrictTLS bool) apiclient.Clientset
}

func NewServicesFactory() ServicesFactory {
	return &servicesFactory{}
}

func (sf *servicesFactory) CreateRepoClientset(repoServerAddress string, repoServerTimeoutSeconds int, repoServerPlaintext, repoServerStrictTLS bool) apiclient.Clientset {
	tlsConfig := apiclient.TLSConfiguration{
		DisableTLS:       repoServerPlaintext,
		StrictValidation: repoServerStrictTLS,
	}

	// Load CA information to use for validating connections to the
	// repository server, if strict TLS validation was requested.
	if !repoServerPlaintext && repoServerStrictTLS {
		pool, err := tls.LoadX509CertPool(
			fmt.Sprintf("%s/server/tls/tls.crt", env.StringFromEnv(common.EnvAppConfigPath, common.DefaultAppConfigPath)),
			fmt.Sprintf("%s/server/tls/ca.crt", env.StringFromEnv(common.EnvAppConfigPath, common.DefaultAppConfigPath)),
		)
		if err != nil {
			log.Fatalf("%v", err)
		}
		tlsConfig.Certificates = pool
	}

	return apiclient.NewRepoServerClientset(repoServerAddress, repoServerTimeoutSeconds, tlsConfig)
}
