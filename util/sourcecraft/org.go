package sourcecraft

import (
	"context"
	"fmt"
)

// ListOrgReposOptions specifies optional parameters for listing organization repositories.
type ListOrgReposOptions struct {
	ListOptions
}

// ListOrgReposResponse represents the response from listing organization repositories.
type ListOrgReposResponse struct {
	Repositories  []*Repository `json:"repositories"`
	NextPageToken string        `json:"next_page_token"`
}

// ListOrgRepos lists all repositories for the specified organization.
// It returns a paginated list of repositories along with a next page token for pagination.
//
// Parameters:
//   - ctx: The context for the request
//   - orgSlug: The organization slug identifier
//   - opt: Optional parameters for pagination and filtering
//
// Returns the list of repositories, HTTP response metadata, and any error encountered.
func (c *Client) ListOrgRepos(ctx context.Context, orgSlug string, opt ListOrgReposOptions) (*ListOrgReposResponse, *Response, error) {
	if err := escapeValidatePathSegments(&orgSlug); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	reposResp := ListOrgReposResponse{}
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/orgs/%s/repos?%s", orgSlug, opt.getURLQuery().Encode()), nil, nil, &reposResp)
	return &reposResp, resp, err
}
