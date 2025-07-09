package v1alpha1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTruncateByDepth_OK(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxDepth int64
		want     string
	}{
		{
			name:     "TruncateBeyondDepth2",
			input:    `{"level1":{"level2":{"level3":{"tooDeep":true}},"simple":"keep me"},"root":"keep me too"}`,
			maxDepth: 3,
			want:     `{"level1":{"level2":{"level3":"...(truncated)"},"simple":"keep me"},"root":"keep me too"}`,
		},
		{
			name:     "HandleArrays",
			input:    `{"data":[{"deep":{"value":1}},{"deep":{"value":2}}]}`,
			maxDepth: 3,
			want:     `{"data":[{"deep":"...(truncated)"},{"deep":"...(truncated)"}]}`,
		},
		{
			name:     "ScalarsStayNestedTruncated",
			input:    `{"string":"hello","number":123,"boolean":true,"null":null,"nested":{"deep":{"deeper":"gone"}}}`,
			maxDepth: 2,
			want:     `{"string":"hello","number":123,"boolean":true,"null":null,"nested":{"deep":"...(truncated)"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := TruncateByDepth([]byte(tt.input), tt.maxDepth)
			require.NoError(t, err)

			var wantObj, gotObj interface{}
			require.NoError(t, json.Unmarshal([]byte(tt.want), &wantObj))
			require.NoError(t, json.Unmarshal(result, &gotObj))

			require.Equal(t, wantObj, gotObj)
		})
	}
}

func TestTruncateByDepth_Error(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxDepth int64
	}{
		{
			name:     "InvalidJSONInput",
			input:    `{"bad": [}`,
			maxDepth: 2,
		},
		{
			name:     "MissingComma",
			input:    `{"a":1 "b":2}`,
			maxDepth: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := TruncateByDepth([]byte(tt.input), tt.maxDepth)
			require.Error(t, err)
			require.Contains(t, err.Error(), "unmarshal")
		})
	}
}
