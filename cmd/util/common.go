package util

import (
	stderrors "errors"

	"github.com/argoproj/argo-cd/v3/util/errors"
)

var (
	LogFormat string
	LogLevel  string
)

func ValidateBearerTokenForHTTPSRepoOnly(bearerToken string, isHTTPS bool) {
	// Bearer token is only valid for HTTPS repositories
	if bearerToken != "" {
		if !isHTTPS {
			err := stderrors.New("--bearer-token is only supported for HTTPS repositories")
			errors.CheckError(err)
		}
	}
}

func ValidateBearerTokenForGitOnly(bearerToken string, repoType string) {
	// Bearer token is only valid for Git repositories
	if bearerToken != "" && repoType != "git" {
		err := stderrors.New("--bearer-token is only supported for Git repositories")
		errors.CheckError(err)
	}
}

func ValidateBearerTokenAndPasswordCombo(bearerToken string, password string) {
	// Either the password or the bearer token must be set, but not both
	if bearerToken != "" && password != "" {
		err := stderrors.New("only --bearer-token or --password is allowed, not both")
		errors.CheckError(err)
	}
}
