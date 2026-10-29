import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import '@testing-library/jest-dom';
import {MemoryRouter, Route} from 'react-router';

import {Provider} from '../../../shared/context';
import {services} from '../../../shared/services';
import {AccountDetails} from './account-details';

jest.mock('../../../shared/components', () => ({
    ...jest.requireActual('../../../shared/components'),
    Page: (props: {children?: React.ReactNode}) => props.children
}));

jest.mock('../../../shared/services', () => ({
    services: {accounts: {get: jest.fn(), createToken: jest.fn(), deleteToken: jest.fn(() => Promise.resolve())}}
}));

const account = (tokens: {id: string; issuedAt: number}[]) => ({name: 'alice', enabled: true, capabilities: ['apiKey'], tokens});

const renderDetails = () =>
    render(
        <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
            <Provider value={{notifications: {show: jest.fn()}, navigation: {goto: jest.fn()}, popup: {confirm: () => Promise.resolve(true)}} as any}>
                <MemoryRouter initialEntries={['/settings/accounts/alice']}>
                    <Route path='/settings/accounts/:name' component={AccountDetails} />
                </MemoryRouter>
            </Provider>
        </QueryClientProvider>
    );

describe('AccountDetails', () => {
    beforeEach(() => jest.clearAllMocks());

    it('loads the account and its tokens with a single request', async () => {
        jest.mocked(services.accounts.get).mockResolvedValue(account([{id: 'ci', issuedAt: 1}]) as any);
        renderDetails();

        expect(screen.getByText('Loading...')).toBeInTheDocument();
        expect(await screen.findByText('ci')).toBeInTheDocument();
        expect(screen.getByText('apiKey')).toBeInTheDocument();
        expect(services.accounts.get).toHaveBeenCalledTimes(1);
        expect(services.accounts.get).toHaveBeenCalledWith('alice');
    });

    it('refetches the account after deleting a token', async () => {
        jest.mocked(services.accounts.get)
            .mockResolvedValueOnce(account([{id: 'ci', issuedAt: 1}]) as any)
            .mockResolvedValueOnce(account([]) as any);
        renderDetails();

        fireEvent.click(await screen.findByTitle('Delete'));

        await waitFor(() => expect(services.accounts.deleteToken).toHaveBeenCalledWith('alice', 'ci'));
        expect(await screen.findByText("Account has no tokens. Click 'Generate New' to create one.")).toBeInTheDocument();
        expect(services.accounts.get).toHaveBeenCalledTimes(2);
    });

    it('refetches the account after generating a token', async () => {
        jest.mocked(services.accounts.get)
            .mockResolvedValueOnce(account([]) as any)
            .mockResolvedValueOnce(account([{id: 'deploy', issuedAt: 1}]) as any);
        jest.mocked(services.accounts.createToken).mockResolvedValue('new-token-value');
        renderDetails();

        fireEvent.click(await screen.findByText('Generate New'));

        expect(await screen.findByText('new-token-value')).toBeInTheDocument();
        expect(await screen.findByText('deploy')).toBeInTheDocument();
        expect(services.accounts.get).toHaveBeenCalledTimes(2);
    });

    it('keeps the new token visible when the refetch after generating it fails', async () => {
        jest.mocked(services.accounts.get)
            .mockResolvedValueOnce(account([]) as any)
            .mockRejectedValueOnce(new Error('boom'));
        jest.mocked(services.accounts.createToken).mockResolvedValue('new-token-value');
        renderDetails();

        fireEvent.click(await screen.findByText('Generate New'));

        await waitFor(() => expect(services.accounts.get).toHaveBeenCalledTimes(2));
        expect(await screen.findByText('new-token-value')).toBeInTheDocument();
        expect(screen.queryByText('try again')).not.toBeInTheDocument();
    });

    it('offers a retry when the account fails to load', async () => {
        jest.mocked(services.accounts.get)
            .mockRejectedValueOnce(new Error('boom'))
            .mockResolvedValueOnce(account([]) as any);
        renderDetails();

        fireEvent.click(await screen.findByText('try again'));

        expect(await screen.findByText('alice')).toBeInTheDocument();
        expect(services.accounts.get).toHaveBeenCalledTimes(2);
    });
});
