package generators

// Legacy* accessors are the sole allowed readers of deprecated SCMConfig fields.
// InitConfigProvider captures them into StaticFields; product code reads via
// configProvider / SCMConfig resolve helpers.

func (c *SCMConfig) LegacyScmRootCAPath() string {
	return c.scmRootCAPath
}

//nolint:staticcheck // SA1019: sole allowed reader of deprecated allowedSCMProviders
func (c *SCMConfig) LegacyAllowedSCMProviders() []string {
	return c.allowedSCMProviders
}

//nolint:staticcheck // SA1019: sole allowed reader of deprecated enableSCMProviders
func (c *SCMConfig) LegacyEnableSCMProviders() bool {
	return c.enableSCMProviders
}

//nolint:staticcheck // SA1019: sole allowed reader of deprecated enableGitHubAPIMetrics
func (c *SCMConfig) LegacyEnableGitHubAPIMetrics() bool {
	return c.enableGitHubAPIMetrics
}

//nolint:staticcheck // SA1019: sole allowed reader of deprecated tokenRefStrictMode
func (c *SCMConfig) LegacyTokenRefStrictMode() bool {
	return c.tokenRefStrictMode
}

//nolint:staticcheck // SA1019: sole allowed reader of deprecated scmProxyURL
func (c *SCMConfig) LegacyScmProxyURL() string {
	return c.scmProxyURL
}

//nolint:staticcheck // SA1019: sole allowed reader of deprecated scmNoProxy
func (c *SCMConfig) LegacyScmNoProxy() string {
	return c.scmNoProxy
}
