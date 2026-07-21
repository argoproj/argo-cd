package identity

// # Azure Workload Identity Setup
//
// This file implements Azure Workload Identity Federation for authenticating to Azure Container
// Registry (ACR) using Kubernetes service account tokens.
//
// ## Required Azure Setup
//
// 1. Set environment variables:
//
//	export AZURE_SUBSCRIPTION_ID=$(az account show --query id -o tsv)
//	export AZURE_TENANT_ID=$(az account show --query tenantId -o tsv)
//	export RESOURCE_GROUP="<your-resource-group>"
//	export ACR_NAME="<your-acr-name>"
//	export ARGOCD_NAMESPACE="argocd"
//	export PROJECT_NAME="default"
//
// 2. Get your cluster's OIDC issuer URL (for AKS):
//
//	export OIDC_ISSUER=$(az aks show --resource-group $RESOURCE_GROUP \
//	    --name $CLUSTER_NAME --query "oidcIssuerProfile.issuerUrl" -o tsv)
//
// 3. Create an Azure AD application:
//
//	export APP_NAME="argocd-project-${PROJECT_NAME}"
//	az ad app create --display-name $APP_NAME
//	export APP_CLIENT_ID=$(az ad app list --display-name $APP_NAME --query "[0].appId" -o tsv)
//	az ad sp create --id $APP_CLIENT_ID
//
// 4. Add federated credential (trust the K8s ServiceAccount):
//
//	cat <<EOF > federated-credential.json
//	{
//	  "name": "argocd-${PROJECT_NAME}-federated",
//	  "issuer": "${OIDC_ISSUER}",
//	  "subject": "system:serviceaccount:${ARGOCD_NAMESPACE}:argocd-project-${PROJECT_NAME}",
//	  "audiences": ["api://AzureADTokenExchange"]
//	}
//	EOF
//	az ad app federated-credential create --id $APP_CLIENT_ID --parameters federated-credential.json
//
// 5. Grant the application access to ACR:
//
//	export ACR_ID=$(az acr show --name $ACR_NAME --query id -o tsv)
//	az role assignment create --assignee $APP_CLIENT_ID --role "AcrPull" --scope $ACR_ID
//
// ## Required Kubernetes ServiceAccount Annotations
//
// The Kubernetes ServiceAccount (argocd-project-<name>) needs these annotations:
//
//   - azure.workload.identity/client-id: The Azure AD application (client) ID
//   - azure.workload.identity/tenant-id: The Azure AD tenant ID
//
// ## Required Repository Secret Fields
//
//   - useWorkloadIdentity: "true"
//   - workloadIdentityProvider: "azure"
//   - project: "<argocd-project-name>" (maps to argocd-project-<name> ServiceAccount)
//
// ## Authentication Flow
//
// 1. Request a K8s token for the project ServiceAccount via TokenRequest API
//    (with audience "api://AzureADTokenExchange")
// 2. Exchange the K8s token for an Azure access token via Azure AD OAuth endpoint
// 3. Exchange the Azure access token for an ACR refresh token
// 4. Return the ACR refresh token for use with the registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity/v2/repository"
)

const (
	AnnotationAzureClientID = "azure.workload.identity/client-id"
	AnnotationAzureTenantID = "azure.workload.identity/tenant-id"
	DefaultAzureAudience    = "api://AzureADTokenExchange"
)

// AzureProvider exchanges K8s JWTs for Azure credentials via STS
type AzureProvider struct {
	repo *v1alpha1.Repository
	k8s  *K8sProvider
}

func (p *AzureProvider) DefaultRepositoryAuthenticator() repository.Authenticator {
	if p.repo.Type == "git" {
		return repository.NewPassthroughAuthenticator()
	}
	return repository.NewACRAuthenticator()
}

// NewAzureProvider creates a new Azure identity provider
func NewAzureProvider(repo *v1alpha1.Repository, k8s *K8sProvider) *AzureProvider {
	return &AzureProvider{
		repo: repo,
		k8s:  k8s,
	}
}

// GetToken exchanges a K8s JWT for Azure credentials
func (p *AzureProvider) GetToken(ctx context.Context, audience string, tokenURL string) (*repository.Token, error) {
	sa, err := p.k8s.LoadSA(ctx)
	if err != nil {
		return nil, err
	}
	// Get Azure client ID and tenant ID from standard Azure Workload Identity annotations on service account
	clientID := sa.Annotations[AnnotationAzureClientID]
	if clientID == "" {
		return nil, fmt.Errorf("service account %s missing %s annotation", sa.Name, AnnotationAzureClientID)
	}

	tenantID := sa.Annotations[AnnotationAzureTenantID]
	if tenantID == "" {
		return nil, fmt.Errorf("service account %s missing %s annotation", sa.Name, AnnotationAzureTenantID)
	}

	log.WithFields(log.Fields{
		"serviceAccount": sa.Name,
		"clientID":       clientID,
		"tenantID":       tenantID,
	}).Info("Azure Workload Identity: exchanging K8s token for Azure access token")

	// Use configured audience or default to Azure AD token exchange
	if audience == "" {
		audience = DefaultAzureAudience
	}

	// Request K8s token with Azure audience
	k8sToken, err := p.k8s.GetToken(ctx, audience, "")
	if err != nil {
		return nil, fmt.Errorf("failed to request K8s token: %w", err)
	}

	// Get OAuth endpoint (allow override for sovereign clouds) from repository config
	if tokenURL == "" {
		tokenURL = fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID)
	} else {
		// Replace {tenantID} placeholder in custom endpoint
		tokenURL = strings.ReplaceAll(tokenURL, "{tenantID}", tenantID)
		log.WithField("tokenURL", tokenURL).Debug("Azure Workload Identity: using custom token endpoint")
	}

	// Exchange K8s JWT for Azure access token using client credentials flow
	log.Debug("Azure Workload Identity: requesting access token from Azure AD")
	azureToken, expiresAt, err := p.getAzureAccessToken(ctx, tokenURL, clientID, k8sToken.Token)
	if err != nil {
		log.WithFields(log.Fields{
			"clientID": clientID,
			"tenantID": tenantID,
			"error":    err.Error(),
		}).Error("Azure Workload Identity: failed to get access token")
		return nil, fmt.Errorf("failed to get Azure access token: %w", err)
	}

	log.WithFields(log.Fields{
		"clientID": clientID,
		"tenantID": tenantID,
	}).Info("Azure Workload Identity: successfully obtained access token")

	token := &repository.Token{
		Type:      repository.TokenTypeBearer,
		Token:     azureToken,
		ExpiresAt: expiresAt,
	}

	// For git repos (Azure DevOps), set the username for passthrough auth
	if p.repo.Type == "git" {
		token.Username = "oauth"
	}

	return token, nil
}

// getAzureAccessToken exchanges a K8s JWT for an Azure access token and its
// expiry (derived from the response's expires_in, nil when absent).
func (p *AzureProvider) getAzureAccessToken(ctx context.Context, tokenURL, clientID, k8sToken string) (string, *time.Time, error) {
	// Prepare OAuth 2.0 client credentials request with JWT bearer assertion
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	data.Set("client_assertion", k8sToken)
	data.Set("scope", "https://management.azure.com/.default")
	data.Set("grant_type", "client_credentials")

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Execute request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var expiresAt *time.Time
	if tokenResp.ExpiresIn > 0 {
		expiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		expiresAt = &expiry
	}
	return tokenResp.AccessToken, expiresAt, nil
}

// Ensure GCPProvider implements Provider
var _ Provider = (*AzureProvider)(nil)
