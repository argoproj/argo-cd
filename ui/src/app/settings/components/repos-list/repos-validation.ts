interface GitHubAppCredentialValues {
    ghType?: string;
    githubAppId?: bigint | number | null;
    githubAppInstallationId?: bigint | number | null;
    githubAppPrivateKey?: string;
    tlsClientCertData?: string;
    tlsClientCertKey?: string;
    write?: boolean;
}

export const normalizeGitHubAppCredentials = <T extends GitHubAppCredentialValues>(values: T) => ({
    ...values,
    githubAppPrivateKey: values.githubAppPrivateKey?.trim() ? values.githubAppPrivateKey : '',
    tlsClientCertData: values.ghType === 'GitHub' ? '' : values.tlsClientCertData?.trim() || '',
    tlsClientCertKey: values.ghType === 'GitHub' ? '' : values.tlsClientCertKey?.trim() || ''
});

export const validateGitHubAppCredentials = (values: GitHubAppCredentialValues, requireCredentials = false) => {
    const normalizedValues = normalizeGitHubAppCredentials(values);
    const hasDirectCredentials =
        [normalizedValues.githubAppId, normalizedValues.githubAppInstallationId].some(value => value != null && !Number.isNaN(value)) ||
        Boolean(normalizedValues.githubAppPrivateKey || normalizedValues.tlsClientCertData || normalizedValues.tlsClientCertKey);
    if (!requireCredentials && !normalizedValues.write && !hasDirectCredentials) {
        return {};
    }

    return {
        githubAppId: !normalizedValues.githubAppId && 'GitHub App ID is required',
        githubAppPrivateKey: !normalizedValues.githubAppPrivateKey && 'GitHub App private Key is required',
        ...(normalizedValues.tlsClientCertKey && !normalizedValues.tlsClientCertData ? {tlsClientCertData: 'TLS client cert is required if TLS client cert key is given.'} : {}),
        ...(normalizedValues.tlsClientCertData && !normalizedValues.tlsClientCertKey ? {tlsClientCertKey: 'TLS client cert key is required if TLS client cert is given.'} : {})
    };
};
