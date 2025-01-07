package plugin

import (
	"context"
	"fmt"
	"net/http"

	internalhttp "github.com/argoproj/argo-cd/v2/applicationset/services/internal/http"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

// ServiceRequest is the request object sent to the plugin service.
type ServiceRequest struct {
	// ApplicationSetName is the appSetName of the ApplicationSet for which we're requesting parameters. Useful for logging in
	// the plugin service.
	ApplicationSetName string `json:"applicationSetName"`
	// Input is the map of parameters set in the ApplicationSet spec for this generator.
	Input v1alpha1.PluginInput `json:"input"`
}

type Output struct {
	// Parameters is the list of parameter sets returned by the plugin.
	Parameters []map[string]interface{} `json:"parameters"`
}

// ServiceResponse is the response object returned by the plugin service.
type ServiceResponse struct {
	// Output is the map of outputs returned by the plugin.
	Output Output `json:"output"`
}

type Service struct {
	client     *internalhttp.Client
	appSetName string
}

func NewPluginService(ctx context.Context, appSetName string, baseURL string, token string, requestTimeout int) (*Service, error) {
	var clientOptionFns []internalhttp.ClientOptionFunc

	clientOptionFns = append(clientOptionFns, internalhttp.WithToken(token))

	if requestTimeout != 0 {
		clientOptionFns = append(clientOptionFns, internalhttp.WithTimeout(requestTimeout))
	}

	client, err := internalhttp.NewClient(baseURL, clientOptionFns...)
	if err != nil {
		return nil, fmt.Errorf("error creating plugin client: %w", err)
	}

	return &Service{
		client:     client,
		appSetName: appSetName,
	}, nil
}

func (p *Service) List(ctx context.Context, parameters v1alpha1.PluginParameters) (*ServiceResponse, error) {
	req, err := p.client.NewRequest(http.MethodPost, "api/v1/getparams.execute", ServiceRequest{ApplicationSetName: p.appSetName, Input: v1alpha1.PluginInput{Parameters: parameters}}, nil)
	if err != nil {
		return nil, fmt.Errorf("NewRequest returned unexpected error: %w", err)
	}

	var data ServiceResponse

	_, err = p.client.Do(ctx, req, &data)
	if err != nil {
		return nil, fmt.Errorf("error get api '%s': %w", p.appSetName, err)
	}

	return &data, err
}
