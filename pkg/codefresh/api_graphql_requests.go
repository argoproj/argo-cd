package codefresh

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CodefreshGraphQLRequests struct {
	client CodefreshClientInterface
}

type CodefreshGraphQLInterface interface {
	GetApplicationConfiguration(app *metav1.ObjectMeta) (*ApplicationConfiguration, error)
}

// GetApplicationConfiguration method to get application configuration
func (r *CodefreshGraphQLRequests) GetApplicationConfiguration(app *metav1.ObjectMeta) (*ApplicationConfiguration, error) {
	type ResponseData struct {
		ApplicationConfigurationByRuntime ApplicationConfiguration `json:"applicationConfigurationByRuntime"`
	}

	query := GraphQLQuery{
		Query: `
		query ($applicationMetadata: Object!) {
		  applicationConfigurationByRuntime(applicationMetadata: $applicationMetadata) {
			versionSource {
			  file
			  jsonPath
			}
		  }
		}
		`,
		Variables: map[string]interface{}{
			"applicationMetadata": app,
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
