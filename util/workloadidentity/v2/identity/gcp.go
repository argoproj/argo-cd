package identity

// Package identity provides credential resolution for cloud provider workload identity.
//
// # GCP Workload Identity Setup
//
// This package supports GCP Workload Identity Federation for authenticating to GCP services
// (Artifact Registry, GCR) using Kubernetes service account tokens.
//
// ## Required GCP Setup
//
// 1. Create a Workload Identity Pool and OIDC provider that trusts your Kubernetes cluster:
//
//	# Create the pool
//	gcloud iam workload-identity-pools create <POOL_NAME> \
//	    --location="global" \
//	    --display-name="<DISPLAY_NAME>"
//
//	# Create an OIDC provider trusting your cluster's issuer
//	gcloud iam workload-identity-pools providers create-oidc <PROVIDER_NAME> \
//	    --location="global" \
//	    --workload-identity-pool="<POOL_NAME>" \
//	    --issuer-uri="<CLUSTER_OIDC_ISSUER>" \
//	    --attribute-mapping="google.subject=assertion.sub"
//
// For GKE, the issuer URI is:
//
//	https://container.googleapis.com/v1/projects/<PROJECT>/locations/<LOCATION>/clusters/<CLUSTER>
//
// 2. Create a GCP service account for the ArgoCD project:
//
//	gcloud iam service-accounts create argocd-project-<PROJECT_NAME>
//
// 3. Grant the federated identity permission to impersonate the GCP service account:
//
//	gcloud iam service-accounts add-iam-policy-binding \
//	    <GCP_SA>@<PROJECT>.iam.gserviceaccount.com \
//	    --role="roles/iam.workloadIdentityUser" \
//	    --member="principal://iam.googleapis.com/projects/<PROJECT_NUMBER>/locations/global/workloadIdentityPools/<POOL>/subject/system:serviceaccount:<K8S_NS>:<K8S_SA>"
//
// 4. Grant the GCP service account access to Artifact Registry:
//
//	gcloud projects add-iam-policy-binding <PROJECT> \
//	    --member="serviceAccount:<GCP_SA>@<PROJECT>.iam.gserviceaccount.com" \
//	    --role="roles/artifactregistry.reader"
//
// ## Required Kubernetes ServiceAccount Annotations
//
// The Kubernetes ServiceAccount (argocd-project-<name>) needs these annotations:
//
//   - iam.gke.io/gcp-service-account: The GCP service account email to impersonate
//   - iam.gke.io/workload-identity-provider: The full WIF provider path:
//     //iam.googleapis.com/projects/<PROJECT_NUMBER>/locations/global/workloadIdentityPools/<POOL>/providers/<PROVIDER>
//
// ## Required Repository Secret Fields
//
//   - useWorkloadIdentity: "true"
//   - workloadIdentityProvider: "gcp"
//   - project: "<argocd-project-name>" (maps to argocd-project-<name> ServiceAccount)
//
// ## Authentication Flow
//
// 1. Request a K8s token for the project ServiceAccount via TokenRequest API
// 2. Exchange the K8s token with GCP STS for a federated access token
// 3. Use the federated token to impersonate the target GCP service account
// 4. Return the access token for use with Artifact Registry/GCR

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"

	"github.com/argoproj/argo-cd/v3/util/workloadidentity/v2/repository"
)

const (
	// DefaultGCPSTSURL is the default Google Cloud STS endpoint for token exchange
	DefaultGCPSTSURL = "https://sts.googleapis.com/v1/token"
	// DefaultGCPIAMCredentialsURL is the IAM Credentials API endpoint for service account impersonation
	DefaultGCPIAMCredentialsURL = "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/%s:generateAccessToken"
	// GCPMetadataTokenURL is the GKE metadata server endpoint for getting the pod's own token
	GCPMetadataTokenURL = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"
	// AnnotationGCPWorkloadIdentity is the annotation key for the Workload Identity Federation provider path
	AnnotationGCPWorkloadIdentity = "iam.gke.io/workload-identity-provider"
	AnnotationGCPSA               = "iam.gke.io/gcp-service-account"
)

// GCPProvider exchanges K8s JWTs for GCP access tokens via Workload Identity Federation
type GCPProvider struct {
	k8s *K8sProvider
}

func (p *GCPProvider) DefaultRepositoryAuthenticator() repository.Authenticator {
	return repository.NewPassthroughAuthenticator()
}

// NewGCPProvider creates a new GCP identity provider
func NewGCPProvider(k8s *K8sProvider) *GCPProvider {
	return &GCPProvider{
		k8s: k8s,
	}
}

// GetToken exchanges a K8s JWT for GCP credentials
func (p *GCPProvider) GetToken(ctx context.Context, audience string, tokenURL string) (*repository.Token, error) {
	sa, err := p.k8s.LoadSA(ctx)
	if err != nil {
		return nil, err
	}

	// Get GCP service account from standard GKE annotation
	gcpSA := sa.Annotations[AnnotationGCPSA]
	if gcpSA == "" {
		return nil, fmt.Errorf("service account %s missing %s annotation", sa.Name, AnnotationGCPSA)
	}

	log.WithFields(log.Fields{
		"serviceAccount": fmt.Sprintf("%s/%s", sa.Namespace, sa.Name),
		"gcpSA":          gcpSA,
		"audience":       audience,
	}).Debug("resolveGCP: resolving GCP credentials")

	// Try GKE metadata server first (works for GKE Workload Identity)
	accessToken, expiresAt, err := p.resolveGCPViaMetadata(ctx, gcpSA)
	if err != nil {
		log.Infof("resolveGCP: metadata server approach failed: %v, trying STS", err)
		// Fall back to STS token exchange (for Workload Identity Federation)
		accessToken, expiresAt, err = p.resolveGCPViaSTS(ctx, sa, gcpSA, audience, tokenURL)
		if err != nil {
			return nil, err
		}
	}

	return &repository.Token{
		Type:      repository.TokenTypeBearer,
		Token:     accessToken,
		Username:  "oauth2accesstoken", // GCR/GAR require this specific username
		ExpiresAt: expiresAt,
	}, nil
}

// resolveGCPViaMetadata uses the GKE metadata server to get a token, then impersonates the target SA
func (p *GCPProvider) resolveGCPViaMetadata(ctx context.Context, targetSA string) (string, *time.Time, error) {
	// Get token from metadata server (this is the pod's own GCP identity)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GCPMetadataTokenURL, http.NoBody)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create metadata request: %w", err)
	}
	req.Header.Set("Metadata-Flavor", "Google")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("metadata request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("metadata server returned status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", nil, fmt.Errorf("failed to decode metadata response: %w", err)
	}

	log.Infof("resolveGCPViaMetadata: got token from metadata server, impersonating %s", targetSA)

	// Use this token to impersonate the target service account
	return p.impersonateServiceAccount(ctx, tokenResp.AccessToken, targetSA)
}

// resolveGCPViaSTS uses STS token exchange for Workload Identity Federation
func (p *GCPProvider) resolveGCPViaSTS(ctx context.Context, sa *corev1.ServiceAccount, gcpSA string, audience string, tokenURL string) (string, *time.Time, error) {
	// Get the workload identity provider audience
	if audience == "" {
		audience = sa.Annotations[AnnotationGCPWorkloadIdentity]
	}
	if audience == "" {
		return "", nil, fmt.Errorf("workload identity provider audience not specified: set workloadIdentityAudience in repository config or add %s annotation to service account %s", AnnotationGCPWorkloadIdentity, sa.Name)
	}

	log.Infof("resolveGCPViaSTS: using audience=%q", audience)

	// Step 1: Request K8s token with the GCP audience
	k8sToken, err := p.k8s.GetToken(ctx, audience, "")
	if err != nil {
		return "", nil, fmt.Errorf("failed to request K8s token: %w", err)
	}

	// Step 2: Exchange K8s token with GCP STS for a federated token
	federatedToken, err := p.exchangeTokenWithSTS(ctx, k8sToken.Token, audience, tokenURL)
	if err != nil {
		return "", nil, fmt.Errorf("STS token exchange failed: %w", err)
	}

	// Step 3: Use federated token to impersonate the GCP service account
	return p.impersonateServiceAccount(ctx, federatedToken, gcpSA)
}

// exchangeTokenWithSTS exchanges a K8s service account token for a GCP federated access token
func (p *GCPProvider) exchangeTokenWithSTS(ctx context.Context, k8sToken, audience, tokenURL string) (string, error) {
	if tokenURL == "" {
		tokenURL = DefaultGCPSTSURL
	}

	log.Infof("GCP STS exchange: audience=%q, tokenURL=%q", audience, tokenURL)

	data := url.Values{}
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	data.Set("subject_token", k8sToken)
	data.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")
	data.Set("requested_token_type", "urn:ietf:params:oauth:token-type:access_token")
	data.Set("audience", audience)
	data.Set("scope", "https://www.googleapis.com/auth/cloud-platform")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return tokenResp.AccessToken, nil
}

// impersonateServiceAccount uses a federated token to get an access token for a GCP service account.
// It also returns the token expiry from the response's expireTime, or nil when absent/unparseable.
func (p *GCPProvider) impersonateServiceAccount(ctx context.Context, federatedToken, serviceAccountEmail string) (string, *time.Time, error) {
	impersonateURL := fmt.Sprintf(DefaultGCPIAMCredentialsURL, serviceAccountEmail)

	requestBody := map[string]any{
		"scope": []string{"https://www.googleapis.com/auth/cloud-platform"},
	}
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, impersonateURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+federatedToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"accessToken"`
		ExpireTime  string `json:"expireTime"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var expiresAt *time.Time
	if expiry, err := time.Parse(time.RFC3339, tokenResp.ExpireTime); err == nil {
		expiresAt = &expiry
	}
	return tokenResp.AccessToken, expiresAt, nil
}

// Ensure GCPProvider implements Provider
var _ Provider = (*GCPProvider)(nil)
