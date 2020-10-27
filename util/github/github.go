package github

import (
	"regexp"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/util/git"
	"github.com/dgrijalva/jwt-go"
)

var (
	ownerAndRepoRegex = regexp.MustCompile("\\/([\\w\\-\\_]+)\\/([\\w\\-\\_\\.]+)")
)

// Installation represents a GitHub Apps installation.
type Installation struct {
	ID          *int64                   `json:"id,omitempty"`
	Permissions *InstallationPermissions `json:"permissions,omitempty"`
}

// InstallationPermissions lists the repository and organization permissions for an installation.
//
// Permission names taken from:
//   https://docs.github.com/en/rest/reference/apps/permissions/
//   https://developer.github.com/enterprise/v3/apps/permissions/
type InstallationPermissions struct {
	Contents *string `json:"contents,omitempty"`
	Metadata *string `json:"metadata,omitempty"`
}

// InstallationAccessToken represents a GitHub Apps installation access token.
type InstallationAccessToken struct {
	Token       *int64                   `json:"token,omitempty"`
	ExpiresAt   *string                  `json:"expires_at,omitempty"`
	Permissions *InstallationPermissions `json:"permissions,omitempty"`
}

// Bearer create JWT bearer for GitHub App
func Bearer(appID, privateKey string) (string, error) {
	// TODO Consider caching in memory?
	now := time.Now()
	claims := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		// issued at time
		"iat": now.Unix(),
		// JWT expiration time (10 minute maximum)
		"exp": now.Add(time.Minute * 10).Unix(),
		// GitHub App's identifier
		"iss": appID,
	})
	bearer, err := claims.SignedString(privateKey)
	if err != nil {
		return "", err
	}
	return bearer, nil
}

// BaseURL fixes the base url for github api
func BaseURL(baseURL string) string {
	if baseURL == "" {
		baseURL = "https://api.github.com"
	} else {
		// unsure on the base url for enterprise: https://github.com/github/docs/discussions/832
		baseURL = strings.TrimRight(baseURL, "/")
		if !strings.HasSuffix(baseURL, "/api/v3") {
			baseURL += "/api/v3"
		}
	}
	return baseURL
}

// OwnerAndRepoName extracts owner and repo name from an normalized URL
func OwnerAndRepoName(repo string) (string, string) {
	repoURL := git.NormalizeGitURL(repo)
	matches := ownerAndRepoRegex.FindStringSubmatch(repoURL)
	if len(matches) > 2 {
		return matches[1], matches[2]
	}
	return "", ""
}
