import * as React from 'react';
import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import '@testing-library/jest-dom';
import {createMemoryHistory} from 'history';

import {Context} from '../../shared/context';
import {AuthSettings} from '../../shared/models';
import {services} from '../../shared/services';
import {pauseAutoLogin, redirectToSSO} from '../../shared/sso-auto-login';
import {Login} from './login';

jest.mock('../../shared/services', () => ({
    services: {
        authService: {settings: jest.fn()},
        users: {login: jest.fn()}
    }
}));

jest.mock('../../shared/sso-auto-login', () => ({
    ...jest.requireActual('../../shared/sso-auto-login'),
    redirectToSSO: jest.fn()
}));

const settings = (overrides: Partial<AuthSettings> = {}) => ({ssoAutoLogin: true, oidcConfig: {name: 'Keycloak'}, ...overrides}) as AuthSettings;

const renderLogin = (path: string) => {
    const history = createMemoryHistory({initialEntries: [path]});
    const appContext = {navigation: {goto: jest.fn()}, baseHref: '/'} as any;
    return render(
        <Context.Provider value={appContext}>
            <Login history={history} location={history.location} match={{} as any} />
        </Context.Provider>
    );
};

beforeEach(() => {
    window.localStorage.clear();
    jest.mocked(redirectToSSO).mockClear();
    jest.mocked(services.users.login).mockResolvedValue({token: 'token'});
});

describe('Login', () => {
    it('redirects to SSO when auto-login is enabled', async () => {
        jest.mocked(services.authService.settings).mockResolvedValue(settings());

        renderLogin('/login?return_url=%2Fapplications');

        await waitFor(() => expect(redirectToSSO).toHaveBeenCalledWith('auth/login?return_url=%2Fapplications'));
    });

    it('waits for a click when auto-login is disabled', async () => {
        jest.mocked(services.authService.settings).mockResolvedValue(settings({ssoAutoLogin: false}));

        renderLogin('/login');

        expect(await screen.findByText('Log in via Keycloak')).toBeInTheDocument();
        expect(redirectToSSO).not.toHaveBeenCalled();
    });

    it('waits for a click while auto-login is paused', async () => {
        jest.mocked(services.authService.settings).mockResolvedValue(settings());
        pauseAutoLogin();

        renderLogin('/login');

        expect(await screen.findByText('Log in via Keycloak')).toBeInTheDocument();
        expect(redirectToSSO).not.toHaveBeenCalled();
    });

    it('shows the local login form when opted out', async () => {
        jest.mocked(services.authService.settings).mockResolvedValue(settings());

        renderLogin('/login?sso_auto_login=false');

        expect(await screen.findByLabelText('Username')).toBeInTheDocument();
        expect(redirectToSSO).not.toHaveBeenCalled();
    });

    it('pauses auto-login after a local account login', async () => {
        jest.mocked(services.authService.settings).mockResolvedValue(settings());

        const {container} = renderLogin('/login?sso_auto_login=false');
        fireEvent.change(await screen.findByLabelText('Username'), {target: {value: 'admin'}});
        fireEvent.change(screen.getByLabelText('Password'), {target: {value: 'password'}});
        fireEvent.submit(container.querySelector('form'));

        await waitFor(() => expect(services.users.login).toHaveBeenCalledWith('admin', 'password'));
        await waitFor(() => expect(window.localStorage.getItem('argocd.sso.autoLoginPaused')).toBe('true'));
    });
});
