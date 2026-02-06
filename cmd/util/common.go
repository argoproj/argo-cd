package util

import (
	stderrors "errors"
)

var (
	LogFormat string
	LogLevel  string
)

func ValidateBearerTokenForHTTPSRepoOnly(bearerToken string, isHTTPS bool) error {
	// Bearer token is only valid for HTTPS repositories
	if bearerToken != "" {
		if !isHTTPS {
			err := stderrors.New("--bearer-token is only supported for HTTPS repositories")
			return err
		}
	}
	return nil
}

func ValidateBearerTokenForGitOnly(bearerToken string, repoType string) error {
	// Bearer token is only valid for Git repositories
	if bearerToken != "" && repoType != "git" {
		err := stderrors.New("--bearer-token is only supported for Git repositories")
		return err
	}
	return nil
}

func ValidateBearerTokenAndPasswordCombo(bearerToken string, password string) error {
	// Either the password or the bearer token must be set, but not both
	if bearerToken != "" && password != "" {
		err := stderrors.New("only --bearer-token or --password is allowed, not both")
		return err
	}
	return nil
}
