package generators

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
)

// type clientSet struct {
// 	RepoServerServiceClient apiclient.RepoServerServiceClient
// }

// func (c *clientSet) NewRepoServerClient() (io.Closer, apiclient.RepoServerServiceClient, error) {
// 	return io.NewCloser(func() error { return nil }), c.RepoServerServiceClient, nil
// }

type argoCDServiceMock struct {
	mock *mock.Mock
}

func (a argoCDServiceMock) GetApps(ctx context.Context, repoURL string, revision string) ([]string, error) {
	args := a.mock.Called(ctx, repoURL, revision)

	return args.Get(0).([]string), args.Error(1)
}

func (a argoCDServiceMock) GetFiles(ctx context.Context, repoURL string, revision string, pattern string) (map[string][]byte, error) {
	args := a.mock.Called(ctx, repoURL, revision, pattern)

	return args.Get(0).(map[string][]byte), args.Error(1)
}

func (a argoCDServiceMock) GetFileContent(ctx context.Context, repoURL string, revision string, path string) ([]byte, error) {
	args := a.mock.Called(ctx, repoURL, revision, path)

	return args.Get(0).([]byte), args.Error(1)
}

func (a argoCDServiceMock) GetDirectories(ctx context.Context, repoURL string, revision string) ([]string, error) {
	args := a.mock.Called(ctx, repoURL, revision)
	return args.Get(0).([]string), args.Error(1)
}

func TestGitGenerateParamsFromDirectories(t *testing.T) {

	cases := []struct {
		name          string
		directories   []argoprojiov1alpha1.GitDirectoryGeneratorItem
		repoApps      []string
		repoError     error
		expected      []map[string]string
		expectedError error
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
			expected: []map[string]string{
				{"path": "app1", "path.basename": "app1", "path.basenameNormalized": "app1"},
				{"path": "app2", "path.basename": "app2", "path.basenameNormalized": "app2"},
				{"path": "app_3", "path.basename": "app_3", "path.basenameNormalized": "app-3"},
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
			expected: []map[string]string{
				{"path": "p1/app2", "path.basename": "app2", "path[0]": "p1", "path.basenameNormalized": "app2"},
				{"path": "p1/p2/app3", "path.basename": "app3", "path[0]": "p1", "path[1]": "p2", "path.basenameNormalized": "app3"},
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
			expected: []map[string]string{
				{"path": "app1", "path.basename": "app1", "path.basenameNormalized": "app1"},
				{"path": "app2", "path.basename": "app2", "path.basenameNormalized": "app2"},
				{"path": "p2/app3", "path.basename": "app3", "path[0]": "p2", "path.basenameNormalized": "app3"},
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
			expected: []map[string]string{
				{"path": "app1", "path.basename": "app1", "path.basenameNormalized": "app1"},
				{"path": "app2", "path.basename": "app2", "path.basenameNormalized": "app2"},
				{"path": "p2/app3", "path.basename": "app3", "path[0]": "p2", "path.basenameNormalized": "app3"},
			},
			expectedError: nil,
		},
		{
			name:          "handles empty response from repo server",
			directories:   []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
			repoApps:      []string{},
			repoError:     nil,
			expected:      []map[string]string{},
			expectedError: nil,
		},
		{
			name:          "handles error from repo server",
			directories:   []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
			repoApps:      []string{},
			repoError:     fmt.Errorf("error"),
			expected:      []map[string]string{},
			expectedError: fmt.Errorf("error"),
		},
	}

	for _, testCase := range cases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			argoCDServiceMock := argoCDServiceMock{mock: &mock.Mock{}}

			argoCDServiceMock.mock.On("GetDirectories", mock.Anything, mock.Anything, mock.Anything).Return(testCaseCopy.repoApps, testCaseCopy.repoError)

			var gitGenerator = NewGitGenerator(argoCDServiceMock)
			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					Generators: []argoprojiov1alpha1.ApplicationSetGenerator{{
						Git: &argoprojiov1alpha1.GitGenerator{
							RepoURL:     "RepoURL",
							Revision:    "Revision",
							Directories: testCaseCopy.directories,
						},
					}},
				},
			}

			got, err := gitGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], nil)

			if testCaseCopy.expectedError != nil {
				assert.EqualError(t, err, testCaseCopy.expectedError.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCaseCopy.expected, got)
			}

			argoCDServiceMock.mock.AssertExpectations(t)
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
		expected       []map[string]string
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
			expected: []map[string]string{
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
					"path.basenameNormalized": "production",
				},
				{
					"cluster.owner":           "foo.bar@example.com",
					"cluster.name":            "staging",
					"cluster.address":         "https://kubernetes.default.svc",
					"path":                    "cluster-config/staging",
					"path.basename":           "staging",
					"path[0]":                 "cluster-config",
					"path.basenameNormalized": "staging",
				},
			},
			expectedError: nil,
		},
		{
			name:             "handles error during getting repo paths",
			files:            []argoprojiov1alpha1.GitFileGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{},
			repoPathsError:   fmt.Errorf("paths error"),
			expected:         []map[string]string{},
			expectedError:    fmt.Errorf("paths error"),
		},
		{
			name:  "test invalid JSON file returns error",
			files: []argoprojiov1alpha1.GitFileGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.json": []byte(`invalid json file`),
			},
			repoPathsError: nil,
			expected:       []map[string]string{},
			expectedError:  fmt.Errorf("unable to process file 'cluster-config/production/config.json': unable to parse file: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type map[string]interface {}"),
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
			expected: []map[string]string{
				{
					"cluster.owner":           "john.doe@example.com",
					"cluster.name":            "production",
					"cluster.address":         "https://kubernetes.default.svc",
					"cluster.inner.one":       "two",
					"path":                    "cluster-config/production",
					"path.basename":           "production",
					"path[0]":                 "cluster-config",
					"path.basenameNormalized": "production",
				},
				{
					"cluster.owner":           "john.doe@example.com",
					"cluster.name":            "staging",
					"cluster.address":         "https://kubernetes.default.svc",
					"path":                    "cluster-config/production",
					"path.basename":           "production",
					"path[0]":                 "cluster-config",
					"path.basenameNormalized": "production",
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
			expected: []map[string]string{
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
					"path.basenameNormalized": "production",
				},
				{
					"cluster.owner":           "foo.bar@example.com",
					"cluster.name":            "staging",
					"cluster.address":         "https://kubernetes.default.svc",
					"path":                    "cluster-config/staging",
					"path.basename":           "staging",
					"path[0]":                 "cluster-config",
					"path.basenameNormalized": "staging",
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
			expected: []map[string]string{
				{
					"cluster.owner":           "john.doe@example.com",
					"cluster.name":            "production",
					"cluster.address":         "https://kubernetes.default.svc",
					"cluster.inner.one":       "two",
					"path":                    "cluster-config/production",
					"path.basename":           "production",
					"path[0]":                 "cluster-config",
					"path.basenameNormalized": "production",
				},
				{
					"cluster.owner":           "john.doe@example.com",
					"cluster.name":            "staging",
					"cluster.address":         "https://kubernetes.default.svc",
					"path":                    "cluster-config/production",
					"path.basename":           "production",
					"path[0]":                 "cluster-config",
					"path.basenameNormalized": "production",
				},
			},
			expectedError: nil,
		},
	}

	for _, testCase := range cases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			argoCDServiceMock := argoCDServiceMock{mock: &mock.Mock{}}
			argoCDServiceMock.mock.On("GetFiles", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(testCaseCopy.repoFileContents, testCaseCopy.repoPathsError)

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
						},
					}},
				},
			}

			got, err := gitGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], nil)
			fmt.Println(got, err)

			if testCaseCopy.expectedError != nil {
				assert.EqualError(t, err, testCaseCopy.expectedError.Error())
			} else {
				assert.NoError(t, err)
				assert.ElementsMatch(t, testCaseCopy.expected, got)
			}

			argoCDServiceMock.mock.AssertExpectations(t)
		})
	}

}
