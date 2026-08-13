export interface GitHubAppCredentialValues {
    githubAppId?: bigint;
    githubAppInstallationId?: bigint;
    githubAppPrivateKey?: string;
}

export interface GitHubAppCredentialErrors {
    githubAppId?: string | false;
    githubAppPrivateKey?: string | false;
}

export const validateGitHubAppCredentials = (values: GitHubAppCredentialValues): GitHubAppCredentialErrors => {
    const hasDirectCredentials = Boolean(values.githubAppId || values.githubAppInstallationId || values.githubAppPrivateKey);
    if (!hasDirectCredentials) {
        return {};
    }

    return {
        githubAppId: !values.githubAppId && 'GitHub App ID is required',
        githubAppPrivateKey: !values.githubAppPrivateKey && 'GitHub App private Key is required'
    };
};
