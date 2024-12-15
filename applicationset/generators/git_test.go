package generators

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj/argo-cd/v2/applicationset/services/mocks"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func Test_generateParamsFromGitFile(t *testing.T) {
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
		noRevisionCache   bool
		verifyCommit      bool
		extraParamFiles   map[string]interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    []map[string]interface{}
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
			want: []map[string]interface{}{
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
			want: []map[string]interface{}{
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
			want: []map[string]interface{}{
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
				filePath:      "path/dir/file_name.yaml",
				fileContent:   defaultContent,
				values:        map[string]string{},
				useGoTemplate: true,
			},
			want: []map[string]interface{}{
				{
					"foo": map[string]interface{}{
						"bar": "baz",
					},
					"path": map[string]interface{}{
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
			want: []map[string]interface{}{
				{
					"foo": map[string]interface{}{
						"bar": "baz",
					},
					"myRepo": map[string]interface{}{
						"path": map[string]interface{}{
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
			// Extra param files should be merged in the order : firstConfig.json, secondConfig.json (alphabetical sorted in the same glob), override.yaml
			name: "extra params with go template",
			args: args{
				filePath:        "path/dir/file_name.yaml",
				fileContent:     defaultContent,
				values:          map[string]string{},
				useGoTemplate:   true,
				pathParamPrefix: "myRepo",
				extraParamFiles: map[string]interface{}{
					"path/dir/**/*.json": map[string][]byte{
						"path/dir/mypath/other/firstConfig.json": []byte(`{
	"firstKey": {
		"foo":  "bar",
		"foo2": "bar2",
		"foo3": true,
		"foo4": {
			"foo4foo":  "barbar1",
			"foo4foo2": "barbar2",
		},
	},
	"secondKey": false,
	"thirdKey":  "thirdValue",
}`),
						"path/dir/mypath/other/secondConfig.json": []byte(`{
	"firstKey": {
		"foo":  "barbarbar",
	},
	"thirdKey":  "thirdValueOverriden",
}`),
					},
					"path/otherdir/override.yaml": map[string][]byte{
						"path/otherdir/override.yaml": []byte(`
firstKey:
  foo2: bar2Override
  foo4:
    foo4foo2: barbar2Override
    foo5foo3: barbar3Override
secondKey: true
otherKey:  otherValue
`),
					},
				},
			},
			want: []map[string]interface{}{
				{
					"foo": map[string]interface{}{
						"bar": "baz",
					},
					"myRepo": map[string]interface{}{
						"path": map[string]interface{}{
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
					"extraParams": map[string]interface{}{
						"firstKey": map[string]interface{}{
							"foo":  "barbarbar",
							"foo2": "bar2Override",
							"foo3": true,
							"foo4": map[string]interface{}{
								"foo4foo":  "barbar1",
								"foo4foo2": "barbar2Override",
								"foo5foo3": "barbar3Override",
							},
						},
						"secondKey": true,
						"thirdKey":  "thirdValueOverriden",
						"otherKey":  "otherValue",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argoCDServiceMock := mocks.Repos{}

			var extraParamFiles []string
			if len(tt.args.extraParamFiles) != 0 {
				keys := make([]string, 0, len(tt.args.extraParamFiles))
				for k := range tt.args.extraParamFiles {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				extraParamFiles = make([]string, 0)
				for _, key := range keys {
					argoCDServiceMock.On("GetFiles", mock.Anything, mock.Anything, mock.Anything, key, mock.Anything, mock.Anything).
						Return(tt.args.extraParamFiles[key], nil)
					extraParamFiles = append(extraParamFiles, key)
				}
			}

			git := argoprojiov1alpha1.GitGenerator{
				Values:              tt.args.values,
				PathParamPrefix:     tt.args.pathParamPrefix,
				ExtraParameterFiles: extraParamFiles,
			}

			gitGenerator := NewGitGenerator(&argoCDServiceMock, "").(*GitGenerator)
			params, err := gitGenerator.generateParamsFromGitFile(tt.args.filePath, tt.args.fileContent, &git, tt.args.goTemplateOptions, tt.args.useGoTemplate, tt.args.noRevisionCache, tt.args.verifyCommit)
			if (err != nil) != tt.wantErr {
				t.Errorf("GitGenerator.generateParamsFromGitFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, params)
		})
	}
}

func TestGenerateParamsFromGitFileWithExtraParamsAndTemplatizedPath(t *testing.T) {
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
		noRevisionCache   bool
		verifyCommit      bool
		templatizedPath   string
		extraParamFiles   map[string]interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    []map[string]interface{}
		wantErr string
	}{
		{
			name: "extra params with templatized path",
			args: args{
				filePath:          "path/dir/file_name.yaml",
				fileContent:       defaultContent,
				values:            map[string]string{},
				useGoTemplate:     true,
				goTemplateOptions: []string{"missingkey=error"},
				pathParamPrefix:   "myRepo",
				templatizedPath:   "path/dir/{{.foo.bar}}.json",
				extraParamFiles: map[string]interface{}{
					"path/dir/baz.json": map[string][]byte{
						"path/dir/baz.json": []byte(`{
	"firstKey": {
		"foo":  "bar",
		"foo2": "bar2",
		"foo3": true,
		"foo4": {
			"foo4foo":  "barbar1",
			"foo4foo2": "barbar2",
		},
	},
	"secondKey": false,
	"thirdKey":  "thirdValue",
}`),
					},
				},
			},
			want: []map[string]interface{}{
				{
					"foo": map[string]interface{}{
						"bar": "baz",
					},
					"myRepo": map[string]interface{}{
						"path": map[string]interface{}{
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
					"extraParams": map[string]interface{}{
						"firstKey": map[string]interface{}{
							"foo":  "bar",
							"foo2": "bar2",
							"foo3": true,
							"foo4": map[string]interface{}{
								"foo4foo":  "barbar1",
								"foo4foo2": "barbar2",
							},
						},
						"secondKey": false,
						"thirdKey":  "thirdValue",
					},
				},
			},
		},
		{
			name: "extra params with templatized path, value not found",
			args: args{
				filePath:          "path/dir/file_name.yaml",
				fileContent:       defaultContent,
				values:            map[string]string{},
				useGoTemplate:     true,
				goTemplateOptions: []string{"missingkey=error"},
				pathParamPrefix:   "myRepo",
				templatizedPath:   "path/dir/{{.foo.barNotFound}}.json",
				extraParamFiles:   map[string]interface{}{},
			},
			wantErr: "failed to append extra parameters from files: failed to replace templated string with rendered values: failed to execute go template path/dir/{{.foo.barNotFound}}.json: template: :1:15: executing \"\" at <.foo.barNotFound>: map has no entry for key \"barNotFound\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argoCDServiceMock := mocks.Repos{}

			if len(tt.args.extraParamFiles) != 0 {
				for fileName, extraParamFileContent := range tt.args.extraParamFiles {
					argoCDServiceMock.On("GetFiles", mock.Anything, mock.Anything, mock.Anything, fileName, mock.Anything, mock.Anything).
						Return(extraParamFileContent, nil)
				}
			}

			git := argoprojiov1alpha1.GitGenerator{
				Values:          tt.args.values,
				PathParamPrefix: tt.args.pathParamPrefix,
			}
			if tt.args.templatizedPath != "" {
				git.ExtraParameterFiles = []string{tt.args.templatizedPath}
			}

			gitGenerator := NewGitGenerator(&argoCDServiceMock, "").(*GitGenerator)
			params, err := gitGenerator.generateParamsFromGitFile(tt.args.filePath, tt.args.fileContent, &git, tt.args.goTemplateOptions, tt.args.useGoTemplate, tt.args.noRevisionCache, tt.args.verifyCommit)
			if err != nil {
				if tt.wantErr == "" {
					t.Errorf("GitGenerator.generateParamsFromGitFile() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				require.EqualError(t, err, tt.wantErr)
			}
			assert.Equal(t, tt.want, params)
		})
	}
}

func TestGitGenerateParamsFromDirectories(t *testing.T) {
	cases := []struct {
		name            string
		directories     []argoprojiov1alpha1.GitDirectoryGeneratorItem
		pathParamPrefix string
		repoApps        []string
		repoError       error
		values          map[string]string
		extraParamFiles map[string][]byte
		expected        []map[string]interface{}
		expectedError   error
	}{
		{
			name:        "happy flow - created apps",
			directories: []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
			repoApps: []string{
				"app1",
				"app2",
				"app_3",
				"p1/app4",
			},
			repoError: nil,
			expected: []map[string]interface{}{
				{"path": "app1", "path.basename": "app1", "path.basenameNormalized": "app1", "path[0]": "app1"},
				{"path": "app2", "path.basename": "app2", "path.basenameNormalized": "app2", "path[0]": "app2"},
				{"path": "app_3", "path.basename": "app_3", "path.basenameNormalized": "app-3", "path[0]": "app_3"},
			},
			expectedError: nil,
		},
		{
			name:            "It prefixes path parameters with PathParamPrefix",
			directories:     []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
			pathParamPrefix: "myRepo",
			repoApps: []string{
				"app1",
				"app2",
				"app_3",
				"p1/app4",
			},
			repoError: nil,
			expected: []map[string]interface{}{
				{"myRepo.path": "app1", "myRepo.path.basename": "app1", "myRepo.path.basenameNormalized": "app1", "myRepo.path[0]": "app1"},
				{"myRepo.path": "app2", "myRepo.path.basename": "app2", "myRepo.path.basenameNormalized": "app2", "myRepo.path[0]": "app2"},
				{"myRepo.path": "app_3", "myRepo.path.basename": "app_3", "myRepo.path.basenameNormalized": "app-3", "myRepo.path[0]": "app_3"},
			},
			expectedError: nil,
		},
		{
			name:        "It filters application according to the paths",
			directories: []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "p1/*"}, {Path: "p1/*/*"}},
			repoApps: []string{
				"app1",
				"p1/app2",
				"p1/p2/app3",
				"p1/p2/p3/app4",
			},
			repoError: nil,
			expected: []map[string]interface{}{
				{"path": "p1/app2", "path.basename": "app2", "path[0]": "p1", "path[1]": "app2", "path.basenameNormalized": "app2"},
				{"path": "p1/p2/app3", "path.basename": "app3", "path[0]": "p1", "path[1]": "p2", "path[2]": "app3", "path.basenameNormalized": "app3"},
			},
			expectedError: nil,
		},
		{
			name:        "It filters application according to the paths with Exclude",
			directories: []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "p1/*", Exclude: true}, {Path: "*"}, {Path: "*/*"}},
			repoApps: []string{
				"app1",
				"app2",
				"p1/app2",
				"p1/app3",
				"p2/app3",
			},
			repoError: nil,
			expected: []map[string]interface{}{
				{"path": "app1", "path.basename": "app1", "path[0]": "app1", "path.basenameNormalized": "app1"},
				{"path": "app2", "path.basename": "app2", "path[0]": "app2", "path.basenameNormalized": "app2"},
				{"path": "p2/app3", "path.basename": "app3", "path[0]": "p2", "path[1]": "app3", "path.basenameNormalized": "app3"},
			},
			expectedError: nil,
		},
		{
			name:        "Expecting same exclude behavior with different order",
			directories: []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}, {Path: "*/*"}, {Path: "p1/*", Exclude: true}},
			repoApps: []string{
				"app1",
				"app2",
				"p1/app2",
				"p1/app3",
				"p2/app3",
			},
			repoError: nil,
			expected: []map[string]interface{}{
				{"path": "app1", "path.basename": "app1", "path[0]": "app1", "path.basenameNormalized": "app1"},
				{"path": "app2", "path.basename": "app2", "path[0]": "app2", "path.basenameNormalized": "app2"},
				{"path": "p2/app3", "path.basename": "app3", "path[0]": "p2", "path[1]": "app3", "path.basenameNormalized": "app3"},
			},
			expectedError: nil,
		},
		{
			name:        "Value variable interpolation",
			directories: []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}, {Path: "*/*"}},
			repoApps: []string{
				"app1",
				"p1/app2",
			},
			repoError: nil,
			values: map[string]string{
				"foo":   "bar",
				"aaa":   "{{ path[0] }}",
				"no-op": "{{ this-does-not-exist }}",
			},
			expected: []map[string]interface{}{
				{"values.foo": "bar", "values.no-op": "{{ this-does-not-exist }}", "values.aaa": "app1", "path": "app1", "path.basename": "app1", "path[0]": "app1", "path.basenameNormalized": "app1"},
				{"values.foo": "bar", "values.no-op": "{{ this-does-not-exist }}", "values.aaa": "p1", "path": "p1/app2", "path.basename": "app2", "path[0]": "p1", "path[1]": "app2", "path.basenameNormalized": "app2"},
			},
			expectedError: nil,
		},
		{
			name:          "handles empty response from repo server",
			directories:   []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
			repoApps:      []string{},
			repoError:     nil,
			expected:      []map[string]interface{}{},
			expectedError: nil,
		},
		{
			name:          "handles error from repo server",
			directories:   []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
			repoApps:      []string{},
			repoError:     fmt.Errorf("error"),
			expected:      []map[string]interface{}{},
			expectedError: fmt.Errorf("error generating params from git: error getting directories from repo: error"),
		},
		{
			name:        "With extra params",
			directories: []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
			repoApps: []string{
				"app1",
				"app2",
				"app_3",
				"p1/app4",
			},
			repoError: nil,
			extraParamFiles: map[string][]byte{
				"path/dir/**/*/json": []byte(`{
"firstKey": {
	"foo":  "bar",
	"foo2": "bar2",
	"foo3": true,
	"foo4": {
		"foo4foo":  "barbar1",
		"foo4foo2": "barbar2",
	},
},
"secondKey": false,
"thirdKey":  "thirdValue",
}`),
				"path/otherdir/override.yaml": []byte(`
firstKey: 
  foo2: bar2Override
  foo4:
    foo4foo2: barbar2Override
    foo5foo3: barbar3Override
secondKey: true
otherKey:  otherValue
`),
			},
			expected: []map[string]interface{}{
				{
					"path":                               "app1",
					"path.basename":                      "app1",
					"path.basenameNormalized":            "app1",
					"path[0]":                            "app1",
					"extraParams.firstKey.foo":           "bar",
					"extraParams.firstKey.foo2":          "bar2Override",
					"extraParams.firstKey.foo3":          "true",
					"extraParams.firstKey.foo4.foo4foo":  "barbar1",
					"extraParams.firstKey.foo4.foo4foo2": "barbar2Override",
					"extraParams.firstKey.foo4.foo5foo3": "barbar3Override",
					"extraParams.secondKey":              "true",
					"extraParams.thirdKey":               "thirdValue",
					"extraParams.otherKey":               "otherValue",
				},
				{
					"path":                               "app2",
					"path.basename":                      "app2",
					"path.basenameNormalized":            "app2",
					"path[0]":                            "app2",
					"extraParams.firstKey.foo":           "bar",
					"extraParams.firstKey.foo2":          "bar2Override",
					"extraParams.firstKey.foo3":          "true",
					"extraParams.firstKey.foo4.foo4foo":  "barbar1",
					"extraParams.firstKey.foo4.foo4foo2": "barbar2Override",
					"extraParams.firstKey.foo4.foo5foo3": "barbar3Override",
					"extraParams.secondKey":              "true",
					"extraParams.thirdKey":               "thirdValue",
					"extraParams.otherKey":               "otherValue",
				},
				{
					"path":                               "app_3",
					"path.basename":                      "app_3",
					"path.basenameNormalized":            "app-3",
					"path[0]":                            "app_3",
					"extraParams.firstKey.foo":           "bar",
					"extraParams.firstKey.foo2":          "bar2Override",
					"extraParams.firstKey.foo3":          "true",
					"extraParams.firstKey.foo4.foo4foo":  "barbar1",
					"extraParams.firstKey.foo4.foo4foo2": "barbar2Override",
					"extraParams.firstKey.foo4.foo5foo3": "barbar3Override",
					"extraParams.secondKey":              "true",
					"extraParams.thirdKey":               "thirdValue",
					"extraParams.otherKey":               "otherValue",
				},
			},
		},
	}

	for _, testCase := range cases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			argoCDServiceMock := mocks.Repos{}

			argoCDServiceMock.On("GetDirectories", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(testCaseCopy.repoApps, testCaseCopy.repoError)

			var extraParamFiles []string
			if len(testCaseCopy.extraParamFiles) != 0 {
				keys := make([]string, 0, len(testCaseCopy.extraParamFiles))
				for k := range testCaseCopy.extraParamFiles {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				extraParamFiles = make([]string, 0)
				for _, key := range keys {
					extraParamFiles = append(extraParamFiles, key)

					argoCDServiceMock.On("GetFiles", mock.Anything, mock.Anything, mock.Anything, key, mock.Anything, mock.Anything).
						Return(map[string][]byte{
							key: testCaseCopy.extraParamFiles[key],
						}, nil)
				}
			}

			gitGenerator := NewGitGenerator(&argoCDServiceMock, "")
			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					Generators: []argoprojiov1alpha1.ApplicationSetGenerator{{
						Git: &argoprojiov1alpha1.GitGenerator{
							RepoURL:             "RepoURL",
							Revision:            "Revision",
							Directories:         testCaseCopy.directories,
							PathParamPrefix:     testCaseCopy.pathParamPrefix,
							Values:              testCaseCopy.values,
							ExtraParameterFiles: extraParamFiles,
						},
					}},
				},
			}

			scheme := runtime.NewScheme()
			err := v1alpha1.AddToScheme(scheme)
			require.NoError(t, err)
			appProject := argoprojiov1alpha1.AppProject{}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appProject).Build()

			got, err := gitGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo, client)

			if testCaseCopy.expectedError != nil {
				require.EqualError(t, err, testCaseCopy.expectedError.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, testCaseCopy.expected, got)
			}

			argoCDServiceMock.AssertExpectations(t)
		})
	}
}

func TestGitGenerateParamsFromDirectoriesGoTemplate(t *testing.T) {
	cases := []struct {
		name            string
		directories     []argoprojiov1alpha1.GitDirectoryGeneratorItem
		pathParamPrefix string
		repoApps        []string
		repoError       error
		extraParamFiles map[string][]byte
		expected        []map[string]interface{}
		expectedError   error
	}{
		{
			name:        "happy flow - created apps",
			directories: []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
			repoApps: []string{
				"app1",
				"app2",
				"app_3",
				"p1/app4",
			},
			repoError: nil,
			expected: []map[string]interface{}{
				{
					"path": map[string]interface{}{
						"path":               "app1",
						"basename":           "app1",
						"basenameNormalized": "app1",
						"segments": []string{
							"app1",
						},
					},
				},
				{
					"path": map[string]interface{}{
						"path":               "app2",
						"basename":           "app2",
						"basenameNormalized": "app2",
						"segments": []string{
							"app2",
						},
					},
				},
				{
					"path": map[string]interface{}{
						"path":               "app_3",
						"basename":           "app_3",
						"basenameNormalized": "app-3",
						"segments": []string{
							"app_3",
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name:            "It prefixes path parameters with PathParamPrefix",
			directories:     []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
			pathParamPrefix: "myRepo",
			repoApps: []string{
				"app1",
				"app2",
				"app_3",
				"p1/app4",
			},
			repoError: nil,
			expected: []map[string]interface{}{
				{
					"myRepo": map[string]interface{}{
						"path": map[string]interface{}{
							"path":               "app1",
							"basename":           "app1",
							"basenameNormalized": "app1",
							"segments": []string{
								"app1",
							},
						},
					},
				},
				{
					"myRepo": map[string]interface{}{
						"path": map[string]interface{}{
							"path":               "app2",
							"basename":           "app2",
							"basenameNormalized": "app2",
							"segments": []string{
								"app2",
							},
						},
					},
				},
				{
					"myRepo": map[string]interface{}{
						"path": map[string]interface{}{
							"path":               "app_3",
							"basename":           "app_3",
							"basenameNormalized": "app-3",
							"segments": []string{
								"app_3",
							},
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name:        "It filters application according to the paths",
			directories: []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "p1/*"}, {Path: "p1/*/*"}},
			repoApps: []string{
				"app1",
				"p1/app2",
				"p1/p2/app3",
				"p1/p2/p3/app4",
			},
			repoError: nil,
			expected: []map[string]interface{}{
				{
					"path": map[string]interface{}{
						"path":               "p1/app2",
						"basename":           "app2",
						"basenameNormalized": "app2",
						"segments": []string{
							"p1",
							"app2",
						},
					},
				},
				{
					"path": map[string]interface{}{
						"path":               "p1/p2/app3",
						"basename":           "app3",
						"basenameNormalized": "app3",
						"segments": []string{
							"p1",
							"p2",
							"app3",
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name:        "It filters application according to the paths with Exclude",
			directories: []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "p1/*", Exclude: true}, {Path: "*"}, {Path: "*/*"}},
			repoApps: []string{
				"app1",
				"app2",
				"p1/app2",
				"p1/app3",
				"p2/app3",
			},
			repoError: nil,
			expected: []map[string]interface{}{
				{
					"path": map[string]interface{}{
						"path":               "app1",
						"basename":           "app1",
						"basenameNormalized": "app1",
						"segments": []string{
							"app1",
						},
					},
				},
				{
					"path": map[string]interface{}{
						"path":               "app2",
						"basename":           "app2",
						"basenameNormalized": "app2",
						"segments": []string{
							"app2",
						},
					},
				},
				{
					"path": map[string]interface{}{
						"path":               "p2/app3",
						"basename":           "app3",
						"basenameNormalized": "app3",
						"segments": []string{
							"p2",
							"app3",
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name:        "Expecting same exclude behavior with different order",
			directories: []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}, {Path: "*/*"}, {Path: "p1/*", Exclude: true}},
			repoApps: []string{
				"app1",
				"app2",
				"p1/app2",
				"p1/app3",
				"p2/app3",
			},
			repoError: nil,
			expected: []map[string]interface{}{
				{
					"path": map[string]interface{}{
						"path":               "app1",
						"basename":           "app1",
						"basenameNormalized": "app1",
						"segments": []string{
							"app1",
						},
					},
				},
				{
					"path": map[string]interface{}{
						"path":               "app2",
						"basename":           "app2",
						"basenameNormalized": "app2",
						"segments": []string{
							"app2",
						},
					},
				},
				{
					"path": map[string]interface{}{
						"path":               "p2/app3",
						"basename":           "app3",
						"basenameNormalized": "app3",
						"segments": []string{
							"p2",
							"app3",
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name:          "handles empty response from repo server",
			directories:   []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
			repoApps:      []string{},
			repoError:     nil,
			expected:      []map[string]interface{}{},
			expectedError: nil,
		},
		{
			name:          "handles error from repo server",
			directories:   []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
			repoApps:      []string{},
			repoError:     fmt.Errorf("error"),
			expected:      []map[string]interface{}{},
			expectedError: fmt.Errorf("error generating params from git: error getting directories from repo: error"),
		},
		{
			name:        "With extra params",
			directories: []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
			repoApps: []string{
				"app1",
				"app2",
				"app_3",
				"p1/app4",
			},
			extraParamFiles: map[string][]byte{
				"path/dir/**/*/json": []byte(`{
"firstKey": {
	"foo":  "bar",
	"foo2": "bar2",
	"foo3": true,
	"foo4": {
		"foo4foo":  "barbar1",
		"foo4foo2": "barbar2",
	},
},
"secondKey": false,
"thirdKey":  "thirdValue",
}`),
				"path/otherdir/override.yaml": []byte(`
firstKey: 
  foo2: bar2Override
  foo4:
    foo4foo2: barbar2Override
    foo5foo3: barbar3Override
secondKey: true
otherKey:  otherValue
`),
			},
			repoError: nil,
			expected: []map[string]interface{}{
				{
					"path": map[string]interface{}{
						"path":               "app1",
						"basename":           "app1",
						"basenameNormalized": "app1",
						"segments": []string{
							"app1",
						},
					},
					"extraParams": map[string]interface{}{
						"firstKey": map[string]interface{}{
							"foo":  "bar",
							"foo2": "bar2Override",
							"foo3": true,
							"foo4": map[string]interface{}{
								"foo4foo":  "barbar1",
								"foo4foo2": "barbar2Override",
								"foo5foo3": "barbar3Override",
							},
						},
						"secondKey": true,
						"thirdKey":  "thirdValue",
						"otherKey":  "otherValue",
					},
				},
				{
					"path": map[string]interface{}{
						"path":               "app2",
						"basename":           "app2",
						"basenameNormalized": "app2",
						"segments": []string{
							"app2",
						},
					},
					"extraParams": map[string]interface{}{
						"firstKey": map[string]interface{}{
							"foo":  "bar",
							"foo2": "bar2Override",
							"foo3": true,
							"foo4": map[string]interface{}{
								"foo4foo":  "barbar1",
								"foo4foo2": "barbar2Override",
								"foo5foo3": "barbar3Override",
							},
						},
						"secondKey": true,
						"thirdKey":  "thirdValue",
						"otherKey":  "otherValue",
					},
				},
				{
					"path": map[string]interface{}{
						"path":               "app_3",
						"basename":           "app_3",
						"basenameNormalized": "app-3",
						"segments": []string{
							"app_3",
						},
					},
					"extraParams": map[string]interface{}{
						"firstKey": map[string]interface{}{
							"foo":  "bar",
							"foo2": "bar2Override",
							"foo3": true,
							"foo4": map[string]interface{}{
								"foo4foo":  "barbar1",
								"foo4foo2": "barbar2Override",
								"foo5foo3": "barbar3Override",
							},
						},
						"secondKey": true,
						"thirdKey":  "thirdValue",
						"otherKey":  "otherValue",
					},
				},
			},
			expectedError: nil,
		},
	}

	for _, testCase := range cases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			argoCDServiceMock := mocks.Repos{}

			argoCDServiceMock.On("GetDirectories", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(testCaseCopy.repoApps, testCaseCopy.repoError)
			var extraParamFiles []string

			if len(testCaseCopy.extraParamFiles) != 0 {
				// Make sure we process the extra files in the same order
				keys := make([]string, 0, len(testCaseCopy.extraParamFiles))
				for k := range testCaseCopy.extraParamFiles {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				extraParamFiles = make([]string, 0)
				for _, key := range keys {
					extraParamFiles = append(extraParamFiles, key)

					argoCDServiceMock.On("GetFiles", mock.Anything, mock.Anything, mock.Anything, key, mock.Anything, mock.Anything).
						Return(map[string][]byte{
							key: testCaseCopy.extraParamFiles[key],
						}, nil)
				}
			}

			gitGenerator := NewGitGenerator(&argoCDServiceMock, "")
			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					GoTemplate: true,
					Generators: []argoprojiov1alpha1.ApplicationSetGenerator{{
						Git: &argoprojiov1alpha1.GitGenerator{
							RepoURL:             "RepoURL",
							Revision:            "Revision",
							Directories:         testCaseCopy.directories,
							PathParamPrefix:     testCaseCopy.pathParamPrefix,
							ExtraParameterFiles: extraParamFiles,
						},
					}},
				},
			}

			scheme := runtime.NewScheme()
			err := v1alpha1.AddToScheme(scheme)
			require.NoError(t, err)
			appProject := argoprojiov1alpha1.AppProject{}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appProject).Build()

			got, err := gitGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo, client)

			if testCaseCopy.expectedError != nil {
				require.EqualError(t, err, testCaseCopy.expectedError.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, testCaseCopy.expected, got)
			}

			argoCDServiceMock.AssertExpectations(t)
		})
	}
}

func TestGitGenerateParamsFromFiles(t *testing.T) {
	cases := []struct {
		name string
		// files is the list of paths/globs to match
		files []argoprojiov1alpha1.GitFileGeneratorItem
		// repoFileContents maps repo path to the literal contents of that path
		repoFileContents map[string][]byte
		// if repoPathsError is non-nil, the call to GetPaths(...) will return this error value
		repoPathsError  error
		values          map[string]string
		extraParamFiles map[string][]byte
		expected        []map[string]interface{}
		expectedError   error
	}{
		{
			name:  "happy flow: create params from git files",
			files: []argoprojiov1alpha1.GitFileGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.json": []byte(`{
   "cluster": {
       "owner": "john.doe@example.com",
       "name": "production",
       "address": "https://kubernetes.default.svc"
   },
   "key1": "val1",
   "key2": {
       "key2_1": "val2_1",
       "key2_2": {
           "key2_2_1": "val2_2_1"
       }
   },
   "key3": 123
}`),
				"cluster-config/staging/config.json": []byte(`{
   "cluster": {
       "owner": "foo.bar@example.com",
       "name": "staging",
       "address": "https://kubernetes.default.svc"
   }
}`),
			},
			repoPathsError: nil,
			expected: []map[string]interface{}{
				{
					"cluster.owner":           "john.doe@example.com",
					"cluster.name":            "production",
					"cluster.address":         "https://kubernetes.default.svc",
					"key1":                    "val1",
					"key2.key2_1":             "val2_1",
					"key2.key2_2.key2_2_1":    "val2_2_1",
					"key3":                    "123",
					"path":                    "cluster-config/production",
					"path.basename":           "production",
					"path[0]":                 "cluster-config",
					"path[1]":                 "production",
					"path.basenameNormalized": "production",
					"path.filename":           "config.json",
					"path.filenameNormalized": "config.json",
				},
				{
					"cluster.owner":           "foo.bar@example.com",
					"cluster.name":            "staging",
					"cluster.address":         "https://kubernetes.default.svc",
					"path":                    "cluster-config/staging",
					"path.basename":           "staging",
					"path[0]":                 "cluster-config",
					"path[1]":                 "staging",
					"path.basenameNormalized": "staging",
					"path.filename":           "config.json",
					"path.filenameNormalized": "config.json",
				},
			},
			expectedError: nil,
		},
		{
			name:  "Value variable interpolation",
			files: []argoprojiov1alpha1.GitFileGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.json": []byte(`{
   "cluster": {
       "owner": "john.doe@example.com",
       "name": "production",
       "address": "https://kubernetes.default.svc"
   },
   "key1": "val1",
   "key2": {
       "key2_1": "val2_1",
       "key2_2": {
           "key2_2_1": "val2_2_1"
       }
   },
   "key3": 123
}`),
				"cluster-config/staging/config.json": []byte(`{
   "cluster": {
       "owner": "foo.bar@example.com",
       "name": "staging",
       "address": "https://kubernetes.default.svc"
   }
}`),
			},
			repoPathsError: nil,
			values: map[string]string{
				"aaa":   "{{ cluster.owner }}",
				"no-op": "{{ this-does-not-exist }}",
			},
			expected: []map[string]interface{}{
				{
					"cluster.owner":           "john.doe@example.com",
					"cluster.name":            "production",
					"cluster.address":         "https://kubernetes.default.svc",
					"key1":                    "val1",
					"key2.key2_1":             "val2_1",
					"key2.key2_2.key2_2_1":    "val2_2_1",
					"key3":                    "123",
					"path":                    "cluster-config/production",
					"path.basename":           "production",
					"path[0]":                 "cluster-config",
					"path[1]":                 "production",
					"path.basenameNormalized": "production",
					"path.filename":           "config.json",
					"path.filenameNormalized": "config.json",
					"values.aaa":              "john.doe@example.com",
					"values.no-op":            "{{ this-does-not-exist }}",
				},
				{
					"cluster.owner":           "foo.bar@example.com",
					"cluster.name":            "staging",
					"cluster.address":         "https://kubernetes.default.svc",
					"path":                    "cluster-config/staging",
					"path.basename":           "staging",
					"path[0]":                 "cluster-config",
					"path[1]":                 "staging",
					"path.basenameNormalized": "staging",
					"path.filename":           "config.json",
					"path.filenameNormalized": "config.json",
					"values.aaa":              "foo.bar@example.com",
					"values.no-op":            "{{ this-does-not-exist }}",
				},
			},
			expectedError: nil,
		},
		{
			name:             "handles error during getting repo paths",
			files:            []argoprojiov1alpha1.GitFileGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{},
			repoPathsError:   fmt.Errorf("paths error"),
			expected:         []map[string]interface{}{},
			expectedError:    fmt.Errorf("error generating params from git: paths error"),
		},
		{
			name:  "test invalid JSON file returns error",
			files: []argoprojiov1alpha1.GitFileGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.json": []byte(`invalid json file`),
			},
			repoPathsError: nil,
			expected:       []map[string]interface{}{},
			expectedError:  fmt.Errorf("error generating params from git: unable to process file 'cluster-config/production/config.json': unable to parse file: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type map[string]interface {}"),
		},
		{
			name:  "test JSON array",
			files: []argoprojiov1alpha1.GitFileGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.json": []byte(`
[
	{
		"cluster": {
			"owner": "john.doe@example.com",
			"name": "production",
			"address": "https://kubernetes.default.svc",
			"inner": {
				"one" : "two"
			}
		}
	},
	{
		"cluster": {
			"owner": "john.doe@example.com",
			"name": "staging",
			"address": "https://kubernetes.default.svc"
		}
	}
]`),
			},
			repoPathsError: nil,
			expected: []map[string]interface{}{
				{
					"cluster.owner":           "john.doe@example.com",
					"cluster.name":            "production",
					"cluster.address":         "https://kubernetes.default.svc",
					"cluster.inner.one":       "two",
					"path":                    "cluster-config/production",
					"path.basename":           "production",
					"path[0]":                 "cluster-config",
					"path[1]":                 "production",
					"path.basenameNormalized": "production",
					"path.filename":           "config.json",
					"path.filenameNormalized": "config.json",
				},
				{
					"cluster.owner":           "john.doe@example.com",
					"cluster.name":            "staging",
					"cluster.address":         "https://kubernetes.default.svc",
					"path":                    "cluster-config/production",
					"path.basename":           "production",
					"path[0]":                 "cluster-config",
					"path[1]":                 "production",
					"path.basenameNormalized": "production",
					"path.filename":           "config.json",
					"path.filenameNormalized": "config.json",
				},
			},
			expectedError: nil,
		},
		{
			name:  "Test YAML flow",
			files: []argoprojiov1alpha1.GitFileGeneratorItem{{Path: "**/config.yaml"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.yaml": []byte(`
cluster:
  owner: john.doe@example.com
  name: production
  address: https://kubernetes.default.svc
key1: val1
key2:
  key2_1: val2_1
  key2_2:
    key2_2_1: val2_2_1
`),
				"cluster-config/staging/config.yaml": []byte(`
cluster:
  owner: foo.bar@example.com
  name: staging
  address: https://kubernetes.default.svc
`),
			},
			repoPathsError: nil,
			expected: []map[string]interface{}{
				{
					"cluster.owner":           "john.doe@example.com",
					"cluster.name":            "production",
					"cluster.address":         "https://kubernetes.default.svc",
					"key1":                    "val1",
					"key2.key2_1":             "val2_1",
					"key2.key2_2.key2_2_1":    "val2_2_1",
					"path":                    "cluster-config/production",
					"path.basename":           "production",
					"path[0]":                 "cluster-config",
					"path[1]":                 "production",
					"path.basenameNormalized": "production",
					"path.filename":           "config.yaml",
					"path.filenameNormalized": "config.yaml",
				},
				{
					"cluster.owner":           "foo.bar@example.com",
					"cluster.name":            "staging",
					"cluster.address":         "https://kubernetes.default.svc",
					"path":                    "cluster-config/staging",
					"path.basename":           "staging",
					"path[0]":                 "cluster-config",
					"path[1]":                 "staging",
					"path.basenameNormalized": "staging",
					"path.filename":           "config.yaml",
					"path.filenameNormalized": "config.yaml",
				},
			},
			expectedError: nil,
		},
		{
			name:  "test YAML array",
			files: []argoprojiov1alpha1.GitFileGeneratorItem{{Path: "**/config.yaml"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.yaml": []byte(`
- cluster:
    owner: john.doe@example.com
    name: production
    address: https://kubernetes.default.svc
    inner:
      one: two
- cluster:
    owner: john.doe@example.com
    name: staging
    address: https://kubernetes.default.svc`),
			},
			repoPathsError: nil,
			expected: []map[string]interface{}{
				{
					"cluster.owner":           "john.doe@example.com",
					"cluster.name":            "production",
					"cluster.address":         "https://kubernetes.default.svc",
					"cluster.inner.one":       "two",
					"path":                    "cluster-config/production",
					"path.basename":           "production",
					"path[0]":                 "cluster-config",
					"path[1]":                 "production",
					"path.basenameNormalized": "production",
					"path.filename":           "config.yaml",
					"path.filenameNormalized": "config.yaml",
				},
				{
					"cluster.owner":           "john.doe@example.com",
					"cluster.name":            "staging",
					"cluster.address":         "https://kubernetes.default.svc",
					"path":                    "cluster-config/production",
					"path.basename":           "production",
					"path[0]":                 "cluster-config",
					"path[1]":                 "production",
					"path.basenameNormalized": "production",
					"path.filename":           "config.yaml",
					"path.filenameNormalized": "config.yaml",
				},
			},
			expectedError: nil,
		},
		{
			name:  "with extra params",
			files: []argoprojiov1alpha1.GitFileGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.json": []byte(`{
   "cluster": {
       "owner": "john.doe@example.com",
       "name": "production",
       "address": "https://kubernetes.default.svc"
   },
   "key1": "val1",
   "key2": {
       "key2_1": "val2_1",
       "key2_2": {
           "key2_2_1": "val2_2_1"
       }
   },
   "key3": 123
}`),
				"cluster-config/staging/config.json": []byte(`{
   "cluster": {
       "owner": "foo.bar@example.com",
       "name": "staging",
       "address": "https://kubernetes.default.svc"
   }
}`),
			},
			extraParamFiles: map[string][]byte{
				"path/dir/**/*/json": []byte(`{
"firstKey": {
	"foo":  "bar",
	"foo2": "bar2",
	"foo3": true,
	"foo4": {
		"foo4foo":  "barbar1",
		"foo4foo2": "barbar2",
	},
},
"secondKey": false,
"thirdKey":  "thirdValue",
}`),
				"path/otherdir/override.yaml": []byte(`
firstKey: 
  foo2: bar2Override
  foo4:
    foo4foo2: barbar2Override
    foo5foo3: barbar3Override
secondKey: true
otherKey:  otherValue
`),
			},
			repoPathsError: nil,
			expected: []map[string]interface{}{
				{
					"cluster.owner":                      "john.doe@example.com",
					"cluster.name":                       "production",
					"cluster.address":                    "https://kubernetes.default.svc",
					"key1":                               "val1",
					"key2.key2_1":                        "val2_1",
					"key2.key2_2.key2_2_1":               "val2_2_1",
					"key3":                               "123",
					"path":                               "cluster-config/production",
					"path.basename":                      "production",
					"path[0]":                            "cluster-config",
					"path[1]":                            "production",
					"path.basenameNormalized":            "production",
					"path.filename":                      "config.json",
					"path.filenameNormalized":            "config.json",
					"extraParams.firstKey.foo":           "bar",
					"extraParams.firstKey.foo2":          "bar2Override",
					"extraParams.firstKey.foo3":          "true",
					"extraParams.firstKey.foo4.foo4foo":  "barbar1",
					"extraParams.firstKey.foo4.foo4foo2": "barbar2Override",
					"extraParams.firstKey.foo4.foo5foo3": "barbar3Override",
					"extraParams.secondKey":              "true",
					"extraParams.thirdKey":               "thirdValue",
					"extraParams.otherKey":               "otherValue",
				},
				{
					"cluster.owner":                      "foo.bar@example.com",
					"cluster.name":                       "staging",
					"cluster.address":                    "https://kubernetes.default.svc",
					"path":                               "cluster-config/staging",
					"path.basename":                      "staging",
					"path[0]":                            "cluster-config",
					"path[1]":                            "staging",
					"path.basenameNormalized":            "staging",
					"path.filename":                      "config.json",
					"path.filenameNormalized":            "config.json",
					"extraParams.firstKey.foo":           "bar",
					"extraParams.firstKey.foo2":          "bar2Override",
					"extraParams.firstKey.foo3":          "true",
					"extraParams.firstKey.foo4.foo4foo":  "barbar1",
					"extraParams.firstKey.foo4.foo4foo2": "barbar2Override",
					"extraParams.firstKey.foo4.foo5foo3": "barbar3Override",
					"extraParams.secondKey":              "true",
					"extraParams.thirdKey":               "thirdValue",
					"extraParams.otherKey":               "otherValue",
				},
			},
			expectedError: nil,
		},
	}

	for _, testCase := range cases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			argoCDServiceMock := mocks.Repos{}
			argoCDServiceMock.On("GetFiles", mock.Anything, mock.Anything, mock.Anything, testCaseCopy.files[0].Path, mock.Anything, mock.Anything).
				Return(testCaseCopy.repoFileContents, testCaseCopy.repoPathsError)
			var extraParamFiles []string
			if len(testCaseCopy.extraParamFiles) != 0 {
				keys := make([]string, 0, len(testCaseCopy.extraParamFiles))
				for k := range testCaseCopy.extraParamFiles {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				extraParamFiles = make([]string, 0)
				for _, key := range keys {
					extraParamFiles = append(extraParamFiles, key)

					argoCDServiceMock.On("GetFiles", mock.Anything, mock.Anything, mock.Anything, key, mock.Anything, mock.Anything).
						Return(map[string][]byte{
							key: testCaseCopy.extraParamFiles[key],
						}, nil)
				}
			}

			gitGenerator := NewGitGenerator(&argoCDServiceMock, "")
			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					Generators: []argoprojiov1alpha1.ApplicationSetGenerator{{
						Git: &argoprojiov1alpha1.GitGenerator{
							RepoURL:             "RepoURL",
							Revision:            "Revision",
							Files:               testCaseCopy.files,
							Values:              testCaseCopy.values,
							ExtraParameterFiles: extraParamFiles,
						},
					}},
				},
			}

			scheme := runtime.NewScheme()
			err := v1alpha1.AddToScheme(scheme)
			require.NoError(t, err)
			appProject := argoprojiov1alpha1.AppProject{}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appProject).Build()

			got, err := gitGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo, client)
			fmt.Println(got, err)

			if testCaseCopy.expectedError != nil {
				require.EqualError(t, err, testCaseCopy.expectedError.Error())
			} else {
				require.NoError(t, err)
				assert.ElementsMatch(t, testCaseCopy.expected, got)
			}

			argoCDServiceMock.AssertExpectations(t)
		})
	}
}

func TestGitGenerateParamsFromFilesGoTemplate(t *testing.T) {
	cases := []struct {
		name string
		// files is the list of paths/globs to match
		files []argoprojiov1alpha1.GitFileGeneratorItem
		// repoFileContents maps repo path to the literal contents of that path
		repoFileContents map[string][]byte
		// if repoPathsError is non-nil, the call to GetPaths(...) will return this error value
		repoPathsError  error
		extraParamFiles map[string][]byte
		expected        []map[string]interface{}
		expectedError   error
	}{
		{
			name:  "happy flow: create params from git files",
			files: []argoprojiov1alpha1.GitFileGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.json": []byte(`{
   "cluster": {
       "owner": "john.doe@example.com",
       "name": "production",
       "address": "https://kubernetes.default.svc"
   },
   "key1": "val1",
   "key2": {
       "key2_1": "val2_1",
       "key2_2": {
           "key2_2_1": "val2_2_1"
       }
   },
   "key3": 123
}`),
				"cluster-config/staging/config.json": []byte(`{
   "cluster": {
       "owner": "foo.bar@example.com",
       "name": "staging",
       "address": "https://kubernetes.default.svc"
   }
}`),
			},
			repoPathsError: nil,
			expected: []map[string]interface{}{
				{
					"cluster": map[string]interface{}{
						"owner":   "john.doe@example.com",
						"name":    "production",
						"address": "https://kubernetes.default.svc",
					},
					"key1": "val1",
					"key2": map[string]interface{}{
						"key2_1": "val2_1",
						"key2_2": map[string]interface{}{
							"key2_2_1": "val2_2_1",
						},
					},
					"key3": float64(123),
					"path": map[string]interface{}{
						"path":               "cluster-config/production",
						"basename":           "production",
						"filename":           "config.json",
						"basenameNormalized": "production",
						"filenameNormalized": "config.json",
						"segments": []string{
							"cluster-config",
							"production",
						},
					},
				},
				{
					"cluster": map[string]interface{}{
						"owner":   "foo.bar@example.com",
						"name":    "staging",
						"address": "https://kubernetes.default.svc",
					},
					"path": map[string]interface{}{
						"path":               "cluster-config/staging",
						"basename":           "staging",
						"filename":           "config.json",
						"basenameNormalized": "staging",
						"filenameNormalized": "config.json",
						"segments": []string{
							"cluster-config",
							"staging",
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name:             "handles error during getting repo paths",
			files:            []argoprojiov1alpha1.GitFileGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{},
			repoPathsError:   fmt.Errorf("paths error"),
			expected:         []map[string]interface{}{},
			expectedError:    fmt.Errorf("error generating params from git: paths error"),
		},
		{
			name:  "test invalid JSON file returns error",
			files: []argoprojiov1alpha1.GitFileGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.json": []byte(`invalid json file`),
			},
			repoPathsError: nil,
			expected:       []map[string]interface{}{},
			expectedError:  fmt.Errorf("error generating params from git: unable to process file 'cluster-config/production/config.json': unable to parse file: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type map[string]interface {}"),
		},
		{
			name:  "test JSON array",
			files: []argoprojiov1alpha1.GitFileGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.json": []byte(`
[
	{
		"cluster": {
			"owner": "john.doe@example.com",
			"name": "production",
			"address": "https://kubernetes.default.svc",
			"inner": {
				"one" : "two"
			}
		}
	},
	{
		"cluster": {
			"owner": "john.doe@example.com",
			"name": "staging",
			"address": "https://kubernetes.default.svc"
		}
	}
]`),
			},
			repoPathsError: nil,
			expected: []map[string]interface{}{
				{
					"cluster": map[string]interface{}{
						"owner":   "john.doe@example.com",
						"name":    "production",
						"address": "https://kubernetes.default.svc",
						"inner": map[string]interface{}{
							"one": "two",
						},
					},
					"path": map[string]interface{}{
						"path":               "cluster-config/production",
						"basename":           "production",
						"filename":           "config.json",
						"basenameNormalized": "production",
						"filenameNormalized": "config.json",
						"segments": []string{
							"cluster-config",
							"production",
						},
					},
				},
				{
					"cluster": map[string]interface{}{
						"owner":   "john.doe@example.com",
						"name":    "staging",
						"address": "https://kubernetes.default.svc",
					},
					"path": map[string]interface{}{
						"path":               "cluster-config/production",
						"basename":           "production",
						"filename":           "config.json",
						"basenameNormalized": "production",
						"filenameNormalized": "config.json",
						"segments": []string{
							"cluster-config",
							"production",
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name:  "Test YAML flow",
			files: []argoprojiov1alpha1.GitFileGeneratorItem{{Path: "**/config.yaml"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.yaml": []byte(`
cluster:
  owner: john.doe@example.com
  name: production
  address: https://kubernetes.default.svc
key1: val1
key2:
  key2_1: val2_1
  key2_2:
    key2_2_1: val2_2_1
`),
				"cluster-config/staging/config.yaml": []byte(`
cluster:
  owner: foo.bar@example.com
  name: staging
  address: https://kubernetes.default.svc
`),
			},
			repoPathsError: nil,
			expected: []map[string]interface{}{
				{
					"cluster": map[string]interface{}{
						"owner":   "john.doe@example.com",
						"name":    "production",
						"address": "https://kubernetes.default.svc",
					},
					"key1": "val1",
					"key2": map[string]interface{}{
						"key2_1": "val2_1",
						"key2_2": map[string]interface{}{
							"key2_2_1": "val2_2_1",
						},
					},
					"path": map[string]interface{}{
						"path":               "cluster-config/production",
						"basename":           "production",
						"filename":           "config.yaml",
						"basenameNormalized": "production",
						"filenameNormalized": "config.yaml",
						"segments": []string{
							"cluster-config",
							"production",
						},
					},
				},
				{
					"cluster": map[string]interface{}{
						"owner":   "foo.bar@example.com",
						"name":    "staging",
						"address": "https://kubernetes.default.svc",
					},
					"path": map[string]interface{}{
						"path":               "cluster-config/staging",
						"basename":           "staging",
						"filename":           "config.yaml",
						"basenameNormalized": "staging",
						"filenameNormalized": "config.yaml",
						"segments": []string{
							"cluster-config",
							"staging",
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name:  "test YAML array",
			files: []argoprojiov1alpha1.GitFileGeneratorItem{{Path: "**/config.yaml"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.yaml": []byte(`
- cluster:
    owner: john.doe@example.com
    name: production
    address: https://kubernetes.default.svc
    inner:
      one: two
- cluster:
    owner: john.doe@example.com
    name: staging
    address: https://kubernetes.default.svc`),
			},
			repoPathsError: nil,
			expected: []map[string]interface{}{
				{
					"cluster": map[string]interface{}{
						"owner":   "john.doe@example.com",
						"name":    "production",
						"address": "https://kubernetes.default.svc",
						"inner": map[string]interface{}{
							"one": "two",
						},
					},
					"path": map[string]interface{}{
						"path":               "cluster-config/production",
						"basename":           "production",
						"filename":           "config.yaml",
						"basenameNormalized": "production",
						"filenameNormalized": "config.yaml",
						"segments": []string{
							"cluster-config",
							"production",
						},
					},
				},
				{
					"cluster": map[string]interface{}{
						"owner":   "john.doe@example.com",
						"name":    "staging",
						"address": "https://kubernetes.default.svc",
					},
					"path": map[string]interface{}{
						"path":               "cluster-config/production",
						"basename":           "production",
						"filename":           "config.yaml",
						"basenameNormalized": "production",
						"filenameNormalized": "config.yaml",
						"segments": []string{
							"cluster-config",
							"production",
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name:  "with extra params",
			files: []argoprojiov1alpha1.GitFileGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.json": []byte(`{
   "cluster": {
       "owner": "john.doe@example.com",
       "name": "production",
       "address": "https://kubernetes.default.svc"
   },
   "key1": "val1",
   "key2": {
       "key2_1": "val2_1",
       "key2_2": {
           "key2_2_1": "val2_2_1"
       }
   },
   "key3": 123
}`),
				"cluster-config/staging/config.json": []byte(`{
   "cluster": {
       "owner": "foo.bar@example.com",
       "name": "staging",
       "address": "https://kubernetes.default.svc"
   }
}`),
			},
			extraParamFiles: map[string][]byte{
				"path/dir/**/*/json": []byte(`{
"firstKey": {
	"foo":  "bar",
	"foo2": "bar2",
	"foo3": true,
	"foo4": {
		"foo4foo":  "barbar1",
		"foo4foo2": "barbar2",
	},
},
"secondKey": false,
"thirdKey":  "thirdValue",
}`),
				"path/otherdir/override.yaml": []byte(`
firstKey: 
  foo2: bar2Override
  foo4:
    foo4foo2: barbar2Override
    foo5foo3: barbar3Override
secondKey: true
otherKey:  otherValue
`),
			},
			repoPathsError: nil,
			expected: []map[string]interface{}{
				{
					"cluster": map[string]interface{}{
						"owner":   "john.doe@example.com",
						"name":    "production",
						"address": "https://kubernetes.default.svc",
					},
					"key1": "val1",
					"key2": map[string]interface{}{
						"key2_1": "val2_1",
						"key2_2": map[string]interface{}{
							"key2_2_1": "val2_2_1",
						},
					},
					"key3": float64(123),
					"path": map[string]interface{}{
						"path":               "cluster-config/production",
						"basename":           "production",
						"filename":           "config.json",
						"basenameNormalized": "production",
						"filenameNormalized": "config.json",
						"segments": []string{
							"cluster-config",
							"production",
						},
					},
					"extraParams": map[string]interface{}{
						"firstKey": map[string]interface{}{
							"foo":  "bar",
							"foo2": "bar2Override",
							"foo3": true,
							"foo4": map[string]interface{}{
								"foo4foo":  "barbar1",
								"foo4foo2": "barbar2Override",
								"foo5foo3": "barbar3Override",
							},
						},
						"secondKey": true,
						"thirdKey":  "thirdValue",
						"otherKey":  "otherValue",
					},
				},
				{
					"cluster": map[string]interface{}{
						"owner":   "foo.bar@example.com",
						"name":    "staging",
						"address": "https://kubernetes.default.svc",
					},
					"path": map[string]interface{}{
						"path":               "cluster-config/staging",
						"basename":           "staging",
						"filename":           "config.json",
						"basenameNormalized": "staging",
						"filenameNormalized": "config.json",
						"segments": []string{
							"cluster-config",
							"staging",
						},
					},
					"extraParams": map[string]interface{}{
						"firstKey": map[string]interface{}{
							"foo":  "bar",
							"foo2": "bar2Override",
							"foo3": true,
							"foo4": map[string]interface{}{
								"foo4foo":  "barbar1",
								"foo4foo2": "barbar2Override",
								"foo5foo3": "barbar3Override",
							},
						},
						"secondKey": true,
						"thirdKey":  "thirdValue",
						"otherKey":  "otherValue",
					},
				},
			},
			expectedError: nil,
		},
	}

	for _, testCase := range cases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			argoCDServiceMock := mocks.Repos{}
			argoCDServiceMock.On("GetFiles", mock.Anything, mock.Anything, mock.Anything, testCaseCopy.files[0].Path, mock.Anything, mock.Anything).
				Return(testCaseCopy.repoFileContents, testCaseCopy.repoPathsError)

			var extraParamFiles []string
			if len(testCaseCopy.extraParamFiles) != 0 {
				keys := make([]string, 0, len(testCaseCopy.extraParamFiles))
				for k := range testCaseCopy.extraParamFiles {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				extraParamFiles = make([]string, 0)
				for _, key := range keys {
					extraParamFiles = append(extraParamFiles, key)

					argoCDServiceMock.On("GetFiles", mock.Anything, mock.Anything, mock.Anything, key, mock.Anything, mock.Anything).
						Return(map[string][]byte{
							key: testCaseCopy.extraParamFiles[key],
						}, nil)
				}
			}

			gitGenerator := NewGitGenerator(&argoCDServiceMock, "")
			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					GoTemplate: true,
					Generators: []argoprojiov1alpha1.ApplicationSetGenerator{{
						Git: &argoprojiov1alpha1.GitGenerator{
							RepoURL:             "RepoURL",
							Revision:            "Revision",
							Files:               testCaseCopy.files,
							ExtraParameterFiles: extraParamFiles,
						},
					}},
				},
			}

			scheme := runtime.NewScheme()
			err := v1alpha1.AddToScheme(scheme)
			require.NoError(t, err)
			appProject := argoprojiov1alpha1.AppProject{}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appProject).Build()

			got, err := gitGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo, client)
			fmt.Println(got, err)

			if testCaseCopy.expectedError != nil {
				require.EqualError(t, err, testCaseCopy.expectedError.Error())
			} else {
				require.NoError(t, err)
				assert.ElementsMatch(t, testCaseCopy.expected, got)
			}

			argoCDServiceMock.AssertExpectations(t)
		})
	}
}

func TestGitGenerator_GenerateParams(t *testing.T) {
	cases := []struct {
		name               string
		directories        []argoprojiov1alpha1.GitDirectoryGeneratorItem
		pathParamPrefix    string
		repoApps           []string
		repoPathsError     error
		repoFileContents   map[string][]byte
		values             map[string]string
		expected           []map[string]interface{}
		expectedError      error
		appset             argoprojiov1alpha1.ApplicationSet
		callGetDirectories bool
	}{
		{
			name: "Signature Verification - ignores templated project field",
			repoApps: []string{
				"app1",
			},
			repoPathsError: nil,
			appset: argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "set",
					Namespace: "namespace",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					Generators: []argoprojiov1alpha1.ApplicationSetGenerator{{
						Git: &argoprojiov1alpha1.GitGenerator{
							RepoURL:         "RepoURL",
							Revision:        "Revision",
							Directories:     []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
							PathParamPrefix: "",
							Values: map[string]string{
								"foo": "bar",
							},
						},
					}},
					Template: argoprojiov1alpha1.ApplicationSetTemplate{
						Spec: argoprojiov1alpha1.ApplicationSpec{
							Project: "{{.project}}",
						},
					},
				},
			},
			callGetDirectories: true,
			expected:           []map[string]interface{}{{"path": "app1", "path.basename": "app1", "path.basenameNormalized": "app1", "path[0]": "app1", "values.foo": "bar"}},
			expectedError:      nil,
		},
		{
			name: "Signature Verification - Checks for non-templated project field",
			repoApps: []string{
				"app1",
			},
			repoPathsError: nil,
			appset: argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "set",
					Namespace: "namespace",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					Generators: []argoprojiov1alpha1.ApplicationSetGenerator{{
						Git: &argoprojiov1alpha1.GitGenerator{
							RepoURL:         "RepoURL",
							Revision:        "Revision",
							Directories:     []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
							PathParamPrefix: "",
							Values: map[string]string{
								"foo": "bar",
							},
						},
					}},
					Template: argoprojiov1alpha1.ApplicationSetTemplate{
						Spec: argoprojiov1alpha1.ApplicationSpec{
							Project: "project",
						},
					},
				},
			},
			callGetDirectories: false,
			expected:           []map[string]interface{}{{"path": "app1", "path.basename": "app1", "path.basenameNormalized": "app1", "path[0]": "app1", "values.foo": "bar"}},
			expectedError:      fmt.Errorf("error getting project project: appprojects.argoproj.io \"project\" not found"),
		},
	}
	for _, testCase := range cases {
		argoCDServiceMock := mocks.Repos{}

		if testCase.callGetDirectories {
			argoCDServiceMock.On("GetDirectories", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(testCase.repoApps, testCase.repoPathsError)
		}
		gitGenerator := NewGitGenerator(&argoCDServiceMock, "namespace")

		scheme := runtime.NewScheme()
		err := v1alpha1.AddToScheme(scheme)
		require.NoError(t, err)
		appProject := argoprojiov1alpha1.AppProject{}

		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appProject).Build()

		got, err := gitGenerator.GenerateParams(&testCase.appset.Spec.Generators[0], &testCase.appset, client)

		if testCase.expectedError != nil {
			require.EqualError(t, err, testCase.expectedError.Error())
		} else {
			require.NoError(t, err)
			assert.Equal(t, testCase.expected, got)
		}

		argoCDServiceMock.AssertExpectations(t)
	}
}
