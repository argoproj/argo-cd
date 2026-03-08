package sourcecraft

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// ListRepoBranchesOptions specifies options for listing repository branches.
type ListRepoBranchesOptions struct {
	ListOptions
}

// ListRepoBranchesResponse contains the result of listing repository branches.
type ListRepoBranchesResponse struct {
	Branches      []*Branch `json:"branches"`
	NextPageToken string    `json:"next_page_token"`
}

// ListRepoBranches lists all branches in the specified repository.
// It returns a paginated list of branches for the given organization and repository slugs.
func (c *Client) ListRepoBranches(ctx context.Context, orgSlug, repoSlug string, opt ListRepoBranchesOptions) (*ListRepoBranchesResponse, *Response, error) {
	if err := escapeValidatePathSegments(&orgSlug, &repoSlug); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	branchesResp := ListRepoBranchesResponse{}
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/branches?%s", orgSlug, repoSlug, opt.getURLQuery().Encode()), nil, nil, &branchesResp)
	return &branchesResp, resp, err
}

// GetRepoBranch retrieves a specific branch by name from the repository.
// It filters the branch list by name and returns the first matching branch.
func (c *Client) GetRepoBranch(ctx context.Context, orgSlug, repoSlug, branch string) (*Branch, *Response, error) {
	branchesResp, resp, err := c.ListRepoBranches(ctx, orgSlug, repoSlug, ListRepoBranchesOptions{ListOptions{Filter: branch}})
	if branchesResp == nil || len(branchesResp.Branches) == 0 {
		return nil, resp, err
	}
	return branchesResp.Branches[0], resp, err
}

// ListLabelsOptions specifies options for listing repository labels.
type ListLabelsOptions struct {
	ListOptions
}

// ListRepoLabelsResponse contains the result of listing repository labels.
type ListRepoLabelsResponse struct {
	Labels        []*Label `json:"items"`
	NextPageToken string   `json:"next_page_token"`
}

// ListRepoLabels lists all labels in the specified repository.
// It returns a paginated list of labels for the given organization and repository slugs.
func (c *Client) ListRepoLabels(ctx context.Context, orgSlug, repoSlug string, opt ListLabelsOptions) (*ListRepoLabelsResponse, *Response, error) {
	if err := escapeValidatePathSegments(&orgSlug, &repoSlug); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	labelsResp := ListRepoLabelsResponse{}
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/labels?%s", orgSlug, repoSlug, opt.getURLQuery().Encode()), nil, nil, &labelsResp)
	return &labelsResp, resp, err
}

// ListRepoFileTreeOptions specifies options for listing repository file trees.
type ListRepoFileTreeOptions struct {
	ListOptions

	// Recursive determines whether to recursively list subdirectories.
	Recursive *bool
}

func (o ListRepoFileTreeOptions) getURLQuery() url.Values {
	query := o.ListOptions.getURLQuery()
	if o.Recursive != nil {
		query.Add("recursive", strconv.FormatBool(*o.Recursive))
	}
	return query
}

// ListRepoFileTreeResponse contains the result of listing repository file trees.
type ListRepoFileTreeResponse struct {
	Trees         []*TreeEntry `json:"trees"`
	NextPageToken string       `json:"next_page_token"`
}

// ListRepoFileTree lists files and directories in the repository at a specific revision and path.
// The revision parameter specifies the branch, tag, or commit SHA to query.
// The path parameter specifies the directory path within the repository.
func (c *Client) ListRepoFileTree(ctx context.Context, orgSlug, repoSlug string, revision string, path string, opt ListRepoFileTreeOptions) (*ListRepoFileTreeResponse, *Response, error) {
	if err := escapeValidatePathSegments(&orgSlug, &repoSlug); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	query := opt.getURLQuery()
	query.Add("revision", revision)
	query.Add("path", path)

	tressResp := ListRepoFileTreeResponse{}
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/trees?%s", orgSlug, repoSlug, query.Encode()), nil, nil, &tressResp)
	return &tressResp, resp, err
}

// ListRepoPullRequestsOptions specifies options for listing repository pull requests.
type ListRepoPullRequestsOptions struct {
	ListOptions
}

// ListRepoPullRequestsResponse contains the result of listing repository pull requests.
type ListRepoPullRequestsResponse struct {
	PullRequests  []*PullRequest `json:"pull_requests"`
	NextPageToken string         `json:"next_page_token"`
}

// ListRepoPullRequests lists all pull requests in the specified repository.
// It returns a paginated list of pull requests for the given organization and repository slugs.
func (c *Client) ListRepoPullRequests(ctx context.Context, orgSlug, repoSlug string, opt ListRepoPullRequestsOptions) (*ListRepoPullRequestsResponse, *Response, error) {
	if err := escapeValidatePathSegments(&orgSlug, &repoSlug); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	prsResp := ListRepoPullRequestsResponse{}
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/pulls?%s", orgSlug, repoSlug, opt.getURLQuery().Encode()), nil, nil, &prsResp)
	return &prsResp, resp, err
}
