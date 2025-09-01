package helm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAWSECRWorkloadIdentityCreds_GetUsername(t *testing.T) {
	creds := NewAWSECRWorkloadIdentityCreds("123456789.dkr.ecr.us-west-2.amazonaws.com", "", "", "", nil, nil, false)
	assert.Equal(t, "AWS", creds.GetUsername(), "ECR username should always be 'AWS'")
}

func TestAWSECRWorkloadIdentityCreds_GetCAPath(t *testing.T) {
	testCAPath := "/path/to/ca.crt"
	creds := NewAWSECRWorkloadIdentityCreds("123456789.dkr.ecr.us-west-2.amazonaws.com", "", "", testCAPath, nil, nil, false)
	assert.Equal(t, testCAPath, creds.GetCAPath())
}

func TestAWSECRWorkloadIdentityCreds_GetCertData(t *testing.T) {
	testCertData := []byte("test-cert-data")
	creds := NewAWSECRWorkloadIdentityCreds("123456789.dkr.ecr.us-west-2.amazonaws.com", "", "", "", testCertData, nil, false)
	assert.Equal(t, testCertData, creds.GetCertData())
}

func TestAWSECRWorkloadIdentityCreds_GetKeyData(t *testing.T) {
	testKeyData := []byte("test-key-data")
	creds := NewAWSECRWorkloadIdentityCreds("123456789.dkr.ecr.us-west-2.amazonaws.com", "", "", "", nil, testKeyData, false)
	assert.Equal(t, testKeyData, creds.GetKeyData())
}

func TestAWSECRWorkloadIdentityCreds_GetInsecureSkipVerify(t *testing.T) {
	t.Run("insecure=true", func(t *testing.T) {
		creds := NewAWSECRWorkloadIdentityCreds("123456789.dkr.ecr.us-west-2.amazonaws.com", "", "", "", nil, nil, true)
		assert.True(t, creds.GetInsecureSkipVerify())
	})
	
	t.Run("insecure=false", func(t *testing.T) {
		creds := NewAWSECRWorkloadIdentityCreds("123456789.dkr.ecr.us-west-2.amazonaws.com", "", "", "", nil, nil, false)
		assert.False(t, creds.GetInsecureSkipVerify())
	})
}

func TestExtractRegionFromECRURL(t *testing.T) {
	testCases := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "Standard ECR URL",
			url:      "123456789.dkr.ecr.us-west-2.amazonaws.com",
			expected: "us-west-2",
		},
		{
			name:     "ECR URL with OCI prefix",
			url:      "oci://123456789.dkr.ecr.eu-central-1.amazonaws.com",
			expected: "eu-central-1",
		},
		{
			name:     "ECR URL with HTTPS prefix",
			url:      "https://123456789.dkr.ecr.ap-south-1.amazonaws.com",
			expected: "ap-south-1",
		},
		{
			name:     "Invalid URL format",
			url:      "invalid.url.com",
			expected: "us-east-1", // default
		},
		{
			name:     "Empty URL",
			url:      "",
			expected: "us-east-1", // default
		},
		{
			name:     "ECR URL with path",
			url:      "oci://123456789.dkr.ecr.us-east-1.amazonaws.com/my-repo",
			expected: "us-east-1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractRegionFromECRURL(tc.url)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestCalculateCacheExpiry(t *testing.T) {
	now := time.Now()

	t.Run("Normal 12-hour token", func(t *testing.T) {
		tokenExpiry := now.Add(12 * time.Hour)
		cacheExpiry := calculateCacheExpiry(tokenExpiry)
		
		// Should cache for 11 hours (12 - 1 hour safety margin)
		expected := 11 * time.Hour
		assert.InDelta(t, expected.Seconds(), cacheExpiry.Seconds(), 60, "Cache expiry should be ~11 hours")
	})

	t.Run("Token expiring soon", func(t *testing.T) {
		tokenExpiry := now.Add(30 * time.Second)
		cacheExpiry := calculateCacheExpiry(tokenExpiry)
		
		// Should use minimum cache time of 1 minute
		expected := time.Minute
		assert.Equal(t, expected, cacheExpiry)
	})

	t.Run("Very long token expiry", func(t *testing.T) {
		tokenExpiry := now.Add(24 * time.Hour)
		cacheExpiry := calculateCacheExpiry(tokenExpiry)
		
		// Should cap at 11 hours maximum
		expected := 11 * time.Hour
		assert.Equal(t, expected, cacheExpiry)
	})
}

func TestECRTokenCacheOperations(t *testing.T) {
	// Clear cache before test
	clearECRTokenCache()
	
	testKey := "test-ecr-token-key"
	testToken := "test-ecr-token-value"
	expiry := 5 * time.Minute

	t.Run("Store and retrieve token", func(t *testing.T) {
		// Store token
		storeECRToken(testKey, testToken, expiry)
		
		// Retrieve token
		token, found := getCachedECRToken(testKey)
		assert.True(t, found, "Token should be found in cache")
		assert.Equal(t, testToken, token, "Retrieved token should match stored token")
	})

	t.Run("Cache miss", func(t *testing.T) {
		// Try to get non-existent token
		token, found := getCachedECRToken("non-existent-key")
		assert.False(t, found, "Non-existent token should not be found")
		assert.Equal(t, "", token, "Token should be empty string for cache miss")
	})

	t.Run("Clear cache", func(t *testing.T) {
		// Store a token
		storeECRToken("test-key-2", "test-token-2", 5*time.Minute)
		
		// Verify it exists
		_, found := getCachedECRToken("test-key-2")
		assert.True(t, found, "Token should exist before clear")
		
		// Clear cache
		clearECRTokenCache()
		
		// Verify it's gone
		_, found = getCachedECRToken("test-key-2")
		assert.False(t, found, "Token should not exist after clear")
	})

	t.Run("Invalid expiration", func(t *testing.T) {
		initialCount, _ := getECRCacheStats()
		
		// Try to store with invalid expiration
		storeECRToken("invalid-key", "invalid-token", -time.Minute)
		
		// Should not be stored
		_, found := getCachedECRToken("invalid-key")
		assert.False(t, found, "Token with invalid expiration should not be stored")
		
		// Cache count should not change
		finalCount, _ := getECRCacheStats()
		assert.Equal(t, initialCount, finalCount, "Cache count should not change for invalid expiration")
	})
}

func TestNewAWSECRWorkloadIdentityCreds_RegionAutoDetection(t *testing.T) {
	t.Run("Auto-detect region from URL", func(t *testing.T) {
		repoURL := "123456789.dkr.ecr.us-west-2.amazonaws.com"
		creds := NewAWSECRWorkloadIdentityCreds(repoURL, "", "", "", nil, nil, false)
		assert.Equal(t, "us-west-2", creds.region, "Region should be auto-detected from URL")
	})

	t.Run("Explicit region overrides auto-detection", func(t *testing.T) {
		repoURL := "123456789.dkr.ecr.us-west-2.amazonaws.com"
		explicitRegion := "eu-central-1"
		creds := NewAWSECRWorkloadIdentityCreds(repoURL, explicitRegion, "", "", nil, nil, false)
		assert.Equal(t, explicitRegion, creds.region, "Explicit region should override auto-detection")
	})

	t.Run("Invalid URL uses default region", func(t *testing.T) {
		repoURL := "invalid.url.com"
		creds := NewAWSECRWorkloadIdentityCreds(repoURL, "", "", "", nil, nil, false)
		assert.Equal(t, "us-east-1", creds.region, "Should use default region for invalid URL")
	})
}

func TestAWSECRWorkloadIdentityCreds_Interface(t *testing.T) {
	// Verify that AWSECRWorkloadIdentityCreds implements Creds interface
	var _ Creds = AWSECRWorkloadIdentityCreds{}
	
	// Test all interface methods are implemented
	creds := NewAWSECRWorkloadIdentityCreds("123456789.dkr.ecr.us-west-2.amazonaws.com", "", "", "", nil, nil, false)
	
	assert.Equal(t, "AWS", creds.GetUsername())
	assert.Equal(t, "", creds.GetCAPath())
	assert.Nil(t, creds.GetCertData())
	assert.Nil(t, creds.GetKeyData())
	assert.False(t, creds.GetInsecureSkipVerify())
}
