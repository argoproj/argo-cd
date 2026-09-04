interface GitHubAppCredentialValues {
    githubAppId?: bigint | number | null;
    githubAppInstallationId?: bigint | number | null;
    githubAppPrivateKey?: string;
}

export const validateGitHubAppCredentials = (values: GitHubAppCredentialValues, requireCredentials = false) => {
    const hasDirectCredentials =
        [values.githubAppId, values.githubAppInstallationId].some(value => value != null && !Number.isNaN(value)) || Boolean(values.githubAppPrivateKey?.trim());
    if (!requireCredentials && !hasDirectCredentials) {
        return {};
    }

    return {
        githubAppId: !values.githubAppId && 'GitHub App ID is required',
        githubAppPrivateKey: !values.githubAppPrivateKey?.trim() && 'GitHub App private Key is required'
    };
};
