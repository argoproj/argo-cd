package helm

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	gocache "github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"

	argoutils "github.com/argoproj/argo-cd/v3/util"
)

// In memory cache for storing ECR tokens
var ecrTokenCache *gocache.Cache

// RWMutex to ensure thread-safe access to ECR token cache operations
var ecrCacheMutex sync.RWMutex

func init() {
	// Create cache with 5-minute cleanup interval for expired items
	ecrTokenCache = gocache.New(gocache.NoExpiration, 5*time.Minute)
}

// StoreECRToken stores a token in the cache with expiration (thread-safe)
func storeECRToken(key, token string, expiration time.Duration) {
	if expiration <= 0 {
		log.Warnf("Invalid expiration duration for ECR token cache: %v", expiration)
		return
	}
	
	ecrCacheMutex.Lock()
	defer ecrCacheMutex.Unlock()
	
	ecrTokenCache.Set(key, token, expiration)
	log.Debugf("Stored ECR token in cache with key %s, expires in %v", key, expiration)
}

// getCachedECRToken retrieves a token from the cache if it exists and is valid (thread-safe)
func getCachedECRToken(key string) (string, bool) {
	ecrCacheMutex.RLock()
	defer ecrCacheMutex.RUnlock()
	
	if token, found := ecrTokenCache.Get(key); found {
		log.Debug("ECR token found in cache")
		return token.(string), true
	}
	log.Debug("ECR token not found in cache")
	return "", false
}

// clearECRTokenCache clears all cached ECR tokens (thread-safe)
func clearECRTokenCache() {
	ecrCacheMutex.Lock()
	defer ecrCacheMutex.Unlock()
	
	ecrTokenCache.Flush()
	log.Debug("Cleared all ECR tokens from cache")
}

// getECRCacheStats returns cache statistics for monitoring (thread-safe)
func getECRCacheStats() (int, int) {
	ecrCacheMutex.RLock()
	defer ecrCacheMutex.RUnlock()
	
	return ecrTokenCache.ItemCount(), len(ecrTokenCache.Items())
}

// calculateCacheExpiry calculates the cache expiry time for ECR tokens
// ECR tokens are valid for 12 hours, we cache for 11 hours to avoid expiry issues
func calculateCacheExpiry(tokenExpiry time.Time) time.Duration {
	const safetyMargin = time.Hour
	cacheExpiry := time.Until(tokenExpiry) - safetyMargin
	
	// Ensure minimum cache time of 1 minute to avoid excessive API calls
	const minimumCacheTime = time.Minute
	if cacheExpiry < minimumCacheTime {
		log.Warnf("ECR token expires very soon (%v), using minimum cache time", time.Until(tokenExpiry))
		return minimumCacheTime
	}
	
	// Cap maximum cache time at 11 hours for safety
	const maximumCacheTime = 11 * time.Hour
	if cacheExpiry > maximumCacheTime {
		log.Debugf("Calculated cache expiry exceeds maximum, capping at %v", maximumCacheTime)
		return maximumCacheTime
	}
	
	return cacheExpiry
}

var _ Creds = AWSECRWorkloadIdentityCreds{}

type AWSECRWorkloadIdentityCreds struct {
	repoURL            string
	region             string
	registryID         string
	CAPath             string
	CertData           []byte
	KeyData            []byte
	InsecureSkipVerify bool
}

func (creds AWSECRWorkloadIdentityCreds) GetUsername() string {
	return "AWS"
}

func (creds AWSECRWorkloadIdentityCreds) GetPassword() (string, error) {
	return creds.GetECRToken()
}

func (creds AWSECRWorkloadIdentityCreds) GetCAPath() string {
	return creds.CAPath
}

func (creds AWSECRWorkloadIdentityCreds) GetCertData() []byte {
	return creds.CertData
}

func (creds AWSECRWorkloadIdentityCreds) GetKeyData() []byte {
	return creds.KeyData
}

func (creds AWSECRWorkloadIdentityCreds) GetInsecureSkipVerify() bool {
	return creds.InsecureSkipVerify
}

func NewAWSECRWorkloadIdentityCreds(repoURL string, region string, registryID string, caPath string, certData []byte, keyData []byte, insecureSkipVerify bool) AWSECRWorkloadIdentityCreds {
	// Auto-detect region from ECR URL if not explicitly provided
	if region == "" {
		region = extractRegionFromECRURL(repoURL)
	}
	
	return AWSECRWorkloadIdentityCreds{
		repoURL:            repoURL,
		region:             region,
		registryID:         registryID,
		CAPath:             caPath,
		CertData:           certData,
		KeyData:            keyData,
		InsecureSkipVerify: insecureSkipVerify,
	}
}

// extractRegionFromECRURL extracts AWS region from ECR repository URL
// Example: 123456789.dkr.ecr.us-west-2.amazonaws.com -> us-west-2
func extractRegionFromECRURL(repoURL string) string {
	// Default fallback region
	defaultRegion := "us-east-1"
	
	// Remove any protocol prefix
	url := strings.TrimPrefix(repoURL, "oci://")
	url = strings.TrimPrefix(url, "https://")
	
	// ECR URL format: {account-id}.dkr.ecr.{region}.amazonaws.com
	parts := strings.Split(url, ".")
	if len(parts) >= 5 && parts[1] == "dkr" && parts[2] == "ecr" && strings.Contains(parts[4], "amazonaws") {
		// Extract region (should be parts[3])
		if len(parts) > 3 {
			region := parts[3]
			log.Debugf("Auto-detected AWS region '%s' from ECR URL: %s", region, repoURL)
			return region
		}
	}
	
	log.Warnf("Could not auto-detect region from ECR URL '%s', using default region '%s'", repoURL, defaultRegion)
	return defaultRegion
}

func (creds AWSECRWorkloadIdentityCreds) GetECRToken() (string, error) {
	registryHost := strings.Split(creds.repoURL, "/")[0]

	// Compute cache key for ECR token
	key, err := argoutils.GenerateCacheKey("ecrtoken-%s-%s", registryHost, creds.region)
	if err != nil {
		return "", fmt.Errorf("failed to compute cache key: %w", err)
	}

	// First, try to get from cache (read-only operation)
	if token, found := getCachedECRToken(key); found {
		return token, nil
	}

	// If not in cache, we need to fetch a new token
	// Use a separate lock to prevent multiple goroutines from fetching the same token simultaneously
	ecrCacheMutex.Lock()
	defer ecrCacheMutex.Unlock()

	// Double-check cache after acquiring write lock (another goroutine might have fetched it)
	if token, found := ecrTokenCache.Get(key); found {
		log.Debug("ECR token found in cache after acquiring lock")
		return token.(string), nil
	}

	log.Debugf("ECR token cache miss, fetching new token for registry %s in region %s", registryHost, creds.region)

	// Get new token from ECR
	token, expiry, err := creds.fetchECRToken()
	if err != nil {
		return "", fmt.Errorf("failed to fetch ECR token: %w", err)
	}

	// Calculate cache expiry with proper handling
	cacheExpiry := calculateCacheExpiry(expiry)
	
	// Store in cache (we already have the write lock)
	ecrTokenCache.Set(key, token, cacheExpiry)
	log.Debugf("Stored ECR token in cache with key %s, expires in %v", key, cacheExpiry)

	log.Infof("Successfully retrieved and cached ECR token for registry %s, expires at %v", registryHost, expiry)
	return token, nil
}

func (creds AWSECRWorkloadIdentityCreds) fetchECRToken() (string, time.Time, error) {
	ctx := context.Background()

	// Load AWS config (will use IRSA automatically)
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(creds.region))
	if err != nil {
		// Enhanced error messages for IRSA misconfiguration
		if strings.Contains(err.Error(), "no EC2 IMDS role found") || strings.Contains(err.Error(), "no credentials") {
			return "", time.Time{}, fmt.Errorf("IRSA configuration issue: ensure the service account has eks.amazonaws.com/role-arn annotation and the IAM role has ECR permissions: %w", err)
		}
		return "", time.Time{}, fmt.Errorf("failed to load AWS config for ECR authentication: %w", err)
	}

	// Create ECR client
	client := ecr.NewFromConfig(cfg)

	// Get authorization token
	input := &ecr.GetAuthorizationTokenInput{}
	if creds.registryID != "" {
		input.RegistryIds = []string{creds.registryID}
	}

	result, err := client.GetAuthorizationToken(ctx, input)
	if err != nil {
		// Enhanced error messages for ECR service issues
		if strings.Contains(err.Error(), "AccessDenied") {
			return "", time.Time{}, fmt.Errorf("ECR permissions issue: IAM role lacks 'ecr:GetAuthorizationToken' permission or ECR registry access: %w", err)
		}
		if strings.Contains(err.Error(), "UnrecognizedClientException") {
			return "", time.Time{}, fmt.Errorf("ECR authentication failed: invalid AWS credentials or IRSA configuration: %w", err)
		}
		if strings.Contains(err.Error(), "RepositoryNotFoundException") {
			return "", time.Time{}, fmt.Errorf("ECR registry not found: check registry ID '%s' and region '%s': %w", creds.registryID, creds.region, err)
		}
		return "", time.Time{}, fmt.Errorf("failed to get ECR authorization token: %w", err)
	}

	if len(result.AuthorizationData) == 0 {
		return "", time.Time{}, fmt.Errorf("no authorization data returned from ECR")
	}

	authData := result.AuthorizationData[0]
	if authData.AuthorizationToken == nil {
		return "", time.Time{}, fmt.Errorf("no authorization token in response")
	}

	// Decode the base64 token to get username:password
	decoded, err := base64.StdEncoding.DecodeString(*authData.AuthorizationToken)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to decode authorization token: %w", err)
	}

	// Extract password (format is "AWS:password")
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 || parts[0] != "AWS" {
		return "", time.Time{}, fmt.Errorf("invalid authorization token format")
	}

	expiry := time.Now()
	if authData.ExpiresAt != nil {
		expiry = *authData.ExpiresAt
	}

	return parts[1], expiry, nil
}
