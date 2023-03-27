package plugin

import (
	"context"
	"fmt"
	"net/http"

	internalhttp "github.com/argoproj/argo-cd/v2/applicationset/services/internal/http"
)

type PluginService struct {
	client *internalhttp.Client
	name   string
}

func NewPluginService(ctx context.Context, name string, baseURL string, token string, requestTimeout int) (*PluginService, error) {
	var clientOptionFns []internalhttp.ClientOptionFunc

	clientOptionFns = append(clientOptionFns, internalhttp.WithToken(token))

	if requestTimeout != 0 {
		clientOptionFns = append(clientOptionFns, internalhttp.WithTimeout(requestTimeout))
	}

	client, err := internalhttp.NewClient(baseURL, clientOptionFns...)
	if err != nil {
		return nil, fmt.Errorf("error creating plugin client: %v", err)
	}

	return &PluginService{
		client: client,
		name:   name,
	}, nil
}

func (p *PluginService) List(ctx context.Context, params map[string]string) ([]map[string]interface{}, *http.Response, error) {

	req, err := p.client.NewRequest("POST", "api/v1/getparams.execute", params, nil)

	if err != nil {
		return nil, nil, fmt.Errorf("NewRequest returned unexpected error: %v", err)
	}

	var data []map[string]interface{}

	resp, err := p.client.Do(ctx, req, &data)

	if err != nil {
		return nil, nil, fmt.Errorf("error get api '%s': %v", p.name, err)
	}

	return data, resp, err
}
