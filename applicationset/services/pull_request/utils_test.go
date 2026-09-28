package pull_request

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestFilterBranchMatchBadRegexp(t *testing.T) {
	t.Parallel()
	provider, _ := NewFakeService(
		t.Context(),
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
			BranchMatch: new("("),
		},
	}
	_, err := ListPullRequests(t.Context(), provider, filters)
	require.Error(t, err)
}

func TestFilterBranchMatchLookahead(t *testing.T) {
	t.Parallel()
	provider, _ := NewFakeService(
		t.Context(),
		[]*PullRequest{
			{
				Number:       1,
				Title:        "PR feature",
				Branch:       "feature/one",
				TargetBranch: "master",
				HeadSHA:      "189d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name1",
			},
			{
				Number:       2,
				Title:        "PR release",
				Branch:       "release/1.0",
				TargetBranch: "master",
				HeadSHA:      "289d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name2",
			},
		},
		nil,
	)
	filters := []argoprojiov1alpha1.PullRequestGeneratorFilter{
		{
			BranchMatch: new("^(?!release/).*"),
		},
	}
	pullRequests, err := ListPullRequests(t.Context(), provider, filters)
	require.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, "feature/one", pullRequests[0].Branch)
}

func TestFilterTitleMatchLookbehind(t *testing.T) {
	t.Parallel()
	provider, _ := NewFakeService(
		t.Context(),
		[]*PullRequest{
			{
				Number:       1,
				Title:        "WIP: add feature",
				Branch:       "one",
				TargetBranch: "master",
				HeadSHA:      "189d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name1",
			},
			{
				Number:       2,
				Title:        "add feature",
				Branch:       "two",
				TargetBranch: "master",
				HeadSHA:      "289d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name2",
			},
		},
		nil,
	)
	filters := []argoprojiov1alpha1.PullRequestGeneratorFilter{
		{
			TitleMatch: new("(?<=WIP: ).*"),
		},
	}
	pullRequests, err := ListPullRequests(t.Context(), provider, filters)
	require.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, "one", pullRequests[0].Branch)
}

// Patterns written for Go's regexp package must keep their meaning: \d only matches ASCII digits.
func TestFilterBranchMatchKeepsGoRegexpSemantics(t *testing.T) {
	t.Parallel()
	provider, _ := NewFakeService(
		t.Context(),
		[]*PullRequest{
			{
				Number:       1,
				Title:        "PR ascii",
				Branch:       "feature-123",
				TargetBranch: "master",
				HeadSHA:      "189d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name1",
			},
			{
				Number:       2,
				Title:        "PR arabic-indic digits",
				Branch:       "feature-1١٢",
				TargetBranch: "master",
				HeadSHA:      "289d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name2",
			},
		},
		nil,
	)
	filters := []argoprojiov1alpha1.PullRequestGeneratorFilter{
		{
			BranchMatch: new(`^feature-1\d{2}$`),
		},
	}
	pullRequests, err := ListPullRequests(t.Context(), provider, filters)
	require.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, "feature-123", pullRequests[0].Branch)
}

func TestFilterTitleMatchNamedGroup(t *testing.T) {
	t.Parallel()
	provider, _ := NewFakeService(
		t.Context(),
		[]*PullRequest{
			{
				Number:       1,
				Title:        "feat: add thing",
				Branch:       "one",
				TargetBranch: "master",
				HeadSHA:      "189d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name1",
			},
			{
				Number:       2,
				Title:        "chore: tidy",
				Branch:       "two",
				TargetBranch: "master",
				HeadSHA:      "289d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name2",
			},
		},
		nil,
	)
	filters := []argoprojiov1alpha1.PullRequestGeneratorFilter{
		{
			TitleMatch: new(`^(?P<type>feat|fix): .*`),
		},
	}
	pullRequests, err := ListPullRequests(t.Context(), provider, filters)
	require.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, "one", pullRequests[0].Branch)
}

// A filter that cannot be evaluated must fail generation. Silently dropping the pull request would make the
// generator succeed with fewer results, and the ApplicationSet controller would delete the pull request's Application.
func TestFilterMatchErrorFailsGeneration(t *testing.T) {
	t.Parallel()
	evilInput := strings.Repeat("a", 28) + "!"
	tests := []struct {
		name   string
		filter argoprojiov1alpha1.PullRequestGeneratorFilter
	}{
		{"BranchMatch", argoprojiov1alpha1.PullRequestGeneratorFilter{BranchMatch: new(`^(a+)+$`)}},
		{"TargetBranchMatch", argoprojiov1alpha1.PullRequestGeneratorFilter{TargetBranchMatch: new(`^(a+)+$`)}},
		{"TitleMatch", argoprojiov1alpha1.PullRequestGeneratorFilter{TitleMatch: new(`^(a+)+$`)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider, _ := NewFakeService(
				t.Context(),
				[]*PullRequest{
					{
						Number:       1,
						Title:        evilInput,
						Branch:       evilInput,
						TargetBranch: evilInput,
						HeadSHA:      "189d92cbf9ff857a39e6feccd32798ca700fb958",
						Author:       "name1",
					},
				},
				nil,
			)
			pullRequests, err := ListPullRequests(t.Context(), provider, []argoprojiov1alpha1.PullRequestGeneratorFilter{tt.filter})
			require.Error(t, err)
			assert.Nil(t, pullRequests)
		})
	}
}

func TestFilterBranchMatch(t *testing.T) {
	t.Parallel()
	provider, _ := NewFakeService(
		t.Context(),
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
			BranchMatch: new("w"),
		},
	}
	pullRequests, err := ListPullRequests(t.Context(), provider, filters)
	require.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, "two", pullRequests[0].Branch)
}

func TestFilterTargetBranchMatch(t *testing.T) {
	t.Parallel()
	provider, _ := NewFakeService(
		t.Context(),
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
			TargetBranchMatch: new("1"),
		},
	}
	pullRequests, err := ListPullRequests(t.Context(), provider, filters)
	require.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, "two", pullRequests[0].Branch)
}

func TestFilterTitleMatch(t *testing.T) {
	t.Parallel()
	provider, _ := NewFakeService(
		t.Context(),
		[]*PullRequest{
			{
				Number:       1,
				Title:        "PR one - filter",
				Branch:       "one",
				TargetBranch: "master",
				HeadSHA:      "189d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name1",
			},
			{
				Number:       2,
				Title:        "PR two - ignore",
				Branch:       "two",
				TargetBranch: "branch1",
				HeadSHA:      "289d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name2",
			},
			{
				Number:       3,
				Title:        "[filter] PR three",
				Branch:       "three",
				TargetBranch: "branch2",
				HeadSHA:      "389d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name3",
			},
			{
				Number:       4,
				Title:        "[ignore] PR four",
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
			TitleMatch: new("\\[filter]"),
		},
	}
	pullRequests, err := ListPullRequests(t.Context(), provider, filters)
	require.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, "three", pullRequests[0].Branch)
}

func TestMultiFilterOrWithTitle(t *testing.T) {
	t.Parallel()
	provider, _ := NewFakeService(
		t.Context(),
		[]*PullRequest{
			{
				Number:       1,
				Title:        "PR one - filter",
				Branch:       "one",
				TargetBranch: "master",
				HeadSHA:      "189d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name1",
			},
			{
				Number:       2,
				Title:        "PR two - ignore",
				Branch:       "two",
				TargetBranch: "branch1",
				HeadSHA:      "289d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name2",
			},
			{
				Number:       3,
				Title:        "[filter] PR three",
				Branch:       "three",
				TargetBranch: "branch2",
				HeadSHA:      "389d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name3",
			},
			{
				Number:       4,
				Title:        "[ignore] PR four",
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
			TitleMatch: new("\\[filter]"),
		},
		{
			TitleMatch: new("- filter"),
		},
	}
	pullRequests, err := ListPullRequests(t.Context(), provider, filters)
	require.NoError(t, err)
	assert.Len(t, pullRequests, 2)
	assert.Equal(t, "one", pullRequests[0].Branch)
	assert.Equal(t, "three", pullRequests[1].Branch)
}

func TestMultiFilterOr(t *testing.T) {
	t.Parallel()
	provider, _ := NewFakeService(
		t.Context(),
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
			BranchMatch: new("w"),
		},
		{
			BranchMatch: new("r"),
		},
	}
	pullRequests, err := ListPullRequests(t.Context(), provider, filters)
	require.NoError(t, err)
	assert.Len(t, pullRequests, 3)
	assert.Equal(t, "two", pullRequests[0].Branch)
	assert.Equal(t, "three", pullRequests[1].Branch)
	assert.Equal(t, "four", pullRequests[2].Branch)
}

func TestMultiFilterOrWithTargetBranchFilterOrWithTitleFilter(t *testing.T) {
	t.Parallel()
	provider, _ := NewFakeService(
		t.Context(),
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
			{
				Number:       5,
				Title:        "PR title is different than branch name",
				Branch:       "five",
				TargetBranch: "branch3",
				HeadSHA:      "489d92cbf9ff857a39e6feccd32798ca700fb958",
				Author:       "name5",
			},
		},
		nil,
	)
	filters := []argoprojiov1alpha1.PullRequestGeneratorFilter{
		{
			BranchMatch:       new("w"),
			TargetBranchMatch: new("1"),
		},
		{
			BranchMatch:       new("r"),
			TargetBranchMatch: new("3"),
		},
		{
			TitleMatch: new("two"),
		},
		{
			BranchMatch: new("five"),
			TitleMatch:  new("PR title is different than branch name"),
		},
	}
	pullRequests, err := ListPullRequests(t.Context(), provider, filters)
	require.NoError(t, err)
	assert.Len(t, pullRequests, 3)
	assert.Equal(t, "two", pullRequests[0].Branch)
	assert.Equal(t, "four", pullRequests[1].Branch)
	assert.Equal(t, "five", pullRequests[2].Branch)
	assert.Equal(t, "PR title is different than branch name", pullRequests[2].Title)
}

func TestNoFilters(t *testing.T) {
	t.Parallel()
	provider, _ := NewFakeService(
		t.Context(),
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
	repos, err := ListPullRequests(t.Context(), provider, filters)
	require.NoError(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, "one", repos[0].Branch)
	assert.Equal(t, "two", repos[1].Branch)
}
