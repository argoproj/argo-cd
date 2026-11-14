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
	// initErr is the error returned when the credential was created, if any. It is kept per instance so that
	// providers created concurrently do not share or overwrite each other's initialization errors.
	initErr error
}

func NewWorkloadIdentityTokenProvider() TokenProvider {
	cred, err := azidentity.NewDefaultAzureCredential(&azidentity.DefaultAzureCredentialOptions{})
	return WorkloadIdentityTokenProvider{tokenCredential: cred, initErr: err}
}

// NewStrictWorkloadIdentityTokenProvider returns a TokenProvider that only uses Azure Workload Identity
// (the federated token projected into the pod by the Azure Workload Identity webhook). Unlike
// NewWorkloadIdentityTokenProvider, it never falls back to other credentials such as the node's managed
// identity or the Azure CLI, so tokens can only be obtained when workload identity is configured for the pod.
func NewStrictWorkloadIdentityTokenProvider() TokenProvider {
	cred, err := azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{})
	return WorkloadIdentityTokenProvider{tokenCredential: cred, initErr: err}
}

func (c WorkloadIdentityTokenProvider) GetToken(scope string) (*Token, error) {
	if c.initErr != nil {
		return nil, c.initErr
	}

	token, err := c.tokenCredential.GetToken(context.Background(), policy.TokenRequestOptions{
		Scopes: []string{scope},
	})
	if err != nil {
		return nil, err
	}

	return &Token{AccessToken: token.Token, ExpiresOn: token.ExpiresOn}, nil
}
