package repo

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/expr-lang/expr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	nestedGitLabURL = "https://gitlab.example.com/group/sub/subsub/project.git"
	gitHubBrowseURL = "https://github.com/argoproj/argo-cd/tree/master/manifests"
)

// TestFullNameByRepoURL checks the default two-segment result, the optional
// segment count, and that HTTPS, SSH and scp-style URLs resolve to the same path.
func TestFullNameByRepoURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		segments []int
		expected string
	}{
		{"github https", "https://github.com/argoproj/argo-cd.git", nil, "argoproj/argo-cd"},
		{"github scp", "git@github.com:argoproj/argo-cd.git", nil, "argoproj/argo-cd"},
		{"github browse link keeps two segments", gitHubBrowseURL, nil, "argoproj/argo-cd"},
		{"bitbucket cloud", "https://bitbucket.org/workspace/repo.git", nil, "workspace/repo"},
		{"gitlab top-level project", "https://gitlab.com/group/project.git", nil, "group/project"},
		{"gitlab nested default", nestedGitLabURL, nil, "group/sub"},
		{"gitlab nested 3 segments", nestedGitLabURL, []int{3}, "group/sub/subsub"},
		{"gitlab nested all segments", nestedGitLabURL, []int{0}, "group/sub/subsub/project"},
		{"gitlab nested negative means all", nestedGitLabURL, []int{-1}, "group/sub/subsub/project"},
		{"gitlab nested scp all segments", "git@gitlab.example.com:group/sub/project.git", []int{0}, "group/sub/project"},
		{"gitlab nested ssh with port all segments", "ssh://git@gitlab.example.com:2222/group/sub/project.git", []int{0}, "group/sub/project"},
		{"more segments than the path has", "https://gitlab.com/group/project.git", []int{5}, "group/project"},
		{"short path keeps the legacy result", "https://example.com/repo.git", nil, "/repo"},
		{"short path with segments", "https://example.com/repo.git", []int{2}, "repo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, FullNameByRepoURL(tt.url, tt.segments...))
		})
	}
}

// TestFullNameByRepoURLCallPaths checks that the optional argument works through
// the two engines that call the function: text/template for notification
// templates and expr for trigger conditions.
func TestFullNameByRepoURLCallPaths(t *testing.T) {
	tests := []struct {
		name     string
		template string
		expr     string
		url      string
		expected string
	}{
		{"nested default", `{{call .repo.FullNameByRepoURL .url}}`, `repo.FullNameByRepoURL(url)`, nestedGitLabURL, "group/sub"},
		{"nested 3 segments", `{{call .repo.FullNameByRepoURL .url 3}}`, `repo.FullNameByRepoURL(url, 3)`, nestedGitLabURL, "group/sub/subsub"},
		{"nested all segments", `{{call .repo.FullNameByRepoURL .url 0}}`, `repo.FullNameByRepoURL(url, 0)`, nestedGitLabURL, "group/sub/subsub/project"},
		{"nested negative means all", `{{call .repo.FullNameByRepoURL .url -1}}`, `repo.FullNameByRepoURL(url, -1)`, nestedGitLabURL, "group/sub/subsub/project"},
		{"github browse link default", `{{call .repo.FullNameByRepoURL .url}}`, `repo.FullNameByRepoURL(url)`, gitHubBrowseURL, "argoproj/argo-cd"},
	}
	for _, tt := range tests {
		vars := map[string]any{"repo": NewExprs(nil, nil), "url": tt.url}

		t.Run(tt.name+" template", func(t *testing.T) {
			tmpl, err := template.New("test").Parse(tt.template)
			require.NoError(t, err)
			var out bytes.Buffer
			require.NoError(t, tmpl.Execute(&out, vars))
			assert.Equal(t, tt.expected, out.String())
		})

		t.Run(tt.name+" expr", func(t *testing.T) {
			// notifications-engine compiles trigger conditions without an env.
			program, err := expr.Compile(tt.expr)
			require.NoError(t, err)
			out, err := expr.Run(program, vars)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, out)
		})
	}
}
