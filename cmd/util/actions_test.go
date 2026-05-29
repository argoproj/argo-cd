package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
)

func strPtr(s string) *string { return &s }

func TestParseActionParameters(t *testing.T) {
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
			name:        "empty key",
			params:      []string{"=value"},
			expectError: true,
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
		{
			name:   "duplicate keys",
			params: []string{"replicas=2", "replicas=3"},
			expectedParams: []*applicationpkg.ResourceActionParameters{
				{Name: strPtr("replicas"), Value: strPtr("2")},
				{Name: strPtr("replicas"), Value: strPtr("3")},
			},
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

func TestDuplicateActionParameterNames(t *testing.T) {
	testCases := []struct {
		name          string
		params        []*applicationpkg.ResourceActionParameters
		expectedDupes []string
	}{
		{
			name:          "empty",
			params:        []*applicationpkg.ResourceActionParameters{},
			expectedDupes: nil,
		},
		{
			name: "no duplicates",
			params: []*applicationpkg.ResourceActionParameters{
				{Name: strPtr("replicas"), Value: strPtr("2")},
			},
			expectedDupes: nil,
		},
		{
			name: "single duplicate",
			params: []*applicationpkg.ResourceActionParameters{
				{Name: strPtr("replicas"), Value: strPtr("2")},
				{Name: strPtr("replicas"), Value: strPtr("3")},
			},
			expectedDupes: []string{"replicas"},
		},
		{
			name: "multiple duplicates",
			params: []*applicationpkg.ResourceActionParameters{
				{Name: strPtr("replicas"), Value: strPtr("2")},
				{Name: strPtr("replicas"), Value: strPtr("3")},
				{Name: strPtr("replicas"), Value: strPtr("4")},
			},
			expectedDupes: []string{"replicas"},
		},
		{
			name: "multiple distinct duplicates",
			params: []*applicationpkg.ResourceActionParameters{
				{Name: strPtr("replicas"), Value: strPtr("2")},
				{Name: strPtr("replicas"), Value: strPtr("3")},
				{Name: strPtr("replicas"), Value: strPtr("4")},
				{Name: strPtr("image"), Value: strPtr("foo")},
				{Name: strPtr("image"), Value: strPtr("bar")},
				{Name: strPtr("image"), Value: strPtr("baz")},
			},
			expectedDupes: []string{"replicas", "image"},
		},
		{
			name: "nil element in slice",
			params: []*applicationpkg.ResourceActionParameters{
				nil,
			},
			expectedDupes: nil,
		},
		{
			name: "nil name field",
			params: []*applicationpkg.ResourceActionParameters{
				{Name: nil, Value: nil},
			},
			expectedDupes: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := DuplicateActionParameterNames(tc.params)
			assert.ElementsMatch(t, tc.expectedDupes, result)
		})
	}
}
