package oci

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	gocache "github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"

	argoutils "github.com/argoproj/argo-cd/v3/util"
	"github.com/argoproj/argo-cd/v3/util/env"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity"
)

// In memory cache for storing Azure tokens
var azureTokenCache *gocache.Cache

func init() {
	azureTokenCache = gocache.New(gocache.NoExpiration, 0)
}

// storeAzureToken stores a token in the cache
func storeAzureToken(key, token string, expiration time.Duration) {
	azureTokenCache.Set(key, token, expiration)
}

// Creds is an interface for OCI credentials
type Creds interface {
	GetUsername() string
	GetPassword() (string, error)
	GetCAPath() string
	GetCertData() []byte
	GetKeyData() []byte
	GetInsecureSkipVerify() bool
	GetInsecureHTTPOnly() bool
}

var _ Creds = OCICreds{}

// OCICreds represents standard username/password credentials for OCI registries
type OCICreds struct {
	Username           string
	Password           string
	CAPath             string
	CertData           []byte
	KeyData            []byte
	InsecureSkipVerify bool
	InsecureHTTPOnly   bool
}

func (creds OCICreds) GetUsername() string {
	return creds.Username
}

func (creds OCICreds) GetPassword() (string, error) {
	return creds.Password, nil
}

func (creds OCICreds) GetCAPath() string {
	return creds.CAPath
}

func (creds OCICreds) GetCertData() []byte {
	return creds.CertData
}

func (creds OCICreds) GetKeyData() []byte {
	return creds.KeyData
}

func (creds OCICreds) GetInsecureSkipVerify() bool {
	return creds.InsecureSkipVerify
}

func (creds OCICreds) GetInsecureHTTPOnly() bool {
	return creds.InsecureHTTPOnly
}

var _ Creds = AzureWorkloadIdentityCreds{}

// AzureWorkloadIdentityCreds represents workload identity credentials for Azure Container Registry
type AzureWorkloadIdentityCreds struct {
	repoURL            string
	CAPath             string
	CertData           []byte
	KeyData            []byte
	InsecureSkipVerify bool
	InsecureHTTPOnly   bool
	tokenProvider      workloadidentity.TokenProvider
}

func (creds AzureWorkloadIdentityCreds) GetUsername() string {
	return workloadidentity.EmptyGuid
}

func (creds AzureWorkloadIdentityCreds) GetPassword() (string, error) {
	return creds.GetAccessToken()
}

func (creds AzureWorkloadIdentityCreds) GetCAPath() string {
	return creds.CAPath
}

func (creds AzureWorkloadIdentityCreds) GetCertData() []byte {
	return creds.CertData
}

func (creds AzureWorkloadIdentityCreds) GetKeyData() []byte {
	return creds.KeyData
}

func (creds AzureWorkloadIdentityCreds) GetInsecureSkipVerify() bool {
	return creds.InsecureSkipVerify
}

func (creds AzureWorkloadIdentityCreds) GetInsecureHTTPOnly() bool {
	return creds.InsecureHTTPOnly
}

// NewAzureWorkloadIdentityCreds creates a new AzureWorkloadIdentityCreds instance
func NewAzureWorkloadIdentityCreds(repoURL string, caPath string, certData []byte, keyData []byte, insecureSkipVerify bool, insecureHTTPOnly bool, tokenProvider workloadidentity.TokenProvider) AzureWorkloadIdentityCreds {
	return AzureWorkloadIdentityCreds{
		repoURL:            repoURL,
		CAPath:             caPath,
		CertData:           certData,
		KeyData:            keyData,
		InsecureSkipVerify: insecureSkipVerify,
		InsecureHTTPOnly:   insecureHTTPOnly,
		tokenProvider:      tokenProvider,
	}
}

// GetAccessToken retrieves an Azure Container Registry access token using workload identity
func (creds AzureWorkloadIdentityCreds) GetAccessToken() (string, error) {
	parsed, err := url.Parse(creds.repoURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse oci repo url: %w", err)
	}
	ctx := context.Background()

	// Compute hash as key for refresh token in the cache
	key, err := argoutils.GenerateCacheKey("accesstoken-%s", parsed.Host)
	if err != nil {
		return "", fmt.Errorf("failed to compute key for cache: %w", err)
	}

	// Check cache for access token
	t, found := azureTokenCache.Get(key)
	if found {
		fmt.Println("access token found token in cache")
		return t.(string), nil
	}

	tokenParams, err := creds.challengeAzureContainerRegistry(ctx, parsed.Host)
	if err != nil {
		return "", fmt.Errorf("failed to challenge Azure Container Registry: %w", err)
	}

	token, err := creds.getAccessTokenAfterChallenge(ctx, tokenParams)
	if err != nil {
		return "", fmt.Errorf("failed to get Azure access token after challenge: %w", err)
	}

	tokenExpiry, err := getJWTExpiry(token)
	if err != nil {
		log.Warnf("failed to get token expiry from JWT: %v, using current time as fallback", err)
		tokenExpiry = time.Now()
	}

	cacheExpiry := workloadidentity.CalculateCacheExpiryBasedOnTokenExpiry(tokenExpiry)
	if cacheExpiry > 0 {
		storeAzureToken(key, token, cacheExpiry)
	}
	return token, nil
}

// getJWTExpiry extracts the expiration time from a JWT token
func getJWTExpiry(token string) (time.Time, error) {
	parser := jwt.NewParser()
	claims := jwt.MapClaims{}
	_, _, err := parser.ParseUnverified(token, claims)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse JWT: %w", err)
	}
	exp, err := claims.GetExpirationTime()
	if err != nil {
		return time.Time{}, fmt.Errorf("'exp' claim not found or invalid in token: %w", err)
	}
	if exp == nil {
		return time.Time{}, errors.New("'exp' claim is nil in token")
	}
	return time.UnixMilli(exp.UnixMilli()), nil
}

// getAccessTokenAfterChallenge exchanges an ARM token for an ACR access token
func (creds AzureWorkloadIdentityCreds) getAccessTokenAfterChallenge(ctx context.Context, tokenParams map[string]string) (string, error) {
	realm := tokenParams["realm"]
	service := tokenParams["service"]

	armTokenScope := env.StringFromEnv("AZURE_ARM_TOKEN_RESOURCE", "https://management.core.windows.net")
	armAccessToken, err := creds.tokenProvider.GetToken(armTokenScope + "/.default")
	if err != nil {
		return "", fmt.Errorf("failed to get Azure access token: %w", err)
	}

	parsedURL, _ := url.Parse(realm)
	parsedURL.Path = "/oauth2/exchange"
	refreshTokenURL := parsedURL.String()

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: creds.GetInsecureSkipVerify(),
			},
		},
	}

	formValues := url.Values{}
	formValues.Add("grant_type", "access_token")
	formValues.Add("service", service)
	formValues.Add("access_token", armAccessToken.AccessToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, refreshTokenURL, strings.NewReader(formValues.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create request to get refresh token: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("unable to connect to registry '%w'", err)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get refresh token: %s", resp.Status)
	}

	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	type Response struct {
		RefreshToken string `json:"refresh_token"`
	}

	var res Response
	err = json.Unmarshal(body, &res)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return res.RefreshToken, nil
}

// challengeAzureContainerRegistry challenges the Azure Container Registry to get authentication parameters
func (creds AzureWorkloadIdentityCreds) challengeAzureContainerRegistry(ctx context.Context, azureContainerRegistry string) (map[string]string, error) {
	requestURL := fmt.Sprintf("https://%s/v2/", azureContainerRegistry)

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: creds.GetInsecureSkipVerify(),
			},
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to registry '%w'", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized || resp.Header.Get("Www-Authenticate") == "" {
		return nil, fmt.Errorf("registry '%s' did not issue a challenge", azureContainerRegistry)
	}

	authenticate := resp.Header.Get("Www-Authenticate")
	tokens := strings.Split(authenticate, " ")

	if !strings.EqualFold(tokens[0], "bearer") {
		return nil, fmt.Errorf("registry does not allow 'Bearer' authentication, got '%s'", tokens[0])
	}

	tokenParams := make(map[string]string)

	for _, token := range strings.Split(tokens[1], ",") {
		kvPair := strings.Split(token, "=")
		tokenParams[kvPair[0]] = strings.Trim(kvPair[1], "\"")
	}

	if _, realmExists := tokenParams["realm"]; !realmExists {
		return nil, errors.New("realm parameter not found in challenge")
	}

	if _, serviceExists := tokenParams["service"]; !serviceExists {
		return nil, errors.New("service parameter not found in challenge")
	}

	return tokenParams, nil
}
