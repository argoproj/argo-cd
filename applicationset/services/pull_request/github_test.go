package pull_request

import (
	"testing"

	"github.com/google/go-github/v63/github"
	"github.com/stretchr/testify/assert"
)

func toPtr(s string) *string {
	return &s
}

func TestContainLabels(t *testing.T) {
	cases := []struct {
		Name       string
		Labels     []string
		PullLabels []*github.Label
		Expect     bool
	}{
		{
			Name:   "Match labels",
			Labels: []string{"label1", "label2"},
			PullLabels: []*github.Label{
				{Name: toPtr("label1")},
				{Name: toPtr("label2")},
				{Name: toPtr("label3")},
			},
			Expect: true,
		},
		{
			Name:   "Not match labels",
			Labels: []string{"label1", "label4"},
			PullLabels: []*github.Label{
				{Name: toPtr("label1")},
				{Name: toPtr("label2")},
				{Name: toPtr("label3")},
			},
			Expect: false,
		},
		{
			Name:   "No specify",
			Labels: []string{},
			PullLabels: []*github.Label{
				{Name: toPtr("label1")},
				{Name: toPtr("label2")},
				{Name: toPtr("label3")},
			},
			Expect: true,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if got := containLabels(c.Labels, c.PullLabels); got != c.Expect {
				t.Errorf("expect: %v, got: %v", c.Expect, got)
			}
		})
	}
}

func TestGetGitHubPRLabelNames(t *testing.T) {
	Tests := []struct {
		Name           string
		PullLabels     []*github.Label
		ExpectedResult []string
	}{
		{
			Name: "PR has labels",
			PullLabels: []*github.Label{
				{Name: toPtr("label1")},
				{Name: toPtr("label2")},
				{Name: toPtr("label3")},
			},
			ExpectedResult: []string{"label1", "label2", "label3"},
		},
		{
			Name:           "PR does not have labels",
			PullLabels:     []*github.Label{},
			ExpectedResult: nil,
		},
	}
	for _, test := range Tests {
		t.Run(test.Name, func(t *testing.T) {
			labels := getGithubPRLabelNames(test.PullLabels)
			assert.Equal(t, test.ExpectedResult, labels)
		})
	}
}
