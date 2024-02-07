package codefresh

import (
	"encoding/json"
)

type CodefreshGraphQLRequests struct {
	client CodefreshClientInterface
}

type CodefreshGraphQLInterface interface {
	GetApplicationConfiguration(app *ApplicationIdentity) (*ApplicationConfiguration, error)
}

// GetApplicationConfiguration method to get application configuration
func (r *CodefreshGraphQLRequests) GetApplicationConfiguration(app *ApplicationIdentity) (*ApplicationConfiguration, error) {
	type ResponseData struct {
		ApplicationConfigurationByRuntime ApplicationConfiguration `json:"applicationConfigurationByRuntime"`
	}

	query := GraphQLQuery{
		Query: `
		query ($cluster: String!, $namespace: String!, $name: String!) {
		  applicationConfigurationByRuntime(cluster: $cluster, namespace: $namespace, name: $name) {
			versionSource {
			  file
			  jsonPath
			}
		  }
		}
		`,
		Variables: map[string]interface{}{
			"cluster":   app.Cluster,
			"namespace": app.Namespace,
			"name":      app.Name,
		},
	}

	responseJSON, err := r.client.SendGraphQL(query)
	if err != nil {
		return nil, err
	}

	var responseData ResponseData
	if err := json.Unmarshal(*responseJSON, &responseData); err != nil {
		return nil, err
	}

	return &responseData.ApplicationConfigurationByRuntime, nil
}

func NewCodefreshGraphQLRequests(client CodefreshClientInterface) CodefreshGraphQLInterface {
	return &CodefreshGraphQLRequests{client}
}
