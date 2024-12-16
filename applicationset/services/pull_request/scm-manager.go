package pull_request

import (
	"context"
	"net/http"
	"os"
	"strconv"

	"github.com/argoproj/argo-cd/v2/applicationset/utils"

	scmm "github.com/scm-manager/goscm"
)

type ScmManagerService struct {
	client    *scmm.Client
	namespace string
	name      string
}

var _ PullRequestService = (*ScmManagerService)(nil)

func NewScmManagerService(ctx context.Context, token, url, namespace, name string, insecure bool, scmRootCAPath string, caCerts []byte) (PullRequestService, error) {
	if token == "" {
		token = os.Getenv("SCMM_TOKEN")
	}

	httpClient := &http.Client{}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = utils.GetTlsConfig(scmRootCAPath, insecure, caCerts)
	httpClient.Transport = tr

	client, err := scmm.NewClient(url, token)
	if err != nil {
		return nil, err
	}

	client.SetHttpClient(httpClient)
	return &ScmManagerService{
		client:    client,
		namespace: namespace,
		name:      name,
	}, nil
}

func (g *ScmManagerService) List(ctx context.Context) ([]*PullRequest, error) {
	prs, err := g.client.ListPullRequests(g.namespace, g.name, g.client.NewPullRequestListFilter())
	if err != nil {
		return nil, err
	}
	list := []*PullRequest{}
	for _, pr := range prs.Embedded.PullRequests {
		changeset, err := g.client.GetHeadChangesetForBranch(g.namespace, g.name, pr.Source)
		if err != nil {
			return nil, err
		}
		prId, err := strconv.Atoi(pr.Id)
		if err != nil {
			return nil, err
		}
		list = append(list, &PullRequest{
			Number:  prId,
			Branch:  pr.Source,
			HeadSHA: changeset.Id,
			Labels:  make([]string, 0),
		})
	}
	return list, nil
}
