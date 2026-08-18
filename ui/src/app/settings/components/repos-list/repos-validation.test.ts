import {normalizeGitHubAppCredentials, validateGitHubAppCredentials} from './repos-validation';

describe('validateGitHubAppCredentials', () => {
    const requiredErrors = {
        githubAppId: 'GitHub App ID is required',
        githubAppPrivateKey: 'GitHub App private Key is required'
    };
    const tlsClientCertDataRequired = 'TLS client cert is required if TLS client cert key is given.';
    const tlsClientCertKeyRequired = 'TLS client cert key is required if TLS client cert is given.';

    test('allows all credential fields to be empty so a matching template can be used', () => {
        expect(validateGitHubAppCredentials({})).toEqual({});
    });

    test('requires credentials when saving a credentials template', () => {
        expect(validateGitHubAppCredentials({}, true)).toEqual(requiredErrors);
    });

    test('requires credentials when connecting a write repository', () => {
        expect(validateGitHubAppCredentials({write: true})).toEqual(requiredErrors);
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
        expect(validateGitHubAppCredentials({githubAppInstallationId: 456n})).toEqual(requiredErrors);
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

    test('treats whitespace-only TLS credentials as empty', () => {
        expect(validateGitHubAppCredentials({tlsClientCertData: '  \n', tlsClientCertKey: '\t'})).toEqual({});
    });

    test('requires direct credentials when a TLS client certificate prevents template inheritance', () => {
        expect(validateGitHubAppCredentials({tlsClientCertData: 'certificate'})).toEqual({...requiredErrors, tlsClientCertKey: tlsClientCertKeyRequired});
    });

    test('requires direct credentials and certificate data when a TLS client certificate key prevents template inheritance', () => {
        expect(validateGitHubAppCredentials({tlsClientCertKey: 'key'})).toEqual({...requiredErrors, tlsClientCertData: tlsClientCertDataRequired});
    });

    test('requires a TLS client certificate key with direct GitHub App credentials and certificate data', () => {
        expect(validateGitHubAppCredentials({githubAppId: 123n, githubAppPrivateKey: 'private-key', tlsClientCertData: 'certificate'})).toEqual({
            githubAppId: false,
            githubAppPrivateKey: false,
            tlsClientCertKey: tlsClientCertKeyRequired
        });
    });

    test('normalizes TLS credentials before submission', () => {
        expect(
            normalizeGitHubAppCredentials({
                ghType: 'GitHub Enterprise',
                tlsClientCertData: ' \n-----BEGIN CERTIFICATE-----\ncertificate\n-----END CERTIFICATE-----\n ',
                tlsClientCertKey: '\n key \t'
            })
        ).toMatchObject({tlsClientCertData: '-----BEGIN CERTIFICATE-----\ncertificate\n-----END CERTIFICATE-----', tlsClientCertKey: 'key'});
        expect(normalizeGitHubAppCredentials({ghType: 'GitHub', tlsClientCertData: 'certificate', tlsClientCertKey: 'key'})).toMatchObject({
            tlsClientCertData: '',
            tlsClientCertKey: ''
        });
        expect(validateGitHubAppCredentials({ghType: 'GitHub', tlsClientCertData: 'certificate'})).toEqual({});
    });
});
