package github_app

import (
	"testing"

	"github.com/aburan28/httpcache"
	"github.com/argoproj/argo-cd/v3/applicationset/services/github_app_auth"
	"github.com/google/go-github/v66/github"
)

func TestClient(t *testing.T) {
	tests := []struct {
		name string
		github_app_auth.Authentication
		url   string
		cache httpcache.Cache
		want  *github.Client
	}{}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			got, err := Client(tt.Authentication, tt.url, tt.cache)
			if err != nil {
				t.Errorf("Client() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Client() got = %v, want %v", got, tt.want)
			}
		})
	}

}
