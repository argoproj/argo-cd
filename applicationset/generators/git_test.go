package generators

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/applicationset/services/mocks"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func Test_generateParamsFromGitFile(t *testing.T) {
	values := map[string]string{}
	params, err := (*GitGenerator)(nil).generateParamsFromGitFile("0123456789abcdef", "path/dir/file_name.yaml", []byte(`
foo:
  bar: baz
`), values, false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []map[string]interface{}{
		{
			"commitSHA":               "0123456789abcdef",
			"foo.bar":                 "baz",
			"path":                    "path/dir",
			"path.basename":           "dir",
			"path.filename":           "file_name.yaml",
			"path.basenameNormalized": "dir",
			"path.filenameNormalized": "file-name.yaml",
			"path[0]":                 "path",
			"path[1]":                 "dir",
		},
	}, params)
}

func Test_generatePrefixedParamsFromGitFile(t *testing.T) {
	values := map[string]string{}
	params, err := (*GitGenerator)(nil).generateParamsFromGitFile("0123456789abcdef", "path/dir/file_name.yaml", []byte(`
foo:
  bar: baz
`), values, false, nil, "myRepo")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []map[string]interface{}{
		{
			"foo.bar":                        "baz",
			"myRepo.commitSHA":               "0123456789abcdef",
			"myRepo.path":                    "path/dir",
			"myRepo.path.basename":           "dir",
			"myRepo.path.filename":           "file_name.yaml",
			"myRepo.path.basenameNormalized": "dir",
			"myRepo.path.filenameNormalized": "file-name.yaml",
			"myRepo.path[0]":                 "path",
			"myRepo.path[1]":                 "dir",
		},
	}, params)
}

func Test_generateParamsFromGitFileGoTemplate(t *testing.T) {
	values := map[string]string{}
	params, err := (*GitGenerator)(nil).generateParamsFromGitFile("0123456789abcdef", "path/dir/file_name.yaml", []byte(`
foo:
  bar: baz
`), values, true, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []map[string]interface{}{
		{
			"commitSHA": "0123456789abcdef",
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
	}, params)
}

func Test_generatePrefixedParamsFromGitFileGoTemplate(t *testing.T) {
	values := map[string]string{}
	params, err := (*GitGenerator)(nil).generateParamsFromGitFile("0123456789abcdef", "path/dir/file_name.yaml", []byte(`
foo:
  bar: baz
`), values, true, nil, "myRepo")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []map[string]interface{}{
		{
			"foo": map[string]interface{}{
				"bar": "baz",
			},
			"myRepo": map[string]interface{}{
				"commitSHA": "0123456789abcdef",
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
	}, params)
}

func TestGitGenerateParamsFromDirectories(t *testing.T) {

	cases := []struct {
		name            string
		directories     []argoprojiov1alpha1.GitDirectoryGeneratorItem
		pathParamPrefix string
		repoApps        []string
		repoError       error
		values          map[string]string
		expected        []map[string]interface{}
		expectedError   error
		commitSHA       string
		commitError     error
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
			repoError:   nil,
			commitSHA:   "0123456789abcdef",
			commitError: nil,
			expected: []map[string]interface{}{
				{"commitSHA": "0123456789abcdef", "path": "app1", "path.basename": "app1", "path.basenameNormalized": "app1", "path[0]": "app1"},
				{"commitSHA": "0123456789abcdef", "path": "app2", "path.basename": "app2", "path.basenameNormalized": "app2", "path[0]": "app2"},
				{"commitSHA": "0123456789abcdef", "path": "app_3", "path.basename": "app_3", "path.basenameNormalized": "app-3", "path[0]": "app_3"},
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
			repoError:   nil,
			commitSHA:   "0123456789abcdef",
			commitError: nil,
			expected: []map[string]interface{}{
				{"myRepo.commitSHA": "0123456789abcdef", "myRepo.path": "app1", "myRepo.path.basename": "app1", "myRepo.path.basenameNormalized": "app1", "myRepo.path[0]": "app1"},
				{"myRepo.commitSHA": "0123456789abcdef", "myRepo.path": "app2", "myRepo.path.basename": "app2", "myRepo.path.basenameNormalized": "app2", "myRepo.path[0]": "app2"},
				{"myRepo.commitSHA": "0123456789abcdef", "myRepo.path": "app_3", "myRepo.path.basename": "app_3", "myRepo.path.basenameNormalized": "app-3", "myRepo.path[0]": "app_3"},
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
			repoError:   nil,
			commitSHA:   "0123456789abcdef",
			commitError: nil,
			expected: []map[string]interface{}{
				{"commitSHA": "0123456789abcdef", "path": "p1/app2", "path.basename": "app2", "path[0]": "p1", "path[1]": "app2", "path.basenameNormalized": "app2"},
				{"commitSHA": "0123456789abcdef", "path": "p1/p2/app3", "path.basename": "app3", "path[0]": "p1", "path[1]": "p2", "path[2]": "app3", "path.basenameNormalized": "app3"},
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
			repoError:   nil,
			commitSHA:   "0123456789abcdef",
			commitError: nil,
			expected: []map[string]interface{}{
				{"commitSHA": "0123456789abcdef", "path": "app1", "path.basename": "app1", "path[0]": "app1", "path.basenameNormalized": "app1"},
				{"commitSHA": "0123456789abcdef", "path": "app2", "path.basename": "app2", "path[0]": "app2", "path.basenameNormalized": "app2"},
				{"commitSHA": "0123456789abcdef", "path": "p2/app3", "path.basename": "app3", "path[0]": "p2", "path[1]": "app3", "path.basenameNormalized": "app3"},
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
			repoError:   nil,
			commitSHA:   "0123456789abcdef",
			commitError: nil,
			expected: []map[string]interface{}{
				{"commitSHA": "0123456789abcdef", "path": "app1", "path.basename": "app1", "path[0]": "app1", "path.basenameNormalized": "app1"},
				{"commitSHA": "0123456789abcdef", "path": "app2", "path.basename": "app2", "path[0]": "app2", "path.basenameNormalized": "app2"},
				{"commitSHA": "0123456789abcdef", "path": "p2/app3", "path.basename": "app3", "path[0]": "p2", "path[1]": "app3", "path.basenameNormalized": "app3"},
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
			repoError:   nil,
			commitSHA:   "0123456789abcdef",
			commitError: nil,
			values: map[string]string{
				"foo":   "bar",
				"aaa":   "{{ path[0] }}",
				"no-op": "{{ this-does-not-exist }}",
			},
			expected: []map[string]interface{}{
				{"commitSHA": "0123456789abcdef", "values.foo": "bar", "values.no-op": "{{ this-does-not-exist }}", "values.aaa": "app1", "path": "app1", "path.basename": "app1", "path[0]": "app1", "path.basenameNormalized": "app1"},
				{"commitSHA": "0123456789abcdef", "values.foo": "bar", "values.no-op": "{{ this-does-not-exist }}", "values.aaa": "p1", "path": "p1/app2", "path.basename": "app2", "path[0]": "p1", "path[1]": "app2", "path.basenameNormalized": "app2"},
			},
			expectedError: nil,
		},
		{
			name:          "handles empty response from repo server",
			directories:   []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
			repoApps:      []string{},
			repoError:     nil,
			commitSHA:     "0123456789abcdef",
			commitError:   nil,
			expected:      []map[string]interface{}{},
			expectedError: nil,
		},
		{
			name:          "handles error from repo server",
			directories:   []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
			repoApps:      []string{},
			repoError:     fmt.Errorf("error"),
			commitSHA:     "0123456789abcdef",
			commitError:   nil,
			expected:      []map[string]interface{}{},
			expectedError: fmt.Errorf("error generating params from git: error getting directories from repo: error"),
		},
	}

	for _, testCase := range cases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			argoCDServiceMock := &mocks.Repos{}

			argoCDServiceMock.On("GetDirectories", mock.Anything, mock.Anything, mock.Anything).Return(testCaseCopy.repoApps, testCaseCopy.repoError)

			argoCDServiceMock.On("CommitSHA", mock.Anything, mock.Anything, mock.Anything).Return(testCaseCopy.commitSHA, testCaseCopy.commitError)

			var gitGenerator = NewGitGenerator(argoCDServiceMock)
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
		directories     []argoprojiov1alpha1.GitDirectoryGeneratorItem
		pathParamPrefix string
		repoApps        []string
		repoError       error
		expected        []map[string]interface{}
		expectedError   error
		commitSHA       string
		commitError     error
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
			repoError:   nil,
			commitSHA:   "0123456789abcdef",
			commitError: nil,
			expected: []map[string]interface{}{
				{
					"commitSHA": "0123456789abcdef",
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
					"commitSHA": "0123456789abcdef",
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
					"commitSHA": "0123456789abcdef",
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
			repoError:   nil,
			commitSHA:   "0123456789abcdef",
			commitError: nil,
			expected: []map[string]interface{}{
				{
					"myRepo": map[string]interface{}{
						"commitSHA": "0123456789abcdef",
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
						"commitSHA": "0123456789abcdef",
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
						"commitSHA": "0123456789abcdef",
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
			repoError:   nil,
			commitSHA:   "0123456789abcdef",
			commitError: nil,
			expected: []map[string]interface{}{
				{
					"commitSHA": "0123456789abcdef",
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
					"commitSHA": "0123456789abcdef",
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
			repoError:   nil,
			commitSHA:   "0123456789abcdef",
			commitError: nil,
			expected: []map[string]interface{}{
				{
					"commitSHA": "0123456789abcdef",
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
					"commitSHA": "0123456789abcdef",
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
					"commitSHA": "0123456789abcdef",
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
			repoError:   nil,
			commitSHA:   "0123456789abcdef",
			commitError: nil,
			expected: []map[string]interface{}{
				{
					"commitSHA": "0123456789abcdef",
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
					"commitSHA": "0123456789abcdef",
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
					"commitSHA": "0123456789abcdef",
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
			commitSHA:     "0123456789abcdef",
			commitError:   nil,
			expected:      []map[string]interface{}{},
			expectedError: nil,
		},
		{
			name:          "handles error from repo server",
			directories:   []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
			repoApps:      []string{},
			repoError:     fmt.Errorf("error"),
			commitSHA:     "0123456789abcdef",
			commitError:   nil,
			expected:      []map[string]interface{}{},
			expectedError: fmt.Errorf("error generating params from git: error getting directories from repo: error"),
		},
	}

	for _, testCase := range cases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			argoCDServiceMock := &mocks.Repos{}

			argoCDServiceMock.On("GetDirectories", mock.Anything, mock.Anything, mock.Anything).Return(testCaseCopy.repoApps, testCaseCopy.repoError)

			argoCDServiceMock.On("CommitSHA", mock.Anything, mock.Anything, mock.Anything).Return(testCaseCopy.commitSHA, testCaseCopy.commitError)

			var gitGenerator = NewGitGenerator(argoCDServiceMock)
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
		files []argoprojiov1alpha1.GitFileGeneratorItem
		// repoFileContents maps repo path to the literal contents of that path
		repoFileContents map[string][]byte
		// if repoPathsError is non-nil, the call to GetPaths(...) will return this error value
		repoPathsError error
		values         map[string]string
		commitSHA      string
		commitError    error
		expected       []map[string]interface{}
		expectedError  error
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
			commitSHA:      "0123456789abcdef",
			commitError:    nil,
			expected: []map[string]interface{}{
				{
					"commitSHA":               "0123456789abcdef",
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
					"commitSHA":               "0123456789abcdef",
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
			commitSHA:      "0123456789abcdef",
			commitError:    nil,
			values: map[string]string{
				"aaa":   "{{ cluster.owner }}",
				"no-op": "{{ this-does-not-exist }}",
			},
			expected: []map[string]interface{}{
				{
					"commitSHA":               "0123456789abcdef",
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
					"commitSHA":               "0123456789abcdef",
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
			commitSHA:        "0123456789abcdef",
			commitError:      nil,
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
			commitSHA:      "0123456789abcdef",
			commitError:    nil,
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
			commitSHA:      "0123456789abcdef",
			commitError:    nil,
			expected: []map[string]interface{}{
				{
					"cluster.owner":           "john.doe@example.com",
					"cluster.name":            "production",
					"cluster.address":         "https://kubernetes.default.svc",
					"cluster.inner.one":       "two",
					"commitSHA":               "0123456789abcdef",
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
					"commitSHA":               "0123456789abcdef",
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
			commitSHA:      "0123456789abcdef",
			commitError:    nil,
			expected: []map[string]interface{}{
				{
					"cluster.owner":           "john.doe@example.com",
					"cluster.name":            "production",
					"cluster.address":         "https://kubernetes.default.svc",
					"commitSHA":               "0123456789abcdef",
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
					"commitSHA":               "0123456789abcdef",
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
			commitSHA:      "0123456789abcdef",
			commitError:    nil,
			expected: []map[string]interface{}{
				{
					"cluster.owner":           "john.doe@example.com",
					"cluster.name":            "production",
					"cluster.address":         "https://kubernetes.default.svc",
					"cluster.inner.one":       "two",
					"commitSHA":               "0123456789abcdef",
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
					"commitSHA":               "0123456789abcdef",
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
	}

	for _, testCase := range cases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			argoCDServiceMock := &mocks.Repos{}
			argoCDServiceMock.On("GetFiles", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(testCaseCopy.repoFileContents, testCaseCopy.repoPathsError)
			argoCDServiceMock.On("CommitSHA", mock.Anything, mock.Anything, mock.Anything).Return(testCaseCopy.commitSHA, testCaseCopy.commitError)

			var gitGenerator = NewGitGenerator(argoCDServiceMock)
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
		files []argoprojiov1alpha1.GitFileGeneratorItem
		// repoFileContents maps repo path to the literal contents of that path
		repoFileContents map[string][]byte
		// if repoPathsError is non-nil, the call to GetPaths(...) will return this error value
		repoPathsError error
		commitSHA      string
		commitError    error
		expected       []map[string]interface{}
		expectedError  error
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
			commitSHA:      "0123456789abcdef",
			commitError:    nil,
			expected: []map[string]interface{}{
				{
					"cluster": map[string]interface{}{
						"owner":   "john.doe@example.com",
						"name":    "production",
						"address": "https://kubernetes.default.svc",
					},
					"commitSHA": "0123456789abcdef",
					"key1":      "val1",
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
					"commitSHA": "0123456789abcdef",
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
			commitSHA:        "0123456789abcdef",
			commitError:      nil,
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
			commitSHA:      "0123456789abcdef",
			commitError:    nil,
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
			commitSHA:      "0123456789abcdef",
			commitError:    nil,
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
					"commitSHA": "0123456789abcdef",
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
					"commitSHA": "0123456789abcdef",
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
			commitSHA:      "0123456789abcdef",
			commitError:    nil,
			expected: []map[string]interface{}{
				{
					"cluster": map[string]interface{}{
						"owner":   "john.doe@example.com",
						"name":    "production",
						"address": "https://kubernetes.default.svc",
					},
					"commitSHA": "0123456789abcdef",
					"key1":      "val1",
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
					"commitSHA": "0123456789abcdef",
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
			commitSHA:      "0123456789abcdef",
			commitError:    nil,
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
					"commitSHA": "0123456789abcdef",
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
					"commitSHA": "0123456789abcdef",
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
	}

	for _, testCase := range cases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			argoCDServiceMock := &mocks.Repos{}
			argoCDServiceMock.On("GetFiles", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(testCaseCopy.repoFileContents, testCaseCopy.repoPathsError)
			argoCDServiceMock.On("CommitSHA", mock.Anything, mock.Anything, mock.Anything).Return(testCaseCopy.commitSHA, testCaseCopy.commitError)

			var gitGenerator = NewGitGenerator(argoCDServiceMock)
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
