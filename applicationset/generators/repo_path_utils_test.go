package generators

import (
	"context"
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// repoServerFiles is what the repo-server returns for a pattern set: the union of the
// include matches with the exclude matches removed. Both filters are applied there now,
// so the generator issues a single call and receives the already-filtered result.
func repoServerFiles(includeFiles, excludeFiles map[string][]byte) map[string][]byte {
	files := maps.Clone(includeFiles)
	for path := range excludeFiles {
		delete(files, path)
	}
	return files
}

func TestParseFileParams(t *testing.T) {
	defaultContent := []byte(`
foo:
  bar: baz
`)
	type args struct {
		filePath          string
		fileContent       []byte
		values            map[string]string
		useGoTemplate     bool
		goTemplateOptions []string
		pathParamPrefix   string
	}
	tests := []struct {
		name    string
		args    args
		want    []map[string]any
		wantErr bool
	}{
		{
			name: "empty file returns path parameters",
			args: args{
				filePath:      "path/dir/file_name.yaml",
				fileContent:   []byte(""),
				values:        map[string]string{},
				useGoTemplate: false,
			},
			want: []map[string]any{
				{
					"path":                    "path/dir",
					"path.basename":           "dir",
					"path.filename":           "file_name.yaml",
					"path.basenameNormalized": "dir",
					"path.filenameNormalized": "file-name.yaml",
					"path[0]":                 "path",
					"path[1]":                 "dir",
				},
			},
		},
		{
			name: "invalid json/yaml file returns error",
			args: args{
				filePath:      "path/dir/file_name.yaml",
				fileContent:   []byte("this is not json or yaml"),
				values:        map[string]string{},
				useGoTemplate: false,
			},
			wantErr: true,
		},
		{
			name: "file parameters are added to params",
			args: args{
				filePath:      "path/dir/file_name.yaml",
				fileContent:   defaultContent,
				values:        map[string]string{},
				useGoTemplate: false,
			},
			want: []map[string]any{
				{
					"foo.bar":                 "baz",
					"path":                    "path/dir",
					"path.basename":           "dir",
					"path.filename":           "file_name.yaml",
					"path.basenameNormalized": "dir",
					"path.filenameNormalized": "file-name.yaml",
					"path[0]":                 "path",
					"path[1]":                 "dir",
				},
			},
		},
		{
			name: "path parameter are prefixed",
			args: args{
				filePath:        "path/dir/file_name.yaml",
				fileContent:     defaultContent,
				values:          map[string]string{},
				useGoTemplate:   false,
				pathParamPrefix: "myRepo",
			},
			want: []map[string]any{
				{
					"foo.bar":                        "baz",
					"myRepo.path":                    "path/dir",
					"myRepo.path.basename":           "dir",
					"myRepo.path.filename":           "file_name.yaml",
					"myRepo.path.basenameNormalized": "dir",
					"myRepo.path.filenameNormalized": "file-name.yaml",
					"myRepo.path[0]":                 "path",
					"myRepo.path[1]":                 "dir",
				},
			},
		},
		{
			name: "file parameters are added to params with go template",
			args: args{
				filePath:    "path/dir/file_name.yaml",
				fileContent: defaultContent,
				values: map[string]string{
					"somekey": "{{.path.basename}}",
				},
				useGoTemplate: true,
			},
			want: []map[string]any{
				{
					"values": map[string]string{
						"somekey": "dir",
					},
					"foo": map[string]any{
						"bar": "baz",
					},
					"path": map[string]any{
						"path":               "path/dir",
						"basename":           "dir",
						"filename":           "file_name.yaml",
						"basenameNormalized": "dir",
						"filenameNormalized": "file-name.yaml",
						"segments": []string{
							"path",
							"dir",
						},
					},
				},
			},
		},
		{
			name: "path parameter are prefixed with go template",
			args: args{
				filePath:        "path/dir/file_name.yaml",
				fileContent:     defaultContent,
				values:          map[string]string{},
				useGoTemplate:   true,
				pathParamPrefix: "myRepo",
			},
			want: []map[string]any{
				{
					"foo": map[string]any{
						"bar": "baz",
					},
					"myRepo": map[string]any{
						"path": map[string]any{
							"path":               "path/dir",
							"basename":           "dir",
							"filename":           "file_name.yaml",
							"basenameNormalized": "dir",
							"filenameNormalized": "file-name.yaml",
							"segments": []string{
								"path",
								"dir",
							},
						},
					},
				},
			},
		},
		{
			name: "path parameter are prefixed with go template and values",
			args: args{
				filePath:    "path/dir/file_name.yaml",
				fileContent: defaultContent,
				values: map[string]string{
					"somekey": "{{.myRepo.path.basename}}",
				},
				useGoTemplate:   true,
				pathParamPrefix: "myRepo",
			},
			want: []map[string]any{
				{
					"foo": map[string]any{
						"bar": "baz",
					},
					"myRepo": map[string]any{
						"path": map[string]any{
							"path":               "path/dir",
							"basename":           "dir",
							"filename":           "file_name.yaml",
							"basenameNormalized": "dir",
							"filenameNormalized": "file-name.yaml",
							"segments": []string{
								"path",
								"dir",
							},
						},
					},
					"values": map[string]string{
						"somekey": "dir",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, err := parseFileParams(tt.args.filePath, tt.args.fileContent, tt.args.pathParamPrefix, tt.args.values, tt.args.useGoTemplate, tt.args.goTemplateOptions)
			if tt.wantErr {
				assert.Error(t, err, "GitGenerator.parseFileParams()")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, params)
			}
		})
	}
}

func TestFilterPaths(t *testing.T) {
	tests := []struct {
		name        string
		directories []pathPattern
		allPaths    []string
		expected    []string
	}{
		{
			name:        "single wildcard matches all top-level",
			directories: []pathPattern{{Path: "*"}},
			allPaths:    []string{"app1", "app2", "p1/app3"},
			expected:    []string{"app1", "app2"},
		},
		{
			name:        "nested wildcard",
			directories: []pathPattern{{Path: "p1/*"}},
			allPaths:    []string{"app1", "p1/app2", "p1/p2/app3"},
			expected:    []string{"p1/app2"},
		},
		{
			name:        "exclude pattern",
			directories: []pathPattern{{Path: "*"}, {Path: "app1", Exclude: true}},
			allPaths:    []string{"app1", "app2", "app3"},
			expected:    []string{"app2", "app3"},
		},
		{
			name:        "exclude takes precedence",
			directories: []pathPattern{{Path: "*"}, {Path: "*/*"}, {Path: "p1/*", Exclude: true}},
			allPaths:    []string{"app1", "p1/app2", "p2/app3"},
			expected:    []string{"app1", "p2/app3"},
		},
		{
			name:        "multiple patterns with exclusions",
			directories: []pathPattern{{Path: "*/prod"}, {Path: "*/staging"}, {Path: "old/*", Exclude: true}},
			allPaths:    []string{"cluster1/prod", "cluster1/staging", "cluster2/prod", "old/prod", "old/staging"},
			expected:    []string{"cluster1/prod", "cluster1/staging", "cluster2/prod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterPaths(tt.directories, tt.allPaths)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildPathParameters(t *testing.T) {
	tests := []struct {
		name            string
		appPath         string
		pathParamPrefix string
		useGoTemplate   bool
		expected        map[string]any
	}{
		{
			name:            "flat parameters without prefix",
			appPath:         "p1/p2/app",
			pathParamPrefix: "",
			useGoTemplate:   false,
			expected: map[string]any{
				"path":                    "p1/p2/app",
				"path.basename":           "app",
				"path.basenameNormalized": "app",
				"path[0]":                 "p1",
				"path[1]":                 "p2",
				"path[2]":                 "app",
			},
		},
		{
			name:            "flat parameters with prefix",
			appPath:         "app1",
			pathParamPrefix: "myRepo",
			useGoTemplate:   false,
			expected: map[string]any{
				"myRepo.path":                    "app1",
				"myRepo.path.basename":           "app1",
				"myRepo.path.basenameNormalized": "app1",
				"myRepo.path[0]":                 "app1",
			},
		},
		{
			name:            "go template without prefix",
			appPath:         "p1/app",
			pathParamPrefix: "",
			useGoTemplate:   true,
			expected: map[string]any{
				"path": map[string]any{
					"path":               "p1/app",
					"basename":           "app",
					"basenameNormalized": "app",
					"segments":           []string{"p1", "app"},
				},
			},
		},
		{
			name:            "go template with prefix",
			appPath:         "app_name",
			pathParamPrefix: "myRepo",
			useGoTemplate:   true,
			expected: map[string]any{
				"myRepo": map[string]any{
					"path": map[string]any{
						"path":               "app_name",
						"basename":           "app_name",
						"basenameNormalized": "app-name",
						"segments":           []string{"app_name"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildPathParameters(tt.appPath, tt.pathParamPrefix, tt.useGoTemplate)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateParamsFromPaths(t *testing.T) {
	tests := []struct {
		name              string
		paths             []string
		pathParamPrefix   string
		values            map[string]string
		useGoTemplate     bool
		goTemplateOptions []string
		expected          []map[string]any
		wantErr           bool
	}{
		{
			name:            "multiple paths flat",
			paths:           []string{"app1", "app2"},
			pathParamPrefix: "",
			values:          nil,
			useGoTemplate:   false,
			expected: []map[string]any{
				{
					"path":                    "app1",
					"path.basename":           "app1",
					"path.basenameNormalized": "app1",
					"path[0]":                 "app1",
				},
				{
					"path":                    "app2",
					"path.basename":           "app2",
					"path.basenameNormalized": "app2",
					"path[0]":                 "app2",
				},
			},
		},
		{
			name:            "paths with values interpolation",
			paths:           []string{"env/prod", "env/staging"},
			pathParamPrefix: "",
			values: map[string]string{
				"cluster": "{{ path[1] }}",
			},
			useGoTemplate: false,
			expected: []map[string]any{
				{
					"path":                    "env/prod",
					"path.basename":           "prod",
					"path.basenameNormalized": "prod",
					"path[0]":                 "env",
					"path[1]":                 "prod",
					"values.cluster":          "prod",
				},
				{
					"path":                    "env/staging",
					"path.basename":           "staging",
					"path.basenameNormalized": "staging",
					"path[0]":                 "env",
					"path[1]":                 "staging",
					"values.cluster":          "staging",
				},
			},
		},
		{
			name:            "go template mode",
			paths:           []string{"app1"},
			pathParamPrefix: "",
			values:          nil,
			useGoTemplate:   true,
			expected: []map[string]any{
				{
					"path": map[string]any{
						"path":               "app1",
						"basename":           "app1",
						"basenameNormalized": "app1",
						"segments":           []string{"app1"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := generateParamsFromPaths(tt.paths, tt.pathParamPrefix, tt.values, tt.useGoTemplate, tt.goTemplateOptions)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestResolveProjectName(t *testing.T) {
	tests := []struct {
		name     string
		project  string
		expected string
	}{
		{
			name:     "plain project name",
			project:  "my-project",
			expected: "my-project",
		},
		{
			name:     "templated project name returns empty",
			project:  "{{ .Values.project }}",
			expected: "",
		},
		{
			name:     "partial template returns empty",
			project:  "project-{{ .Values.env }}",
			expected: "",
		},
		{
			name:     "empty project name",
			project:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveProjectName(tt.project)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGenerateRepoSourceFileParamsSingleCall pins the contract that made this shared:
// however many include and exclude patterns a generator declares, the repo source is
// asked exactly once, since resolving it (cloning a repo, or fetching and decompressing
// an OCI artifact) is the expensive part.
func TestGenerateRepoSourceFileParamsSingleCall(t *testing.T) {
	t.Parallel()

	t.Run("every pattern travels in one call", func(t *testing.T) {
		t.Parallel()
		src := &countingRepoSource{files: map[string][]byte{"apps/prod/config.json": []byte(`{"env":"prod"}`)}}

		params, err := generateRepoSourceFileParams(t.Context(), repoSourceCallParams{
			src:  src,
			kind: repoSourceKindOCI,
			spec: repoSourceSpec{
				URL:      "oci://ghcr.io/example/manifests",
				Revision: "v1.0.0",
				Files: []pathPattern{
					{Path: "apps/*/config.json"},
					{Path: "shared/*.json"},
					{Path: "apps/skip/config.json", Exclude: true},
				},
			},
		})
		require.NoError(t, err)

		assert.Equal(t, 1, src.calls)
		assert.Equal(t, []string{"apps/*/config.json", "shared/*.json"}, src.includePatterns)
		assert.Equal(t, []string{"apps/skip/config.json"}, src.excludePatterns)
		assert.Len(t, params, 1)
		assert.Equal(t, "prod", params[0]["env"])
	})

	t.Run("exclude patterns alone fetch nothing", func(t *testing.T) {
		t.Parallel()
		src := &countingRepoSource{}

		params, err := generateRepoSourceFileParams(t.Context(), repoSourceCallParams{
			src:  src,
			kind: repoSourceKindGit,
			spec: repoSourceSpec{Files: []pathPattern{{Path: "apps/*.json", Exclude: true}}},
		})
		require.NoError(t, err)
		assert.Empty(t, params)
		assert.Equal(t, 0, src.calls)
	})
}

// countingRepoSource records what getFiles was asked for and how often.
type countingRepoSource struct {
	files           map[string][]byte
	calls           int
	includePatterns []string
	excludePatterns []string
}

func (s *countingRepoSource) resolveSourceIntegrity(_ context.Context, _ *argoprojiov1alpha1.ApplicationSet, _ client.Client) (*argoprojiov1alpha1.SourceIntegrity, error) {
	return nil, nil
}

func (s *countingRepoSource) listDirectories(_ context.Context, _, _, _ string, _ bool, _ *argoprojiov1alpha1.SourceIntegrity) ([]string, error) {
	return nil, nil
}

func (s *countingRepoSource) getFiles(_ context.Context, _, _, _ string, includePatterns, excludePatterns []string, _ bool, _ *argoprojiov1alpha1.SourceIntegrity) (map[string][]byte, error) {
	s.calls++
	s.includePatterns = includePatterns
	s.excludePatterns = excludePatterns
	return s.files, nil
}
