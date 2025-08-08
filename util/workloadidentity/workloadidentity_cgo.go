//go:build !darwin || (cgo && darwin)

package workloadidentity

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	azcloud "github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"

	log "github.com/sirupsen/logrus"
)

var newDefaultAzureCredential = azidentity.NewDefaultAzureCredential

type WorkloadIdentityTokenProvider struct {
	tokenCredential azcore.TokenCredential
}

func NewWorkloadIdentityTokenProvider(azureCloud string) TokenProvider {
	cloud := azcloud.AzurePublic
	switch azureCloud {
	case "AzureChina":
		log.Info("Using Azure China cloud for Workload Identity")
		cloud = azcloud.AzureChina
	case "AzureGovernment":
		log.Info("Using Azure Government cloud for Workload Identity")
		cloud = azcloud.AzureGovernment
	}

	if azureCloud != "" && azureCloud != "AzurePublic" && azureCloud != "AzureChina" && azureCloud != "AzureGovernment" {
		log.Warnf("Could not parse Azure cloud '%s'. Possible values are: AzurePublic, AzureChina, AzureGovernment", azureCloud)
	}

	cred, err := newDefaultAzureCredential(
		&azidentity.DefaultAzureCredentialOptions{
			ClientOptions: policy.ClientOptions{
				Cloud: cloud,
			},
		})
	initError = err
	return WorkloadIdentityTokenProvider{tokenCredential: cred}
}

func (c WorkloadIdentityTokenProvider) GetToken(scope string) (*Token, error) {
	if initError != nil {
		return nil, initError
	}

	token, err := c.tokenCredential.GetToken(context.Background(), policy.TokenRequestOptions{
		Scopes: []string{scope},
	})
	if err != nil {
		return nil, err
	}

	return &Token{AccessToken: token.Token, ExpiresOn: token.ExpiresOn}, nil
}
