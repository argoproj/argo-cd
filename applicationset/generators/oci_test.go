package generators

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj/argo-cd/v3/applicationset/services/mocks"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestOciGenerateParamsFromDirectories(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		directories     []v1alpha1.OciDirectoryGeneratorItem
		pathParamPrefix string
		repoApps        []string
		repoError       error
		values          map[string]string
		expected        []map[string]any
		expectedError   error
	}{
		{
			name:        "happy flow - created apps",
			directories: []v1alpha1.OciDirectoryGeneratorItem{{Path: "*"}},
			repoApps: []string{
				"app1",
				"app2",
				"app_3",
				"p1/app4",
			},
			expected: []map[string]any{
				{"path": "app1", "path.basename": "app1", "path.basenameNormalized": "app1", "path[0]": "app1"},
				{"path": "app2", "path.basename": "app2", "path.basenameNormalized": "app2", "path[0]": "app2"},
				{"path": "app_3", "path.basename": "app_3", "path.basenameNormalized": "app-3", "path[0]": "app_3"},
			},
			expectedError: nil,
		},
		{
			name:            "It prefixes path parameters with PathParamPrefix",
			directories:     []v1alpha1.OciDirectoryGeneratorItem{{Path: "*"}},
			pathParamPrefix: "myRepo",
			repoApps: []string{
				"app1",
				"app2",
				"app_3",
				"p1/app4",
			},
			repoError: nil,
			expected: []map[string]any{
				{"myRepo.path": "app1", "myRepo.path.basename": "app1", "myRepo.path.basenameNormalized": "app1", "myRepo.path[0]": "app1"},
				{"myRepo.path": "app2", "myRepo.path.basename": "app2", "myRepo.path.basenameNormalized": "app2", "myRepo.path[0]": "app2"},
				{"myRepo.path": "app_3", "myRepo.path.basename": "app_3", "myRepo.path.basenameNormalized": "app-3", "myRepo.path[0]": "app_3"},
			},
			expectedError: nil,
		},
		{
			name:        "It filters application according to the paths",
			directories: []v1alpha1.OciDirectoryGeneratorItem{{Path: "p1/*"}, {Path: "p1/*/*"}},
			repoApps: []string{
				"app1",
				"p1/app2",
				"p1/p2/app3",
				"p1/p2/p3/app4",
			},
			expected: []map[string]any{
				{"path": "p1/app2", "path.basename": "app2", "path[0]": "p1", "path[1]": "app2", "path.basenameNormalized": "app2"},
				{"path": "p1/p2/app3", "path.basename": "app3", "path[0]": "p1", "path[1]": "p2", "path[2]": "app3", "path.basenameNormalized": "app3"},
			},
			expectedError: nil,
		},
		{
			name:        "It filters application according to the paths with Exclude",
			directories: []v1alpha1.OciDirectoryGeneratorItem{{Path: "p1/*", Exclude: true}, {Path: "*"}, {Path: "*/*"}},
			repoApps: []string{
				"app1",
				"app2",
				"p1/app2",
				"p1/app3",
				"p2/app3",
			},
			repoError: nil,
			expected: []map[string]any{
				{"path": "app1", "path.basename": "app1", "path[0]": "app1", "path.basenameNormalized": "app1"},
				{"path": "app2", "path.basename": "app2", "path[0]": "app2", "path.basenameNormalized": "app2"},
				{"path": "p2/app3", "path.basename": "app3", "path[0]": "p2", "path[1]": "app3", "path.basenameNormalized": "app3"},
			},
			expectedError: nil,
		},
		{
			name:        "Expecting same exclude behavior with different order",
			directories: []v1alpha1.OciDirectoryGeneratorItem{{Path: "*"}, {Path: "*/*"}, {Path: "p1/*", Exclude: true}},
			repoApps: []string{
				"app1",
				"app2",
				"p1/app2",
				"p1/app3",
				"p2/app3",
			},
			repoError: nil,
			expected: []map[string]any{
				{"path": "app1", "path.basename": "app1", "path[0]": "app1", "path.basenameNormalized": "app1"},
				{"path": "app2", "path.basename": "app2", "path[0]": "app2", "path.basenameNormalized": "app2"},
				{"path": "p2/app3", "path.basename": "app3", "path[0]": "p2", "path[1]": "app3", "path.basenameNormalized": "app3"},
			},
			expectedError: nil,
		},
		{
			name:        "Value variable interpolation",
			directories: []v1alpha1.OciDirectoryGeneratorItem{{Path: "*"}, {Path: "*/*"}},
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
			expected: []map[string]any{
				{"values.foo": "bar", "values.no-op": "{{ this-does-not-exist }}", "values.aaa": "app1", "path": "app1", "path.basename": "app1", "path[0]": "app1", "path.basenameNormalized": "app1"},
				{"values.foo": "bar", "values.no-op": "{{ this-does-not-exist }}", "values.aaa": "p1", "path": "p1/app2", "path.basename": "app2", "path[0]": "p1", "path[1]": "app2", "path.basenameNormalized": "app2"},
			},
			expectedError: nil,
		},
		{
			name:          "handles empty response from repo server",
			directories:   []v1alpha1.OciDirectoryGeneratorItem{{Path: "*"}},
			repoApps:      []string{},
			repoError:     nil,
			expected:      []map[string]any{},
			expectedError: nil,
		},
		{
			name:          "handles error from repo server",
			directories:   []v1alpha1.OciDirectoryGeneratorItem{{Path: "*"}},
			repoApps:      []string{},
			repoError:     errors.New("error"),
			expected:      []map[string]any{},
			expectedError: errors.New("error generating params from oci: error getting directories from oci: error"),
		},
	}

	for _, testCase := range cases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			argoCDServiceMock := mocks.NewRepos(t)

			argoCDServiceMock.EXPECT().GetOciDirectories(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(testCaseCopy.repoApps, testCaseCopy.repoError)

			ociGenerator := NewOciGenerator(argoCDServiceMock)
			applicationSetInfo := v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Generators: []v1alpha1.ApplicationSetGenerator{{
						Oci: &v1alpha1.OciGenerator{
							RepoURL:         "oci://ghcr.io/example/manifests",
							Revision:        "v1.0.0",
							Directories:     testCaseCopy.directories,
							PathParamPrefix: testCaseCopy.pathParamPrefix,
							Values:          testCaseCopy.values,
						},
					}},
				},
			}

			scheme := runtime.NewScheme()
			err := v1alpha1.AddToScheme(scheme)
			require.NoError(t, err)
			appProject := v1alpha1.AppProject{}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appProject).Build()

			got, err := ociGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo, client)

			if testCaseCopy.expectedError != nil {
				require.EqualError(t, err, testCaseCopy.expectedError.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, testCaseCopy.expected, got)
			}
		})
	}
}

func TestOciGenerateParamsFromDirectoriesGoTemplate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		directories     []v1alpha1.OciDirectoryGeneratorItem
		pathParamPrefix string
		repoApps        []string
		repoError       error
		expected        []map[string]any
		expectedError   error
	}{
		{
			name:        "happy flow - created apps",
			directories: []v1alpha1.OciDirectoryGeneratorItem{{Path: "*"}},
			repoApps: []string{
				"app1",
				"app2",
				"app_3",
				"p1/app4",
			},
			repoError: nil,
			expected: []map[string]any{
				{
					"path": map[string]any{
						"path":               "app1",
						"basename":           "app1",
						"basenameNormalized": "app1",
						"segments": []string{
							"app1",
						},
					},
				},
				{
					"path": map[string]any{
						"path":               "app2",
						"basename":           "app2",
						"basenameNormalized": "app2",
						"segments": []string{
							"app2",
						},
					},
				},
				{
					"path": map[string]any{
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
			directories:     []v1alpha1.OciDirectoryGeneratorItem{{Path: "*"}},
			pathParamPrefix: "myRepo",
			repoApps: []string{
				"app1",
				"app2",
				"app_3",
				"p1/app4",
			},
			repoError: nil,
			expected: []map[string]any{
				{
					"myRepo": map[string]any{
						"path": map[string]any{
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
					"myRepo": map[string]any{
						"path": map[string]any{
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
					"myRepo": map[string]any{
						"path": map[string]any{
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
			directories: []v1alpha1.OciDirectoryGeneratorItem{{Path: "p1/*"}, {Path: "p1/*/*"}},
			repoApps: []string{
				"app1",
				"p1/app2",
				"p1/p2/app3",
				"p1/p2/p3/app4",
			},
			repoError: nil,
			expected: []map[string]any{
				{
					"path": map[string]any{
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
					"path": map[string]any{
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
			directories: []v1alpha1.OciDirectoryGeneratorItem{{Path: "p1/*", Exclude: true}, {Path: "*"}, {Path: "*/*"}},
			repoApps: []string{
				"app1",
				"app2",
				"p1/app2",
				"p1/app3",
				"p2/app3",
			},
			repoError: nil,
			expected: []map[string]any{
				{
					"path": map[string]any{
						"path":               "app1",
						"basename":           "app1",
						"basenameNormalized": "app1",
						"segments": []string{
							"app1",
						},
					},
				},
				{
					"path": map[string]any{
						"path":               "app2",
						"basename":           "app2",
						"basenameNormalized": "app2",
						"segments": []string{
							"app2",
						},
					},
				},
				{
					"path": map[string]any{
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
			directories: []v1alpha1.OciDirectoryGeneratorItem{{Path: "*"}, {Path: "*/*"}, {Path: "p1/*", Exclude: true}},
			repoApps: []string{
				"app1",
				"app2",
				"p1/app2",
				"p1/app3",
				"p2/app3",
			},
			repoError: nil,
			expected: []map[string]any{
				{
					"path": map[string]any{
						"path":               "app1",
						"basename":           "app1",
						"basenameNormalized": "app1",
						"segments": []string{
							"app1",
						},
					},
				},
				{
					"path": map[string]any{
						"path":               "app2",
						"basename":           "app2",
						"basenameNormalized": "app2",
						"segments": []string{
							"app2",
						},
					},
				},
				{
					"path": map[string]any{
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
			directories:   []v1alpha1.OciDirectoryGeneratorItem{{Path: "*"}},
			repoApps:      []string{},
			repoError:     nil,
			expected:      []map[string]any{},
			expectedError: nil,
		},
		{
			name:          "handles error from repo server",
			directories:   []v1alpha1.OciDirectoryGeneratorItem{{Path: "*"}},
			repoApps:      []string{},
			repoError:     errors.New("error"),
			expected:      []map[string]any{},
			expectedError: errors.New("error generating params from oci: error getting directories from oci: error"),
		},
	}

	for _, testCase := range cases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			argoCDServiceMock := mocks.NewRepos(t)

			argoCDServiceMock.EXPECT().GetOciDirectories(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(testCaseCopy.repoApps, testCaseCopy.repoError)

			ociGenerator := NewOciGenerator(argoCDServiceMock)
			applicationSetInfo := v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					GoTemplate: true,
					Generators: []v1alpha1.ApplicationSetGenerator{{
						Oci: &v1alpha1.OciGenerator{
							RepoURL:         "oci://ghcr.io/example/manifests",
							Revision:        "v1.0.0",
							Directories:     testCaseCopy.directories,
							PathParamPrefix: testCaseCopy.pathParamPrefix,
						},
					}},
				},
			}

			scheme := runtime.NewScheme()
			err := v1alpha1.AddToScheme(scheme)
			require.NoError(t, err)
			appProject := v1alpha1.AppProject{}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appProject).Build()

			got, err := ociGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo, client)

			if testCaseCopy.expectedError != nil {
				require.EqualError(t, err, testCaseCopy.expectedError.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, testCaseCopy.expected, got)
			}
		})
	}
}

func TestOciGenerateParamsFromFiles(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		// files is the list of paths/globs to match
		files []v1alpha1.OciFileGeneratorItem
		// repoFileContents maps repo path to the literal contents of that path
		repoFileContents map[string][]byte
		// if repoPathsError is non-nil, the call to GetOciFiles(...) will return this error value
		repoPathsError error
		values         map[string]string
		expected       []map[string]any
		expectedError  error
	}{
		{
			name:  "happy flow: create params from oci files",
			files: []v1alpha1.OciFileGeneratorItem{{Path: "**/config.json"}},
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
			expected: []map[string]any{
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
			files: []v1alpha1.OciFileGeneratorItem{{Path: "**/config.json"}},
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
			expected: []map[string]any{
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
			files:            []v1alpha1.OciFileGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{},
			repoPathsError:   errors.New("paths error"),
			expected:         []map[string]any{},
			expectedError:    errors.New("error generating params from oci: paths error"),
		},
		{
			name:  "test invalid JSON file returns error",
			files: []v1alpha1.OciFileGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.json": []byte(`invalid json file`),
			},
			repoPathsError: nil,
			expected:       []map[string]any{},
			expectedError:  errors.New("error generating params from oci: unable to process file 'cluster-config/production/config.json' from repository 'oci://ghcr.io/example/manifests': unable to parse file: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type []map[string]interface {}"),
		},
		{
			name:  "test JSON array",
			files: []v1alpha1.OciFileGeneratorItem{{Path: "**/config.json"}},
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
			expected: []map[string]any{
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
			files: []v1alpha1.OciFileGeneratorItem{{Path: "**/config.yaml"}},
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
			expected: []map[string]any{
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
			files: []v1alpha1.OciFileGeneratorItem{{Path: "**/config.yaml"}},
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
			expected: []map[string]any{
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
			name:  "test empty YAML array",
			files: []v1alpha1.OciFileGeneratorItem{{Path: "**/config.yaml"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.yaml": []byte(`[]`),
			},
			repoPathsError: nil,
			expected:       []map[string]any{},
			expectedError:  nil,
		},
	}

	for _, testCase := range cases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			argoCDServiceMock := mocks.NewRepos(t)
			argoCDServiceMock.EXPECT().GetOciFiles(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(testCaseCopy.repoFileContents, testCaseCopy.repoPathsError)

			ociGenerator := NewOciGenerator(argoCDServiceMock)
			applicationSetInfo := v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Generators: []v1alpha1.ApplicationSetGenerator{{
						Oci: &v1alpha1.OciGenerator{
							RepoURL:  "oci://ghcr.io/example/manifests",
							Revision: "v1.0.0",
							Files:    testCaseCopy.files,
							Values:   testCaseCopy.values,
						},
					}},
				},
			}

			scheme := runtime.NewScheme()
			err := v1alpha1.AddToScheme(scheme)
			require.NoError(t, err)
			appProject := v1alpha1.AppProject{}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appProject).Build()

			got, err := ociGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo, client)

			if testCaseCopy.expectedError != nil {
				require.EqualError(t, err, testCaseCopy.expectedError.Error())
			} else {
				require.NoError(t, err)
				assert.ElementsMatch(t, testCaseCopy.expected, got)
			}
		})
	}
}

func TestOciGeneratorParamsFromFilesWithExcludeOption(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		// files is the list of paths/globs to match
		files []v1alpha1.OciFileGeneratorItem
		// includePattern contains a list of file patterns that needs to be included
		includePattern []string
		// excludePattern contains a list of file patterns that needs to be excluded
		excludePattern []string
		// includeFiles is a map with key as absolute path to file and value as the content in bytes that satisfies the includePattern
		includeFiles map[string][]byte
		// excludeFiles is a map with key as absolute path to file and value as the content in bytes that satisfies the excludePattern
		// This means all the files should be excluded
		excludeFiles map[string][]byte
		// noMatchFiles contains all the files that neither match include pattern nor exclude pattern
		noMatchFiles map[string][]byte
		// if repoPathsError is non-nil, the call to GetOciFiles(...) will return this error value
		repoPathsError error
		values         map[string]string
		expected       []map[string]any
		expectedError  error
	}{
		{
			name: "filter files according to file-path with exclude",
			files: []v1alpha1.OciFileGeneratorItem{
				{
					Path: "**/config.json",
				},
				{
					Path:    "p1/**/config.json",
					Exclude: true,
				},
			},
			includePattern: []string{"**/config.json"},
			excludePattern: []string{"p1/**/config.json"},
			includeFiles: map[string][]byte{
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
				}
`),
				"p1/config.json": []byte(`{
				  "database": {
				    "admin": "db.admin@example.com",
				    "name": "user-data",
				    "host": "db.internal.local",
				    "settings": {
				      "replicas": 3,
				      "backup": "daily"
				    }
				  }
				}
`),
				"p1/p2/config.json": []byte(``),
			},
			excludeFiles: map[string][]byte{
				"p1/config.json": []byte(`{
				  "database": {
				    "admin": "db.admin@example.com",
				    "name": "user-data",
				    "host": "db.internal.local",
				    "settings": {
				      "replicas": 3,
				      "backup": "daily"
				    }
				  }
				}
`),
				"p1/p2/config.json": []byte(``),
			},
			repoPathsError: nil,
			expected: []map[string]any{
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
			},
		},
		{
			name: "filter files according to multiple file-paths with exclude",
			files: []v1alpha1.OciFileGeneratorItem{
				{Path: "**/config.json"},
				{Path: "p1/app2/config.json", Exclude: true},
				{Path: "p1/app3/config.json", Exclude: true},
			},
			includePattern: []string{"**/config.json"},
			excludePattern: []string{"p1/app2/config.json", "p1/app3/config.json"},
			includeFiles: map[string][]byte{
				"p1/config.json": []byte(`{
					"cluster": {
			"owner": "john.doe@example.com",
			"name": "production",
			"address": "https://kubernetes.default.svc",
			"inner": {
				"one" : "two"
			}
		}
}`),
				"p1/app2/config.json": []byte(`{}`),
				"p1/app3/config.json": []byte(`{}`),
			},
			excludeFiles: map[string][]byte{
				"p1/app2/config.json": []byte(`{}`),
				"p1/app3/config.json": []byte(`{}`),
			},
			repoPathsError: nil,
			expected: []map[string]any{
				{
					"cluster.owner":           "john.doe@example.com",
					"cluster.name":            "production",
					"cluster.address":         "https://kubernetes.default.svc",
					"cluster.inner.one":       "two",
					"path":                    "p1",
					"path.basename":           "p1",
					"path[0]":                 "p1",
					"path.basenameNormalized": "p1",
					"path.filename":           "config.json",
					"path.filenameNormalized": "config.json",
				},
			},
		},
		{
			name:           "docs example test case to filter files according to multiple file-paths with exclude",
			files:          []v1alpha1.OciFileGeneratorItem{{Path: "cluster-config/**/config.json"}, {Path: "cluster-config/*/dev/config.json", Exclude: true}},
			includePattern: []string{"cluster-config/**/config.json"},
			excludePattern: []string{"cluster-config/*/dev/config.json"},
			includeFiles: map[string][]byte{
				"cluster-config/engineering/prod/config.json": []byte(`
cluster:
  owner: john.doe@example.com
  name: production
  address: https://kubernetes.default.svc
`),
				"cluster-config/engineering/dev/config.json": []byte(`
cluster:
  owner: foo.bar@example.com
  name: staging
  address: https://kubernetes.default.svc
`),
			},
			excludeFiles: map[string][]byte{
				"cluster-config/engineering/dev/config.json": []byte(`
cluster:
  owner: foo.bar@example.com
  name: staging
  address: https://kubernetes.default.svc
`),
			},
			repoPathsError: nil,
			expected: []map[string]any{
				{
					"cluster.owner":           "john.doe@example.com",
					"cluster.name":            "production",
					"cluster.address":         "https://kubernetes.default.svc",
					"path":                    "cluster-config/engineering/prod",
					"path.basename":           "prod",
					"path[0]":                 "cluster-config",
					"path[1]":                 "engineering",
					"path[2]":                 "prod",
					"path.basenameNormalized": "prod",
					"path.filename":           "config.json",
					"path.filenameNormalized": "config.json",
				},
			},
		},
		{
			name:           "testcase to verify new globbing pattern without any exclude",
			files:          []v1alpha1.OciFileGeneratorItem{{Path: "some-path/*.yaml"}},
			includePattern: []string{"some-path/*.yaml"},
			excludePattern: nil,
			includeFiles: map[string][]byte{
				"some-path/values.yaml": []byte(`
cluster:
  owner: john.doe@example.com
  name: production
  address: https://kubernetes.default.svc
`),
			},
			excludeFiles: map[string][]byte{},
			noMatchFiles: map[string][]byte{
				"some-path/staging/values.yaml": []byte(`
cluster:
  owner: foo.bar@example.com
  name: staging
  address: https://kubernetes.default.svc
`),
			},
			repoPathsError: nil,
			expected: []map[string]any{
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
			},
			expectedError: nil,
		},
		{
			name:           "test to verify the solution for OCI File Generator globbing",
			files:          []v1alpha1.OciFileGeneratorItem{{Path: "cluster-charts/*/*/values.yaml"}, {Path: "cluster-charts/*/values.yaml", Exclude: true}},
			includePattern: []string{"cluster-charts/*/*/values.yaml"},
			excludePattern: []string{"cluster-charts/*/values.yaml"},
			includeFiles: map[string][]byte{
				"cluster-charts/cluster1/mychart/values.yaml": []byte(`
env: staging
`),
				"cluster-charts/cluster1/myotherchart/values.yaml": []byte(`
env: prod
`),
			},
			excludeFiles: map[string][]byte{
				"cluster-charts/cluster2/values.yaml": []byte(`
env: dev
`),
			},
			noMatchFiles: map[string][]byte{
				"cluster-charts/cluster1/mychart/charts/mysubchart/values.yaml": []byte(`
env: testing
`),
			},
			repoPathsError: nil,
			expected: []map[string]any{
				{
					"env":                     "staging",
					"path":                    "cluster-charts/cluster1/mychart",
					"path.filenameNormalized": "values.yaml",
					"path[0]":                 "cluster-charts",
					"path[1]":                 "cluster1",
					"path[2]":                 "mychart",
					"path.basename":           "mychart",
					"path.filename":           "values.yaml",
					"path.basenameNormalized": "mychart",
				},
				{
					"env":                     "prod",
					"path":                    "cluster-charts/cluster1/myotherchart",
					"path.filenameNormalized": "values.yaml",
					"path[0]":                 "cluster-charts",
					"path[1]":                 "cluster1",
					"path[2]":                 "myotherchart",
					"path.basename":           "myotherchart",
					"path.filename":           "values.yaml",
					"path.basenameNormalized": "myotherchart",
				},
			},
			expectedError: nil,
		},
	}

	for _, testCase := range cases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			argoCDServiceMock := mocks.NewRepos(t)

			for _, pattern := range testCaseCopy.excludePattern {
				argoCDServiceMock.EXPECT().GetOciFiles(mock.Anything, mock.Anything, mock.Anything, mock.Anything, pattern, mock.Anything).
					Return(testCaseCopy.excludeFiles, testCaseCopy.repoPathsError)
			}

			for _, pattern := range testCaseCopy.includePattern {
				argoCDServiceMock.EXPECT().GetOciFiles(mock.Anything, mock.Anything, mock.Anything, mock.Anything, pattern, mock.Anything).
					Return(testCaseCopy.includeFiles, testCaseCopy.repoPathsError)
			}

			ociGenerator := NewOciGenerator(argoCDServiceMock)
			applicationSetInfo := v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Generators: []v1alpha1.ApplicationSetGenerator{{
						Oci: &v1alpha1.OciGenerator{
							RepoURL:  "oci://ghcr.io/example/manifests",
							Revision: "v1.0.0",
							Files:    testCaseCopy.files,
							Values:   testCaseCopy.values,
						},
					}},
				},
			}

			scheme := runtime.NewScheme()
			err := v1alpha1.AddToScheme(scheme)
			require.NoError(t, err)
			appProject := v1alpha1.AppProject{}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appProject).Build()

			got, err := ociGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo, client)

			if testCaseCopy.expectedError != nil {
				require.EqualError(t, err, testCaseCopy.expectedError.Error())
			} else {
				require.NoError(t, err)
				assert.ElementsMatch(t, testCaseCopy.expected, got)
			}
		})
	}
}

func TestOciGenerateParamsFromFilesGoTemplate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		files            []v1alpha1.OciFileGeneratorItem
		repoFileContents map[string][]byte
		repoPathsError   error
		values           map[string]string
		expected         []map[string]any
		expectedError    error
	}{
		{
			name:  "happy flow with go template",
			files: []v1alpha1.OciFileGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.json": []byte(`{
   "cluster": {
       "owner": "john.doe@example.com",
       "name": "production",
       "address": "https://kubernetes.default.svc"
   }
}`),
			},
			repoPathsError: nil,
			expected: []map[string]any{
				{
					"cluster": map[string]any{
						"owner":   "john.doe@example.com",
						"name":    "production",
						"address": "https://kubernetes.default.svc",
					},
					"path": map[string]any{
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
			name:             "handles error from repo server with go template",
			files:            []v1alpha1.OciFileGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{},
			repoPathsError:   errors.New("paths error"),
			expected:         nil,
			expectedError:    errors.New("error generating params from oci: paths error"),
		},
		{
			name:  "values interpolation with go template",
			files: []v1alpha1.OciFileGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.json": []byte(`{
   "cluster": {
       "owner": "john.doe@example.com",
       "name": "production"
   }
}`),
			},
			values: map[string]string{
				"env": "prod",
			},
			expected: []map[string]any{
				{
					"cluster": map[string]any{
						"owner": "john.doe@example.com",
						"name":  "production",
					},
					"path": map[string]any{
						"path":               "cluster-config/production",
						"basename":           "production",
						"filename":           "config.json",
						"basenameNormalized": "production",
						"filenameNormalized": "config.json",
						"segments":           []string{"cluster-config", "production"},
					},
					"values": map[string]string{
						"env": "prod",
					},
				},
			},
			expectedError: nil,
		},
		{
			name:  "JSON array with go template",
			files: []v1alpha1.OciFileGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.json": []byte(`[
	{"cluster": {"name": "production"}},
	{"cluster": {"name": "staging"}}
]`),
			},
			expected: []map[string]any{
				{
					"cluster": map[string]any{"name": "production"},
					"path": map[string]any{
						"path":               "cluster-config/production",
						"basename":           "production",
						"filename":           "config.json",
						"basenameNormalized": "production",
						"filenameNormalized": "config.json",
						"segments":           []string{"cluster-config", "production"},
					},
				},
				{
					"cluster": map[string]any{"name": "staging"},
					"path": map[string]any{
						"path":               "cluster-config/production",
						"basename":           "production",
						"filename":           "config.json",
						"basenameNormalized": "production",
						"filenameNormalized": "config.json",
						"segments":           []string{"cluster-config", "production"},
					},
				},
			},
			expectedError: nil,
		},
		{
			name:  "test invalid JSON file returns error",
			files: []v1alpha1.OciFileGeneratorItem{{Path: "**/config.json"}},
			repoFileContents: map[string][]byte{
				"cluster-config/production/config.json": []byte(`invalid json file`),
			},
			repoPathsError: nil,
			expected:       []map[string]any{},
			expectedError:  errors.New("error generating params from oci: unable to process file 'cluster-config/production/config.json' from repository 'oci://ghcr.io/example/manifests': unable to parse file: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type []map[string]interface {}"),
		},
		{
			name:  "Test YAML flow",
			files: []v1alpha1.OciFileGeneratorItem{{Path: "**/config.yaml"}},
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
			expected: []map[string]any{
				{
					"cluster": map[string]any{
						"owner":   "john.doe@example.com",
						"name":    "production",
						"address": "https://kubernetes.default.svc",
					},
					"key1": "val1",
					"key2": map[string]any{
						"key2_1": "val2_1",
						"key2_2": map[string]any{
							"key2_2_1": "val2_2_1",
						},
					},
					"path": map[string]any{
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
					"cluster": map[string]any{
						"owner":   "foo.bar@example.com",
						"name":    "staging",
						"address": "https://kubernetes.default.svc",
					},
					"path": map[string]any{
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
			files: []v1alpha1.OciFileGeneratorItem{{Path: "**/config.yaml"}},
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
			expected: []map[string]any{
				{
					"cluster": map[string]any{
						"owner":   "john.doe@example.com",
						"name":    "production",
						"address": "https://kubernetes.default.svc",
						"inner": map[string]any{
							"one": "two",
						},
					},
					"path": map[string]any{
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
					"cluster": map[string]any{
						"owner":   "john.doe@example.com",
						"name":    "staging",
						"address": "https://kubernetes.default.svc",
					},
					"path": map[string]any{
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

			argoCDServiceMock := mocks.NewRepos(t)
			argoCDServiceMock.EXPECT().GetOciFiles(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(testCaseCopy.repoFileContents, testCaseCopy.repoPathsError)

			ociGenerator := NewOciGenerator(argoCDServiceMock)
			applicationSetInfo := v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					GoTemplate: true,
					Generators: []v1alpha1.ApplicationSetGenerator{{
						Oci: &v1alpha1.OciGenerator{
							RepoURL:  "oci://ghcr.io/example/manifests",
							Revision: "v1.0.0",
							Files:    testCaseCopy.files,
							Values:   testCaseCopy.values,
						},
					}},
				},
			}

			scheme := runtime.NewScheme()
			err := v1alpha1.AddToScheme(scheme)
			require.NoError(t, err)
			appProject := v1alpha1.AppProject{}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appProject).Build()

			got, err := ociGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo, client)

			if testCaseCopy.expectedError != nil {
				require.EqualError(t, err, testCaseCopy.expectedError.Error())
			} else {
				require.NoError(t, err)
				assert.ElementsMatch(t, testCaseCopy.expected, got)
			}
		})
	}
}

func TestOciGenerateParamsFromFilesGoTemplateWithExclude(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		files          []v1alpha1.OciFileGeneratorItem
		includePattern []string
		excludePattern []string
		includeFiles   map[string][]byte
		excludeFiles   map[string][]byte
		expected       []map[string]any
		expectedError  error
	}{
		{
			name: "exclude with go template",
			files: []v1alpha1.OciFileGeneratorItem{
				{Path: "**/config.json"},
				{Path: "p1/**/config.json", Exclude: true},
			},
			includePattern: []string{"**/config.json"},
			excludePattern: []string{"p1/**/config.json"},
			includeFiles: map[string][]byte{
				"cluster-config/production/config.json": []byte(`{"cluster": {"name": "production"}}`),
				"p1/config.json":                        []byte(`{"cluster": {"name": "p1"}}`),
			},
			excludeFiles: map[string][]byte{
				"p1/config.json": []byte(`{"cluster": {"name": "p1"}}`),
			},
			expected: []map[string]any{
				{
					"cluster": map[string]any{"name": "production"},
					"path": map[string]any{
						"path":               "cluster-config/production",
						"basename":           "production",
						"filename":           "config.json",
						"basenameNormalized": "production",
						"filenameNormalized": "config.json",
						"segments":           []string{"cluster-config", "production"},
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "filter files according to multiple file-paths with exclude",
			files: []v1alpha1.OciFileGeneratorItem{
				{Path: "**/config.json"},
				{Path: "p1/app2/config.json", Exclude: true},
				{Path: "p1/app3/config.json", Exclude: true},
			},
			includePattern: []string{"**/config.json"},
			excludePattern: []string{"p1/app2/config.json", "p1/app3/config.json"},
			includeFiles: map[string][]byte{
				"p1/config.json": []byte(`{
					"cluster": {
					"owner": "john.doe@example.com",
					"name": "production",
					"address": "https://kubernetes.default.svc",
					"inner": {
						"one" : "two"
					}
				}
}`),
			},
			excludeFiles: map[string][]byte{
				"p1/app2/config.json": []byte(`{
                    "database": {
                        "admin": "alice.smith@example.com",
					    "env": "staging",
					    "url": "postgres://db.internal.svc:5432",
					    "settings": {
					      "replicas": 3,
					      "backup": "enabled"
					    }
                    }
				}
`),
				"p1/app3/config.json": []byte(`{
					"storage": {
					    "owner": "charlie.brown@example.com",
					    "bucketName": "app-assets",
					    "region": "us-west-2",
					    "options": {
					      "versioning": true,
					      "encryption": "AES256"
					    }
					}
				}
`),
			},
			expected: []map[string]any{
				{
					"cluster": map[string]any{
						"owner":   "john.doe@example.com",
						"name":    "production",
						"address": "https://kubernetes.default.svc",
						"inner": map[string]any{
							"one": "two",
						},
					},
					"path": map[string]any{
						"path":               "p1",
						"basename":           "p1",
						"filename":           "config.json",
						"basenameNormalized": "p1",
						"filenameNormalized": "config.json",
						"segments": []string{
							"p1",
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

			argoCDServiceMock := mocks.NewRepos(t)

			for _, includePattern := range testCaseCopy.includePattern {
				argoCDServiceMock.EXPECT().GetOciFiles(mock.Anything, mock.Anything, mock.Anything, mock.Anything, includePattern, mock.Anything).
					Return(testCaseCopy.includeFiles, nil).Once()
			}
			for _, excludePattern := range testCaseCopy.excludePattern {
				argoCDServiceMock.EXPECT().GetOciFiles(mock.Anything, mock.Anything, mock.Anything, mock.Anything, excludePattern, mock.Anything).
					Return(testCaseCopy.excludeFiles, nil).Once()
			}

			ociGenerator := NewOciGenerator(argoCDServiceMock)
			applicationSetInfo := v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{Name: "set"},
				Spec: v1alpha1.ApplicationSetSpec{
					GoTemplate: true,
					Generators: []v1alpha1.ApplicationSetGenerator{{
						Oci: &v1alpha1.OciGenerator{
							RepoURL:  "oci://ghcr.io/example/manifests",
							Revision: "v1.0.0",
							Files:    testCaseCopy.files,
						},
					}},
				},
			}

			scheme := runtime.NewScheme()
			require.NoError(t, v1alpha1.AddToScheme(scheme))
			appProject := v1alpha1.AppProject{}
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appProject).Build()

			got, err := ociGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo, c)

			if testCaseCopy.expectedError != nil {
				require.EqualError(t, err, testCaseCopy.expectedError.Error())
			} else {
				require.NoError(t, err)
				assert.ElementsMatch(t, testCaseCopy.expected, got)
			}
		})
	}
}

func TestOciGenerateParamsExcludePatternError(t *testing.T) {
	t.Parallel()

	argoCDServiceMock := mocks.NewRepos(t)

	// Include pattern succeeds.
	argoCDServiceMock.EXPECT().GetOciFiles(mock.Anything, mock.Anything, mock.Anything, mock.Anything, "**/config.json", mock.Anything).
		Return(map[string][]byte{"cluster-config/production/config.json": []byte(`{}`)}, nil).Once()
	// Exclude pattern fails.
	argoCDServiceMock.EXPECT().GetOciFiles(mock.Anything, mock.Anything, mock.Anything, mock.Anything, "p1/**/config.json", mock.Anything).
		Return(nil, errors.New("exclude fetch error")).Once()

	ociGenerator := NewOciGenerator(argoCDServiceMock)
	applicationSetInfo := v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{Name: "set"},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{{
				Oci: &v1alpha1.OciGenerator{
					RepoURL:  "oci://ghcr.io/example/manifests",
					Revision: "v1.0.0",
					Files: []v1alpha1.OciFileGeneratorItem{
						{Path: "**/config.json"},
						{Path: "p1/**/config.json", Exclude: true},
					},
				},
			}},
		},
	}

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	appProject := v1alpha1.AppProject{}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appProject).Build()

	_, err := ociGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo, c)
	require.EqualError(t, err, "error generating params from oci: exclude fetch error")
}

func TestOciGenerateParamsEmptySpec(t *testing.T) {
	t.Parallel()

	argoCDServiceMock := mocks.NewRepos(t)
	ociGenerator := NewOciGenerator(argoCDServiceMock)
	applicationSetInfo := v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{Name: "set"},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{{
				Oci: &v1alpha1.OciGenerator{
					RepoURL:  "oci://ghcr.io/example/manifests",
					Revision: "v1.0.0",
					// Neither Directories nor Files set.
				},
			}},
		},
	}

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	appProject := v1alpha1.AppProject{}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appProject).Build()

	_, err := ociGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo, c)
	require.ErrorIs(t, err, ErrEmptyAppSetGenerator)
}

func TestOciGenerator_GenerateParams(t *testing.T) {
	// OciGenerator.resolveSourceIntegrity always returns nil (no AppProject lookup),
	// so the only project-related behaviour to verify is that resolveProjectName
	// correctly translates the Template.Spec.Project value before passing it to
	// GetOciDirectories.
	cases := []struct {
		name               string
		repoApps           []string
		expected           []map[string]any
		expectedError      error
		expectedProject    *string
		appset             v1alpha1.ApplicationSet
		callGetDirectories bool
	}{
		{
			name:     "project field is templated - empty string passed to repo-server",
			repoApps: []string{"app1"},
			appset: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{Name: "set", Namespace: "namespace"},
				Spec: v1alpha1.ApplicationSetSpec{
					Generators: []v1alpha1.ApplicationSetGenerator{{
						Oci: &v1alpha1.OciGenerator{
							RepoURL:     "oci://ghcr.io/example/manifests",
							Revision:    "v1.0.0",
							Directories: []v1alpha1.OciDirectoryGeneratorItem{{Path: "*"}},
							Values:      map[string]string{"foo": "bar"},
						},
					}},
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{Project: "{{.project}}"},
					},
				},
			},
			callGetDirectories: true,
			expectedProject:    new(""),
			expected:           []map[string]any{{"path": "app1", "path.basename": "app1", "path.basenameNormalized": "app1", "path[0]": "app1", "values.foo": "bar"}},
		},
		{
			name:     "project field is not templated - project name passed as-is to repo-server",
			repoApps: []string{"app1"},
			appset: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{Name: "set", Namespace: "namespace"},
				Spec: v1alpha1.ApplicationSetSpec{
					Generators: []v1alpha1.ApplicationSetGenerator{{
						Oci: &v1alpha1.OciGenerator{
							RepoURL:     "oci://ghcr.io/example/manifests",
							Revision:    "v1.0.0",
							Directories: []v1alpha1.OciDirectoryGeneratorItem{{Path: "*"}},
							Values:      map[string]string{"foo": "bar"},
						},
					}},
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{Project: "my-project"},
					},
				},
			},
			callGetDirectories: true,
			expectedProject:    new("my-project"),
			expected:           []map[string]any{{"path": "app1", "path.basename": "app1", "path.basenameNormalized": "app1", "path[0]": "app1", "values.foo": "bar"}},
		},
	}

	for _, testCase := range cases {
		argoCDServiceMock := mocks.NewRepos(t)

		if testCase.callGetDirectories {
			var project any
			if testCase.expectedProject != nil {
				project = *testCase.expectedProject
			} else {
				project = mock.Anything
			}
			argoCDServiceMock.EXPECT().GetOciDirectories(mock.Anything, mock.Anything, mock.Anything, project, mock.Anything).Return(testCase.repoApps, nil)
		}

		ociGenerator := NewOciGenerator(argoCDServiceMock)

		scheme := runtime.NewScheme()
		require.NoError(t, v1alpha1.AddToScheme(scheme))
		c := fake.NewClientBuilder().WithScheme(scheme).Build()

		got, err := ociGenerator.GenerateParams(&testCase.appset.Spec.Generators[0], &testCase.appset, c)

		if testCase.expectedError != nil {
			require.EqualError(t, err, testCase.expectedError.Error())
		} else {
			require.NoError(t, err)
			assert.Equal(t, testCase.expected, got)
		}
	}
}
