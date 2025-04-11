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
