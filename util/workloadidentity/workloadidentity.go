package workloadidentity

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

const (
	EmptyGuid = "00000000-0000-0000-0000-000000000000"
)

type TokenProvider interface {
	GetToken(scope string) (string, error)
}

type WorkloadIdentityTokenProvider struct {
	tokenCredential azcore.TokenCredential
}

// Used to propagate initialization error if any
var initError error

func NewWorkloadIdentityTokenProvider() TokenProvider {
	cred, err := azidentity.NewDefaultAzureCredential(&azidentity.DefaultAzureCredentialOptions{})
	initError = err
	return WorkloadIdentityTokenProvider{tokenCredential: cred}
}

func (c WorkloadIdentityTokenProvider) GetToken(scope string) (string, error) {
	if initError != nil {
		return "", initError
	}

	token, err := c.tokenCredential.GetToken(context.Background(), policy.TokenRequestOptions{
		Scopes: []string{scope},
	})
	if err != nil {
		return "", err
	}

	return token.Token, nil
}
