package pull_request

import (
	"context"
	"testing"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func strp(s string) *string {
	return &s
}
func TestFilterBranchMatchBadRegexp(t *testing.T) {
	provider, _ := NewFakeService(
		context.Background(),
		[]*PullRequest{
			{
				Number:  1,
				Branch:  "branch1",
				HeadSHA: "089d92cbf9ff857a39e6feccd32798ca700fb958",
			},
		},
		nil,
	)
	filters := []argoprojiov1alpha1.PullRequestGeneratorFilter{
		{
			BranchMatch: strp("("),
		},
	}
	_, err := ListPullRequests(context.Background(), provider, filters)
	assert.Error(t, err)
}

func TestFilterBranchMatch(t *testing.T) {
	provider, _ := NewFakeService(
		context.Background(),
		[]*PullRequest{
			{
				Number:  1,
				Branch:  "one",
				HeadSHA: "189d92cbf9ff857a39e6feccd32798ca700fb958",
			},
			{
				Number:  2,
				Branch:  "two",
				HeadSHA: "289d92cbf9ff857a39e6feccd32798ca700fb958",
			},
			{
				Number:  3,
				Branch:  "three",
				HeadSHA: "389d92cbf9ff857a39e6feccd32798ca700fb958",
			},
			{
				Number:  4,
				Branch:  "four",
				HeadSHA: "489d92cbf9ff857a39e6feccd32798ca700fb958",
			},
		},
		nil,
	)
	filters := []argoprojiov1alpha1.PullRequestGeneratorFilter{
		{
			BranchMatch: strp("w"),
		},
	}
	pullRequests, err := ListPullRequests(context.Background(), provider, filters)
	assert.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, "two", pullRequests[0].Branch)
}

func TestMultiFilterOr(t *testing.T) {
	provider, _ := NewFakeService(
		context.Background(),
		[]*PullRequest{
			{
				Number:  1,
				Branch:  "one",
				HeadSHA: "189d92cbf9ff857a39e6feccd32798ca700fb958",
			},
			{
				Number:  2,
				Branch:  "two",
				HeadSHA: "289d92cbf9ff857a39e6feccd32798ca700fb958",
			},
			{
				Number:  3,
				Branch:  "three",
				HeadSHA: "389d92cbf9ff857a39e6feccd32798ca700fb958",
			},
			{
				Number:  4,
				Branch:  "four",
				HeadSHA: "489d92cbf9ff857a39e6feccd32798ca700fb958",
			},
		},
		nil,
	)
	filters := []argoprojiov1alpha1.PullRequestGeneratorFilter{
		{
			BranchMatch: strp("w"),
		},
		{
			BranchMatch: strp("r"),
		},
	}
	pullRequests, err := ListPullRequests(context.Background(), provider, filters)
	assert.NoError(t, err)
	assert.Len(t, pullRequests, 3)
	assert.Equal(t, "two", pullRequests[0].Branch)
	assert.Equal(t, "three", pullRequests[1].Branch)
	assert.Equal(t, "four", pullRequests[2].Branch)
}

func TestNoFilters(t *testing.T) {
	provider, _ := NewFakeService(
		context.Background(),
		[]*PullRequest{
			{
				Number:  1,
				Branch:  "one",
				HeadSHA: "189d92cbf9ff857a39e6feccd32798ca700fb958",
			},
			{
				Number:  2,
				Branch:  "two",
				HeadSHA: "289d92cbf9ff857a39e6feccd32798ca700fb958",
			},
		},
		nil,
	)
	filters := []argoprojiov1alpha1.PullRequestGeneratorFilter{}
	repos, err := ListPullRequests(context.Background(), provider, filters)
	assert.NoError(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, "one", repos[0].Branch)
	assert.Equal(t, "two", repos[1].Branch)
}
