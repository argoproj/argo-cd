package helm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	gocache "github.com/patrickmn/go-cache"

	argoutils "github.com/argoproj/argo-cd/v2/util"
	"github.com/argoproj/argo-cd/v2/util/workloadidentity"
)

// In memory cache for storing Azure tokens
var azureTokenCache *gocache.Cache

func init() {
	azureTokenCache = gocache.New(gocache.NoExpiration, 0)
}

type Creds interface {
	GetUsername() string
	GetPassword() string
	GetCAPath() string
	GetCertData() []byte
	GetKeyData() []byte
	GetInsecureSkipVerify() bool
}

type HelmCreds struct {
	Username           string
	Password           string
	CAPath             string
	CertData           []byte
	KeyData            []byte
	InsecureSkipVerify bool
}

func (creds HelmCreds) GetUsername() string {
	return creds.Username
}

func (creds HelmCreds) GetPassword() string {
	return creds.Password
}

func (creds HelmCreds) GetCAPath() string {
	return creds.CAPath
}

func (creds HelmCreds) GetCertData() []byte {
	return creds.CertData
}

func (creds HelmCreds) GetKeyData() []byte {
	return creds.KeyData
}

func (creds HelmCreds) GetInsecureSkipVerify() bool {
	return creds.InsecureSkipVerify
}

type AzureWorkloadIdentityCreds struct {
	repoUrl            string
	CAPath             string
	CertData           []byte
	KeyData            []byte
	InsecureSkipVerify bool
}

func (creds AzureWorkloadIdentityCreds) GetUsername() string {
	return "00000000-0000-0000-0000-000000000000"
}

func (creds AzureWorkloadIdentityCreds) GetPassword() string {
	token, _ := creds.GetAccessToken(creds.repoUrl) // TODO: propagate error

	return token
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

func NewAzureWorkloadIdentityCreds(caPath string, certData []byte, keyData []byte, insecureSkipVerify bool) AzureWorkloadIdentityCreds {
	return AzureWorkloadIdentityCreds{
		CAPath:             caPath,
		CertData:           certData,
		KeyData:            keyData,
		InsecureSkipVerify: insecureSkipVerify,
	}
}

func (c AzureWorkloadIdentityCreds) GetAccessToken(azureContainerRegistry string) (string, error) {
	// Compute hash as key for refresh token in the cache
	key, err := argoutils.GenerateCacheKey("accesstoken-%s", azureContainerRegistry)
	if err != nil {
		return "", fmt.Errorf("failed to compute key for cache: %w", err)
	}

	// Check cache for GitHub transport which helps fetch an API token
	t, found := azureTokenCache.Get(key)
	if found {
		fmt.Println("access token found token in cache")
		return t.(string), nil
	}

	tokenParams, err := c.challengeAzureContainerRegistry(azureContainerRegistry)
	if err != nil {
		return "", fmt.Errorf("failed to challenge Azure Container Registry: %w", err)
	}

	token, err := c.getAccessTokenAfterChallenge(tokenParams)
	if err != nil {
		return "", fmt.Errorf("failed to get Azure access token after challenge: %w", err)
	}

	// Access token has a lifetime of 3 hours
	azureTokenCache.Set(key, token, 2*time.Hour)
	return token, nil
}

func (c AzureWorkloadIdentityCreds) getAccessTokenAfterChallenge(tokenParams map[string]string) (string, error) {
	realm := tokenParams["realm"]
	service := tokenParams["service"]

	armAccessToken, err := workloadidentity.GetWorkloadIdentityToken("https://management.core.windows.net//.default") // TODO: parameterize
	if err != nil {
		return "", fmt.Errorf("failed to get Azure access token: %w", err)
	}

	parsedUrl, _ := url.Parse(realm)
	parsedUrl.Path = "/oauth2/exchange"
	refreshTokenUrl := parsedUrl.String()

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	formValues := url.Values{}
	formValues.Add("grant_type", "access_token")
	formValues.Add("service", service)
	formValues.Add("access_token", armAccessToken)

	resp, err := client.PostForm(refreshTokenUrl, formValues)
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

func (c AzureWorkloadIdentityCreds) challengeAzureContainerRegistry(azureContainerRegistry string) (map[string]string, error) {
	requestUrl := fmt.Sprintf("https://%s/v2/", azureContainerRegistry)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, requestUrl, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to registry '%w'", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized && resp.Header.Get("Www-Authenticate") == "" {
		return nil, fmt.Errorf("registry '%s' did not issue a challenge", azureContainerRegistry)
	}

	authenticate := resp.Header.Get("Www-Authenticate")
	tokens := strings.Split(authenticate, " ")

	if strings.ToLower(tokens[0]) != "bearer" {
		return nil, fmt.Errorf("registry does not allow 'Bearer' authentication, got '%s'", tokens[0])
	}

	tokenParams := make(map[string]string)

	for _, token := range strings.Split(tokens[1], ",") {
		kvPair := strings.Split(token, "=")
		tokenParams[kvPair[0]] = strings.Trim(kvPair[1], "\"")
	}

	if _, realmExists := tokenParams["realm"]; !realmExists {
		return nil, fmt.Errorf("realm parameter not found in challenge")
	}

	if _, serviceExists := tokenParams["service"]; !serviceExists {
		return nil, fmt.Errorf("service parameter not found in challenge")
	}

	return tokenParams, nil
}
