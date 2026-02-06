package workloadidentity

import (
	"fmt"
	"time"

	azcloud "github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
)

const (
	EmptyGuid = "00000000-0000-0000-0000-000000000000" //nolint:revive //FIXME(var-naming)
)

type Token struct {
	AccessToken string
	ExpiresOn   time.Time
}

type TokenProvider interface {
	GetToken(scope string) (*Token, error)
	GetCloudConfiguration() azcloud.Configuration
}

// Used to propagate initialization error if any
var initError error

func CalculateCacheExpiryBasedOnTokenExpiry(tokenExpiry time.Time) time.Duration {
	// Calculate the cache expiry as 5 minutes before the token expires
	cacheExpiry := time.Until(tokenExpiry) - time.Minute*5
	return cacheExpiry
}

func GetAzureCloudConfigByName(cloudName string) (azcloud.Configuration, error) {
	if cloudName != "" && cloudName != "AzurePublic" && cloudName != "AzureChina" && cloudName != "AzureUSGovernment" {
		return azcloud.Configuration{}, fmt.Errorf("could not parse Azure cloud '%s' (possible values are: AzurePublic, AzureChina, AzureUSGovernment)", cloudName)
	}

	cloud := azcloud.AzurePublic
	switch cloudName {
	case "AzureChina":
		cloud = azcloud.AzureChina
	case "AzureUSGovernment":
		cloud = azcloud.AzureGovernment
	}
	return cloud, nil
}
