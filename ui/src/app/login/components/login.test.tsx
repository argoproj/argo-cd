import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {act, fireEvent, render, screen, waitFor} from '@testing-library/react';
import {createMemoryHistory} from 'history';
import {RouteComponentProps} from 'react-router';

import {Context} from '../../shared/context';
import {services} from '../../shared/services';
import {Login} from './login';

jest.mock('../../shared/services', () => ({services: {authService: {settings: jest.fn()}, users: {login: jest.fn()}}}));

const renderLogin = (search = '', baseHref = '/') => {
    const client = new QueryClient();
    client.setQueryData(['applications', 'list'], ['previous-user-app']);
    client.setQueryData(['projects', 'list'], ['previous-user-project']);
    const goto = jest.fn(() => client.getQueryCache().getAll().length);
    const history = createMemoryHistory({initialEntries: [`/login${search}`]});
    render(
        <QueryClientProvider client={client}>
            <Context.Provider value={{navigation: {goto}, baseHref} as React.ContextType<typeof Context>}>
                <Login {...({history} as RouteComponentProps<{}>)} />
            </Context.Provider>
        </QueryClientProvider>
    );
    return {client, goto};
};

const submitLogin = async () => {
    await screen.findByRole('button', {name: 'Sign In'});
    fireEvent.change(screen.getByLabelText('Username'), {target: {value: 'new-user'}});
    fireEvent.change(screen.getByLabelText('Password'), {target: {value: 'password'}});
    await act(async () => {
        fireEvent.submit(screen.getByRole('form'));
    });
};

describe('Login query cache', () => {
    beforeEach(() => {
        jest.resetAllMocks();
        jest.mocked(services.authService.settings).mockResolvedValue({} as any);
    });

    it.each([
        ['', '/', '/applications'],
        ['?return_url=%2Fargo%2Fapplications%2Fmy-app%3Fview%3Dtree', '/argo', '/applications/my-app?view=tree']
    ])('clears previous session data before redirecting after successful login (%s)', async (search, baseHref, redirectPath) => {
        let completeLogin: (result: {token: string}) => void;
        jest.mocked(services.users.login).mockReturnValue(new Promise(resolve => (completeLogin = resolve)));
        const {client, goto} = renderLogin(search, baseHref);

        await submitLogin();
        expect(services.users.login).toHaveBeenCalledWith('new-user', 'password');
        expect(client.getQueryCache().getAll()).toHaveLength(2);
        expect(screen.getByRole('button', {name: 'Sign In'})).toBeDisabled();

        await act(async () => completeLogin({token: 'new-session'}));
        await waitFor(() => expect(goto).toHaveBeenCalledWith(redirectPath, undefined, {replace: true}));
        expect(goto.mock.results[1].value).toBe(0);
        expect(client.getQueryCache().getAll()).toHaveLength(0);
    });

    it('preserves the cache and stays on the login page when authentication fails', async () => {
        jest.mocked(services.users.login).mockRejectedValue(new Error('Invalid credentials'));
        const {client, goto} = renderLogin();

        await submitLogin();
        expect(await screen.findByText('Invalid credentials')).toBeInTheDocument();
        expect(client.getQueryData(['applications', 'list'])).toEqual(['previous-user-app']);
        expect(client.getQueryData(['projects', 'list'])).toEqual(['previous-user-project']);
        expect(goto).toHaveBeenCalledTimes(1);
        expect(goto).toHaveBeenCalledWith('.', {sso_error: null});
        expect(screen.getByRole('button', {name: 'Sign In'})).toBeEnabled();
    });
});
