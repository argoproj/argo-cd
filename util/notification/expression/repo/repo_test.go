package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/util/notification/argocd/mocks"
	"github.com/argoproj/argo-cd/v3/util/notification/expression/shared"
)

func TestGetAppDetails_SourceIndexTypeHandling(t *testing.T) {
	app := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "test-app",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"project": "default",
				"sources": []interface{}{
					map[string]interface{}{
						"repoURL":        "https://github.com/foo/values.git",
						"targetRevision": "HEAD",
						"ref":            "values",
					},
					map[string]interface{}{
						"repoURL":        "oci://registry.example.com/helm",
						"chart":          "my-chart",
						"targetRevision": "1.0.0",
					},
				},
			},
		},
	}

	expectedDetail := shared.AppDetail{
		Type: "helm",
		Helm: &shared.CustomHelmAppSpec{},
	}

	mockSvc := mocks.NewService(t)
	mockSvc.EXPECT().
		GetAppDetails(mock.Anything, mock.Anything, mock.Anything).
		Return(&expectedDetail, nil).Maybe()

	exprs := NewExprs(mockSvc, app)
	getAppDetailsFn, ok := exprs["GetAppDetails"].(func(...interface{}) interface{})
	require.True(t, ok, "GetAppDetails should be registered")

	testCases := []struct {
		name        string
		args        []interface{}
		expectPanic bool
	}{
		{"no args uses index 0", []interface{}{}, false},
		{"int", []interface{}{1}, false},
		{"int64", []interface{}{int64(2)}, false},
		{"int32", []interface{}{int32(1)}, false},
		{"float64", []interface{}{float64(1)}, false},
		{"float32", []interface{}{float32(1)}, false},
		{"nil arg uses index 0", []interface{}{nil}, false},
		{"string panics", []interface{}{"1"}, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectPanic {
				assert.Panics(t, func() { getAppDetailsFn(tc.args...) })
				return
			}
			result := getAppDetailsFn(tc.args...)
			assert.NotNil(t, result)
			detail, ok := result.(shared.AppDetail)
			assert.True(t, ok)
			assert.Equal(t, expectedDetail.Type, detail.Type)
		})
	}
}
