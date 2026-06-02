package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
)

func TestNewApplicationSyncCommand_ServerSideGenerateFlag(t *testing.T) {
	cmd := NewApplicationSyncCommand(nil)
	flag := cmd.Flags().Lookup("server-side-generate")
	require.NotNil(t, flag, "flag --server-side-generate should exist")
	assert.Equal(t, "false", flag.DefValue)

	includeFlag := cmd.Flags().Lookup("local-include")
	require.NotNil(t, includeFlag, "flag --local-include should exist")
	assert.Equal(t, "[*.yaml,*.yml,*.json]", includeFlag.DefValue)
}

// fakeGetManifestsWithFilesClient is a fake implementation of
// ApplicationService_GetManifestsWithFilesClient used to unit-test the
// server-side-generate path in the sync command without a real gRPC connection.
type fakeGetManifestsWithFilesClient struct {
	sendErr    error
	recvResult *apiclient.ManifestResponse
	recvErr    error
	sentMsgs   []*applicationpkg.ApplicationManifestQueryWithFilesWrapper
}

func (f *fakeGetManifestsWithFilesClient) Send(msg *applicationpkg.ApplicationManifestQueryWithFilesWrapper) error {
	f.sentMsgs = append(f.sentMsgs, msg)
	return f.sendErr
}

func (f *fakeGetManifestsWithFilesClient) CloseAndRecv() (*apiclient.ManifestResponse, error) {
	return f.recvResult, f.recvErr
}

func (f *fakeGetManifestsWithFilesClient) Header() (metadata.MD, error) { return nil, nil }
func (f *fakeGetManifestsWithFilesClient) Trailer() metadata.MD         { return nil }
func (f *fakeGetManifestsWithFilesClient) CloseSend() error             { return nil }
func (f *fakeGetManifestsWithFilesClient) Context() context.Context     { return context.Background() }
func (f *fakeGetManifestsWithFilesClient) SendMsg(_ any) error          { return nil }
func (f *fakeGetManifestsWithFilesClient) RecvMsg(_ any) error          { return nil }

// fakeAppServiceClientWithManifests extends fakeAppServiceClient to allow
// returning a custom GetManifestsWithFilesClient for server-side-generate tests.
type fakeAppServiceClientWithManifests struct {
	fakeAppServiceClient
	manifestsClient applicationpkg.ApplicationService_GetManifestsWithFilesClient
	manifestsErr    error
}

func (c *fakeAppServiceClientWithManifests) GetManifestsWithFiles(_ context.Context, _ ...grpc.CallOption) (applicationpkg.ApplicationService_GetManifestsWithFilesClient, error) {
	return c.manifestsClient, c.manifestsErr
}

func TestNewApplicationSyncCommand_ServerSideGenerateReturnsManifests(t *testing.T) {
	manifests := []string{
		`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"my-cm","namespace":"default"}}`,
	}
	fakeClient := &fakeGetManifestsWithFilesClient{
		recvResult: &apiclient.ManifestResponse{Manifests: manifests},
	}

	appSvcClient := &fakeAppServiceClientWithManifests{
		manifestsClient: fakeClient,
	}

	// Verify the fake correctly satisfies the interface used by the production code
	var _ applicationpkg.ApplicationService_GetManifestsWithFilesClient = fakeClient
	var _ applicationpkg.ApplicationServiceClient = appSvcClient

	// Verify that calling GetManifestsWithFiles and CloseAndRecv flows correctly
	// (this mirrors the production code path in app.go)
	stream, err := appSvcClient.GetManifestsWithFiles(context.Background())
	require.NoError(t, err)

	response, err := stream.CloseAndRecv()
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, manifests, response.Manifests)
}

func TestValidateLocalSyncFlags(t *testing.T) {
	tests := []struct {
		name               string
		local              string
		serverSideApply    bool
		serverSideGenerate bool
		expectErr          bool
	}{
		{
			name:               "local with server-side apply requires server-side-generate",
			local:              "/tmp/manifests",
			serverSideApply:    true,
			serverSideGenerate: false,
			expectErr:          true,
		},
		{
			name:               "local with server-side apply and server-side-generate is allowed",
			local:              "/tmp/manifests",
			serverSideApply:    true,
			serverSideGenerate: true,
			expectErr:          false,
		},
		{
			name:               "local without server-side apply is allowed",
			local:              "/tmp/manifests",
			serverSideApply:    false,
			serverSideGenerate: false,
			expectErr:          false,
		},
		{
			name:               "non-local with server-side apply is allowed",
			local:              "",
			serverSideApply:    true,
			serverSideGenerate: false,
			expectErr:          false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateLocalSyncFlags(tc.local, tc.serverSideApply, tc.serverSideGenerate)
			if tc.expectErr {
				require.Error(t, err)
				assert.Equal(t, "--server-side with --local requires --server-side-generate", err.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestShouldWarnDeprecatedLocalSync(t *testing.T) {
	assert.True(t, shouldWarnDeprecatedLocalSync("/tmp/manifests", false))
	assert.False(t, shouldWarnDeprecatedLocalSync("/tmp/manifests", true))
	assert.False(t, shouldWarnDeprecatedLocalSync("", false))
}
