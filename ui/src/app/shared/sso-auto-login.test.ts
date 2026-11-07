import {AuthSettings, UserInfo} from './models';
import {hasSSOProvider, isExternalIssuer, pauseAutoLogin, resumeAutoLoginForSession, shouldAutoLogin} from './sso-auto-login';

const oidcSettings = (overrides: Partial<AuthSettings> = {}) => ({ssoAutoLogin: true, oidcConfig: {name: 'Okta'}, ...overrides}) as AuthSettings;
const session = (overrides: Partial<UserInfo> = {}) => ({loggedIn: true, username: 'alice', iss: 'https://idp.example.com', groups: [], ...overrides}) as UserInfo;

beforeEach(() => {
    window.localStorage.clear();
});

describe('hasSSOProvider', () => {
    test('is false without settings', () => {
        expect(hasSSOProvider(null)).toBe(false);
    });

    test('is true with an OIDC provider', () => {
        expect(hasSSOProvider({oidcConfig: {name: 'Okta'}} as AuthSettings)).toBe(true);
    });

    test('is true with a Dex connector', () => {
        expect(hasSSOProvider({dexConfig: {connectors: [{name: 'GitHub', type: 'github'}]}} as AuthSettings)).toBe(true);
    });

    test('is false with Dex but no connectors', () => {
        expect(hasSSOProvider({dexConfig: {connectors: []}} as AuthSettings)).toBe(false);
    });
});

describe('isExternalIssuer', () => {
    test('is true for an SSO issuer', () => {
        expect(isExternalIssuer(session())).toBe(true);
    });

    test('is false for a token issued by Argo CD', () => {
        expect(isExternalIssuer(session({iss: 'argocd'}))).toBe(false);
    });

    test('is false without an issuer', () => {
        expect(isExternalIssuer(session({iss: ''}))).toBe(false);
        expect(isExternalIssuer(null)).toBe(false);
    });
});

describe('shouldAutoLogin', () => {
    test('redirects when enabled and SSO is configured', () => {
        expect(shouldAutoLogin(oidcSettings(), new URLSearchParams())).toBe(true);
    });

    test('waits for settings to load', () => {
        expect(shouldAutoLogin(null, new URLSearchParams())).toBe(false);
    });

    test('does not redirect when disabled', () => {
        expect(shouldAutoLogin(oidcSettings({ssoAutoLogin: false}), new URLSearchParams())).toBe(false);
    });

    test('does not redirect when SSO is not configured', () => {
        expect(shouldAutoLogin({ssoAutoLogin: true} as AuthSettings, new URLSearchParams())).toBe(false);
    });

    test('does not redirect after an SSO error', () => {
        expect(shouldAutoLogin(oidcSettings(), new URLSearchParams('has_sso_error=true'))).toBe(false);
    });

    test('does not redirect when opted out through the query string', () => {
        expect(shouldAutoLogin(oidcSettings(), new URLSearchParams('sso_auto_login=false&return_url=%2Fapplications'))).toBe(false);
    });

    test('does not redirect while paused', () => {
        pauseAutoLogin();
        expect(shouldAutoLogin(oidcSettings(), new URLSearchParams())).toBe(false);
    });

    test('does not redirect when storage is unavailable', () => {
        const getItem = jest.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
            throw new Error('blocked');
        });
        try {
            expect(shouldAutoLogin(oidcSettings(), new URLSearchParams())).toBe(false);
        } finally {
            getItem.mockRestore();
        }
    });
});

describe('resumeAutoLoginForSession', () => {
    test('resumes once the browser holds an SSO session', () => {
        pauseAutoLogin();
        resumeAutoLoginForSession(session());
        expect(shouldAutoLogin(oidcSettings(), new URLSearchParams())).toBe(true);
    });

    test('stays paused for a local account session', () => {
        pauseAutoLogin();
        resumeAutoLoginForSession(session({iss: 'argocd', username: 'admin'}));
        expect(shouldAutoLogin(oidcSettings(), new URLSearchParams())).toBe(false);
    });

    test('stays paused without a session', () => {
        pauseAutoLogin();
        resumeAutoLoginForSession(session({loggedIn: false}));
        resumeAutoLoginForSession(null);
        expect(shouldAutoLogin(oidcSettings(), new URLSearchParams())).toBe(false);
    });
});
