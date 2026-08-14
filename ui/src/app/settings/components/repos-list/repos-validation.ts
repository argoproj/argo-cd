export interface GitHubAppCredentialValues {
    githubAppId?: bigint | number;
    githubAppInstallationId?: bigint | number;
    githubAppPrivateKey?: string;
}

export interface GitHubAppCredentialErrors {
    githubAppId?: string | false;
    githubAppPrivateKey?: string | false;
}

export const validateGitHubAppCredentials = (values: GitHubAppCredentialValues): GitHubAppCredentialErrors => {
    const hasNumericCredential = (value: bigint | number | undefined) => value !== undefined;
    const hasDirectCredentials = hasNumericCredential(values.githubAppId) || hasNumericCredential(values.githubAppInstallationId) || Boolean(values.githubAppPrivateKey?.trim());
    if (!hasDirectCredentials) {
        return {};
    }

    return {
        githubAppId: !values.githubAppId && 'GitHub App ID is required',
        githubAppPrivateKey: !values.githubAppPrivateKey?.trim() && 'GitHub App private Key is required'
    };
};
