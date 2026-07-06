package kube

import (
	"testing"

	testingutils "github.com/argoproj/argo-cd/gitops-engine/pkg/utils/testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
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
	} {
		t.Run(tt.name, func(t *testing.T) {
			obj := testingutils.UnstructuredFromFile("testdata/" + tt.file)
			target, err := schema.ParseGroupVersion(tt.outputVersion)
			require.NoError(t, err)
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
