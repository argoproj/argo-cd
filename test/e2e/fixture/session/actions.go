package session

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oauth2-proxy/mockoidc"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v3/util/localconfig"
)

var (
	// mockServer is a package-level singleton mock OIDC server shared across all tests.
	mockServer     *mockoidc.MockOIDC
	mockServerOnce sync.Once
)

// Actions implements the "when" part of given/when/then for session/logout tests.
type Actions struct {
	context *Context

	// configPaths maps token names to per-token CLI config file paths
	configPaths map[string]string

	// tokenStore tracks named tokens read from CLI config files
	tokenStore map[string]string

	// lastOutput holds the stdout from the most recent CLI action
	lastOutput string

	// lastError holds the error from the most recent action
	lastError error
}

// getTokenStore initializes the token store
func (a *Actions) getTokenStore() map[string]string {
	if a.tokenStore == nil {
		a.tokenStore = make(map[string]string)
	}
	return a.tokenStore
}

// getConfigPaths initializes the config paths map
func (a *Actions) getConfigPaths() map[string]string {
	if a.configPaths == nil {
		a.configPaths = make(map[string]string)
	}
	return a.configPaths
}

// configPathFor returns (or creates) a per-token temp config file path.
func (a *Actions) configPathFor(tokenName string) string {
	a.context.T().Helper()
	paths := a.getConfigPaths()
	if p, ok := paths[tokenName]; ok {
		return p
	}
	p := filepath.Join(a.context.T().TempDir(), tokenName+"-config")
	paths[tokenName] = p
	return p
}

func (a *Actions) getSharedMockOIDCServer() *mockoidc.MockOIDC {
	mockServerOnce.Do(func() {
		t := a.context.T()
		// Create a fresh RSA Private Key for token signing
		rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		oidcPort := os.Getenv("ARGOCD_E2E_OIDC_PORT")
		if oidcPort == "" {
			oidcPort = "5556"
		}
		lc := net.ListenConfig{}
		ctx, cancelFunc := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancelFunc()
		ln, err := lc.Listen(ctx, "tcp", "localhost:"+oidcPort)
		require.NoError(t, err)
		mockServer, err = mockoidc.NewServer(rsaKey)
		require.NoError(t, err)
		err = mockServer.Start(ln, nil)
		require.NoError(t, err)
		t.Logf("OIDC server listening on %s", ln.Addr())
		t.Cleanup(func() {
			err := mockServer.Shutdown()
			require.NoError(t, err, "error shutting down mock oidc server")
		})
	})
	return mockServer
}

// WithDirectOIDC configures ArgoCD with oidc.config pointing directly to
// a shared mock OIDC server (bypassing Dex). Tests using this setup should
// use LoginWithSSO to mint OIDC tokens programmatically. This exercises the IDP
// token verification and revocation code path in SessionManager.VerifyToken.
func (a *Actions) WithDirectOIDC() *Actions {
	a.context.T().Helper()

	m := a.getSharedMockOIDCServer()

	// Do NOT shut down the mock server in t.Cleanup. The API server's
	// SessionManager caches the OIDC provider (mgr.prov) with this server's
	// issuer URL. If we shut it down, subsequent tests would start a new
	// server on a different port, causing issuer mismatch errors.

	// Configure oidc.config in argocd-cm pointing to the mock OIDC server
	oidcConfig := fmt.Sprintf("name: Mock OIDC\nissuer: %s\nclientID: %s\nclientSecret: %s\n",
		m.Issuer(), m.Config().ClientID, m.Config().ClientSecret)

	err := fixture.SetOIDCConfig(oidcConfig)
	require.NoError(a.context.T(), err)

	// Grant all users admin access so OIDC-authenticated users can call APIs
	err = fixture.SetParamInRBACConfigMap("policy.default", "role:admin")
	require.NoError(a.context.T(), err)

	a.context.T().Logf("Direct OIDC Issuer: %s, ClientID: %s", m.Issuer(), m.Config().ClientID)

	fixture.RestartAPIServer(a.context.T())
	return a
}

// Login creates a new session as admin using the argocd CLI login command.
// Each token name gets its own isolated config file so multiple sessions
// can coexist without interfering with each other.
func (a *Actions) Login(tokenName string) *Actions {
	a.context.T().Helper()

	cfgPath := a.configPathFor(tokenName)

	output, err := fixture.RunCliWithConfigFile(cfgPath,
		"login",
		fixture.GetApiServerAddress(),
		"--username", "admin",
		"--password", fixture.AdminPassword,
		"--skip-test-tls",
	)
	require.NoError(a.context.T(), err, "CLI login failed: %s", output)

	// Read the token back from the config file
	localCfg, err := localconfig.ReadLocalConfig(cfgPath)
	require.NoError(a.context.T(), err)
	require.NotNil(a.context.T(), localCfg)

	token := localCfg.GetToken(localCfg.CurrentContext)
	require.NotEmpty(a.context.T(), token, "no token found in config file after login")

	a.getTokenStore()[tokenName] = token
	return a
}

// Logout revokes the named token using the argocd CLI logout command.
// This triggers the full CLI logout flow including server-side token revocation.
func (a *Actions) Logout(tokenName string) *Actions {
	a.context.T().Helper()

	cfgPath := a.getConfigPaths()[tokenName]
	require.NotEmpty(a.context.T(), cfgPath, "config path for %q not found; call Login first", tokenName)

	output, err := fixture.RunCliWithConfigFile(cfgPath,
		"logout",
		fixture.GetApiServerAddress(),
	)
	require.NoError(a.context.T(), err, "CLI logout failed: %s", output)

	return a
}

// LoginWithSSO mints an OIDC token directly from the shared mock OIDC server,
// bypassing the normal browser-based SSO flow. Each call creates a unique user
// identity based on tokenName so multiple SSO sessions produce different tokens.
// This exercises the IDP token verification and revocation code path in
// SessionManager.VerifyToken.
func (a *Actions) LoginWithSSO(tokenName string) *Actions {
	a.context.T().Helper()

	require.NotNil(a.context.T(), mockServer, "LoginWithSSO requires WithDirectOIDC (mockServer is nil)")

	m := mockServer

	// Create a unique user for each token name
	user := &mockoidc.MockUser{
		Subject:           "sso-" + tokenName,
		Email:             tokenName + "@example.com",
		EmailVerified:     true,
		PreferredUsername: tokenName,
		Groups:            []string{"admins"},
	}

	// Create a session on the mock OIDC server
	session, err := m.SessionStore.NewSession(
		"openid email profile groups",
		"", // nonce
		user,
		"", // codeChallenge
		"", // codeChallengeMethod
	)
	require.NoError(a.context.T(), err)

	// Mint a signed ID token using the mock server's keypair
	idToken, err := session.IDToken(
		m.Config(),
		m.Keypair,
		m.Now(),
	)
	require.NoError(a.context.T(), err)

	a.getTokenStore()[tokenName] = idToken

	// Write a local config file so CLI logout can find and revoke this token
	serverAddr := fixture.GetApiServerAddress()
	cfgPath := a.configPathFor(tokenName)
	localCfg := localconfig.LocalConfig{
		CurrentContext: serverAddr,
		Contexts: []localconfig.ContextRef{
			{Name: serverAddr, Server: serverAddr, User: serverAddr},
		},
		Servers: []localconfig.Server{
			{Server: serverAddr, PlainText: true, Insecure: true},
		},
		Users: []localconfig.User{
			{Name: serverAddr, AuthToken: idToken},
		},
	}
	err = localconfig.WriteLocalConfig(localCfg, cfgPath)
	require.NoError(a.context.T(), err)

	return a
}

// runCli executes an argocd CLI command using the named token and records
// the output and error for assertion in Then().
func (a *Actions) runCli(tokenName string, args ...string) {
	a.context.T().Helper()

	token := a.getTokenStore()[tokenName]
	require.NotEmpty(a.context.T(), token, "token %q not found; call Login first", tokenName)

	a.lastOutput, a.lastError = fixture.RunCliWithToken(token, args...)
}

// GetUserInfo calls "argocd account get-user-info" using the named token.
func (a *Actions) GetUserInfo(tokenName string) *Actions {
	a.context.T().Helper()
	a.runCli(tokenName, "account", "get-user-info", "--grpc-web")
	return a
}

// ListProjects calls "argocd proj list" using the named token.
func (a *Actions) ListProjects(tokenName string) *Actions {
	a.context.T().Helper()
	a.runCli(tokenName, "proj", "list", "--grpc-web")
	return a
}

// ListApplications calls "argocd app list" using the named token.
func (a *Actions) ListApplications(tokenName string) *Actions {
	a.context.T().Helper()
	a.runCli(tokenName, "app", "list", "--grpc-web")
	log.Debugf("[token: %s, output: %s, err: %s]", tokenName, a.lastOutput, a.lastError)
	return a
}

// ListRepositories calls "argocd repo list" using the named token.
func (a *Actions) ListRepositories(tokenName string) *Actions {
	a.context.T().Helper()
	a.runCli(tokenName, "repo", "list", "--grpc-web")
	return a
}

// ListAccounts calls "argocd account list" using the named token.
func (a *Actions) ListAccounts(tokenName string) *Actions {
	a.context.T().Helper()
	a.runCli(tokenName, "account", "list")
	return a
}

// Sleep pauses execution for the given duration.
func (a *Actions) Sleep(d time.Duration) *Actions {
	time.Sleep(d)
	return a
}

func (a *Actions) Then() *Consequences {
	a.context.T().Helper()
	time.Sleep(fixture.WhenThenSleepInterval)
	return &Consequences{context: a.context, actions: a}
}
