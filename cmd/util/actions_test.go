package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
)

func TestParseActionParameters(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	testCases := []struct {
		name           string
		params         []string
		expectedParams []*applicationpkg.ResourceActionParameters
		expectError    bool
	}{
		{
			name:           "empty",
			params:         []string{},
			expectedParams: []*applicationpkg.ResourceActionParameters{},
		},
		{
			name:   "single parameter",
			params: []string{"replicas=2"},
			expectedParams: []*applicationpkg.ResourceActionParameters{
				{Name: strPtr("replicas"), Value: strPtr("2")},
			},
		},
		{
			name:   "multiple parameters",
			params: []string{"replicas=2", "image=foo"},
			expectedParams: []*applicationpkg.ResourceActionParameters{
				{Name: strPtr("replicas"), Value: strPtr("2")},
				{Name: strPtr("image"), Value: strPtr("foo")},
			},
		},
		{
			name:   "value containing equals sign",
			params: []string{"url=http://example.com?foo=bar"},
			expectedParams: []*applicationpkg.ResourceActionParameters{
				{Name: strPtr("url"), Value: strPtr("http://example.com?foo=bar")},
			},
		},
		{
			name:   "empty value",
			params: []string{"key="},
			expectedParams: []*applicationpkg.ResourceActionParameters{
				{Name: strPtr("key"), Value: strPtr("")},
			},
		},
		{
			name:        "missing equals sign",
			params:      []string{"replicas"},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseActionParameters(tc.params)
			if tc.expectError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, result, len(tc.expectedParams))
			for i, p := range result {
				assert.Equal(t, *tc.expectedParams[i].Name, *p.Name)
				assert.Equal(t, *tc.expectedParams[i].Value, *p.Value)
			}
		})
	}
}
