package creds

import (
	"net/url"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/cert"
	"github.com/argoproj/argo-cd/util/git"
)

func GetRepoCreds(repo *v1alpha1.Repository) git.Creds {
	if repo == nil {
		return git.NopCreds{}
	}
	if repo.Username != "" && repo.Password != "" {
		return git.NewHTTPSCreds(repo.Username, repo.Password, repo.TLSClientCertData, repo.TLSClientCertKey, repo.TLSClientCAData, repo.IsInsecure())
	}
	if repo.SSHPrivateKey != "" {
		return git.NewSSHCreds(repo.SSHPrivateKey, getCAPath(repo.Repo), repo.IsInsecure())
	}
	return git.NopCreds{}
}

func getCAPath(repoURL string) string {
	if git.IsHTTPSURL(repoURL) {
		if parsedURL, err := url.Parse(repoURL); err == nil {
			if caPath, err := cert.GetCertBundlePathForRepository(parsedURL.Host); err != nil {
				return caPath
			} else {
				log.Warnf("Could not get cert bundle path for host '%s'", parsedURL.Host)
			}
		} else {
			// We don't fail if we cannot parse the URL, but log a warning in that
			// case. And we execute the command in a verbatim way.
			log.Warnf("Could not parse repo URL '%s'", repoURL)
		}
	}
	return ""
}
