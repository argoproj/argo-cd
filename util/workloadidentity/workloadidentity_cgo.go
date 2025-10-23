//go:build !darwin || (cgo && darwin)

package workloadidentity

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

type WorkloadIdentityTokenProvider struct {
	tokenCredential azcore.TokenCredential
}

func NewWorkloadIdentityTokenProvider() TokenProvider {
	cred, err := azidentity.NewDefaultAzureCredential(&azidentity.DefaultAzureCredentialOptions{})
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
