package hash

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFNVa(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  uint32
	}{
		{
			name:  "empty string",
			input: "",
			want:  2166136261,
		},
		{
			name:  "ASCII string",
			input: "argo-cd",
			want:  88954688,
		},
		{
			name:  "case-sensitive input",
			input: "Argo CD",
			want:  653228367,
		},
		{
			name:  "Unicode input",
			input: "🚀",
			want:  2141686490,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FNVa(tt.input))
		})
	}
}
