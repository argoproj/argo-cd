package generators

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/applicationset/services/scm_provider"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func TestSCMProviderGenerateParams(t *testing.T) {
	cases := []struct {
		name          string
		repos         []*scm_provider.Repository
		values        map[string]string
		expected      []map[string]interface{}
		expectedError error
	}{
		{
			name: "Multiple repos with labels",
			repos: []*scm_provider.Repository{
				{
					Organization: "myorg",
					Repository:   "repo1",
					URL:          "git@github.com:myorg/repo1.git",
					Branch:       "main",
					SHA:          "0bc57212c3cbbec69d20b34c507284bd300def5b",
					Labels:       []string{"prod", "staging"},
				},
				{
					Organization: "myorg",
					Repository:   "repo2",
					URL:          "git@github.com:myorg/repo2.git",
					Branch:       "main",
					SHA:          "59d0",
				},
			},
			expected: []map[string]interface{}{
				{
					"organization":     "myorg",
					"repository":       "repo1",
					"url":              "git@github.com:myorg/repo1.git",
					"branch":           "main",
					"branchNormalized": "main",
					"sha":              "0bc57212c3cbbec69d20b34c507284bd300def5b",
					"short_sha":        "0bc57212",
					"short_sha_7":      "0bc5721",
					"labels":           "prod,staging",
				},
				{
					"organization":     "myorg",
					"repository":       "repo2",
					"url":              "git@github.com:myorg/repo2.git",
					"branch":           "main",
					"branchNormalized": "main",
					"sha":              "59d0",
					"short_sha":        "59d0",
					"short_sha_7":      "59d0",
					"labels":           "",
				},
			},
		},
		{
			name: "Value interpolation",
			repos: []*scm_provider.Repository{
				{
					Organization: "myorg",
					Repository:   "repo3",
					URL:          "git@github.com:myorg/repo3.git",
					Branch:       "main",
					SHA:          "0bc57212c3cbbec69d20b34c507284bd300def5b",
					Labels:       []string{"prod", "staging"},
				},
			},
			values: map[string]string{
				"foo":                    "bar",
				"should_i_force_push_to": "{{ branch }}?",
			},
			expected: []map[string]interface{}{
				{
					"organization":                  "myorg",
					"repository":                    "repo3",
					"url":                           "git@github.com:myorg/repo3.git",
					"branch":                        "main",
					"branchNormalized":              "main",
					"sha":                           "0bc57212c3cbbec69d20b34c507284bd300def5b",
					"short_sha":                     "0bc57212",
					"short_sha_7":                   "0bc5721",
					"labels":                        "prod,staging",
					"values.foo":                    "bar",
					"values.should_i_force_push_to": "main?",
				},
			},
		},
	}

	for _, testCase := range cases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			mockProvider := &scm_provider.MockProvider{
				Repos: testCaseCopy.repos,
			}
			scmGenerator := &SCMProviderGenerator{overrideProvider: mockProvider, SCMConfig: SCMConfig{enableSCMProviders: true}}
			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					Generators: []argoprojiov1alpha1.ApplicationSetGenerator{{
						SCMProvider: &argoprojiov1alpha1.SCMProviderGenerator{
							Values: testCaseCopy.values,
						},
					}},
				},
			}

			got, err := scmGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo, nil)

			if testCaseCopy.expectedError != nil {
				assert.EqualError(t, err, testCaseCopy.expectedError.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, testCaseCopy.expected, got)
			}
		})
	}
}

func TestAllowedSCMProvider(t *testing.T) {
	cases := []struct {
		name           string
		providerConfig *argoprojiov1alpha1.SCMProviderGenerator
	}{
		{
			name: "Error Github",
			providerConfig: &argoprojiov1alpha1.SCMProviderGenerator{
				Github: &argoprojiov1alpha1.SCMProviderGeneratorGithub{
					API: "https://myservice.mynamespace.svc.cluster.local",
				},
			},
		},
		{
			name: "Error Gitlab",
			providerConfig: &argoprojiov1alpha1.SCMProviderGenerator{
				Gitlab: &argoprojiov1alpha1.SCMProviderGeneratorGitlab{
					API: "https://myservice.mynamespace.svc.cluster.local",
				},
			},
		},
		{
			name: "Error Gitea",
			providerConfig: &argoprojiov1alpha1.SCMProviderGenerator{
				Gitea: &argoprojiov1alpha1.SCMProviderGeneratorGitea{
					API: "https://myservice.mynamespace.svc.cluster.local",
				},
			},
		},
		{
			name: "Error Bitbucket",
			providerConfig: &argoprojiov1alpha1.SCMProviderGenerator{
				BitbucketServer: &argoprojiov1alpha1.SCMProviderGeneratorBitbucketServer{
					API: "https://myservice.mynamespace.svc.cluster.local",
				},
			},
		},
		{
			name: "Error AzureDevops",
			providerConfig: &argoprojiov1alpha1.SCMProviderGenerator{
				AzureDevOps: &argoprojiov1alpha1.SCMProviderGeneratorAzureDevOps{
					API: "https://myservice.mynamespace.svc.cluster.local",
				},
			},
		},
	}

	for _, testCase := range cases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			scmGenerator := &SCMProviderGenerator{
				SCMConfig: SCMConfig{
					allowedSCMProviders: []string{
						"github.myorg.com",
						"gitlab.myorg.com",
						"gitea.myorg.com",
						"bitbucket.myorg.com",
						"azuredevops.myorg.com",
					},
					enableSCMProviders: true,
				},
			}

			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					Generators: []argoprojiov1alpha1.ApplicationSetGenerator{{
						SCMProvider: testCaseCopy.providerConfig,
					}},
				},
			}

			_, err := scmGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo, nil)

			require.Error(t, err, "Must return an error")
			var expectedError ErrDisallowedSCMProvider
			assert.ErrorAs(t, err, &expectedError)
		})
	}
}

func TestSCMProviderDisabled_SCMGenerator(t *testing.T) {
	generator := &SCMProviderGenerator{SCMConfig: SCMConfig{enableSCMProviders: false}}

	applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "set",
		},
		Spec: argoprojiov1alpha1.ApplicationSetSpec{
			Generators: []argoprojiov1alpha1.ApplicationSetGenerator{{
				SCMProvider: &argoprojiov1alpha1.SCMProviderGenerator{
					Github: &argoprojiov1alpha1.SCMProviderGeneratorGithub{
						API: "https://myservice.mynamespace.svc.cluster.local",
					},
				},
			}},
		},
	}

	_, err := generator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo, nil)
	assert.ErrorIs(t, err, ErrSCMProvidersDisabled)
}
