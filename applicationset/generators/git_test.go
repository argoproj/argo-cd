package generators

import (
	"fmt"
	"testing"

	"github.com/argoproj/argo-cd/v2/applicationset/services/mocks"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, err := (*GitGenerator)(nil).generateParamsFromGitFile(tt.args.filePath, tt.args.fileContent, tt.args.values, tt.args.useGoTemplate, tt.args.goTemplateOptions, tt.args.pathParamPrefix)
			if (err != nil) != tt.wantErr {
				t.Errorf("GitGenerator.generateParamsFromGitFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, params)
		})
	}
}

func TestGitGenerateParamsFromDirectories(t *testing.T) {

	cases := []struct {
		name            string
		directories     []argoprojiov1alpha1.GitGeneratorItem
		pathParamPrefix string
		repoApps        []string
		repoError       error
		values          map[string]string
		expected        []map[string]interface{}
		expectedError   error
	}{
		{
			name:        "happy flow - created apps",
			directories: []argoprojiov1alpha1.GitGeneratorItem{{Path: "*"}},
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
			directories:     []argoprojiov1alpha1.GitGeneratorItem{{Path: "*"}},
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
			directories: []argoprojiov1alpha1.GitGeneratorItem{{Path: "p1/*"}, {Path: "p1/*/*"}},
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
			directories: []argoprojiov1alpha1.GitGeneratorItem{{Path: "p1/*", Exclude: true}, {Path: "*"}, {Path: "*/*"}},
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
			directories: []argoprojiov1alpha1.GitGeneratorItem{{Path: "*"}, {Path: "*/*"}, {Path: "p1/*", Exclude: true}},
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
			directories: []argoprojiov1alpha1.GitGeneratorItem{{Path: "*"}, {Path: "*/*"}},
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
			directories:   []argoprojiov1alpha1.GitGeneratorItem{{Path: "*"}},
			repoApps:      []string{},
			repoError:     nil,
			expected:      []map[string]interface{}{},
			expectedError: nil,
		},
		{
			name:          "handles error from repo server",
			directories:   []argoprojiov1alpha1.GitGeneratorItem{{Path: "*"}},
			repoApps:      []string{},
			repoError:     fmt.Errorf("error"),
			expected:      []map[string]interface{}{},
			expectedError: fmt.Errorf("error generating params from git: error getting directories from repo: error"),
		},
	}

	for _, testCase := range cases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			argoCDServiceMock := mocks.Repos{}

			argoCDServiceMock.On("GetDirectories", mock.Anything, mock.Anything, mock.Anything).Return(testCaseCopy.repoApps, testCaseCopy.repoError)

			var gitGenerator = NewGitGenerator(&argoCDServiceMock)
			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					Generators: []argoprojiov1alpha1.ApplicationSetGenerator{{
						Git: &argoprojiov1alpha1.GitGenerator{
							RepoURL:         "RepoURL",
							Revision:        "Revision",
							Directories:     testCaseCopy.directories,
							PathParamPrefix: testCaseCopy.pathParamPrefix,
							Values:          testCaseCopy.values,
						},
					}},
				},
			}

			got, err := gitGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo)

			if testCaseCopy.expectedError != nil {
				assert.EqualError(t, err, testCaseCopy.expectedError.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCaseCopy.expected, got)
			}

			argoCDServiceMock.AssertExpectations(t)
		})
	}
}

func TestGitGenerateParamsFromDirectoriesGoTemplate(t *testing.T) {

	cases := []struct {
		name            string
		directories     []argoprojiov1alpha1.GitGeneratorItem
		pathParamPrefix string
		repoApps        []string
		repoError       error
		expected        []map[string]interface{}
		expectedError   error
	}{
		{
			name:        "happy flow - created apps",
			directories: []argoprojiov1alpha1.GitGeneratorItem{{Path: "*"}},
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
			directories:     []argoprojiov1alpha1.GitGeneratorItem{{Path: "*"}},
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
			directories: []argoprojiov1alpha1.GitGeneratorItem{{Path: "p1/*"}, {Path: "p1/*/*"}},
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
			directories: []argoprojiov1alpha1.GitGeneratorItem{{Path: "p1/*", Exclude: true}, {Path: "*"}, {Path: "*/*"}},
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
			directories: []argoprojiov1alpha1.GitGeneratorItem{{Path: "*"}, {Path: "*/*"}, {Path: "p1/*", Exclude: true}},
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
			directories:   []argoprojiov1alpha1.GitGeneratorItem{{Path: "*"}},
			repoApps:      []string{},
			repoError:     nil,
			expected:      []map[string]interface{}{},
			expectedError: nil,
		},
		{
			name:          "handles error from repo server",
			directories:   []argoprojiov1alpha1.GitGeneratorItem{{Path: "*"}},
			repoApps:      []string{},
			repoError:     fmt.Errorf("error"),
			expected:      []map[string]interface{}{},
			expectedError: fmt.Errorf("error generating params from git: error getting directories from repo: error"),
		},
	}

	for _, testCase := range cases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			argoCDServiceMock := mocks.Repos{}

			argoCDServiceMock.On("GetDirectories", mock.Anything, mock.Anything, mock.Anything).Return(testCaseCopy.repoApps, testCaseCopy.repoError)

			var gitGenerator = NewGitGenerator(&argoCDServiceMock)
			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					GoTemplate: true,
					Generators: []argoprojiov1alpha1.ApplicationSetGenerator{{
						Git: &argoprojiov1alpha1.GitGenerator{
							RepoURL:         "RepoURL",
							Revision:        "Revision",
							Directories:     testCaseCopy.directories,
							PathParamPrefix: testCaseCopy.pathParamPrefix,
						},
					}},
				},
			}

			got, err := gitGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo)

			if testCaseCopy.expectedError != nil {
				assert.EqualError(t, err, testCaseCopy.expectedError.Error())
			} else {
				assert.NoError(t, err)
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
		files []argoprojiov1alpha1.GitGeneratorItem
		// repoFileContents maps repo path to the literal contents of that path
		repoFileContents map[string][]byte
		// if repoPathsError is non-nil, the call to GetPaths(...) will return this error value
		repoPathsError error
		values         map[string]string
		expected       []map[string]interface{}
		expectedError  error
	}{
		{
			name:  "happy flow: create params from git files",
			files: []argoprojiov1alpha1.GitGeneratorItem{{Path: "**/config.json"}},
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
			files: []argoprojiov1alpha1.GitGeneratorItem{{Path: "**/config.json"}},
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
			files:            []argoprojiov1alpha1.GitGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{},
			repoPathsError:   fmt.Errorf("paths error"),
			expected:         []map[string]interface{}{},
			expectedError:    fmt.Errorf("error generating params from git: paths error"),
		},
		{
			name:  "test invalid JSON file returns error",
			files: []argoprojiov1alpha1.GitGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.json": []byte(`invalid json file`),
			},
			repoPathsError: nil,
			expected:       []map[string]interface{}{},
			expectedError:  fmt.Errorf("error generating params from git: unable to process file 'cluster-config/production/config.json': unable to parse file: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type map[string]interface {}"),
		},
		{
			name:  "test JSON array",
			files: []argoprojiov1alpha1.GitGeneratorItem{{Path: "**/config.json"}},
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
			files: []argoprojiov1alpha1.GitGeneratorItem{{Path: "**/config.yaml"}},
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
			files: []argoprojiov1alpha1.GitGeneratorItem{{Path: "**/config.yaml"}},
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
			name:  "filter files according to file-path with exclude",
			files: []argoprojiov1alpha1.GitGeneratorItem{{Path: "**/config.json"}, {Path: "p1/**/config.json", Exclude: true}},
			repoFileContents: map[string][]byte{
				"app1/config.json":    []byte(`{}`),
				"app2/config.json":    []byte(`{}`),
				"p1/app2/config.json": []byte(`{}`),
				"p1/app3/config.json": []byte(`{}`),
				"p2/app3/config.json": []byte(`{}`),
			},
			repoPathsError: nil,
			expected: []map[string]interface{}{
				{
					"path":                    "app1",
					"path.basename":           "app1",
					"path.basenameNormalized": "app1",
					"path.filename":           "config.json",
					"path.filenameNormalized": "config.json",
					"path[0]":                 "app1",
				},
				{
					"path":                    "app2",
					"path.basename":           "app2",
					"path.basenameNormalized": "app2",
					"path.filename":           "config.json",
					"path.filenameNormalized": "config.json",
					"path[0]":                 "app2",
				},
				{
					"path":                    "p2/app3",
					"path.basename":           "app3",
					"path.basenameNormalized": "app3",
					"path.filename":           "config.json",
					"path.filenameNormalized": "config.json",
					"path[0]":                 "p2",
					"path[1]":                 "app3",
				},
			},
			expectedError: nil,
		},
		{
			name:  "filter files according to a directory-Path with exclude",
			files: []argoprojiov1alpha1.GitGeneratorItem{{Path: "**/config.json"}, {Path: "p1/**", Exclude: true}},
			repoFileContents: map[string][]byte{
				"app1/config.json":    []byte(`{}`),
				"app2/config.json":    []byte(`{}`),
				"p1/app2/config.json": []byte(`{}`),
				"p1/app3/config.json": []byte(`{}`),
				"p2/app3/config.json": []byte(`{}`),
			},
			repoPathsError: nil,
			expected: []map[string]interface{}{
				{
					"path":                    "app1",
					"path.basename":           "app1",
					"path.basenameNormalized": "app1",
					"path.filename":           "config.json",
					"path.filenameNormalized": "config.json",
					"path[0]":                 "app1",
				},
				{
					"path":                    "app2",
					"path.basename":           "app2",
					"path.basenameNormalized": "app2",
					"path.filename":           "config.json",
					"path.filenameNormalized": "config.json",
					"path[0]":                 "app2",
				},
				{
					"path":                    "p2/app3",
					"path.basename":           "app3",
					"path.basenameNormalized": "app3",
					"path.filename":           "config.json",
					"path.filenameNormalized": "config.json",
					"path[0]":                 "p2",
					"path[1]":                 "app3",
				},
			},
			expectedError: nil,
		},
		{
			name: "filter files according to multiple file-paths with exclude",
			files: []argoprojiov1alpha1.GitGeneratorItem{
				{Path: "**/config.json"},
				{Path: "p1/app2/config.json", Exclude: true},
				{Path: "p1/app3/config.json", Exclude: true},
			},
			repoFileContents: map[string][]byte{
				"app1/config.json":    []byte(`{}`),
				"app2/config.json":    []byte(`{}`),
				"p1/app2/config.json": []byte(`{}`),
				"p1/app3/config.json": []byte(`{}`),
				"p2/app3/config.json": []byte(`{}`),
			},
			repoPathsError: nil,
			expected: []map[string]interface{}{
				{
					"path":                    "app1",
					"path.basename":           "app1",
					"path.basenameNormalized": "app1",
					"path.filename":           "config.json",
					"path.filenameNormalized": "config.json",
					"path[0]":                 "app1",
				},
				{
					"path":                    "app2",
					"path.basename":           "app2",
					"path.basenameNormalized": "app2",
					"path.filename":           "config.json",
					"path.filenameNormalized": "config.json",
					"path[0]":                 "app2",
				},
				{
					"path":                    "p2/app3",
					"path.basename":           "app3",
					"path.basenameNormalized": "app3",
					"path.filename":           "config.json",
					"path.filenameNormalized": "config.json",
					"path[0]":                 "p2",
					"path[1]":                 "app3",
				},
			},
			expectedError: nil,
		},
		{
			name:  "test compatibility with greedy git file generator globbing and exclude filter",
			files: []argoprojiov1alpha1.GitGeneratorItem{{Path: "some-path/*.yaml"}},
			repoFileContents: map[string][]byte{
				"some-path/values.yaml": []byte(`
cluster:
  owner: john.doe@example.com
  name: production
  address: https://kubernetes.default.svc
`),
				"some-path/staging/values.yaml": []byte(`
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
					"path":                    "some-path",
					"path.basename":           "some-path",
					"path[0]":                 "some-path",
					"path.basenameNormalized": "some-path",
					"path.filename":           "values.yaml",
					"path.filenameNormalized": "values.yaml",
				},
				{
					"cluster.owner":           "foo.bar@example.com",
					"cluster.name":            "staging",
					"cluster.address":         "https://kubernetes.default.svc",
					"path":                    "some-path/staging",
					"path.basename":           "staging",
					"path[0]":                 "some-path",
					"path[1]":                 "staging",
					"path.basenameNormalized": "staging",
					"path.filename":           "values.yaml",
					"path.filenameNormalized": "values.yaml",
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
			argoCDServiceMock.On("GetFiles", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(testCaseCopy.repoFileContents, testCaseCopy.repoPathsError)

			var gitGenerator = NewGitGenerator(&argoCDServiceMock)
			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					Generators: []argoprojiov1alpha1.ApplicationSetGenerator{{
						Git: &argoprojiov1alpha1.GitGenerator{
							RepoURL:  "RepoURL",
							Revision: "Revision",
							Files:    testCaseCopy.files,
							Values:   testCaseCopy.values,
						},
					}},
				},
			}

			got, err := gitGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo)
			fmt.Println(got, err)

			if testCaseCopy.expectedError != nil {
				assert.EqualError(t, err, testCaseCopy.expectedError.Error())
			} else {
				assert.NoError(t, err)
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
		files []argoprojiov1alpha1.GitGeneratorItem
		// repoFileContents maps repo path to the literal contents of that path
		repoFileContents map[string][]byte
		// if repoPathsError is non-nil, the call to GetPaths(...) will return this error value
		repoPathsError error
		expected       []map[string]interface{}
		expectedError  error
	}{
		{
			name:  "happy flow: create params from git files",
			files: []argoprojiov1alpha1.GitGeneratorItem{{Path: "**/config.json"}},
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
			files:            []argoprojiov1alpha1.GitGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{},
			repoPathsError:   fmt.Errorf("paths error"),
			expected:         []map[string]interface{}{},
			expectedError:    fmt.Errorf("error generating params from git: paths error"),
		},
		{
			name:  "test invalid JSON file returns error",
			files: []argoprojiov1alpha1.GitGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.json": []byte(`invalid json file`),
			},
			repoPathsError: nil,
			expected:       []map[string]interface{}{},
			expectedError:  fmt.Errorf("error generating params from git: unable to process file 'cluster-config/production/config.json': unable to parse file: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type map[string]interface {}"),
		},
		{
			name:  "test JSON array",
			files: []argoprojiov1alpha1.GitGeneratorItem{{Path: "**/config.json"}},
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
			files: []argoprojiov1alpha1.GitGeneratorItem{{Path: "**/config.yaml"}},
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
			files: []argoprojiov1alpha1.GitGeneratorItem{{Path: "**/config.yaml"}},
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
			name:  "filter files according to file-path with exclude",
			files: []argoprojiov1alpha1.GitGeneratorItem{{Path: "**/config.json"}, {Path: "cluster-config/dev/config.json", Exclude: true}},
			repoFileContents: map[string][]byte{
				"cluster-config/dev/config.json": []byte(`{
   "cluster": {
       "owner": "john.doe@example.com",
       "name": "dev",
       "address": "https://kubernetes.default.svc"
   }
}
`),
				"cluster-config/production/config.json": []byte(`{
   "cluster": {
       "owner": "john.doe@example.com",
       "name": "production",
       "address": "https://kubernetes.default.svc"
   }
}
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
	}

	for _, testCase := range cases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			argoCDServiceMock := mocks.Repos{}
			argoCDServiceMock.On("GetFiles", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(testCaseCopy.repoFileContents, testCaseCopy.repoPathsError)

			var gitGenerator = NewGitGenerator(&argoCDServiceMock)
			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					GoTemplate: true,
					Generators: []argoprojiov1alpha1.ApplicationSetGenerator{{
						Git: &argoprojiov1alpha1.GitGenerator{
							RepoURL:  "RepoURL",
							Revision: "Revision",
							Files:    testCaseCopy.files,
						},
					}},
				},
			}

			got, err := gitGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo)
			fmt.Println(got, err)

			if testCaseCopy.expectedError != nil {
				assert.EqualError(t, err, testCaseCopy.expectedError.Error())
			} else {
				assert.NoError(t, err)
				assert.ElementsMatch(t, testCaseCopy.expected, got)
			}

			argoCDServiceMock.AssertExpectations(t)
		})
	}
}
