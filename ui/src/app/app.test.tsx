import * as React from 'react';
import {act, render} from '@testing-library/react';
import '@testing-library/jest-dom';

import {services} from './shared/services';
import {App} from './app';

jest.mock('./shared/services', () => ({
    services: {
        viewPreferences: {init: jest.fn(), getPreferences: jest.fn(() => ({subscribe: jest.fn(() => ({unsubscribe: jest.fn()}))}))},
        version: {version: jest.fn(() => Promise.resolve({Version: 'v0.0.0'}))},
        extensions: {addEventListener: jest.fn()},
        authService: {settings: jest.fn()},
        users: {get: jest.fn()}
    }
}));

jest.mock('./shared/services/requests', () => ({
    __esModule: true,
    default: {
        setBaseHRef: jest.fn(),
        toAbsURL: (url: string) => url,
        onError: {subscribe: jest.fn(() => ({unsubscribe: jest.fn()}))}
    }
}));

const pausedKey = 'argocd.sso.autoLoginPaused';

const renderApp = async () => {
    await act(async () => {
        render(<App />);
    });
    expect(console.error).not.toHaveBeenCalledWith('Failed to bootstrap app session', expect.anything());
};

beforeEach(() => {
    window.localStorage.clear();
    window.localStorage.setItem(pausedKey, 'true');
    jest.mocked(services.authService.settings).mockResolvedValue({oidcConfig: {name: 'Keycloak'}, ssoAutoLogin: true} as any);
    jest.spyOn(console, 'error');
});

afterEach(() => {
    jest.mocked(console.error).mockRestore();
});

describe('App bootstrap', () => {
    it('resumes SSO auto-login when the browser holds an SSO session', async () => {
        jest.mocked(services.users.get).mockResolvedValue({loggedIn: true, username: 'alice', iss: 'https://idp.example.com', groups: []});

        await renderApp();

        expect(window.localStorage.getItem(pausedKey)).toBeNull();
    });

    it('keeps SSO auto-login paused for a local account session', async () => {
        jest.mocked(services.users.get).mockResolvedValue({loggedIn: true, username: 'admin', iss: 'argocd', groups: []});

        await renderApp();

        expect(window.localStorage.getItem(pausedKey)).toBe('true');
    });
});
