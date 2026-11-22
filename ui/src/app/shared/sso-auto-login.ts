import {AuthSettings, UserInfo} from './models';

function pausedStorageKey(): string {
    return `argocd.sso.autoLoginPaused:${document.querySelector('head > base')?.getAttribute('href') || '/'}`;
}

/** Query parameter that keeps the login page from redirecting to SSO when set to "false". */
export const ssoAutoLoginParam = 'sso_auto_login';

/** Reports whether an OIDC provider or at least one Dex connector is configured. */
export function hasSSOProvider(authSettings: AuthSettings | null | undefined): boolean {
    return (authSettings?.dexConfig?.connectors?.length ?? 0) > 0 || !!authSettings?.oidcConfig;
}

/** Reports whether the session token was issued by an SSO provider rather than by Argo CD itself. */
export function isExternalIssuer(userInfo: UserInfo | null | undefined): boolean {
    return !!userInfo?.iss && userInfo.iss !== 'argocd';
}

/**
 * Stops the login page of this Argo CD instance from redirecting to SSO in this browser, after a logout or a local
 * account login, so the user is neither signed straight back in nor moved off their local account when the session expires.
 */
export function pauseAutoLogin() {
    try {
        window.localStorage.setItem(pausedStorageKey(), String(Date.now()));
    } catch {
        return;
    }
}

/**
 * Resumes automatic SSO login once the browser holds an SSO session again, however the user reached it.
 * sessionCheckStartedAt is when the user info request started: a pause written at or after that time is newer than
 * the session the user info describes, so it is kept.
 */
export function resumeAutoLoginForSession(userInfo: UserInfo | null, sessionCheckStartedAt: number) {
    if (!userInfo?.loggedIn || !isExternalIssuer(userInfo)) {
        return;
    }
    try {
        const key = pausedStorageKey();
        const pausedAt = window.localStorage.getItem(key);
        if (pausedAt !== null && Number(pausedAt) < sessionCheckStartedAt) {
            window.localStorage.removeItem(key);
        }
    } catch {
        return;
    }
}

function isAutoLoginPaused(): boolean {
    try {
        return window.localStorage.getItem(pausedStorageKey()) !== null;
    } catch {
        return true;
    }
}

/** Reports whether the login page should redirect to the SSO provider without waiting for a click. */
export function shouldAutoLogin(authSettings: AuthSettings | null, search: URLSearchParams): boolean {
    if (!authSettings || !authSettings.ssoAutoLogin || !hasSSOProvider(authSettings)) {
        return false;
    }
    if (search.get('has_sso_error') === 'true' || search.get(ssoAutoLoginParam) === 'false') {
        return false;
    }
    return !isAutoLoginPaused();
}

/** Sends the browser to the given SSO login URL. */
export function redirectToSSO(url: string) {
    window.location.href = url;
}
