package kube

import (
	"testing"

	testingutils "github.com/argoproj/argo-cd/engine/pkg/utils/testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type testcase struct {
	name          string
	file          string
	outputVersion string
	fields        []checkField
}

type checkField struct {
	expected string
}

func Test_convertToVersionWithScheme(t *testing.T) {
	for _, tt := range []testcase{
		{
			name:          "apps deployment to extensions deployment",
			file:          "appsdeployment.yaml",
			outputVersion: "extensions/v1beta1",
			fields: []checkField{
				{
					expected: "apiVersion: extensions/v1beta1",
				},
			},
		},
		{
			name:          "extensions deployment to apps deployment",
			file:          "extensionsdeployment.yaml",
			outputVersion: "apps/v1beta2",
			fields: []checkField{
				{
					expected: "apiVersion: apps/v1beta2",
				},
			},
		},
		{
			name:          "v1 HPA to v2beta1 HPA",
			file:          "v1HPA.yaml",
			outputVersion: "autoscaling/v2beta1",
			fields: []checkField{
				{
					expected: "apiVersion: autoscaling/v2beta1",
				},
				{
					expected: "name: cpu",
				},
				{
					expected: "targetAverageUtilization: 50",
				},
			},
		},
		{
			name:          "v2beta1 HPA to v1 HPA",
			file:          "v2beta1HPA.yaml",
			outputVersion: "autoscaling/v1",
			fields: []checkField{
				{
					expected: "apiVersion: autoscaling/v1",
				},
				{
					expected: "targetCPUUtilizationPercentage: 50",
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			obj := testingutils.UnstructuredFromFile("testdata/" + tt.file)
			target, err := schema.ParseGroupVersion(tt.outputVersion)
			assert.NoError(t, err)
			out, err := convertToVersionWithScheme(obj, target.Group, target.Version)
			if assert.NoError(t, err) {
				assert.NotNil(t, out)
				assert.Equal(t, target.Group, out.GroupVersionKind().Group)
				assert.Equal(t, target.Version, out.GroupVersionKind().Version)
				bytes, err := yaml.Marshal(out)
				assert.NoError(t, err)
				for _, field := range tt.fields {
					assert.Contains(t, string(bytes), field.expected)
				}
			}
		})
	}
}
