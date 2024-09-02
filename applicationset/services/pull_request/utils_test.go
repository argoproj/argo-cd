package pull_request

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func strp(s string) *string {
	return &s
}

func TestFilterBranchMatchBadRegexp(t *testing.T) {
	provider, _ := NewFakeService(
		context.Background(),
		[]*PullRequest{
			{
				Number:       1,
				Title:        "PR branch1",
				Branch:       "branch1",
				TargetBranch: "master",
				HeadSHA:      "089d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name1",
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
	require.Error(t, err)
}

func TestFilterBranchMatch(t *testing.T) {
	provider, _ := NewFakeService(
		context.Background(),
		[]*PullRequest{
			{
				Number:       1,
				Title:        "PR one",
				Branch:       "one",
				TargetBranch: "master",
				HeadSHA:      "189d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name1",
			},
			{
				Number:       2,
				Title:        "PR two",
				Branch:       "two",
				TargetBranch: "master",
				HeadSHA:      "289d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name2",
			},
			{
				Number:       3,
				Title:        "PR three",
				Branch:       "three",
				TargetBranch: "master",
				HeadSHA:      "389d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name3",
			},
			{
				Number:       4,
				Title:        "PR four",
				Branch:       "four",
				TargetBranch: "master",
				HeadSHA:      "489d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name4",
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
	require.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, "two", pullRequests[0].Branch)
}

func TestFilterTargetBranchMatch(t *testing.T) {
	provider, _ := NewFakeService(
		context.Background(),
		[]*PullRequest{
			{
				Number:       1,
				Title:        "PR one",
				Branch:       "one",
				TargetBranch: "master",
				HeadSHA:      "189d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name1",
			},
			{
				Number:       2,
				Title:        "PR two",
				Branch:       "two",
				TargetBranch: "branch1",
				HeadSHA:      "289d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name2",
			},
			{
				Number:       3,
				Title:        "PR three",
				Branch:       "three",
				TargetBranch: "branch2",
				HeadSHA:      "389d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name3",
			},
			{
				Number:       4,
				Title:        "PR four",
				Branch:       "four",
				TargetBranch: "branch3",
				HeadSHA:      "489d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name4",
			},
		},
		nil,
	)
	filters := []argoprojiov1alpha1.PullRequestGeneratorFilter{
		{
			TargetBranchMatch: strp("1"),
		},
	}
	pullRequests, err := ListPullRequests(context.Background(), provider, filters)
	require.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, "two", pullRequests[0].Branch)
}

func TestMultiFilterOr(t *testing.T) {
	provider, _ := NewFakeService(
		context.Background(),
		[]*PullRequest{
			{
				Number:       1,
				Title:        "PR one",
				Branch:       "one",
				TargetBranch: "master",
				HeadSHA:      "189d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name1",
			},
			{
				Number:       2,
				Title:        "PR two",
				Branch:       "two",
				TargetBranch: "master",
				HeadSHA:      "289d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name2",
			},
			{
				Number:       3,
				Title:        "PR three",
				Branch:       "three",
				TargetBranch: "master",
				HeadSHA:      "389d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name3",
			},
			{
				Number:       4,
				Title:        "PR four",
				Branch:       "four",
				TargetBranch: "master",
				HeadSHA:      "489d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name4",
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
	require.NoError(t, err)
	assert.Len(t, pullRequests, 3)
	assert.Equal(t, "two", pullRequests[0].Branch)
	assert.Equal(t, "three", pullRequests[1].Branch)
	assert.Equal(t, "four", pullRequests[2].Branch)
}

func TestMultiFilterOrWithTargetBranchFilter(t *testing.T) {
	provider, _ := NewFakeService(
		context.Background(),
		[]*PullRequest{
			{
				Number:       1,
				Title:        "PR one",
				Branch:       "one",
				TargetBranch: "master",
				HeadSHA:      "189d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name1",
			},
			{
				Number:       2,
				Title:        "PR two",
				Branch:       "two",
				TargetBranch: "branch1",
				HeadSHA:      "289d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name2",
			},
			{
				Number:       3,
				Title:        "PR three",
				Branch:       "three",
				TargetBranch: "branch2",
				HeadSHA:      "389d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name3",
			},
			{
				Number:       4,
				Title:        "PR four",
				Branch:       "four",
				TargetBranch: "branch3",
				HeadSHA:      "489d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name4",
			},
		},
		nil,
	)
	filters := []argoprojiov1alpha1.PullRequestGeneratorFilter{
		{
			BranchMatch:       strp("w"),
			TargetBranchMatch: strp("1"),
		},
		{
			BranchMatch:       strp("r"),
			TargetBranchMatch: strp("3"),
		},
	}
	pullRequests, err := ListPullRequests(context.Background(), provider, filters)
	require.NoError(t, err)
	assert.Len(t, pullRequests, 2)
	assert.Equal(t, "two", pullRequests[0].Branch)
	assert.Equal(t, "four", pullRequests[1].Branch)
}

func TestNoFilters(t *testing.T) {
	provider, _ := NewFakeService(
		context.Background(),
		[]*PullRequest{
			{
				Number:       1,
				Title:        "PR one",
				Branch:       "one",
				TargetBranch: "master",
				HeadSHA:      "189d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name1",
			},
			{
				Number:       2,
				Title:        "PR two",
				Branch:       "two",
				TargetBranch: "master",
				HeadSHA:      "289d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name2",
			},
		},
		nil,
	)
	filters := []argoprojiov1alpha1.PullRequestGeneratorFilter{}
	repos, err := ListPullRequests(context.Background(), provider, filters)
	require.NoError(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, "one", repos[0].Branch)
	assert.Equal(t, "two", repos[1].Branch)
}
