package commands

import (
	"context"
	"net"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	sessionpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/session"
	sessionmocks "github.com/argoproj/argo-cd/v3/pkg/apiclient/session/mocks"
	versionpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/v3/util/localconfig"
)

// fakeVersionServer satisfies VersionServiceServer so apiclient.NewClient passes its
// version probe without requiring a full Argo CD server.
type fakeVersionServer struct {
	versionpkg.UnimplementedVersionServiceServer
}

func (f *fakeVersionServer) Version(_ context.Context, _ *emptypb.Empty) (*versionpkg.VersionMessage, error) {
	return &versionpkg.VersionMessage{Version: "v0.0.0-test"}, nil
}

// makeTestArgoJWT returns a signed JWT with iss:"argocd". The signature is checked only
// by the server; Claims() on the stored token uses ParseUnverified, so any key is fine.
func makeTestArgoJWT(t *testing.T) string {
	t.Helper()
	claims := jwt.MapClaims{
		"iss": "argocd",
		"sub": "admin:local",
		"exp": jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("test-secret"))
	require.NoError(t, err)
	return signed
}

func TestNewReloginCommand(t *testing.T) {
	clientOpts := argocdclient.ClientOptions{
		ConfigPath: "/path/to/config",
	}

	cmd := NewReloginCommand(&clientOpts)

	assert.Equal(t, "relogin", cmd.Use, "Unexpected command Use")
	assert.Equal(t, "Refresh an expired authenticate token", cmd.Short, "Unexpected command Short")
	assert.Equal(t, "Refresh an expired authenticate token", cmd.Long, "Unexpected command Long")

	// Assert command flags
	passwordFlag := cmd.Flags().Lookup("password")
	assert.NotNil(t, passwordFlag, "Expected flag --password to be defined")
	assert.Empty(t, passwordFlag.Value.String(), "Unexpected default value for --password flag")

	ssoPortFlag := cmd.Flags().Lookup("sso-port")
	port, err := strconv.Atoi(ssoPortFlag.Value.String())
	assert.NotNil(t, ssoPortFlag, "Expected flag --sso-port to be defined")
	require.NoError(t, err, "Failed to convert sso-port flag value to integer")
	assert.Equal(t, 8085, port, "Unexpected default value for --sso-port flag")
}

func TestNewReloginCommandWithClientOptions(t *testing.T) {
	clientOpts := argocdclient.ClientOptions{
		ConfigPath:        "/path/to/config",
		ServerAddr:        "https://argocd-server.example.com",
		Insecure:          true,
		ClientCertFile:    "/path/to/client-cert",
		ClientCertKeyFile: "/path/to/client-cert-key",
		GRPCWeb:           true,
		GRPCWebRootPath:   "/path/to/grpc-web-root-path",
		PlainText:         true,
		Headers:           []string{"header1", "header2"},
	}

	cmd := NewReloginCommand(&clientOpts)

	assert.Equal(t, "relogin", cmd.Use, "Unexpected command Use")
	assert.Equal(t, "Refresh an expired authenticate token", cmd.Short, "Unexpected command Short")
	assert.Equal(t, "Refresh an expired authenticate token", cmd.Long, "Unexpected command Long")

	// Assert command flags
	passwordFlag := cmd.Flags().Lookup("password")
	assert.NotNil(t, passwordFlag, "Expected flag --password to be defined")
	assert.Empty(t, passwordFlag.Value.String(), "Unexpected default value for --password flag")

	ssoPortFlag := cmd.Flags().Lookup("sso-port")
	port, err := strconv.Atoi(ssoPortFlag.Value.String())
	assert.NotNil(t, ssoPortFlag, "Expected flag --sso-port to be defined")
	require.NoError(t, err, "Failed to convert sso-port flag value to integer")
	assert.Equal(t, 8085, port, "Unexpected default value for --sso-port flag")
}

// TestReloginUsesArgocdContext is a regression test for https://github.com/argoproj/argo-cd/issues/28453.
// It verifies that `argocd relogin --argocd-context <name>` contacts the correct server and
// persists the refreshed token under the correct context entry, even when that context differs
// from the active current-context.
func TestReloginUsesArgocdContext(t *testing.T) {
	// Start a plain-text gRPC server that handles the version probe (so apiclient.NewClient
	// initialises cleanly) and the session Create call (the actual relogin RPC).
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	const newToken = "new-token-for-ctx-b"
	mockSvc := sessionmocks.NewSessionServiceServer(t)
	mockSvc.EXPECT().Create(mock.Anything, mock.Anything).Return(
		&sessionpkg.SessionResponse{Token: newToken}, nil,
	)

	grpcServer := grpc.NewServer()
	versionpkg.RegisterVersionServiceServer(grpcServer, &fakeVersionServer{})
	sessionpkg.RegisterSessionServiceServer(grpcServer, mockSvc)
	go func() { _ = grpcServer.Serve(lis) }()
	defer grpcServer.Stop()

	addr := lis.Addr().String()
	argoJWT := makeTestArgoJWT(t)

	// Two contexts: ctx-a is current; ctx-b points to our mock server.
	// Using 192.0.2.1 (TEST-NET-1, RFC 5737) for ctx-a so it is unreachable.
	configFile := filepath.Join(t.TempDir(), "config")
	cfg := localconfig.LocalConfig{
		CurrentContext: "ctx-a",
		Contexts: []localconfig.ContextRef{
			{Name: "ctx-a", Server: "192.0.2.1:8080", User: "ctx-a"},
			{Name: "ctx-b", Server: addr, User: "ctx-b"},
		},
		Servers: []localconfig.Server{
			{Server: "192.0.2.1:8080", PlainText: true},
			{Server: addr, PlainText: true},
		},
		Users: []localconfig.User{
			{Name: "ctx-a", AuthToken: argoJWT},
			{Name: "ctx-b", AuthToken: argoJWT},
		},
	}
	err = localconfig.WriteLocalConfig(cfg, configFile)
	require.NoError(t, err)

	// Setting clientOpts.Context simulates `--argocd-context ctx-b` on the CLI.
	clientOpts := &argocdclient.ClientOptions{
		ConfigPath: configFile,
		Context:    "ctx-b",
	}
	cmd := NewReloginCommand(clientOpts)
	cmd.SetArgs([]string{"--password", "test-password"})
	cmd.SetContext(context.Background())
	require.NoError(t, cmd.Execute())

	updated, err := localconfig.ReadLocalConfig(configFile)
	require.NoError(t, err)
	assert.Equal(t, newToken, updated.GetToken("ctx-b"), "ctx-b token should be refreshed")
	assert.Equal(t, argoJWT, updated.GetToken("ctx-a"), "ctx-a token should be unchanged")
}
