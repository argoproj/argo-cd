package preview

import (
	"context"

	"github.com/argoproj/argo-cd/util/oauth"

	"github.com/google/go-github/github"
	log "github.com/sirupsen/logrus"
)

type StatusService struct {
	oAuthService oauth.OAuthService
}

func NewStatusService(oAuthService oauth.OAuthService) StatusService {
	return StatusService{oAuthService: oAuthService}
}

func (s StatusService) SetStatus(appName, owner, repo, sha, state string) error {

	description := appName
	switch state {
	case "pending":
		description = "Syncing " + appName
	case "failed":
		description = "Failed to sync " + appName
	case "success":
		description = "Successfully synced " + appName
	}
	log.Infof("setting status state=%s, description=%s", state, description)

	oauthClient, err := s.oAuthService.GetClient()
	if err != nil {
		return err
	}

	client := github.NewClient(oauthClient)

	name := "Argo CD"
	url := "http://cacae153.ngrok.io/applications"
	_, _, err = client.Repositories.CreateStatus(context.Background(), owner, repo, sha, &github.RepoStatus{
		Context:     &name,
		State:       &state,
		Description: &description,
		URL:         &url,
	})

	return err
}

func (s StatusService) AddMessageStatus(appName, owner, repo, sha, message string) error {
	oauthClient, err := s.oAuthService.GetClient()
	if err != nil {
		return err
	}

	client := github.NewClient(oauthClient)
	_, _, err = client.Repositories.CreateComment(context.Background(), owner, repo, sha, &github.RepositoryComment{
		Body: &message,
	})

	return err

}
