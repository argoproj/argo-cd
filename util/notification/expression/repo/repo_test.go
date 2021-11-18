package repo

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/argoproj-labs/argocd-notifications/expr/shared"
	"github.com/argoproj-labs/argocd-notifications/shared/argocd/mocks"
)

func TestToHttps(t *testing.T) {
	for in, expected := range map[string]string{
		"git@github.com:argoproj/argo-cd.git":    "https://github.com/argoproj/argo-cd.git",
		"http://github.com/argoproj/argo-cd.git": "https://github.com/argoproj/argo-cd.git",
	} {
		actual := repoURLToHTTPS(in)
		assert.Equal(t, actual, expected)
	}
}

func TestParseFullName(t *testing.T) {
	for in, expected := range map[string]string{
		"git@github.com:argoproj/argo-cd.git":             "argoproj/argo-cd",
		"http://github.com/argoproj/argo-cd.git":          "argoproj/argo-cd",
		"http://github.com/argoproj/argo-cd":              "argoproj/argo-cd",
		"https://user@bitbucket.org/argoproj/argo-cd.git": "argoproj/argo-cd",
		"git@gitlab.com:argoproj/argo-cd.git":             "argoproj/argo-cd",
		"https://gitlab.com/argoproj/argo-cd.git":         "argoproj/argo-cd",
	} {
		actual := FullNameByRepoURL(in)
		assert.Equal(t, actual, expected)
	}
}

func TestGetCommitMetadata(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	argocdService := mocks.NewMockService(ctrl)
	expectedMeta := &shared.CommitMetadata{Message: "hello"}
	argocdService.EXPECT().GetCommitMetadata(context.Background(), "http://myrepo-url.git", "abc").Return(expectedMeta, nil)
	commitMeta, err := getCommitMetadata("abc", NewApp("guestbook", WithRepoURL("http://myrepo-url.git")), argocdService)

	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, expectedMeta, commitMeta)

}
