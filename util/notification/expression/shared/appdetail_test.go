package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
)

func TestGetParameterValueByName(t *testing.T) {
	helmAppSpec := CustomHelmAppSpec{
		HelmAppSpec: apiclient.HelmAppSpec{
			Parameters: []*v1alpha1.HelmParameter{
				{
					Name:  "param1",
					Value: "value1",
				},
			},
		},
		HelmParameterOverrides: []v1alpha1.HelmParameter{
			{
				Name:  "param1",
				Value: "value-override",
			},
		},
	}

	value := helmAppSpec.GetParameterValueByName("param1")
	assert.Equal(t, "value-override", value)
}
