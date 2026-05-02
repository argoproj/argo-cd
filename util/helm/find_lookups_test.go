package helm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectLookupUsage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		files   map[string]string
		wantHit []string
	}{
		{
			name: "no lookup",
			files: map[string]string{
				"Chart.yaml":             "name: c\nversion: 0.1.0\n",
				"templates/cm.yaml":      "{{ .Values.foo }}",
				"templates/_helpers.tpl": "{{- define \"x\" -}}y{{- end -}}",
			},
		},
		{
			name: "lookup in template",
			files: map[string]string{
				"Chart.yaml":         "name: c\nversion: 0.1.0\n",
				"templates/sec.yaml": "data: {{ (lookup \"v1\" \"Secret\" .Release.Namespace \"foo\").data.bar }}",
			},
			wantHit: []string{"templates/sec.yaml"},
		},
		{
			name: "lookup with leading whitespace trim",
			files: map[string]string{
				"templates/cm.yaml": "{{- $s := lookup \"v1\" \"Secret\" \"ns\" \"name\" -}}",
			},
			wantHit: []string{"templates/cm.yaml"},
		},
		{
			name: "lookup inside comment is ignored",
			files: map[string]string{
				"templates/cm.yaml": "{{/* example: lookup \"v1\" ... */}}\n{{ .Values.foo }}",
			},
		},
		{
			name: "lookup in subchart templates",
			files: map[string]string{
				"templates/cm.yaml":             "{{ .Values.foo }}",
				"charts/sub/templates/sec.yaml": "{{ lookup \"v1\" \"Secret\" \"ns\" \"x\" }}",
			},
			wantHit: []string{"charts/sub/templates/sec.yaml"},
		},
		{
			name: "non-template file outside templates dir ignored",
			files: map[string]string{
				"values.yaml":      "# lookup mentioned only in values\nfoo: bar",
				"README.md":        "Uses lookup function in docs",
				"templates/x.yaml": "{{ .Values.foo }}",
			},
		},
		{
			name: "word boundary - lookups should not match",
			files: map[string]string{
				"templates/cm.yaml": "{{ .Values.lookups.enabled }}",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			for rel, content := range tc.files {
				full := filepath.Join(dir, rel)
				require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
				require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
			}

			got, err := DetectLookupUsage(dir)
			require.NoError(t, err)
			assert.ElementsMatch(t, tc.wantHit, got)
		})
	}
}

func TestDetectLookupUsage_MissingDir(t *testing.T) {
	t.Parallel()
	// Detection is best-effort: a missing directory must not cause an error,
	// so callers can safely invoke it without pre-checking the path.
	got, err := DetectLookupUsage(filepath.Join(t.TempDir(), "does-not-exist"))
	require.NoError(t, err)
	assert.Empty(t, got)
}
