package commands

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"regexp"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	appmocks "github.com/argoproj/argo-cd/v3/pkg/apiclient/application/mocks"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// fakeGetClient provides a mock application client for get command tests.
type fakeGetClient struct {
	apiclient.Client
	appClient *appmocks.ApplicationServiceClient
}

func (f *fakeGetClient) NewApplicationClient() (io.Closer, applicationpkg.ApplicationServiceClient, error) {
	return io.NopCloser(nil), f.appClient, nil
}

func (f *fakeGetClient) NewApplicationClientOrDie() (io.Closer, applicationpkg.ApplicationServiceClient) {
	closer, client, err := f.NewApplicationClient()
	if err != nil {
		panic(err)
	}
	return closer, client
}

func mockAppWithMultiSources(testAppName string, sources ...v1alpha1.ApplicationSource) *v1alpha1.Application {
	return &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{Name: testAppName},
		Spec: v1alpha1.ApplicationSpec{
			Sources: sources,
		},
	}
}

func mockAppWithSingleSource(testAppName string, source *v1alpha1.ApplicationSource) *v1alpha1.Application {
	return &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{Name: testAppName},
		Spec: v1alpha1.ApplicationSpec{
			Source: source,
		},
	}
}

// TestApplicationGetCommand_FatalErrors tests fatal errors using the 'exec' pattern.
func TestApplicationGetCommand_FatalErrors(t *testing.T) {
	const testAppName = "my-app"
	multiSourceApp := mockAppWithMultiSources(
		testAppName,
		v1alpha1.ApplicationSource{RepoURL: "https://one", Path: "one"},
		v1alpha1.ApplicationSource{RepoURL: "https://two", Path: "two"},
	)
	singleSourceApp := mockAppWithSingleSource(
		testAppName,
		&v1alpha1.ApplicationSource{RepoURL: "https://one", Path: "one"},
	)

	testCases := []struct {
		name        string
		args        []string
		mockApp     *v1alpha1.Application
		expectedMsg string
	}{
		{
			name:        "ErrorOnUsingPositionAndNameFlagsTogether",
			args:        []string{testAppName, "--source-position", "1", "--source-name", "test"},
			mockApp:     singleSourceApp,
			expectedMsg: "Only one of source-position and source-name can be specified.",
		},
		{
			name:        "ErrorOnInvalidPositionZero",
			args:        []string{testAppName, "--show-params", "--source-position", "0"},
			mockApp:     multiSourceApp,
			expectedMsg: "Source position should be specified and must be greater than 0 for applications with multiple sources",
		},
		{
			name:        "ErrorOnPositionOutOfRange",
			args:        []string{testAppName, "--show-params", "--source-position", "3"},
			mockApp:     multiSourceApp,
			expectedMsg: "Source position should be less than the number of sources in the application",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This is the "child" process. If the env var is set, it runs the code
			// that is expected to call os.Exit() and then returns.
			if os.Getenv("GO_TEST_SUBPROCESS") == "1" {
				mockAppClient := appmocks.NewApplicationServiceClient(t)
				mockAppClient.On("Get", mock.Anything, mock.Anything).Return(tc.mockApp, nil)

				originalNewClient := newClientOrDie
				newClientOrDie = func(_ *apiclient.ClientOptions, _ *cobra.Command) apiclient.Client {
					return &fakeGetClient{appClient: mockAppClient}
				}
				defer func() { newClientOrDie = originalNewClient }()

				cmd := NewApplicationGetCommand(&apiclient.ClientOptions{})
				cmd.SetOut(io.Discard)
				cmd.SetErr(io.Discard)
				cmd.SetArgs(tc.args)
				_ = cmd.Execute()
				return
			}

			// This is the "parent" process. It re-executes itself as a child process
			// with the special environment variable.
			// It uses t.Name() to ensure only the current sub-test is re-run.
			cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^"+regexp.QuoteMeta(t.Name())+"$")
			cmd.Env = append(os.Environ(), "GO_TEST_SUBPROCESS=1")

			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			// Run the child process. It should return an error because it exits with a non-zero code.
			err := cmd.Run()

			require.Error(t, err, "command should exit with an error")

			var exitErr *exec.ExitError
			require.ErrorAs(t, err, &exitErr, "error should be of type *exec.ExitError")
			assert.False(t, exitErr.Success(), "command should fail (non-zero exit code)")
			assert.Contains(t, stderr.String(), tc.expectedMsg, "stderr should contain the expected fatal error message")
		})
	}
}
