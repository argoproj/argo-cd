package session

import (
	"path/filepath"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v3/util/localconfig"
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
