import {validateGitHubAppCredentials} from './repos-validation';

describe('validateGitHubAppCredentials', () => {
    test('allows all credential fields to be empty so a matching template can be used', () => {
        expect(validateGitHubAppCredentials({})).toEqual({});
    });

    test('accepts directly supplied GitHub App credentials', () => {
        expect(
            validateGitHubAppCredentials({
                githubAppId: 123n,
                githubAppInstallationId: 456n,
                githubAppPrivateKey: 'private-key'
            })
        ).toEqual({githubAppId: false, githubAppPrivateKey: false});
    });

    test('requires the app ID when other GitHub App credentials are supplied', () => {
        expect(validateGitHubAppCredentials({githubAppPrivateKey: 'private-key'})).toEqual({
            githubAppId: 'GitHub App ID is required',
            githubAppPrivateKey: false
        });
    });

    test('requires the private key when other GitHub App credentials are supplied', () => {
        expect(validateGitHubAppCredentials({githubAppId: 123n})).toEqual({
            githubAppId: false,
            githubAppPrivateKey: 'GitHub App private Key is required'
        });
    });

    test('does not treat an installation ID by itself as template-based authentication', () => {
        expect(validateGitHubAppCredentials({githubAppInstallationId: 456n})).toEqual({
            githubAppId: 'GitHub App ID is required',
            githubAppPrivateKey: 'GitHub App private Key is required'
        });
    });

    test('does not treat a zero app ID as empty credentials', () => {
        expect(validateGitHubAppCredentials({githubAppId: 0})).toEqual({
            githubAppId: 'GitHub App ID is required',
            githubAppPrivateKey: 'GitHub App private Key is required'
        });
    });

    test('does not treat a zero installation ID as empty credentials', () => {
        expect(validateGitHubAppCredentials({githubAppInstallationId: 0})).toEqual({
            githubAppId: 'GitHub App ID is required',
            githubAppPrivateKey: 'GitHub App private Key is required'
        });
    });

    test.each([{githubAppId: Number.NaN}, {githubAppInstallationId: Number.NaN}])('does not allow an unnormalized numeric field: %p', values => {
        expect(validateGitHubAppCredentials(values)).toEqual({
            githubAppId: 'GitHub App ID is required',
            githubAppPrivateKey: 'GitHub App private Key is required'
        });
    });
});
