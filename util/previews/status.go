package previews

import (
	"context"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"

	"github.com/argoproj/argo-cd/util/oauth"

	"github.com/google/go-github/github"
	log "github.com/sirupsen/logrus"
)

type StatusService struct {
	oAuthService oauth.OAuthService
	url          string
}

func NewStatusService(oAuthService oauth.OAuthService, url string) StatusService {
	return StatusService{oAuthService: oAuthService, url: url}
}

func (s StatusService) SetStatus(app v1alpha1.Application, sha, state string) error {

	log.Infof("settings status for appName=%s, sha=%s, state=%s", app.Name, sha, state)

	description := app.Name
	switch state {
	case "created":
		description = app.Name + " created"
	case "pending":
		description = "Syncing " + app.Name
	case "failed":
		description = "Failed to sync " + app.Name
	case "success":
		description = "Successfully synced " + app.Name
	}
	log.Infof("setting status state=%s, description=%s", state, description)

	oauthClient, err := s.oAuthService.GetClient()
	if err != nil {
		return err
	}

	client := github.NewClient(oauthClient)

	name := "Argo CD"
	url := s.url + "/applications/" + app.Name

	_, _, err = client.Repositories.CreateStatus(context.Background(), app.Spec.Preview.Owner, app.Spec.Preview.Repo, sha, &github.RepoStatus{
		Context:     &name,
		State:       &state,
		Description: &description,
		URL:         &url,
	})

	return err
}
