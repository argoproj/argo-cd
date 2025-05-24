package workloadidentity

import (
	"context"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

const (
	EmptyGuid = "00000000-0000-0000-0000-000000000000" //nolint:revive //FIXME(var-naming)
)

type Token struct {
	AccessToken string
	ExpiresOn   time.Time
}

type TokenProvider interface {
	GetToken(scope string) (Token, error)
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

func (c WorkloadIdentityTokenProvider) GetToken(scope string) (Token, error) {
	if initError != nil {
		return Token{}, initError
	}

	token, err := c.tokenCredential.GetToken(context.Background(), policy.TokenRequestOptions{
		Scopes: []string{scope},
	})
	if err != nil {
		return Token{}, err
	}

	return Token{AccessToken: token.Token, ExpiresOn: token.ExpiresOn}, nil
}
