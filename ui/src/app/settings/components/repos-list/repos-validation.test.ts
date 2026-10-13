import {validateGitHubAppCredentials} from './repos-validation';

describe('validateGitHubAppCredentials', () => {
    const requiredErrors = {
        githubAppId: 'GitHub App ID is required',
        githubAppPrivateKey: 'GitHub App private Key is required'
    };
    test('allows all credential fields to be empty so a matching template can be used', () => {
        expect(validateGitHubAppCredentials({})).toEqual({});
    });

    test('requires credentials when requested', () => {
        expect(validateGitHubAppCredentials({}, true)).toEqual(requiredErrors);
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

    test.each([
        [{githubAppPrivateKey: 'private-key'}, {githubAppId: 'GitHub App ID is required', githubAppPrivateKey: false}],
        [{githubAppId: 123n}, {githubAppId: false, githubAppPrivateKey: 'GitHub App private Key is required'}],
        [{githubAppInstallationId: 456n}, requiredErrors]
    ])('requires complete direct credentials: %p', (values, expected) => {
        expect(validateGitHubAppCredentials(values)).toEqual(expected);
    });

    test.each([{githubAppId: 0}, {githubAppInstallationId: 0}])('does not treat a zero ID as empty credentials: %p', values => {
        expect(validateGitHubAppCredentials(values)).toEqual(requiredErrors);
    });

    test.each([{githubAppId: Number.NaN}, {githubAppInstallationId: Number.NaN}])('treats an unnormalized numeric field as empty: %p', values => {
        expect(validateGitHubAppCredentials(values)).toEqual({});
    });

    test('treats a whitespace-only private key as empty', () => {
        expect(validateGitHubAppCredentials({githubAppPrivateKey: '  \n'})).toEqual({});
    });
});
