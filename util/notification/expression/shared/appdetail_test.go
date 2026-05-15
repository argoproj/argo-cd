package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
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

func TestHelmAppSpecName(t *testing.T) {
	helmAppSpec := CustomHelmAppSpec{
		HelmAppSpec: apiclient.HelmAppSpec{
			Name: "test-helm-app",
		},
	}

	assert.Equal(t, "test-helm-app", helmAppSpec.Name)
}

func TestGetParameterValueByNameFromHelmAppSpec(t *testing.T) {
	helmAppSpec := CustomHelmAppSpec{
		HelmAppSpec: apiclient.HelmAppSpec{
			Parameters: []*v1alpha1.HelmParameter{
				{
					Name:  "simple",
					Value: "easy",
				},
				{
					Name:  "another",
					Value: "value",
				},
			},
		},
	}

	value := helmAppSpec.GetParameterValueByName("simple")
	assert.Equal(t, "easy", value)

	value = helmAppSpec.GetParameterValueByName("another")
	assert.Equal(t, "value", value)

	value = helmAppSpec.GetParameterValueByName("non-existent")
	assert.Empty(t, value)
}

func TestGetFileParameterPathByNameFromHelmAppSpec(t *testing.T) {
	helmAppSpec := CustomHelmAppSpec{
		HelmAppSpec: apiclient.HelmAppSpec{
			FileParameters: []*v1alpha1.HelmFileParameter{
				{
					Name: "config",
					Path: "/path/to/config",
				},
				{
					Name: "secret",
					Path: "/path/to/secret",
				},
			},
		},
	}

	path := helmAppSpec.GetFileParameterPathByName("config")
	assert.Equal(t, "/path/to/config", path)

	path = helmAppSpec.GetFileParameterPathByName("secret")
	assert.Equal(t, "/path/to/secret", path)

	path = helmAppSpec.GetFileParameterPathByName("non-existent")
	assert.Empty(t, path)
}
