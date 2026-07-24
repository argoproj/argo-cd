package files

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pattern  string
		path     string
		expected bool
		wantErr  bool
	}{
		{
			name:     "simple filename match",
			pattern:  "*.yaml",
			path:     "kustomization.yaml",
			expected: true,
		},
		{
			name:     "simple filename no match",
			pattern:  "*.yaml",
			path:     "_helpers.tpl",
			expected: false,
		},
		{
			name:     "charts/** matches nested file",
			pattern:  "charts/**",
			path:     "charts/podinfo/templates/_helpers.tpl",
			expected: true,
		},
		{
			name:     "charts/** matches deeply nested file",
			pattern:  "charts/**",
			path:     "charts/podinfo-6.7.0/podinfo/templates/_helpers.tpl",
			expected: true,
		},
		{
			name:     "charts/** matches direct child",
			pattern:  "charts/**",
			path:     "charts/Chart.yaml",
			expected: true,
		},
		{
			name:     "charts/** does not match file outside charts",
			pattern:  "charts/**",
			path:     "kustomization.yaml",
			expected: false,
		},
		{
			name:     "charts/** does not match sibling directory",
			pattern:  "charts/**",
			path:     "other/file.yaml",
			expected: false,
		},
		{
			name:     "exact path match",
			pattern:  "charts/podinfo/values.yaml",
			path:     "charts/podinfo/values.yaml",
			expected: true,
		},
		{
			name:     "exact path no match",
			pattern:  "charts/podinfo/values.yaml",
			path:     "charts/other/values.yaml",
			expected: false,
		},
		{
			name:     "wildcard segment",
			pattern:  "charts/*/values.yaml",
			path:     "charts/podinfo/values.yaml",
			expected: true,
		},
		{
			name:     "wildcard segment does not span directories",
			pattern:  "charts/*/values.yaml",
			path:     "charts/podinfo/nested/values.yaml",
			expected: false,
		},
		{
			name:     "** matches zero segments at root",
			pattern:  "charts/**",
			path:     "charts/file.tpl",
			expected: true, // ** matches a single segment too
		},
		{
			name:    "invalid pattern returns error",
			pattern: "charts/[invalid",
			path:    "charts/foo.yaml",
			wantErr: true,
		},
		{
			name:     "OS-native separator in pattern is normalised before matching",
			pattern:  filepath.FromSlash("charts/**"),
			path:     "charts/podinfo/templates/_helpers.tpl",
			expected: true,
		},
		{
			name:     "OS-native separator in path is normalised before matching",
			pattern:  "charts/**",
			path:     filepath.FromSlash("charts/podinfo/templates/_helpers.tpl"),
			expected: true,
		},
		{
			name:     "OS-native separator in both pattern and path",
			pattern:  filepath.FromSlash("charts/**"),
			path:     filepath.FromSlash("charts/podinfo/values.yaml"),
			expected: true,
		},
		{
			name:     "OS-native separator pattern does not match outside dir",
			pattern:  filepath.FromSlash("charts/**"),
			path:     filepath.FromSlash("other/values.yaml"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := matchPath(tt.pattern, tt.path)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestMatchesPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		pattern      string
		base         string
		relativePath string
		expected     bool
		wantErr      bool
	}{
		{
			name:         "simple glob matches base filename",
			pattern:      "*.yaml",
			base:         "kustomization.yaml",
			relativePath: "kustomization.yaml",
			expected:     true,
		},
		{
			name:         "simple glob does not match base filename",
			pattern:      "*.yaml",
			base:         "_helpers.tpl",
			relativePath: "_helpers.tpl",
			expected:     false,
		},
		{
			name:         "path pattern matches relative path",
			pattern:      "charts/**",
			base:         "_helpers.tpl",
			relativePath: "charts/podinfo/templates/_helpers.tpl",
			expected:     true,
		},
		{
			name:         "path pattern does not match relative path outside dir",
			pattern:      "charts/**",
			base:         "values.yaml",
			relativePath: "other/values.yaml",
			expected:     false,
		},
		{
			name:         "exact path pattern matches",
			pattern:      "charts/podinfo/values.yaml",
			base:         "values.yaml",
			relativePath: "charts/podinfo/values.yaml",
			expected:     true,
		},
		{
			name:         "exact path pattern does not match different path",
			pattern:      "charts/podinfo/values.yaml",
			base:         "values.yaml",
			relativePath: "charts/other/values.yaml",
			expected:     false,
		},
		{
			name:         "invalid simple glob returns error",
			pattern:      "[invalid",
			base:         "foo.yaml",
			relativePath: "foo.yaml",
			wantErr:      true,
		},
		{
			name:         "invalid path pattern returns error",
			pattern:      "charts/[invalid",
			base:         "foo.yaml",
			relativePath: "charts/foo.yaml",
			wantErr:      true,
		},
		{
			name:         "slug-less glob matches nested file via basename",
			pattern:      "*.yaml",
			base:         "values.yaml",
			relativePath: "charts/podinfo/values.yaml",
			expected:     true,
		},
		{
			name:         "OS-native separator path pattern matches relative path",
			pattern:      filepath.FromSlash("charts/**"),
			base:         "_helpers.tpl",
			relativePath: "charts/podinfo/templates/_helpers.tpl",
			expected:     true,
		},
		{
			name:         "OS-native separator exact path pattern matches",
			pattern:      filepath.FromSlash("charts/podinfo/values.yaml"),
			base:         "values.yaml",
			relativePath: "charts/podinfo/values.yaml",
			expected:     true,
		},
		{
			name:         "OS-native separator path pattern does not match outside dir",
			pattern:      filepath.FromSlash("charts/**"),
			base:         "values.yaml",
			relativePath: "other/values.yaml",
			expected:     false,
		},
		{
			name:         "OS-native separator multi-segment pattern matches nested path",
			pattern:      filepath.FromSlash("applicationset/latest/**"),
			base:         "kustomization.yaml",
			relativePath: "applicationset/latest/kustomization.yaml",
			expected:     true,
		},
		{
			name:         "OS-native separator multi-segment pattern does not match sibling dir",
			pattern:      filepath.FromSlash("applicationset/latest/**"),
			base:         "kustomization.yaml",
			relativePath: "applicationset/stable/kustomization.yaml",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := matchesPattern(tt.pattern, tt.base, tt.relativePath)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestMatchesExclusionPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		pattern      string
		relativePath string
		expected     bool
		wantErr      bool
	}{
		{
			name:         "slug-less glob matches root-level file",
			pattern:      "*.yaml",
			relativePath: "values.yaml",
			expected:     true,
		},
		{
			name:         "slug-less glob does not match nested file",
			pattern:      "*.yaml",
			relativePath: "charts/podinfo/values.yaml",
			expected:     false,
		},
		{
			name:         "slug-less exact name matches root-level file",
			pattern:      "README.md",
			relativePath: "README.md",
			expected:     true,
		},
		{
			name:         "slug-less exact name does not match nested file with same basename",
			pattern:      "README.md",
			relativePath: "docs/README.md",
			expected:     false,
		},
		{
			name:         "slash pattern with ** matches nested file",
			pattern:      "charts/**",
			relativePath: "charts/podinfo/templates/_helpers.tpl",
			expected:     true,
		},
		{
			name:         "slash pattern with ** does not match outside dir",
			pattern:      "charts/**",
			relativePath: "other/values.yaml",
			expected:     false,
		},
		{
			name:         "exact slash pattern matches",
			pattern:      "charts/podinfo/values.yaml",
			relativePath: "charts/podinfo/values.yaml",
			expected:     true,
		},
		{
			name:         "exact slash pattern does not match different path",
			pattern:      "charts/podinfo/values.yaml",
			relativePath: "charts/other/values.yaml",
			expected:     false,
		},
		{
			name:         "doublestar-only pattern matches nested yaml",
			pattern:      "**/*.yaml",
			relativePath: "charts/podinfo/values.yaml",
			expected:     true,
		},
		{
			name:         "doublestar-only pattern does not match non-yaml",
			pattern:      "**/*.yaml",
			relativePath: "charts/podinfo/_helpers.tpl",
			expected:     false,
		},
		{
			name:         "invalid slug-less pattern returns error",
			pattern:      "[invalid",
			relativePath: "foo.yaml",
			wantErr:      true,
		},
		{
			name:         "invalid slash pattern returns error",
			pattern:      "charts/[invalid",
			relativePath: "charts/foo.yaml",
			wantErr:      true,
		},
		{
			name:         "OS-native separator slug-less pattern matches root-level file",
			pattern:      filepath.FromSlash("*.yaml"),
			relativePath: "values.yaml",
			expected:     true,
		},
		{
			name:         "OS-native separator slug-less pattern does not match nested file",
			pattern:      filepath.FromSlash("*.yaml"),
			relativePath: "charts/podinfo/values.yaml",
			expected:     false,
		},
		{
			name:         "OS-native separator slash pattern matches nested file",
			pattern:      filepath.FromSlash("charts/**"),
			relativePath: "charts/podinfo/values.yaml",
			expected:     true,
		},
		{
			name:         "OS-native separator multi-segment pattern matches",
			pattern:      filepath.FromSlash("applicationset/latest/**"),
			relativePath: "applicationset/latest/kustomization.yaml",
			expected:     true,
		},
		{
			name:         "OS-native separator multi-segment pattern does not match sibling",
			pattern:      filepath.FromSlash("applicationset/latest/**"),
			relativePath: "applicationset/stable/kustomization.yaml",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := matchesExclusionPattern(tt.pattern, tt.relativePath)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func BenchmarkMatchPath(b *testing.B) {
	benchmarks := []struct {
		name    string
		pattern string
		path    string
	}{
		{
			name:    "simple wildcard match",
			pattern: "*.yaml",
			path:    "kustomization.yaml",
		},
		{
			name:    "doublestar match nested file",
			pattern: "charts/**",
			path:    "charts/podinfo/templates/_helpers.tpl",
		},
		{
			name:    "doublestar match deeply nested file",
			pattern: "charts/**",
			path:    "charts/podinfo-6.7.0/podinfo/templates/_helpers.tpl",
		},
		{
			name:    "exact path match",
			pattern: "charts/podinfo/values.yaml",
			path:    "charts/podinfo/values.yaml",
		},
		{
			name:    "wildcard segment match",
			pattern: "charts/*/values.yaml",
			path:    "charts/podinfo/values.yaml",
		},
		{
			name:    "no match",
			pattern: "charts/**",
			path:    "other/file.yaml",
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for b.Loop() {
				_, _ = matchPath(bm.pattern, bm.path)
			}
		})
	}
}
