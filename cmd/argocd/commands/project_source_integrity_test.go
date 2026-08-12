package commands

import (
	"bytes"
	"context"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	applicationmocks "github.com/argoproj/argo-cd/v3/pkg/apiclient/application/mocks"
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
	items := make([]appsv1.GnuPGPublicKey, 0, len(keys))
	for _, key := range keys {
		items = append(items, appsv1.GnuPGPublicKey{KeyID: key})
	}
	keyring := &appsv1.GnuPGPublicKeyList{Items: items}
	return mockClient.On("List", mock.Anything, mock.Anything).Return(keyring, nil).Maybe()
}

func mockApplicationClient(t *testing.T) *applicationmocks.ApplicationServiceClient {
	t.Helper()
	mockClient := applicationmocks.NewApplicationServiceClient(t)
	newApplicationClient = func(_ *argocdclient.ClientOptions, _ *cobra.Command) (io.Closer, application.ApplicationServiceClient) {
		return io.NopCloser(nil), mockClient
	}
	return mockClient
}

func runCmd(t *testing.T, cmd *cobra.Command, args ...string) (stdout string, stderr string, e error) {
	t.Helper()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	cmd.SetArgs(args)

	var outbuf bytes.Buffer
	cmd.SetOut(&outbuf)
	var errbuf bytes.Buffer
	cmd.SetErr(&errbuf)

	err := cmd.ExecuteContext(t.Context())
	// Make sure the messare from the error reported by Command.RunE() is appended to the errbuf for verification (same as in main.go)
	if err != nil {
		errMsg, _ := NewDefaultPluginHandler().HandleCommandExecutionError(err, true, args)
		if errMsg != "" {
			errbuf.WriteString(errMsg)
		}
	}

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

	t.Run("Add source integrity verification policy without gpg-mode", func(t *testing.T) {
		projects := mockProjectClient(t)
		mockProjectGet(projects, projectName, dummyProject(projectName, dummySourceIntegrity()), nil).Maybe()

		cmd := NewProjectSourceIntegrityGitPoliciesAddCommand(&argocdclient.ClientOptions{})
		_, stderr, err := runCmd(t, cmd,
			"--gpg-key=0123456789ABCDEF",
			"--repo-url=*",
			projectName,
		)
		require.Error(t, err)
		assert.Equal(t, "Error: gpg-mode must be set\n", stderr)
	})
}

func TestProjectSourceIntegrityListCommand(t *testing.T) {
	projectName := "test-project"

	testCases := []struct {
		name            string
		sourceIntegrity *appsv1.SourceIntegrity
		projectName     string
		expectedStdout  string
		expectedStderr  string
	}{
		{
			name:            "with policies",
			sourceIntegrity: dummySourceIntegrity(),
			projectName:     projectName,
			expectedStdout: `ID	GPG-MODE	GPG-KEYS	REPO-URLS
0	head	ABCD1234ABCD1234	*, !https://github.com/argoproj/argo-cd.git
1	strict	1234ABCD1234ABCD	https://github.com/argoproj/argo-cd.git
`,
			expectedStderr: "",
		},
		{
			name:            "without policies",
			sourceIntegrity: nil,
			projectName:     projectName,
			expectedStdout:  "",
			expectedStderr:  "Error: no source integrity git policies defined for project \"test-project\"\n",
		},
		{
			name:            "no project",
			sourceIntegrity: dummySourceIntegrity(),
			projectName:     "not-a-project",
			expectedStdout:  "",
			expectedStderr:  "Error: failed getting project \"not-a-project\": rpc error: code = NotFound desc = appprojects.argoproj.io \"not-a-project\" not found\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			projects := mockProjectClient(t)
			mockProjectGet(projects, projectName, dummyProject(projectName, tc.sourceIntegrity), nil).Maybe()
			mockProjectGet(projects, "not-a-project", nil, errors.New(`rpc error: code = NotFound desc = appprojects.argoproj.io "not-a-project" not found`)).Maybe()

			cmd := NewProjectSourceIntegrityGitPoliciesListCommand(&argocdclient.ClientOptions{})
			out, stderr, _ := runCmd(t, cmd, tc.projectName)

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
		name            string
		sourceIntegrity *appsv1.SourceIntegrity
		args            []string

		expectedStdout string
		expectedStderr string
		expectedPolicy *appsv1.SourceIntegrityGitPolicy
	}{
		{
			name:            "Set GPG mode",
			sourceIntegrity: dummySourceIntegrity(),
			args:            []string{projectName, "0", "--gpg-mode=strict", "--yes"},
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
			name:            "Set GPG keys",
			sourceIntegrity: dummySourceIntegrity(),
			args:            []string{projectName, "1", "--gpg-key=FEDCBA9876543210", "--gpg-key=FEDCBA9876543219", "--yes"},
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
			name:            "Delete GPG key",
			sourceIntegrity: dummySourceIntegrity(),
			args:            []string{projectName, "0", "--delete-gpg-key=ABCD1234ABCD1234", "-y"},
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
			name:            "Set repo URLs",
			sourceIntegrity: dummySourceIntegrity(),
			args:            []string{projectName, "1", "--repo-url=https://github.com/example/repo.git", "--repo-url=https://github.com/example/other.git", "--yes"},
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
			name:            "Remove repo URL",
			sourceIntegrity: dummySourceIntegrity(),
			args:            []string{projectName, "0", "--delete-repo-url=*", "--delete-repo-url=not://present.is/ignored", "--yes"},
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
			name:            "Update multiple attributes at once",
			sourceIntegrity: dummySourceIntegrity(),
			args: []string{
				projectName, "0", "--gpg-mode=none",
				"--add-gpg-key=9876543210FEDCBA",
				"--add-repo-url=https://new-repo.com",
				"--delete-gpg-key=ABCD1234ABCD1234",
				"--delete-repo-url=!https://github.com/argoproj/argo-cd.git",
				"--yes",
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
		{
			name:            "Update policy with invalid project name",
			sourceIntegrity: dummySourceIntegrity(),
			args:            []string{"not-a-project", "0", "--gpg-mode=strict", "--yes"},
			expectedStderr:  "Error: failed getting project \"not-a-project\": rpc error: code = NotFound desc = appprojects.argoproj.io \"not-a-project\" not found\n",
		},
		{
			name:            "Update policy with invalid ID",
			sourceIntegrity: dummySourceIntegrity(),
			args:            []string{projectName, "42", "--gpg-mode=strict", "--yes"},
			expectedStderr:  "Error: the POLICY_ID 42 is out of range (0-1)\n",
		},
		{
			name:            "Update policy with incorrect ID",
			sourceIntegrity: dummySourceIntegrity(),
			args:            []string{projectName, "the first one, bro", "--gpg-mode=strict", "--yes"},
			expectedStderr:  "Error: invalid POLICY_ID 'the first one, bro'\n",
		},
		{
			name:            "Update policy when no Source Integrity",
			sourceIntegrity: nil,
			args:            []string{projectName, "0", "--gpg-mode=strict", "--yes"},
			expectedStderr:  "Error: no source integrity git policies defined for project \"test-project\"\n",
		},
		{
			name:            "Set and add key",
			sourceIntegrity: dummySourceIntegrity(),
			args:            []string{projectName, "0", "--gpg-key=FAKE", "--add-gpg-key=FAKE2", "--gpg-mode=strict", "--yes"},
			expectedStderr:  "Error: option --gpg-key, cannot be combined with --add-gpg-key or --delete-gpg-key\n",
		},
		{
			name:            "Set and add repo",
			sourceIntegrity: dummySourceIntegrity(),
			args:            []string{projectName, "0", "--repo-url=FAKE", "--delete-repo-url=FAKE2", "--gpg-mode=strict", "--yes"},
			expectedStderr:  "Error: option --repo-url, cannot be combined with --add-repo-url or --delete-repo-url\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			projects := mockProjectClient(t)
			mockProjectGet(projects, projectName, dummyProject(projectName, tc.sourceIntegrity), nil).Maybe()
			mockProjectGet(projects, "not-a-project", nil, errors.New(`rpc error: code = NotFound desc = appprojects.argoproj.io "not-a-project" not found`)).Maybe()
			projects.On("Update", mock.Anything, mock.Anything).Return(nil, nil).Maybe()

			cmd := NewProjectSourceIntegrityGitPoliciesUpdateCommand(&argocdclient.ClientOptions{})
			out, stderr, _ := runCmd(t, cmd, tc.args...)

			tabbedOut := regexp.MustCompile(" {2,}").ReplaceAllString(out, "\t")
			assert.Equal(t, tc.expectedStdout, tabbedOut)
			assert.Equal(t, tc.expectedStderr, stderr)

			if tc.expectedPolicy != nil {
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
			}
		})
	}
}

func TestProjectSourceIntegrityDeleteCommand(t *testing.T) {
	projectName := "test-project"

	testCases := []struct {
		name            string
		sourceIntegrity *appsv1.SourceIntegrity
		args            []string

		expectedSI     *appsv1.SourceIntegrity
		expectedStderr string
	}{
		{
			name:            "Delete source integrity verification policy 0",
			sourceIntegrity: dummySourceIntegrity(),
			args:            []string{projectName, "0", "--yes"},
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
			name:            "Delete source integrity verification policy 1",
			sourceIntegrity: dummySourceIntegrity(),
			args:            []string{projectName, "--yes", "1"},
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
			name:            "Delete source integrity verification policies (asc)",
			sourceIntegrity: dummySourceIntegrity(),
			args:            []string{projectName, "--yes", "0", "1"},
			expectedSI:      nil,
		},
		{
			name:            "Delete source integrity verification policies (desc)",
			sourceIntegrity: dummySourceIntegrity(),
			args:            []string{projectName, "--yes", "1", "0"},
			expectedSI:      nil,
		},
		{
			name:            "Delete policy with invalid project name",
			sourceIntegrity: dummySourceIntegrity(),
			args:            []string{"not-a-project", "--yes", "0"},
			expectedStderr:  "Error: failed getting project \"not-a-project\": rpc error: code = NotFound desc = appprojects.argoproj.io \"not-a-project\" not found\n",
		},
		{
			name:            "Delete policy with invalid ID",
			sourceIntegrity: dummySourceIntegrity(),
			args:            []string{projectName, "--yes", "42"},
			expectedStderr:  "Error: the POLICY_ID 42 is out of range (0-1)\n",
		},
		{
			name:            "Delete policy with incorrect ID",
			sourceIntegrity: dummySourceIntegrity(),
			args:            []string{projectName, "--yes", "the first one, bro"},
			expectedStderr:  "Error: invalid POLICY_ID 'the first one, bro'\n",
		},
		{
			name:            "Delete policy when there is no Source Integrity",
			sourceIntegrity: nil,
			args:            []string{projectName, "--yes", "0"},
			expectedStderr:  "Error: no source integrity git policies defined for project \"test-project\"\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			projects := mockProjectClient(t)
			mockProjectGet(projects, projectName, dummyProject(projectName, tc.sourceIntegrity), nil).Maybe()
			mockProjectGet(projects, "not-a-project", nil, errors.New(`rpc error: code = NotFound desc = appprojects.argoproj.io "not-a-project" not found`)).Maybe()
			projects.On("Update", mock.Anything, mock.Anything).Return(nil, nil).Maybe()

			cmd := NewProjectSourceIntegrityGitPoliciesDeleteCommand(&argocdclient.ClientOptions{})
			stdout, stderr, err := runCmd(t, cmd, tc.args...)
			assert.Equal(t, tc.expectedStderr, stderr, "Err: %w, Out: %s", err, stdout)

			if tc.expectedSI != nil {
				updatedProject := captureProjectUpdate(t, projects, projectName)
				assert.Equal(t, tc.expectedSI, updatedProject.Spec.SourceIntegrity)
			}
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

func TestProjectSourceIntegrityGpgInspectRepoCommand_WarningsAndReturnCodes(t *testing.T) {
	projectName := "test-project"
	applicationName := "test-app"

	usageMessageParts := []string{
		"Inspect the Git/GPG source integrity of an application in a project",
		"Usage:",
		"gpg-inspect-repo PROJECT APPNAME [flags]",
	}

	tests := []struct {
		name           string
		mockSetup      func(ctx context.Context, mock *applicationmocks.ApplicationServiceClient)
		args           []string
		expectedError  error
		expectedStdout []string
		expectedStderr []string
	}{
		{
			name:           "no args",
			args:           []string{},
			mockSetup:      nil,
			expectedError:  NewExitError(1, nil),
			expectedStdout: usageMessageParts,
			expectedStderr: []string{},
		},
		{
			name:           "single arg",
			args:           []string{projectName},
			mockSetup:      nil,
			expectedError:  NewExitError(1, nil),
			expectedStdout: usageMessageParts,
			expectedStderr: []string{},
		},
		{
			name:           "three args",
			args:           []string{projectName, applicationName, "test-repo"},
			mockSetup:      nil,
			expectedError:  NewExitError(1, nil),
			expectedStdout: usageMessageParts,
			expectedStderr: []string{},
		},
		{
			name: "rpc fails",
			args: []string{projectName, applicationName},
			mockSetup: func(ctx context.Context, mock *applicationmocks.ApplicationServiceClient) {
				mock.EXPECT().
					InspectGitGPGSourceIntegrity(ctx, &application.InspectGitGPGSourceIntegrityQuery{Name: &applicationName, Project: &projectName}).
					Return(nil, errors.New("rpc error"))
			},
			expectedError:  errors.New("failed inspecting git gpg source integrity for application \"test-app\": rpc error"),
			expectedStdout: []string{},
			expectedStderr: []string{`Error: failed inspecting git gpg source integrity for application "test-app": rpc error`},
		},
		{
			name: "source integrity not configured for any source",
			args: []string{projectName, applicationName},
			mockSetup: func(ctx context.Context, mock *applicationmocks.ApplicationServiceClient) {
				mock.EXPECT().
					InspectGitGPGSourceIntegrity(ctx, &application.InspectGitGPGSourceIntegrityQuery{Name: &applicationName, Project: &projectName}).
					Return(&application.InspectGitGPGSourceIntegrityListResponse{Items: []*application.InspectGitGPGSourceIntegrityResponse{}}, nil)
			},
			expectedError:  NewExitError(3, nil),
			expectedStdout: []string{},
			expectedStderr: []string{`Git/GPG source integrity is not configured for any source of application "test-app", check the project and application configuration.`},
		},
		{
			name: "source integrity has problems",
			args: []string{projectName, applicationName},
			mockSetup: func(ctx context.Context, mock *applicationmocks.ApplicationServiceClient) {
				mock.EXPECT().
					InspectGitGPGSourceIntegrity(ctx, &application.InspectGitGPGSourceIntegrityQuery{Name: &applicationName, Project: &projectName}).
					Return(&application.InspectGitGPGSourceIntegrityListResponse{
						Items: []*application.InspectGitGPGSourceIntegrityResponse{
							prepareInspectGitGPGSourceIntegrityResponse(
								"https://github.com/argoproj/argo-cd.git",
								"v1.0.0",
								"abcd1234",
								nil,
								[]*application.GitGPGCommitInfo{},
								"multiple git/gpg policies are configured, invalid configuration",
							),
						},
					}, nil)
			},
			expectedError:  NewExitError(2, nil),
			expectedStdout: []string{"PROBLEMS: multiple git/gpg policies are configured, invalid configuration"},
			expectedStderr: []string{},
		},
		{
			name: "source integrity in head mode",
			args: []string{projectName, applicationName},
			mockSetup: func(ctx context.Context, mock *applicationmocks.ApplicationServiceClient) {
				mock.EXPECT().
					InspectGitGPGSourceIntegrity(ctx, &application.InspectGitGPGSourceIntegrityQuery{Name: &applicationName, Project: &projectName}).
					Return(&application.InspectGitGPGSourceIntegrityListResponse{
						Items: []*application.InspectGitGPGSourceIntegrityResponse{
							prepareInspectGitGPGSourceIntegrityResponse(
								"https://github.com/argoproj/argo-cd.git",
								"v1.0.0",
								"abcd1234",
								&appsv1.SourceIntegrityGitPolicyGPG{
									Mode: appsv1.SourceIntegrityGitPolicyGPGModeHead,
									Keys: []string{"ABCD1234ABCD1234"},
								},
								[]*application.GitGPGCommitInfo{},
								"",
							),
						},
					}, nil)
			},
			expectedError: nil, // verification passed, with just a warning - no error return code
			expectedStdout: []string{
				"Repo URL: https://github.com/argoproj/argo-cd.git",
				"Resolved Revision: v1.0.0",
				"Target Revision: abcd1234",
				"GPG Mode: head (SIMULATED STRICT MODE)",
				"GPG Keys:",
				"ABCD1234ABCD1234",
				`WARNING: Project does not have strict Git/GPG mode enabled. Strict GPG verification is not actually enforced.`,
				"Source passes strict Git/GPG source integrity checks.",
			},
			expectedStderr: []string{},
		},
		{
			name: "source integrity in none mode",
			args: []string{projectName, applicationName},
			mockSetup: func(ctx context.Context, mock *applicationmocks.ApplicationServiceClient) {
				mock.EXPECT().
					InspectGitGPGSourceIntegrity(ctx, &application.InspectGitGPGSourceIntegrityQuery{Name: &applicationName, Project: &projectName}).
					Return(&application.InspectGitGPGSourceIntegrityListResponse{
						Items: []*application.InspectGitGPGSourceIntegrityResponse{
							prepareInspectGitGPGSourceIntegrityResponse(
								"https://github.com/argoproj/argo-cd.git",
								"v1.0.0",
								"abcd1234",
								&appsv1.SourceIntegrityGitPolicyGPG{
									Mode: appsv1.SourceIntegrityGitPolicyGPGModeNone,
									Keys: []string{"ABCD1234ABCD1234"},
								},
								[]*application.GitGPGCommitInfo{},
								"",
							),
						},
					}, nil)
			},
			expectedError: nil, // verification passed, with just a warning - no error return code
			expectedStdout: []string{
				"Repo URL: https://github.com/argoproj/argo-cd.git",
				"Resolved Revision: v1.0.0",
				"Target Revision: abcd1234",
				"GPG Mode: none (SIMULATED STRICT MODE)",
				"GPG Keys:",
				"ABCD1234ABCD1234",
				`WARNING: Project does not have strict Git/GPG mode enabled. Strict GPG verification is not actually enforced.`,
				"Source passes strict Git/GPG source integrity checks.",
			},
			expectedStderr: []string{},
		},
		{
			name: "source integrity in strict mode",
			args: []string{projectName, applicationName},
			mockSetup: func(ctx context.Context, mock *applicationmocks.ApplicationServiceClient) {
				mock.EXPECT().
					InspectGitGPGSourceIntegrity(ctx, &application.InspectGitGPGSourceIntegrityQuery{Name: &applicationName, Project: &projectName}).
					Return(&application.InspectGitGPGSourceIntegrityListResponse{
						Items: []*application.InspectGitGPGSourceIntegrityResponse{
							prepareInspectGitGPGSourceIntegrityResponse(
								"https://github.com/argoproj/argo-cd.git",
								"v1.0.0",
								"abcd1234",
								&appsv1.SourceIntegrityGitPolicyGPG{
									Mode: appsv1.SourceIntegrityGitPolicyGPGModeStrict,
									Keys: []string{"ABCD1234ABCD1234"},
								},
								[]*application.GitGPGCommitInfo{},
								"",
							),
						},
					}, nil)
			},
			expectedError: nil, // verification passed
			expectedStdout: []string{
				"Repo URL: https://github.com/argoproj/argo-cd.git",
				"Resolved Revision: v1.0.0",
				"Target Revision: abcd1234",
				"GPG Mode: strict",
				"GPG Keys:",
				"ABCD1234ABCD1234",
				"Source passes strict Git/GPG source integrity checks.",
			},
			expectedStderr: []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			applications := mockApplicationClient(t)
			if test.mockSetup != nil {
				test.mockSetup(t.Context(), applications)
			}

			cmd := NewProjectSourceIntegrityGitGpgInspectRepoCommand(&argocdclient.ClientOptions{})
			stdout, stderr, err := runCmd(t, cmd, test.args...)

			assert.Equal(t, ExitCodeForError(test.expectedError), ExitCodeForError(err))
			assert.Equal(t, CLIErrorMessage(test.expectedError), CLIErrorMessage(err))
			assertContainsAllParts(t, test.expectedStderr, stderr)
			assertContainsAllParts(t, test.expectedStdout, stdout)
		})
	}
}

func TestProjectSourceIntegrityGpgInspectRepoCommand_SinglePassingSource(t *testing.T) {
	projectName := "test-project"
	applicationName := "test-app"

	stdoutParts := []string{
		"Repo URL: https://github.com/argoproj/argo-cd.git",
		"Target Revision: v1.0.0",
		"Resolved Revision: abcd1234",
		"GPG Mode: head (SIMULATED STRICT MODE)",
		"GPG Keys:",
		"  ABCD1234ABCD1234",
		"",
		"WARNING: Project does not have strict Git/GPG mode enabled. Strict GPG verification is not actually enforced.",
		"",
		"Source passes strict Git/GPG source integrity checks.",
		"",
		"To inspect repository:",
		"  git fetch --tags",
		"  git checkout abcd1234",
		"  git log --oneline abcd1234",
		"",
		"To create seal commit (this will trust all problematic commits up to this point):",
		"  git commit --signoff --gpg-sign --trailer=\"Argocd-gpg-seal: <justification>\"",
	}
	expectedStdout := strings.Join(stdoutParts, "\n") + "\n"

	applications := mockApplicationClient(t)
	applications.EXPECT().
		InspectGitGPGSourceIntegrity(t.Context(), &application.InspectGitGPGSourceIntegrityQuery{Name: &applicationName, Project: &projectName}).
		Return(&application.InspectGitGPGSourceIntegrityListResponse{
			Items: []*application.InspectGitGPGSourceIntegrityResponse{
				prepareInspectGitGPGSourceIntegrityResponse(
					"https://github.com/argoproj/argo-cd.git",
					"abcd1234",
					"v1.0.0",
					&appsv1.SourceIntegrityGitPolicyGPG{
						Mode: appsv1.SourceIntegrityGitPolicyGPGModeHead,
						Keys: []string{"ABCD1234ABCD1234"},
					},
					[]*application.GitGPGCommitInfo{},
					"",
				),
			},
		}, nil)

	cmd := NewProjectSourceIntegrityGitGpgInspectRepoCommand(&argocdclient.ClientOptions{})
	stdout, stderr, err := runCmd(t, cmd, projectName, applicationName)

	assert.Equal(t, ExitCodeForError(nil), ExitCodeForError(err))
	assert.Equal(t, CLIErrorMessage(nil), CLIErrorMessage(err))
	assert.Empty(t, stderr)
	assert.Equal(t, expectedStdout, stdout)
}

func TestProjectSourceIntegrityGpgInspectRepoCommand_SingleProblematicCommitsSource(t *testing.T) {
	projectName := "test-project"
	applicationName := "test-app"

	stdoutParts := []string{
		"Repo URL: https://github.com/argoproj/argo-cd.git",
		"Target Revision: v1.0.0",
		"Resolved Revision: abcd1234",
		"GPG Mode: strict",
		"GPG Keys:",
		"  ABCD1234ABCD1234",
		"",
		"PROBLEMATIC COMMITS:",
		"  Revision  Date                             Author                             Subject          Result",
		"  abcd1234  Fri, 02 Jan 2026 15:55:44 +0200  Jim Smith <jim.smith@example.com>  Add test commit  unsigned(key_id=)",
		"  defe2234  Sun, 14 Dec 2025 15:50:22 +0000  John Doe <john.doe@example.com>    Fix app port     unsigned(key_id=)",
		"",
		"To inspect repository:",
		"  git fetch --tags",
		"  git checkout abcd1234",
		"  git log --oneline abcd1234",
		"  git log -p --no-walk abcd1234 defe2234",
		"",
		"To create seal commit (this will trust all problematic commits up to this point):",
		"  git commit --signoff --gpg-sign --trailer=\"Argocd-gpg-seal: <justification>\"",
	}
	expectedStdout := strings.Join(stdoutParts, "\n") + "\n"
	expectedErr := NewExitError(2, nil)

	applications := mockApplicationClient(t)
	applications.EXPECT().
		InspectGitGPGSourceIntegrity(t.Context(), &application.InspectGitGPGSourceIntegrityQuery{Name: &applicationName, Project: &projectName}).
		Return(&application.InspectGitGPGSourceIntegrityListResponse{
			Items: []*application.InspectGitGPGSourceIntegrityResponse{
				prepareInspectGitGPGSourceIntegrityResponse(
					"https://github.com/argoproj/argo-cd.git",
					"abcd1234",
					"v1.0.0",
					&appsv1.SourceIntegrityGitPolicyGPG{
						Mode: appsv1.SourceIntegrityGitPolicyGPGModeStrict,
						Keys: []string{"ABCD1234ABCD1234"},
					},
					[]*application.GitGPGCommitInfo{
						prepareGitGPGCommitInfo("abcd1234", "Fri, 2 Jan 2026 15:55:44 +0200", "Jim Smith <jim.smith@example.com>", "Add test commit", "unsigned(key_id=)"),
						prepareGitGPGCommitInfo("defe2234", "Sun, 14 Dec 2025 15:50:22 +0000", "John Doe <john.doe@example.com>", "Fix app port", "unsigned(key_id=)"),
					},
					"",
				),
			},
		}, nil)

	cmd := NewProjectSourceIntegrityGitGpgInspectRepoCommand(&argocdclient.ClientOptions{})
	stdout, stderr, err := runCmd(t, cmd, projectName, applicationName)

	assert.Equal(t, ExitCodeForError(expectedErr), ExitCodeForError(err))
	assert.Equal(t, CLIErrorMessage(expectedErr), CLIErrorMessage(err))
	assert.Empty(t, stderr)
	assert.Equal(t, expectedStdout, stdout)
}

func TestProjectSourceIntegrityGpgInspectRepoCommand_MultipleSources(t *testing.T) {
	projectName := "test-project"
	applicationName := "test-app"

	stdoutParts := []string{
		"Repo URL: https://github.com/argoproj/argo-cd.git",
		"Target Revision: v1.0.0",
		"Resolved Revision: abcd1234",
		"GPG Mode: strict",
		"GPG Keys:",
		"  1234ABCD1234ABCD",
		"",
		"Source passes strict Git/GPG source integrity checks.",
		"",
		"To inspect repository:",
		"  git fetch --tags",
		"  git checkout abcd1234",
		"  git log --oneline abcd1234",
		"",
		"To create seal commit (this will trust all problematic commits up to this point):",
		"  git commit --signoff --gpg-sign --trailer=\"Argocd-gpg-seal: <justification>\"",
		"",
		"--------------------------------",
		"",
		"Repo URL: https://github.com/argoproj/argo-cd-fork.git",
		"Target Revision: v1.0.1",
		"Resolved Revision: eef2234",
		"GPG Mode: head (SIMULATED STRICT MODE)",
		"GPG Keys:",
		"  ABCD1234ABCD1234",
		"  1234ABCD1234ABCD",
		"",
		"WARNING: Project does not have strict Git/GPG mode enabled. Strict GPG verification is not actually enforced.",
		"",
		"PROBLEMATIC COMMITS:",
		"  Revision  Date                             Author                             Subject          Result",
		"  abcd1234  Fri, 02 Jan 2026 15:55:44 +0200  Jim Smith <jim.smith@example.com>  Add test commit  unsigned(key_id=)",
		"  defe2234  Sun, 14 Dec 2025 15:50:22 +0000  John Doe <john.doe@example.com>    Fix app port     signed with expired key(key_id=ABCD1234ABCD1234)",
		"",
		"To inspect repository:",
		"  git fetch --tags",
		"  git checkout eef2234",
		"  git log --oneline eef2234",
		"  git log -p --no-walk abcd1234 defe2234",
		"",
		"To create seal commit (this will trust all problematic commits up to this point):",
		"  git commit --signoff --gpg-sign --trailer=\"Argocd-gpg-seal: <justification>\"",
		"",
		"--------------------------------",
		"",
		"Repo URL: https://github.com/argoproj/argo-cd-fork2.git",
		"Target Revision: v1.0.2",
		"",
		"PROBLEMS: multiple git/gpg policies are configured, invalid configuration",
		"",
		"To inspect repository:",
		"  git fetch --tags",
		"  git checkout v1.0.2",
		"  git log --oneline v1.0.2",
		"",
		"To create seal commit (this will trust all problematic commits up to this point):",
		"  git commit --signoff --gpg-sign --trailer=\"Argocd-gpg-seal: <justification>\"",
	}
	expectedStdout := strings.Join(stdoutParts, "\n") + "\n"
	expectedErr := NewExitError(2, nil)

	applications := mockApplicationClient(t)
	applications.EXPECT().
		InspectGitGPGSourceIntegrity(t.Context(), &application.InspectGitGPGSourceIntegrityQuery{Name: &applicationName, Project: &projectName}).
		Return(&application.InspectGitGPGSourceIntegrityListResponse{
			Items: []*application.InspectGitGPGSourceIntegrityResponse{
				prepareInspectGitGPGSourceIntegrityResponse(
					"https://github.com/argoproj/argo-cd.git",
					"abcd1234",
					"v1.0.0",
					&appsv1.SourceIntegrityGitPolicyGPG{
						Mode: appsv1.SourceIntegrityGitPolicyGPGModeStrict,
						Keys: []string{"1234ABCD1234ABCD"},
					},
					[]*application.GitGPGCommitInfo{},
					"",
				),
				prepareInspectGitGPGSourceIntegrityResponse(
					"https://github.com/argoproj/argo-cd-fork.git",
					"eef2234",
					"v1.0.1",
					&appsv1.SourceIntegrityGitPolicyGPG{
						Mode: appsv1.SourceIntegrityGitPolicyGPGModeHead,
						Keys: []string{"ABCD1234ABCD1234", "1234ABCD1234ABCD"},
					},
					[]*application.GitGPGCommitInfo{
						prepareGitGPGCommitInfo("abcd1234", "Fri, 2 Jan 2026 15:55:44 +0200", "Jim Smith <jim.smith@example.com>", "Add test commit", "unsigned(key_id=)"),
						prepareGitGPGCommitInfo("defe2234", "Sun, 14 Dec 2025 15:50:22 +0000", "John Doe <john.doe@example.com>", "Fix app port", "signed with expired key(key_id=ABCD1234ABCD1234)"),
					},
					"",
				),
				prepareInspectGitGPGSourceIntegrityResponse(
					"https://github.com/argoproj/argo-cd-fork2.git",
					"v1.0.2",
					"v1.0.2",
					nil,
					[]*application.GitGPGCommitInfo{},
					"multiple git/gpg policies are configured, invalid configuration",
				),
			},
		}, nil)

	cmd := NewProjectSourceIntegrityGitGpgInspectRepoCommand(&argocdclient.ClientOptions{})
	stdout, stderr, err := runCmd(t, cmd, projectName, applicationName)

	assert.Equal(t, ExitCodeForError(expectedErr), ExitCodeForError(err))
	assert.Equal(t, CLIErrorMessage(expectedErr), CLIErrorMessage(err))
	assert.Empty(t, stderr)
	assert.Equal(t, expectedStdout, stdout)
}

func prepareInspectGitGPGSourceIntegrityResponse(repoUrl string, resolvedRevision string, targetRevision string, gitGpgPolicy *appsv1.SourceIntegrityGitPolicyGPG, commits []*application.GitGPGCommitInfo, errorMessage string) *application.InspectGitGPGSourceIntegrityResponse {
	return &application.InspectGitGPGSourceIntegrityResponse{
		RepoUrl:          &repoUrl,
		ResolvedRevision: &resolvedRevision,
		TargetRevision:   &targetRevision,
		GitGpgPolicy:     gitGpgPolicy,
		Commits:          commits,
		ErrorMessage:     &errorMessage,
	}
}

func prepareGitGPGCommitInfo(revision string, date string, author string, subject string, verificationResult string) *application.GitGPGCommitInfo {
	return &application.GitGPGCommitInfo{
		Revision:           &revision,
		Date:               &date,
		Author:             &author,
		Subject:            &subject,
		VerificationResult: &verificationResult,
	}
}

func assertContainsAllParts(t *testing.T, expected []string, actual string) {
	t.Helper()
	if len(expected) == 0 {
		assert.Empty(t, actual)
		return
	}
	for _, expected := range expected {
		assert.Contains(t, actual, expected)
	}
}
