package khulnasoft

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KhulnasoftGraphQLRequests struct {
	client KhulnasoftClientInterface
}

type KhulnasoftGraphQLInterface interface {
	GetPromotionTemplate(app *metav1.ObjectMeta) (*PromotionTemplate, error)
}

// GetPromotionTemplate method to get application configuration
func (r *KhulnasoftGraphQLRequests) GetPromotionTemplate(app *metav1.ObjectMeta) (*PromotionTemplate, error) {
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

func NewKhulnasoftGraphQLRequests(client KhulnasoftClientInterface) KhulnasoftGraphQLInterface {
	return &KhulnasoftGraphQLRequests{client}
}
