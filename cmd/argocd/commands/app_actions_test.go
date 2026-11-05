package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseParamArgs(t *testing.T) {
	tests := []struct {
		name        string
		params      []string
		wantNames   []string
		wantValues  []string
		wantErr     bool
	}{
		{
			name:       "no params returns empty slice",
			params:     []string{},
			wantNames:  []string{},
			wantValues: []string{},
		},
		{
			name:       "single valid param",
			params:     []string{"replica-count=3"},
			wantNames:  []string{"replica-count"},
			wantValues: []string{"3"},
		},
		{
			name:       "multiple valid params",
			params:     []string{"key1=value1", "key2=value2"},
			wantNames:  []string{"key1", "key2"},
			wantValues: []string{"value1", "value2"},
		},
		{
			name:       "value containing equals sign is preserved",
			params:     []string{"url=https://example.com?foo=bar"},
			wantNames:  []string{"url"},
			wantValues: []string{"https://example.com?foo=bar"},
		},
		{
			name:    "missing equals sign returns error",
			params:  []string{"invalidparam"},
			wantErr: true,
		},
		{
			name:    "empty string returns error",
			params:  []string{""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseParamArgs(tt.params)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, got, len(tt.wantNames))
			for i, p := range got {
				assert.Equal(t, tt.wantNames[i], *p.Name)
				assert.Equal(t, tt.wantValues[i], *p.Value)
			}
		})
	}
}
