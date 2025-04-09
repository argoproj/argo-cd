package pull_request

import (
	"testing"

	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/require"
)

func TestGetGitHubPRLabelNames(t *testing.T) {
	tests := []struct {
		Name           string
		PullLabels     []githubLabel
		ExpectedResult []string
	}{
		{
			Name: "PR has labels",
			PullLabels: []githubLabel{
				{Name: githubv4.String("label1")},
				{Name: githubv4.String("label2")},
				{Name: githubv4.String("label3")},
			},
			ExpectedResult: []string{"label1", "label2", "label3"},
		},
		{
			Name:           "PR does not have labels",
			PullLabels:     []githubLabel{},
			ExpectedResult: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			labels := getGithubPRLabelNames(test.PullLabels)
			require.Equal(t, test.ExpectedResult, labels)
		})
	}
}

func TestGetGithubPRLabels(t *testing.T) {
	tests := []struct {
		Name           string
		Labels         []string
		ExpectedResult []githubv4.String
	}{
		{
			Name:   "Multiple labels",
			Labels: []string{"bug", "enhancement", "help wanted"},
			ExpectedResult: []githubv4.String{
				githubv4.String("bug"),
				githubv4.String("enhancement"),
				githubv4.String("help wanted"),
			},
		},
		{
			Name:           "No labels",
			Labels:         []string{},
			ExpectedResult: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			labels := getGithubPRLabels(test.Labels)
			require.Equal(t, test.ExpectedResult, labels)
		})
	}
}
