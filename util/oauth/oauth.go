package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"golang.org/x/oauth2/github"

	"github.com/argoproj/argo-cd/util/settings"

	"golang.org/x/oauth2"
)

const oauthStateString = "13451346245735"
const tokenFile = "token"

type OAuthService struct {
	settings settings.ArgoCDSettings
}

func NewOAuthService(settings settings.ArgoCDSettings) OAuthService {

	return OAuthService{settings: settings}
}

func (s *OAuthService) NewAuthorizeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if s.getConfig().ClientID == "" {
			http.Error(w, "clientID not configured", http.StatusServiceUnavailable)
			return
		}
		http.Redirect(w, r, s.getConfig().AuthCodeURL(oauthStateString, oauth2.AccessTypeOnline), http.StatusTemporaryRedirect)
	}
}

func (s *OAuthService) NewTokenHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.getConfig().ClientID == "" {
			http.Error(w, "clientID not configured", http.StatusServiceUnavailable)
			return
		}
		err := r.ParseForm()
		if err != nil {
			http.Error(w, fmt.Sprintf("could not parse query: %v", err), http.StatusBadRequest)
			return
		}

		state := r.FormValue("state")
		if state != oauthStateString {
			http.Error(w, fmt.Sprintf("invalid oauth state, expected '%s', got '%s'", oauthStateString, state), http.StatusBadRequest)
			return
		}

		code := r.FormValue("code")

		token, err := s.getConfig().Exchange(context.Background(), code)
		if err != nil {
			http.Error(w, fmt.Sprintf("token exchange failed: %v", err), http.StatusInternalServerError)
			return
		}

		bytes, err := json.Marshal(token)
		if err != nil {
			http.Error(w, fmt.Sprintf("marshalling token failed: %v", err), http.StatusInternalServerError)
			return
		}
		err = ioutil.WriteFile(tokenFile, bytes, 0600)
		if err != nil {
			http.Error(w, fmt.Sprintf("saving token failed: %v", err), http.StatusInternalServerError)
			return
		}

		http.Error(w, "Complete", http.StatusOK)
	}
}

func (s *OAuthService) getConfig() *oauth2.Config {

	return &oauth2.Config{
		ClientID:     s.settings.GitHubClientID,
		ClientSecret: s.settings.GitHubClientSecret,
		Scopes:       []string{"public_repo", "repo:status"},
		Endpoint:     github.Endpoint,
	}
}

func (s *OAuthService) GetClient() (*http.Client, error) {
	bytes, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		return nil, err
	}
	var token *oauth2.Token
	err = json.Unmarshal(bytes, &token)
	if err != nil {
		return nil, err
	}
	return s.getConfig().Client(context.Background(), token), nil
}
