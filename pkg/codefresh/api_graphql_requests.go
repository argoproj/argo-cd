package codefresh

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CodefreshGraphQLRequests struct {
	client CodefreshClientInterface
}

type CodefreshGraphQLInterface interface {
	GetPromotionTemplate(app *metav1.ObjectMeta) (*PromotionTemplate, error)
}

// GetPromotionTemplate method to get application configuration
func (r *CodefreshGraphQLRequests) GetPromotionTemplate(app *metav1.ObjectMeta) (*PromotionTemplate, error) {
	type ResponseData struct {
		PromotionTemplateByRuntime PromotionTemplate `json:"promotionTemplateByRuntime"`
	}

	query := GraphQLQuery{
		Query: `
		query ($applicationMetadata: Object!) {
			promotionTemplateByRuntime(applicationMetadata: $applicationMetadata) {
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

	return &responseData.PromotionTemplateByRuntime, nil
}

func NewCodefreshGraphQLRequests(client CodefreshClientInterface) CodefreshGraphQLInterface {
	return &CodefreshGraphQLRequests{client}
}
