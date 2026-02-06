package workloadidentity

import (
	"time"
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
}

// Used to propagate initialization error if any
var initError error

func CalculateCacheExpiryBasedOnTokenExpiry(tokenExpiry time.Time) time.Duration {
	// Calculate the cache expiry as 5 minutes before the token expires
	cacheExpiry := time.Until(tokenExpiry) - time.Minute*5
	return cacheExpiry
}
