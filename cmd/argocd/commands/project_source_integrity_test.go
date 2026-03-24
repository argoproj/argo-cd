package commands

import (
	"bytes"
	"io"
	"regexp"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/gpgkey"
	gpgkeymocks "github.com/argoproj/argo-cd/v3/pkg/apiclient/gpgkey/mocks"
	projectmocks "github.com/argoproj/argo-cd/v3/pkg/apiclient/project/mocks"

	projectpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/project"
	appsv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func dummySourceIntegrity() *appsv1.SourceIntegrity {
	return &appsv1.SourceIntegrity{Git: &appsv1.SourceIntegrityGit{
		Policies: []*appsv1.SourceIntegrityGitPolicy{
			{
				Repos: []appsv1.SourceIntegrityGitPolicyRepo{{
					URL: "*",
				}, {
					URL: "!https://github.com/argoproj/argo-cd.git",
				}},
				GPG: &appsv1.SourceIntegrityGitPolicyGPG{
					Mode: appsv1.SourceIntegrityGitPolicyGPGModeHead,
					Keys: []string{"ABCD1234ABCD1234"},
				},
			},
			{
				Repos: []appsv1.SourceIntegrityGitPolicyRepo{{
					URL: "https://github.com/argoproj/argo-cd.git",
				}},
				GPG: &appsv1.SourceIntegrityGitPolicyGPG{
					Mode: appsv1.SourceIntegrityGitPolicyGPGModeStrict,
					Keys: []string{"1234ABCD1234ABCD"},
				},
			},
		},
	}}
}

func dummyProject(projectName string, si *appsv1.SourceIntegrity) *appsv1.AppProject {
	return &appsv1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: projectName},
		Spec: appsv1.AppProjectSpec{
			SourceIntegrity: si,
		},
	}
}

func mockProjectClient(t *testing.T) *projectmocks.ProjectServiceClient {
	t.Helper()
	mockClient := projectmocks.NewProjectServiceClient(t)
	newProjectClient = func(_ *argocdclient.ClientOptions, _ *cobra.Command) (io.Closer, projectpkg.ProjectServiceClient) {
		return io.NopCloser(nil), mockClient
	}
	return mockClient
}

func mockProjectGet(mockClient *projectmocks.ProjectServiceClient, name string, retProj *appsv1.AppProject, retErr error) *mock.Call {
	return mockClient.On("Get", mock.Anything, mock.MatchedBy(func(q *projectpkg.ProjectQuery) bool {
		return q.Name == name
	})).Return(retProj, retErr)
}

func mockGpgKeysClient(t *testing.T) *gpgkeymocks.GPGKeyServiceClient {
	t.Helper()
	mockClient := gpgkeymocks.NewGPGKeyServiceClient(t)
	newGpgKeyClient = func(_ *argocdclient.ClientOptions, _ *cobra.Command) (io.Closer, gpgkey.GPGKeyServiceClient) {
		return io.NopCloser(nil), mockClient
	}
	return mockClient
}

// mockKeyring fakes that keys are added to a repo-server through `argocd gpg add`, so those do not show up in warnings.
func mockKeyring(mockClient *gpgkeymocks.GPGKeyServiceClient, keys ...string) *mock.Call {
	items := make([]appsv1.GnuPGPublicKey, len(keys))
	for _, key := range keys {
		items = append(items, appsv1.GnuPGPublicKey{KeyID: key})
	}
	keyring := &appsv1.GnuPGPublicKeyList{Items: items}
	return mockClient.On("List", mock.Anything, mock.Anything).Return(keyring, nil).Maybe()
}

func runCmd(t *testing.T, cmd *cobra.Command, args ...string) (stdout string, stderr string, e error) {
	t.Helper()
	cmd.SetArgs(args)

	var outbuf bytes.Buffer
	cmd.SetOut(&outbuf)
	var errbuf bytes.Buffer
	cmd.SetErr(&errbuf)

	err := cmd.ExecuteContext(t.Context())
	return outbuf.String(), errbuf.String(), err
}

func TestProjectSourceIntegrityAddCommand(t *testing.T) {
	projectName := "test-project"

	mockKeyring(mockGpgKeysClient(t), "ABCD1234ABCD1234")

	t.Run("Add source integrity verification policy to empty", func(t *testing.T) {
		projects := mockProjectClient(t)
		projects.On("Update", mock.Anything, mock.Anything).Return(nil, nil)
		mockProjectGet(projects, projectName, dummyProject(projectName, nil), nil).Maybe()

		cmd := NewProjectSourceIntegrityGitPoliciesAddCommand(&argocdclient.ClientOptions{})
		out, _, err := runCmd(t, cmd,
			"--gpg-mode=strict",
			"--gpg-key=0123456789ABCDEF",
			"--repo-url=*",
			"--gpg-key", "ABCDEF0123456789",
			"--repo-url", "!*internal*",
			projectName,
		)
		require.NoError(t, err)

		expectedOut := `ID	GPG-MODE	GPG-KEYS	REPO-URLS
0	strict	0123456789ABCDEF, ABCDEF0123456789	*, !*internal*
`
		tabbedOut := regexp.MustCompile(" {2,}").ReplaceAllString(out, "\t")
		assert.Equal(t, expectedOut, tabbedOut)

		updatedProject := captureProjectUpdate(t, projects, projectName)
		require.NotNil(t, updatedProject)
		require.NotNil(t, updatedProject.Spec.SourceIntegrity)
		si := updatedProject.Spec.SourceIntegrity
		require.NotNil(t, si.Git)
		assert.Len(t, si.Git.Policies, 1)
		expected := []*appsv1.SourceIntegrityGitPolicy{{
			Repos: []appsv1.SourceIntegrityGitPolicyRepo{{
				URL: "*",
			}, {
				URL: "!*internal*",
			}},
			GPG: &appsv1.SourceIntegrityGitPolicyGPG{
				Mode: appsv1.SourceIntegrityGitPolicyGPGModeStrict,
				Keys: []string{"0123456789ABCDEF", "ABCDEF0123456789"},
			},
		}}
		assert.Equal(t, expected, si.Git.Policies)
	})

	t.Run("Add source integrity verification policy to existing", func(t *testing.T) {
		projects := mockProjectClient(t)
		projects.On("Update", mock.Anything, mock.Anything).Return(nil, nil)
		mockProjectGet(projects, projectName, dummyProject(projectName, dummySourceIntegrity()), nil).Maybe()

		cmd := NewProjectSourceIntegrityGitPoliciesAddCommand(&argocdclient.ClientOptions{})
		out, _, err := runCmd(t, cmd,
			"--gpg-mode=head",
			"--gpg-key=0123456789ABCDEF",
			"--repo-url=*",
			projectName,
		)
		require.NoError(t, err)

		expectedOut := `ID	GPG-MODE	GPG-KEYS	REPO-URLS
0	head	ABCD1234ABCD1234	*, !https://github.com/argoproj/argo-cd.git
1	strict	1234ABCD1234ABCD	https://github.com/argoproj/argo-cd.git
2	head	0123456789ABCDEF	*
`
		tabbedOut := regexp.MustCompile(" {2,}").ReplaceAllString(out, "\t")
		assert.Equal(t, expectedOut, tabbedOut)

		updatedProject := captureProjectUpdate(t, projects, projectName)
		require.NotNil(t, updatedProject)
		require.NotNil(t, updatedProject.Spec.SourceIntegrity)
		si := updatedProject.Spec.SourceIntegrity
		require.NotNil(t, si.Git)
		assert.Len(t, si.Git.Policies, 3)
		expected := &appsv1.SourceIntegrityGitPolicy{
			Repos: []appsv1.SourceIntegrityGitPolicyRepo{{
				URL: "*",
			}},
			GPG: &appsv1.SourceIntegrityGitPolicyGPG{
				Mode: appsv1.SourceIntegrityGitPolicyGPGModeHead,
				Keys: []string{"0123456789ABCDEF"},
			},
		}
		assert.Equal(t, expected, si.Git.Policies[2])
	})

	t.Run("Add source integrity verification policy - warn existing", func(t *testing.T) {
		projects := mockProjectClient(t)
		projects.On("Update", mock.Anything, mock.Anything).Return(nil, nil)
		mockProjectGet(projects, projectName, dummyProject(projectName, nil), nil).Maybe()

		cmd := NewProjectSourceIntegrityGitPoliciesAddCommand(&argocdclient.ClientOptions{})
		out, stderr, err := runCmd(t, cmd, projectName, "--gpg-mode=head")
		require.NoError(t, err)

		expectedOut := `ID	GPG-MODE	GPG-KEYS	REPO-URLS
0	head	<none>	<none>
`
		tabbedOut := regexp.MustCompile(" {2,}").ReplaceAllString(out, "\t")
		assert.Equal(t, expectedOut, tabbedOut)
		assert.Equal(t, "Warning: Policy has no repository URLs and will never be used\nWarning: Policy has no GPG keys and will never validate any revision\n", stderr)
	})
}

func TestProjectSourceIntegrityListCommand(t *testing.T) {
	projectName := "test-project"

	testCases := []struct {
		name           string
		projectName    string
		expectedStdout string
		expectedStderr string
	}{
		{
			name:        "with policies",
			projectName: projectName,
			expectedStdout: `ID	GPG-MODE	GPG-KEYS	REPO-URLS
0	head	ABCD1234ABCD1234	*, !https://github.com/argoproj/argo-cd.git
1	strict	1234ABCD1234ABCD	https://github.com/argoproj/argo-cd.git
`,
			expectedStderr: "",
		},
		{
			name:           "without policies",
			projectName:    "no-source-integrity",
			expectedStdout: "",
			expectedStderr: "No source integrity git policies defined for project\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			projects := mockProjectClient(t)
			mockProjectGet(projects, projectName, dummyProject(projectName, dummySourceIntegrity()), nil).Maybe()
			mockProjectGet(projects, "no-source-integrity", dummyProject(projectName, nil), nil).Maybe()

			cmd := NewProjectSourceIntegrityGitPoliciesListCommand(&argocdclient.ClientOptions{})
			out, stderr, err := runCmd(t, cmd, tc.projectName)
			require.NoError(t, err)

			tabbedOut := regexp.MustCompile(" {2,}").ReplaceAllString(out, "\t")
			assert.Equal(t, tc.expectedStdout, tabbedOut)
			assert.Equal(t, tc.expectedStderr, stderr)
		})
	}
}

func TestProjectSourceIntegrityUpdateCommand(t *testing.T) {
	projectName := "test-project"

	mockKeyring(mockGpgKeysClient(t), "ABCD1234ABCD1234")

	testCases := []struct {
		name           string
		args           []string
		expectedStdout string
		expectedStderr string
		expectedPolicy *appsv1.SourceIntegrityGitPolicy
	}{
		{
			name: "Set GPG mode",
			args: []string{projectName, "0", "--gpg-mode=strict"},
			expectedStdout: `ID	GPG-MODE	GPG-KEYS	REPO-URLS
0	strict	ABCD1234ABCD1234	*, !https://github.com/argoproj/argo-cd.git
1	strict	1234ABCD1234ABCD	https://github.com/argoproj/argo-cd.git
`,
			expectedStderr: "",
			expectedPolicy: &appsv1.SourceIntegrityGitPolicy{
				Repos: []appsv1.SourceIntegrityGitPolicyRepo{{
					URL: "*",
				}, {
					URL: "!https://github.com/argoproj/argo-cd.git",
				}},
				GPG: &appsv1.SourceIntegrityGitPolicyGPG{
					Mode: appsv1.SourceIntegrityGitPolicyGPGModeStrict,
					Keys: []string{"ABCD1234ABCD1234"},
				},
			},
		},
		{
			name: "Set GPG keys",
			args: []string{projectName, "1", "--gpg-key=FEDCBA9876543210", "--gpg-key=FEDCBA9876543219"},
			expectedStdout: `ID	GPG-MODE	GPG-KEYS	REPO-URLS
0	head	ABCD1234ABCD1234	*, !https://github.com/argoproj/argo-cd.git
1	strict	FEDCBA9876543210, FEDCBA9876543219	https://github.com/argoproj/argo-cd.git
`,
			expectedStderr: "Warning: Following GPG keys are not in repo-server keyring: FEDCBA9876543210, FEDCBA9876543219\n",
			expectedPolicy: &appsv1.SourceIntegrityGitPolicy{
				Repos: []appsv1.SourceIntegrityGitPolicyRepo{{
					URL: "https://github.com/argoproj/argo-cd.git",
				}},
				GPG: &appsv1.SourceIntegrityGitPolicyGPG{
					Mode: appsv1.SourceIntegrityGitPolicyGPGModeStrict,
					Keys: []string{"FEDCBA9876543210", "FEDCBA9876543219"},
				},
			},
		},
		{
			name: "Delete GPG key",
			args: []string{projectName, "0", "--delete-gpg-key=ABCD1234ABCD1234"},
			expectedStdout: `ID	GPG-MODE	GPG-KEYS	REPO-URLS
0	head	<none>	*, !https://github.com/argoproj/argo-cd.git
1	strict	1234ABCD1234ABCD	https://github.com/argoproj/argo-cd.git
`,
			expectedStderr: "Warning: Policy has no GPG keys and will never validate any revision\n",
			expectedPolicy: &appsv1.SourceIntegrityGitPolicy{
				Repos: []appsv1.SourceIntegrityGitPolicyRepo{{
					URL: "*",
				}, {
					URL: "!https://github.com/argoproj/argo-cd.git",
				}},
				GPG: &appsv1.SourceIntegrityGitPolicyGPG{
					Mode: appsv1.SourceIntegrityGitPolicyGPGModeHead,
					Keys: []string{},
				},
			},
		},
		{
			name: "Set repo URLs",
			args: []string{projectName, "1", "--repo-url=https://github.com/example/repo.git", "--repo-url=https://github.com/example/other.git"},
			expectedStdout: `ID	GPG-MODE	GPG-KEYS	REPO-URLS
0	head	ABCD1234ABCD1234	*, !https://github.com/argoproj/argo-cd.git
1	strict	1234ABCD1234ABCD	https://github.com/example/repo.git, https://github.com/example/other.git
`,
			expectedStderr: "Warning: Following GPG keys are not in repo-server keyring: 1234ABCD1234ABCD\n",
			expectedPolicy: &appsv1.SourceIntegrityGitPolicy{
				Repos: []appsv1.SourceIntegrityGitPolicyRepo{{
					URL: "https://github.com/example/repo.git",
				}, {
					URL: "https://github.com/example/other.git",
				}},
				GPG: &appsv1.SourceIntegrityGitPolicyGPG{
					Mode: appsv1.SourceIntegrityGitPolicyGPGModeStrict,
					Keys: []string{"1234ABCD1234ABCD"},
				},
			},
		},
		{
			name: "Remove repo URL",
			args: []string{projectName, "0", "--delete-repo-url=*", "--delete-repo-url=not://present.is/ignored"},
			expectedStdout: `ID	GPG-MODE	GPG-KEYS	REPO-URLS
0	head	ABCD1234ABCD1234	!https://github.com/argoproj/argo-cd.git
1	strict	1234ABCD1234ABCD	https://github.com/argoproj/argo-cd.git
`,
			expectedStderr: "",
			expectedPolicy: &appsv1.SourceIntegrityGitPolicy{
				Repos: []appsv1.SourceIntegrityGitPolicyRepo{{
					URL: "!https://github.com/argoproj/argo-cd.git",
				}},
				GPG: &appsv1.SourceIntegrityGitPolicyGPG{
					Mode: appsv1.SourceIntegrityGitPolicyGPGModeHead,
					Keys: []string{"ABCD1234ABCD1234"},
				},
			},
		},
		{
			name: "Update multiple attributes at once",
			args: []string{
				projectName, "0", "--gpg-mode=none",
				"--add-gpg-key=9876543210FEDCBA",
				"--add-repo-url=https://new-repo.com",
				"--delete-gpg-key=ABCD1234ABCD1234",
				"--delete-repo-url=!https://github.com/argoproj/argo-cd.git",
			},
			expectedStdout: `ID	GPG-MODE	GPG-KEYS	REPO-URLS
0	none	9876543210FEDCBA	*, https://new-repo.com
1	strict	1234ABCD1234ABCD	https://github.com/argoproj/argo-cd.git
`,
			expectedStderr: "Warning: Following GPG keys are not in repo-server keyring: 9876543210FEDCBA\n",
			expectedPolicy: &appsv1.SourceIntegrityGitPolicy{
				Repos: []appsv1.SourceIntegrityGitPolicyRepo{{
					URL: "*",
				}, {
					URL: "https://new-repo.com",
				}},
				GPG: &appsv1.SourceIntegrityGitPolicyGPG{
					Mode: appsv1.SourceIntegrityGitPolicyGPGModeNone,
					Keys: []string{"9876543210FEDCBA"},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			projects := mockProjectClient(t)
			mockProjectGet(projects, projectName, dummyProject(projectName, dummySourceIntegrity()), nil).Maybe()
			projects.On("Update", mock.Anything, mock.Anything).Return(nil, nil)

			cmd := NewProjectSourceIntegrityGitPoliciesUpdateCommand(&argocdclient.ClientOptions{})
			out, stderr, err := runCmd(t, cmd, tc.args...)
			require.NoError(t, err)

			tabbedOut := regexp.MustCompile(" {2,}").ReplaceAllString(out, "\t")
			assert.Equal(t, tc.expectedStdout, tabbedOut)
			assert.Equal(t, tc.expectedStderr, stderr)

			updatedProject := captureProjectUpdate(t, projects, projectName)
			require.NotNil(t, updatedProject)
			require.NotNil(t, updatedProject.Spec.SourceIntegrity)
			si := updatedProject.Spec.SourceIntegrity
			require.NotNil(t, si.Git)

			// Extract the policy ID from args to verify the correct policy was updated
			policyID := tc.args[1]
			var idx int
			if policyID == "0" {
				idx = 0
			} else {
				idx = 1
			}

			assert.Equal(t, tc.expectedPolicy, si.Git.Policies[idx])
		})
	}
}

func TestProjectSourceIntegrityDeleteCommand(t *testing.T) {
	projectName := "test-project"

	testCases := []struct {
		name       string
		args       []string
		expectedSI *appsv1.SourceIntegrity
	}{
		{
			name: "Delete source integrity verification policy 0",
			args: []string{projectName, "0"},
			expectedSI: &appsv1.SourceIntegrity{
				Git: &appsv1.SourceIntegrityGit{
					Policies: []*appsv1.SourceIntegrityGitPolicy{{
						Repos: []appsv1.SourceIntegrityGitPolicyRepo{{
							URL: "https://github.com/argoproj/argo-cd.git",
						}},
						GPG: &appsv1.SourceIntegrityGitPolicyGPG{
							Mode: appsv1.SourceIntegrityGitPolicyGPGModeStrict,
							Keys: []string{"1234ABCD1234ABCD"},
						},
					}},
				},
			},
		},
		{
			name: "Delete source integrity verification policy 1",
			args: []string{projectName, "1"},
			expectedSI: &appsv1.SourceIntegrity{
				Git: &appsv1.SourceIntegrityGit{
					Policies: []*appsv1.SourceIntegrityGitPolicy{{
						Repos: []appsv1.SourceIntegrityGitPolicyRepo{{
							URL: "*",
						}, {
							URL: "!https://github.com/argoproj/argo-cd.git",
						}},
						GPG: &appsv1.SourceIntegrityGitPolicyGPG{
							Mode: appsv1.SourceIntegrityGitPolicyGPGModeHead,
							Keys: []string{"ABCD1234ABCD1234"},
						},
					}},
				},
			},
		},
		{
			name:       "Delete source integrity verification policies (asc)",
			args:       []string{projectName, "0", "1"},
			expectedSI: nil,
		},
		{
			name:       "Delete source integrity verification policies (desc)",
			args:       []string{projectName, "1", "0"},
			expectedSI: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			projects := mockProjectClient(t)
			mockProjectGet(projects, projectName, dummyProject(projectName, dummySourceIntegrity()), nil).Maybe()
			projects.On("Update", mock.Anything, mock.Anything).Return(nil, nil)

			cmd := NewProjectSourceIntegrityGitPoliciesDeleteCommand(&argocdclient.ClientOptions{})
			_, _, err := runCmd(t, cmd, tc.args...)
			require.NoError(t, err)

			updatedProject := captureProjectUpdate(t, projects, projectName)
			assert.Equal(t, tc.expectedSI, updatedProject.Spec.SourceIntegrity)
		})
	}
}

func captureProjectUpdate(t *testing.T, projects *projectmocks.ProjectServiceClient, name string) (updatedProject *appsv1.AppProject) {
	t.Helper()
	capture := mock.MatchedBy(func(q *projectpkg.ProjectUpdateRequest) bool {
		if q.Project.Name != name {
			return false
		}
		updatedProject = q.Project
		return true
	})

	projects.AssertCalled(t, "Update", mock.Anything, capture)
	return updatedProject
}
